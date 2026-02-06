package services

import (
	"context"

	"github.com/google/uuid"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
)

type CategoryService interface {
	// Create สร้าง category ใหม่
	Create(ctx context.Context, req *dto.CreateCategoryRequest) (*models.Category, error)

	// GetByID ดึง category ตาม ID
	GetByID(ctx context.Context, id uuid.UUID) (*models.Category, error)

	// GetBySlug ดึง category ตาม slug
	GetBySlug(ctx context.Context, slug string) (*models.Category, error)

	// GetOrCreateByName หา category ตามชื่อ ถ้าไม่มีก็สร้างใหม่
	GetOrCreateByName(ctx context.Context, name string) (*models.Category, error)

	// Update อัปเดต category
	Update(ctx context.Context, id uuid.UUID, req *dto.UpdateCategoryRequest) (*models.Category, error)

	// Delete ลบ category
	Delete(ctx context.Context, id uuid.UUID) error

	// List ดึง categories ทั้งหมด (flat list)
	List(ctx context.Context) ([]*models.Category, error)

	// ListTree ดึง categories แบบ tree structure
	ListTree(ctx context.Context) ([]*models.Category, error)

	// Reorder จัดเรียง categories ใหม่
	Reorder(ctx context.Context, req *dto.ReorderCategoriesRequest) error

	// GetVideoCounts ดึงจำนวนวิดีโอในแต่ละ category
	GetVideoCounts(ctx context.Context) (map[uuid.UUID]int64, error)
}
