package dto

import (
	"time"

	"github.com/google/uuid"
	"gofiber-template/domain/models"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Scheduled Job DTOs (Cron Jobs)
// Note: เดิมชื่อ Job* แต่ rename เพื่อแยกจาก WorkerJob* (queue jobs)
// ═══════════════════════════════════════════════════════════════════════════════

type CreateScheduledJobRequest struct {
	Name     string `json:"name" validate:"required,min=1,max=100"`
	CronExpr string `json:"cronExpr" validate:"required,min=5,max=50"`
	Payload  string `json:"payload" validate:"omitempty,json"`
}

type UpdateScheduledJobRequest struct {
	Name     string `json:"name" validate:"omitempty,min=1,max=100"`
	CronExpr string `json:"cronExpr" validate:"omitempty,min=5,max=50"`
	Payload  string `json:"payload" validate:"omitempty,json"`
	IsActive bool   `json:"isActive"`
}

type ScheduledJobResponse struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	CronExpr  string     `json:"cronExpr"`
	Payload   string     `json:"payload"`
	Status    string     `json:"status"`
	LastRun   *time.Time `json:"lastRun"`
	NextRun   *time.Time `json:"nextRun"`
	IsActive  bool       `json:"isActive"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

type ScheduledJobListResponse struct {
	Jobs []ScheduledJobResponse `json:"jobs"`
	Meta PaginationMeta         `json:"meta"`
}

type ScheduledJobFilterRequest struct {
	Status   string `query:"status" validate:"omitempty,oneof=active inactive running completed failed"`
	IsActive *bool  `query:"isActive"`
	Limit    int    `query:"limit" validate:"omitempty,min=1,max=100"`
	Offset   int    `query:"offset" validate:"omitempty,min=0"`
}

type ScheduledJobExecutionRequest struct {
	JobID uuid.UUID `json:"jobId" validate:"required"`
}

type ScheduledJobExecutionResponse struct {
	JobID      uuid.UUID `json:"jobId"`
	Status     string    `json:"status"`
	Message    string    `json:"message"`
	ExecutedAt time.Time `json:"executedAt"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// Mappers
// ═══════════════════════════════════════════════════════════════════════════════

func ScheduledJobToResponse(job *models.ScheduledJob) *ScheduledJobResponse {
	return &ScheduledJobResponse{
		ID:        job.ID,
		Name:      job.Name,
		CronExpr:  job.CronExpr,
		Payload:   job.Payload,
		Status:    job.Status,
		LastRun:   job.LastRun,
		NextRun:   job.NextRun,
		IsActive:  job.IsActive,
		CreatedAt: job.CreatedAt,
		UpdatedAt: job.UpdatedAt,
	}
}
