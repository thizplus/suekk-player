package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
	"gorm.io/datatypes"
)

// ═══════════════════════════════════════════════════════════════════════════════
// WorkerJobService - Interface for Worker Queue Jobs
// ═══════════════════════════════════════════════════════════════════════════════

type WorkerJobService interface {
	// ═══ Job Lifecycle ═══
	CreateJob(ctx context.Context, req *dto.CreateWorkerJobRequest) (*models.WorkerJob, error)
	GetJob(ctx context.Context, id uuid.UUID) (*models.WorkerJob, error)
	CancelJob(ctx context.Context, id uuid.UUID) error

	// ═══ Query ═══
	ListJobs(ctx context.Context, filter *dto.WorkerJobFilterRequest) ([]*models.WorkerJob, int64, error)
	GetJobsByEntity(ctx context.Context, entityType models.WorkerEntityType, entityID uuid.UUID) ([]*models.WorkerJob, error)
	GetLatestJobByEntity(ctx context.Context, entityType models.WorkerEntityType, entityID uuid.UUID, jobType models.WorkerJobType) (*models.WorkerJob, error)
	GetStuckJobs(ctx context.Context, timeout time.Duration) ([]*models.WorkerJob, error)

	// ═══ Stats ═══
	GetStats(ctx context.Context, jobType *models.WorkerJobType) (*dto.WorkerJobStatsResponse, error)
	GetAllStats(ctx context.Context) (map[models.WorkerJobType]*dto.WorkerJobStatsResponse, error)

	// ═══ Status Updates (called from progress broadcaster) ═══
	MarkAsQueued(ctx context.Context, id uuid.UUID) error
	MarkAsStarted(ctx context.Context, id uuid.UUID, workerID string) error
	UpdateProgress(ctx context.Context, id uuid.UUID, progress float64, stage, message string) error
	MarkAsCompleted(ctx context.Context, id uuid.UUID, outputData datatypes.JSON) error
	MarkAsFailed(ctx context.Context, id uuid.UUID, errMsg, errCode, stage string, isRetryable bool) error

	// ═══ Retry Management ═══
	RetryJob(ctx context.Context, id uuid.UUID) error
	ProcessPendingRetries(ctx context.Context) error

	// ═══ Cleanup ═══
	DeleteJob(ctx context.Context, id uuid.UUID) error
	DeleteOrphanedJobs(ctx context.Context) (int64, error)
	DeleteCompletedJobs(ctx context.Context, olderThanDays int) (int64, error)
	DeleteFailedJobs(ctx context.Context) (int64, error)
}
