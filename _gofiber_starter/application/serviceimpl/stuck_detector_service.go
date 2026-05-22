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

// StuckDetectorConfig การตั้งค่าสำหรับ stuck detector
type StuckDetectorConfig struct {
	CheckInterval     time.Duration // ทุกกี่วินาทีจะตรวจสอบ (default: 30s)
	ProcessingTimeout time.Duration // ถ้า processing นานกว่านี้ถือว่า stuck (default: 1m)
	PendingTimeout    time.Duration // ถ้า pending นานกว่านี้ถือว่า stuck (default: 5m)
	QueuedTimeout     time.Duration // ถ้า queued นานกว่านี้ถือว่า message หายจาก NATS (default: 60m)
}

// StuckDetectorService ตรวจจับและจัดการ stuck jobs
// ใช้ WorkerJob table แทน Video.ProcessingStartedAt
type StuckDetectorService struct {
	config           StuckDetectorConfig
	workerJobService services.WorkerJobService
	videoRepo        repositories.VideoRepository
	scheduler        scheduler.EventScheduler
}

// NewStuckDetectorService สร้าง service ใหม่
func NewStuckDetectorService(
	config StuckDetectorConfig,
	workerJobService services.WorkerJobService,
	videoRepo repositories.VideoRepository,
	eventScheduler scheduler.EventScheduler,
) *StuckDetectorService {
	service := &StuckDetectorService{
		config:           config,
		workerJobService: workerJobService,
		videoRepo:        videoRepo,
		scheduler:        eventScheduler,
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
	if service.config.QueuedTimeout == 0 {
		service.config.QueuedTimeout = 60 * time.Minute
	}

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
	// 1. ตรวจสอบ processing jobs ที่ค้าง (ใช้ WorkerJob.StartedAt)
	processingStuck := s.detectStuckProcessing(ctx)

	// 2. ตรวจสอบ pending jobs ที่ค้าง (ไม่ถูก publish เข้า queue)
	pendingStuck := s.detectStuckPending(ctx)

	// 3. ตรวจสอบ queued videos ที่ค้าง (message หายจาก NATS)
	queuedStuck := s.detectStuckQueued(ctx)

	// Log สรุปเฉพาะเมื่อมี stuck jobs
	totalStuck := processingStuck + pendingStuck + queuedStuck
	if totalStuck > 0 {
		logger.InfoContext(ctx, "Stuck detection completed",
			"processing_stuck", processingStuck,
			"pending_stuck", pendingStuck,
			"queued_stuck", queuedStuck,
			"total_marked_failed", totalStuck,
		)
	}
}

// detectStuckProcessing ตรวจสอบ WorkerJob ที่ processing นานเกิน timeout
// ใช้ WorkerJob.StartedAt เพื่อ fast detection (1 นาที)
func (s *StuckDetectorService) detectStuckProcessing(ctx context.Context) int {
	// Get stuck jobs from WorkerJob table
	stuckJobs, err := s.workerJobService.GetStuckJobs(ctx, s.config.ProcessingTimeout)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get stuck processing jobs", "error", err)
		return 0
	}

	count := 0
	for _, job := range stuckJobs {
		// Only handle video-related job types
		if job.EntityType != models.WorkerEntityTypeVideo {
			continue
		}

		// Skip non-video job types (subtitle, reel handled by their own detectors)
		switch job.JobType {
		case models.WorkerJobTypeTranscode, models.WorkerJobTypeGallery, models.WorkerJobTypeWarmCache:
			// These are video jobs - handle them
		default:
			continue
		}

		logger.WarnContext(ctx, "Detected stuck processing job",
			"job_id", job.ID,
			"job_type", job.JobType,
			"entity_id", job.EntityID,
			"entity_code", job.EntityCode,
			"started_at", job.StartedAt,
			"timeout", s.config.ProcessingTimeout,
		)

		// Mark WorkerJob as failed
		errorMsg := "Processing timeout: worker not responding for more than " + s.config.ProcessingTimeout.String()
		if err := s.workerJobService.MarkAsFailed(ctx, job.ID, errorMsg, "WORKER_TIMEOUT", "processing", false); err != nil {
			logger.ErrorContext(ctx, "Failed to mark job as failed", "job_id", job.ID, "error", err)
			continue
		}

		// Also mark the Video as failed (for backward compat)
		if job.JobType == models.WorkerJobTypeTranscode {
			if err := s.videoRepo.MarkVideoFailed(ctx, job.EntityID, errorMsg); err != nil {
				logger.ErrorContext(ctx, "Failed to mark video as failed", "video_id", job.EntityID, "error", err)
			}
		}

		logger.InfoContext(ctx, "Marked stuck job as failed",
			"job_id", job.ID,
			"job_type", job.JobType,
			"entity_code", job.EntityCode,
		)
		count++
	}

	return count
}

// detectStuckQueued ตรวจสอบ videos ที่ queued นานเกินไป (message หายจาก NATS)
// เกิดได้จาก: NATS restart, MaxAge 24h หมด, MaxDeliver 5 ครบ, stream purge
// จะ mark video เป็น failed เพื่อให้ retry ได้ผ่าน queue management
func (s *StuckDetectorService) detectStuckQueued(ctx context.Context) int {
	threshold := time.Now().Add(-s.config.QueuedTimeout)

	stuckVideos, err := s.videoRepo.GetStuckByStatus(ctx, models.VideoStatusQueued, threshold)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get stuck queued videos", "error", err)
		return 0
	}

	count := 0
	for _, video := range stuckVideos {
		logger.WarnContext(ctx, "Detected stuck queued video (NATS message likely lost)",
			"video_id", video.ID,
			"video_code", video.Code,
			"queued_since", video.UpdatedAt,
			"timeout", s.config.QueuedTimeout,
		)

		// Mark as failed เพื่อให้ retry ได้
		errorMsg := "Queued timeout: no worker picked up the job within " + s.config.QueuedTimeout.String() + " (NATS message may have expired)"
		if err := s.videoRepo.MarkVideoFailed(ctx, video.ID, errorMsg); err != nil {
			logger.ErrorContext(ctx, "Failed to mark queued video as failed", "video_id", video.ID, "error", err)
			continue
		}

		// Mark related WorkerJob as failed too
		if s.workerJobService != nil {
			jobs, _ := s.workerJobService.GetJobsByEntity(ctx, models.WorkerEntityTypeVideo, video.ID)
			for _, job := range jobs {
				if job.Status == models.WorkerJobStatusQueued || job.Status == models.WorkerJobStatusPending {
					s.workerJobService.MarkAsFailed(ctx, job.ID, errorMsg, "QUEUED_TIMEOUT", "queued", false)
				}
			}
		}

		logger.InfoContext(ctx, "Marked stuck queued video as failed",
			"video_id", video.ID,
			"video_code", video.Code,
		)
		count++
	}

	return count
}

// detectStuckPending ตรวจสอบ videos ที่ pending นานเกินไป (ไม่ถูก publish)
// ยังใช้ Video table เพราะ WorkerJob ยังไม่ถูกสร้าง
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
