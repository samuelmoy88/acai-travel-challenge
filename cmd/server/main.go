package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/acai-travel/tech-challenge/internal/chat"
	"github.com/acai-travel/tech-challenge/internal/chat/assistant"
	"github.com/acai-travel/tech-challenge/internal/chat/model"
	"github.com/acai-travel/tech-challenge/internal/httpx"
	"github.com/acai-travel/tech-challenge/internal/mongox"
	"github.com/acai-travel/tech-challenge/internal/pb"
	"github.com/acai-travel/tech-challenge/internal/telemetry"
	"github.com/gorilla/mux"
	"github.com/twitchtv/twirp"
)

func main() {
	ctx := context.Background()

	// Initialize OpenTelemetry metrics
	shutdownMetrics, err := telemetry.InitMetrics(ctx)
	if err != nil {
		slog.Error("Failed to initialize metrics", "error", err)
		os.Exit(1)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownMetrics(ctx); err != nil {
			slog.Error("Failed to shutdown metrics", "error", err)
		}
	}()

	// Initialize dependencies
	mongo := mongox.MustConnect()
	repo := model.New(mongo)
	assist := assistant.New()
	server := chat.NewServer(repo, assist)

	// Create metrics middleware
	metricsMiddleware, err := httpx.NewMetricsMiddleware()
	if err != nil {
		slog.Error("Failed to create metrics middleware", "error", err)
		os.Exit(1)
	}

	// Configure handler
	handler := mux.NewRouter()
	handler.Use(
		metricsMiddleware.Handler(), // Add metrics FIRST
		httpx.Logger(),
		httpx.Recovery(),
	)

	handler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "Hi, my name is Clippy!")
	})

	handler.PathPrefix("/twirp/").Handler(
		pb.NewChatServiceServer(server, twirp.WithServerJSONSkipDefaults(true)),
	)

	// Start server with graceful shutdown
	srv := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	// Channel to listen for shutdown signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		slog.Info("Starting the server on :8080...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	<-stop
	slog.Info("Shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown error", "error", err)
	}

	slog.Info("Server stopped")
}
