package routes

import (
	"github.com/gofiber/fiber/v2"
	"gofiber-template/interfaces/api/handlers"
	"gofiber-template/interfaces/api/middleware"
)

func SetupProgressRoutes(router fiber.Router, h *handlers.Handlers) {
	progress := router.Group("/progress")

	// Public route - get progress by video ID
	progress.Get("/video/:id", h.ProgressHandler.GetProgress)

	// Protected routes
	protected := progress.Group("")
	protected.Use(middleware.Protected())
	protected.Get("/my", h.ProgressHandler.GetMyProgress)
}
