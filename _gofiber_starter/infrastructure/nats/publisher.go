package nats

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
	"gorm.io/datatypes"
)

// Publisher publishes transcode jobs to JetStream
type Publisher struct {
	client           *Client
	workerJobService services.WorkerJobService // For Dual Write - track jobs in DB
}

// NewPublisher สร้าง Publisher ใหม่
func NewPublisher(client *Client) *Publisher {
	return &Publisher{
		client: client,
	}
}

// SetWorkerJobService sets the WorkerJobService for Dual Write
// Called after services are initialized in container.go
func (p *Publisher) SetWorkerJobService(svc services.WorkerJobService) {
	p.workerJobService = svc
	logger.Info("WorkerJobService injected into Publisher (Dual Write enabled)")
}

// ═══════════════════════════════════════════════════════════════════════════════
// Job Creation Helper (Create BEFORE Publish to get job_id)
// ═══════════════════════════════════════════════════════════════════════════════

// createWorkerJob creates a WorkerJob record BEFORE publishing
// Returns the job_id to include in the NATS message
func (p *Publisher) createWorkerJob(ctx context.Context, jobType models.WorkerJobType, entityType models.WorkerEntityType, entityID uuid.UUID, entityCode string, priority int, jobData interface{}) (*models.WorkerJob, error) {
	if p.workerJobService == nil {
		return nil, fmt.Errorf("WorkerJobService not initialized")
	}

	// Serialize job data to JSON
	jobDataJSON, err := json.Marshal(jobData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job data: %w", err)
	}

	req := &dto.CreateWorkerJobRequest{
		JobType:    jobType,
		EntityType: entityType,
		EntityID:   entityID,
		EntityCode: entityCode,
		Priority:   priority,
		MaxRetries: 3,
		JobData:    datatypes.JSON(jobDataJSON),
	}

	job, err := p.workerJobService.CreateJob(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create WorkerJob: %w", err)
	}

	logger.InfoContext(ctx, "WorkerJob created",
		"job_id", job.ID,
		"job_type", jobType,
		"entity_type", entityType,
		"entity_id", entityID,
	)

	return job, nil
}

