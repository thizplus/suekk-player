package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gofiber-template/domain/services"
	natspkg "gofiber-template/infrastructure/nats"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/utils"
)

type QueueHandler struct {
	queueService  services.QueueService
	natsPublisher *natspkg.Publisher
}

func NewQueueHandler(queueService services.QueueService, natsPublisher *natspkg.Publisher) *QueueHandler {
	return &QueueHandler{
		queueService:  queueService,
		natsPublisher: natsPublisher,
	}
}

// GetQueueStats ดึงสถิติ queue ทั้งหมด
// GET /api/v1/admin/queues/stats
func (h *QueueHandler) GetQueueStats(c *fiber.Ctx) error {
	ctx := c.UserContext()

	stats, err := h.queueService.GetQueueStats(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get queue stats", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, stats)
}

// === Transcode Queue ===

// GetTranscodeFailed ดึงรายการ transcode failed
// GET /api/v1/admin/queues/transcode/failed
func (h *QueueHandler) GetTranscodeFailed(c *fiber.Ctx) error {
	ctx := c.UserContext()

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	items, total, err := h.queueService.GetTranscodeFailed(ctx, page, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get transcode failed", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.PaginatedSuccessResponse(c, items, total, page, limit)
}

// RetryTranscodeFailed retry transcode failed ทั้งหมด
// POST /api/v1/admin/queues/transcode/retry-all
func (h *QueueHandler) RetryTranscodeFailed(c *fiber.Ctx) error {
	ctx := c.UserContext()

	logger.InfoContext(ctx, "Retry transcode failed request")

	result, err := h.queueService.RetryTranscodeFailed(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to retry transcode", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, result)
}

// RetryTranscodeOne retry transcode 1 video
// POST /api/v1/admin/queues/transcode/:id/retry
func (h *QueueHandler) RetryTranscodeOne(c *fiber.Ctx) error {
	ctx := c.UserContext()

	videoIDStr := c.Params("id")
	videoID, err := uuid.Parse(videoIDStr)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid video ID")
	}

	logger.InfoContext(ctx, "Retry transcode one request", "video_id", videoID)

	if err := h.queueService.RetryTranscodeOne(ctx, videoID); err != nil {
		logger.WarnContext(ctx, "Failed to retry transcode", "video_id", videoID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	return utils.SuccessResponse(c, fiber.Map{
		"message": "Transcode job queued",
	})
}

// === Subtitle Queue ===

// GetSubtitleStuck ดึงรายการ subtitle stuck
// GET /api/v1/admin/queues/subtitle/stuck
func (h *QueueHandler) GetSubtitleStuck(c *fiber.Ctx) error {
	ctx := c.UserContext()

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	items, total, err := h.queueService.GetSubtitleStuck(ctx, page, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get subtitle stuck", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.PaginatedSuccessResponse(c, items, total, page, limit)
}

// GetSubtitleFailed ดึงรายการ subtitle failed
// GET /api/v1/admin/queues/subtitle/failed
func (h *QueueHandler) GetSubtitleFailed(c *fiber.Ctx) error {
	ctx := c.UserContext()

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	items, total, err := h.queueService.GetSubtitleFailed(ctx, page, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get subtitle failed", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.PaginatedSuccessResponse(c, items, total, page, limit)
}

// RetrySubtitleStuck retry subtitle stuck ทั้งหมด
// POST /api/v1/admin/queues/subtitle/retry-all
func (h *QueueHandler) RetrySubtitleStuck(c *fiber.Ctx) error {
	ctx := c.UserContext()

	logger.InfoContext(ctx, "Retry subtitle stuck request")

	result, err := h.queueService.RetrySubtitleStuck(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to retry subtitle stuck", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, result)
}

// ClearSubtitleStuck ลบ subtitle stuck ทั้งหมด
// DELETE /api/v1/admin/queues/subtitle/clear-all
func (h *QueueHandler) ClearSubtitleStuck(c *fiber.Ctx) error {
	ctx := c.UserContext()

	logger.InfoContext(ctx, "Clear subtitle stuck request")

	result, err := h.queueService.ClearSubtitleStuck(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to clear subtitle stuck", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, result)
}

// QueueMissingSubtitles สแกน videos ที่ยังไม่มี subtitle แล้ว queue ใหม่
// POST /api/v1/admin/queues/subtitle/queue-missing
func (h *QueueHandler) QueueMissingSubtitles(c *fiber.Ctx) error {
	ctx := c.UserContext()

	logger.InfoContext(ctx, "Queue missing subtitles request")

	result, err := h.queueService.QueueMissingSubtitles(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to queue missing subtitles", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, result)
}

// === Warm Cache Queue ===

// GetWarmCachePending ดึงรายการ video ที่ยังไม่ได้ warm cache
// GET /api/v1/admin/queues/warm-cache/pending
func (h *QueueHandler) GetWarmCachePending(c *fiber.Ctx) error {
	ctx := c.UserContext()

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	items, total, err := h.queueService.GetWarmCachePending(ctx, page, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get warm cache pending", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.PaginatedSuccessResponse(c, items, total, page, limit)
}

// GetWarmCacheFailed ดึงรายการ video ที่ warm cache failed
// GET /api/v1/admin/queues/warm-cache/failed
func (h *QueueHandler) GetWarmCacheFailed(c *fiber.Ctx) error {
	ctx := c.UserContext()

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	items, total, err := h.queueService.GetWarmCacheFailed(ctx, page, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get warm cache failed", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.PaginatedSuccessResponse(c, items, total, page, limit)
}

// WarmCacheOne warm cache video 1 ตัว
// POST /api/v1/admin/queues/warm-cache/:id/warm
func (h *QueueHandler) WarmCacheOne(c *fiber.Ctx) error {
	ctx := c.UserContext()

	videoIDStr := c.Params("id")
	videoID, err := uuid.Parse(videoIDStr)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid video ID")
	}

	logger.InfoContext(ctx, "Warm cache one request", "video_id", videoID)

	result, err := h.queueService.WarmCacheOne(ctx, videoID)
	if err != nil {
		logger.WarnContext(ctx, "Failed to warm cache", "video_id", videoID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	return utils.SuccessResponse(c, result)
}

// WarmCacheAll warm cache ทุก video ที่ยังไม่ได้ warm
// POST /api/v1/admin/queues/warm-cache/warm-all
func (h *QueueHandler) WarmCacheAll(c *fiber.Ctx) error {
	ctx := c.UserContext()

	logger.InfoContext(ctx, "Warm cache all request")

	result, err := h.queueService.WarmCacheAll(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to warm cache all", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, result)
}

// === Gallery Queue ===

// GetGalleryProcessing ดึงรายการ gallery ที่กำลังสร้าง
// GET /api/v1/admin/queues/gallery/processing
func (h *QueueHandler) GetGalleryProcessing(c *fiber.Ctx) error {
	ctx := c.UserContext()

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	items, total, err := h.queueService.GetGalleryProcessing(ctx, page, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get gallery processing", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.PaginatedSuccessResponse(c, items, total, page, limit)
}

// GetGalleryFailed ดึงรายการ gallery ที่ล้มเหลว
// GET /api/v1/admin/queues/gallery/failed
func (h *QueueHandler) GetGalleryFailed(c *fiber.Ctx) error {
	ctx := c.UserContext()

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	items, total, err := h.queueService.GetGalleryFailed(ctx, page, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get gallery failed", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.PaginatedSuccessResponse(c, items, total, page, limit)
}

// RetryGalleryAll retry gallery ที่ failed ทั้งหมด
// POST /api/v1/admin/queues/gallery/retry-all
func (h *QueueHandler) RetryGalleryAll(c *fiber.Ctx) error {
	ctx := c.UserContext()

	logger.InfoContext(ctx, "Retry gallery all request")

	result, err := h.queueService.RetryGalleryAll(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to retry gallery all", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, result)
}

// === Reel Queue ===

// GetReelExporting ดึงรายการ reel ที่กำลัง export
// GET /api/v1/admin/queues/reel/exporting
func (h *QueueHandler) GetReelExporting(c *fiber.Ctx) error {
	ctx := c.UserContext()

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	items, total, err := h.queueService.GetReelExporting(ctx, page, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get reel exporting", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.PaginatedSuccessResponse(c, items, total, page, limit)
}

// GetReelFailed ดึงรายการ reel ที่ export failed
// GET /api/v1/admin/queues/reel/failed
func (h *QueueHandler) GetReelFailed(c *fiber.Ctx) error {
	ctx := c.UserContext()

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	items, total, err := h.queueService.GetReelFailed(ctx, page, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get reel failed", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.PaginatedSuccessResponse(c, items, total, page, limit)
}

// RetryReelAll retry reel ที่ failed ทั้งหมด
// POST /api/v1/admin/queues/reel/retry-all
func (h *QueueHandler) RetryReelAll(c *fiber.Ctx) error {
	ctx := c.UserContext()

	logger.InfoContext(ctx, "Retry reel all request")

	result, err := h.queueService.RetryReelAll(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to retry reel all", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, result)
}

// === Online Workers ===

// GetOnlineWorkers ดึงรายการ Workers ที่ online จาก NATS KV
// GET /api/v1/admin/queues/workers/online
func (h *QueueHandler) GetOnlineWorkers(c *fiber.Ctx) error {
	ctx := c.UserContext()

	if h.natsPublisher == nil {
		return utils.SuccessResponse(c, fiber.Map{
			"workers":      []interface{}{},
			"total_online": 0,
			"message":      "NATS not connected",
		})
	}

	workers, err := h.natsPublisher.GetAllWorkers(ctx)
	if err != nil {
		logger.WarnContext(ctx, "Failed to get workers", "error", err)
		return utils.SuccessResponse(c, fiber.Map{
			"workers":      []interface{}{},
			"total_online": 0,
		})
	}

	// Calculate summary
	var totalIdle, totalProcessing, totalStopping, totalPaused int
	var totalJobs int
	var transcodeCount, subtitleCount int
	for _, w := range workers {
		switch w.Status {
		case "idle":
			totalIdle++
		case "processing":
			totalProcessing++
		case "stopping":
			totalStopping++
		case "paused":
			totalPaused++
		}
		totalJobs += len(w.CurrentJobs)

		// Count by worker type
		switch w.WorkerType {
		case "subtitle":
			subtitleCount++
		default:
			transcodeCount++ // Default to transcode if not specified
		}
	}

	return utils.SuccessResponse(c, fiber.Map{
		"workers":      workers,
		"total_online": len(workers),
		"summary": fiber.Map{
			"idle":       totalIdle,
			"processing": totalProcessing,
			"stopping":   totalStopping,
			"paused":     totalPaused,
			"total_jobs": totalJobs,
			"by_type": fiber.Map{
				"transcode": transcodeCount,
				"subtitle":  subtitleCount,
			},
		},
	})
}

// === Stream Management ===

// PurgeTranscodeStream ลบ messages ทั้งหมดใน TRANSCODE_JOBS stream
// DELETE /api/v1/admin/queues/transcode/purge
func (h *QueueHandler) PurgeTranscodeStream(c *fiber.Ctx) error {
	ctx := c.UserContext()

	if h.natsPublisher == nil {
		return utils.BadRequestResponse(c, "NATS not connected")
	}

	logger.InfoContext(ctx, "Purge transcode stream request")

	if err := h.natsPublisher.PurgeStream(ctx); err != nil {
		logger.ErrorContext(ctx, "Failed to purge transcode stream", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, fiber.Map{
		"message": "Transcode stream purged successfully",
	})
}

// === Batch Subtitle Actions ===

// GetSubtitleStats ดึงสถิติ subtitle แยกตาม step + filter by category
// GET /api/v1/admin/queues/subtitle/stats?category=xxx
func (h *QueueHandler) GetSubtitleStats(c *fiber.Ctx) error {
	ctx := c.UserContext()
	categoryID := c.Query("category", "")

	stats, err := h.queueService.GetSubtitleStats(ctx, categoryID)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get subtitle stats", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, stats)
}

// BatchDetectAll detect language ให้ video ที่ยังไม่ detect
// POST /api/v1/admin/queues/subtitle/detect-all?category=xxx&limit=50
func (h *QueueHandler) BatchDetectAll(c *fiber.Ctx) error {
	ctx := c.UserContext()
	categoryID := c.Query("category", "")
	limit, _ := strconv.Atoi(c.Query("limit", "50"))

	result, err := h.queueService.BatchDetectAll(ctx, categoryID, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Batch detect failed", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, result)
}

// BatchTranscribeAll transcribe ให้ video ที่ detect แล้วแต่ยังไม่มี SRT
// POST /api/v1/admin/queues/subtitle/transcribe-all?category=xxx&limit=50
func (h *QueueHandler) BatchTranscribeAll(c *fiber.Ctx) error {
	ctx := c.UserContext()
	categoryID := c.Query("category", "")
	limit, _ := strconv.Atoi(c.Query("limit", "50"))

	result, err := h.queueService.BatchTranscribeAll(ctx, categoryID, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Batch transcribe failed", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, result)
}

// BatchTranslateAll translate ให้ video ที่มี SRT แล้วแต่ยังไม่แปล
// POST /api/v1/admin/queues/subtitle/translate-all?category=xxx&limit=50&target=th
func (h *QueueHandler) BatchTranslateAll(c *fiber.Ctx) error {
	ctx := c.UserContext()
	categoryID := c.Query("category", "")
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	targetLang := c.Query("target", "th")

	result, err := h.queueService.BatchTranslateAll(ctx, categoryID, targetLang, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Batch translate failed", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, result)
}
