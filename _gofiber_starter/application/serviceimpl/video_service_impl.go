package serviceimpl

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
	"gofiber-template/domain/ports"
	"gofiber-template/domain/repositories"
	"gofiber-template/domain/services"
	"gofiber-template/infrastructure/redis"
	"gofiber-template/pkg/config"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/utils"
)

// Storage quota errors
var (
	ErrStorageQuotaExceeded = errors.New("storage quota exceeded")
)

const (
	// Cache keys และ TTLs สำหรับ Video
	videoCachePrefix   = "video:"
	videoCodeCacheKey  = "video:code:"
	videoCacheTTL      = 1 * time.Minute // Cache video 1 นาที
)

type VideoServiceImpl struct {
	videoRepo    repositories.VideoRepository
	categoryRepo repositories.CategoryRepository
	userRepo     repositories.UserRepository
	subtitleRepo repositories.SubtitleRepository
	reelRepo     repositories.ReelRepository // สำหรับนับ reel count
	storage      ports.StoragePort
	redisClient  *redis.Client  // optional - ถ้าไม่มีจะ query DB ตลอด
	config       *config.Config // for storage quota
}

func NewVideoService(
	videoRepo repositories.VideoRepository,
	categoryRepo repositories.CategoryRepository,
	userRepo repositories.UserRepository,
	subtitleRepo repositories.SubtitleRepository,
	reelRepo repositories.ReelRepository,
	storage ports.StoragePort,
	cfg *config.Config,
) services.VideoService {
	return &VideoServiceImpl{
		videoRepo:    videoRepo,
		categoryRepo: categoryRepo,
		userRepo:     userRepo,
		subtitleRepo: subtitleRepo,
		reelRepo:     reelRepo,
		storage:      storage,
		config:       cfg,
		redisClient:  nil,
	}
}

// NewVideoServiceWithCache สร้าง video service พร้อม Redis cache
func NewVideoServiceWithCache(
	videoRepo repositories.VideoRepository,
	categoryRepo repositories.CategoryRepository,
	userRepo repositories.UserRepository,
	subtitleRepo repositories.SubtitleRepository,
	reelRepo repositories.ReelRepository,
	storage ports.StoragePort,
	redisClient *redis.Client,
	cfg *config.Config,
) services.VideoService {
	return &VideoServiceImpl{
		videoRepo:    videoRepo,
		categoryRepo: categoryRepo,
		userRepo:     userRepo,
		subtitleRepo: subtitleRepo,
		reelRepo:     reelRepo,
		storage:      storage,
		redisClient:  redisClient,
		config:       cfg,
	}
}

func (s *VideoServiceImpl) Upload(ctx context.Context, userID uuid.UUID, fileHeader *multipart.FileHeader, req *dto.CreateVideoRequest) (*models.Video, error) {
	// ตรวจสอบ user
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		logger.WarnContext(ctx, "User not found for video upload", "user_id", userID)
		return nil, errors.New("user not found")
	}

	// ตรวจสอบ category (ถ้ามี)
	if req.CategoryID != nil {
		_, err := s.categoryRepo.GetByID(ctx, *req.CategoryID)
		if err != nil {
			logger.WarnContext(ctx, "Category not found", "category_id", req.CategoryID)
			return nil, errors.New("category not found")
		}
	}

	// เปิดไฟล์
	file, err := fileHeader.Open()
	if err != nil {
		logger.ErrorContext(ctx, "Failed to open video file", "filename", fileHeader.Filename, "error", err)
		return nil, err
	}
	defer file.Close()

	// สร้าง video code
	videoCode := s.generateVideoCode()

	// สร้าง path สำหรับเก็บไฟล์
	fileExt := filepath.Ext(fileHeader.Filename)
	originalFileName := fmt.Sprintf("original%s", fileExt)
	storagePath := fmt.Sprintf("videos/%s/%s", videoCode, originalFileName)

	// Normalize path
	storagePath = strings.ReplaceAll(storagePath, "\\", "/")

	// อัปโหลดไปยัง storage
	logger.InfoContext(ctx, "Uploading video to storage", "user_id", userID, "code", videoCode, "path", storagePath)

	mimeType := fileHeader.Header.Get("Content-Type")
	_, err = s.storage.UploadFile(file, storagePath, mimeType)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to upload video to storage", "path", storagePath, "error", err)
		return nil, err
	}

	// สร้าง video record
	video := &models.Video{
		ID:           uuid.New(),
		UserID:       userID,
		CategoryID:   req.CategoryID,
		Code:         videoCode,
		Title:        req.Title,
		Description:  req.Description,
		OriginalPath: storagePath,
		Status:       models.VideoStatusPending,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.videoRepo.Create(ctx, video); err != nil {
		logger.ErrorContext(ctx, "Failed to save video record", "video_id", video.ID, "error", err)
		// Rollback: ลบไฟล์ที่อัปโหลดไป
		s.storage.DeleteFile(storagePath)
		return nil, err
	}

	logger.InfoContext(ctx, "Video uploaded successfully", "video_id", video.ID, "code", videoCode, "user_id", userID)

	// TODO: ส่งเข้า transcoding queue (asynq)

	return video, nil
}

