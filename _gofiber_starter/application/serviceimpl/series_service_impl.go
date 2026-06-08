package serviceimpl

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gosimple/slug"

	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
)

type SeriesServiceImpl struct {
	seriesRepo  repositories.SeriesRepository
	episodeRepo repositories.SeriesEpisodeRepository
}

func NewSeriesService(
	seriesRepo repositories.SeriesRepository,
	episodeRepo repositories.SeriesEpisodeRepository,
) services.SeriesService {
	return &SeriesServiceImpl{
		seriesRepo:  seriesRepo,
		episodeRepo: episodeRepo,
	}
}

// ═══════════════════════════════════════════
// Series CRUD
// ═══════════════════════════════════════════

func (s *SeriesServiceImpl) Create(ctx context.Context, req *dto.CreateSeriesRequest) (*models.Series, error) {
	// เช็ค slug ซ้ำ
	existing, _ := s.seriesRepo.GetBySlug(ctx, req.Slug)
	if existing != nil {
		return nil, errors.New("series slug already exists")
	}

	// Generate code จาก slug + year
	code := s.generateCode(req.Slug, req.Year)

	series := &models.Series{
		ID:               uuid.New(),
		Code:             code,
		Title:            req.Title,
		ThaiTitle:        req.ThaiTitle,
		Slug:             slug.Make(req.Slug),
		Description:      req.Description,
		Year:             req.Year,
		Rating:           req.Rating,
		Quality:          req.Quality,
		AudioType:        req.AudioType,
		TrailerYoutubeID: req.TrailerYoutubeID,
		TotalEpisodes:    req.TotalEpisodes,
		IsCompleted:      req.IsCompleted,
		SeriesCategoryID: req.CategoryID,
		Status:           "active",
		SourceSite:       req.SourceSite,
		SourceID:         req.SourceID,
		SourceURL:        req.SourceURL,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := s.seriesRepo.Create(ctx, series); err != nil {
		logger.ErrorContext(ctx, "Failed to create series", "error", err)
		return nil, err
	}

	logger.InfoContext(ctx, "Series created", "series_id", series.ID, "code", series.Code, "title", series.Title)
	return series, nil
}

func (s *SeriesServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*models.Series, error) {
	return s.seriesRepo.GetByID(ctx, id)
}

func (s *SeriesServiceImpl) GetByCode(ctx context.Context, code string) (*models.Series, error) {
	return s.seriesRepo.GetByCode(ctx, code)
}

func (s *SeriesServiceImpl) GetBySlug(ctx context.Context, slug string) (*models.Series, error) {
	return s.seriesRepo.GetBySlug(ctx, slug)
}

func (s *SeriesServiceImpl) Update(ctx context.Context, id uuid.UUID, req *dto.UpdateSeriesRequest) (*models.Series, error) {
	series, err := s.seriesRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.New("series not found")
	}

	if req.Title != nil {
		series.Title = *req.Title
	}
	if req.ThaiTitle != nil {
		series.ThaiTitle = *req.ThaiTitle
	}
	if req.Description != nil {
		series.Description = *req.Description
	}
	if req.Year != nil {
		series.Year = *req.Year
	}
	if req.Rating != nil {
		series.Rating = *req.Rating
	}
	if req.Quality != nil {
		series.Quality = *req.Quality
	}
	if req.AudioType != nil {
		series.AudioType = *req.AudioType
	}
	if req.TrailerYoutubeID != nil {
		series.TrailerYoutubeID = *req.TrailerYoutubeID
	}
	if req.TotalEpisodes != nil {
		series.TotalEpisodes = *req.TotalEpisodes
	}
	if req.IsCompleted != nil {
		series.IsCompleted = *req.IsCompleted
	}
	if req.CategoryID != nil {
		series.SeriesCategoryID = req.CategoryID
	}
	if req.Status != nil {
		series.Status = *req.Status
	}
	if req.PosterPath != nil {
		series.PosterPath = *req.PosterPath
	}

	series.UpdatedAt = time.Now()

	if err := s.seriesRepo.Update(ctx, series); err != nil {
		logger.ErrorContext(ctx, "Failed to update series", "error", err, "series_id", id)
		return nil, err
	}

	return series, nil
}

func (s *SeriesServiceImpl) Delete(ctx context.Context, id uuid.UUID) error {
	// CASCADE จะลบ episodes ให้อัตโนมัติ
	return s.seriesRepo.Delete(ctx, id)
}

func (s *SeriesServiceImpl) List(ctx context.Context, filter *dto.SeriesFilterRequest) ([]*models.Series, int64, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 || limit > 100 {
		limit = 24
	}

	filters := map[string]interface{}{
		"search":     filter.Search,
		"audio_type": filter.AudioType,
		"status":     filter.Status,
		"sort_by":    filter.SortBy,
		"sort_order": filter.SortOrder,
	}

	if filter.Year > 0 {
		filters["year"] = filter.Year
	}
	if filter.CategoryID != "" {
		if catID, err := uuid.Parse(filter.CategoryID); err == nil {
			filters["category_id"] = catID
		}
	}

	return s.seriesRepo.List(ctx, page, limit, filters)
}

