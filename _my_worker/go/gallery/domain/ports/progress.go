package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/suekk/my_worker/go/gallery/domain"
)

// ProgressPort defines interface for publishing progress updates
type ProgressPort interface {
	Publish(ctx context.Context, update *ProgressUpdate) error
}

// ProgressUpdate represents a progress message
type ProgressUpdate struct {
	JobID      uuid.UUID `json:"job_id"`
	JobType    string    `json:"job_type"`
	EntityType string    `json:"entity_type"`
	EntityID   uuid.UUID `json:"entity_id"`
	EntityCode string    `json:"entity_code"`
	WorkerID   string    `json:"worker_id"`

	Status   string  `json:"status"`   // processing, completed, failed
	Progress float64 `json:"progress"` // 0-100
	Stage    string  `json:"stage"`    // Current stage name
	Message  string  `json:"message"`  // Human readable message

	StartedAt time.Time `json:"started_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Optional fields for completion
	DurationSec  *float64              `json:"duration_sec,omitempty"`
	Output       *domain.GalleryOutput `json:"output,omitempty"`
	Error        string                `json:"error,omitempty"`
	ErrorDetails *ErrorInfo            `json:"error_details,omitempty"`
}

// ErrorInfo provides detailed error information
type ErrorInfo struct {
	Code        string `json:"code"`
	Stage       string `json:"stage"`
	Attempt     int    `json:"attempt"`
	IsRetryable bool   `json:"is_retryable"`
}

// ProgressCallback for tracking operation progress
type ProgressCallback func(current, total int, message string)
