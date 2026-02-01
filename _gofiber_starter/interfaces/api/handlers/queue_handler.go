package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/utils"
)

type QueueHandler struct {
	queueService services.QueueService
}

func NewQueueHandler(queueService services.QueueService) *QueueHandler {
	return &QueueHandler{
		queueService: queueService,
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
