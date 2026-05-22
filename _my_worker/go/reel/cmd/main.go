package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/suekk/my_worker/go/reel/application"
	"github.com/suekk/my_worker/go/reel/config"
	"github.com/suekk/my_worker/go/reel/infrastructure/ffmpeg"
	natspkg "github.com/suekk/my_worker/go/reel/infrastructure/nats"
	"github.com/suekk/my_worker/go/reel/infrastructure/storage"
)

func main() {
	godotenv.Load("../../.env", "../.env", ".env")

	logLevel := slog.LevelInfo
	if os.Getenv("DEBUG") == "true" {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	logger.Info("========================================")
	logger.Info("   Reel Worker Starting                ")
	logger.Info("========================================")

	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	logger.Info("Config loaded",
		"worker_id", cfg.WorkerID,
		"nats_url", cfg.NATSURL,
		"stream", cfg.StreamName,
		"gpu_enabled", cfg.GPUEnabled,
	)

	// Create S3 storage client
	s3Client, err := storage.NewS3Client(cfg)
	if err != nil {
		logger.Error("Failed to create S3 client", "error", err)
		os.Exit(1)
	}

	// Create FFmpeg processor
	processor := ffmpeg.NewProcessor(cfg)

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
	handler := application.NewHandler(cfg, s3Client, processor, progressPub)

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
