package handlers

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"gofiber-template/domain/models"
	"gofiber-template/domain/services"
	natspkg "gofiber-template/infrastructure/nats"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/utils"
)

type TranscodingHandler struct {
	videoService   services.VideoService
	settingService services.SettingService
	natsPublisher  *natspkg.Publisher
}

func NewTranscodingHandler(videoService services.VideoService, settingService services.SettingService, natsPublisher *natspkg.Publisher) *TranscodingHandler {
	return &TranscodingHandler{
		videoService:   videoService,
		settingService: settingService,
		natsPublisher:  natsPublisher,
	}
}

// getDefaultQualities ดึงค่า default qualities จาก Settings
func (h *TranscodingHandler) getDefaultQualities(ctx context.Context) []string {
	defaultQualities := []string{"1080p", "720p", "480p"}

	if h.settingService == nil {
		logger.WarnContext(ctx, "SettingService is nil, using default qualities", "qualities", defaultQualities)
		return defaultQualities
	}

	qualitiesStr, err := h.settingService.Get(ctx, "transcoding", "default_qualities")
	if err != nil || qualitiesStr == "" {
		logger.WarnContext(ctx, "No qualities in settings, using defaults", "qualities", defaultQualities)
		return defaultQualities
	}

	// แยก comma-separated string เป็น slice
	parts := strings.Split(qualitiesStr, ",")
	var qualities []string
	for _, p := range parts {
		q := strings.TrimSpace(p)
		if q != "" {
			qualities = append(qualities, q)
		}
	}

	if len(qualities) == 0 {
		logger.WarnContext(ctx, "Parsed qualities empty, using defaults", "qualities", defaultQualities)
		return defaultQualities
	}

	logger.InfoContext(ctx, "Using transcoding qualities from settings", "qualities", qualities)
	return qualities
}

