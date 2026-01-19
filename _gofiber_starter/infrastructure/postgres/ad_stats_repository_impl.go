package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
)

type AdStatsRepositoryImpl struct {
	db *gorm.DB
}

func NewAdStatsRepository(db *gorm.DB) repositories.AdStatsRepository {
	return &AdStatsRepositoryImpl{db: db}
}

// ==================== Create ====================

func (r *AdStatsRepositoryImpl) Create(ctx context.Context, impression *models.AdImpression) error {
	return r.db.WithContext(ctx).Create(impression).Error
}

// ==================== Query ====================

func (r *AdStatsRepositoryImpl) GetByID(ctx context.Context, id uuid.UUID) (*models.AdImpression, error) {
	var impression models.AdImpression
	err := r.db.WithContext(ctx).First(&impression, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &impression, nil
}

func (r *AdStatsRepositoryImpl) List(ctx context.Context, offset, limit int) ([]*models.AdImpression, error) {
	var impressions []*models.AdImpression
	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&impressions).Error
	return impressions, err
}

func (r *AdStatsRepositoryImpl) ListByProfile(ctx context.Context, profileID uuid.UUID, offset, limit int) ([]*models.AdImpression, error) {
	var impressions []*models.AdImpression
	err := r.db.WithContext(ctx).
		Where("profile_id = ?", profileID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&impressions).Error
	return impressions, err
}

func (r *AdStatsRepositoryImpl) ListByVideoCode(ctx context.Context, videoCode string, offset, limit int) ([]*models.AdImpression, error) {
	var impressions []*models.AdImpression
	err := r.db.WithContext(ctx).
		Where("video_code = ?", videoCode).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&impressions).Error
	return impressions, err
}

func (r *AdStatsRepositoryImpl) ListByDateRange(ctx context.Context, start, end time.Time, offset, limit int) ([]*models.AdImpression, error) {
	var impressions []*models.AdImpression
	err := r.db.WithContext(ctx).
		Where("created_at BETWEEN ? AND ?", start, end).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&impressions).Error
	return impressions, err
}

// ==================== Count ====================

func (r *AdStatsRepositoryImpl) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.AdImpression{}).Count(&count).Error
	return count, err
}

func (r *AdStatsRepositoryImpl) CountByProfile(ctx context.Context, profileID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.AdImpression{}).
		Where("profile_id = ?", profileID).
		Count(&count).Error
	return count, err
}

func (r *AdStatsRepositoryImpl) CountByVideoCode(ctx context.Context, videoCode string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.AdImpression{}).
		Where("video_code = ?", videoCode).
		Count(&count).Error
	return count, err
}

// ==================== Statistics ====================

func (r *AdStatsRepositoryImpl) GetOverallStats(ctx context.Context, start, end time.Time) (*models.AdImpressionStats, error) {
	var result struct {
		TotalImpressions int64
		Completed        int64
		Skipped          int64
		Errors           int64
		TotalWatchTime   int64
		TotalSkipTime    int64
		SkippedCount     int64
	}

	err := r.db.WithContext(ctx).
		Model(&models.AdImpression{}).
		Where("created_at BETWEEN ? AND ?", start, end).
		Select(`
			COUNT(*) as total_impressions,
			SUM(CASE WHEN completed = true THEN 1 ELSE 0 END) as completed,
			SUM(CASE WHEN skipped = true THEN 1 ELSE 0 END) as skipped,
			SUM(CASE WHEN error_occurred = true THEN 1 ELSE 0 END) as errors,
			SUM(watch_duration) as total_watch_time,
			SUM(CASE WHEN skipped = true THEN skipped_at ELSE 0 END) as total_skip_time,
			SUM(CASE WHEN skipped = true THEN 1 ELSE 0 END) as skipped_count
		`).
		Scan(&result).Error

	if err != nil {
		return nil, err
	}

	stats := &models.AdImpressionStats{
		TotalImpressions: result.TotalImpressions,
		Completed:        result.Completed,
		Skipped:          result.Skipped,
		Errors:           result.Errors,
	}

	if result.TotalImpressions > 0 {
		stats.CompletionRate = float64(result.Completed) / float64(result.TotalImpressions) * 100
		stats.SkipRate = float64(result.Skipped) / float64(result.TotalImpressions) * 100
		stats.ErrorRate = float64(result.Errors) / float64(result.TotalImpressions) * 100
		stats.AvgWatchDuration = float64(result.TotalWatchTime) / float64(result.TotalImpressions)
	}

	if result.SkippedCount > 0 {
		stats.AvgSkipTime = float64(result.TotalSkipTime) / float64(result.SkippedCount)
	}

	return stats, nil
}

