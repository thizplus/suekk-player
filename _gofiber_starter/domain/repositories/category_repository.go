package repositories

import (
	"context"

	"github.com/google/uuid"
	"gofiber-template/domain/models"
)

type CategoryRepository interface {
	Create(ctx context.Context, category *models.Category) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Category, error)
	GetBySlug(ctx context.Context, slug string) (*models.Category, error)
	GetByName(ctx context.Context, name string) (*models.Category, error)
	Update(ctx context.Context, category *models.Category) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context) ([]*models.Category, error)
	ListTree(ctx context.Context) ([]*models.Category, error)
	GetMaxSortOrder(ctx context.Context, parentID *uuid.UUID) (int, error)
	UpdateMany(ctx context.Context, categories []*models.Category) error
	Count(ctx context.Context) (int64, error)
	// GetVideoCounts คืน map ของ category_id -> จำนวนวิดีโอ
	GetVideoCounts(ctx context.Context) (map[uuid.UUID]int64, error)
}
