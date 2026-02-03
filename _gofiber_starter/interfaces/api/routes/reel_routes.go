package routes

import (
	"github.com/gofiber/fiber/v2"
	"gofiber-template/interfaces/api/handlers"
	"gofiber-template/interfaces/api/middleware"
)

// SetupReelRoutes กำหนด routes สำหรับ reel operations
func SetupReelRoutes(api fiber.Router, h *handlers.Handlers) {
	// === Templates (Public - สำหรับดู templates) ===
	templates := api.Group("/reels/templates")
	templates.Get("/", h.ReelHandler.GetTemplates)          // GET /api/v1/reels/templates
	templates.Get("/:id", h.ReelHandler.GetTemplateByID)    // GET /api/v1/reels/templates/:id

	// === Reels (Protected) ===
	reels := api.Group("/reels", middleware.Protected())
	reels.Post("/", h.ReelHandler.Create)                   // POST /api/v1/reels
	reels.Get("/", h.ReelHandler.List)                      // GET /api/v1/reels
	reels.Get("/:id", h.ReelHandler.GetByID)                // GET /api/v1/reels/:id
	reels.Put("/:id", h.ReelHandler.Update)                 // PUT /api/v1/reels/:id
	reels.Delete("/:id", h.ReelHandler.Delete)              // DELETE /api/v1/reels/:id
	reels.Post("/:id/export", h.ReelHandler.Export)         // POST /api/v1/reels/:id/export

	// === Video Reels (Protected) ===
	videos := api.Group("/videos", middleware.Protected())
	videos.Get("/:id/reels", h.ReelHandler.ListByVideo)     // GET /api/v1/videos/:id/reels
}
