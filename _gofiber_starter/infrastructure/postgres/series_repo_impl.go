package postgres

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
)

// ═══════════════════════════════════════════
// Series Repository
// ═══════════════════════════════════════════

type SeriesRepositoryImpl struct {
	db *gorm.DB
}

func NewSeriesRepository(db *gorm.DB) repositories.SeriesRepository {
	return &SeriesRepositoryImpl{db: db}
}

func (r *SeriesRepositoryImpl) Create(ctx context.Context, series *models.Series) error {
	return r.db.WithContext(ctx).Create(series).Error
}

func (r *SeriesRepositoryImpl) GetByID(ctx context.Context, id uuid.UUID) (*models.Series, error) {
	var series models.Series
	err := r.db.WithContext(ctx).
		Preload("SeriesCategory").
		Preload("Episodes", func(db *gorm.DB) *gorm.DB {
			return db.Order("episode_number ASC").Preload("Video")
		}).
		Where("id = ?", id).First(&series).Error
	if err != nil {
		return nil, err
	}
	return &series, nil
}

func (r *SeriesRepositoryImpl) GetByCode(ctx context.Context, code string) (*models.Series, error) {
	var series models.Series
	err := r.db.WithContext(ctx).
		Preload("SeriesCategory").
		Preload("Episodes", func(db *gorm.DB) *gorm.DB {
			return db.Order("episode_number ASC").Preload("Video")
		}).
		Where("code = ?", code).First(&series).Error
	if err != nil {
		return nil, err
	}
	return &series, nil
}

func (r *SeriesRepositoryImpl) GetBySlug(ctx context.Context, slug string) (*models.Series, error) {
	var series models.Series
	err := r.db.WithContext(ctx).
		Preload("SeriesCategory").
		Preload("Episodes", func(db *gorm.DB) *gorm.DB {
			return db.Order("episode_number ASC").Preload("Video")
		}).
		Where("slug = ?", slug).First(&series).Error
	if err != nil {
		return nil, err
	}
	return &series, nil
}

func (r *SeriesRepositoryImpl) GetBySource(ctx context.Context, sourceSite string, sourceID int) (*models.Series, error) {
	var series models.Series
	err := r.db.WithContext(ctx).
		Where("source_site = ? AND source_id = ?", sourceSite, sourceID).
		First(&series).Error
	if err != nil {
		return nil, err
	}
	return &series, nil
}

func (r *SeriesRepositoryImpl) Update(ctx context.Context, series *models.Series) error {
	return r.db.WithContext(ctx).Save(series).Error
}

func (r *SeriesRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.Series{}).Error
}

func (r *SeriesRepositoryImpl) List(ctx context.Context, page, limit int, filters map[string]interface{}) ([]*models.Series, int64, error) {
	var series []*models.Series
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Series{})

	// Apply filters
	if status, ok := filters["status"].(string); ok && status != "" {
		query = query.Where("status = ?", status)
	}
	if categoryID, ok := filters["category_id"].(uuid.UUID); ok {
		query = query.Where("series_category_id = ?", categoryID)
	}
	if audioType, ok := filters["audio_type"].(string); ok && audioType != "" {
		query = query.Where("audio_type = ?", audioType)
	}
	if year, ok := filters["year"].(int); ok && year > 0 {
		query = query.Where("year = ?", year)
	}
	if search, ok := filters["search"].(string); ok && search != "" {
		query = query.Where("title ILIKE ? OR thai_title ILIKE ? OR slug ILIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	// Count
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Sort
	sortBy := "created_at"
	sortOrder := "DESC"
	if s, ok := filters["sort_by"].(string); ok && s != "" {
		switch s {
		case "year", "rating", "title", "created_at", "total_episodes":
			sortBy = s
		}
	}
	if s, ok := filters["sort_order"].(string); ok && s == "asc" {
		sortOrder = "ASC"
	}

	// Paginate + fetch
	offset := (page - 1) * limit
	err := query.
		Preload("SeriesCategory").
		Order(sortBy + " " + sortOrder).
		Offset(offset).
		Limit(limit).
		Find(&series).Error

	return series, total, err
}

func (r *SeriesRepositoryImpl) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.Series{}).Count(&count).Error
	return count, err
}

// ═══════════════════════════════════════════
// SeriesEpisode Repository
// ═══════════════════════════════════════════

type SeriesEpisodeRepositoryImpl struct {
	db *gorm.DB
}

func NewSeriesEpisodeRepository(db *gorm.DB) repositories.SeriesEpisodeRepository {
	return &SeriesEpisodeRepositoryImpl{db: db}
}

