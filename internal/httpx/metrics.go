package httpx

import (
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// MetricsMiddleware tracks HTTP metrics using OpenTelemetry
type MetricsMiddleware struct {
	requestCounter  metric.Int64Counter
	requestDuration metric.Float64Histogram
	activeRequests  metric.Int64UpDownCounter
}

// NewMetricsMiddleware creates middleware with OpenTelemetry metrics
func NewMetricsMiddleware() (*MetricsMiddleware, error) {
	meter := otel.Meter("acai.chat.http")

	// Total number of HTTP requests
	requestCounter, err := meter.Int64Counter(
		"http.server.request.count",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request counter: %w", err)
	}

	// HTTP request duration in seconds
	requestDuration, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("Duration of HTTP requests"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request duration histogram: %w", err)
	}

	// Number of active HTTP requests
	activeRequests, err := meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Number of active HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create active requests gauge: %w", err)
	}

	return &MetricsMiddleware{
		requestCounter:  requestCounter,
		requestDuration: requestDuration,
		activeRequests:  activeRequests,
	}, nil
}

// Handler returns an HTTP middleware that records metrics
func (m *MetricsMiddleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx := r.Context()

			// Track active requests
			attributes := []attribute.KeyValue{
				attribute.String("http.method", r.Method),
				attribute.String("http.route", r.URL.Path),
			}
			m.activeRequests.Add(ctx, 1, metric.WithAttributes(attributes...))
			defer m.activeRequests.Add(ctx, -1, metric.WithAttributes(attributes...))

			// Wrap response writer to capture status code
			srw := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Handle request
			next.ServeHTTP(srw, r)

			// Calculate duration
			duration := time.Since(start).Seconds()

			// Record metrics with dimensions
			metricAttrs := []attribute.KeyValue{
				attribute.String("http.method", r.Method),
				attribute.String("http.route", r.URL.Path),
				attribute.Int("http.status_code", srw.statusCode),
			}

			m.requestCounter.Add(ctx, 1, metric.WithAttributes(metricAttrs...))
			m.requestDuration.Record(ctx, duration, metric.WithAttributes(metricAttrs...))
		})
	}
}

// statusResponseWriter wraps http.ResponseWriter to capture status code
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (w *statusResponseWriter) WriteHeader(statusCode int) {
	if !w.written {
		w.statusCode = statusCode
		w.written = true
		w.ResponseWriter.WriteHeader(statusCode)
	}
}

func (w *statusResponseWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}
