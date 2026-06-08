package services

import (
	"context"

	"github.com/google/uuid"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
)

type SeriesService interface {
	// Series CRUD
	Create(ctx context.Context, req *dto.CreateSeriesRequest) (*models.Series, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Series, error)
	GetByCode(ctx context.Context, code string) (*models.Series, error)
	GetBySlug(ctx context.Context, slug string) (*models.Series, error)
	Update(ctx context.Context, id uuid.UUID, req *dto.UpdateSeriesRequest) (*models.Series, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter *dto.SeriesFilterRequest) ([]*models.Series, int64, error)

	// Upsert (Bot ใช้ — สร้างหรืออัพเดท by source_site + source_id)
	Upsert(ctx context.Context, req *dto.UpsertSeriesRequest) (*models.Series, bool, error) // returns (series, isNew, err)

	// Episodes
	AddEpisodes(ctx context.Context, seriesID uuid.UUID, req *dto.AddEpisodesRequest) ([]models.SeriesEpisode, error)
	UpdateEpisode(ctx context.Context, seriesID uuid.UUID, episodeNumber int, req *dto.UpdateEpisodeRequest) (*models.SeriesEpisode, error)
	ListEpisodes(ctx context.Context, seriesID uuid.UUID) ([]models.SeriesEpisode, error)
}

type SeriesCategoryService interface {
	Create(ctx context.Context, name, slug string, parentID *uuid.UUID) (*models.SeriesCategory, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.SeriesCategory, error)
	GetOrCreateByName(ctx context.Context, name string) (*models.SeriesCategory, error)
	Update(ctx context.Context, id uuid.UUID, name, slug string) (*models.SeriesCategory, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context) ([]*models.SeriesCategory, error)
	GetSeriesCounts(ctx context.Context) (map[uuid.UUID]int64, error)
}
