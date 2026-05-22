package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
)

// ═══════════════════════════════════════════════════════════════════════════════
// WorkerJobRepositoryImpl - PostgreSQL implementation
// ═══════════════════════════════════════════════════════════════════════════════

type WorkerJobRepositoryImpl struct {
	db *gorm.DB
}

func NewWorkerJobRepository(db *gorm.DB) repositories.WorkerJobRepository {
	return &WorkerJobRepositoryImpl{db: db}
}

// ═══════════════════════════════════════════════════════════════════════════════
// CRUD Operations
// ═══════════════════════════════════════════════════════════════════════════════

func (r *WorkerJobRepositoryImpl) Create(ctx context.Context, job *models.WorkerJob) error {
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *WorkerJobRepositoryImpl) GetByID(ctx context.Context, id uuid.UUID) (*models.WorkerJob, error) {
	var job models.WorkerJob
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *WorkerJobRepositoryImpl) Update(ctx context.Context, job *models.WorkerJob) error {
	return r.db.WithContext(ctx).Save(job).Error
}

func (r *WorkerJobRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.WorkerJob{}).Error
}

func (r *WorkerJobRepositoryImpl) DeleteByEntityID(ctx context.Context, entityType models.WorkerEntityType, entityID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Delete(&models.WorkerJob{}).Error
}

func (r *WorkerJobRepositoryImpl) DeleteOrphaned(ctx context.Context) (int64, error) {
	// Delete video jobs where video doesn't exist
	videoResult := r.db.WithContext(ctx).Exec(`
		DELETE FROM worker_jobs
		WHERE entity_type = 'video'
		AND entity_id NOT IN (SELECT id FROM videos)
	`)
	if videoResult.Error != nil {
		return 0, videoResult.Error
	}

	// Delete subtitle jobs where subtitle doesn't exist
	subtitleResult := r.db.WithContext(ctx).Exec(`
		DELETE FROM worker_jobs
		WHERE entity_type = 'subtitle'
		AND entity_id NOT IN (SELECT id FROM subtitles)
	`)
	if subtitleResult.Error != nil {
		return videoResult.RowsAffected, subtitleResult.Error
	}

	// Delete reel jobs where reel doesn't exist
	reelResult := r.db.WithContext(ctx).Exec(`
		DELETE FROM worker_jobs
		WHERE entity_type = 'reel'
		AND entity_id NOT IN (SELECT id FROM reels)
	`)
	if reelResult.Error != nil {
		return videoResult.RowsAffected + subtitleResult.RowsAffected, reelResult.Error
	}

	return videoResult.RowsAffected + subtitleResult.RowsAffected + reelResult.RowsAffected, nil
}

func (r *WorkerJobRepositoryImpl) DeleteCompletedBefore(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("status = ? AND completed_at < ?", models.WorkerJobStatusCompleted, before).
		Delete(&models.WorkerJob{})
	return result.RowsAffected, result.Error
}

func (r *WorkerJobRepositoryImpl) DeleteFailed(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("status = ?", models.WorkerJobStatusFailed).
		Delete(&models.WorkerJob{})
	return result.RowsAffected, result.Error
}

// ═══════════════════════════════════════════════════════════════════════════════
// Query Operations
// ═══════════════════════════════════════════════════════════════════════════════

func (r *WorkerJobRepositoryImpl) GetByEntity(ctx context.Context, entityType models.WorkerEntityType, entityID uuid.UUID) ([]*models.WorkerJob, error) {
	var jobs []*models.WorkerJob
	err := r.db.WithContext(ctx).
		Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Order("created_at DESC").
		Find(&jobs).Error
	return jobs, err
}

func (r *WorkerJobRepositoryImpl) GetByStatus(ctx context.Context, status models.WorkerJobStatus, offset, limit int) ([]*models.WorkerJob, int64, error) {
	var jobs []*models.WorkerJob
	var total int64

	query := r.db.WithContext(ctx).Model(&models.WorkerJob{}).Where("status = ?", status)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&jobs).Error
	return jobs, total, err
}

