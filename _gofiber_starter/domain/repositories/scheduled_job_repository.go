package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gofiber-template/domain/models"
)

// ScheduledJobRepository interface for cron/scheduled jobs
// Note: เดิมชื่อ JobRepository แต่ rename เพื่อแยกจาก WorkerJobRepository
type ScheduledJobRepository interface {
	Create(ctx context.Context, job *models.ScheduledJob) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.ScheduledJob, error)
	GetByName(ctx context.Context, name string) (*models.ScheduledJob, error)
	GetActiveJobs(ctx context.Context) ([]*models.ScheduledJob, error)
	Update(ctx context.Context, id uuid.UUID, job *models.ScheduledJob) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, offset, limit int) ([]*models.ScheduledJob, error)
	Count(ctx context.Context) (int64, error)
	UpdateLastRun(ctx context.Context, id uuid.UUID, lastRun *time.Time) error
	UpdateNextRun(ctx context.Context, id uuid.UUID, nextRun *time.Time) error
}
