package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"seo-worker/config"
	"seo-worker/container"
)

func main() {
	// Setup logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("Starting SEO Content Worker")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	logger.Info("Configuration loaded",
		"worker_id", cfg.Worker.ID,
		"concurrency", cfg.Worker.Concurrency,
		"gemini_model", cfg.Gemini.Model,
	)

	// Create container
	c, err := container.NewContainer(cfg)
	if err != nil {
		logger.Error("Failed to create container", "error", err)
		os.Exit(1)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("Received shutdown signal", "signal", sig)
		cancel()
	}()

	// Start worker
	logger.Info("Worker starting...")
	if err := c.Start(ctx); err != nil {
		logger.Error("Worker error", "error", err)
	}

	// Graceful shutdown
	c.Stop()
	logger.Info("Worker shutdown complete")
}
