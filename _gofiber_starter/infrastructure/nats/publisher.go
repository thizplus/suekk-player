package nats

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
)

// Publisher publishes transcode jobs to JetStream
type Publisher struct {
	client *Client
}

// NewPublisher สร้าง Publisher ใหม่
func NewPublisher(client *Client) *Publisher {
	return &Publisher{
		client: client,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Publish Methods
// ═══════════════════════════════════════════════════════════════════════════════

// PublishTranscodeJob ส่ง transcode job ไปยัง JetStream
func (p *Publisher) PublishTranscodeJob(ctx context.Context, job *TranscodeJob) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// Publish to JetStream
	ack, err := p.client.js.Publish(ctx, SubjectJobs, data)
	if err != nil {
		logger.Error("Failed to publish transcode job",
			"video_id", job.VideoID,
			"error", err,
		)
		return fmt.Errorf("failed to publish job: %w", err)
	}

	logger.Info("Transcode job published to JetStream",
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
// Subtitle Job Publishing (implements services.SubtitleJobPublisher)
// ═══════════════════════════════════════════════════════════════════════════════

// PublishDetectJob ส่ง detect language job ไปยัง NATS
func (p *Publisher) PublishDetectJob(ctx context.Context, job *services.DetectJob) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal detect job: %w", err)
	}

	// Publish to JetStream
	ack, err := p.client.js.Publish(ctx, SubjectSubtitleDetect, data)
	if err != nil {
		logger.Error("Failed to publish detect job",
			"video_id", job.VideoID,
			"error", err,
		)
		return fmt.Errorf("failed to publish detect job: %w", err)
	}

	logger.Info("Detect job published to JetStream",
		"video_id", job.VideoID,
		"video_code", job.VideoCode,
		"stream", ack.Stream,
		"sequence", ack.Sequence,
	)

	return nil
}

// PublishTranscribeJob ส่ง transcribe job ไปยัง NATS
func (p *Publisher) PublishTranscribeJob(ctx context.Context, job *services.TranscribeJob) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal transcribe job: %w", err)
	}

	// Publish to JetStream
	ack, err := p.client.js.Publish(ctx, SubjectSubtitleTranscribe, data)
	if err != nil {
		logger.Error("Failed to publish transcribe job",
			"subtitle_id", job.SubtitleID,
			"video_id", job.VideoID,
			"error", err,
		)
		return fmt.Errorf("failed to publish transcribe job: %w", err)
	}

	logger.Info("Transcribe job published to JetStream",
		"subtitle_id", job.SubtitleID,
		"video_id", job.VideoID,
		"language", job.Language,
		"stream", ack.Stream,
		"sequence", ack.Sequence,
	)

	return nil
}

// PublishTranslateJob ส่ง translate job ไปยัง NATS
func (p *Publisher) PublishTranslateJob(ctx context.Context, job *services.TranslateJob) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal translate job: %w", err)
	}

	// Publish to JetStream
	ack, err := p.client.js.Publish(ctx, SubjectSubtitleTranslate, data)
	if err != nil {
		logger.Error("Failed to publish translate job",
			"video_id", job.VideoID,
			"target_languages", job.TargetLanguages,
			"error", err,
		)
		return fmt.Errorf("failed to publish translate job: %w", err)
	}

	logger.Info("Translate job published to JetStream",
		"video_id", job.VideoID,
		"source_language", job.SourceLanguage,
		"target_languages", job.TargetLanguages,
		"stream", ack.Stream,
		"sequence", ack.Sequence,
	)

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Warm Cache Job Publishing
// ═══════════════════════════════════════════════════════════════════════════════

// PublishWarmCacheJob ส่ง warm cache job ไปยัง NATS
func (p *Publisher) PublishWarmCacheJob(ctx context.Context, job *WarmCacheJob) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal warm cache job: %w", err)
	}

	// Publish to JetStream
	ack, err := p.client.js.Publish(ctx, SubjectWarmCache, data)
	if err != nil {
		logger.Error("Failed to publish warm cache job",
			"video_id", job.VideoID,
			"video_code", job.VideoCode,
			"error", err,
		)
		return fmt.Errorf("failed to publish warm cache job: %w", err)
	}

	logger.Info("Warm cache job published to JetStream",
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
// Reel Export Job Publishing (implements services.ReelJobPublisher)
// ═══════════════════════════════════════════════════════════════════════════════

// PublishReelExportJob ส่ง reel export job ไปยัง NATS
func (p *Publisher) PublishReelExportJob(ctx context.Context, job *ReelExportJob) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal reel export job: %w", err)
	}

	// Publish to JetStream
	ack, err := p.client.js.Publish(ctx, SubjectReelExport, data)
	if err != nil {
		logger.Error("Failed to publish reel export job",
			"reel_id", job.ReelID,
			"video_code", job.VideoCode,
			"error", err,
		)
		return fmt.Errorf("failed to publish reel export job: %w", err)
	}

	logger.Info("Reel export job published to JetStream",
		"reel_id", job.ReelID,
		"video_id", job.VideoID,
		"video_code", job.VideoCode,
		"segment", fmt.Sprintf("%.2f-%.2f", job.SegmentStart, job.SegmentEnd),
		"layers", len(job.Layers),
		"stream", ack.Stream,
		"sequence", ack.Sequence,
	)

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Gallery Job Publishing
// ═══════════════════════════════════════════════════════════════════════════════

// PublishGalleryJob ส่ง gallery generate job ไปยัง NATS
func (p *Publisher) PublishGalleryJob(ctx context.Context, job *GalleryJob) error {
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal gallery job: %w", err)
	}

	// Publish to JetStream
	ack, err := p.client.js.Publish(ctx, SubjectGalleryGenerate, data)
	if err != nil {
		logger.Error("Failed to publish gallery job",
			"video_id", job.VideoID,
			"video_code", job.VideoCode,
			"error", err,
		)
		return fmt.Errorf("failed to publish gallery job: %w", err)
	}

	logger.Info("Gallery job published to JetStream",
		"video_id", job.VideoID,
		"video_code", job.VideoCode,
		"hls_path", job.HLSPath,
		"image_count", job.ImageCount,
		"stream", ack.Stream,
		"sequence", ack.Sequence,
	)

	return nil
}
