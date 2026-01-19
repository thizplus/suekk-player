package routes

import (
	"github.com/gofiber/fiber/v2"
	"gofiber-template/interfaces/api/handlers"
	"gofiber-template/interfaces/api/middleware"
)

func SetupStorageRoutes(router fiber.Router, h *handlers.Handlers) {
	storage := router.Group("/storage")

	// Protected routes (require auth)
	storage.Use(middleware.Protected())

	// Storage usage (for quota display)
	storage.Get("/usage", h.StorageHandler.GetStorageUsage)

	// Admin routes - storage management
	storage.Get("/stats", h.StorageHandler.GetStorageStats)
	storage.Post("/cleanup", h.StorageHandler.TriggerCleanup)
}
