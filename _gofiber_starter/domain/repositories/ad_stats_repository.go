package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gofiber-template/domain/models"
)

// AdStatsRepository interface สำหรับจัดการ Ad Impressions
type AdStatsRepository interface {
	// Create บันทึก ad impression ใหม่
	Create(ctx context.Context, impression *models.AdImpression) error

	// Query
	GetByID(ctx context.Context, id uuid.UUID) (*models.AdImpression, error)
	List(ctx context.Context, offset, limit int) ([]*models.AdImpression, error)
	ListByProfile(ctx context.Context, profileID uuid.UUID, offset, limit int) ([]*models.AdImpression, error)
	ListByVideoCode(ctx context.Context, videoCode string, offset, limit int) ([]*models.AdImpression, error)
	ListByDateRange(ctx context.Context, start, end time.Time, offset, limit int) ([]*models.AdImpression, error)

	// Count
	Count(ctx context.Context) (int64, error)
	CountByProfile(ctx context.Context, profileID uuid.UUID) (int64, error)
	CountByVideoCode(ctx context.Context, videoCode string) (int64, error)

	// Statistics
	GetOverallStats(ctx context.Context, start, end time.Time) (*models.AdImpressionStats, error)
	GetStatsByProfile(ctx context.Context, profileID uuid.UUID, start, end time.Time) (*models.AdImpressionStats, error)
	GetStatsByVideoCode(ctx context.Context, videoCode string, start, end time.Time) (*models.AdImpressionStats, error)

	// Device breakdown
	GetDeviceStats(ctx context.Context, start, end time.Time) (*models.DeviceStats, error)
	GetDeviceStatsByProfile(ctx context.Context, profileID uuid.UUID, start, end time.Time) (*models.DeviceStats, error)

	// Skip time distribution
	GetSkipTimeDistribution(ctx context.Context, start, end time.Time) (map[int]int64, error)

	// Profile performance ranking
	GetProfileRanking(ctx context.Context, start, end time.Time, limit int) ([]*models.ProfileAdStats, error)

	// Cleanup old data
	DeleteOlderThan(ctx context.Context, before time.Time) (int64, error)
}
