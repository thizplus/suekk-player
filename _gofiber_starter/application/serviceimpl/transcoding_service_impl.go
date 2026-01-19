package serviceimpl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"gofiber-template/domain/models"
	"gofiber-template/domain/ports"
	"gofiber-template/domain/repositories"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/progress"
)

type TranscodingConfig struct {
	VideoBasePath        string   // base path for video storage
	TempPath             string   // temp directory
	CleanupOriginal      bool     // delete original after transcoding
	FFmpegPreset         string   // ffmpeg preset (ultrafast, fast, medium, slow)
	CRF                  int      // quality (0-51, lower = better)
	GenerateH264Fallback bool     // generate H.264 fallback for older devices
	UseAdaptiveBitrate   bool     // use multi-quality adaptive bitrate
	MaxRetries           int      // max retry attempts for failed jobs
	DefaultQualities     []string // default qualities ["1080p", "720p", "480p"]
}

type TranscodingServiceImpl struct {
	videoRepo      repositories.VideoRepository
	transcoder     ports.TranscoderPort
	storage        ports.StoragePort
	jobQueue       ports.JobQueuePort      // NATS Job Queue (distributed workers)
	settingService services.SettingService // Settings service for runtime config
	config         TranscodingConfig
}

// NewTranscodingService สร้าง TranscodingService ที่ใช้ NATS Job Queue
// ส่ง job ไปยัง distributed workers
func NewTranscodingService(
	videoRepo repositories.VideoRepository,
	transcoder ports.TranscoderPort,
	storage ports.StoragePort,
	jobQueue ports.JobQueuePort,
	settingService services.SettingService,
	config TranscodingConfig,
) services.TranscodingService {
	return &TranscodingServiceImpl{
		videoRepo:      videoRepo,
		transcoder:     transcoder,
		storage:        storage,
		jobQueue:       jobQueue,
		settingService: settingService,
		config:         config,
	}
}

// QueueTranscoding ส่งวิดีโอเข้า transcoding queue (NATS Distributed Workers)
func (s *TranscodingServiceImpl) QueueTranscoding(ctx context.Context, videoID uuid.UUID) error {
	// ตรวจสอบว่า video มีอยู่จริง
	video, err := s.videoRepo.GetByID(ctx, videoID)
	if err != nil {
		logger.WarnContext(ctx, "Video not found for transcoding", "video_id", videoID)
		return errors.New("video not found")
	}

	// ตรวจสอบว่า video อยู่ในสถานะที่สามารถ transcode ได้ (pending, queued หรือ failed สำหรับ retry)
	if video.Status != models.VideoStatusPending && video.Status != models.VideoStatusQueued && video.Status != models.VideoStatusFailed {
		logger.WarnContext(ctx, "Video not in valid status for transcoding", "video_id", videoID, "status", video.Status)
		return fmt.Errorf("video must be in pending, queued or failed status to queue (current: %s)", video.Status)
	}

	// Reset status เป็น pending ก่อน queue (สำหรับ retry)
	if video.Status == models.VideoStatusFailed {
		if err := s.videoRepo.UpdateStatus(ctx, videoID, models.VideoStatusPending); err != nil {
			logger.ErrorContext(ctx, "Failed to reset video status", "video_id", videoID, "error", err)
			return fmt.Errorf("failed to reset video status: %w", err)
		}
		logger.InfoContext(ctx, "Video status reset for retry", "video_id", videoID)
	}

	// ตรวจสอบว่ามี Job Queue หรือไม่
	if s.jobQueue == nil {
		logger.ErrorContext(ctx, "Job queue not initialized", "video_id", videoID)
		return errors.New("job queue not available")
	}

	// ตรวจสอบ queue overflow protection
	if err := s.checkQueueOverflow(ctx); err != nil {
		logger.WarnContext(ctx, "Queue overflow protection triggered", "video_id", videoID, "error", err)
		return err
	}

	// กำหนด qualities จาก Settings (runtime config)
	qualities := s.getDefaultQualities(ctx)

	// Log qualities ที่จะส่งไป worker
	logger.InfoContext(ctx, "Qualities to be sent to worker",
		"video_id", videoID,
		"qualities", qualities,
		"qualities_count", len(qualities),
	)

	// ส่ง job ไปยัง NATS JetStream ให้ distributed workers ประมวลผล
	jobData := &ports.TranscodeJobData{
		VideoID:      videoID.String(),
		VideoCode:    video.Code,
		InputPath:    video.OriginalPath,
		OutputPath:   filepath.Join("videos", video.Code),
		Codec:        "h264",
		Qualities:    qualities,
		UseByteRange: false,
	}

	if err := s.jobQueue.PublishJob(ctx, jobData); err != nil {
		logger.ErrorContext(ctx, "Failed to publish job to NATS", "video_id", videoID, "error", err)
		return fmt.Errorf("failed to queue job: %w", err)
	}

	logger.InfoContext(ctx, "Video queued for transcoding",
		"video_id", videoID,
		"code", video.Code,
		"qualities_sent", qualities,
	)

	return nil
}