// CreateVideo สร้าง video record โดยไม่ upload (สำหรับ Direct Upload)
func (s *VideoServiceImpl) CreateVideo(ctx context.Context, video *models.Video) error {
	// ตรวจสอบ user
	_, err := s.userRepo.GetByID(ctx, video.UserID)
	if err != nil {
		logger.WarnContext(ctx, "User not found for video creation", "user_id", video.UserID)
		return errors.New("user not found")
	}

	// ตั้งค่า timestamps
	video.CreatedAt = time.Now()
	video.UpdatedAt = time.Now()

	if err := s.videoRepo.Create(ctx, video); err != nil {
		logger.ErrorContext(ctx, "Failed to create video record", "video_id", video.ID, "error", err)
		return err
	}

	logger.InfoContext(ctx, "Video record created (direct upload)",
		"video_id", video.ID,
		"code", video.Code,
		"user_id", video.UserID,
	)

	return nil
}

func (s *VideoServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*models.Video, error) {
	video, err := s.videoRepo.GetByID(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "Video not found", "video_id", id)
		return nil, errors.New("video not found")
	}
	return video, nil
}

func (s *VideoServiceImpl) GetByCode(ctx context.Context, code string) (*models.Video, error) {
	// ถ้ามี Redis cache ใช้ GetOrSet pattern (Singleflight)
	if s.redisClient != nil {
		cacheKey := videoCodeCacheKey + code
		var video models.Video

		err := s.redisClient.GetOrSet(ctx, cacheKey, &video, videoCacheTTL, func() (interface{}, error) {
			// Fetch from DB
			v, err := s.videoRepo.GetByCode(ctx, code)
			if err != nil {
				return nil, err
			}
			logger.InfoContext(ctx, "Video fetched from DB (cache miss)", "code", code)
			return v, nil
		})

		if err != nil {
			logger.WarnContext(ctx, "Video not found", "code", code, "error", err)
			return nil, errors.New("video not found")
		}
		return &video, nil
	}

	// ไม่มี Redis - query DB ตรงๆ
	video, err := s.videoRepo.GetByCode(ctx, code)
	if err != nil {
		logger.WarnContext(ctx, "Video not found", "code", code)
		return nil, errors.New("video not found")
	}
	return video, nil
}

func (s *VideoServiceImpl) GetUserVideos(ctx context.Context, userID uuid.UUID, page, limit int) ([]*models.Video, int64, error) {
	offset := (page - 1) * limit
	videos, err := s.videoRepo.GetByUserID(ctx, userID, offset, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get user videos", "user_id", userID, "error", err)
		return nil, 0, err
	}

	total, err := s.videoRepo.CountByUserID(ctx, userID)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to count user videos", "user_id", userID, "error", err)
		return nil, 0, err
	}

	return videos, total, nil
}

func (s *VideoServiceImpl) GetByCategory(ctx context.Context, categoryID uuid.UUID, page, limit int) ([]*models.Video, int64, error) {
	offset := (page - 1) * limit
	videos, err := s.videoRepo.GetByCategory(ctx, categoryID, offset, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get videos by category", "category_id", categoryID, "error", err)
		return nil, 0, err
	}

	// Count จะต้องเพิ่ม method ใน repository แต่ตอนนี้ใช้ Count ทั่วไปก่อน
	total := int64(len(videos))

	return videos, total, nil
}

