package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/suekk/my_worker/go/gallery/application"
	"github.com/suekk/my_worker/go/gallery/config"
	"github.com/suekk/my_worker/go/gallery/infrastructure/classifier"
	"github.com/suekk/my_worker/go/gallery/infrastructure/ffmpeg"
	natspkg "github.com/suekk/my_worker/go/gallery/infrastructure/nats"
	"github.com/suekk/my_worker/go/gallery/infrastructure/storage"
)

func main() {
	// Load .env from multiple locations
	// 1. _my_worker/.env (shared)
	// 2. _my_worker/go/gallery/.env (worker-specific override)
	// 3. Current directory
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

	logger.Info("========================================")
	logger.Info("   Gallery Worker Starting             ")
	logger.Info("========================================")

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
		"s3_endpoint", cfg.S3Endpoint,
		"s3_bucket", cfg.S3Bucket,
		"python_path", cfg.PythonPath,
		"concurrency", cfg.Concurrency,
	)

	// Create S3 storage client
	s3Client, err := storage.NewS3Client(cfg)
	if err != nil {
		logger.Error("Failed to create S3 client", "error", err)
		os.Exit(1)
	}

	// Create FFmpeg extractor
	extractor := ffmpeg.NewExtractor(cfg)

	// Create NSFW classifier
	nsfwClassifier := classifier.NewNSFWClassifier(cfg)

	// Check classifier availability
	if !nsfwClassifier.IsAvailable() {
		logger.Warn("NSFW classifier not fully available, will fallback to safe classification")
	}

	// Create NATS consumer
	consumer, err := natspkg.NewConsumer(cfg)
	if err != nil {
		logger.Error("Failed to create NATS consumer", "error", err)
		os.Exit(1)
	}
	defer consumer.Close()

	// Create progress publisher
	progressPub := natspkg.NewProgressPublisher(consumer.Connection(), cfg.WorkerID)

	// Create handler
	handler := application.NewHandler(cfg, s3Client, extractor, nsfwClassifier, progressPub)

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
	logger.Info("========================================")
	logger.Info("   Worker Ready - Waiting for Jobs     ")
	logger.Info("========================================")

	if err := consumer.Start(ctx, handler.HandleJob); err != nil {
		logger.Error("Consumer error", "error", err)
		os.Exit(1)
	}

	logger.Info("========================================")
	logger.Info("   Worker Stopped                      ")
	logger.Info("========================================")
}