// ProcessTranscoding ทำ transcoding (เรียกจาก worker)
func (s *TranscodingServiceImpl) ProcessTranscoding(ctx context.Context, videoID uuid.UUID) error {
	return s.processTranscodingWithRetry(ctx, videoID, 0)
}

// processTranscodingWithRetry ทำ transcoding พร้อม retry mechanism
func (s *TranscodingServiceImpl) processTranscodingWithRetry(ctx context.Context, videoID uuid.UUID, attempt int) error {
	maxRetries := s.config.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3 // default 3 retries
	}

	logger.InfoContext(ctx, "Starting transcoding process", "video_id", videoID, "attempt", attempt+1)

	// ดึงข้อมูล video
	video, err := s.videoRepo.GetByID(ctx, videoID)
	if err != nil {
		logger.ErrorContext(ctx, "Video not found", "video_id", videoID)
		return err
	}

	// Get progress tracker
	tracker := progress.GetTracker()

	// Start transcoding progress
	tracker.StartTranscoding(video.UserID, videoID, video.Code, video.Title)

	// อัพเดทสถานะเป็น processing
	if err := s.videoRepo.UpdateStatus(ctx, videoID, models.VideoStatusProcessing); err != nil {
		logger.ErrorContext(ctx, "Failed to update video status", "video_id", videoID, "error", err)
		tracker.FailProgress(video.UserID, videoID, "Failed to update status")
		return err
	}

	tracker.UpdateTranscodingProgress(video.UserID, videoID, 5, "เตรียมไฟล์", "กำลังเตรียมไฟล์สำหรับแปลง")

	// เตรียม paths
	inputPath := filepath.Join(s.config.VideoBasePath, video.OriginalPath)
	outputDir := filepath.Join(s.config.VideoBasePath, "videos", video.Code)

	// Normalize path
	inputPath = strings.ReplaceAll(inputPath, "\\", "/")
	outputDir = strings.ReplaceAll(outputDir, "\\", "/")

	// ตรวจสอบว่า input file มีอยู่จริง
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		logger.ErrorContext(ctx, "Original video file not found", "video_id", videoID, "path", inputPath)
		s.videoRepo.UpdateStatus(ctx, videoID, models.VideoStatusFailed)
		tracker.FailProgress(video.UserID, videoID, "Original file not found")
		return fmt.Errorf("original file not found: %s", inputPath)
	}

	tracker.UpdateTranscodingProgress(video.UserID, videoID, 10, "กำลังแปลง", "เริ่มแปลงไฟล์")

	// Create progress callback that maps FFmpeg progress (0-100) to our range (10-80)
	progressCallback := func(ffmpegPercent int) {
		// Map 0-100 from FFmpeg to 10-80 in our progress (10% is start, 80% is after FFmpeg)
		mappedPercent := 10 + (ffmpegPercent * 70 / 100)
		tracker.UpdateTranscodingProgress(video.UserID, videoID, mappedPercent, "กำลังแปลง", fmt.Sprintf("แปลงไฟล์: %d%%", ffmpegPercent))
	}

	var result *ports.TranscodeResult

	// เลือกใช้ Adaptive Bitrate หรือ Single Quality
	if s.config.UseAdaptiveBitrate {
		// Adaptive Bitrate Transcoding (multi-quality + H.264 fallback)
		adaptiveOpts := &ports.AdaptiveTranscodeOptions{
			InputPath:    inputPath,
			OutputDir:    outputDir,
			GenerateH264: s.config.GenerateH264Fallback,
			Preset:       s.config.FFmpegPreset,
			SegmentTime:  10,
			OnProgress:   progressCallback,
		}

		if adaptiveOpts.Preset == "" {
			adaptiveOpts.Preset = "medium"
		}

		result, err = s.transcoder.TranscodeAdaptive(ctx, adaptiveOpts)
	} else {
		// Single Quality Transcoding
		// ใช้ H264Config เป็น default (nil = H264Config)
		// เพื่อ browser compatibility - รองรับทุก browser รวมถึง Safari/iOS
		opts := &ports.TranscodeOptions{
			InputPath:   inputPath,
			OutputDir:   filepath.Join(outputDir, "hls"),
			CodecConfig: nil, // default: H264Config (รองรับทุก browser)
			AudioCodec:  "aac",
			Preset:      s.config.FFmpegPreset,
			CRF:         s.config.CRF,
			SegmentTime: 10,
			OnProgress:  progressCallback,
		}

		if opts.Preset == "" {
			opts.Preset = "medium"
		}
		if opts.CRF == 0 {
			opts.CRF = 23 // CRF 23 เหมาะสำหรับ H.264 (18-28 is good range)
		}

		result, err = s.transcoder.Transcode(ctx, opts)
	}

	// Handle transcoding error with retry
	if err != nil {
		logger.ErrorContext(ctx, "Transcoding failed", "video_id", videoID, "attempt", attempt+1, "error", err)

		// Retry if attempts remaining
		if attempt < maxRetries-1 {
			logger.InfoContext(ctx, "Retrying transcoding", "video_id", videoID, "next_attempt", attempt+2)
			tracker.UpdateTranscodingProgress(video.UserID, videoID, 10, "ลองใหม่", fmt.Sprintf("กำลังลองใหม่ (ครั้งที่ %d/%d)", attempt+2, maxRetries))
			return s.processTranscodingWithRetry(ctx, videoID, attempt+1)
		}

		// Max retries exceeded
		s.videoRepo.UpdateStatus(ctx, videoID, models.VideoStatusFailed)
		tracker.FailProgress(video.UserID, videoID, fmt.Sprintf("แปลงไฟล์ล้มเหลวหลังจากลอง %d ครั้ง", maxRetries))
		return fmt.Errorf("transcoding failed after %d attempts: %w", maxRetries, err)
	}

	tracker.UpdateTranscodingProgress(video.UserID, videoID, 80, "กำลังจบ", "แปลงไฟล์เสร็จ กำลังจัดเก็บ")

	// อัพเดท video record
	var relativeHLSPath, relativeH264Path, relativeThumbnailPath string

	if s.config.UseAdaptiveBitrate {
		// Default codec คือ H.264 (ใน AdaptiveTranscodeOptions ถ้าไม่กำหนด CodecConfig)
		// Directory จะเป็น h264/ (ตาม codec type)
		relativeHLSPath = fmt.Sprintf("videos/%s/h264/master.m3u8", video.Code)
		// H.264 fallback จะสร้างเฉพาะเมื่อ primary codec ไม่ใช่ H.264
		if s.config.GenerateH264Fallback && result.HLSPathH264 != "" {
			relativeH264Path = fmt.Sprintf("videos/%s/h264/master.m3u8", video.Code)
		}
		relativeThumbnailPath = fmt.Sprintf("videos/%s/thumbnail.jpg", video.Code)
	} else {
		relativeHLSPath = fmt.Sprintf("videos/%s/hls/master.m3u8", video.Code)
		relativeThumbnailPath = fmt.Sprintf("videos/%s/hls/thumbnail.jpg", video.Code)
	}

	video.HLSPath = relativeHLSPath
	video.HLSPathH264 = relativeH264Path
	video.ThumbnailURL = s.storage.GetFileURL(relativeThumbnailPath)
	video.Duration = result.Duration
	video.Quality = result.Quality
	video.DiskUsage = result.DiskUsage
	video.Status = models.VideoStatusReady

	tracker.UpdateTranscodingProgress(video.UserID, videoID, 90, "บันทึก", "กำลังบันทึกข้อมูลวิดีโอ")

	if err := s.videoRepo.Update(ctx, video); err != nil {
		logger.ErrorContext(ctx, "Failed to update video after transcoding", "video_id", videoID, "error", err)
		tracker.FailProgress(video.UserID, videoID, "บันทึกข้อมูลล้มเหลว")
		return err
	}

	// ลบไฟล์ต้นฉบับถ้าตั้งค่าไว้
	if s.config.CleanupOriginal {
		tracker.UpdateTranscodingProgress(video.UserID, videoID, 95, "ล้างไฟล์", "กำลังลบไฟล์ต้นฉบับ")
		if err := os.Remove(inputPath); err != nil {
			logger.WarnContext(ctx, "Failed to delete original file", "path", inputPath, "error", err)
		} else {
			logger.InfoContext(ctx, "Original file deleted", "path", inputPath)
			s.videoRepo.ClearOriginalPath(ctx, videoID)
		}
	}

	// Mark transcoding as completed
	tracker.CompleteTranscoding(video.UserID, videoID)

	logger.InfoContext(ctx, "Transcoding completed successfully",
		"video_id", videoID,
		"code", video.Code,
		"duration", result.Duration,
		"quality", result.Quality,
		"disk_usage_mb", result.DiskUsage/1024/1024,
		"h264_fallback", relativeH264Path != "",
	)

	return nil
}