func (r *AdStatsRepositoryImpl) GetStatsByProfile(ctx context.Context, profileID uuid.UUID, start, end time.Time) (*models.AdImpressionStats, error) {
	var result struct {
		TotalImpressions int64
		Completed        int64
		Skipped          int64
		Errors           int64
		TotalWatchTime   int64
		TotalSkipTime    int64
		SkippedCount     int64
	}

	err := r.db.WithContext(ctx).
		Model(&models.AdImpression{}).
		Where("profile_id = ? AND created_at BETWEEN ? AND ?", profileID, start, end).
		Select(`
			COUNT(*) as total_impressions,
			SUM(CASE WHEN completed = true THEN 1 ELSE 0 END) as completed,
			SUM(CASE WHEN skipped = true THEN 1 ELSE 0 END) as skipped,
			SUM(CASE WHEN error_occurred = true THEN 1 ELSE 0 END) as errors,
			SUM(watch_duration) as total_watch_time,
			SUM(CASE WHEN skipped = true THEN skipped_at ELSE 0 END) as total_skip_time,
			SUM(CASE WHEN skipped = true THEN 1 ELSE 0 END) as skipped_count
		`).
		Scan(&result).Error

	if err != nil {
		return nil, err
	}

	stats := &models.AdImpressionStats{
		TotalImpressions: result.TotalImpressions,
		Completed:        result.Completed,
		Skipped:          result.Skipped,
		Errors:           result.Errors,
	}

	if result.TotalImpressions > 0 {
		stats.CompletionRate = float64(result.Completed) / float64(result.TotalImpressions) * 100
		stats.SkipRate = float64(result.Skipped) / float64(result.TotalImpressions) * 100
		stats.ErrorRate = float64(result.Errors) / float64(result.TotalImpressions) * 100
		stats.AvgWatchDuration = float64(result.TotalWatchTime) / float64(result.TotalImpressions)
	}

	if result.SkippedCount > 0 {
		stats.AvgSkipTime = float64(result.TotalSkipTime) / float64(result.SkippedCount)
	}

	return stats, nil
}

func (r *AdStatsRepositoryImpl) GetStatsByVideoCode(ctx context.Context, videoCode string, start, end time.Time) (*models.AdImpressionStats, error) {
	var result struct {
		TotalImpressions int64
		Completed        int64
		Skipped          int64
		Errors           int64
		TotalWatchTime   int64
		TotalSkipTime    int64
		SkippedCount     int64
	}

	err := r.db.WithContext(ctx).
		Model(&models.AdImpression{}).
		Where("video_code = ? AND created_at BETWEEN ? AND ?", videoCode, start, end).
		Select(`
			COUNT(*) as total_impressions,
			SUM(CASE WHEN completed = true THEN 1 ELSE 0 END) as completed,
			SUM(CASE WHEN skipped = true THEN 1 ELSE 0 END) as skipped,
			SUM(CASE WHEN error_occurred = true THEN 1 ELSE 0 END) as errors,
			SUM(watch_duration) as total_watch_time,
			SUM(CASE WHEN skipped = true THEN skipped_at ELSE 0 END) as total_skip_time,
			SUM(CASE WHEN skipped = true THEN 1 ELSE 0 END) as skipped_count
		`).
		Scan(&result).Error

	if err != nil {
		return nil, err
	}

	stats := &models.AdImpressionStats{
		TotalImpressions: result.TotalImpressions,
		Completed:        result.Completed,
		Skipped:          result.Skipped,
		Errors:           result.Errors,
	}

	if result.TotalImpressions > 0 {
		stats.CompletionRate = float64(result.Completed) / float64(result.TotalImpressions) * 100
		stats.SkipRate = float64(result.Skipped) / float64(result.TotalImpressions) * 100
		stats.ErrorRate = float64(result.Errors) / float64(result.TotalImpressions) * 100
		stats.AvgWatchDuration = float64(result.TotalWatchTime) / float64(result.TotalImpressions)
	}

	if result.SkippedCount > 0 {
		stats.AvgSkipTime = float64(result.TotalSkipTime) / float64(result.SkippedCount)
	}

	return stats, nil
}

