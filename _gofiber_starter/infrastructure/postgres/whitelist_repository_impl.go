package postgres

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
)

type WhitelistRepositoryImpl struct {
	db *gorm.DB
}

func NewWhitelistRepository(db *gorm.DB) repositories.WhitelistRepository {
	return &WhitelistRepositoryImpl{db: db}
}

// ==================== Profile CRUD ====================

func (r *WhitelistRepositoryImpl) Create(ctx context.Context, profile *models.WhitelistProfile) error {
	return r.db.WithContext(ctx).Create(profile).Error
}

func (r *WhitelistRepositoryImpl) GetByID(ctx context.Context, id uuid.UUID) (*models.WhitelistProfile, error) {
	var profile models.WhitelistProfile
	err := r.db.WithContext(ctx).First(&profile, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *WhitelistRepositoryImpl) GetByIDWithDomains(ctx context.Context, id uuid.UUID) (*models.WhitelistProfile, error) {
	var profile models.WhitelistProfile
	err := r.db.WithContext(ctx).
		Preload("Domains").
		Preload("PrerollAds", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC")
		}).
		First(&profile, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *WhitelistRepositoryImpl) Update(ctx context.Context, profile *models.WhitelistProfile) error {
	return r.db.WithContext(ctx).Save(profile).Error
}

func (r *WhitelistRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	// Domains จะถูกลบอัตโนมัติเพราะ ON DELETE CASCADE
	return r.db.WithContext(ctx).Delete(&models.WhitelistProfile{}, "id = ?", id).Error
}

func (r *WhitelistRepositoryImpl) List(ctx context.Context, offset, limit int) ([]*models.WhitelistProfile, error) {
	var profiles []*models.WhitelistProfile
	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&profiles).Error
	return profiles, err
}

func (r *WhitelistRepositoryImpl) ListWithDomains(ctx context.Context, offset, limit int) ([]*models.WhitelistProfile, error) {
	var profiles []*models.WhitelistProfile
	err := r.db.WithContext(ctx).
		Preload("Domains").
		Preload("PrerollAds", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC")
		}).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&profiles).Error
	return profiles, err
}

func (r *WhitelistRepositoryImpl) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.WhitelistProfile{}).Count(&count).Error
	return count, err
}

func (r *WhitelistRepositoryImpl) ListActive(ctx context.Context) ([]*models.WhitelistProfile, error) {
	var profiles []*models.WhitelistProfile
	err := r.db.WithContext(ctx).
		Preload("Domains").
		Preload("PrerollAds", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC")
		}).
		Where("is_active = ?", true).
		Order("name ASC").
		Find(&profiles).Error
	return profiles, err
}

// ==================== Domain Management ====================

func (r *WhitelistRepositoryImpl) AddDomain(ctx context.Context, domain *models.ProfileDomain) error {
	return r.db.WithContext(ctx).Create(domain).Error
}

func (r *WhitelistRepositoryImpl) GetDomainByID(ctx context.Context, domainID uuid.UUID) (*models.ProfileDomain, error) {
	var domain models.ProfileDomain
	err := r.db.WithContext(ctx).First(&domain, "id = ?", domainID).Error
	if err != nil {
		return nil, err
	}
	return &domain, nil
}

func (r *WhitelistRepositoryImpl) RemoveDomain(ctx context.Context, domainID uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.ProfileDomain{}, "id = ?", domainID).Error
}

func (r *WhitelistRepositoryImpl) GetDomainsByProfileID(ctx context.Context, profileID uuid.UUID) ([]*models.ProfileDomain, error) {
	var domains []*models.ProfileDomain
	err := r.db.WithContext(ctx).
		Where("profile_id = ?", profileID).
		Order("domain ASC").
		Find(&domains).Error
	return domains, err
}

// ==================== Domain Lookup ====================

// FindProfileByDomain ค้นหา Profile จาก domain (สำหรับ middleware)
// ใช้ domain matching logic รองรับ wildcard
func (r *WhitelistRepositoryImpl) FindProfileByDomain(ctx context.Context, domain string) (*models.WhitelistProfile, error) {
	// ดึง domains ทั้งหมดของ active profiles
	var allDomains []*models.ProfileDomain
	err := r.db.WithContext(ctx).
		Joins("JOIN whitelist_profiles ON whitelist_profiles.id = profile_domains.profile_id").
		Where("whitelist_profiles.is_active = ?", true).
		Find(&allDomains).Error
	if err != nil {
		return nil, err
	}

	// ค้นหา domain ที่ match
	for _, d := range allDomains {
		if models.MatchDomain(d.Domain, domain) {
			// ดึง profile พร้อม domains
			return r.GetByIDWithDomains(ctx, d.ProfileID)
		}
	}

	return nil, gorm.ErrRecordNotFound
}

func (r *WhitelistRepositoryImpl) GetAllDomains(ctx context.Context) ([]*models.ProfileDomain, error) {
	var domains []*models.ProfileDomain
	err := r.db.WithContext(ctx).
		Preload("Profile").
		Find(&domains).Error
	return domains, err
}

// ==================== Watermark ====================

func (r *WhitelistRepositoryImpl) UpdateWatermarkURL(ctx context.Context, profileID uuid.UUID, url string) error {
	return r.db.WithContext(ctx).
		Model(&models.WhitelistProfile{}).
		Where("id = ?", profileID).
		Update("watermark_url", url).Error
}

// ==================== Preroll Ads ====================

func (r *WhitelistRepositoryImpl) AddPrerollAd(ctx context.Context, preroll *models.PrerollAd) error {
	// หา sort_order สูงสุดของ profile นี้
	var maxOrder int
	r.db.WithContext(ctx).
		Model(&models.PrerollAd{}).
		Where("profile_id = ?", preroll.ProfileID).
		Select("COALESCE(MAX(sort_order), -1)").
		Scan(&maxOrder)

	preroll.SortOrder = maxOrder + 1
	return r.db.WithContext(ctx).Create(preroll).Error
}

func (r *WhitelistRepositoryImpl) GetPrerollAdByID(ctx context.Context, prerollID uuid.UUID) (*models.PrerollAd, error) {
	var preroll models.PrerollAd
	err := r.db.WithContext(ctx).Where("id = ?", prerollID).First(&preroll).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &preroll, nil
}

func (r *WhitelistRepositoryImpl) UpdatePrerollAd(ctx context.Context, preroll *models.PrerollAd) error {
	return r.db.WithContext(ctx).Save(preroll).Error
}

func (r *WhitelistRepositoryImpl) DeletePrerollAd(ctx context.Context, prerollID uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.PrerollAd{}, "id = ?", prerollID).Error
}

func (r *WhitelistRepositoryImpl) GetPrerollAdsByProfileID(ctx context.Context, profileID uuid.UUID) ([]*models.PrerollAd, error) {
	var prerolls []*models.PrerollAd
	err := r.db.WithContext(ctx).
		Where("profile_id = ?", profileID).
		Order("sort_order ASC").
		Find(&prerolls).Error
	return prerolls, err
}

func (r *WhitelistRepositoryImpl) ReorderPrerollAds(ctx context.Context, profileID uuid.UUID, prerollIDs []uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, id := range prerollIDs {
			if err := tx.Model(&models.PrerollAd{}).
				Where("id = ? AND profile_id = ?", id, profileID).
				Update("sort_order", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
