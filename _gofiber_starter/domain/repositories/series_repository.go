package repositories

import (
	"context"

	"github.com/google/uuid"
	"gofiber-template/domain/models"
)

type SeriesRepository interface {
	Create(ctx context.Context, series *models.Series) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Series, error)
	GetByCode(ctx context.Context, code string) (*models.Series, error)
	GetBySlug(ctx context.Context, slug string) (*models.Series, error)
	GetBySource(ctx context.Context, sourceSite string, sourceID int) (*models.Series, error)
	Update(ctx context.Context, series *models.Series) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, page, limit int, filters map[string]interface{}) ([]*models.Series, int64, error)
	Count(ctx context.Context) (int64, error)
}

type SeriesEpisodeRepository interface {
	Create(ctx context.Context, episode *models.SeriesEpisode) error
	CreateBatch(ctx context.Context, episodes []models.SeriesEpisode) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.SeriesEpisode, error)
	GetBySeriesAndEpisode(ctx context.Context, seriesID uuid.UUID, episodeNumber int) (*models.SeriesEpisode, error)
	ListBySeriesID(ctx context.Context, seriesID uuid.UUID) ([]models.SeriesEpisode, error)
	Update(ctx context.Context, episode *models.SeriesEpisode) error
	Delete(ctx context.Context, id uuid.UUID) error
	CountBySeriesID(ctx context.Context, seriesID uuid.UUID) (int64, error)
}

type SeriesCategoryRepository interface {
	Create(ctx context.Context, category *models.SeriesCategory) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.SeriesCategory, error)
	GetBySlug(ctx context.Context, slug string) (*models.SeriesCategory, error)
	Update(ctx context.Context, category *models.SeriesCategory) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context) ([]*models.SeriesCategory, error)
	GetSeriesCounts(ctx context.Context) (map[uuid.UUID]int64, error)
}
