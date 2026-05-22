package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ProgressUpdate is the standard progress format sent to API
// ตาม ___WORKER_INTERFACE_STANDARD.md
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

// ProgressPort defines interface for publishing progress updates
type ProgressPort interface {
	// Publish sends a progress update via NATS Pub/Sub
	Publish(ctx context.Context, update *ProgressUpdate) error
}