func (s *VideoServiceImpl) ListVideos(ctx context.Context, page, limit int) ([]*models.Video, int64, error) {
	offset := (page - 1) * limit
	videos, err := s.videoRepo.List(ctx, offset, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to list videos", "error", err)
		return nil, 0, err
	}

	total, err := s.videoRepo.Count(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to count videos", "error", err)
		return nil, 0, err
	}

	return videos, total, nil
}

func (s *VideoServiceImpl) ListWithFilters(ctx context.Context, params *dto.VideoFilterRequest) ([]*models.Video, int64, error) {
	videos, total, err := s.videoRepo.ListWithFilters(ctx, params)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to list videos with filters", "error", err)
		return nil, 0, err
	}
	return videos, total, nil
}

// GetReelCountsForVideos นับจำนวน reels สำหรับแต่ละ video (batch query)
func (s *VideoServiceImpl) GetReelCountsForVideos(ctx context.Context, videos []*models.Video) (map[uuid.UUID]int64, error) {
	if s.reelRepo == nil || len(videos) == 0 {
		return make(map[uuid.UUID]int64), nil
	}

	// Extract video IDs
	videoIDs := make([]uuid.UUID, len(videos))
	for i, v := range videos {
		videoIDs[i] = v.ID
	}

	// Batch query reel counts
	counts, err := s.reelRepo.CountByVideoIDs(ctx, videoIDs)
	if err != nil {
		logger.WarnContext(ctx, "Failed to get reel counts", "error", err)
		return make(map[uuid.UUID]int64), nil // ไม่ fail ทั้งหมด แค่ return empty
	}

	return counts, nil
}

func (s *VideoServiceImpl) ListVideosByStatus(ctx context.Context, status models.VideoStatus, page, limit int) ([]*models.Video, int64, error) {
	offset := (page - 1) * limit
	videos, err := s.videoRepo.GetByStatus(ctx, status, offset, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to list videos by status", "status", status, "error", err)
		return nil, 0, err
	}

	total, err := s.videoRepo.CountByStatus(ctx, status)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to count videos by status", "status", status, "error", err)
		return nil, 0, err
	}

	return videos, total, nil
}

func (s *VideoServiceImpl) ListReadyVideos(ctx context.Context, page, limit int) ([]*models.Video, int64, error) {
	offset := (page - 1) * limit
	videos, err := s.videoRepo.ListReady(ctx, offset, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to list ready videos", "error", err)
		return nil, 0, err
	}

	total, err := s.videoRepo.CountByStatus(ctx, models.VideoStatusReady)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to count ready videos", "error", err)
		return nil, 0, err
	}

	return videos, total, nil
}

func (s *VideoServiceImpl) Update(ctx context.Context, id uuid.UUID, req *dto.UpdateVideoRequest) (*models.Video, error) {
	video, err := s.videoRepo.GetByID(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "Video not found for update", "video_id", id)
		return nil, errors.New("video not found")
	}

	// อัปเดตเฉพาะ field ที่ส่งมา
	if req.Title != nil {
		video.Title = *req.Title
	}
	if req.Description != nil {
		video.Description = *req.Description
	}
	if req.CategoryID != nil {
		// ตรวจสอบ category
		_, err := s.categoryRepo.GetByID(ctx, *req.CategoryID)
		if err != nil {
			logger.WarnContext(ctx, "Category not found", "category_id", req.CategoryID)
			return nil, errors.New("category not found")
		}
		video.CategoryID = req.CategoryID
	}

	// Gallery fields - Manual Selection Flow
	if req.GalleryPath != nil {
		video.GalleryPath = *req.GalleryPath
	}
	if req.GalleryStatus != nil {
		video.GalleryStatus = *req.GalleryStatus
	}
	if req.GallerySourceCount != nil {
		video.GallerySourceCount = *req.GallerySourceCount
	}
	if req.GalleryCount != nil {
		video.GalleryCount = *req.GalleryCount
	}
	if req.GallerySafeCount != nil {
		video.GallerySafeCount = *req.GallerySafeCount
	}
	if req.GalleryNsfwCount != nil {
		video.GalleryNsfwCount = *req.GalleryNsfwCount
	}
	if req.GallerySuperSafeCount != nil {
		video.GallerySuperSafeCount = *req.GallerySuperSafeCount // Deprecated
	}

	video.UpdatedAt = time.Now()

	if err := s.videoRepo.Update(ctx, video); err != nil {
		logger.ErrorContext(ctx, "Failed to update video", "video_id", id, "error", err)
		return nil, err
	}

	logger.InfoContext(ctx, "Video updated", "video_id", id)
	return video, nil
}