func (r *WorkerJobRepositoryImpl) GetByTypeAndStatus(ctx context.Context, jobType models.WorkerJobType, status models.WorkerJobStatus, offset, limit int) ([]*models.WorkerJob, int64, error) {
	var jobs []*models.WorkerJob
	var total int64

	query := r.db.WithContext(ctx).Model(&models.WorkerJob{}).
		Where("job_type = ? AND status = ?", jobType, status)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&jobs).Error
	return jobs, total, err
}

func (r *WorkerJobRepositoryImpl) GetLatestByEntity(ctx context.Context, entityType models.WorkerEntityType, entityID uuid.UUID, jobType models.WorkerJobType) (*models.WorkerJob, error) {
	var job models.WorkerJob
	err := r.db.WithContext(ctx).
		Where("entity_type = ? AND entity_id = ? AND job_type = ?", entityType, entityID, jobType).
		Order("created_at DESC").
		First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *WorkerJobRepositoryImpl) GetStuckJobs(ctx context.Context, timeout time.Duration) ([]*models.WorkerJob, error) {
	var jobs []*models.WorkerJob
	cutoff := time.Now().Add(-timeout)

	err := r.db.WithContext(ctx).
		Where("status = ? AND started_at IS NOT NULL AND started_at < ?", models.WorkerJobStatusProcessing, cutoff).
		Find(&jobs).Error
	return jobs, err
}

func (r *WorkerJobRepositoryImpl) GetPendingRetry(ctx context.Context) ([]*models.WorkerJob, error) {
	var jobs []*models.WorkerJob
	now := time.Now()

	err := r.db.WithContext(ctx).
		Where("status = ? AND next_retry_at IS NOT NULL AND next_retry_at <= ?", models.WorkerJobStatusFailed, now).
		Where("retry_count < max_retries").
		Order("next_retry_at ASC").
		Find(&jobs).Error
	return jobs, err
}

// ═══════════════════════════════════════════════════════════════════════════════
// Stats Operations
// ═══════════════════════════════════════════════════════════════════════════════

func (r *WorkerJobRepositoryImpl) CountByTypeAndStatus(ctx context.Context) (map[models.WorkerJobType]map[models.WorkerJobStatus]int64, error) {
	type Result struct {
		JobType string `gorm:"column:job_type"`
		Status  string `gorm:"column:status"`
		Count   int64  `gorm:"column:count"`
	}

	var results []Result
	err := r.db.WithContext(ctx).
		Model(&models.WorkerJob{}).
		Select("job_type, status, COUNT(*) as count").
		Where("created_at > NOW() - INTERVAL '7 days'").
		Group("job_type, status").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	// Convert to nested map
	stats := make(map[models.WorkerJobType]map[models.WorkerJobStatus]int64)
	for _, r := range results {
		jobType := models.WorkerJobType(r.JobType)
		status := models.WorkerJobStatus(r.Status)

		if stats[jobType] == nil {
			stats[jobType] = make(map[models.WorkerJobStatus]int64)
		}
		stats[jobType][status] = r.Count
	}

	return stats, nil
}

func (r *WorkerJobRepositoryImpl) CountByStatusLast24h(ctx context.Context) (map[models.WorkerJobStatus]int64, error) {
	type Result struct {
		Status string `gorm:"column:status"`
		Count  int64  `gorm:"column:count"`
	}

	var results []Result
	err := r.db.WithContext(ctx).
		Model(&models.WorkerJob{}).
		Select("status, COUNT(*) as count").
		Where("created_at > NOW() - INTERVAL '24 hours'").
		Group("status").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	stats := make(map[models.WorkerJobStatus]int64)
	for _, r := range results {
		stats[models.WorkerJobStatus(r.Status)] = r.Count
	}

	return stats, nil
}

