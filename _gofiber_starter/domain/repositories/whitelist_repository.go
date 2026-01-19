package repositories

import (
	"context"

	"github.com/google/uuid"
	"gofiber-template/domain/models"
)

// WhitelistRepository interface สำหรับจัดการ Whitelist Profiles
type WhitelistRepository interface {
	// Profile CRUD
	Create(ctx context.Context, profile *models.WhitelistProfile) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.WhitelistProfile, error)
	GetByIDWithDomains(ctx context.Context, id uuid.UUID) (*models.WhitelistProfile, error)
	Update(ctx context.Context, profile *models.WhitelistProfile) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, offset, limit int) ([]*models.WhitelistProfile, error)
	ListWithDomains(ctx context.Context, offset, limit int) ([]*models.WhitelistProfile, error)
	Count(ctx context.Context) (int64, error)
	ListActive(ctx context.Context) ([]*models.WhitelistProfile, error)

	// Domain management
	AddDomain(ctx context.Context, domain *models.ProfileDomain) error
	GetDomainByID(ctx context.Context, domainID uuid.UUID) (*models.ProfileDomain, error)
	RemoveDomain(ctx context.Context, domainID uuid.UUID) error
	GetDomainsByProfileID(ctx context.Context, profileID uuid.UUID) ([]*models.ProfileDomain, error)

	// Domain lookup (สำหรับ middleware)
	FindProfileByDomain(ctx context.Context, domain string) (*models.WhitelistProfile, error)
	GetAllDomains(ctx context.Context) ([]*models.ProfileDomain, error)

	// Watermark
	UpdateWatermarkURL(ctx context.Context, profileID uuid.UUID, url string) error

	// Preroll Ads
	AddPrerollAd(ctx context.Context, preroll *models.PrerollAd) error
	GetPrerollAdByID(ctx context.Context, prerollID uuid.UUID) (*models.PrerollAd, error)
	UpdatePrerollAd(ctx context.Context, preroll *models.PrerollAd) error
	DeletePrerollAd(ctx context.Context, prerollID uuid.UUID) error
	GetPrerollAdsByProfileID(ctx context.Context, profileID uuid.UUID) ([]*models.PrerollAd, error)
	ReorderPrerollAds(ctx context.Context, profileID uuid.UUID, prerollIDs []uuid.UUID) error
}