func (s *VideoServiceImpl) Delete(ctx context.Context, id uuid.UUID) error {
	video, err := s.videoRepo.GetByID(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "Video not found for deletion", "video_id", id)
		return errors.New("video not found")
	}

	// เก็บข้อมูลที่ต้องใช้ลบไฟล์ (ก่อนลบ record)
	videoCode := video.Code
	originalPath := video.OriginalPath
	audioPath := video.AudioPath

	// ดึง subtitles ก่อนลบ (เพื่อเก็บ paths สำหรับลบไฟล์)
	var subtitlePaths []string
	if s.subtitleRepo != nil {
		subtitles, _ := s.subtitleRepo.GetByVideoID(ctx, id)
		for _, sub := range subtitles {
			if sub.SRTPath != "" {
				subtitlePaths = append(subtitlePaths, sub.SRTPath)
			}
		}
	}

	logger.InfoContext(ctx, "Deleting video",
		"video_id", id,
		"video_code", videoCode,
		"original_path", originalPath,
		"audio_path", audioPath,
		"hls_path", video.HLSPath,
		"subtitle_count", len(subtitlePaths),
	)

	// ลบ subtitle records จาก database
	if s.subtitleRepo != nil {
		if err := s.subtitleRepo.DeleteByVideoID(ctx, id); err != nil {
			logger.WarnContext(ctx, "Failed to delete subtitle records", "video_id", id, "error", err)
			// ไม่ return error - ยังคงลบ video ต่อไป
		} else {
			logger.InfoContext(ctx, "Subtitle records deleted", "video_id", id)
		}
	}

	// ลบ video record จาก database (เร็ว) - ให้ UI อัปเดตทันที
	if err := s.videoRepo.Delete(ctx, id); err != nil {
		logger.ErrorContext(ctx, "Failed to delete video record", "video_id", id, "error", err)
		return err
	}

	logger.InfoContext(ctx, "Video record deleted, cleaning up files in background", "video_id", id, "video_code", videoCode)

	// ลบไฟล์ใน background (ไม่ block response)
	go func() {
		bgCtx := context.Background()

		// ลบไฟล์ original จาก storage
		if originalPath != "" {
			if err := s.storage.DeleteFile(originalPath); err != nil {
				logger.WarnContext(bgCtx, "Failed to delete original file", "path", originalPath, "error", err)
			} else {
				logger.InfoContext(bgCtx, "Deleted original file", "path", originalPath)
			}
		}

		// ลบไฟล์ audio จาก storage
		if audioPath != "" {
			if err := s.storage.DeleteFile(audioPath); err != nil {
				logger.WarnContext(bgCtx, "Failed to delete audio file", "path", audioPath, "error", err)
			} else {
				logger.InfoContext(bgCtx, "Deleted audio file", "path", audioPath)
			}
		}

		// ลบไฟล์ subtitle (.srt) จาก storage
		for _, srtPath := range subtitlePaths {
			if err := s.storage.DeleteFile(srtPath); err != nil {
				logger.WarnContext(bgCtx, "Failed to delete subtitle file", "path", srtPath, "error", err)
			} else {
				logger.InfoContext(bgCtx, "Deleted subtitle file", "path", srtPath)
			}
		}

		// ลบ subtitles folder (subtitles/<code>/)
		if videoCode != "" {
			subtitlesFolder := fmt.Sprintf("subtitles/%s/", videoCode)
			if err := s.storage.DeleteFolder(subtitlesFolder); err != nil {
				logger.WarnContext(bgCtx, "Failed to delete subtitles folder", "folder", subtitlesFolder, "error", err)
			} else {
				logger.InfoContext(bgCtx, "Deleted subtitles folder", "folder", subtitlesFolder)
			}
		}

		// ลบ folder videos/<code>/ (ถ้ามี files อื่นๆ ใน folder)
		if videoCode != "" {
			videoFolder := fmt.Sprintf("videos/%s/", videoCode)
			if err := s.storage.DeleteFolder(videoFolder); err != nil {
				logger.WarnContext(bgCtx, "Failed to delete video folder", "folder", videoFolder, "error", err)
			} else {
				logger.InfoContext(bgCtx, "Deleted video folder", "folder", videoFolder)
			}
		}

		// ลบ HLS folder ทั้งหมด (hls/<code>/) - มีหลายไฟล์ย่อย
		if videoCode != "" {
			hlsFolder := fmt.Sprintf("hls/%s/", videoCode)
			if err := s.storage.DeleteFolder(hlsFolder); err != nil {
				logger.WarnContext(bgCtx, "Failed to delete HLS folder", "folder", hlsFolder, "error", err)
			} else {
				logger.InfoContext(bgCtx, "Deleted HLS folder", "folder", hlsFolder)
			}
		}

		logger.InfoContext(bgCtx, "Video files cleanup completed", "video_code", videoCode)
	}()

	return nil
}

