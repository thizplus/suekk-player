package routes

import (
	"github.com/gofiber/fiber/v2"
	"gofiber-template/interfaces/api/handlers"
	"gofiber-template/interfaces/api/middleware"
)

// SetupSubtitleRoutes กำหนด routes สำหรับ subtitle operations
func SetupSubtitleRoutes(api fiber.Router, h *handlers.Handlers) {
	// === Public Routes ===
	subtitles := api.Group("/subtitles")
	subtitles.Get("/languages", h.SubtitleHandler.GetSupportedLanguages) // รายการภาษาที่รองรับ

	// === Public Embed Routes (ไม่ต้อง auth - สำหรับ embed player) ===
	embed := api.Group("/embed")
	embed.Get("/videos/:code/subtitles", h.SubtitleHandler.GetSubtitlesByCode) // ดึง subtitles โดยใช้ video code

	// === Internal Worker Callback Routes (ไม่ต้อง auth) ===
	// ใช้ path /internal/... เพื่อหลีกเลี่ยง conflict กับ routes อื่น
	internal := api.Group("/internal")

	// Job started callback (สำหรับ queue → processing transition)
	internal.Post("/subtitles/job-started", h.SubtitleHandler.JobStarted) // callback เมื่อ worker เริ่มทำ job

	// Callback สำหรับ detect language (ใช้ video_id)
	internal.Post("/videos/:id/subtitle/callback/detect", h.SubtitleHandler.DetectComplete) // callback เมื่อ detect เสร็จ

	// Callback สำหรับ subtitle by ID
	internal.Post("/subtitles/:id/callback/transcribe", h.SubtitleHandler.TranscribeComplete) // callback เมื่อ transcribe เสร็จ
	internal.Post("/subtitles/:id/callback/translate", h.SubtitleHandler.TranslationComplete) // callback เมื่อ translate เสร็จ
	internal.Post("/subtitles/:id/callback/failed", h.SubtitleHandler.SubtitleFailed)         // callback เมื่อ failed

	// === Video Subtitle Routes (Protected) ===
	videos := api.Group("/videos")
	protected := videos.Group("", middleware.Protected())
	protected.Get("/:id/subtitles", h.SubtitleHandler.GetSubtitles)                  // ดึง subtitles ของ video
	protected.Post("/:id/subtitle/detect", h.SubtitleHandler.TriggerDetectLanguage)  // trigger detect language
	protected.Post("/:id/subtitle/language", h.SubtitleHandler.SetLanguage)          // ตั้งค่าภาษาด้วยตนเอง
	protected.Post("/:id/subtitle/transcribe", h.SubtitleHandler.TriggerTranscribe)  // trigger transcribe
	protected.Post("/:id/subtitle/translate", h.SubtitleHandler.TriggerTranslation)  // trigger translation

	// === Subtitle Management Routes (Protected) ===
	subtitlesProtected := subtitles.Group("", middleware.Protected())
	subtitlesProtected.Delete("/:id", h.SubtitleHandler.DeleteSubtitle)              // ลบ subtitle
	subtitlesProtected.Get("/:id/content", h.SubtitleHandler.GetSubtitleContent)     // ดึง content ของ subtitle (SRT)
	subtitlesProtected.Put("/:id/content", h.SubtitleHandler.UpdateSubtitleContent)  // อัปเดต content ของ subtitle (SRT)

	// === Admin Routes (Protected) ===
	admin := api.Group("/admin", middleware.Protected())
	admin.Post("/subtitles/retry-stuck", h.SubtitleHandler.RetryStuckSubtitles) // retry stuck subtitles ทั้งหมด
}