// ==================== Device Stats ====================

func (r *AdStatsRepositoryImpl) GetDeviceStats(ctx context.Context, start, end time.Time) (*models.DeviceStats, error) {
	var results []struct {
		DeviceType string
		Count      int64
	}

	err := r.db.WithContext(ctx).
		Model(&models.AdImpression{}).
		Where("created_at BETWEEN ? AND ?", start, end).
		Select("device_type, COUNT(*) as count").
		Group("device_type").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	stats := &models.DeviceStats{}
	for _, r := range results {
		switch r.DeviceType {
		case string(models.DeviceMobile):
			stats.Mobile = r.Count
		case string(models.DeviceDesktop):
			stats.Desktop = r.Count
		case string(models.DeviceTablet):
			stats.Tablet = r.Count
		}
	}

	return stats, nil
}

func (r *AdStatsRepositoryImpl) GetDeviceStatsByProfile(ctx context.Context, profileID uuid.UUID, start, end time.Time) (*models.DeviceStats, error) {
	var results []struct {
		DeviceType string
		Count      int64
	}

	err := r.db.WithContext(ctx).
		Model(&models.AdImpression{}).
		Where("profile_id = ? AND created_at BETWEEN ? AND ?", profileID, start, end).
		Select("device_type, COUNT(*) as count").
		Group("device_type").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	stats := &models.DeviceStats{}
	for _, r := range results {
		switch r.DeviceType {
		case string(models.DeviceMobile):
			stats.Mobile = r.Count
		case string(models.DeviceDesktop):
			stats.Desktop = r.Count
		case string(models.DeviceTablet):
			stats.Tablet = r.Count
		}
	}

	return stats, nil
}

// ==================== Skip Time Distribution ====================

func (r *AdStatsRepositoryImpl) GetSkipTimeDistribution(ctx context.Context, start, end time.Time) (map[int]int64, error) {
	var results []struct {
		SkippedAt int
		Count     int64
	}

	err := r.db.WithContext(ctx).
		Model(&models.AdImpression{}).
		Where("created_at BETWEEN ? AND ? AND skipped = true", start, end).
		Select("skipped_at, COUNT(*) as count").
		Group("skipped_at").
		Order("skipped_at ASC").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	distribution := make(map[int]int64)
	for _, r := range results {
		distribution[r.SkippedAt] = r.Count
	}

	return distribution, nil
}

// ==================== Profile Ranking ====================

func (r *AdStatsRepositoryImpl) GetProfileRanking(ctx context.Context, start, end time.Time, limit int) ([]*models.ProfileAdStats, error) {
	var results []struct {
		ProfileID      uuid.UUID
		ProfileName    string
		TotalViews     int64
		CompletedCount int64
	}

	err := r.db.WithContext(ctx).
		Model(&models.AdImpression{}).
		Joins("LEFT JOIN whitelist_profiles ON whitelist_profiles.id = ad_impressions.profile_id").
		Where("ad_impressions.created_at BETWEEN ? AND ? AND ad_impressions.profile_id IS NOT NULL", start, end).
		Select(`
			ad_impressions.profile_id,
			whitelist_profiles.name as profile_name,
			COUNT(*) as total_views,
			SUM(CASE WHEN ad_impressions.completed = true THEN 1 ELSE 0 END) as completed_count
		`).
		Group("ad_impressions.profile_id, whitelist_profiles.name").
		Order("total_views DESC").
		Limit(limit).
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	rankings := make([]*models.ProfileAdStats, len(results))
	for i, r := range results {
		completionRate := float64(0)
		if r.TotalViews > 0 {
			completionRate = float64(r.CompletedCount) / float64(r.TotalViews) * 100
		}

		rankings[i] = &models.ProfileAdStats{
			ProfileID:      r.ProfileID,
			ProfileName:    r.ProfileName,
			TotalViews:     r.TotalViews,
			CompletionRate: completionRate,
		}
	}

	return rankings, nil
}

// ==================== Cleanup ====================

func (r *AdStatsRepositoryImpl) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("created_at < ?", before).
		Delete(&models.AdImpression{})
	return result.RowsAffected, result.Error
}