func (r *SeriesEpisodeRepositoryImpl) Create(ctx context.Context, episode *models.SeriesEpisode) error {
	return r.db.WithContext(ctx).Create(episode).Error
}

func (r *SeriesEpisodeRepositoryImpl) CreateBatch(ctx context.Context, episodes []models.SeriesEpisode) error {
	if len(episodes) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&episodes).Error
}

func (r *SeriesEpisodeRepositoryImpl) GetByID(ctx context.Context, id uuid.UUID) (*models.SeriesEpisode, error) {
	var episode models.SeriesEpisode
	err := r.db.WithContext(ctx).Preload("Video").Where("id = ?", id).First(&episode).Error
	if err != nil {
		return nil, err
	}
	return &episode, nil
}

func (r *SeriesEpisodeRepositoryImpl) GetBySeriesAndEpisode(ctx context.Context, seriesID uuid.UUID, episodeNumber int) (*models.SeriesEpisode, error) {
	var episode models.SeriesEpisode
	err := r.db.WithContext(ctx).Preload("Video").
		Where("series_id = ? AND episode_number = ?", seriesID, episodeNumber).
		First(&episode).Error
	if err != nil {
		return nil, err
	}
	return &episode, nil
}

func (r *SeriesEpisodeRepositoryImpl) ListBySeriesID(ctx context.Context, seriesID uuid.UUID) ([]models.SeriesEpisode, error) {
	var episodes []models.SeriesEpisode
	err := r.db.WithContext(ctx).Preload("Video").
		Where("series_id = ?", seriesID).
		Order("episode_number ASC").
		Find(&episodes).Error
	return episodes, err
}

func (r *SeriesEpisodeRepositoryImpl) Update(ctx context.Context, episode *models.SeriesEpisode) error {
	return r.db.WithContext(ctx).Save(episode).Error
}

func (r *SeriesEpisodeRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.SeriesEpisode{}).Error
}

func (r *SeriesEpisodeRepositoryImpl) CountBySeriesID(ctx context.Context, seriesID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.SeriesEpisode{}).
		Where("series_id = ?", seriesID).Count(&count).Error
	return count, err
}

// ═══════════════════════════════════════════
// SeriesCategory Repository
// ═══════════════════════════════════════════

type SeriesCategoryRepositoryImpl struct {
	db *gorm.DB
}

func NewSeriesCategoryRepository(db *gorm.DB) repositories.SeriesCategoryRepository {
	return &SeriesCategoryRepositoryImpl{db: db}
}

func (r *SeriesCategoryRepositoryImpl) Create(ctx context.Context, category *models.SeriesCategory) error {
	return r.db.WithContext(ctx).Create(category).Error
}

func (r *SeriesCategoryRepositoryImpl) GetByID(ctx context.Context, id uuid.UUID) (*models.SeriesCategory, error) {
	var cat models.SeriesCategory
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&cat).Error
	if err != nil {
		return nil, err
	}
	return &cat, nil
}

func (r *SeriesCategoryRepositoryImpl) GetBySlug(ctx context.Context, slug string) (*models.SeriesCategory, error) {
	var cat models.SeriesCategory
	err := r.db.WithContext(ctx).Where("slug = ?", slug).First(&cat).Error
	if err != nil {
		return nil, err
	}
	return &cat, nil
}

func (r *SeriesCategoryRepositoryImpl) Update(ctx context.Context, category *models.SeriesCategory) error {
	return r.db.WithContext(ctx).Save(category).Error
}

func (r *SeriesCategoryRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.SeriesCategory{}).Error
}

func (r *SeriesCategoryRepositoryImpl) List(ctx context.Context) ([]*models.SeriesCategory, error) {
	var cats []*models.SeriesCategory
	err := r.db.WithContext(ctx).
		Where("parent_id IS NULL").
		Order("sort_order ASC, name ASC").
		Preload("Children", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, name ASC")
		}).
		Find(&cats).Error
	return cats, err
}

func (r *SeriesCategoryRepositoryImpl) GetSeriesCounts(ctx context.Context) (map[uuid.UUID]int64, error) {
	type result struct {
		CategoryID uuid.UUID
		Count      int64
	}
	var results []result
	err := r.db.WithContext(ctx).Model(&models.Series{}).
		Select("series_category_id as category_id, COUNT(*) as count").
		Where("series_category_id IS NOT NULL AND status = 'active'").
		Group("series_category_id").
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	counts := make(map[uuid.UUID]int64)
	for _, r := range results {
		counts[r.CategoryID] = r.Count
	}
	return counts, nil
}