// markJobAsQueued marks job as queued after successful NATS publish
func (p *Publisher) markJobAsQueued(ctx context.Context, jobID uuid.UUID) {
	if p.workerJobService == nil {
		return
	}

	if err := p.workerJobService.MarkAsQueued(ctx, jobID); err != nil {
		logger.WarnContext(ctx, "Failed to mark WorkerJob as queued",
			"job_id", jobID,
			"error", err,
		)
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Publish Methods (New Standard Format with _meta wrapper)
// ═══════════════════════════════════════════════════════════════════════════════

// PublishTranscodeJob ส่ง transcode job ไปยัง JetStream (NEW FORMAT)
func (p *Publisher) PublishTranscodeJob(ctx context.Context, job *TranscodeJob) error {
	// Parse video ID
	videoID, err := uuid.Parse(job.VideoID)
	if err != nil {
		return fmt.Errorf("invalid video_id: %w", err)
	}

	// Create input for new format
	input := TranscodeInput{
		InputPath:    job.InputPath,
		OutputPath:   job.OutputPath,
		Codec:        job.Codec,
		Qualities:    job.Qualities,
		UseByteRange: job.UseByteRange,
	}

	// Create WorkerJob FIRST to get job_id
	workerJob, err := p.createWorkerJob(ctx, models.WorkerJobTypeTranscode, models.WorkerEntityTypeVideo, videoID, job.VideoCode, 1, input)
	if err != nil {
		return fmt.Errorf("failed to create worker job: %w", err)
	}

	// Create new format message with _meta wrapper
	msg := NewTranscodeMessage(workerJob.ID, videoID, job.VideoCode, input, 1, 0, 3)

	// Marshal and publish
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	ack, err := p.client.js.Publish(ctx, SubjectJobs, data)
	if err != nil {
		logger.Error("Failed to publish transcode job",
			"job_id", workerJob.ID,
			"video_id", job.VideoID,
			"error", err,
		)
		return fmt.Errorf("failed to publish job: %w", err)
	}

	// Mark as queued after successful publish
	p.markJobAsQueued(ctx, workerJob.ID)

	logger.Info("Transcode job published (new format)",
		"job_id", workerJob.ID,
		"video_id", job.VideoID,
		"video_code", job.VideoCode,
		"stream", ack.Stream,
		"sequence", ack.Sequence,
	)

	return nil
}

// EnqueueTranscode helper method ที่รับ parameters แยก (เหมือน Asynq เดิม)
func (p *Publisher) EnqueueTranscode(ctx context.Context, videoID, videoCode, inputPath, outputPath, codec string, qualities []string, useByteRange bool) error {
	job := NewTranscodeJob(videoID, videoCode, inputPath, outputPath, codec, qualities, useByteRange)
	return p.PublishTranscodeJob(ctx, job)
}

// ═══════════════════════════════════════════════════════════════════════════════
// Queue Inspection
// ═══════════════════════════════════════════════════════════════════════════════

// GetQueueStats ดึงสถิติของ queue (เหมือน Asynq)
type QueueStats struct {
	Pending   uint64 `json:"pending"`
	Active    int    `json:"active"`
	Completed uint64 `json:"completed"`
}

func (p *Publisher) GetQueueStats(ctx context.Context) (*QueueStats, error) {
	streamInfo, err := p.client.stream.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get stream info: %w", err)
	}

	var numAckPending int
	consumer, err := p.client.stream.Consumer(ctx, ConsumerName)
	if err == nil {
		ci, err := consumer.Info(ctx)
		if err == nil {
			numAckPending = ci.NumAckPending
		}
	}

	return &QueueStats{
		Pending:   streamInfo.State.Msgs,
		Active:    numAckPending,
		Completed: streamInfo.State.LastSeq - streamInfo.State.Msgs,
	}, nil
}

// GetJetStreamStatus ดึงสถานะ JetStream (สำหรับ Monitoring API)
func (p *Publisher) GetJetStreamStatus(ctx context.Context) (*JetStreamStatus, error) {
	return p.client.GetStatus(ctx)
}

// ═══════════════════════════════════════════════════════════════════════════════
// Job Management
// ═══════════════════════════════════════════════════════════════════════════════

// PurgeStream ลบทุก messages ใน stream (ใช้ตอน debug)
func (p *Publisher) PurgeStream(ctx context.Context) error {
	return p.client.stream.Purge(ctx, jetstream.WithPurgeSubject(SubjectJobs))
}

// ═══════════════════════════════════════════════════════════════════════════════
// Worker Status (จาก Heartbeat KV)
// ═══════════════════════════════════════════════════════════════════════════════

// WorkerStatus สถานะของ Worker (ตรงกับ heartbeat.WorkerStatus)
type WorkerStatus struct {
	WorkerID    string       `json:"worker_id"`
	WorkerType  string       `json:"worker_type"` // transcode, subtitle
	Hostname    string       `json:"hostname"`
	InternalIP  string       `json:"internal_ip"`
	StartedAt   string       `json:"started_at"`
	LastSeen    string       `json:"last_seen"`
	Status      string       `json:"status"` // idle, processing, stopping, paused
	CurrentJobs []WorkerJob  `json:"current_jobs"`
	Stats       WorkerStats  `json:"stats"`
	Config      WorkerConfig `json:"config"`
	Disk        DiskStatus   `json:"disk"`
}

// DiskStatus สถานะ disk ของ Worker
type DiskStatus struct {
	UsagePercent float64 `json:"usage_percent"`
	TotalGB      float64 `json:"total_gb"`
	FreeGB       float64 `json:"free_gb"`
	UsedGB       float64 `json:"used_gb"`
	Level        string  `json:"level"`     // normal, warning, caution, critical
	IsPaused     bool    `json:"is_paused"` // job paused due to disk
}

// WorkerJob งานที่ Worker กำลังทำ
type WorkerJob struct {
	VideoID   string  `json:"video_id"`
	VideoCode string  `json:"video_code"`
	Title     string  `json:"title"`
	Progress  float64 `json:"progress"`
	Stage     string  `json:"stage"`
	StartedAt string  `json:"started_at"`
	ETA       string  `json:"eta"`
}

// WorkerStats สถิติของ Worker
type WorkerStats struct {
	TotalProcessed int            `json:"total_processed"`
	TotalFailed    int            `json:"total_failed"`
	TotalRetries   int            `json:"total_retries"`
	UptimeSeconds  int64          `json:"uptime_seconds"`
	RecentJobs     []CompletedJob `json:"recent_jobs,omitempty"`
}

// CompletedJob งานที่เสร็จแล้ว (สำหรับ history)
type CompletedJob struct {
	VideoID     string  `json:"video_id"`
	VideoCode   string  `json:"video_code"`
	Title       string  `json:"title"`
	Status      string  `json:"status"`                 // success, failed
	DurationSec float64 `json:"duration_sec,omitempty"` // เวลาที่ใช้ประมวลผล
	CompletedAt string  `json:"completed_at"`
	Error       string  `json:"error,omitempty"`
	JobType     string  `json:"job_type,omitempty"` // สำหรับ subtitle: transcribe, translate
}

// WorkerConfig การตั้งค่าของ Worker
type WorkerConfig struct {
	GPUEnabled  bool   `json:"gpu_enabled"`
	Concurrency int    `json:"concurrency"`
	Preset      string `json:"preset"`
}

// GetAllWorkers ดึงสถานะของ Workers ทั้งหมด
func (p *Publisher) GetAllWorkers(ctx context.Context) ([]WorkerStatus, error) {
	kv := p.client.WorkerKV()
	if kv == nil {
		// พยายาม refresh ก่อน
		if err := p.client.RefreshWorkerKV(ctx); err != nil {
			return []WorkerStatus{}, nil // คืน empty array แทน error
		}
		kv = p.client.WorkerKV()
		if kv == nil {
			return []WorkerStatus{}, nil
		}
	}

	// Get all keys
	keys, err := kv.Keys(ctx)
	if err != nil {
		// ถ้าไม่มี keys เลย จะ error - คืน empty array
		return []WorkerStatus{}, nil
	}

	var workers []WorkerStatus
	for _, key := range keys {
		entry, err := kv.Get(ctx, key)
		if err != nil {
			continue
		}

		var status WorkerStatus
		if err := json.Unmarshal(entry.Value(), &status); err != nil {
			continue
		}
		workers = append(workers, status)
	}

	return workers, nil
}

// GetWorkerCount ดึงจำนวน Workers ที่ online
func (p *Publisher) GetWorkerCount(ctx context.Context) int {
	workers, _ := p.GetAllWorkers(ctx)
	return len(workers)
}

// ═══════════════════════════════════════════════════════════════════════════════
// Subtitle Job Publishing (implements services.SubtitleJobPublisher) - NEW FORMAT
// ═══════════════════════════════════════════════════════════════════════════════

// PublishDetectJob ส่ง detect language job ไปยัง NATS (NEW FORMAT)
func (p *Publisher) PublishDetectJob(ctx context.Context, job *services.DetectJob) error {
	// Parse video ID
	videoID, err := uuid.Parse(job.VideoID)
	if err != nil {
		return fmt.Errorf("invalid video_id: %w", err)
	}

	// Create input for new format
	input := SubtitleDetectInput{
		AudioPath: job.AudioPath,
	}

	// Create WorkerJob FIRST to get job_id
	workerJob, err := p.createWorkerJob(ctx, models.WorkerJobTypeSubtitleDetect, models.WorkerEntityTypeVideo, videoID, job.VideoCode, 2, input)
	if err != nil {
		return fmt.Errorf("failed to create worker job: %w", err)
	}

	// Create new format message with _meta wrapper
	msg := NewSubtitleDetectMessage(workerJob.ID, videoID, job.VideoCode, input, 2, 0, 3)

	// Marshal and publish
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal detect job: %w", err)
	}

	ack, err := p.client.js.Publish(ctx, SubjectSubtitleDetect, data)
	if err != nil {
		logger.Error("Failed to publish detect job",
			"job_id", workerJob.ID,
			"video_id", job.VideoID,
			"error", err,
		)
		return fmt.Errorf("failed to publish detect job: %w", err)
	}

	// Mark as queued
	p.markJobAsQueued(ctx, workerJob.ID)

	logger.Info("Detect job published (new format)",
		"job_id", workerJob.ID,
		"video_id", job.VideoID,
		"video_code", job.VideoCode,
		"stream", ack.Stream,
		"sequence", ack.Sequence,
	)

	return nil
}

