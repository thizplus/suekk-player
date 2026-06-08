package serviceimpl

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	slugify "github.com/gosimple/slug"

	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
)

type SeriesCategoryServiceImpl struct {
	repo repositories.SeriesCategoryRepository
}

func NewSeriesCategoryService(repo repositories.SeriesCategoryRepository) services.SeriesCategoryService {
	return &SeriesCategoryServiceImpl{repo: repo}
}

func (s *SeriesCategoryServiceImpl) Create(ctx context.Context, name, slug string, parentID *uuid.UUID) (*models.SeriesCategory, error) {
	existing, _ := s.repo.GetBySlug(ctx, slug)
	if existing != nil {
		return nil, errors.New("series category slug already exists")
	}

	cat := &models.SeriesCategory{
		ID:        uuid.New(),
		Name:      name,
		Slug:      slugify.Make(slug),
		ParentID:  parentID,
		CreatedAt: time.Now(),
	}

	if err := s.repo.Create(ctx, cat); err != nil {
		logger.ErrorContext(ctx, "Failed to create series category", "error", err)
		return nil, err
	}

	logger.InfoContext(ctx, "Series category created", "id", cat.ID, "name", cat.Name)
	return cat, nil
}

func (s *SeriesCategoryServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*models.SeriesCategory, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *SeriesCategoryServiceImpl) GetOrCreateByName(ctx context.Context, name string) (*models.SeriesCategory, error) {
	sl := slugify.Make(name)
	existing, _ := s.repo.GetBySlug(ctx, sl)
	if existing != nil {
		return existing, nil
	}

	return s.Create(ctx, name, sl, nil)
}

func (s *SeriesCategoryServiceImpl) Update(ctx context.Context, id uuid.UUID, name, slug string) (*models.SeriesCategory, error) {
	cat, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.New("series category not found")
	}

	cat.Name = name
	cat.Slug = slugify.Make(slug)

	if err := s.repo.Update(ctx, cat); err != nil {
		return nil, err
	}
	return cat, nil
}

func (s *SeriesCategoryServiceImpl) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *SeriesCategoryServiceImpl) List(ctx context.Context) ([]*models.SeriesCategory, error) {
	return s.repo.List(ctx)
}

func (s *SeriesCategoryServiceImpl) GetSeriesCounts(ctx context.Context) (map[uuid.UUID]int64, error) {
	return s.repo.GetSeriesCounts(ctx)
}
