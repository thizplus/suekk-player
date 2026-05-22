package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ═══════════════════════════════════════════════════════════════════════════════
// WorkerJob - Job Queue Model
// Tracks all worker jobs (transcode, gallery, subtitle, reel, warmcache)
// ═══════════════════════════════════════════════════════════════════════════════

type WorkerJob struct {
	// Identity
	ID      uuid.UUID     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	JobType WorkerJobType `gorm:"column:job_type;size:50;not null;index:idx_worker_jobs_type_status"`

	// Entity Reference (polymorphic)
	EntityType WorkerEntityType `gorm:"column:entity_type;size:50;not null;index:idx_worker_jobs_entity"` // video, reel, subtitle
	EntityID   uuid.UUID        `gorm:"type:uuid;not null;index:idx_worker_jobs_entity"`                  // FK to video/reel/subtitle
	EntityCode string           `gorm:"size:50"`                                                          // video_code for logging

	// Status & Progress
	Status       WorkerJobStatus `gorm:"size:20;not null;default:'pending';index:idx_worker_jobs_type_status;index:idx_worker_jobs_status"`
	Progress     float64         `gorm:"default:0"`   // 0-100
	CurrentStage string          `gorm:"size:100"`    // downloading, transcoding, uploading
	Message      string          `gorm:"type:text"`   // Human readable status

	// Worker Assignment
	WorkerID string `gorm:"size:100;index:idx_worker_jobs_worker"` // Which worker is processing
	Priority int    `gorm:"default:2"`                             // 1=high, 2=normal, 3=low

	// Timing
	CreatedAt   time.Time  `gorm:"index:idx_worker_jobs_created"`
	QueuedAt    *time.Time `gorm:"type:timestamptz"` // When published to NATS
	StartedAt   *time.Time `gorm:"type:timestamptz;index:idx_worker_jobs_stuck"` // When worker picked up
	CompletedAt *time.Time `gorm:"type:timestamptz"` // When finished

	// Retry
	RetryCount  int        `gorm:"default:0"`
	MaxRetries  int        `gorm:"default:3"`
	NextRetryAt *time.Time `gorm:"type:timestamptz;index:idx_worker_jobs_retry"` // For delayed retry

	// Data (JSONB) - flexible per job type
	JobData    datatypes.JSON `gorm:"type:jsonb;not null"` // Input: TranscodeJob, GalleryJob, etc.
	OutputData datatypes.JSON `gorm:"type:jsonb"`          // Output: Result data

	// Errors
	LastError    string         `gorm:"type:text"`
	ErrorHistory datatypes.JSON `gorm:"type:jsonb;default:'[]'"` // Array of ErrorRecord
}

// TableName returns the table name
func (WorkerJob) TableName() string {
	return "worker_jobs"
}

// ═══════════════════════════════════════════════════════════════════════════════
// ErrorRecord - Error history entry
// ═══════════════════════════════════════════════════════════════════════════════

type WorkerJobErrorRecord struct {
	Attempt   int       `json:"attempt"`
	Error     string    `json:"error"`
	ErrorCode string    `json:"error_code,omitempty"`
	WorkerID  string    `json:"worker_id"`
	Stage     string    `json:"stage"`
	Timestamp time.Time `json:"timestamp"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// Helper Methods
// ═══════════════════════════════════════════════════════════════════════════════

// IsTerminal returns true if the job has reached a terminal state
func (j *WorkerJob) IsTerminal() bool {
	return j.Status.IsTerminal()
}

// CanRetry returns true if the job can be retried
func (j *WorkerJob) CanRetry() bool {
	return j.RetryCount < j.MaxRetries && !j.IsTerminal()
}

// GetDurationSec returns the duration of the job in seconds (if completed)
func (j *WorkerJob) GetDurationSec() float64 {
	if j.StartedAt == nil {
		return 0
	}
	endTime := time.Now()
	if j.CompletedAt != nil {
		endTime = *j.CompletedAt
	}
	return endTime.Sub(*j.StartedAt).Seconds()
}