// PublishTranscribeJob ส่ง transcribe job ไปยัง NATS (NEW FORMAT)
func (p *Publisher) PublishTranscribeJob(ctx context.Context, job *services.TranscribeJob) error {
	// Parse subtitle ID (entity is subtitle for transcribe)
	subtitleID, err := uuid.Parse(job.SubtitleID)
	if err != nil {
		return fmt.Errorf("invalid subtitle_id: %w", err)
	}

	// Create input for new format
	input := SubtitleTranscribeInput{
		VideoID:       job.VideoID,
		VideoCode:     job.VideoCode,
		AudioPath:     job.AudioPath,
		Language:      job.Language,
		OutputPath:    job.OutputPath,
		RefineWithLLM: job.RefineWithLLM,
		Context:       job.Context,
	}

	// Create WorkerJob FIRST to get job_id
	workerJob, err := p.createWorkerJob(ctx, models.WorkerJobTypeSubtitleTranscribe, models.WorkerEntityTypeSubtitle, subtitleID, job.VideoCode, 2, input)
	if err != nil {
		return fmt.Errorf("failed to create worker job: %w", err)
	}

	// Create new format message with _meta wrapper
	msg := NewSubtitleTranscribeMessage(workerJob.ID, subtitleID, job.VideoCode, input, 2, 0, 3)

	// Marshal and publish
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal transcribe job: %w", err)
	}

	ack, err := p.client.js.Publish(ctx, SubjectSubtitleTranscribe, data)
	if err != nil {
		logger.Error("Failed to publish transcribe job",
			"job_id", workerJob.ID,
			"subtitle_id", job.SubtitleID,
			"video_id", job.VideoID,
			"error", err,
		)
		return fmt.Errorf("failed to publish transcribe job: %w", err)
	}

	// Mark as queued
	p.markJobAsQueued(ctx, workerJob.ID)

	logger.Info("Transcribe job published (new format)",
		"job_id", workerJob.ID,
		"subtitle_id", job.SubtitleID,
		"video_id", job.VideoID,
		"language", job.Language,
		"stream", ack.Stream,
		"sequence", ack.Sequence,
	)

	return nil
}

