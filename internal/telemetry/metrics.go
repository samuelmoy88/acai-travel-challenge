package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// InitMetrics initializes OpenTelemetry metrics with a simple stdout exporter
func InitMetrics(ctx context.Context) (func(context.Context) error, error) {
	// Create a stdout exporter for simplicity
	exporter, err := stdoutmetric.New(
		stdoutmetric.WithPrettyPrint(), // Human-readable output
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout exporter: %w", err)
	}

	// Create resource to identify this service
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("acai-chat-service"),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create meter provider with periodic reader
	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(
			metric.NewPeriodicReader(
				exporter,
				metric.WithInterval(10*time.Second), // Export every 10 seconds
			),
		),
	)

	// Set global meter provider
	otel.SetMeterProvider(meterProvider)

	// Return shutdown function
	return meterProvider.Shutdown, nil
}