// GetQueueStatus ดึงสถานะของ queue (deprecated - ใช้ NATS monitoring แทน)
func (s *TranscodingServiceImpl) GetQueueStatus() *services.TranscodingQueueStatus {
	// สำหรับ NATS distributed workers ใช้ monitoring จาก NATS โดยตรง
	return &services.TranscodingQueueStatus{
		QueueSize:      0,
		WorkersRunning: s.jobQueue != nil,
	}
}

// GetStats ดึงสถิติจำนวนวิดีโอตาม status
func (s *TranscodingServiceImpl) GetStats(ctx context.Context) (*services.TranscodingStats, error) {
	pending, err := s.videoRepo.CountByStatus(ctx, models.VideoStatusPending)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to count pending videos", "error", err)
		return nil, err
	}

	queued, err := s.videoRepo.CountByStatus(ctx, models.VideoStatusQueued)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to count queued videos", "error", err)
		return nil, err
	}

	processing, err := s.videoRepo.CountByStatus(ctx, models.VideoStatusProcessing)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to count processing videos", "error", err)
		return nil, err
	}

	completed, err := s.videoRepo.CountByStatus(ctx, models.VideoStatusReady)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to count completed videos", "error", err)
		return nil, err
	}

	failed, err := s.videoRepo.CountByStatus(ctx, models.VideoStatusFailed)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to count failed videos", "error", err)
		return nil, err
	}

	logger.InfoContext(ctx, "Transcoding stats retrieved",
		"pending", pending,
		"queued", queued,
		"processing", processing,
		"completed", completed,
		"failed", failed,
	)

	return &services.TranscodingStats{
		Pending:    pending,
		Queued:     queued,
		Processing: processing,
		Completed:  completed,
		Failed:     failed,
	}, nil
}

