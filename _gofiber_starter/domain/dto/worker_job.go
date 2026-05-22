package dto

import (
	"time"

	"github.com/google/uuid"
	"gofiber-template/domain/models"
	"gorm.io/datatypes"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Worker Job DTOs (Queue Jobs)
// ═══════════════════════════════════════════════════════════════════════════════

// CreateWorkerJobRequest - Internal use when creating a job
type CreateWorkerJobRequest struct {
	JobType    models.WorkerJobType   `json:"job_type" validate:"required"`
	EntityType models.WorkerEntityType `json:"entity_type" validate:"required"`
	EntityID   uuid.UUID              `json:"entity_id" validate:"required"`
	EntityCode string                 `json:"entity_code"`
	Priority   int                    `json:"priority"` // 1=high, 2=normal, 3=low
	MaxRetries int                    `json:"max_retries"`
	JobData    datatypes.JSON         `json:"job_data" validate:"required"`
}

// WorkerJobResponse - Response for API
type WorkerJobResponse struct {
	ID           uuid.UUID               `json:"id"`
	JobType      models.WorkerJobType    `json:"job_type"`
	EntityType   models.WorkerEntityType `json:"entity_type"`
	EntityID     uuid.UUID               `json:"entity_id"`
	EntityCode   string                  `json:"entity_code"`
	Status       models.WorkerJobStatus  `json:"status"`
	Progress     float64                 `json:"progress"`
	CurrentStage string                  `json:"current_stage"`
	Message      string                  `json:"message"`
	WorkerID     string                  `json:"worker_id,omitempty"`
	Priority     int                     `json:"priority"`
	CreatedAt    time.Time               `json:"created_at"`
	QueuedAt     *time.Time              `json:"queued_at,omitempty"`
	StartedAt    *time.Time              `json:"started_at,omitempty"`
	CompletedAt  *time.Time              `json:"completed_at,omitempty"`
	RetryCount   int                     `json:"retry_count"`
	MaxRetries   int                     `json:"max_retries"`
	LastError    string                  `json:"last_error,omitempty"`
	DurationSec  float64                 `json:"duration_sec,omitempty"` // Calculated field
}

// WorkerJobDetailResponse - Detailed response including job_data and output_data
type WorkerJobDetailResponse struct {
	WorkerJobResponse
	JobData      interface{}                    `json:"job_data,omitempty"`
	OutputData   interface{}                    `json:"output_data,omitempty"`
	ErrorHistory []models.WorkerJobErrorRecord  `json:"error_history,omitempty"`
}

// WorkerJobListResponse - Paginated list response
type WorkerJobListResponse struct {
	Jobs []WorkerJobResponse `json:"jobs"`
	Meta PaginationMeta      `json:"meta"`
}

// WorkerJobStatsResponse - Stats for monitoring
type WorkerJobStatsResponse struct {
	TotalJobs      int64   `json:"total_jobs"`
	PendingJobs    int64   `json:"pending_jobs"`
	QueuedJobs     int64   `json:"queued_jobs"`
	ProcessingJobs int64   `json:"processing_jobs"`
	CompletedJobs  int64   `json:"completed_jobs"`
	FailedJobs     int64   `json:"failed_jobs"`
	AvgDurationSec float64 `json:"avg_duration_sec"`
	SuccessRate    float64 `json:"success_rate"`
}

// WorkerJobTypeStatsResponse - Stats grouped by job type
type WorkerJobTypeStatsResponse struct {
	JobType string                `json:"job_type"`
	Stats   WorkerJobStatsResponse `json:"stats"`
}

// WorkerJobFilterRequest - Filter for listing jobs
type WorkerJobFilterRequest struct {
	JobType    string `query:"job_type" validate:"omitempty"`
	Status     string `query:"status" validate:"omitempty"`
	EntityType string `query:"entity_type" validate:"omitempty"`
	EntityID   string `query:"entity_id" validate:"omitempty,uuid"`
	Page       int    `query:"page" validate:"omitempty,min=1"`
	Limit      int    `query:"limit" validate:"omitempty,min=1,max=100"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// Mappers
// ═══════════════════════════════════════════════════════════════════════════════

func WorkerJobToResponse(job *models.WorkerJob) *WorkerJobResponse {
	if job == nil {
		return nil
	}
	return &WorkerJobResponse{
		ID:           job.ID,
		JobType:      job.JobType,
		EntityType:   job.EntityType,
		EntityID:     job.EntityID,
		EntityCode:   job.EntityCode,
		Status:       job.Status,
		Progress:     job.Progress,
		CurrentStage: job.CurrentStage,
		Message:      job.Message,
		WorkerID:     job.WorkerID,
		Priority:     job.Priority,
		CreatedAt:    job.CreatedAt,
		QueuedAt:     job.QueuedAt,
		StartedAt:    job.StartedAt,
		CompletedAt:  job.CompletedAt,
		RetryCount:   job.RetryCount,
		MaxRetries:   job.MaxRetries,
		LastError:    job.LastError,
		DurationSec:  job.GetDurationSec(),
	}
}

func WorkerJobToDetailResponse(job *models.WorkerJob) *WorkerJobDetailResponse {
	if job == nil {
		return nil
	}

	resp := &WorkerJobDetailResponse{
		WorkerJobResponse: *WorkerJobToResponse(job),
	}

	// Parse JobData and OutputData from JSON
	if len(job.JobData) > 0 {
		var jobData interface{}
		if err := job.JobData.UnmarshalJSON([]byte(job.JobData)); err == nil {
			resp.JobData = jobData
		}
	}

	if len(job.OutputData) > 0 {
		var outputData interface{}
		if err := job.OutputData.UnmarshalJSON([]byte(job.OutputData)); err == nil {
			resp.OutputData = outputData
		}
	}

	// Parse ErrorHistory
	if len(job.ErrorHistory) > 0 {
		var errorHistory []models.WorkerJobErrorRecord
		if err := job.ErrorHistory.UnmarshalJSON([]byte(job.ErrorHistory)); err == nil {
			resp.ErrorHistory = errorHistory
		}
	}

	return resp
}

func WorkerJobsToListResponse(jobs []*models.WorkerJob, total int64, offset, limit int) *WorkerJobListResponse {
	responses := make([]WorkerJobResponse, len(jobs))
	for i, job := range jobs {
		responses[i] = *WorkerJobToResponse(job)
	}

	return &WorkerJobListResponse{
		Jobs: responses,
		Meta: PaginationMeta{
			Total:  total,
			Offset: offset,
			Limit:  limit,
		},
	}
}