// ═══════════════════════════════════════════
// Upsert (Bot ใช้ — สร้างหรืออัพเดท by source)
// ═══════════════════════════════════════════

func (s *SeriesServiceImpl) Upsert(ctx context.Context, req *dto.UpsertSeriesRequest) (*models.Series, bool, error) {
	if req.SourceSite == "" || req.SourceID == 0 {
		return nil, false, errors.New("sourceSite and sourceId are required for upsert")
	}

	existing, _ := s.seriesRepo.GetBySource(ctx, req.SourceSite, req.SourceID)

	if existing != nil {
		// Update existing
		updateReq := &dto.UpdateSeriesRequest{
			Title:            &req.Title,
			ThaiTitle:        &req.ThaiTitle,
			Description:      &req.Description,
			Year:             &req.Year,
			Rating:           &req.Rating,
			Quality:          &req.Quality,
			AudioType:        &req.AudioType,
			TrailerYoutubeID: &req.TrailerYoutubeID,
			TotalEpisodes:    &req.TotalEpisodes,
			IsCompleted:      &req.IsCompleted,
			CategoryID:       req.CategoryID,
		}

		updated, err := s.Update(ctx, existing.ID, updateReq)
		if err != nil {
			return nil, false, err
		}
		return updated, false, nil
	}

	// Create new
	created, err := s.Create(ctx, &req.CreateSeriesRequest)
	if err != nil {
		return nil, false, err
	}
	return created, true, nil
}

// ═══════════════════════════════════════════
// Episodes
// ═══════════════════════════════════════════

func (s *SeriesServiceImpl) AddEpisodes(ctx context.Context, seriesID uuid.UUID, req *dto.AddEpisodesRequest) ([]models.SeriesEpisode, error) {
	// เช็คว่า series มีจริง
	_, err := s.seriesRepo.GetByID(ctx, seriesID)
	if err != nil {
		return nil, errors.New("series not found")
	}

	var newEpisodes []models.SeriesEpisode
	for _, item := range req.Episodes {
		// Skip ถ้ามีอยู่แล้ว
		existing, _ := s.episodeRepo.GetBySeriesAndEpisode(ctx, seriesID, item.EpisodeNumber)
		if existing != nil {
			continue
		}

		ep := models.SeriesEpisode{
			ID:            uuid.New(),
			SeriesID:      seriesID,
			EpisodeNumber: item.EpisodeNumber,
			SourceURL:     item.SourceURL,
			Status:        "pending",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		newEpisodes = append(newEpisodes, ep)
	}

	if len(newEpisodes) == 0 {
		return nil, nil
	}

	if err := s.episodeRepo.CreateBatch(ctx, newEpisodes); err != nil {
		logger.ErrorContext(ctx, "Failed to create episodes", "error", err, "series_id", seriesID)
		return nil, err
	}

	logger.InfoContext(ctx, "Episodes added", "series_id", seriesID, "count", len(newEpisodes))
	return newEpisodes, nil
}

func (s *SeriesServiceImpl) UpdateEpisode(ctx context.Context, seriesID uuid.UUID, episodeNumber int, req *dto.UpdateEpisodeRequest) (*models.SeriesEpisode, error) {
	ep, err := s.episodeRepo.GetBySeriesAndEpisode(ctx, seriesID, episodeNumber)
	if err != nil {
		return nil, errors.New("episode not found")
	}

	if req.VideoID != nil {
		ep.VideoID = req.VideoID
	}
	if req.Status != nil {
		ep.Status = *req.Status
	}
	if req.SourceURL != nil {
		ep.SourceURL = *req.SourceURL
	}
	ep.UpdatedAt = time.Now()

	if err := s.episodeRepo.Update(ctx, ep); err != nil {
		return nil, err
	}

	return ep, nil
}

func (s *SeriesServiceImpl) ListEpisodes(ctx context.Context, seriesID uuid.UUID) ([]models.SeriesEpisode, error) {
	return s.episodeRepo.ListBySeriesID(ctx, seriesID)
}

// ═══════════════════════════════════════════
// Helpers
// ═══════════════════════════════════════════

func (s *SeriesServiceImpl) generateCode(seriesSlug string, year int) string {
	// "reverse" + 2026 → "REVERSE2026"
	// ถ้ายาวเกิน 50 ตัด
	clean := strings.ToUpper(slug.Make(seriesSlug))
	clean = strings.ReplaceAll(clean, "-", "")
	if year > 0 {
		code := fmt.Sprintf("%s%d", clean, year)
		if len(code) > 50 {
			code = code[:50]
		}
		return code
	}
	if len(clean) > 50 {
		clean = clean[:50]
	}
	return clean
}