// RecoverStuckJobs กู้คืน jobs ที่ค้างอยู่ในสถานะ processing ตอน server restart
// จะ reset status เป็น pending แล้ว queue ใหม่
func (s *TranscodingServiceImpl) RecoverStuckJobs(ctx context.Context) (int, error) {
	// ดึง videos ที่ค้างอยู่ในสถานะ processing (อาจเกิดจาก server crash)
	stuckVideos, err := s.videoRepo.GetByStatus(ctx, models.VideoStatusProcessing, 0, 100)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get stuck videos", "error", err)
		return 0, err
	}

	if len(stuckVideos) == 0 {
		logger.InfoContext(ctx, "No stuck videos found")
		return 0, nil
	}

	logger.InfoContext(ctx, "Found stuck videos to recover", "count", len(stuckVideos))

	recovered := 0
	for _, video := range stuckVideos {
		// Reset status กลับเป็น pending
		if err := s.videoRepo.UpdateStatus(ctx, video.ID, models.VideoStatusPending); err != nil {
			logger.ErrorContext(ctx, "Failed to reset video status", "video_id", video.ID, "error", err)
			continue
		}

		// Queue ใหม่สำหรับ transcoding
		if err := s.QueueTranscoding(ctx, video.ID); err != nil {
			logger.ErrorContext(ctx, "Failed to re-queue video", "video_id", video.ID, "error", err)
			continue
		}

		logger.InfoContext(ctx, "Video recovered and re-queued", "video_id", video.ID, "code", video.Code)
		recovered++
	}

	logger.InfoContext(ctx, "Stuck videos recovery completed", "total", len(stuckVideos), "recovered", recovered)
	return recovered, nil
}

