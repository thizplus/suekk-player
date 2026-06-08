package routes

import (
	"github.com/gofiber/fiber/v2"
	"gofiber-template/interfaces/api/handlers"
	"gofiber-template/interfaces/api/middleware"
)

func SetupSeriesRoutes(api fiber.Router, h *handlers.Handlers) {
	series := api.Group("/series")

	// ═══ Public routes (Frontend / เว็บอื่นใช้) ═══
	series.Get("/", h.SeriesHandler.List)
	series.Get("/code/:code", h.SeriesHandler.GetByCode)
	series.Get("/slug/:slug", h.SeriesHandler.GetBySlug)
	series.Get("/:id/episodes", h.SeriesHandler.ListEpisodes)

	// ═══ Series Categories (Public) ═══
	series.Get("/categories", h.SeriesHandler.ListCategories)

	// ═══ Protected routes (Admin + Bot) ═══
	protected := series.Group("", middleware.Protected())
	protected.Post("/", h.SeriesHandler.Create)
	protected.Post("/upsert", h.SeriesHandler.Upsert)
	protected.Put("/:id", h.SeriesHandler.Update)
	protected.Delete("/:id", h.SeriesHandler.Delete)

	// Episodes management
	protected.Post("/:id/episodes", h.SeriesHandler.AddEpisodes)
	protected.Patch("/:id/episodes/:episode", h.SeriesHandler.UpdateEpisode)

	// Series Categories management
	protected.Post("/categories", h.SeriesHandler.CreateCategory)
}
