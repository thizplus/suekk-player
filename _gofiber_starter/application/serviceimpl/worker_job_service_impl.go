package serviceimpl

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
	"gorm.io/datatypes"
)

// ═══════════════════════════════════════════════════════════════════════════════
// WorkerJobServiceImpl - Implementation
// ═══════════════════════════════════════════════════════════════════════════════

type WorkerJobServiceImpl struct {
	repo repositories.WorkerJobRepository
}

func NewWorkerJobService(repo repositories.WorkerJobRepository) services.WorkerJobService {
	return &WorkerJobServiceImpl{
		repo: repo,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Job Lifecycle
// ═══════════════════════════════════════════════════════════════════════════════

func (s *WorkerJobServiceImpl) CreateJob(ctx context.Context, req *dto.CreateWorkerJobRequest) (*models.WorkerJob, error) {
	// Validate job type
	if !req.JobType.IsValid() {
		logger.WarnContext(ctx, "Invalid job type", "job_type", req.JobType)
		return nil, errors.New("invalid job type")
	}

	// Set defaults
	priority := req.Priority
	if priority < 1 || priority > 3 {
		priority = 2 // Default to normal
	}

	maxRetries := req.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3 // Default
	}

	job := &models.WorkerJob{
		ID:         uuid.New(),
		JobType:    req.JobType,
		EntityType: req.EntityType,
		EntityID:   req.EntityID,
		EntityCode: req.EntityCode,
		Status:     models.WorkerJobStatusPending,
		Priority:   priority,
		MaxRetries: maxRetries,
		JobData:    req.JobData,
		CreatedAt:  time.Now(),
	}

	if err := s.repo.Create(ctx, job); err != nil {
		logger.ErrorContext(ctx, "Failed to create worker job",
			"job_type", req.JobType,
			"entity_type", req.EntityType,
			"entity_id", req.EntityID,
			"error", err,
		)
		return nil, err
	}

	logger.InfoContext(ctx, "Worker job created",
		"job_id", job.ID,
		"job_type", job.JobType,
		"entity_code", job.EntityCode,
	)

	return job, nil
}

func (s *WorkerJobServiceImpl) GetJob(ctx context.Context, id uuid.UUID) (*models.WorkerJob, error) {
	job, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.New("job not found")
	}
	return job, nil
}

func (s *WorkerJobServiceImpl) CancelJob(ctx context.Context, id uuid.UUID) error {
	job, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return errors.New("job not found")
	}

	// Can only cancel pending or queued jobs
	if job.Status != models.WorkerJobStatusPending && job.Status != models.WorkerJobStatusQueued {
		return errors.New("can only cancel pending or queued jobs")
	}

	if err := s.repo.UpdateStatus(ctx, id, models.WorkerJobStatusCancelled); err != nil {
		logger.ErrorContext(ctx, "Failed to cancel job", "job_id", id, "error", err)
		return err
	}

	logger.InfoContext(ctx, "Job cancelled", "job_id", id)
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Query
// ═══════════════════════════════════════════════════════════════════════════════

func (s *WorkerJobServiceImpl) ListJobs(ctx context.Context, filter *dto.WorkerJobFilterRequest) ([]*models.WorkerJob, int64, error) {
	// Set defaults
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		filter.Limit = 20
	}

	offset := (filter.Page - 1) * filter.Limit

	// Query by type and status if specified
	if filter.JobType != "" && filter.Status != "" {
		jobType := models.WorkerJobType(filter.JobType)
		status := models.WorkerJobStatus(filter.Status)
		return s.repo.GetByTypeAndStatus(ctx, jobType, status, offset, filter.Limit)
	}

	// Query by status only
	if filter.Status != "" {
		status := models.WorkerJobStatus(filter.Status)
		return s.repo.GetByStatus(ctx, status, offset, filter.Limit)
	}

	// Query by entity
	if filter.EntityType != "" && filter.EntityID != "" {
		entityType := models.WorkerEntityType(filter.EntityType)
		entityID, _ := uuid.Parse(filter.EntityID)
		jobs, err := s.repo.GetByEntity(ctx, entityType, entityID)
		return jobs, int64(len(jobs)), err
	}

	// Default: return queued and processing jobs
	return s.repo.GetByStatus(ctx, models.WorkerJobStatusProcessing, offset, filter.Limit)
}

