package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"gofiber-template/application/serviceimpl"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
	"gofiber-template/domain/ports"
	"gofiber-template/domain/services"
	natspkg "gofiber-template/infrastructure/nats"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/utils"
)

// DirectUploadHandler จัดการ Direct Upload ผ่าน Presigned URL
type DirectUploadHandler struct {
	storage         ports.StoragePort
	videoService    services.VideoService
	settingService  services.SettingService
	categoryService services.CategoryService
	natsPublisher   *natspkg.Publisher
}

// NewDirectUploadHandler สร้าง DirectUploadHandler
func NewDirectUploadHandler(
	storage ports.StoragePort,
	videoService services.VideoService,
	settingService services.SettingService,
	categoryService services.CategoryService,
	natsPublisher *natspkg.Publisher,
) *DirectUploadHandler {
	return &DirectUploadHandler{
		storage:         storage,
		videoService:    videoService,
		settingService:  settingService,
		categoryService: categoryService,
		natsPublisher:   natsPublisher,
	}
}

const (
	// PartSize ขนาดแต่ละ part (64MB)
	PartSize = 64 * 1024 * 1024
	// PresignedURLExpiry ระยะเวลาที่ presigned URL ใช้ได้ (2 ชั่วโมง)
	PresignedURLExpiry = 2 * time.Hour
	// DefaultMaxFileSize ขนาดไฟล์สูงสุดเริ่มต้น (10GB) - ใช้เมื่อดึงจาก settings ไม่ได้
	DefaultMaxFileSize = 10 * 1024 * 1024 * 1024
)

// allowedVideoTypes MIME types ที่อนุญาต
var allowedVideoTypes = map[string]bool{
	"video/mp4":             true,
	"video/x-matroska":      true, // mkv
	"video/x-msvideo":       true, // avi
	"video/quicktime":       true, // mov
	"video/webm":            true,
	"video/MP2T":            true, // ts (standard)
	"video/mp2t":            true, // ts (lowercase)
	"video/vnd.dlna.mpeg-tts": true, // ts (DLNA)
}

// InitUpload เริ่ม multipart upload และสร้าง presigned URLs
// POST /api/v1/direct-upload/init
func (h *DirectUploadHandler) InitUpload(c *fiber.Ctx) error {
	ctx := c.UserContext()

	user, err := utils.GetUserFromContext(c)
	if err != nil {
		logger.WarnContext(ctx, "Unauthorized access attempt")
		return utils.UnauthorizedResponse(c, "")
	}

	var req dto.InitDirectUploadRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	// ตรวจสอบ content type (อนุญาต .ts/.mts แม้ MIME type ไม่ตรง เพราะบาง OS ส่ง application/octet-stream)
	isTsFile := strings.HasSuffix(strings.ToLower(req.Filename), ".ts") || strings.HasSuffix(strings.ToLower(req.Filename), ".mts")
	if !allowedVideoTypes[req.ContentType] && !isTsFile {
		logger.WarnContext(ctx, "Invalid content type", "content_type", req.ContentType, "filename", req.Filename)
		return utils.BadRequestResponse(c, "Invalid video type. Allowed: mp4, mkv, avi, mov, webm, ts")
	}

	// ตรวจสอบขนาดไฟล์
	maxFileSize := h.getMaxUploadSize(ctx)
	if req.Size > maxFileSize {
		logger.WarnContext(ctx, "File too large", "size", req.Size, "max", maxFileSize)
		return utils.BadRequestResponse(c, fmt.Sprintf("File too large. Maximum size is %d GB", maxFileSize/(1024*1024*1024)))
	}

	// ตรวจสอบ storage quota
	if err := h.videoService.CheckStorageQuota(ctx); err != nil {
		if errors.Is(err, serviceimpl.ErrStorageQuotaExceeded) {
			logger.WarnContext(ctx, "Storage quota exceeded", "user_id", user.ID)
			return utils.ErrorResponse(c, fiber.StatusPaymentRequired, "STORAGE_QUOTA_EXCEEDED",
				"พื้นที่เก็บข้อมูลเต็ม กรุณาลบวิดีโอเก่าหรือติดต่อทีมงาน", nil)
		}
		logger.ErrorContext(ctx, "Failed to check storage quota", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	// สร้าง video code
	videoCode := utils.GenerateVideoCode()

	// สร้าง path สำหรับเก็บไฟล์
	path := fmt.Sprintf("videos/%s/original%s", videoCode, getFileExtension(req.Filename))

	// สร้าง multipart upload
	uploadID, err := h.storage.CreateMultipartUpload(path, req.ContentType)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to create multipart upload", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	// คำนวณจำนวน parts
	totalParts := int((req.Size + PartSize - 1) / PartSize)

	// สร้าง presigned URLs สำหรับแต่ละ part
	presignedURLs := make([]dto.PartURLInfo, totalParts)
	for i := 0; i < totalParts; i++ {
		partNumber := i + 1
		url, err := h.storage.GetPresignedPartURL(path, uploadID, partNumber, PresignedURLExpiry)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to generate presigned URL",
				"part_number", partNumber,
				"error", err,
			)
			// ยกเลิก upload ที่สร้างไว้
			h.storage.AbortMultipartUpload(path, uploadID)
			return utils.InternalServerErrorResponse(c)
		}
		presignedURLs[i] = dto.PartURLInfo{
			PartNumber: partNumber,
			URL:        url,
		}
	}

	// NOTE: ไม่สร้าง video record ตรงนี้แล้ว
	// จะสร้างตอน CompleteUpload เพื่อหลีกเลี่ยง StuckDetector timeout
	// Frontend จะเก็บ videoCode, path, title ไว้ส่งมาตอน complete

	logger.InfoContext(ctx, "Direct upload initialized",
		"user_id", user.ID,
		"video_code", videoCode,
		"filename", req.Filename,
		"size", req.Size,
		"total_parts", totalParts,
	)

	return utils.SuccessResponse(c, dto.InitDirectUploadResponse{
		UploadID:      uploadID,
		VideoCode:     videoCode,
		Path:          path,
		PartSize:      PartSize,
		TotalParts:    totalParts,
		PresignedURLs: presignedURLs,
		ExpiresIn:     int(PresignedURLExpiry.Seconds()),
	})
}

