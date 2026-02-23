package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
	"gofiber-template/domain/services"
	natspkg "gofiber-template/infrastructure/nats"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/progress"
	"gofiber-template/pkg/utils"
)

type VideoHandler struct {
	videoService       services.VideoService
	transcodingService services.TranscodingService
	settingService     services.SettingService
	natsPublisher      *natspkg.Publisher // NATS JetStream publisher (ใช้แทน asynqClient เมื่อ STORAGE_TYPE=s3)
	storagePath        string
	storageType        string // "local" หรือ "s3"
}

func NewVideoHandler(
	videoService services.VideoService,
	transcodingService services.TranscodingService,
	settingService services.SettingService,
	natsPublisher *natspkg.Publisher,
	storagePath string,
	storageType string,
) *VideoHandler {
	return &VideoHandler{
		videoService:       videoService,
		transcodingService: transcodingService,
		settingService:     settingService,
		natsPublisher:      natsPublisher,
		storagePath:        storagePath,
		storageType:        storageType,
	}
}

// getDefaultQualities ดึงค่า default qualities จาก Settings
func (h *VideoHandler) getDefaultQualities(ctx context.Context) []string {
	defaultQualities := []string{"1080p", "720p", "480p"}

	if h.settingService == nil {
		logger.WarnContext(ctx, "SettingService is nil, using default qualities", "qualities", defaultQualities)
		return defaultQualities
	}

	qualitiesStr, err := h.settingService.Get(ctx, "transcoding", "default_qualities")
	if err != nil || qualitiesStr == "" {
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
		return defaultQualities
	}

	return qualities
}

// isAutoQueueEnabled ตรวจสอบว่าเปิด auto-queue หรือไม่
func (h *VideoHandler) isAutoQueueEnabled(ctx context.Context) bool {
	if h.settingService == nil {
		return true // default: เปิด
	}

	autoQueueStr, err := h.settingService.Get(ctx, "transcoding", "auto_queue")
	if err != nil || autoQueueStr == "" {
		return true // default: เปิด
	}

	return autoQueueStr == "true"
}

