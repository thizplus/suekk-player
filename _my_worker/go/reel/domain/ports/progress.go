package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/suekk/my_worker/go/reel/domain"
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

	Status   string  `json:"status"`
	Progress float64 `json:"progress"`
	Stage    string  `json:"stage"`
	Message  string  `json:"message"`

	StartedAt time.Time `json:"started_at"`
	UpdatedAt time.Time `json:"updated_at"`

	DurationSec  *float64           `json:"duration_sec,omitempty"`
	Output       *domain.ReelOutput `json:"output,omitempty"`
	Error        string             `json:"error,omitempty"`
	ErrorDetails *ErrorInfo         `json:"error_details,omitempty"`
}

// ErrorInfo provides detailed error information
type ErrorInfo struct {
	Code        string `json:"code"`
	Stage       string `json:"stage"`
	Attempt     int    `json:"attempt"`
	IsRetryable bool   `json:"is_retryable"`
}
