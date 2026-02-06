package serviceimpl

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/gosimple/slug"

	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
)

type CategoryServiceImpl struct {
	categoryRepo repositories.CategoryRepository
}

func NewCategoryService(categoryRepo repositories.CategoryRepository) services.CategoryService {
	return &CategoryServiceImpl{
		categoryRepo: categoryRepo,
	}
}

func (s *CategoryServiceImpl) Create(ctx context.Context, req *dto.CreateCategoryRequest) (*models.Category, error) {
	// ตรวจสอบว่า slug ซ้ำหรือไม่
	existing, _ := s.categoryRepo.GetBySlug(ctx, req.Slug)
	if existing != nil {
		logger.WarnContext(ctx, "Category slug already exists", "slug", req.Slug)
		return nil, errors.New("category slug already exists")
	}

	// หา max sort order
	maxOrder, _ := s.categoryRepo.GetMaxSortOrder(ctx, req.ParentID)

	category := &models.Category{
		ID:        uuid.New(),
		Name:      req.Name,
		Slug:      slug.Make(req.Slug),
		ParentID:  req.ParentID,
		SortOrder: maxOrder + 1,
		CreatedAt: time.Now(),
	}

	if err := s.categoryRepo.Create(ctx, category); err != nil {
		logger.ErrorContext(ctx, "Failed to create category", "error", err)
		return nil, err
	}

	logger.InfoContext(ctx, "Category created", "category_id", category.ID, "name", category.Name)
	return category, nil
}

func (s *CategoryServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*models.Category, error) {
	category, err := s.categoryRepo.GetByID(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "Category not found", "category_id", id)
		return nil, errors.New("category not found")
	}
	return category, nil
}

func (s *CategoryServiceImpl) GetBySlug(ctx context.Context, slugStr string) (*models.Category, error) {
	category, err := s.categoryRepo.GetBySlug(ctx, slugStr)
	if err != nil {
		logger.WarnContext(ctx, "Category not found", "slug", slugStr)
		return nil, errors.New("category not found")
	}
	return category, nil
}

func (s *CategoryServiceImpl) Update(ctx context.Context, id uuid.UUID, req *dto.UpdateCategoryRequest) (*models.Category, error) {
	category, err := s.categoryRepo.GetByID(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "Category not found for update", "category_id", id)
		return nil, errors.New("category not found")
	}

	if req.Name != nil {
		category.Name = *req.Name
	}
	if req.Slug != nil {
		// ตรวจสอบว่า slug ใหม่ซ้ำหรือไม่
		newSlug := slug.Make(*req.Slug)
		existing, _ := s.categoryRepo.GetBySlug(ctx, newSlug)
		if existing != nil && existing.ID != id {
			logger.WarnContext(ctx, "Category slug already exists", "slug", newSlug)
			return nil, errors.New("category slug already exists")
		}
		category.Slug = newSlug
	}

	if err := s.categoryRepo.Update(ctx, category); err != nil {
		logger.ErrorContext(ctx, "Failed to update category", "category_id", id, "error", err)
		return nil, err
	}

	logger.InfoContext(ctx, "Category updated", "category_id", id)
	return category, nil
}

func (s *CategoryServiceImpl) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.categoryRepo.GetByID(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "Category not found for deletion", "category_id", id)
		return errors.New("category not found")
	}

	// TODO: ตรวจสอบว่ามี video ใช้ category นี้อยู่หรือไม่

	if err := s.categoryRepo.Delete(ctx, id); err != nil {
		logger.ErrorContext(ctx, "Failed to delete category", "category_id", id, "error", err)
		return err
	}

	logger.InfoContext(ctx, "Category deleted", "category_id", id)
	return nil
}

func (s *CategoryServiceImpl) List(ctx context.Context) ([]*models.Category, error) {
	categories, err := s.categoryRepo.List(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to list categories", "error", err)
		return nil, err
	}
	return categories, nil
}

func (s *CategoryServiceImpl) ListTree(ctx context.Context) ([]*models.Category, error) {
	categories, err := s.categoryRepo.ListTree(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to list categories tree", "error", err)
		return nil, err
	}
	return categories, nil
}

func (s *CategoryServiceImpl) Reorder(ctx context.Context, req *dto.ReorderCategoriesRequest) error {
	categories := make([]*models.Category, len(req.Categories))
	for i, item := range req.Categories {
		categories[i] = &models.Category{
			ID:        item.ID,
			ParentID:  item.ParentID,
			SortOrder: item.SortOrder,
		}
	}

	if err := s.categoryRepo.UpdateMany(ctx, categories); err != nil {
		logger.ErrorContext(ctx, "Failed to reorder categories", "error", err)
		return err
	}

	logger.InfoContext(ctx, "Categories reordered", "count", len(categories))
	return nil
}

func (s *CategoryServiceImpl) GetVideoCounts(ctx context.Context) (map[uuid.UUID]int64, error) {
	counts, err := s.categoryRepo.GetVideoCounts(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get video counts", "error", err)
		return nil, err
	}
	return counts, nil
}

func (s *CategoryServiceImpl) GetOrCreateByName(ctx context.Context, name string) (*models.Category, error) {
	// ลองหาจากชื่อก่อน
	category, err := s.categoryRepo.GetByName(ctx, name)
	if err == nil && category != nil {
		return category, nil
	}

	// ถ้าไม่มี ก็สร้างใหม่
	logger.InfoContext(ctx, "Creating new category", "name", name)

	// หา max sort order
	maxOrder, _ := s.categoryRepo.GetMaxSortOrder(ctx, nil)

	newCategory := &models.Category{
		ID:        uuid.New(),
		Name:      name,
		Slug:      slug.Make(name),
		SortOrder: maxOrder + 1,
		CreatedAt: time.Now(),
	}

	if err := s.categoryRepo.Create(ctx, newCategory); err != nil {
		logger.ErrorContext(ctx, "Failed to create category", "name", name, "error", err)
		return nil, err
	}

	logger.InfoContext(ctx, "Category created", "category_id", newCategory.ID, "name", name)
	return newCategory, nil
}