// Upload อัปโหลดวิดีโอใหม่
func (h *VideoHandler) Upload(c *fiber.Ctx) error {
	ctx := c.UserContext()

	user, err := utils.GetUserFromContext(c)
	if err != nil {
		logger.WarnContext(ctx, "Unauthorized access attempt")
		return utils.UnauthorizedResponse(c, "")
	}

	// รับไฟล์วิดีโอ
	file, err := c.FormFile("video")
	if err != nil {
		logger.WarnContext(ctx, "No video file provided", "error", err)
		return utils.BadRequestResponse(c, "No video file provided")
	}

	if file.Size == 0 {
		logger.WarnContext(ctx, "Empty file not allowed", "filename", file.Filename)
		return utils.BadRequestResponse(c, "Empty file not allowed")
	}

	// ตรวจสอบ disk space ก่อน upload (ต้องการพื้นที่ประมาณ 3x ของไฟล์สำหรับ transcoding)
	requiredSpace := file.Size * 3
	hasSpace, diskInfo, err := utils.CheckDiskSpace(h.storagePath, requiredSpace, 10.0)
	if err != nil {
		logger.WarnContext(ctx, "Failed to check disk space", "error", err)
		// ไม่ block upload ถ้าตรวจสอบไม่ได้
	} else if !hasSpace {
		logger.WarnContext(ctx, "Insufficient disk space",
			"required", utils.FormatBytes(uint64(requiredSpace)),
			"available", utils.FormatBytes(diskInfo.Free),
		)
		return utils.BadRequestResponse(c, "Insufficient disk space for video processing")
	}

	// Parse request
	var categoryID *uuid.UUID
	if catID := c.FormValue("category_id"); catID != "" {
		parsed, err := uuid.Parse(catID)
		if err != nil {
			return utils.BadRequestResponse(c, "Invalid category ID")
		}
		categoryID = &parsed
	}

	req := &dto.CreateVideoRequest{
		Title:       c.FormValue("title"),
		Description: c.FormValue("description"),
		CategoryID:  categoryID,
	}

	if err := utils.ValidateStruct(req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	logger.InfoContext(ctx, "Video upload attempt", "user_id", user.ID, "filename", file.Filename, "title", req.Title)

	// Get progress tracker
	tracker := progress.GetTracker()

	// Generate temporary video ID for tracking before actual upload
	tempVideoID := uuid.New()
	tempVideoCode := fmt.Sprintf("uploading-%s", tempVideoID.String()[:8])

	// Start tracking upload progress (use title from request)
	tracker.StartUpload(user.ID, tempVideoID, tempVideoCode, req.Title)
	tracker.UpdateUploadProgress(user.ID, tempVideoID, 10, "กำลังเตรียมอัพโหลด")

	video, err := h.videoService.Upload(ctx, user.ID, file, req)
	if err != nil {
		logger.WarnContext(ctx, "Video upload failed", "user_id", user.ID, "error", err)
		tracker.FailProgress(user.ID, tempVideoID, err.Error())
		return utils.BadRequestResponse(c, err.Error())
	}

	// Update progress with actual video info and complete
	tracker.UpdateUploadProgress(user.ID, tempVideoID, 90, fmt.Sprintf("อัพโหลดแล้ว: %s", video.Code))
	tracker.CompleteUpload(user.ID, tempVideoID)

	logger.InfoContext(ctx, "Video uploaded", "video_id", video.ID, "code", video.Code, "user_id", user.ID)

	// Auto-queue: ส่ง video เข้า transcoding queue อัตโนมัติ (ถ้าเปิดใช้งาน)
	autoEnqueued := false
	if h.isAutoQueueEnabled(ctx) && h.natsPublisher != nil {
		inputPath := video.OriginalPath
		outputPath := "hls/" + video.Code + "/"
		qualities := h.getDefaultQualities(ctx)

		if err := h.natsPublisher.EnqueueTranscode(ctx, video.ID.String(), video.Code, inputPath, outputPath, "h264", qualities, false); err != nil {
			logger.WarnContext(ctx, "Auto-queue failed, video remains pending",
				"video_id", video.ID,
				"video_code", video.Code,
				"error", err,
			)
		} else {
			// Update status to queued
			if updateErr := h.videoService.UpdateVideoStatus(ctx, video.ID, models.VideoStatusQueued); updateErr != nil {
				logger.WarnContext(ctx, "Failed to update video status to queued",
					"video_id", video.ID,
					"error", updateErr,
				)
			} else {
				autoEnqueued = true
				video.Status = models.VideoStatusQueued
				logger.InfoContext(ctx, "Video auto-queued for transcoding",
					"video_id", video.ID,
					"video_code", video.Code,
					"qualities", qualities,
				)
			}
		}
	} else if !h.isAutoQueueEnabled(ctx) {
		logger.InfoContext(ctx, "Auto-queue disabled, video remains pending",
			"video_id", video.ID,
			"video_code", video.Code,
		)
	} else {
		logger.WarnContext(ctx, "NATS publisher not available, video remains pending",
			"video_id", video.ID,
			"video_code", video.Code,
		)
	}

	return utils.CreatedResponse(c, dto.VideoUploadResponse{
		ID:           video.ID,
		Code:         video.Code,
		Title:        video.Title,
		Status:       string(video.Status),
		AutoEnqueued: autoEnqueued,
	})
}

// GetByCode ดึง video ตาม code (สำหรับ embed)
func (h *VideoHandler) GetByCode(c *fiber.Ctx) error {
	ctx := c.UserContext()
	code := c.Params("code")

	if code == "" {
		return utils.BadRequestResponse(c, "Video code is required")
	}

	video, err := h.videoService.GetByCode(ctx, code)
	if err != nil {
		logger.WarnContext(ctx, "Video not found", "code", code)
		return utils.NotFoundResponse(c, "Video not found")
	}

	// Increment views
	go h.videoService.IncrementViews(ctx, video.ID)

	return utils.SuccessResponse(c, dto.VideoToVideoResponse(video))
}

// GetByID ดึง video ตาม ID
func (h *VideoHandler) GetByID(c *fiber.Ctx) error {
	ctx := c.UserContext()
	idParam := c.Params("id")

	id, err := uuid.Parse(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid video ID")
	}

	video, err := h.videoService.GetByID(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "Video not found", "video_id", id)
		return utils.NotFoundResponse(c, "Video not found")
	}

	return utils.SuccessResponse(c, dto.VideoToVideoResponse(video))
}

