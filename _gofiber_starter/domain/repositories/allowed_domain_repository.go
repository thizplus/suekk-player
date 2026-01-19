package repositories

import (
	"context"

	"github.com/google/uuid"
	"gofiber-template/domain/models"
)

type AllowedDomainRepository interface {
	Create(ctx context.Context, domain *models.AllowedDomain) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.AllowedDomain, error)
	GetByVideoID(ctx context.Context, videoID uuid.UUID) ([]*models.AllowedDomain, error)
	GetByDomain(ctx context.Context, domain string) ([]*models.AllowedDomain, error)
	CheckDomainAllowed(ctx context.Context, videoID uuid.UUID, domain string) (bool, error)
	Update(ctx context.Context, domain *models.AllowedDomain) error
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByVideoID(ctx context.Context, videoID uuid.UUID) error
	DeleteByDomain(ctx context.Context, videoID uuid.UUID, domain string) error
	Count(ctx context.Context) (int64, error)
	CountByVideoID(ctx context.Context, videoID uuid.UUID) (int64, error)
}