// QueueVideo ส่งวิดีโอเข้า transcoding queue via NATS JetStream
func (h *TranscodingHandler) QueueVideo(c *fiber.Ctx) error {
	ctx := c.UserContext()

	// ตรวจสอบว่า NATS publisher พร้อมใช้งาน
	if h.natsPublisher == nil {
		logger.WarnContext(ctx, "NATS publisher not available")
		return utils.BadRequestResponse(c, "Transcoding service is not available. NATS connection failed.")
	}

	idParam := c.Params("id")
	videoID, err := uuid.Parse(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid video ID")
	}

	// ดึงข้อมูล video
	video, err := h.videoService.GetByID(ctx, videoID)
	if err != nil {
		logger.WarnContext(ctx, "Video not found", "video_id", videoID)
		return utils.NotFoundResponse(c, "Video not found")
	}

	// ส่ง job เข้า NATS JetStream
	logger.InfoContext(ctx, "Queueing video for transcoding via NATS", "video_id", videoID, "video_code", video.Code)

	// ใช้ OriginalPath จาก database (รองรับทุก extension: .mp4, .mov, .avi, .mkv)
	inputPath := video.OriginalPath
	outputPath := "hls/" + video.Code + "/"
	qualities := h.getDefaultQualities(ctx)

	logger.InfoContext(ctx, "Queueing video with qualities", "video_id", videoID, "qualities", qualities)

	if err := h.natsPublisher.EnqueueTranscode(ctx, video.ID.String(), video.Code, inputPath, outputPath, "h264", qualities, false); err != nil {
		logger.ErrorContext(ctx, "Failed to queue video for transcoding", "video_id", videoID, "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	// Update status to queued หลังจาก publish สำเร็จ
	oldStatus := video.Status
	if updateErr := h.videoService.UpdateVideoStatus(ctx, video.ID, models.VideoStatusQueued); updateErr != nil {
		logger.WarnContext(ctx, "Failed to update video status to queued",
			"video_id", videoID,
			"video_code", video.Code,
			"error", updateErr,
		)
	} else {
		logger.InfoContext(ctx, "Video queued to NATS",
			"video_id", videoID,
			"video_code", video.Code,
			"old_status", oldStatus,
			"new_status", "queued",
		)
	}

	return utils.SuccessResponse(c, fiber.Map{
		"message":    "Video queued for transcoding",
		"video_id":   videoID,
		"video_code": video.Code,
	})
}

// GetQueueStatus ดึงสถานะของ transcoding queue
func (h *TranscodingHandler) GetQueueStatus(c *fiber.Ctx) error {
	ctx := c.UserContext()

	// ตรวจสอบว่า NATS publisher พร้อมใช้งาน
	if h.natsPublisher == nil {
		logger.WarnContext(ctx, "NATS publisher not available")
		return utils.SuccessResponse(c, fiber.Map{
			"available":      false,
			"queueSize":      0,
			"workersRunning": false,
			"message":        "NATS connection not available",
		})
	}

	// ดึง stats จาก VideoService
	stats, err := h.videoService.GetStats(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get video stats", "error", err)
		return utils.SuccessResponse(c, fiber.Map{
			"available":      true,
			"queueSize":      0,
			"workersRunning": true,
		})
	}

	return utils.SuccessResponse(c, fiber.Map{
		"available":      true,
		"queueSize":      stats.PendingVideos + stats.QueuedVideos + stats.ProcessingVideos,
		"workersRunning": true, // Worker runs in separate container
	})
}

// GetStats ดึงสถิติจำนวนวิดีโอตาม status
func (h *TranscodingHandler) GetStats(c *fiber.Ctx) error {
	ctx := c.UserContext()

	// Query จาก VideoService โดยตรง
	stats, err := h.videoService.GetStats(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get video stats", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, fiber.Map{
		"pending":    stats.PendingVideos,
		"queued":     stats.QueuedVideos,
		"processing": stats.ProcessingVideos,
		"completed":  stats.ReadyVideos,
		"failed":     stats.FailedVideos,
	})
}

// RequeueStuckVideos ส่งวิดีโอที่ค้างกลับเข้า queue ใหม่
func (h *TranscodingHandler) RequeueStuckVideos(c *fiber.Ctx) error {
	ctx := c.UserContext()

	// ตรวจสอบว่า NATS publisher พร้อมใช้งาน
	if h.natsPublisher == nil {
		logger.WarnContext(ctx, "NATS publisher not available")
		return utils.BadRequestResponse(c, "Transcoding service is not available. NATS connection failed.")
	}

	// ดึงวิดีโอที่ค้างมากกว่า 5 นาที
	stuckVideos, err := h.videoService.GetStuckVideos(ctx, 5)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get stuck videos", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	if len(stuckVideos) == 0 {
		return utils.SuccessResponse(c, fiber.Map{
			"message": "No stuck videos found",
			"count":   0,
		})
	}

	// Re-queue แต่ละวิดีโอ
	requeuedCount := 0
	failedCount := 0
	qualities := h.getDefaultQualities(ctx)
	for _, video := range stuckVideos {
		// ใช้ OriginalPath จาก database (รองรับทุก extension: .mp4, .mov, .avi, .mkv)
		inputPath := video.OriginalPath
		outputPath := "hls/" + video.Code + "/"

		err := h.natsPublisher.EnqueueTranscode(ctx, video.ID.String(), video.Code, inputPath, outputPath, "h264", qualities, false)
		if err != nil {
			logger.WarnContext(ctx, "Failed to requeue video", "video_id", video.ID, "error", err)
			failedCount++
		} else {
			requeuedCount++
			logger.InfoContext(ctx, "Video requeued", "video_id", video.ID, "video_code", video.Code)
		}
	}

	return utils.SuccessResponse(c, fiber.Map{
		"message":  "Stuck videos requeued",
		"requeued": requeuedCount,
		"failed":   failedCount,
		"total":    len(stuckVideos),
	})
}