func (s *WorkerJobServiceImpl) GetJobsByEntity(ctx context.Context, entityType models.WorkerEntityType, entityID uuid.UUID) ([]*models.WorkerJob, error) {
	return s.repo.GetByEntity(ctx, entityType, entityID)
}

func (s *WorkerJobServiceImpl) GetLatestJobByEntity(ctx context.Context, entityType models.WorkerEntityType, entityID uuid.UUID, jobType models.WorkerJobType) (*models.WorkerJob, error) {
	return s.repo.GetLatestByEntity(ctx, entityType, entityID, jobType)
}

func (s *WorkerJobServiceImpl) GetStuckJobs(ctx context.Context, timeout time.Duration) ([]*models.WorkerJob, error) {
	return s.repo.GetStuckJobs(ctx, timeout)
}

// ═══════════════════════════════════════════════════════════════════════════════
// Stats
// ═══════════════════════════════════════════════════════════════════════════════

func (s *WorkerJobServiceImpl) GetStats(ctx context.Context, jobType *models.WorkerJobType) (*dto.WorkerJobStatsResponse, error) {
	stats, err := s.repo.GetJobStats(ctx, jobType)
	if err != nil {
		return nil, err
	}

	return &dto.WorkerJobStatsResponse{
		TotalJobs:      stats.TotalJobs,
		PendingJobs:    stats.PendingJobs,
		QueuedJobs:     stats.QueuedJobs,
		ProcessingJobs: stats.ProcessingJobs,
		CompletedJobs:  stats.CompletedJobs,
		FailedJobs:     stats.FailedJobs,
		AvgDurationSec: stats.AvgDurationSec,
		SuccessRate:    stats.SuccessRate,
	}, nil
}

