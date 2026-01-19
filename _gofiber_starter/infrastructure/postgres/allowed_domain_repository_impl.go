package postgres

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
)

type AllowedDomainRepositoryImpl struct {
	db *gorm.DB
}

func NewAllowedDomainRepository(db *gorm.DB) repositories.AllowedDomainRepository {
	return &AllowedDomainRepositoryImpl{db: db}
}

func (r *AllowedDomainRepositoryImpl) Create(ctx context.Context, domain *models.AllowedDomain) error {
	return r.db.WithContext(ctx).Create(domain).Error
}

func (r *AllowedDomainRepositoryImpl) GetByID(ctx context.Context, id uuid.UUID) (*models.AllowedDomain, error) {
	var domain models.AllowedDomain
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&domain).Error
	if err != nil {
		return nil, err
	}
	return &domain, nil
}

func (r *AllowedDomainRepositoryImpl) GetByVideoID(ctx context.Context, videoID uuid.UUID) ([]*models.AllowedDomain, error) {
	var domains []*models.AllowedDomain
	err := r.db.WithContext(ctx).Where("video_id = ?", videoID).Order("domain ASC").Find(&domains).Error
	return domains, err
}

func (r *AllowedDomainRepositoryImpl) GetByDomain(ctx context.Context, domain string) ([]*models.AllowedDomain, error) {
	var domains []*models.AllowedDomain
	err := r.db.WithContext(ctx).Where("domain = ?", domain).Find(&domains).Error
	return domains, err
}

func (r *AllowedDomainRepositoryImpl) CheckDomainAllowed(ctx context.Context, videoID uuid.UUID, domain string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.AllowedDomain{}).
		Where("video_id = ? AND domain = ?", videoID, domain).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *AllowedDomainRepositoryImpl) Update(ctx context.Context, domain *models.AllowedDomain) error {
	return r.db.WithContext(ctx).Save(domain).Error
}

func (r *AllowedDomainRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.AllowedDomain{}).Error
}

func (r *AllowedDomainRepositoryImpl) DeleteByVideoID(ctx context.Context, videoID uuid.UUID) error {
	return r.db.WithContext(ctx).Where("video_id = ?", videoID).Delete(&models.AllowedDomain{}).Error
}

func (r *AllowedDomainRepositoryImpl) DeleteByDomain(ctx context.Context, videoID uuid.UUID, domain string) error {
	return r.db.WithContext(ctx).Where("video_id = ? AND domain = ?", videoID, domain).Delete(&models.AllowedDomain{}).Error
}

func (r *AllowedDomainRepositoryImpl) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.AllowedDomain{}).Count(&count).Error
	return count, err
}

func (r *AllowedDomainRepositoryImpl) CountByVideoID(ctx context.Context, videoID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.AllowedDomain{}).Where("video_id = ?", videoID).Count(&count).Error
	return count, err
}