// PublishTranslateJob ส่ง translate job ไปยัง NATS (NEW FORMAT)
func (p *Publisher) PublishTranslateJob(ctx context.Context, job *services.TranslateJob) error {
	// Parse video ID (entity is video since translate affects multiple subtitles)
	videoID, err := uuid.Parse(job.VideoID)
	if err != nil {
		return fmt.Errorf("invalid video_id: %w", err)
	}

	// Create input for new format
	input := SubtitleTranslateInput{
		SubtitleIDs:     job.SubtitleIDs,
		VideoID:         job.VideoID,
		VideoCode:       job.VideoCode,
		SourceSRTPath:   job.SourceSRTPath,
		SourceLanguage:  job.SourceLanguage,
		TargetLanguages: job.TargetLanguages,
		OutputPath:      job.OutputPath,
		Context:         job.Context,
	}

	// Create WorkerJob FIRST to get job_id
	workerJob, err := p.createWorkerJob(ctx, models.WorkerJobTypeSubtitleTranslate, models.WorkerEntityTypeVideo, videoID, job.VideoCode, 2, input)
	if err != nil {
		return fmt.Errorf("failed to create worker job: %w", err)
	}

	// Create new format message with _meta wrapper
	msg := NewSubtitleTranslateMessage(workerJob.ID, videoID, job.VideoCode, input, 2, 0, 3)

	// Marshal and publish
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal translate job: %w", err)
	}

	ack, err := p.client.js.Publish(ctx, SubjectSubtitleTranslate, data)
	if err != nil {
		logger.Error("Failed to publish translate job",
			"job_id", workerJob.ID,
			"video_id", job.VideoID,
			"target_languages", job.TargetLanguages,
			"error", err,
		)
		return fmt.Errorf("failed to publish translate job: %w", err)
	}

	// Mark as queued
	p.markJobAsQueued(ctx, workerJob.ID)

	logger.Info("Translate job published (new format)",
		"job_id", workerJob.ID,
		"video_id", job.VideoID,
		"source_language", job.SourceLanguage,
		"target_languages", job.TargetLanguages,
		"stream", ack.Stream,
		"sequence", ack.Sequence,
	)

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Warm Cache Job Publishing - NEW FORMAT
// ═══════════════════════════════════════════════════════════════════════════════