func (s *WorkerJobServiceImpl) GetAllStats(ctx context.Context) (map[models.WorkerJobType]*dto.WorkerJobStatsResponse, error) {
	result := make(map[models.WorkerJobType]*dto.WorkerJobStatsResponse)

	jobTypes := []models.WorkerJobType{
		models.WorkerJobTypeTranscode,
		models.WorkerJobTypeGallery,
		models.WorkerJobTypeWarmCache,
		models.WorkerJobTypeSubtitleDetect,
		models.WorkerJobTypeSubtitleTranscribe,
		models.WorkerJobTypeSubtitleTranslate,
		models.WorkerJobTypeReel,
	}

	for _, jt := range jobTypes {
		stats, err := s.GetStats(ctx, &jt)
		if err != nil {
			logger.WarnContext(ctx, "Failed to get stats for job type", "job_type", jt, "error", err)
			continue
		}
		result[jt] = stats
	}

	return result, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Status Updates
// ═══════════════════════════════════════════════════════════════════════════════

func (s *WorkerJobServiceImpl) MarkAsQueued(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.UpdateQueued(ctx, id); err != nil {
		logger.ErrorContext(ctx, "Failed to mark job as queued", "job_id", id, "error", err)
		return err
	}
	logger.InfoContext(ctx, "Job marked as queued", "job_id", id)
	return nil
}

func (s *WorkerJobServiceImpl) MarkAsStarted(ctx context.Context, id uuid.UUID, workerID string) error {
	if err := s.repo.UpdateStarted(ctx, id, workerID); err != nil {
		logger.ErrorContext(ctx, "Failed to mark job as started", "job_id", id, "worker_id", workerID, "error", err)
		return err
	}
	logger.InfoContext(ctx, "Job started", "job_id", id, "worker_id", workerID)
	return nil
}

func (s *WorkerJobServiceImpl) UpdateProgress(ctx context.Context, id uuid.UUID, progress float64, stage, message string) error {
	return s.repo.UpdateProgress(ctx, id, progress, stage, message)
}

func (s *WorkerJobServiceImpl) MarkAsCompleted(ctx context.Context, id uuid.UUID, outputData datatypes.JSON) error {
	if err := s.repo.UpdateCompleted(ctx, id, outputData); err != nil {
		logger.ErrorContext(ctx, "Failed to mark job as completed", "job_id", id, "error", err)
		return err
	}
	logger.InfoContext(ctx, "Job completed", "job_id", id)
	return nil
}

func (s *WorkerJobServiceImpl) MarkAsFailed(ctx context.Context, id uuid.UUID, errMsg, errCode, stage string, isRetryable bool) error {
	job, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	errorRecord := models.WorkerJobErrorRecord{
		Attempt:   job.RetryCount + 1,
		Error:     errMsg,
		ErrorCode: errCode,
		WorkerID:  job.WorkerID,
		Stage:     stage,
		Timestamp: time.Now(),
	}

	if err := s.repo.UpdateFailed(ctx, id, errMsg, errorRecord); err != nil {
		logger.ErrorContext(ctx, "Failed to mark job as failed", "job_id", id, "error", err)
		return err
	}

	logger.WarnContext(ctx, "Job failed",
		"job_id", id,
		"error_code", errCode,
		"stage", stage,
		"is_retryable", isRetryable,
		"retry_count", job.RetryCount,
	)

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Retry Management
// ═══════════════════════════════════════════════════════════════════════════════

func (s *WorkerJobServiceImpl) RetryJob(ctx context.Context, id uuid.UUID) error {
	job, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return errors.New("job not found")
	}

	if !job.CanRetry() {
		return errors.New("job cannot be retried (max retries exceeded or already completed)")
	}

	// Calculate backoff: 30s * 2^retryCount
	backoffSec := 30 * (1 << job.RetryCount)
	nextRetryAt := time.Now().Add(time.Duration(backoffSec) * time.Second)

	if err := s.repo.IncrementRetry(ctx, id, &nextRetryAt); err != nil {
		logger.ErrorContext(ctx, "Failed to schedule retry", "job_id", id, "error", err)
		return err
	}

	logger.InfoContext(ctx, "Job retry scheduled",
		"job_id", id,
		"retry_count", job.RetryCount+1,
		"next_retry_at", nextRetryAt,
	)

	return nil
}

func (s *WorkerJobServiceImpl) ProcessPendingRetries(ctx context.Context) error {
	jobs, err := s.repo.GetPendingRetry(ctx)
	if err != nil {
		return err
	}

	if len(jobs) == 0 {
		return nil
	}

	logger.InfoContext(ctx, "Processing pending retries", "count", len(jobs))

	for _, job := range jobs {
		// Reset job to queued status for re-processing
		// The NATS publisher should pick these up and republish
		if err := s.repo.UpdateQueued(ctx, job.ID); err != nil {
			logger.WarnContext(ctx, "Failed to queue retry job", "job_id", job.ID, "error", err)
		} else {
			logger.InfoContext(ctx, "Retry job queued", "job_id", job.ID, "attempt", job.RetryCount)
		}
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Cleanup
// ═══════════════════════════════════════════════════════════════════════════════

func (s *WorkerJobServiceImpl) DeleteJob(ctx context.Context, id uuid.UUID) error {
	job, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return errors.New("job not found")
	}

	// Only allow deleting terminal state jobs
	if !job.IsTerminal() {
		return errors.New("can only delete completed, failed, or cancelled jobs")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		logger.ErrorContext(ctx, "Failed to delete job", "job_id", id, "error", err)
		return err
	}

	logger.InfoContext(ctx, "Job deleted", "job_id", id, "job_type", job.JobType)
	return nil
}

func (s *WorkerJobServiceImpl) DeleteOrphanedJobs(ctx context.Context) (int64, error) {
	count, err := s.repo.DeleteOrphaned(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to delete orphaned jobs", "error", err)
		return 0, err
	}

	logger.InfoContext(ctx, "Orphaned jobs deleted", "count", count)
	return count, nil
}

func (s *WorkerJobServiceImpl) DeleteCompletedJobs(ctx context.Context, olderThanDays int) (int64, error) {
	if olderThanDays < 1 {
		olderThanDays = 7 // Default: delete jobs older than 7 days
	}

	before := time.Now().AddDate(0, 0, -olderThanDays)
	count, err := s.repo.DeleteCompletedBefore(ctx, before)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to delete completed jobs", "older_than_days", olderThanDays, "error", err)
		return 0, err
	}

	logger.InfoContext(ctx, "Completed jobs deleted", "count", count, "older_than_days", olderThanDays)
	return count, nil
}

func (s *WorkerJobServiceImpl) DeleteFailedJobs(ctx context.Context) (int64, error) {
	count, err := s.repo.DeleteFailed(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to delete failed jobs", "error", err)
		return 0, err
	}

	logger.InfoContext(ctx, "Failed jobs deleted", "count", count)
	return count, nil
}
