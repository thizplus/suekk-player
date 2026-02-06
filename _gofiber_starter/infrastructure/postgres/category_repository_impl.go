package postgres

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
)

type CategoryRepositoryImpl struct {
	db *gorm.DB
}

func NewCategoryRepository(db *gorm.DB) repositories.CategoryRepository {
	return &CategoryRepositoryImpl{db: db}
}

func (r *CategoryRepositoryImpl) Create(ctx context.Context, category *models.Category) error {
	return r.db.WithContext(ctx).Create(category).Error
}

func (r *CategoryRepositoryImpl) GetByID(ctx context.Context, id uuid.UUID) (*models.Category, error) {
	var category models.Category
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&category).Error
	if err != nil {
		return nil, err
	}
	return &category, nil
}

func (r *CategoryRepositoryImpl) GetBySlug(ctx context.Context, slug string) (*models.Category, error) {
	var category models.Category
	err := r.db.WithContext(ctx).Where("slug = ?", slug).First(&category).Error
	if err != nil {
		return nil, err
	}
	return &category, nil
}

func (r *CategoryRepositoryImpl) GetByName(ctx context.Context, name string) (*models.Category, error) {
	var category models.Category
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&category).Error
	if err != nil {
		return nil, err
	}
	return &category, nil
}

func (r *CategoryRepositoryImpl) Update(ctx context.Context, category *models.Category) error {
	return r.db.WithContext(ctx).Save(category).Error
}

func (r *CategoryRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	// ลบ children ก่อน (cascade)
	r.db.WithContext(ctx).Where("parent_id = ?", id).Delete(&models.Category{})
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.Category{}).Error
}

func (r *CategoryRepositoryImpl) List(ctx context.Context) ([]*models.Category, error) {
	var categories []*models.Category
	err := r.db.WithContext(ctx).Order("sort_order ASC, name ASC").Find(&categories).Error
	return categories, err
}

func (r *CategoryRepositoryImpl) ListTree(ctx context.Context) ([]*models.Category, error) {
	var categories []*models.Category
	// ดึงเฉพาะ root categories (parent_id IS NULL) พร้อม preload children
	err := r.db.WithContext(ctx).
		Where("parent_id IS NULL").
		Order("sort_order ASC, name ASC").
		Preload("Children", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, name ASC")
		}).
		Find(&categories).Error
	return categories, err
}

func (r *CategoryRepositoryImpl) GetMaxSortOrder(ctx context.Context, parentID *uuid.UUID) (int, error) {
	var maxOrder int
	query := r.db.WithContext(ctx).Model(&models.Category{})
	if parentID == nil {
		query = query.Where("parent_id IS NULL")
	} else {
		query = query.Where("parent_id = ?", parentID)
	}
	err := query.Select("COALESCE(MAX(sort_order), 0)").Scan(&maxOrder).Error
	return maxOrder, err
}

func (r *CategoryRepositoryImpl) UpdateMany(ctx context.Context, categories []*models.Category) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, cat := range categories {
			if err := tx.Model(&models.Category{}).Where("id = ?", cat.ID).Updates(map[string]interface{}{
				"parent_id":  cat.ParentID,
				"sort_order": cat.SortOrder,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *CategoryRepositoryImpl) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.Category{}).Count(&count).Error
	return count, err
}

// GetVideoCounts คืน map ของ category_id -> จำนวนวิดีโอ
func (r *CategoryRepositoryImpl) GetVideoCounts(ctx context.Context) (map[uuid.UUID]int64, error) {
	type result struct {
		CategoryID uuid.UUID
		Count      int64
	}
	var results []result

	err := r.db.WithContext(ctx).
		Model(&models.Video{}).
		Select("category_id, COUNT(*) as count").
		Where("category_id IS NOT NULL").
		Group("category_id").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	counts := make(map[uuid.UUID]int64)
	for _, r := range results {
		counts[r.CategoryID] = r.Count
	}
	return counts, nil
}
