package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/suekk/my_worker/go/warmcache/application"
	"github.com/suekk/my_worker/go/warmcache/config"
	"github.com/suekk/my_worker/go/warmcache/infrastructure/cdn"
	natspkg "github.com/suekk/my_worker/go/warmcache/infrastructure/nats"
)

func main() {
	// Load .env from multiple locations (first found wins)
	// 1. _my_worker/.env (shared)
	// 2. _my_worker/go/warmcache/.env (worker-specific override)
	// 3. Current directory .env
	godotenv.Load("../../.env", "../.env", ".env")

	// Setup structured logging
	logLevel := slog.LevelInfo
	if os.Getenv("DEBUG") == "true" || os.Getenv("DEBUG") == "1" {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	logger.Info("Starting warmcache worker")

	// Load config
	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	logger.Info("Config loaded",
		"worker_id", cfg.WorkerID,
		"nats_url", cfg.NATSURL,
		"stream", cfg.StreamName,
		"consumer", cfg.ConsumerName,
		"cdn_url", cfg.CDNURL,
		"concurrency", cfg.Concurrency,
		"segment_concurrency", cfg.SegmentConcurrency,
	)

	// Create NATS consumer
	consumer, err := natspkg.NewConsumer(cfg)
	if err != nil {
		logger.Error("Failed to create NATS consumer", "error", err)
		os.Exit(1)
	}
	defer consumer.Close()

	// Create progress publisher
	progressPub := natspkg.NewProgressPublisher(consumer.Connection(), cfg.WorkerID)

	// Create CDN warmer
	cdnWarmer := cdn.NewHTTPWarmer(cfg)

	// Create handler
	handler := application.NewHandler(cfg, cdnWarmer, progressPub)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("Received signal, shutting down", "signal", sig)
		cancel()
	}()

	// Start consuming jobs
	logger.Info("Worker ready, waiting for jobs")
	if err := consumer.Start(ctx, handler.HandleJob); err != nil {
		logger.Error("Consumer error", "error", err)
		os.Exit(1)
	}

	logger.Info("Worker stopped")
}