func (s *VideoServiceImpl) IncrementViews(ctx context.Context, id uuid.UUID) error {
	return s.videoRepo.IncrementViews(ctx, id)
}

func (s *VideoServiceImpl) GetStats(ctx context.Context) (*services.VideoStats, error) {
	total, err := s.videoRepo.Count(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to count total videos", "error", err)
		return nil, err
	}

	pending, _ := s.videoRepo.CountByStatus(ctx, models.VideoStatusPending)
	queued, _ := s.videoRepo.CountByStatus(ctx, models.VideoStatusQueued)
	processing, _ := s.videoRepo.CountByStatus(ctx, models.VideoStatusProcessing)
	ready, _ := s.videoRepo.CountByStatus(ctx, models.VideoStatusReady)
	failed, _ := s.videoRepo.CountByStatus(ctx, models.VideoStatusFailed)
	deadLetter, _ := s.videoRepo.CountByStatus(ctx, models.VideoStatusDeadLetter)

	logger.InfoContext(ctx, "Video stats retrieved",
		"total", total,
		"pending", pending,
		"queued", queued,
		"processing", processing,
		"ready", ready,
		"failed", failed,
		"dead_letter", deadLetter,
	)

	return &services.VideoStats{
		TotalVideos:      total,
		PendingVideos:    pending,
		QueuedVideos:     queued,
		ProcessingVideos: processing,
		ReadyVideos:      ready,
		FailedVideos:     failed,
		DeadLetterVideos: deadLetter,
	}, nil
}

// generateVideoCode สร้าง unique video code
func (s *VideoServiceImpl) generateVideoCode() string {
	return utils.GenerateRandomString(8)
}

// GetStuckVideos ดึง videos ที่ค้างสถานะ pending/queued/processing นานเกินกำหนด
// Timeout: pending=5m, queued=60m, processing=30m
func (s *VideoServiceImpl) GetStuckVideos(ctx context.Context, minutes int) ([]*models.Video, error) {
	// ใช้ minutes parameter สำหรับ pending เท่านั้น
	pendingThreshold := time.Now().Add(-time.Duration(minutes) * time.Minute)
	// queued มี timeout ยาวกว่า (60 นาที) เพราะอาจรอ worker นาน
	queuedThreshold := time.Now().Add(-60 * time.Minute)
	// processing มี timeout 30 นาที (transcoding ไม่ควรนานเกินนี้)
	processingThreshold := time.Now().Add(-30 * time.Minute)

	// ดึง pending videos (ยังไม่ถูก publish ไป NATS)
	pendingVideos, err := s.videoRepo.GetStuckByStatus(ctx, models.VideoStatusPending, pendingThreshold)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get stuck pending videos", "error", err)
		return nil, err
	}

	// ดึง queued videos (อยู่ใน NATS queue รอ worker)
	queuedVideos, err := s.videoRepo.GetStuckByStatus(ctx, models.VideoStatusQueued, queuedThreshold)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get stuck queued videos", "error", err)
		return nil, err
	}

	// ดึง processing videos (worker กำลังทำงาน)
	processingVideos, err := s.videoRepo.GetStuckByStatus(ctx, models.VideoStatusProcessing, processingThreshold)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get stuck processing videos", "error", err)
		return nil, err
	}

	// Log สรุปจำนวน stuck videos
	logger.InfoContext(ctx, "Stuck videos summary",
		"pending_count", len(pendingVideos),
		"pending_threshold_minutes", minutes,
		"queued_count", len(queuedVideos),
		"queued_threshold_minutes", 60,
		"processing_count", len(processingVideos),
		"processing_threshold_minutes", 30,
	)

	// รวม results
	result := append(pendingVideos, queuedVideos...)
	result = append(result, processingVideos...)
	return result, nil
}

