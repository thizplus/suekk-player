package routes

import (
	"github.com/gofiber/fiber/v2"
	"gofiber-template/interfaces/api/handlers"
	"gofiber-template/interfaces/api/middleware"
)

func SetupTranscodingRoutes(api fiber.Router, h *handlers.Handlers) {
	transcoding := api.Group("/transcoding")

	// Protected routes (ต้อง login + admin)
	protected := transcoding.Group("", middleware.Protected())
	protected.Post("/queue/:id", h.TranscodingHandler.QueueVideo)          // ส่งวิดีโอเข้า queue
	protected.Get("/status", h.TranscodingHandler.GetQueueStatus)          // ดูสถานะ queue
	protected.Get("/stats", h.TranscodingHandler.GetStats)                 // ดูสถิติจำนวนวิดีโอตาม status
	protected.Post("/queue-pending", h.TranscodingHandler.QueuePendingVideos)    // ส่งวิดีโอ pending ทั้งหมดเข้า queue
	protected.Post("/requeue-stuck", h.TranscodingHandler.RequeueStuckVideos)    // ส่งวิดีโอที่ค้างกลับเข้า queue
	protected.Post("/mark-stuck-failed", h.TranscodingHandler.MarkStuckAsFailed) // มาร์ควิดีโอที่ค้างเป็น failed
	protected.Delete("/clear-all", h.TranscodingHandler.ClearAllVideos)          // ลบ videos ทั้งหมด (testing)

	// Worker Management (Phase 1 - Monitor)
	protected.Get("/workers", h.TranscodingHandler.GetWorkers) // ดึงรายการ Workers ทั้งหมด
}
