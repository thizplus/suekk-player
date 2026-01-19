package serviceimpl

import (
	"context"
	"time"

	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/scheduler"
)

// StuckDetectorConfig การตั้งค่าสำหรับ stuck detector
type StuckDetectorConfig struct {
	CheckInterval     time.Duration // ทุกกี่วินาทีจะตรวจสอบ (default: 30s)
	ProcessingTimeout time.Duration // ถ้า processing นานกว่านี้ถือว่า stuck (default: 1m)
	PendingTimeout    time.Duration // ถ้า pending นานกว่านี้ถือว่า stuck (default: 5m)
	// ไม่มี QueuedTimeout - jobs รอใน queue ได้นานเท่าที่ต้องการ
	// เหตุผล: ถ้ามี 100 videos และ transcode ทำได้ตัวละ 10 นาที = 16+ ชั่วโมง
	// queued timeout 60 นาที จะทำให้ jobs หลังๆ fail โดยไม่จำเป็น
}

// StuckDetectorService ตรวจจับและจัดการ stuck jobs
type StuckDetectorService struct {
	config    StuckDetectorConfig
	videoRepo repositories.VideoRepository
	scheduler scheduler.EventScheduler
}

// NewStuckDetectorService สร้าง service ใหม่
func NewStuckDetectorService(
	config StuckDetectorConfig,
	videoRepo repositories.VideoRepository,
	eventScheduler scheduler.EventScheduler,
) *StuckDetectorService {
	service := &StuckDetectorService{
		config:    config,
		videoRepo: videoRepo,
		scheduler: eventScheduler,
	}

	// Set defaults
	if service.config.CheckInterval == 0 {
		service.config.CheckInterval = 30 * time.Second
	}
	if service.config.ProcessingTimeout == 0 {
		service.config.ProcessingTimeout = 1 * time.Minute // Fast detection - worker crash
	}
	if service.config.PendingTimeout == 0 {
		service.config.PendingTimeout = 5 * time.Minute
	}
	// ไม่มี QueuedTimeout - รอใน queue ได้ไม่จำกัด

	return service
}

// RegisterDetectorJob ลงทะเบียน detector job กับ scheduler
func (s *StuckDetectorService) RegisterDetectorJob() error {
	// รันทุก 30 วินาที (gocron ใช้ format "@every 30s")
	return s.scheduler.AddJob("stuck_detector", "@every 30s", func() {
		ctx := context.Background()
		s.RunDetection(ctx)
	})
}

// RunDetection ตรวจสอบ stuck jobs
func (s *StuckDetectorService) RunDetection(ctx context.Context) {
	// 1. ตรวจสอบ processing ที่ค้าง (ใช้ processing_started_at - fast detection)
	processingStuck := s.detectStuckProcessing(ctx)

	// 2. ตรวจสอบ pending ที่ค้าง (ไม่ถูก publish เข้า queue)
	pendingStuck := s.detectStuckPending(ctx)

	// ไม่ตรวจสอบ queued - jobs รอใน queue ได้นานเท่าที่ต้องการ
	// ตราบใดที่ worker ยังทำงานอยู่ jobs ก็จะถูกทำไปเรื่อยๆ

	// Log สรุปเฉพาะเมื่อมี stuck jobs
	totalStuck := processingStuck + pendingStuck
	if totalStuck > 0 {
		logger.InfoContext(ctx, "Stuck detection completed",
			"processing_stuck", processingStuck,
			"pending_stuck", pendingStuck,
			"total_marked_failed", totalStuck,
		)
	}
}

// detectStuckProcessing ตรวจสอบ videos ที่ processing_started_at เกิน timeout
// ใช้ processing_started_at แทน updated_at เพื่อ fast detection (1 นาที)
func (s *StuckDetectorService) detectStuckProcessing(ctx context.Context) int {
	threshold := time.Now().Add(-s.config.ProcessingTimeout)

	// ใช้ GetStuckProcessing ที่เช็ค processing_started_at
	stuckVideos, err := s.videoRepo.GetStuckProcessing(ctx, threshold)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get stuck processing videos", "error", err)
		return 0
	}

	count := 0
	for _, video := range stuckVideos {
		logger.WarnContext(ctx, "Detected stuck processing video",
			"video_id", video.ID,
			"video_code", video.Code,
			"processing_started_at", video.ProcessingStartedAt,
			"timeout", s.config.ProcessingTimeout,
		)

		// Mark as failed
		errorMsg := "Processing timeout: worker not responding for more than 1 minute"
		if err := s.videoRepo.MarkVideoFailed(ctx, video.ID, errorMsg); err != nil {
			logger.ErrorContext(ctx, "Failed to mark video as failed", "video_id", video.ID, "error", err)
			continue
		}

		logger.InfoContext(ctx, "Marked stuck video as failed",
			"video_id", video.ID,
			"video_code", video.Code,
			"retry_count", video.RetryCount+1,
		)
		count++
	}

	return count
}

// detectStuckPending ตรวจสอบ videos ที่ pending นานเกินไป (ไม่ถูก publish)
func (s *StuckDetectorService) detectStuckPending(ctx context.Context) int {
	threshold := time.Now().Add(-s.config.PendingTimeout)

	stuckVideos, err := s.videoRepo.GetStuckByStatus(ctx, models.VideoStatusPending, threshold)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get stuck pending videos", "error", err)
		return 0
	}

	count := 0
	for _, video := range stuckVideos {
		logger.WarnContext(ctx, "Detected stuck pending video",
			"video_id", video.ID,
			"video_code", video.Code,
			"pending_since", video.UpdatedAt,
			"timeout", s.config.PendingTimeout,
		)

		// Mark as failed
		errorMsg := "Pending timeout: job was not published to queue within 5 minutes"
		if err := s.videoRepo.MarkVideoFailed(ctx, video.ID, errorMsg); err != nil {
			logger.ErrorContext(ctx, "Failed to mark video as failed", "video_id", video.ID, "error", err)
			continue
		}

		logger.InfoContext(ctx, "Marked stuck pending video as failed",
			"video_id", video.ID,
			"video_code", video.Code,
		)
		count++
	}

	return count
}
