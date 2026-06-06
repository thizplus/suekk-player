package serviceimpl

import (
	"context"
	"time"

	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/scheduler"
)

// SubtitleStuckDetectorConfig การตั้งค่าสำหรับ subtitle stuck detector
type SubtitleStuckDetectorConfig struct {
	CheckInterval     time.Duration // ทุกกี่วินาทีจะตรวจสอบ (default: 30s)
	ProcessingTimeout time.Duration // processing/translating นานกว่านี้ถือว่า stuck (default: 10m)
	// ไม่มี QueuedTimeout - jobs รอใน queue ได้นานเท่าที่ต้องการ
}

// SubtitleStuckDetectorService ตรวจจับและจัดการ stuck subtitle jobs
// ใช้ WorkerJob table แทน Subtitle.ProcessingStartedAt
type SubtitleStuckDetectorService struct {
	config           SubtitleStuckDetectorConfig
	workerJobService services.WorkerJobService
	subtitleRepo     repositories.SubtitleRepository
	scheduler        scheduler.EventScheduler
}

// NewSubtitleStuckDetectorService สร้าง service ใหม่
func NewSubtitleStuckDetectorService(
	config SubtitleStuckDetectorConfig,
	workerJobService services.WorkerJobService,
	subtitleRepo repositories.SubtitleRepository,
	eventScheduler scheduler.EventScheduler,
) *SubtitleStuckDetectorService {
	service := &SubtitleStuckDetectorService{
		config:           config,
		workerJobService: workerJobService,
		subtitleRepo:     subtitleRepo,
		scheduler:        eventScheduler,
	}

	// Set defaults
	if service.config.CheckInterval == 0 {
		service.config.CheckInterval = 30 * time.Second
	}
	if service.config.ProcessingTimeout == 0 {
		service.config.ProcessingTimeout = 60 * time.Minute // Transcribe อาจใช้เวลา 30-45 นาที (Demucs+Whisper+Refine)
	}

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
	// ตรวจสอบเฉพาะ processing jobs ที่ค้าง (worker crash)
	// ใช้ WorkerJob.StartedAt
	processingStuck := s.detectStuckProcessing(ctx)

	// Log สรุปเฉพาะเมื่อมี stuck jobs
	if processingStuck > 0 {
		logger.InfoContext(ctx, "Subtitle stuck detection completed",
			"processing_stuck", processingStuck,
		)
	}
}

// detectStuckProcessing ตรวจสอบ WorkerJob ที่ subtitle processing นานเกินไป
func (s *SubtitleStuckDetectorService) detectStuckProcessing(ctx context.Context) int {
	// Get stuck jobs from WorkerJob table
	stuckJobs, err := s.workerJobService.GetStuckJobs(ctx, s.config.ProcessingTimeout)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get stuck processing jobs", "error", err)
		return 0
	}

	count := 0
	for _, job := range stuckJobs {
		// Only handle subtitle-related job types
		switch job.JobType {
		case models.WorkerJobTypeSubtitleDetect, models.WorkerJobTypeSubtitleTranscribe, models.WorkerJobTypeSubtitleTranslate:
			// These are subtitle jobs - handle them
		default:
			continue
		}

		logger.WarnContext(ctx, "Detected stuck subtitle job",
			"job_id", job.ID,
			"job_type", job.JobType,
			"entity_type", job.EntityType,
			"entity_id", job.EntityID,
			"entity_code", job.EntityCode,
			"started_at", job.StartedAt,
			"timeout", s.config.ProcessingTimeout,
		)

		// Mark WorkerJob as failed
		errorMsg := "Processing timeout: worker not responding for more than 60 minutes"
		if err := s.workerJobService.MarkAsFailed(ctx, job.ID, errorMsg, "WORKER_TIMEOUT", "processing", false); err != nil {
			logger.ErrorContext(ctx, "Failed to mark job as failed", "job_id", job.ID, "error", err)
			continue
		}

		// Also mark the Subtitle as failed (for backward compat)
		// Only for transcribe/translate jobs where entity is subtitle
		if job.EntityType == models.WorkerEntityTypeSubtitle {
			if err := s.subtitleRepo.MarkSubtitleFailed(ctx, job.EntityID, errorMsg); err != nil {
				logger.ErrorContext(ctx, "Failed to mark subtitle as failed", "subtitle_id", job.EntityID, "error", err)
			}
		}

		logger.InfoContext(ctx, "Marked stuck subtitle job as failed",
			"job_id", job.ID,
			"job_type", job.JobType,
			"entity_code", job.EntityCode,
		)
		count++
	}

	return count
}
