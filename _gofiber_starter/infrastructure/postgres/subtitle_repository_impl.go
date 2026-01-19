package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
	"gorm.io/gorm"
)

type subtitleRepository struct {
	db *gorm.DB
}

// NewSubtitleRepository สร้าง subtitle repository
func NewSubtitleRepository(db *gorm.DB) repositories.SubtitleRepository {
	return &subtitleRepository{db: db}
}

func (r *subtitleRepository) Create(ctx context.Context, subtitle *models.Subtitle) error {
	return r.db.WithContext(ctx).Create(subtitle).Error
}

func (r *subtitleRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Subtitle, error) {
	var subtitle models.Subtitle
	if err := r.db.WithContext(ctx).First(&subtitle, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &subtitle, nil
}

func (r *subtitleRepository) GetByVideoID(ctx context.Context, videoID uuid.UUID) ([]*models.Subtitle, error) {
	var subtitles []*models.Subtitle
	if err := r.db.WithContext(ctx).
		Where("video_id = ?", videoID).
		Order("type ASC, language ASC"). // original first, then alphabetically
		Find(&subtitles).Error; err != nil {
		return nil, err
	}
	return subtitles, nil
}

func (r *subtitleRepository) GetByVideoIDAndLanguage(ctx context.Context, videoID uuid.UUID, language string) (*models.Subtitle, error) {
	var subtitle models.Subtitle
	if err := r.db.WithContext(ctx).
		Where("video_id = ? AND language = ?", videoID, language).
		First(&subtitle).Error; err != nil {
		return nil, err
	}
	return &subtitle, nil
}

func (r *subtitleRepository) GetOriginalByVideoID(ctx context.Context, videoID uuid.UUID) (*models.Subtitle, error) {
	var subtitle models.Subtitle
	if err := r.db.WithContext(ctx).
		Where("video_id = ? AND type = ?", videoID, models.SubtitleTypeOriginal).
		First(&subtitle).Error; err != nil {
		return nil, err
	}
	return &subtitle, nil
}

func (r *subtitleRepository) Update(ctx context.Context, subtitle *models.Subtitle) error {
	return r.db.WithContext(ctx).Save(subtitle).Error
}

func (r *subtitleRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.SubtitleStatus, errMsg string) error {
	updates := map[string]interface{}{
		"status": status,
		"error":  errMsg,
	}
	return r.db.WithContext(ctx).Model(&models.Subtitle{}).Where("id = ?", id).Updates(updates).Error
}

func (r *subtitleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.Subtitle{}, "id = ?", id).Error
}

func (r *subtitleRepository) DeleteByVideoID(ctx context.Context, videoID uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.Subtitle{}, "video_id = ?", videoID).Error
}

func (r *subtitleRepository) Exists(ctx context.Context, videoID uuid.UUID, language string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.Subtitle{}).
		Where("video_id = ? AND language = ?", videoID, language).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *subtitleRepository) CountByVideoID(ctx context.Context, videoID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.Subtitle{}).
		Where("video_id = ?", videoID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *subtitleRepository) GetReadyByVideoID(ctx context.Context, videoID uuid.UUID) ([]*models.Subtitle, error) {
	var subtitles []*models.Subtitle
	if err := r.db.WithContext(ctx).
		Where("video_id = ? AND status = ?", videoID, models.SubtitleStatusReady).
		Order("type ASC, language ASC").
		Find(&subtitles).Error; err != nil {
		return nil, err
	}
	return subtitles, nil
}

// === Stuck Detection Methods ===

// GetStuckProcessing หา subtitles ที่ processing/translating/detecting นานเกินไป (worker crash)
func (r *subtitleRepository) GetStuckProcessing(ctx context.Context, threshold time.Time) ([]*models.Subtitle, error) {
	var subtitles []*models.Subtitle
	err := r.db.WithContext(ctx).
		Where("status IN ?", []models.SubtitleStatus{
			models.SubtitleStatusProcessing,
			models.SubtitleStatusTranslating,
			models.SubtitleStatusDetecting,
		}).
		Where("processing_started_at IS NOT NULL").
		Where("processing_started_at < ?", threshold).
		Find(&subtitles).Error
	return subtitles, err
}

// MarkSubtitleFailed mark subtitle as failed พร้อม error message
func (r *subtitleRepository) MarkSubtitleFailed(ctx context.Context, id uuid.UUID, errorMsg string) error {
	return r.db.WithContext(ctx).
		Model(&models.Subtitle{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status": models.SubtitleStatusFailed,
			"error":  errorMsg,
		}).Error
}

// UpdateProcessingStartedAt บันทึกเวลาที่ worker เริ่มทำจริง
func (r *subtitleRepository) UpdateProcessingStartedAt(ctx context.Context, id uuid.UUID, startedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.Subtitle{}).
		Where("id = ?", id).
		Update("processing_started_at", startedAt).Error
}
