package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/suekk/my_worker/go/transcode/application"
	"github.com/suekk/my_worker/go/transcode/config"
	"github.com/suekk/my_worker/go/transcode/infrastructure/ffmpeg"
	"github.com/suekk/my_worker/go/transcode/infrastructure/logger"
	natspkg "github.com/suekk/my_worker/go/transcode/infrastructure/nats"
	"github.com/suekk/my_worker/go/transcode/infrastructure/storage"
)

func main() {
	// Load .env from multiple locations
	// 1. _my_worker/.env (shared)
	// 2. _my_worker/go/transcode/.env (worker-specific override)
	// 3. Current directory
	godotenv.Load("../../.env", "../.env", ".env")

	// Load config first (needed for logger config)
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Setup structured logging with file output
	logCfg := logger.Config{
		Level:         cfg.LogLevel,
		Format:        cfg.LogFormat,
		Output:        cfg.LogOutput,
		FilePath:      cfg.LogFilePath,
		ErrorFilePath: cfg.LogErrorPath,
		MaxSizeMB:     cfg.LogMaxSizeMB,
		MaxBackups:    cfg.LogMaxBackups,
		MaxAgeDays:    cfg.LogMaxAgeDays,
		Compress:      true,
	}

	if err := logger.Init(logCfg); err != nil {
		fmt.Printf("Failed to init logger: %v\n", err)
		os.Exit(1)
	}

	slog.Info("========================================")
	slog.Info("   Transcode Worker Starting           ")
	slog.Info("========================================")
	slog.Info("Log file", "path", logCfg.FilePath, "error_path", logCfg.ErrorFilePath)

	slog.Info("Config loaded",
		"worker_id", cfg.WorkerID,
		"nats_url", cfg.NATSURL,
		"stream", cfg.StreamName,
		"consumer", cfg.ConsumerName,
		"s3_endpoint", cfg.S3Endpoint,
		"s3_bucket", cfg.S3Bucket,
		"gpu_enabled", cfg.GPUEnabled,
		"concurrency", cfg.Concurrency,
	)

	// Create S3 storage client
	s3Client, err := storage.NewS3Client(cfg)
	if err != nil {
		slog.Error("Failed to create S3 client", "error", err)
		os.Exit(1)
	}

	// Create FFmpeg transcoder
	transcoder := ffmpeg.NewTranscoder(cfg)

	// Create NATS consumer
	consumer, err := natspkg.NewConsumer(cfg)
	if err != nil {
		slog.Error("Failed to create NATS consumer", "error", err)
		os.Exit(1)
	}
	defer consumer.Close()

	// Create progress publisher
	progressPub := natspkg.NewProgressPublisher(consumer.Connection(), cfg.WorkerID)

	// Create job publisher
	jobPub := natspkg.NewJobPublisher(consumer.JetStream())

	// Create handler
	handler := application.NewHandler(cfg, s3Client, transcoder, progressPub, jobPub)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		slog.Info("Received signal, shutting down", "signal", sig)
		cancel()
	}()

	// Start consuming jobs
	slog.Info("========================================")
	slog.Info("   Worker Ready - Waiting for Jobs     ")
	slog.Info("========================================")

	if err := consumer.Start(ctx, handler.HandleJob); err != nil {
		slog.Error("Consumer error", "error", err)
		os.Exit(1)
	}

	slog.Info("========================================")
	slog.Info("   Worker Stopped                      ")
	slog.Info("========================================")
}