// PublishWarmCacheJob ส่ง warm cache job ไปยัง NATS (NEW FORMAT)
func (p *Publisher) PublishWarmCacheJob(ctx context.Context, job *WarmCacheJob) error {
	// Parse video ID
	videoID, err := uuid.Parse(job.VideoID)
	if err != nil {
		return fmt.Errorf("invalid video_id: %w", err)
	}

	// Create input for new format
	input := WarmCacheInput{
		HLSPath:       job.HLSPath,
		SegmentCounts: job.SegmentCounts,
		Priority:      job.Priority,
	}

	// Create WorkerJob FIRST to get job_id
	workerJob, err := p.createWorkerJob(ctx, models.WorkerJobTypeWarmCache, models.WorkerEntityTypeVideo, videoID, job.VideoCode, 3, input)
	if err != nil {
		return fmt.Errorf("failed to create worker job: %w", err)
	}

	// Create new format message with _meta wrapper
	msg := NewWarmCacheMessage(workerJob.ID, videoID, job.VideoCode, input, 3, 0, 3)

	// Marshal and publish
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal warm cache job: %w", err)
	}

	ack, err := p.client.js.Publish(ctx, SubjectWarmCache, data)
	if err != nil {
		logger.Error("Failed to publish warm cache job",
			"job_id", workerJob.ID,
			"video_id", job.VideoID,
			"video_code", job.VideoCode,
			"error", err,
		)
		return fmt.Errorf("failed to publish warm cache job: %w", err)
	}

	// Mark as queued
	p.markJobAsQueued(ctx, workerJob.ID)

	logger.Info("Warm cache job published (new format)",
		"job_id", workerJob.ID,
		"video_id", job.VideoID,
		"video_code", job.VideoCode,
		"hls_path", job.HLSPath,
		"priority", job.Priority,
		"stream", ack.Stream,
		"sequence", ack.Sequence,
	)

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Reel Export Job Publishing (implements services.ReelJobPublisher) - NEW FORMAT
// ═══════════════════════════════════════════════════════════════════════════════