// UpdateVideoStatus อัปเดต status ของ video
func (s *VideoServiceImpl) UpdateVideoStatus(ctx context.Context, id uuid.UUID, status models.VideoStatus) error {
	video, err := s.videoRepo.GetByID(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "Video not found for status update", "video_id", id)
		return errors.New("video not found")
	}

	video.Status = status
	video.UpdatedAt = time.Now()

	if err := s.videoRepo.Update(ctx, video); err != nil {
		logger.ErrorContext(ctx, "Failed to update video status", "video_id", id, "status", status, "error", err)
		return err
	}

	// Invalidate cache
	s.invalidateVideoCache(ctx, video.Code)

	logger.InfoContext(ctx, "Video status updated", "video_id", id, "status", status)
	return nil
}

// invalidateVideoCache ลบ cache ของ video
func (s *VideoServiceImpl) invalidateVideoCache(ctx context.Context, code string) {
	if s.redisClient == nil {
		return
	}
	cacheKey := videoCodeCacheKey + code
	if err := s.redisClient.Del(ctx, cacheKey); err != nil {
		logger.WarnContext(ctx, "Failed to invalidate video cache", "code", code, "error", err)
	} else {
		logger.InfoContext(ctx, "Video cache invalidated", "code", code)
	}
}

// ResetVideoForRetry reset video สำหรับ retry จาก DLQ (ล้าง retry_count และ last_error)
func (s *VideoServiceImpl) ResetVideoForRetry(ctx context.Context, id uuid.UUID) error {
	video, err := s.videoRepo.GetByID(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "Video not found for retry reset", "video_id", id)
		return errors.New("video not found")
	}

	// บันทึก previous state สำหรับ logging
	previousStatus := video.Status
	previousRetryCount := video.RetryCount

	if err := s.videoRepo.ResetForRetry(ctx, id); err != nil {
		logger.ErrorContext(ctx, "Failed to reset video for retry", "video_id", id, "error", err)
		return err
	}

	logger.InfoContext(ctx, "Video reset for retry",
		"video_id", id,
		"video_code", video.Code,
		"previous_status", previousStatus,
		"previous_retry_count", previousRetryCount,
	)
	return nil
}

// DeleteAll ลบ videos ทั้งหมด (สำหรับ testing)
func (s *VideoServiceImpl) DeleteAll(ctx context.Context) (int64, error) {
	count, err := s.videoRepo.DeleteAll(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to delete all videos", "error", err)
		return 0, err
	}
	logger.InfoContext(ctx, "All videos deleted", "count", count)
	return count, nil
}

// CheckStorageQuota ตรวจสอบว่ายังอัพโหลดได้หรือไม่
// Logic: ถ้า current_used < quota → อนุญาต (ไม่สนใจ file_size ที่จะอัพ)
func (s *VideoServiceImpl) CheckStorageQuota(ctx context.Context) error {
	// ถ้า quota = 0 → unlimited
	if s.config == nil || s.config.Storage.QuotaTotal <= 0 {
		return nil
	}

	totalUsed, err := s.videoRepo.GetTotalStorageUsed(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get total storage used", "error", err)
		return err
	}

	// ถ้า current_used >= quota → block
	if totalUsed >= s.config.Storage.QuotaTotal {
		logger.WarnContext(ctx, "Storage quota exceeded",
			"current_used", totalUsed,
			"quota", s.config.Storage.QuotaTotal,
		)
		return ErrStorageQuotaExceeded
	}

	return nil
}

// GetStorageUsage ดึงข้อมูล storage usage
func (s *VideoServiceImpl) GetStorageUsage(ctx context.Context) (*services.StorageUsage, error) {
	totalUsed, err := s.videoRepo.GetTotalStorageUsed(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get total storage used", "error", err)
		return nil, err
	}

	quota := int64(0)
	if s.config != nil {
		quota = s.config.Storage.QuotaTotal
	}

	usage := &services.StorageUsage{
		TotalUsed:  totalUsed,
		TotalQuota: quota,
		Unlimited:  quota <= 0,
	}

	// Calculate percentage
	if quota > 0 {
		usage.TotalPercent = float64(totalUsed) / float64(quota) * 100
	}

	return usage, nil
}