func (r *WorkerJobRepositoryImpl) GetJobStats(ctx context.Context, jobType *models.WorkerJobType) (*repositories.WorkerJobStats, error) {
	stats := &repositories.WorkerJobStats{}

	query := r.db.WithContext(ctx).Model(&models.WorkerJob{})
	if jobType != nil {
		query = query.Where("job_type = ?", *jobType)
	}

	// Count by status
	type StatusCount struct {
		Status string `gorm:"column:status"`
		Count  int64  `gorm:"column:count"`
	}
	var statusCounts []StatusCount
	err := query.Select("status, COUNT(*) as count").Group("status").Scan(&statusCounts).Error
	if err != nil {
		return nil, err
	}

	for _, sc := range statusCounts {
		stats.TotalJobs += sc.Count
		switch models.WorkerJobStatus(sc.Status) {
		case models.WorkerJobStatusPending:
			stats.PendingJobs = sc.Count
		case models.WorkerJobStatusQueued:
			stats.QueuedJobs = sc.Count
		case models.WorkerJobStatusProcessing:
			stats.ProcessingJobs = sc.Count
		case models.WorkerJobStatusCompleted:
			stats.CompletedJobs = sc.Count
		case models.WorkerJobStatusFailed:
			stats.FailedJobs = sc.Count
		}
	}

	// Calculate success rate
	finishedJobs := stats.CompletedJobs + stats.FailedJobs
	if finishedJobs > 0 {
		stats.SuccessRate = float64(stats.CompletedJobs) / float64(finishedJobs) * 100
	}

	// Calculate average duration (completed jobs only)
	var avgDuration struct {
		Avg float64 `gorm:"column:avg"`
	}
	err = r.db.WithContext(ctx).
		Model(&models.WorkerJob{}).
		Select("AVG(EXTRACT(EPOCH FROM (completed_at - started_at))) as avg").
		Where("status = ? AND started_at IS NOT NULL AND completed_at IS NOT NULL", models.WorkerJobStatusCompleted).
		Scan(&avgDuration).Error

	if err == nil {
		stats.AvgDurationSec = avgDuration.Avg
	}

	return stats, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Status Update Operations (Optimized)
// ═══════════════════════════════════════════════════════════════════════════════

func (r *WorkerJobRepositoryImpl) UpdateStatus(ctx context.Context, id uuid.UUID, status models.WorkerJobStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.WorkerJob{}).
		Where("id = ?", id).
		Update("status", status).Error
}

func (r *WorkerJobRepositoryImpl) UpdateProgress(ctx context.Context, id uuid.UUID, progress float64, stage, message string) error {
	return r.db.WithContext(ctx).
		Model(&models.WorkerJob{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"progress":      progress,
			"current_stage": stage,
			"message":       message,
		}).Error
}

func (r *WorkerJobRepositoryImpl) UpdateStarted(ctx context.Context, id uuid.UUID, workerID string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.WorkerJob{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     models.WorkerJobStatusProcessing,
			"worker_id":  workerID,
			"started_at": now,
		}).Error
}

func (r *WorkerJobRepositoryImpl) UpdateQueued(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.WorkerJob{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":    models.WorkerJobStatusQueued,
			"queued_at": now,
		}).Error
}

func (r *WorkerJobRepositoryImpl) UpdateCompleted(ctx context.Context, id uuid.UUID, outputData datatypes.JSON) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.WorkerJob{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       models.WorkerJobStatusCompleted,
			"progress":     100,
			"completed_at": now,
			"output_data":  outputData,
		}).Error
}

func (r *WorkerJobRepositoryImpl) UpdateFailed(ctx context.Context, id uuid.UUID, errMsg string, errorRecord models.WorkerJobErrorRecord) error {
	// Get current job to append error history
	var job models.WorkerJob
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&job).Error; err != nil {
		return err
	}

	// Append error to history
	var errorHistory []models.WorkerJobErrorRecord
	if len(job.ErrorHistory) > 0 {
		json.Unmarshal(job.ErrorHistory, &errorHistory)
	}
	errorHistory = append(errorHistory, errorRecord)
	errorHistoryJSON, _ := json.Marshal(errorHistory)

	return r.db.WithContext(ctx).
		Model(&models.WorkerJob{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        models.WorkerJobStatusFailed,
			"last_error":    errMsg,
			"error_history": datatypes.JSON(errorHistoryJSON),
		}).Error
}

func (r *WorkerJobRepositoryImpl) IncrementRetry(ctx context.Context, id uuid.UUID, nextRetryAt *time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&models.WorkerJob{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"retry_count":   gorm.Expr("retry_count + 1"),
			"status":        models.WorkerJobStatusQueued,
			"next_retry_at": nextRetryAt,
		})

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("job not found: %s", id)
	}
	return nil
}
