package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
)

type ScheduledJobRepositoryImpl struct {
	db *gorm.DB
}

func NewScheduledJobRepository(db *gorm.DB) repositories.ScheduledJobRepository {
	return &ScheduledJobRepositoryImpl{db: db}
}

func (r *ScheduledJobRepositoryImpl) Create(ctx context.Context, job *models.ScheduledJob) error {
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *ScheduledJobRepositoryImpl) GetByID(ctx context.Context, id uuid.UUID) (*models.ScheduledJob, error) {
	var job models.ScheduledJob
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *ScheduledJobRepositoryImpl) GetByName(ctx context.Context, name string) (*models.ScheduledJob, error) {
	var job models.ScheduledJob
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *ScheduledJobRepositoryImpl) GetActiveJobs(ctx context.Context) ([]*models.ScheduledJob, error) {
	var jobs []*models.ScheduledJob
	err := r.db.WithContext(ctx).Where("is_active = ?", true).Find(&jobs).Error
	return jobs, err
}

func (r *ScheduledJobRepositoryImpl) Update(ctx context.Context, id uuid.UUID, job *models.ScheduledJob) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Updates(job).Error
}

func (r *ScheduledJobRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.ScheduledJob{}).Error
}

func (r *ScheduledJobRepositoryImpl) List(ctx context.Context, offset, limit int) ([]*models.ScheduledJob, error) {
	var jobs []*models.ScheduledJob
	err := r.db.WithContext(ctx).Offset(offset).Limit(limit).Find(&jobs).Error
	return jobs, err
}

func (r *ScheduledJobRepositoryImpl) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.ScheduledJob{}).Count(&count).Error
	return count, err
}

func (r *ScheduledJobRepositoryImpl) UpdateLastRun(ctx context.Context, id uuid.UUID, lastRun *time.Time) error {
	return r.db.WithContext(ctx).Model(&models.ScheduledJob{}).Where("id = ?", id).Update("last_run", lastRun).Error
}

func (r *ScheduledJobRepositoryImpl) UpdateNextRun(ctx context.Context, id uuid.UUID, nextRun *time.Time) error {
	return r.db.WithContext(ctx).Model(&models.ScheduledJob{}).Where("id = ?", id).Update("next_run", nextRun).Error
}