// CompleteUpload รวม parts, สร้าง video record, และ auto-queue transcode
// POST /api/v1/direct-upload/complete
func (h *DirectUploadHandler) CompleteUpload(c *fiber.Ctx) error {
	ctx := c.UserContext()

	user, err := utils.GetUserFromContext(c)
	if err != nil {
		logger.WarnContext(ctx, "Unauthorized access attempt")
		return utils.UnauthorizedResponse(c, "")
	}

	var req dto.CompleteDirectUploadRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	// แปลง DTO parts เป็น ports.CompletedPart
	completedParts := make([]ports.CompletedPart, len(req.Parts))
	for i, p := range req.Parts {
		completedParts[i] = ports.CompletedPart{
			PartNumber: p.PartNumber,
			ETag:       p.ETag,
		}
	}

	// Complete multipart upload ใน S3 ก่อน
	if err := h.storage.CompleteMultipartUpload(req.Path, req.UploadID, completedParts); err != nil {
		logger.ErrorContext(ctx, "Failed to complete multipart upload",
			"upload_id", req.UploadID,
			"path", req.Path,
			"error", err,
		)
		return utils.BadRequestResponse(c, "Failed to complete upload. Some parts may be missing or corrupted.")
	}

	// กำหนด title (ใช้ชื่อไฟล์ถ้าไม่ได้ระบุ)
	title := req.Title
	if title == "" {
		title = strings.TrimSuffix(req.Filename, getFileExtension(req.Filename))
	}

	// สร้าง video record ใน database (ตอนนี้ upload เสร็จแล้ว)
	video := &models.Video{
		ID:           uuid.New(),
		Code:         req.VideoCode,
		Title:        title,
		Description:  req.Description,
		UserID:       user.ID,
		Status:       models.VideoStatusPending, // จะเปลี่ยนเป็น queued ถ้า auto-queue สำเร็จ
		OriginalPath: req.Path,
	}

	// Set CategoryID if category name provided (find or create)
	if req.Category != "" && h.categoryService != nil {
		category, err := h.categoryService.GetOrCreateByName(ctx, req.Category)
		if err != nil {
			logger.WarnContext(ctx, "Failed to get/create category", "category", req.Category, "error", err)
		} else {
			video.CategoryID = &category.ID
			logger.InfoContext(ctx, "Category assigned", "category_id", category.ID, "category_name", req.Category)
		}
	}

	if err := h.videoService.CreateVideo(ctx, video); err != nil {
		logger.ErrorContext(ctx, "Failed to create video record", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	logger.InfoContext(ctx, "Direct upload completed",
		"video_id", video.ID,
		"video_code", video.Code,
		"total_parts", len(req.Parts),
	)

	// Auto-queue สำหรับ transcode
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
				logger.InfoContext(ctx, "Video auto-queued for transcoding",
					"video_id", video.ID,
					"video_code", video.Code,
					"qualities", qualities,
				)
			}
		}
	}

	return utils.SuccessResponse(c, dto.CompleteDirectUploadResponse{
		VideoID:      video.ID,
		VideoCode:    video.Code,
		Title:        video.Title,
		Status:       string(models.VideoStatusQueued),
		AutoEnqueued: autoEnqueued,
	})
}

