package routes

import (
	"github.com/gofiber/fiber/v2"
	"gofiber-template/interfaces/api/handlers"
	"gofiber-template/interfaces/api/middleware"
)

func SetupVideoRoutes(api fiber.Router, h *handlers.Handlers) {
	videos := api.Group("/videos")

	// Public routes
	videos.Get("/ready", h.VideoHandler.ListReady)            // ดึงเฉพาะ videos ที่พร้อม stream
	videos.Get("/code/:code", h.VideoHandler.GetByCode)       // ดึง video ตาม code (สำหรับ embed)
	videos.Get("/embed/:code", h.VideoHandler.GetEmbed)       // ดึงข้อมูลสำหรับ embed player

	// Internal routes (for worker callbacks)
	internal := api.Group("/internal/videos")
	internal.Patch("/:id/gallery", h.VideoHandler.UpdateGallery) // Worker callback เมื่อ gallery เสร็จ

	// Protected routes (ต้อง login)
	protected := videos.Group("", middleware.Protected())
	protected.Post("/", h.VideoHandler.Upload)                // อัปโหลดวิดีโอใหม่
	protected.Post("/upload", h.VideoHandler.Upload)          // Alias for upload (frontend compatibility)
	protected.Post("/batch", h.VideoHandler.BatchUpload)      // อัปโหลดหลายไฟล์พร้อมกัน (สูงสุด 10 ไฟล์)
	protected.Get("/", h.VideoHandler.List)                   // ดึง videos ทั้งหมด (admin)
	protected.Get("/my", h.VideoHandler.GetMyVideos)          // ดึง videos ของตัวเอง
	protected.Get("/stats", h.VideoHandler.GetStats)          // ดึง stats (admin)

	// Dead Letter Queue (DLQ) Management - Admin only
	// ต้องอยู่ก่อน /:id routes เพื่อไม่ให้ "dlq" ถูกจับเป็น :id
	dlq := protected.Group("/dlq")
	dlq.Get("/", h.VideoHandler.ListDLQ)                      // ดึง videos ที่อยู่ใน DLQ
	dlq.Post("/:id/retry", h.VideoHandler.RetryDLQ)           // Retry video จาก DLQ
	dlq.Delete("/:id", h.VideoHandler.DeleteDLQ)              // ลบ video จาก DLQ

	// Parameterized routes - ต้องอยู่หลัง specific routes
	protected.Get("/:id", h.VideoHandler.GetByID)             // ดึง video ตาม ID
	protected.Put("/:id", h.VideoHandler.Update)              // อัปเดต video
	protected.Delete("/:id", h.VideoHandler.Delete)           // ลบ video
	protected.Post("/:id/generate-gallery", h.VideoHandler.GenerateGallery) // สร้าง gallery จาก HLS
}