// checkQueueOverflow ตรวจสอบว่าคิวเต็มหรือไม่
// ถ้าคิวเต็มจะ return error พร้อมข้อความบอกผู้ใช้
func (s *TranscodingServiceImpl) checkQueueOverflow(ctx context.Context) error {
	// ดึงค่า max_queue_size จาก settings (default: 100, 0 = unlimited)
	maxQueueSize := 100
	if s.settingService != nil {
		maxQueueSize = s.settingService.GetInt(ctx, "transcoding", "max_queue_size", 100)
	}

	// ถ้า maxQueueSize = 0 หมายถึงไม่จำกัด
	if maxQueueSize == 0 {
		return nil
	}

	// ดึงสถานะ queue
	status, err := s.jobQueue.GetQueueStatus(ctx)
	if err != nil {
		// ถ้าดึง status ไม่ได้ ให้ผ่านไปก่อน (don't block on monitoring failure)
		logger.WarnContext(ctx, "Failed to get queue status for overflow check", "error", err)
		return nil
	}

	// ตรวจสอบว่า pending jobs เกิน limit หรือไม่
	if status.PendingJobs >= uint64(maxQueueSize) {
		logger.WarnContext(ctx, "Queue overflow detected",
			"pending_jobs", status.PendingJobs,
			"max_queue_size", maxQueueSize,
		)
		return fmt.Errorf("ระบบกำลังยุ่ง มี %d งานรอประมวลผลอยู่ กรุณาลองใหม่ภายหลัง", status.PendingJobs)
	}

	return nil
}

// getDefaultQualities ดึงค่า default qualities จาก Settings
// ถ้าไม่มีหรือผิดพลาดจะใช้ค่า default "1080p,720p,480p"
func (s *TranscodingServiceImpl) getDefaultQualities(ctx context.Context) []string {
	defaultQualities := []string{"1080p", "720p", "480p"}

	if s.settingService == nil {
		logger.WarnContext(ctx, "SettingService is nil, using default qualities", "qualities", defaultQualities)
		return defaultQualities
	}

	qualitiesStr, err := s.settingService.Get(ctx, "transcoding", "default_qualities")
	logger.InfoContext(ctx, "Fetched qualities from settings",
		"raw_value", qualitiesStr,
		"error", err,
	)

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
