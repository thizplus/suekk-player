package repositories

import (
	"context"

	"github.com/google/uuid"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
)

type ReelRepository interface {
	// Reel CRUD
	Create(ctx context.Context, reel *models.Reel) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Reel, error)
	GetByIDWithRelations(ctx context.Context, id uuid.UUID) (*models.Reel, error)
	Update(ctx context.Context, reel *models.Reel) error
	Delete(ctx context.Context, id uuid.UUID) error

	// List & Filter
	ListByUserID(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*models.Reel, int64, error)
	ListByVideoID(ctx context.Context, videoID uuid.UUID, offset, limit int) ([]*models.Reel, int64, error)
	ListWithFilters(ctx context.Context, userID uuid.UUID, params *dto.ReelFilterRequest) ([]*models.Reel, int64, error)

	// Status
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.ReelStatus, errorMsg string) error
	GetByStatus(ctx context.Context, status models.ReelStatus, offset, limit int) ([]*models.Reel, error)

	// Output
	UpdateOutput(ctx context.Context, id uuid.UUID, outputPath, thumbnailURL string, duration int, fileSize int64) error

	// Count
	CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
	CountByVideoID(ctx context.Context, videoID uuid.UUID) (int64, error)
	CountByVideoIDs(ctx context.Context, videoIDs []uuid.UUID) (map[uuid.UUID]int64, error) // Batch count for multiple videos
	CountByStatus(ctx context.Context, status models.ReelStatus) (int64, error)
}

type ReelTemplateRepository interface {
	// Template CRUD
	Create(ctx context.Context, template *models.ReelTemplate) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.ReelTemplate, error)
	Update(ctx context.Context, template *models.ReelTemplate) error
	Delete(ctx context.Context, id uuid.UUID) error

	// List
	ListActive(ctx context.Context) ([]*models.ReelTemplate, error)
	ListAll(ctx context.Context) ([]*models.ReelTemplate, error)
}
