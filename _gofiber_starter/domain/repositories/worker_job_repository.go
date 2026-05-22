package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gofiber-template/domain/models"
	"gorm.io/datatypes"
)

// ═══════════════════════════════════════════════════════════════════════════════
// WorkerJobRepository - Interface for Worker Queue Jobs
// ═══════════════════════════════════════════════════════════════════════════════

type WorkerJobRepository interface {
	// ═══ CRUD ═══
	Create(ctx context.Context, job *models.WorkerJob) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.WorkerJob, error)
	Update(ctx context.Context, job *models.WorkerJob) error
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByEntityID(ctx context.Context, entityType models.WorkerEntityType, entityID uuid.UUID) error

	// ═══ Query ═══
	GetByEntity(ctx context.Context, entityType models.WorkerEntityType, entityID uuid.UUID) ([]*models.WorkerJob, error)
	GetByStatus(ctx context.Context, status models.WorkerJobStatus, offset, limit int) ([]*models.WorkerJob, int64, error)
	GetByTypeAndStatus(ctx context.Context, jobType models.WorkerJobType, status models.WorkerJobStatus, offset, limit int) ([]*models.WorkerJob, int64, error)
	GetLatestByEntity(ctx context.Context, entityType models.WorkerEntityType, entityID uuid.UUID, jobType models.WorkerJobType) (*models.WorkerJob, error)
	GetStuckJobs(ctx context.Context, timeout time.Duration) ([]*models.WorkerJob, error)
	GetPendingRetry(ctx context.Context) ([]*models.WorkerJob, error)

	// ═══ Stats (for Queue Management page) ═══
	CountByTypeAndStatus(ctx context.Context) (map[models.WorkerJobType]map[models.WorkerJobStatus]int64, error)
	CountByStatusLast24h(ctx context.Context) (map[models.WorkerJobStatus]int64, error)
	GetJobStats(ctx context.Context, jobType *models.WorkerJobType) (*WorkerJobStats, error)

	// ═══ Cleanup ═══
	DeleteOrphaned(ctx context.Context) (int64, error)                          // Delete jobs where entity doesn't exist
	DeleteCompletedBefore(ctx context.Context, before time.Time) (int64, error) // Delete old completed jobs
	DeleteFailed(ctx context.Context) (int64, error)                            // Delete all failed jobs

	// ═══ Status Updates (optimized single-field updates) ═══
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.WorkerJobStatus) error
	UpdateProgress(ctx context.Context, id uuid.UUID, progress float64, stage, message string) error
	UpdateStarted(ctx context.Context, id uuid.UUID, workerID string) error
	UpdateQueued(ctx context.Context, id uuid.UUID) error
	UpdateCompleted(ctx context.Context, id uuid.UUID, outputData datatypes.JSON) error
	UpdateFailed(ctx context.Context, id uuid.UUID, errMsg string, errorRecord models.WorkerJobErrorRecord) error
	IncrementRetry(ctx context.Context, id uuid.UUID, nextRetryAt *time.Time) error
}

// ═══════════════════════════════════════════════════════════════════════════════
// WorkerJobStats - Aggregated stats for monitoring
// ═══════════════════════════════════════════════════════════════════════════════

type WorkerJobStats struct {
	TotalJobs       int64   `json:"total_jobs"`
	PendingJobs     int64   `json:"pending_jobs"`
	QueuedJobs      int64   `json:"queued_jobs"`
	ProcessingJobs  int64   `json:"processing_jobs"`
	CompletedJobs   int64   `json:"completed_jobs"`
	FailedJobs      int64   `json:"failed_jobs"`
	AvgDurationSec  float64 `json:"avg_duration_sec"`
	SuccessRate     float64 `json:"success_rate"` // completed / (completed + failed)
}
