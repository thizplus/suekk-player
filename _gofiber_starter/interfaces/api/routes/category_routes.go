package routes

import (
	"github.com/gofiber/fiber/v2"
	"gofiber-template/interfaces/api/handlers"
	"gofiber-template/interfaces/api/middleware"
)

func SetupCategoryRoutes(api fiber.Router, h *handlers.Handlers) {
	categories := api.Group("/categories")

	// Public routes
	categories.Get("/", h.CategoryHandler.List)               // ดึง categories ทั้งหมด (flat)
	categories.Get("/tree", h.CategoryHandler.ListTree)       // ดึง categories แบบ tree
	categories.Get("/:id", h.CategoryHandler.GetByID)         // ดึง category ตาม ID
	categories.Get("/slug/:slug", h.CategoryHandler.GetBySlug) // ดึง category ตาม slug

	// Protected routes (ต้อง login + admin)
	protected := categories.Group("", middleware.Protected())
	protected.Post("/", h.CategoryHandler.Create)             // สร้าง category ใหม่
	protected.Put("/reorder", h.CategoryHandler.Reorder)      // จัดเรียง categories ใหม่
	protected.Put("/:id", h.CategoryHandler.Update)           // อัปเดต category
	protected.Delete("/:id", h.CategoryHandler.Delete)        // ลบ category
}
