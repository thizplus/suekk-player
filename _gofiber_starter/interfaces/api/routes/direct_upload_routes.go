package routes

import (
	"github.com/gofiber/fiber/v2"
	"gofiber-template/interfaces/api/handlers"
	"gofiber-template/interfaces/api/middleware"
)

// SetupDirectUploadRoutes ตั้งค่า routes สำหรับ Direct Upload (Presigned URL)
func SetupDirectUploadRoutes(api fiber.Router, h *handlers.Handlers) {
	directUpload := api.Group("/direct-upload")

	// ทุก endpoint ต้อง login
	protected := directUpload.Group("", middleware.Protected())

	// POST /api/v1/direct-upload/init - เริ่ม multipart upload, รับ presigned URLs
	protected.Post("/init", h.DirectUploadHandler.InitUpload)

	// POST /api/v1/direct-upload/complete - รวม parts และ auto-queue transcode
	protected.Post("/complete", h.DirectUploadHandler.CompleteUpload)

	// DELETE /api/v1/direct-upload/abort - ยกเลิก upload ที่ค้าง
	protected.Delete("/abort", h.DirectUploadHandler.AbortUpload)

	// Config endpoint (ต้อง login)
	config := api.Group("/config", middleware.Protected())

	// GET /api/v1/config/upload-limits - ดึง upload limits สำหรับ frontend
	config.Get("/upload-limits", h.DirectUploadHandler.GetUploadLimits)
}