// PublishReelExportJob ส่ง reel export job ไปยัง NATS (NEW FORMAT)
func (p *Publisher) PublishReelExportJob(ctx context.Context, job *ReelExportJob) error {
	// Parse reel ID
	reelID, err := uuid.Parse(job.ReelID)
	if err != nil {
		return fmt.Errorf("invalid reel_id: %w", err)
	}

	// Convert segments
	var segments []VideoSegment
	for _, s := range job.Segments {
		segments = append(segments, VideoSegment{Start: s.Start, End: s.End})
	}

	// Create input for new format
	input := ReelInput{
		VideoID:      job.VideoID,
		VideoCode:    job.VideoCode,
		HLSPath:      job.HLSPath,
		VideoQuality: job.VideoQuality,
		Segments:     segments,
		SegmentStart: job.SegmentStart,
		SegmentEnd:   job.SegmentEnd,
		CoverTime:    job.CoverTime,
		Style:        job.Style,
		Title:        job.Title,
		Line1:        job.Line1,
		Line2:        job.Line2,
		ShowLogo:     job.ShowLogo,
		LogoPath:     job.LogoPath,
		GradientPath: job.GradientPath,
		CropX:        job.CropX,
		CropY:        job.CropY,
		TTSText:      job.TTSText,
		TTSVoice:     job.TTSVoice,
		OutputPath:   job.OutputPath,
	}

	// Create WorkerJob FIRST to get job_id
	workerJob, err := p.createWorkerJob(ctx, models.WorkerJobTypeReel, models.WorkerEntityTypeReel, reelID, job.VideoCode, 2, input)
	if err != nil {
		return fmt.Errorf("failed to create worker job: %w", err)
	}

	// Create new format message with _meta wrapper
	msg := NewReelMessage(workerJob.ID, reelID, job.VideoCode, input, 2, 0, 3)

	// Marshal and publish
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal reel export job: %w", err)
	}

	ack, err := p.client.js.Publish(ctx, SubjectReelExport, data)
	if err != nil {
		logger.Error("Failed to publish reel export job",
			"job_id", workerJob.ID,
			"reel_id", job.ReelID,
			"video_code", job.VideoCode,
			"error", err,
		)
		return fmt.Errorf("failed to publish reel export job: %w", err)
	}

	// Mark as queued
	p.markJobAsQueued(ctx, workerJob.ID)

	logger.Info("Reel export job published (new format)",
		"job_id", workerJob.ID,
		"reel_id", job.ReelID,
		"video_id", job.VideoID,
		"video_code", job.VideoCode,
		"segment", fmt.Sprintf("%.2f-%.2f", job.SegmentStart, job.SegmentEnd),
		"stream", ack.Stream,
		"sequence", ack.Sequence,
	)

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Gallery Job Publishing - NEW FORMAT
// ═══════════════════════════════════════════════════════════════════════════════

// PublishGalleryJob ส่ง gallery generate job ไปยัง NATS (NEW FORMAT)
func (p *Publisher) PublishGalleryJob(ctx context.Context, job *GalleryJob) error {
	// Parse video ID
	videoID, err := uuid.Parse(job.VideoID)
	if err != nil {
		return fmt.Errorf("invalid video_id: %w", err)
	}

	// Create input for new format
	input := GalleryInput{
		HLSPath:      job.HLSPath,
		VideoQuality: job.VideoQuality,
		Duration:     job.Duration,
		OutputPath:   job.OutputPath,
		ImageCount:   job.ImageCount,
	}

	// Create WorkerJob FIRST to get job_id
	workerJob, err := p.createWorkerJob(ctx, models.WorkerJobTypeGallery, models.WorkerEntityTypeVideo, videoID, job.VideoCode, 2, input)
	if err != nil {
		return fmt.Errorf("failed to create worker job: %w", err)
	}

	// Create new format message with _meta wrapper
	msg := NewGalleryMessage(workerJob.ID, videoID, job.VideoCode, input, 2, 0, 3)

	// Marshal and publish
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal gallery job: %w", err)
	}

	ack, err := p.client.js.Publish(ctx, SubjectGalleryGenerate, data)
	if err != nil {
		logger.Error("Failed to publish gallery job",
			"job_id", workerJob.ID,
			"video_id", job.VideoID,
			"video_code", job.VideoCode,
			"error", err,
		)
		return fmt.Errorf("failed to publish gallery job: %w", err)
	}

	// Mark as queued
	p.markJobAsQueued(ctx, workerJob.ID)

	logger.Info("Gallery job published (new format)",
		"job_id", workerJob.ID,
		"video_id", job.VideoID,
		"video_code", job.VideoCode,
		"hls_path", job.HLSPath,
		"image_count", job.ImageCount,
		"stream", ack.Stream,
		"sequence", ack.Sequence,
	)

	return nil
}
