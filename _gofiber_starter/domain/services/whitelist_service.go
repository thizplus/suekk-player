package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
)

// WhitelistService interface สำหรับจัดการ Whitelist Profiles และ Ad Stats
type WhitelistService interface {
	// ==================== Profile Management ====================

	// CreateProfile สร้าง profile ใหม่
	CreateProfile(ctx context.Context, req *dto.CreateWhitelistProfileRequest) (*models.WhitelistProfile, error)

	// GetProfile ดึง profile ตาม ID
	GetProfile(ctx context.Context, id uuid.UUID) (*models.WhitelistProfile, error)

	// GetProfileWithDomains ดึง profile พร้อม domains ทั้งหมด
	GetProfileWithDomains(ctx context.Context, id uuid.UUID) (*models.WhitelistProfile, error)

	// UpdateProfile อัพเดท profile
	UpdateProfile(ctx context.Context, id uuid.UUID, req *dto.UpdateWhitelistProfileRequest) (*models.WhitelistProfile, error)

	// DeleteProfile ลบ profile (cascade delete domains ด้วย)
	DeleteProfile(ctx context.Context, id uuid.UUID) error

	// ListProfiles ดึง profiles ทั้งหมดพร้อม pagination
	ListProfiles(ctx context.Context, page, limit int) ([]*models.WhitelistProfile, int64, error)

	// ListActiveProfiles ดึง active profiles ทั้งหมด
	ListActiveProfiles(ctx context.Context) ([]*models.WhitelistProfile, error)

	// ==================== Domain Management ====================

	// AddDomain เพิ่ม domain ใน profile
	AddDomain(ctx context.Context, profileID uuid.UUID, domain string) (*models.ProfileDomain, error)

	// RemoveDomain ลบ domain ออกจาก profile
	RemoveDomain(ctx context.Context, domainID uuid.UUID) error

	// GetDomainsByProfile ดึง domains ทั้งหมดของ profile
	GetDomainsByProfile(ctx context.Context, profileID uuid.UUID) ([]*models.ProfileDomain, error)

	// ==================== Domain Lookup (สำหรับ Middleware) ====================

	// FindProfileByDomain ค้นหา profile จาก domain (รองรับ wildcard)
	FindProfileByDomain(ctx context.Context, domain string) (*models.WhitelistProfile, error)

	// IsDomainAllowed ตรวจสอบว่า domain ได้รับอนุญาตหรือไม่
	IsDomainAllowed(ctx context.Context, domain string) (bool, *models.WhitelistProfile, error)

	// ==================== Watermark ====================

	// UpdateWatermark อัพเดท URL ของ watermark
	UpdateWatermark(ctx context.Context, profileID uuid.UUID, watermarkURL string) error

	// ==================== Ad Statistics ====================

	// RecordAdImpression บันทึก ad impression
	RecordAdImpression(ctx context.Context, req *dto.RecordAdImpressionRequest) error

	// GetAdStats ดึงสถิติ ads ในช่วงเวลา
	GetAdStats(ctx context.Context, start, end time.Time) (*models.AdImpressionStats, error)

	// GetAdStatsByProfile ดึงสถิติ ads ของ profile
	GetAdStatsByProfile(ctx context.Context, profileID uuid.UUID, start, end time.Time) (*models.AdImpressionStats, error)

	// GetDeviceStats ดึงสถิติแยกตามอุปกรณ์
	GetDeviceStats(ctx context.Context, start, end time.Time) (*models.DeviceStats, error)

	// GetProfileRanking ดึง ranking ของ profiles ตามจำนวน views
	GetProfileRanking(ctx context.Context, start, end time.Time, limit int) ([]*models.ProfileAdStats, error)

	// GetSkipTimeDistribution ดึง distribution ของ skip time
	GetSkipTimeDistribution(ctx context.Context, start, end time.Time) (map[int]int64, error)

	// CleanupOldStats ลบข้อมูลเก่ากว่าจำนวนวันที่กำหนด
	CleanupOldStats(ctx context.Context, days int) (int64, error)

	// ==================== Preroll Ads ====================

	// AddPrerollAd เพิ่ม preroll ad ใน profile
	AddPrerollAd(ctx context.Context, profileID uuid.UUID, req *dto.AddPrerollAdRequest) (*models.PrerollAd, error)

	// UpdatePrerollAd อัพเดท preroll ad
	UpdatePrerollAd(ctx context.Context, prerollID uuid.UUID, req *dto.UpdatePrerollAdRequest) (*models.PrerollAd, error)

	// DeletePrerollAd ลบ preroll ad
	DeletePrerollAd(ctx context.Context, prerollID uuid.UUID) error

	// GetPrerollAdsByProfile ดึง preroll ads ทั้งหมดของ profile
	GetPrerollAdsByProfile(ctx context.Context, profileID uuid.UUID) ([]*models.PrerollAd, error)

	// ReorderPrerollAds จัดลำดับ preroll ads ใหม่
	ReorderPrerollAds(ctx context.Context, profileID uuid.UUID, prerollIDs []uuid.UUID) error

	// ==================== Cache Management ====================

	// InvalidateDomainCache ลบ cache ของ domain เดียว
	InvalidateDomainCache(ctx context.Context, domain string) error

	// InvalidateAllCache ลบ cache ทั้งหมด (สำหรับ Admin)
	InvalidateAllCache(ctx context.Context) (int64, error)
}
