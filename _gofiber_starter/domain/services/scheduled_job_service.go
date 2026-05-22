package services

import (
	"context"

	"github.com/google/uuid"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
)

// ScheduledJobService interface for cron/scheduled jobs
// Note: เดิมชื่อ JobService แต่ rename เพื่อแยกจาก WorkerJobService
type ScheduledJobService interface {
	CreateJob(ctx context.Context, req *dto.CreateScheduledJobRequest) (*models.ScheduledJob, error)
	GetJob(ctx context.Context, jobID uuid.UUID) (*models.ScheduledJob, error)
	UpdateJob(ctx context.Context, jobID uuid.UUID, req *dto.UpdateScheduledJobRequest) (*models.ScheduledJob, error)
	DeleteJob(ctx context.Context, jobID uuid.UUID) error
	ListJobs(ctx context.Context, offset, limit int) ([]*models.ScheduledJob, int64, error)
	StartJob(ctx context.Context, jobID uuid.UUID) error
	StopJob(ctx context.Context, jobID uuid.UUID) error
	ExecuteJob(ctx context.Context, job *models.ScheduledJob) error
}
