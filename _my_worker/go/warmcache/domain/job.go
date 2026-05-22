package domain

import (
	"time"

	"github.com/google/uuid"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Job Message Format (ตาม ___WORKER_INPUT_STANDARD.md)
// ═══════════════════════════════════════════════════════════════════════════════

// JobMeta contains standard metadata for all job types
// ทุก job ที่ส่งมาจาก backend จะมี _meta wrapper นี้
type JobMeta struct {
	JobID      uuid.UUID `json:"job_id"`
	JobType    string    `json:"job_type"`
	EntityType string    `json:"entity_type"`
	EntityID   uuid.UUID `json:"entity_id"`
	EntityCode string    `json:"entity_code"`
	Priority   int       `json:"priority"`
	RetryCount int       `json:"retry_count"`
	MaxRetries int       `json:"max_retries"`
	TimeoutSec int       `json:"timeout_sec"`
	CreatedAt  int64     `json:"created_at"`
}

// WarmCacheInput defines input for warmcache job
// ตาม ___WORKER_INPUT_STANDARD.md Section: Job Type: `warmcache`
type WarmCacheInput struct {
	HLSPath         string         `json:"hls_path"`         // hls/{code}/
	CDNBaseURL      string         `json:"cdn_base_url"`     // Optional override
	SegmentCounts   map[string]int `json:"segment_counts"`   // {"1080p": 150, "720p": 150}
	VerifyThreshold int            `json:"verify_threshold"` // Optional override (default: 95)
}

// WarmCacheJob is the full job message with _meta wrapper
type WarmCacheJob struct {
	Meta  JobMeta        `json:"_meta"`
	Input WarmCacheInput `json:"input"`
}

// WarmCacheOutput defines output for warmcache job
type WarmCacheOutput struct {
	TotalSegments      int     `json:"total_segments"`
	CachedHit          int     `json:"cached_hit"`
	CachedMiss         int     `json:"cached_miss"`
	Failed             int     `json:"failed"`
	HitPercent         float64 `json:"hit_percent"`
	VerificationPassed bool    `json:"verification_passed"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// Progress Update Format (ตาม ___WORKER_INTERFACE_STANDARD.md)
// ═══════════════════════════════════════════════════════════════════════════════

// ProgressUpdate is the standard progress format sent to API
type ProgressUpdate struct {
	JobID        uuid.UUID   `json:"job_id"`
	JobType      string      `json:"job_type"`
	EntityType   string      `json:"entity_type"`
	EntityID     uuid.UUID   `json:"entity_id"`
	EntityCode   string      `json:"entity_code"`
	WorkerID     string      `json:"worker_id"`
	Status       string      `json:"status"` // processing, completed, failed
	Progress     float64     `json:"progress"`
	Stage        string      `json:"stage"`
	Message      string      `json:"message"`
	StartedAt    *time.Time  `json:"started_at,omitempty"`
	UpdatedAt    time.Time   `json:"updated_at"`
	ETASeconds   *int        `json:"eta_seconds,omitempty"`
	Error        string      `json:"error,omitempty"`
	ErrorDetails *ErrorInfo  `json:"error_details,omitempty"`
	DurationSec  *float64    `json:"duration_sec,omitempty"`
	Output       interface{} `json:"output,omitempty"`
}

// ErrorInfo contains detailed error information
type ErrorInfo struct {
	Code        string `json:"code"`
	Stage       string `json:"stage"`
	Attempt     int    `json:"attempt"`
	IsRetryable bool   `json:"is_retryable"`
}

// NewProgressFromMeta creates a new progress update from job meta
func NewProgressFromMeta(meta JobMeta, workerID string) ProgressUpdate {
	now := time.Now()
	return ProgressUpdate{
		JobID:      meta.JobID,
		JobType:    meta.JobType,
		EntityType: meta.EntityType,
		EntityID:   meta.EntityID,
		EntityCode: meta.EntityCode,
		WorkerID:   workerID,
		Status:     "processing",
		UpdatedAt:  now,
		StartedAt:  &now,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Stages (ตาม ___WORKER_INTERFACE_STANDARD.md)
// ═══════════════════════════════════════════════════════════════════════════════

const (
	StageInitializing = "initializing" // 0-5%
	StageWarming      = "warming"      // 5-80%
	StageVerifying    = "verifying"    // 80-95%
	StageFinalizing   = "finalizing"   // 95-99%
	StageCompleted    = "completed"    // 100%
	StageFailed       = "failed"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Error Codes (ตาม ___WORKER_INTERFACE_STANDARD.md)
// ═══════════════════════════════════════════════════════════════════════════════

const (
	ErrCodeWarmCacheFetchFailed  = "WARMCACHE_FETCH_FAILED"
	ErrCodeWarmCacheVerifyFailed = "WARMCACHE_VERIFY_FAILED"
	ErrCodeInputValidationFailed = "INPUT_VALIDATION_FAILED"
	ErrCodeNetworkTimeout        = "NETWORK_TIMEOUT"
)