// List ดึง videos ทั้งหมด (รองรับ search, filter, sort)
func (h *VideoHandler) List(c *fiber.Ctx) error {
	ctx := c.UserContext()

	// Parse query parameters
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// Build filter params
	params := &dto.VideoFilterRequest{
		Search:     c.Query("search"),
		Status:     c.Query("status"),
		CategoryID: c.Query("categoryId"),
		UserID:     c.Query("userId"),
		DateFrom:   c.Query("dateFrom"),
		DateTo:     c.Query("dateTo"),
		SortBy:     c.Query("sortBy", "created_at"),
		SortOrder:  c.Query("sortOrder", "desc"),
		Page:       page,
		Limit:      limit,
	}

	// Validate status if provided
	if params.Status != "" {
		status := models.VideoStatus(params.Status)
		if status != models.VideoStatusPending &&
			status != models.VideoStatusQueued &&
			status != models.VideoStatusProcessing &&
			status != models.VideoStatusReady &&
			status != models.VideoStatusFailed &&
			status != models.VideoStatusDeadLetter {
			return utils.BadRequestResponse(c, "Invalid status filter. Valid values: pending, queued, processing, ready, failed, dead_letter")
		}
	}

	videos, total, err := h.videoService.ListWithFilters(ctx, params)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to list videos", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	// Get reel counts for all videos (batch query)
	reelCounts, _ := h.videoService.GetReelCountsForVideos(ctx, videos)

	return utils.PaginatedSuccessResponse(c, dto.VideosToVideoResponsesWithReelCounts(videos, reelCounts), total, page, limit)
}

