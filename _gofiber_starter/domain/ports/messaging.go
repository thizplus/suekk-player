package ports

import "context"

// ═══════════════════════════════════════════════════════════════════════════════
// Job Queue Port - สำหรับส่ง/รับ Transcode Jobs
// ═══════════════════════════════════════════════════════════════════════════════

// TranscodeJobData - Plain struct (ไม่มี NATS dependency)
type TranscodeJobData struct {
	VideoID      string
	VideoCode    string
	InputPath    string
	OutputPath   string
	Codec        string
	Qualities    []string
	UseByteRange bool
}

// QueueStatus - สถานะของ job queue
type QueueStatus struct {
	StreamName  string
	PendingJobs uint64
	AckPending  uint64
	TotalBytes  uint64
}

// JobQueuePort - Interface สำหรับ Job Queue
type JobQueuePort interface {
	// PublishJob ส่ง transcode job เข้า queue
	PublishJob(ctx context.Context, job *TranscodeJobData) error

	// GetQueueStatus ดึงสถานะ queue (pending jobs, etc.)
	GetQueueStatus(ctx context.Context) (*QueueStatus, error)
}

// ═══════════════════════════════════════════════════════════════════════════════
// Progress Publisher Port - สำหรับส่ง Progress Updates
// ═══════════════════════════════════════════════════════════════════════════════

// ProgressData - Plain struct สำหรับ progress update
type ProgressData struct {
	VideoID    string
	VideoCode  string
	Status     string  // "processing", "completed", "failed"
	Stage      string  // Subtitle: downloading, transcribing, generating, etc.
	Progress   float64 // 0-100
	Quality    string
	Message    string
	Error      string
	OutputPath string
	AudioPath  string // S3 path to extracted audio (WAV)
	WorkerID   string // Worker ที่ส่ง message นี้

	// Subtitle-specific fields
	SubtitleID      string
	CurrentLanguage string

	// Reel-specific fields
	ReelID   string
	FileSize int64
}

// ProgressPublisherPort - Interface สำหรับส่ง progress
type ProgressPublisherPort interface {
	// PublishProgress ส่ง progress update
	PublishProgress(ctx context.Context, progress *ProgressData) error
}

// ═══════════════════════════════════════════════════════════════════════════════
// Progress Subscriber Port - สำหรับรับ Progress Updates
// ═══════════════════════════════════════════════════════════════════════════════

// ProgressHandler - Callback function type
type ProgressHandler func(progress *ProgressData)

// ProgressSubscriberPort - Interface สำหรับ subscribe progress
// รับ ctx เพื่อให้ cancel subscription ผ่าน context ได้
type ProgressSubscriberPort interface {
	// Subscribe เริ่ม listen progress updates
	Subscribe(ctx context.Context, handler ProgressHandler) error

	// Unsubscribe หยุด listen
	Unsubscribe() error
}