// AbortUpload ยกเลิก upload ที่ค้าง
// DELETE /api/v1/direct-upload/abort
func (h *DirectUploadHandler) AbortUpload(c *fiber.Ctx) error {
	ctx := c.UserContext()

	_, err := utils.GetUserFromContext(c)
	if err != nil {
		logger.WarnContext(ctx, "Unauthorized access attempt")
		return utils.UnauthorizedResponse(c, "")
	}

	var req dto.AbortDirectUploadRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	// Abort multipart upload ใน S3
	// NOTE: ไม่มี video record ให้ลบเพราะยังไม่ได้สร้าง
	if err := h.storage.AbortMultipartUpload(req.Path, req.UploadID); err != nil {
		logger.ErrorContext(ctx, "Failed to abort multipart upload",
			"upload_id", req.UploadID,
			"path", req.Path,
			"error", err,
		)
		// ยังคง return success เพราะอาจจะ abort ไปแล้ว หรือ upload หมดอายุ
	}

	logger.InfoContext(ctx, "Direct upload aborted",
		"upload_id", req.UploadID,
		"path", req.Path,
	)

	return utils.SuccessResponse(c, fiber.Map{
		"message": "Upload aborted successfully",
	})
}

// Helper functions

func (h *DirectUploadHandler) getDefaultQualities(ctx context.Context) []string {
	defaultQualities := []string{"1080p", "720p", "480p"}

	if h.settingService == nil {
		return defaultQualities
	}

	qualitiesStr, err := h.settingService.Get(ctx, "transcoding", "default_qualities")
	if err != nil || qualitiesStr == "" {
		return defaultQualities
	}

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

func (h *DirectUploadHandler) isAutoQueueEnabled(ctx context.Context) bool {
	if h.settingService == nil {
		return true
	}

	autoQueueStr, err := h.settingService.Get(ctx, "transcoding", "auto_queue")
	if err != nil || autoQueueStr == "" {
		return true
	}

	return autoQueueStr == "true"
}

func (h *DirectUploadHandler) getMaxUploadSize(ctx context.Context) int64 {
	if h.settingService == nil {
		return DefaultMaxFileSize
	}

	maxSizeStr, err := h.settingService.Get(ctx, "general", "max_upload_size")
	if err != nil || maxSizeStr == "" {
		return DefaultMaxFileSize
	}

	// Parse GB value to bytes
	var maxSizeGB int64
	if _, err := fmt.Sscanf(maxSizeStr, "%d", &maxSizeGB); err != nil {
		return DefaultMaxFileSize
	}

	if maxSizeGB <= 0 {
		return DefaultMaxFileSize
	}

	return maxSizeGB * 1024 * 1024 * 1024
}

// GetUploadLimits returns upload configuration for frontend
// GET /api/v1/config/upload-limits
func (h *DirectUploadHandler) GetUploadLimits(c *fiber.Ctx) error {
	ctx := c.UserContext()

	maxFileSize := h.getMaxUploadSize(ctx)
	maxFileSizeGB := maxFileSize / (1024 * 1024 * 1024)

	return utils.SuccessResponse(c, fiber.Map{
		"max_file_size":    maxFileSize,
		"max_file_size_gb": maxFileSizeGB,
		"part_size":        PartSize,
		"allowed_types":    []string{"video/mp4", "video/x-matroska", "video/x-msvideo", "video/quicktime", "video/webm", "video/MP2T", "video/mp2t", "video/vnd.dlna.mpeg-tts"},
	})
}

func getFileExtension(filename string) string {
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '.' {
			return filename[i:]
		}
	}
	return ""
}