// ListReady ดึงเฉพาะ videos ที่พร้อม stream
func (h *VideoHandler) ListReady(c *fiber.Ctx) error {
	ctx := c.UserContext()

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	videos, total, err := h.videoService.ListReadyVideos(ctx, page, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to list ready videos", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.PaginatedSuccessResponse(c, dto.VideosToVideoResponses(videos), total, page, limit)
}

// GetMyVideos ดึง videos ของ user ที่ login
func (h *VideoHandler) GetMyVideos(c *fiber.Ctx) error {
	ctx := c.UserContext()

	user, err := utils.GetUserFromContext(c)
	if err != nil {
		return utils.UnauthorizedResponse(c, "")
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	videos, total, err := h.videoService.GetUserVideos(ctx, user.ID, page, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get user videos", "user_id", user.ID, "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.PaginatedSuccessResponse(c, dto.VideosToVideoResponses(videos), total, page, limit)
}

// Update อัปเดต video metadata
func (h *VideoHandler) Update(c *fiber.Ctx) error {
	ctx := c.UserContext()
	idParam := c.Params("id")

	id, err := uuid.Parse(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid video ID")
	}

	var req dto.UpdateVideoRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	video, err := h.videoService.Update(ctx, id, &req)
	if err != nil {
		logger.WarnContext(ctx, "Video update failed", "video_id", id, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Video updated", "video_id", id)
	return utils.SuccessResponse(c, dto.VideoToVideoResponse(video))
}

// Delete ลบ video
func (h *VideoHandler) Delete(c *fiber.Ctx) error {
	ctx := c.UserContext()
	idParam := c.Params("id")

	id, err := uuid.Parse(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid video ID")
	}

	if err := h.videoService.Delete(ctx, id); err != nil {
		logger.WarnContext(ctx, "Video delete failed", "video_id", id, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Video deleted", "video_id", id)
	return utils.SuccessResponse(c, fiber.Map{"message": "Video deleted successfully"})
}

// GetStats ดึง video statistics
func (h *VideoHandler) GetStats(c *fiber.Ctx) error {
	ctx := c.UserContext()

	stats, err := h.videoService.GetStats(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get video stats", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, stats)
}

// GetEmbed ดึงข้อมูลสำหรับ embed player
func (h *VideoHandler) GetEmbed(c *fiber.Ctx) error {
	ctx := c.UserContext()
	code := c.Params("code")

	if code == "" {
		return utils.BadRequestResponse(c, "Video code is required")
	}

	video, err := h.videoService.GetByCode(ctx, code)
	if err != nil {
		return utils.NotFoundResponse(c, "Video not found")
	}

	// ตรวจสอบว่า video พร้อมหรือยัง
	if !video.IsReady() {
		return utils.BadRequestResponse(c, "Video is not ready")
	}

	// Increment views
	go h.videoService.IncrementViews(ctx, video.ID)

	return utils.SuccessResponse(c, dto.VideoToEmbedResponse(video))
}

// ═══════════════════════════════════════════════════════════════════════════════
// Dead Letter Queue (DLQ) Management
// ═══════════════════════════════════════════════════════════════════════════════

// ListDLQ ดึง videos ที่อยู่ใน Dead Letter Queue พร้อม error info
func (h *VideoHandler) ListDLQ(c *fiber.Ctx) error {
	ctx := c.UserContext()

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	videos, total, err := h.videoService.ListVideosByStatus(ctx, models.VideoStatusDeadLetter, page, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to list DLQ videos", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	// Convert to DLQ response with error details
	dlqResponses := make([]dto.DLQVideoResponse, 0, len(videos))
	for _, v := range videos {
		// Convert error history
		errorHistory := make([]dto.ErrorRecordResponse, 0, len(v.ErrorHistory))
		for _, record := range v.ErrorHistory {
			errorHistory = append(errorHistory, dto.ErrorRecordResponse{
				Attempt:   record.Attempt,
				Error:     record.Error,
				WorkerID:  record.WorkerID,
				Stage:     record.Stage,
				Timestamp: record.Timestamp,
			})
		}

		dlqResponses = append(dlqResponses, dto.DLQVideoResponse{
			ID:           v.ID,
			Code:         v.Code,
			Title:        v.Title,
			RetryCount:   v.RetryCount,
			LastError:    v.LastError,
			ErrorHistory: errorHistory,
			CreatedAt:    v.CreatedAt,
			UpdatedAt:    v.UpdatedAt,
			UserID:       v.UserID,
		})
	}

	return utils.PaginatedSuccessResponse(c, dlqResponses, total, page, limit)
}

// RetryDLQ retry video จาก DLQ (reset retry_count และ re-queue)
func (h *VideoHandler) RetryDLQ(c *fiber.Ctx) error {
	ctx := c.UserContext()
	idParam := c.Params("id")

	id, err := uuid.Parse(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid video ID")
	}

	video, err := h.videoService.GetByID(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "Video not found for DLQ retry", "video_id", id)
		return utils.NotFoundResponse(c, "Video not found")
	}

	// ตรวจสอบว่าเป็น dead_letter หรือ failed เท่านั้นที่ retry ได้
	if video.Status != models.VideoStatusDeadLetter && video.Status != models.VideoStatusFailed {
		return utils.BadRequestResponse(c, "Only dead_letter or failed videos can be retried")
	}

	// Reset retry count และ error
	if err := h.videoService.ResetVideoForRetry(ctx, id); err != nil {
		logger.ErrorContext(ctx, "Failed to reset video for retry", "video_id", id, "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	// Re-queue for transcoding
	if h.natsPublisher != nil {
		inputPath := video.OriginalPath
		outputPath := "hls/" + video.Code + "/"
		qualities := h.getDefaultQualities(ctx)

		if err := h.natsPublisher.EnqueueTranscode(ctx, video.ID.String(), video.Code, inputPath, outputPath, "h264", qualities, false); err != nil {
			logger.ErrorContext(ctx, "Failed to re-queue video from DLQ", "video_id", id, "error", err)
			return utils.BadRequestResponse(c, "Failed to queue video for transcoding")
		}

		// Update status to queued
		if err := h.videoService.UpdateVideoStatus(ctx, video.ID, models.VideoStatusQueued); err != nil {
			logger.WarnContext(ctx, "Failed to update video status to queued", "video_id", id, "error", err)
		}

		logger.InfoContext(ctx, "Video retried from DLQ",
			"video_id", id,
			"video_code", video.Code,
			"previous_status", video.Status,
			"previous_retry_count", video.RetryCount,
		)
	}

	return utils.SuccessResponse(c, fiber.Map{
		"message":  "Video queued for retry",
		"video_id": video.ID,
		"code":     video.Code,
	})
}

// DeleteDLQ ลบ video จาก DLQ (พร้อมลบไฟล์ใน storage)
func (h *VideoHandler) DeleteDLQ(c *fiber.Ctx) error {
	ctx := c.UserContext()
	idParam := c.Params("id")

	id, err := uuid.Parse(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid video ID")
	}

	video, err := h.videoService.GetByID(ctx, id)
	if err != nil {
		return utils.NotFoundResponse(c, "Video not found")
	}

	// ตรวจสอบว่าเป็น dead_letter เท่านั้น
	if video.Status != models.VideoStatusDeadLetter {
		return utils.BadRequestResponse(c, "Only dead_letter videos can be deleted from DLQ")
	}

	if err := h.videoService.Delete(ctx, id); err != nil {
		logger.ErrorContext(ctx, "Failed to delete DLQ video", "video_id", id, "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	logger.InfoContext(ctx, "Video deleted from DLQ", "video_id", id, "video_code", video.Code)

	return utils.SuccessResponse(c, fiber.Map{
		"message":  "Video deleted from DLQ",
		"video_id": id,
	})
}

// BatchUpload อัปโหลดหลายไฟล์พร้อมกัน (สูงสุด 10 ไฟล์)
func (h *VideoHandler) BatchUpload(c *fiber.Ctx) error {
	ctx := c.UserContext()

	user, err := utils.GetUserFromContext(c)
	if err != nil {
		logger.WarnContext(ctx, "Unauthorized access attempt")
		return utils.UnauthorizedResponse(c, "")
	}

	// รับไฟล์วิดีโอหลายไฟล์
	form, err := c.MultipartForm()
	if err != nil {
		logger.WarnContext(ctx, "Failed to parse multipart form", "error", err)
		return utils.BadRequestResponse(c, "Invalid form data")
	}

	files := form.File["videos"]
	if len(files) == 0 {
		return utils.BadRequestResponse(c, "No video files provided")
	}

	if len(files) > 10 {
		return utils.BadRequestResponse(c, "Maximum 10 files allowed per batch")
	}

	logger.InfoContext(ctx, "Batch upload attempt", "user_id", user.ID, "file_count", len(files))

	// ประมวลผลทุกไฟล์
	type uploadResult struct {
		Filename string      `json:"filename"`
		Success  bool        `json:"success"`
		Video    interface{} `json:"video,omitempty"`
		Error    string      `json:"error,omitempty"`
	}

	results := make([]uploadResult, 0, len(files))
	successCount := 0
	errorCount := 0

	// เก็บ videos ที่ upload สำเร็จเพื่อ queue ทีหลัง
	var uploadedVideos []*models.Video

	// ====== PHASE 1: Upload ทุกไฟล์ไป MinIO ก่อน ======
	logger.InfoContext(ctx, "PHASE 1: Uploading all files to MinIO", "total_files", len(files))

	for i, file := range files {
		result := uploadResult{Filename: file.Filename}
		fileIndex := i + 1

		// Log: เริ่มประมวลผลไฟล์
		logger.InfoContext(ctx, "Processing file",
			"index", fileIndex,
			"total", len(files),
			"filename", file.Filename,
			"size_bytes", file.Size,
			"size_mb", float64(file.Size)/(1024*1024),
		)

		// ตรวจสอบว่าไฟล์ว่างเปล่าหรือไม่
		if file.Size == 0 {
			logger.WarnContext(ctx, "Empty file skipped", "index", fileIndex, "filename", file.Filename)
			result.Error = "Empty file"
			errorCount++
			results = append(results, result)
			continue
		}

		// ใช้ชื่อไฟล์เป็น title (ตัด extension)
		title := file.Filename
		if dotIdx := len(title) - 1; dotIdx > 0 {
			for i := len(title) - 1; i >= 0; i-- {
				if title[i] == '.' {
					title = title[:i]
					break
				}
			}
		}

		req := &dto.CreateVideoRequest{
			Title: title,
		}

		// Log: เริ่ม upload ไป MinIO
		logger.InfoContext(ctx, "Uploading to MinIO",
			"index", fileIndex,
			"filename", file.Filename,
			"title", title,
		)

		video, err := h.videoService.Upload(ctx, user.ID, file, req)
		if err != nil {
			// Log: Upload ล้มเหลว
			logger.ErrorContext(ctx, "Upload to MinIO FAILED",
				"index", fileIndex,
				"filename", file.Filename,
				"error", err,
			)
			result.Error = err.Error()
			errorCount++
			results = append(results, result)
			continue
		}

		// Log: Upload สำเร็จ - ไฟล์ไปถึง MinIO แล้ว
		logger.InfoContext(ctx, "Upload to MinIO SUCCESS",
			"index", fileIndex,
			"filename", file.Filename,
			"video_id", video.ID,
			"video_code", video.Code,
			"original_path", video.OriginalPath,
		)

		// เก็บ video ไว้ queue ทีหลัง
		uploadedVideos = append(uploadedVideos, video)

		result.Success = true
		result.Video = dto.VideoUploadResponse{
			ID:           video.ID,
			Code:         video.Code,
			Title:        video.Title,
			Status:       string(video.Status),
			AutoEnqueued: true,
		}
		successCount++
		results = append(results, result)
	}

	logger.InfoContext(ctx, "PHASE 1 COMPLETE: All files uploaded to MinIO",
		"total", len(files),
		"success", successCount,
		"errors", errorCount,
	)

	// ====== PHASE 2: Auto-queue videos for transcoding (ถ้าเปิดใช้งาน) ======
	queuedCount := 0
	if h.isAutoQueueEnabled(ctx) && h.natsPublisher != nil && len(uploadedVideos) > 0 {
		qualities := h.getDefaultQualities(ctx)

		for _, video := range uploadedVideos {
			inputPath := video.OriginalPath
			outputPath := "hls/" + video.Code + "/"

			if err := h.natsPublisher.EnqueueTranscode(ctx, video.ID.String(), video.Code, inputPath, outputPath, "h264", qualities, false); err != nil {
				logger.WarnContext(ctx, "Failed to queue video",
					"video_id", video.ID,
					"video_code", video.Code,
					"error", err,
				)
				continue
			}

			// Update status to queued
			if updateErr := h.videoService.UpdateVideoStatus(ctx, video.ID, models.VideoStatusQueued); updateErr != nil {
				logger.WarnContext(ctx, "Failed to update video status",
					"video_id", video.ID,
					"error", updateErr,
				)
			} else {
				queuedCount++
			}
		}

		logger.InfoContext(ctx, "PHASE 2 COMPLETE: Videos queued for transcoding",
			"queued", queuedCount,
			"total_uploaded", len(uploadedVideos),
		)
	} else if !h.isAutoQueueEnabled(ctx) {
		logger.InfoContext(ctx, "PHASE 2 SKIPPED: Auto-queue disabled",
			"pending_count", len(uploadedVideos),
		)
	} else {
		logger.WarnContext(ctx, "PHASE 2 SKIPPED: NATS publisher not available",
			"pending_count", len(uploadedVideos),
		)
	}

	logger.InfoContext(ctx, "Batch upload completed",
		"user_id", user.ID,
		"total", len(files),
		"success", successCount,
		"errors", errorCount,
	)

	return utils.SuccessResponse(c, fiber.Map{
		"total":   len(files),
		"success": successCount,
		"errors":  errorCount,
		"results": results,
	})
}

// ═══════════════════════════════════════════════════════════════════════════════
// Gallery Generation
// ═══════════════════════════════════════════════════════════════════════════════

// GenerateGallery สร้าง gallery images จาก HLS ที่มีอยู่แล้ว
func (h *VideoHandler) GenerateGallery(c *fiber.Ctx) error {
	ctx := c.UserContext()
	idParam := c.Params("id")

	id, err := uuid.Parse(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid video ID")
	}

	video, err := h.videoService.GetByID(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "Video not found for gallery generation", "video_id", id)
		return utils.NotFoundResponse(c, "Video not found")
	}

	// ตรวจสอบว่า video ready แล้ว
	if video.Status != models.VideoStatusReady {
		return utils.BadRequestResponse(c, "Video must be ready before generating gallery")
	}

	// ตรวจสอบว่ามี HLS path
	if video.HLSPath == "" {
		return utils.BadRequestResponse(c, "Video has no HLS content")
	}

	// ตรวจสอบว่ามี gallery แล้วหรือยัง
	if video.GalleryCount > 0 {
		return utils.BadRequestResponse(c, "Gallery already exists for this video")
	}

	// หา best quality ที่มี
	bestQuality := h.getBestAvailableQuality(video)
	if bestQuality == "" {
		return utils.BadRequestResponse(c, "No quality available for gallery generation")
	}

	// สร้าง gallery job
	if h.natsPublisher == nil {
		return utils.BadRequestResponse(c, "NATS publisher not available")
	}

	hlsPath := fmt.Sprintf("hls/%s/%s/playlist.m3u8", video.Code, bestQuality)
	outputPath := fmt.Sprintf("gallery/%s/", video.Code)

	job := natspkg.NewGalleryJob(
		video.ID.String(),
		video.Code,
		hlsPath,
		bestQuality,
		video.Duration,
		outputPath,
		100, // default 100 images
	)

	if err := h.natsPublisher.PublishGalleryJob(ctx, job); err != nil {
		logger.ErrorContext(ctx, "Failed to publish gallery job",
			"video_id", id,
			"video_code", video.Code,
			"error", err,
		)
		return utils.BadRequestResponse(c, "Failed to queue gallery generation")
	}

	logger.InfoContext(ctx, "Gallery job published",
		"video_id", id,
		"video_code", video.Code,
		"quality", bestQuality,
		"duration", video.Duration,
	)

	return utils.SuccessResponse(c, fiber.Map{
		"message":    "Gallery generation queued",
		"video_id":   video.ID,
		"video_code": video.Code,
		"quality":    bestQuality,
	})
}

// RegenerateGallery สร้าง gallery ใหม่ (ลบของเก่าแล้วสร้างใหม่)
func (h *VideoHandler) RegenerateGallery(c *fiber.Ctx) error {
	ctx := c.UserContext()
	idParam := c.Params("id")

	id, err := uuid.Parse(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid video ID")
	}

	video, err := h.videoService.GetByID(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "Video not found for gallery regeneration", "video_id", id)
		return utils.NotFoundResponse(c, "Video not found")
	}

	// ตรวจสอบว่า video ready แล้ว
	if video.Status != models.VideoStatusReady {
		return utils.BadRequestResponse(c, "Video must be ready before regenerating gallery")
	}

	// ตรวจสอบว่ามี HLS path
	if video.HLSPath == "" {
		return utils.BadRequestResponse(c, "Video has no HLS content")
	}

	// หา best quality ที่มี
	bestQuality := h.getBestAvailableQuality(video)
	if bestQuality == "" {
		return utils.BadRequestResponse(c, "No quality available for gallery generation")
	}

	// สร้าง gallery job
	if h.natsPublisher == nil {
		return utils.BadRequestResponse(c, "NATS publisher not available")
	}

	// Reset gallery counts ก่อน (worker จะ update ใหม่เมื่อเสร็จ)
	zero := 0
	emptyPath := ""
	resetReq := &dto.UpdateVideoRequest{
		GalleryPath:      &emptyPath,
		GalleryCount:     &zero,
		GallerySafeCount: &zero,
		GalleryNsfwCount: &zero,
	}
	if _, err := h.videoService.Update(ctx, id, resetReq); err != nil {
		logger.WarnContext(ctx, "Failed to reset gallery counts", "video_id", id, "error", err)
		// Continue anyway - worker will overwrite
	}

	hlsPath := fmt.Sprintf("hls/%s/%s/playlist.m3u8", video.Code, bestQuality)
	outputPath := fmt.Sprintf("gallery/%s/", video.Code)

	job := natspkg.NewGalleryJob(
		video.ID.String(),
		video.Code,
		hlsPath,
		bestQuality,
		video.Duration,
		outputPath,
		100, // default 100 images
	)

	if err := h.natsPublisher.PublishGalleryJob(ctx, job); err != nil {
		logger.ErrorContext(ctx, "Failed to publish gallery regeneration job",
			"video_id", id,
			"video_code", video.Code,
			"error", err,
		)
		return utils.BadRequestResponse(c, "Failed to queue gallery regeneration")
	}

	logger.InfoContext(ctx, "Gallery regeneration job published",
		"video_id", id,
		"video_code", video.Code,
		"quality", bestQuality,
		"duration", video.Duration,
	)

	return utils.SuccessResponse(c, fiber.Map{
		"message":    "Gallery regeneration queued",
		"video_id":   video.ID,
		"video_code": video.Code,
		"quality":    bestQuality,
	})
}

// getBestAvailableQuality หา quality สูงสุดที่มี
func (h *VideoHandler) getBestAvailableQuality(video *models.Video) string {
	// ลำดับความสำคัญ: 1080p > 720p > 480p > 360p
	qualityOrder := []string{"1080p", "720p", "480p", "360p"}

	if video.QualitySizes == nil {
		// ถ้าไม่มี quality sizes ให้ใช้ค่าจาก video.Quality
		if video.Quality != "" {
			return video.Quality
		}
		return "720p" // default
	}

	for _, q := range qualityOrder {
		if _, exists := video.QualitySizes[q]; exists {
			return q
		}
	}

	// ถ้าไม่เจอ ให้ใช้ตัวแรกที่มี
	for q := range video.QualitySizes {
		return q
	}

	return ""
}

// ═══════════════════════════════════════════════════════════════════════════════
// Internal API - Worker Callbacks
// ═══════════════════════════════════════════════════════════════════════════════

// UpdateGalleryRequest request body for updating gallery
type UpdateGalleryRequest struct {
	GalleryPath      string `json:"gallery_path"`
	GalleryCount     int    `json:"gallery_count"`      // Total count (safe + nsfw)
	GallerySafeCount int    `json:"gallery_safe_count"` // Safe images count
	GalleryNsfwCount int    `json:"gallery_nsfw_count"` // NSFW images count
}

// UpdateGallery updates video gallery info (called by worker after gallery generation)
func (h *VideoHandler) UpdateGallery(c *fiber.Ctx) error {
	ctx := c.UserContext()
	idParam := c.Params("id")

	id, err := uuid.Parse(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid video ID")
	}

	var req UpdateGalleryRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	// Calculate total if not provided
	galleryCount := req.GalleryCount
	if galleryCount == 0 && (req.GallerySafeCount > 0 || req.GalleryNsfwCount > 0) {
		galleryCount = req.GallerySafeCount + req.GalleryNsfwCount
	}

	// Update gallery fields via service
	updateReq := &dto.UpdateVideoRequest{
		GalleryPath:      &req.GalleryPath,
		GalleryCount:     &galleryCount,
		GallerySafeCount: &req.GallerySafeCount,
		GalleryNsfwCount: &req.GalleryNsfwCount,
	}

	video, err := h.videoService.Update(ctx, id, updateReq)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to update video gallery",
			"video_id", id,
			"error", err,
		)
		return utils.InternalServerErrorResponse(c)
	}

	logger.InfoContext(ctx, "Gallery updated",
		"video_id", id,
		"video_code", video.Code,
		"gallery_path", req.GalleryPath,
		"gallery_count", galleryCount,
		"safe_count", req.GallerySafeCount,
		"nsfw_count", req.GalleryNsfwCount,
	)

	return utils.SuccessResponse(c, fiber.Map{
		"message":       "Gallery updated",
		"video_id":      video.ID,
		"video_code":    video.Code,
		"gallery_count": galleryCount,
		"safe_count":    req.GallerySafeCount,
		"nsfw_count":    req.GalleryNsfwCount,
	})
}
