package serviceimpl

import (
	"context"
	"time"

	"gofiber-template/domain/repositories"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/scheduler"
)

// SubtitleStuckDetectorConfig การตั้งค่าสำหรับ subtitle stuck detector
type SubtitleStuckDetectorConfig struct {
	CheckInterval     time.Duration // ทุกกี่วินาทีจะตรวจสอบ (default: 30s)
	ProcessingTimeout time.Duration // processing/translating นานกว่านี้ถือว่า stuck (default: 10m)
	// ไม่มี QueuedTimeout - jobs รอใน queue ได้นานเท่าที่ต้องการ
	// เหตุผล: ถ้ามี 100 subtitles และ transcribe ทำได้ตัวละ 5 นาที = 500 นาที (~8 ชั่วโมง)
	// queued timeout จะทำให้ jobs หลังๆ fail โดยไม่จำเป็น
}

// SubtitleStuckDetectorService ตรวจจับและจัดการ stuck subtitle jobs
type SubtitleStuckDetectorService struct {
	config       SubtitleStuckDetectorConfig
	subtitleRepo repositories.SubtitleRepository
	scheduler    scheduler.EventScheduler
}

// NewSubtitleStuckDetectorService สร้าง service ใหม่
func NewSubtitleStuckDetectorService(
	config SubtitleStuckDetectorConfig,
	subtitleRepo repositories.SubtitleRepository,
	eventScheduler scheduler.EventScheduler,
) *SubtitleStuckDetectorService {
	service := &SubtitleStuckDetectorService{
		config:       config,
		subtitleRepo: subtitleRepo,
		scheduler:    eventScheduler,
	}

	// Set defaults
	if service.config.CheckInterval == 0 {
		service.config.CheckInterval = 30 * time.Second
	}
	if service.config.ProcessingTimeout == 0 {
		service.config.ProcessingTimeout = 10 * time.Minute // ถ้า worker ไม่ respond 10 นาที = crash
	}
	// ไม่มี queued timeout - รอใน queue ได้ไม่จำกัด

	return service
}

// RegisterDetectorJob ลงทะเบียน detector job กับ scheduler
func (s *SubtitleStuckDetectorService) RegisterDetectorJob() error {
	return s.scheduler.AddJob("subtitle_stuck_detector", "@every 30s", func() {
		ctx := context.Background()
		s.RunDetection(ctx)
	})
}

// RunDetection ตรวจสอบ stuck subtitle jobs
func (s *SubtitleStuckDetectorService) RunDetection(ctx context.Context) {
	// ตรวจสอบเฉพาะ processing/translating/detecting ที่ค้าง (worker crash)
	// ไม่ตรวจสอบ queued - รอใน queue ได้นานเท่าที่ต้องการ
	processingStuck := s.detectStuckProcessing(ctx)

	// Log สรุปเฉพาะเมื่อมี stuck jobs
	if processingStuck > 0 {
		logger.InfoContext(ctx, "Subtitle stuck detection completed",
			"processing_stuck", processingStuck,
		)
	}
}

// detectStuckProcessing ตรวจสอบ subtitles ที่ processing/translating/detecting นานเกินไป (worker crash)
func (s *SubtitleStuckDetectorService) detectStuckProcessing(ctx context.Context) int {
	threshold := time.Now().Add(-s.config.ProcessingTimeout)

	stuckSubtitles, err := s.subtitleRepo.GetStuckProcessing(ctx, threshold)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get stuck processing subtitles", "error", err)
		return 0
	}

	count := 0
	for _, subtitle := range stuckSubtitles {
		logger.WarnContext(ctx, "Detected stuck processing subtitle",
			"subtitle_id", subtitle.ID,
			"video_id", subtitle.VideoID,
			"language", subtitle.Language,
			"status", subtitle.Status,
			"processing_started_at", subtitle.ProcessingStartedAt,
			"timeout", s.config.ProcessingTimeout,
		)

		// Mark as failed
		errorMsg := "Processing timeout: worker not responding for more than 10 minutes"
		if err := s.subtitleRepo.MarkSubtitleFailed(ctx, subtitle.ID, errorMsg); err != nil {
			logger.ErrorContext(ctx, "Failed to mark subtitle as failed", "subtitle_id", subtitle.ID, "error", err)
			continue
		}

		logger.InfoContext(ctx, "Marked stuck processing subtitle as failed",
			"subtitle_id", subtitle.ID,
			"video_id", subtitle.VideoID,
		)
		count++
	}

	return count
}
