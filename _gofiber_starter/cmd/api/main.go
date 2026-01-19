package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"gofiber-template/interfaces/api/handlers"
	"gofiber-template/interfaces/api/middleware"
	"gofiber-template/interfaces/api/routes"
	"gofiber-template/pkg/di"
	"gofiber-template/pkg/logger"
)

func main() {
	// Initialize DI container
	container := di.NewContainer()

	// Initialize all dependencies (including logger)
	if err := container.Initialize(); err != nil {
		// ใช้ log พื้นฐานก่อน logger init
		panic("Failed to initialize container: " + err.Error())
	}

	// Setup graceful shutdown
	setupGracefulShutdown(container)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		ErrorHandler:          middleware.ErrorHandler(),
		AppName:               container.GetConfig().App.Name,
		BodyLimit:             10 * 1024 * 1024 * 1024, // 10GB for large video uploads
		StreamRequestBody:     true,                    // Stream large files instead of buffering
		DisableStartupMessage: false,
	})

	// Setup middleware (order matters!)
	app.Use(middleware.RequestIDMiddleware()) // ต้องมาก่อน logger
	app.Use(middleware.LoggerMiddleware())
	app.Use(middleware.CorsMiddleware())

	// Create handlers from services
	services := container.GetHandlerServices()
	h := handlers.NewHandlers(services)

	// Setup routes
	routes.SetupRoutes(app, h)

	// Start server
	port := container.GetConfig().App.Port
	logger.Info("Server starting",
		"port", port,
		"env", container.GetConfig().App.Env,
		"app", container.GetConfig().App.Name,
	)
	logger.Info("Endpoints available",
		"health", "http://localhost:"+port+"/health",
		"api", "http://localhost:"+port+"/api/v1",
		"websocket", "ws://localhost:"+port+"/ws",
	)

	if err := app.Listen(":" + port); err != nil {
		logger.Error("Server failed to start", "error", err)
		os.Exit(1)
	}
}

func setupGracefulShutdown(container *di.Container) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		logger.Info("Gracefully shutting down...")

		if err := container.Cleanup(); err != nil {
			logger.Error("Error during cleanup", "error", err)
		}

		logger.Info("Shutdown complete")
		os.Exit(0)
	}()
}
