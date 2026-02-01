package routes

import (
	"github.com/gofiber/fiber/v2"
	"gofiber-template/interfaces/api/handlers"
	"gofiber-template/interfaces/api/middleware"
)

// SetupQueueRoutes กำหนด routes สำหรับ queue management (admin only)
func SetupQueueRoutes(api fiber.Router, h *handlers.Handlers) {
	// All queue routes require authentication
	admin := api.Group("/admin/queues", middleware.Protected())

	// Queue stats
	admin.Get("/stats", h.QueueHandler.GetQueueStats)

	// Transcode queue
	transcode := admin.Group("/transcode")
	transcode.Get("/failed", h.QueueHandler.GetTranscodeFailed)
	transcode.Post("/retry-all", h.QueueHandler.RetryTranscodeFailed)
	transcode.Post("/:id/retry", h.QueueHandler.RetryTranscodeOne)

	// Subtitle queue
	subtitle := admin.Group("/subtitle")
	subtitle.Get("/stuck", h.QueueHandler.GetSubtitleStuck)
	subtitle.Get("/failed", h.QueueHandler.GetSubtitleFailed)
	subtitle.Post("/retry-all", h.QueueHandler.RetrySubtitleStuck)
	subtitle.Delete("/clear-all", h.QueueHandler.ClearSubtitleStuck)
	subtitle.Post("/queue-missing", h.QueueHandler.QueueMissingSubtitles)

	// Warm cache queue
	warmCache := admin.Group("/warm-cache")
	warmCache.Get("/pending", h.QueueHandler.GetWarmCachePending)
	warmCache.Get("/failed", h.QueueHandler.GetWarmCacheFailed)
	warmCache.Post("/:id/warm", h.QueueHandler.WarmCacheOne)
	warmCache.Post("/warm-all", h.QueueHandler.WarmCacheAll)
}