// MarkStuckAsFailed มาร์ควิดีโอที่ค้างเป็น failed
func (h *TranscodingHandler) MarkStuckAsFailed(c *fiber.Ctx) error {
	ctx := c.UserContext()

	// ดึงวิดีโอที่ค้างมากกว่า 5 นาที
	stuckVideos, err := h.videoService.GetStuckVideos(ctx, 5)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get stuck videos", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	if len(stuckVideos) == 0 {
		return utils.SuccessResponse(c, fiber.Map{
			"message": "No stuck videos found",
			"count":   0,
		})
	}

	// Mark เป็น failed
	markedCount := 0
	for _, video := range stuckVideos {
		err := h.videoService.UpdateVideoStatus(ctx, video.ID, models.VideoStatusFailed)
		if err != nil {
			logger.WarnContext(ctx, "Failed to mark video as failed", "video_id", video.ID, "error", err)
		} else {
			markedCount++
			logger.InfoContext(ctx, "Video marked as failed", "video_id", video.ID, "video_code", video.Code)
		}
	}

	return utils.SuccessResponse(c, fiber.Map{
		"message": "Stuck videos marked as failed",
		"marked":  markedCount,
		"total":   len(stuckVideos),
	})
}

// QueuePendingVideos ส่งวิดีโอที่เป็น pending ทั้งหมดเข้า NATS queue
// POST /api/v1/transcoding/queue-pending
func (h *TranscodingHandler) QueuePendingVideos(c *fiber.Ctx) error {
	ctx := c.UserContext()

	// ตรวจสอบว่า NATS publisher พร้อมใช้งาน
	if h.natsPublisher == nil {
		logger.WarnContext(ctx, "NATS publisher not available")
		return utils.BadRequestResponse(c, "Transcoding service is not available. NATS connection failed.")
	}

	// ดึง videos ที่เป็น pending ทั้งหมด
	pendingVideos, total, err := h.videoService.ListVideosByStatus(ctx, models.VideoStatusPending, 1, 100)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get pending videos", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	if len(pendingVideos) == 0 {
		return utils.SuccessResponse(c, fiber.Map{
			"message": "No pending videos found",
			"queued":  0,
			"failed":  0,
			"total":   0,
		})
	}

	logger.InfoContext(ctx, "Queueing pending videos to NATS", "count", len(pendingVideos), "total_pending", total)

	// Queue แต่ละวิดีโอ
	queuedCount := 0
	failedCount := 0
	qualities := h.getDefaultQualities(ctx)
	for _, video := range pendingVideos {
		// ใช้ OriginalPath จาก database (รองรับทุก extension: .mp4, .mov, .avi, .mkv)
		inputPath := video.OriginalPath
		outputPath := "hls/" + video.Code + "/"

		err := h.natsPublisher.EnqueueTranscode(ctx, video.ID.String(), video.Code, inputPath, outputPath, "h264", qualities, false)
		if err != nil {
			logger.WarnContext(ctx, "Failed to queue video", "video_id", video.ID, "video_code", video.Code, "error", err)
			failedCount++
			continue
		}

		// Update status to queued
		if updateErr := h.videoService.UpdateVideoStatus(ctx, video.ID, models.VideoStatusQueued); updateErr != nil {
			logger.WarnContext(ctx, "Failed to update video status to queued",
				"video_id", video.ID,
				"video_code", video.Code,
				"error", updateErr,
			)
		} else {
			logger.InfoContext(ctx, "Video queued to NATS",
				"video_id", video.ID,
				"video_code", video.Code,
				"old_status", "pending",
				"new_status", "queued",
			)
		}
		queuedCount++
	}

	logger.InfoContext(ctx, "Queue pending videos completed",
		"queued", queuedCount,
		"failed", failedCount,
		"total", len(pendingVideos),
	)

	return utils.SuccessResponse(c, fiber.Map{
		"message": "Pending videos queued",
		"queued":  queuedCount,
		"failed":  failedCount,
		"total":   len(pendingVideos),
	})
}

// ClearAllVideos ลบ videos ทั้งหมด (สำหรับ testing เท่านั้น)
func (h *TranscodingHandler) ClearAllVideos(c *fiber.Ctx) error {
	ctx := c.UserContext()

	count, err := h.videoService.DeleteAll(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to delete all videos", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, fiber.Map{
		"message": "All videos deleted",
		"deleted": count,
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// Worker Management (Phase 1 - Monitor Only)
// ═══════════════════════════════════════════════════════════════════════════════

// GetWorkers ดึงรายการ Workers ทั้งหมด
func (h *TranscodingHandler) GetWorkers(c *fiber.Ctx) error {
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
