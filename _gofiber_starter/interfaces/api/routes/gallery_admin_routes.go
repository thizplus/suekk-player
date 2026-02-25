package routes

import (
	"github.com/gofiber/fiber/v2"
	"gofiber-template/interfaces/api/handlers"
	"gofiber-template/interfaces/api/middleware"
)

// SetupGalleryAdminRoutes กำหนด routes สำหรับ Admin จัดการ Gallery
// ใช้ manual selection flow: source → safe/nsfw
func SetupGalleryAdminRoutes(api fiber.Router, h *handlers.Handlers) {
	// Admin Gallery routes (require auth)
	adminGallery := api.Group("/admin/videos", middleware.Protected())

	// ดึงภาพทั้งหมดใน gallery (source, safe, nsfw)
	adminGallery.Get("/:id/gallery", h.GalleryAdminHandler.GetGalleryImages)

	// ย้ายภาพเดี่ยว
	adminGallery.Post("/:id/gallery/move", h.GalleryAdminHandler.MoveImage)

	// ย้ายหลายภาพ (batch)
	adminGallery.Post("/:id/gallery/move-batch", h.GalleryAdminHandler.MoveBatch)

	// Publish gallery (set status = ready)
	adminGallery.Post("/:id/gallery/publish", h.GalleryAdminHandler.PublishGallery)
}
