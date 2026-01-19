package serviceimpl

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
	"gofiber-template/domain/services"
	"gofiber-template/infrastructure/redis"
	"gofiber-template/pkg/logger"
)

const (
	// Cache keys และ TTLs
	whitelistCachePrefix = "whitelist:"
	whitelistCacheTTL    = 5 * time.Minute  // Cache whitelist lookup 5 นาที
	negativeCacheTTL     = 1 * time.Minute  // Cache negative result 1 นาที (ป้องกัน attack)
	nullCacheValue       = "null"           // Value สำหรับ negative cache
)

type WhitelistServiceImpl struct {
	whitelistRepo repositories.WhitelistRepository
	adStatsRepo   repositories.AdStatsRepository
	redisClient   *redis.Client // optional - ถ้าไม่มีจะ query DB ตลอด
}

func NewWhitelistService(
	whitelistRepo repositories.WhitelistRepository,
	adStatsRepo repositories.AdStatsRepository,
) services.WhitelistService {
	return &WhitelistServiceImpl{
		whitelistRepo: whitelistRepo,
		adStatsRepo:   adStatsRepo,
		redisClient:   nil,
	}
}

// NewWhitelistServiceWithCache สร้าง whitelist service พร้อม Redis cache
func NewWhitelistServiceWithCache(
	whitelistRepo repositories.WhitelistRepository,
	adStatsRepo repositories.AdStatsRepository,
	redisClient *redis.Client,
) services.WhitelistService {
	return &WhitelistServiceImpl{
		whitelistRepo: whitelistRepo,
		adStatsRepo:   adStatsRepo,
		redisClient:   redisClient,
	}
}

// ==================== Profile Management ====================

func (s *WhitelistServiceImpl) CreateProfile(ctx context.Context, req *dto.CreateWhitelistProfileRequest) (*models.WhitelistProfile, error) {
	logger.InfoContext(ctx, "Creating whitelist profile", "name", req.Name)

	// สร้าง profile
	profile := &models.WhitelistProfile{
		Name:              req.Name,
		Description:       req.Description,
		IsActive:          req.IsActive,
		ThumbnailURL:      req.ThumbnailURL,
		WatermarkEnabled:  req.WatermarkEnabled,
		WatermarkURL:      req.WatermarkURL,
		WatermarkPosition: req.WatermarkPosition,
		WatermarkOpacity:  req.WatermarkOpacity,
		WatermarkSize:     req.WatermarkSize,
		WatermarkOffsetY:  req.WatermarkOffsetY,
		PrerollEnabled:    req.PrerollEnabled,
		PrerollURL:        req.PrerollURL,
		PrerollSkipAfter:  req.PrerollSkipAfter,
	}

	// Set defaults
	if profile.WatermarkPosition == "" {
		profile.WatermarkPosition = "bottom-right"
	}
	if profile.WatermarkOpacity == 0 {
		profile.WatermarkOpacity = 0.7
	}
	if profile.WatermarkSize == 0 {
		profile.WatermarkSize = 80
	}

	if err := s.whitelistRepo.Create(ctx, profile); err != nil {
		logger.ErrorContext(ctx, "Failed to create whitelist profile", "error", err)
		return nil, err
	}

	// เพิ่ม initial domains ถ้ามี
	for _, domainStr := range req.Domains {
		domain := &models.ProfileDomain{
			ProfileID: profile.ID,
			Domain:    domainStr,
		}
		if err := s.whitelistRepo.AddDomain(ctx, domain); err != nil {
			logger.WarnContext(ctx, "Failed to add initial domain",
				"profile_id", profile.ID,
				"domain", domainStr,
				"error", err,
			)
		}
	}

	logger.InfoContext(ctx, "Whitelist profile created",
		"profile_id", profile.ID,
		"name", profile.Name,
		"domains_count", len(req.Domains),
	)

	// ดึง profile พร้อม domains
	return s.whitelistRepo.GetByIDWithDomains(ctx, profile.ID)
}

func (s *WhitelistServiceImpl) GetProfile(ctx context.Context, id uuid.UUID) (*models.WhitelistProfile, error) {
	return s.whitelistRepo.GetByID(ctx, id)
}

func (s *WhitelistServiceImpl) GetProfileWithDomains(ctx context.Context, id uuid.UUID) (*models.WhitelistProfile, error) {
	return s.whitelistRepo.GetByIDWithDomains(ctx, id)
}

func (s *WhitelistServiceImpl) UpdateProfile(ctx context.Context, id uuid.UUID, req *dto.UpdateWhitelistProfileRequest) (*models.WhitelistProfile, error) {
	logger.InfoContext(ctx, "Updating whitelist profile", "profile_id", id)

	profile, err := s.whitelistRepo.GetByID(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "Profile not found", "profile_id", id, "error", err)
		return nil, err
	}

	// Update fields if provided
	if req.Name != nil {
		profile.Name = *req.Name
	}
	if req.Description != nil {
		profile.Description = *req.Description
	}
	if req.IsActive != nil {
		profile.IsActive = *req.IsActive
	}
	if req.ThumbnailURL != nil {
		profile.ThumbnailURL = *req.ThumbnailURL
	}
	if req.WatermarkEnabled != nil {
		profile.WatermarkEnabled = *req.WatermarkEnabled
	}
	if req.WatermarkURL != nil {
		profile.WatermarkURL = *req.WatermarkURL
	}
	if req.WatermarkPosition != nil {
		profile.WatermarkPosition = *req.WatermarkPosition
	}
	if req.WatermarkOpacity != nil {
		profile.WatermarkOpacity = *req.WatermarkOpacity
	}
	if req.WatermarkSize != nil {
		profile.WatermarkSize = *req.WatermarkSize
	}
	if req.WatermarkOffsetY != nil {
		profile.WatermarkOffsetY = *req.WatermarkOffsetY
	}
	if req.PrerollEnabled != nil {
		profile.PrerollEnabled = *req.PrerollEnabled
	}
	if req.PrerollURL != nil {
		profile.PrerollURL = *req.PrerollURL
	}
	if req.PrerollSkipAfter != nil {
		profile.PrerollSkipAfter = *req.PrerollSkipAfter
	}

	if err := s.whitelistRepo.Update(ctx, profile); err != nil {
		logger.ErrorContext(ctx, "Failed to update whitelist profile", "profile_id", id, "error", err)
		return nil, err
	}

	logger.InfoContext(ctx, "Whitelist profile updated", "profile_id", id)

	// Invalidate cache for all domains in this profile
	s.invalidateProfileDomains(ctx, id)

	return s.whitelistRepo.GetByIDWithDomains(ctx, id)
}

func (s *WhitelistServiceImpl) DeleteProfile(ctx context.Context, id uuid.UUID) error {
	logger.InfoContext(ctx, "Deleting whitelist profile", "profile_id", id)

	// ตรวจสอบว่ามี profile อยู่
	_, err := s.whitelistRepo.GetByID(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "Profile not found for deletion", "profile_id", id, "error", err)
		return err
	}

	// Invalidate cache for all domains in this profile BEFORE deleting
	s.invalidateProfileDomains(ctx, id)

	if err := s.whitelistRepo.Delete(ctx, id); err != nil {
		logger.ErrorContext(ctx, "Failed to delete whitelist profile", "profile_id", id, "error", err)
		return err
	}

	logger.InfoContext(ctx, "Whitelist profile deleted", "profile_id", id)
	return nil
}

func (s *WhitelistServiceImpl) ListProfiles(ctx context.Context, page, limit int) ([]*models.WhitelistProfile, int64, error) {
	offset := (page - 1) * limit
	profiles, err := s.whitelistRepo.ListWithDomains(ctx, offset, limit)
	if err != nil {
		return nil, 0, err
	}

	count, err := s.whitelistRepo.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	return profiles, count, nil
}

func (s *WhitelistServiceImpl) ListActiveProfiles(ctx context.Context) ([]*models.WhitelistProfile, error) {
	return s.whitelistRepo.ListActive(ctx)
}

// ==================== Domain Management ====================

func (s *WhitelistServiceImpl) AddDomain(ctx context.Context, profileID uuid.UUID, domainStr string) (*models.ProfileDomain, error) {
	logger.InfoContext(ctx, "Adding domain to profile",
		"profile_id", profileID,
		"domain", domainStr,
	)

	// ตรวจสอบว่ามี profile อยู่
	_, err := s.whitelistRepo.GetByID(ctx, profileID)
	if err != nil {
		logger.WarnContext(ctx, "Profile not found", "profile_id", profileID, "error", err)
		return nil, err
	}

	domain := &models.ProfileDomain{
		ProfileID: profileID,
		Domain:    domainStr,
	}

	if err := s.whitelistRepo.AddDomain(ctx, domain); err != nil {
		logger.ErrorContext(ctx, "Failed to add domain",
			"profile_id", profileID,
			"domain", domainStr,
			"error", err,
		)
		return nil, err
	}

	logger.InfoContext(ctx, "Domain added to profile",
		"profile_id", profileID,
		"domain_id", domain.ID,
		"domain", domainStr,
	)

	// Invalidate cache for this domain
	s.InvalidateDomainCache(ctx, domainStr)

	return domain, nil
}

func (s *WhitelistServiceImpl) RemoveDomain(ctx context.Context, domainID uuid.UUID) error {
	logger.InfoContext(ctx, "Removing domain", "domain_id", domainID)

	// Get domain string before deleting (for cache invalidation)
	domain, _ := s.whitelistRepo.GetDomainByID(ctx, domainID)

	if err := s.whitelistRepo.RemoveDomain(ctx, domainID); err != nil {
		logger.ErrorContext(ctx, "Failed to remove domain", "domain_id", domainID, "error", err)
		return err
	}

	// Invalidate cache for this domain
	if domain != nil {
		s.InvalidateDomainCache(ctx, domain.Domain)
	}

	logger.InfoContext(ctx, "Domain removed", "domain_id", domainID)
	return nil
}

func (s *WhitelistServiceImpl) GetDomainsByProfile(ctx context.Context, profileID uuid.UUID) ([]*models.ProfileDomain, error) {
	return s.whitelistRepo.GetDomainsByProfileID(ctx, profileID)
}

// ==================== Domain Lookup ====================

func (s *WhitelistServiceImpl) FindProfileByDomain(ctx context.Context, domain string) (*models.WhitelistProfile, error) {
	return s.whitelistRepo.FindProfileByDomain(ctx, domain)
}

func (s *WhitelistServiceImpl) IsDomainAllowed(ctx context.Context, domain string) (bool, *models.WhitelistProfile, error) {
	cacheKey := whitelistCachePrefix + domain

	// 1. Check Redis cache first (if available)
	if s.redisClient != nil {
		cached, err := s.redisClient.Get(ctx, cacheKey)
		if err == nil {
			// Cache HIT
			if cached == nullCacheValue {
				// Negative cache - domain ไม่อยู่ใน whitelist
				return false, nil, nil
			}

			// Unmarshal cached profile
			var profile models.WhitelistProfile
			if err := json.Unmarshal([]byte(cached), &profile); err == nil {
				if !profile.IsActive {
					return false, &profile, errors.New("profile is inactive")
				}
				return true, &profile, nil
			}
		}
		// Cache MISS - continue to DB
	}

	// 2. Query DB
	profile, err := s.whitelistRepo.FindProfileByDomain(ctx, domain)
	if err != nil || profile == nil {
		// ไม่พบ domain ใน whitelist
		// ⚠️ IMPORTANT: Negative Cache (ป้องกัน Cache Penetration)
		// Bot/เว็บที่ไม่ได้ whitelist จะยิง request เข้ามาถล่ม DB
		// ถ้าไม่ cache "null" → ทุก request จะ query DB ตลอด
		if s.redisClient != nil {
			s.redisClient.Set(ctx, cacheKey, nullCacheValue, negativeCacheTTL)
		}
		return false, nil, nil
	}

	// 3. Cache result (5 นาที)
	if s.redisClient != nil {
		data, err := json.Marshal(profile)
		if err == nil {
			s.redisClient.Set(ctx, cacheKey, string(data), whitelistCacheTTL)
		}
	}

	if !profile.IsActive {
		return false, profile, errors.New("profile is inactive")
	}

	return true, profile, nil
}

// ==================== Watermark ====================

func (s *WhitelistServiceImpl) UpdateWatermark(ctx context.Context, profileID uuid.UUID, watermarkURL string) error {
	logger.InfoContext(ctx, "Updating watermark",
		"profile_id", profileID,
		"watermark_url", watermarkURL,
	)

	if err := s.whitelistRepo.UpdateWatermarkURL(ctx, profileID, watermarkURL); err != nil {
		logger.ErrorContext(ctx, "Failed to update watermark", "profile_id", profileID, "error", err)
		return err
	}

	logger.InfoContext(ctx, "Watermark updated", "profile_id", profileID)
	return nil
}

// ==================== Ad Statistics ====================

func (s *WhitelistServiceImpl) RecordAdImpression(ctx context.Context, req *dto.RecordAdImpressionRequest) error {
	// Determine device type from user agent
	deviceType := models.ParseDeviceType(req.UserAgent)

	impression := &models.AdImpression{
		ProfileID:     req.ProfileID,
		VideoCode:     req.VideoCode,
		Domain:        req.Domain,
		AdURL:         req.AdURL,
		AdDuration:    req.AdDuration,
		WatchDuration: req.WatchDuration,
		Completed:     req.Completed,
		Skipped:       req.Skipped,
		SkippedAt:     req.SkippedAt,
		ErrorOccurred: req.ErrorOccurred,
		UserAgent:     req.UserAgent,
		DeviceType:    deviceType,
		IPAddress:     req.IPAddress,
	}

	if err := s.adStatsRepo.Create(ctx, impression); err != nil {
		logger.ErrorContext(ctx, "Failed to record ad impression",
			"video_code", req.VideoCode,
			"error", err,
		)
		return err
	}

	logger.InfoContext(ctx, "Ad impression recorded",
		"video_code", req.VideoCode,
		"completed", req.Completed,
		"skipped", req.Skipped,
		"device_type", deviceType,
	)
	return nil
}

func (s *WhitelistServiceImpl) GetAdStats(ctx context.Context, start, end time.Time) (*models.AdImpressionStats, error) {
	return s.adStatsRepo.GetOverallStats(ctx, start, end)
}

func (s *WhitelistServiceImpl) GetAdStatsByProfile(ctx context.Context, profileID uuid.UUID, start, end time.Time) (*models.AdImpressionStats, error) {
	return s.adStatsRepo.GetStatsByProfile(ctx, profileID, start, end)
}

func (s *WhitelistServiceImpl) GetDeviceStats(ctx context.Context, start, end time.Time) (*models.DeviceStats, error) {
	return s.adStatsRepo.GetDeviceStats(ctx, start, end)
}

func (s *WhitelistServiceImpl) GetProfileRanking(ctx context.Context, start, end time.Time, limit int) ([]*models.ProfileAdStats, error) {
	return s.adStatsRepo.GetProfileRanking(ctx, start, end, limit)
}

func (s *WhitelistServiceImpl) GetSkipTimeDistribution(ctx context.Context, start, end time.Time) (map[int]int64, error) {
	return s.adStatsRepo.GetSkipTimeDistribution(ctx, start, end)
}

func (s *WhitelistServiceImpl) CleanupOldStats(ctx context.Context, days int) (int64, error) {
	before := time.Now().AddDate(0, 0, -days)
	logger.InfoContext(ctx, "Cleaning up old ad stats", "before", before)

	deleted, err := s.adStatsRepo.DeleteOlderThan(ctx, before)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to cleanup old ad stats", "error", err)
		return 0, err
	}

	logger.InfoContext(ctx, "Old ad stats cleaned up", "deleted_count", deleted)
	return deleted, nil
}

// ==================== Preroll Ads ====================

func (s *WhitelistServiceImpl) AddPrerollAd(ctx context.Context, profileID uuid.UUID, req *dto.AddPrerollAdRequest) (*models.PrerollAd, error) {
	logger.InfoContext(ctx, "Adding preroll ad to profile",
		"profile_id", profileID,
		"type", req.Type,
		"url", req.URL,
		"skip_after", req.SkipAfter,
	)

	// ตรวจสอบว่ามี profile อยู่
	_, err := s.whitelistRepo.GetByID(ctx, profileID)
	if err != nil {
		logger.WarnContext(ctx, "Profile not found", "profile_id", profileID, "error", err)
		return nil, err
	}

	preroll := &models.PrerollAd{
		ProfileID: profileID,
		Type:      models.AdType(req.Type),
		URL:       req.URL,
		Duration:  req.Duration,
		SkipAfter: req.SkipAfter,
		ClickURL:  req.ClickURL,
		ClickText: req.ClickText,
		Title:     req.Title,
	}

	if err := s.whitelistRepo.AddPrerollAd(ctx, preroll); err != nil {
		logger.ErrorContext(ctx, "Failed to add preroll ad",
			"profile_id", profileID,
			"error", err,
		)
		return nil, err
	}

	logger.InfoContext(ctx, "Preroll ad added to profile",
		"profile_id", profileID,
		"preroll_id", preroll.ID,
		"type", preroll.Type,
		"sort_order", preroll.SortOrder,
	)
	return preroll, nil
}

func (s *WhitelistServiceImpl) UpdatePrerollAd(ctx context.Context, prerollID uuid.UUID, req *dto.UpdatePrerollAdRequest) (*models.PrerollAd, error) {
	logger.InfoContext(ctx, "Updating preroll ad", "preroll_id", prerollID, "type", req.Type)

	// ต้อง fetch preroll เดิมก่อน เพื่อเอา ProfileID และ field อื่นๆ
	preroll, err := s.whitelistRepo.GetPrerollAdByID(ctx, prerollID)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get preroll ad", "preroll_id", prerollID, "error", err)
		return nil, err
	}
	if preroll == nil {
		logger.WarnContext(ctx, "Preroll ad not found", "preroll_id", prerollID)
		return nil, errors.New("preroll ad not found")
	}

	// Update fields
	preroll.Type = models.AdType(req.Type)
	preroll.URL = req.URL
	preroll.Duration = req.Duration
	preroll.SkipAfter = req.SkipAfter
	preroll.ClickURL = req.ClickURL
	preroll.ClickText = req.ClickText
	preroll.Title = req.Title

	if err := s.whitelistRepo.UpdatePrerollAd(ctx, preroll); err != nil {
		logger.ErrorContext(ctx, "Failed to update preroll ad", "preroll_id", prerollID, "error", err)
		return nil, err
	}

	logger.InfoContext(ctx, "Preroll ad updated", "preroll_id", prerollID, "type", preroll.Type)
	return preroll, nil
}

func (s *WhitelistServiceImpl) DeletePrerollAd(ctx context.Context, prerollID uuid.UUID) error {
	logger.InfoContext(ctx, "Deleting preroll ad", "preroll_id", prerollID)

	if err := s.whitelistRepo.DeletePrerollAd(ctx, prerollID); err != nil {
		logger.ErrorContext(ctx, "Failed to delete preroll ad", "preroll_id", prerollID, "error", err)
		return err
	}

	logger.InfoContext(ctx, "Preroll ad deleted", "preroll_id", prerollID)
	return nil
}

func (s *WhitelistServiceImpl) GetPrerollAdsByProfile(ctx context.Context, profileID uuid.UUID) ([]*models.PrerollAd, error) {
	return s.whitelistRepo.GetPrerollAdsByProfileID(ctx, profileID)
}

func (s *WhitelistServiceImpl) ReorderPrerollAds(ctx context.Context, profileID uuid.UUID, prerollIDs []uuid.UUID) error {
	logger.InfoContext(ctx, "Reordering preroll ads",
		"profile_id", profileID,
		"preroll_ids", prerollIDs,
	)

	if err := s.whitelistRepo.ReorderPrerollAds(ctx, profileID, prerollIDs); err != nil {
		logger.ErrorContext(ctx, "Failed to reorder preroll ads",
			"profile_id", profileID,
			"error", err,
		)
		return err
	}

	logger.InfoContext(ctx, "Preroll ads reordered", "profile_id", profileID)
	return nil
}

// ==================== Cache Management ====================

// InvalidateDomainCache ลบ cache ของ domain เดียว
func (s *WhitelistServiceImpl) InvalidateDomainCache(ctx context.Context, domain string) error {
	if s.redisClient == nil {
		return nil // No Redis, no cache to invalidate
	}

	cacheKey := whitelistCachePrefix + domain
	if err := s.redisClient.Del(ctx, cacheKey); err != nil {
		logger.WarnContext(ctx, "Failed to invalidate domain cache",
			"domain", domain,
			"error", err,
		)
		return err
	}

	logger.InfoContext(ctx, "Domain cache invalidated", "domain", domain)
	return nil
}

// InvalidateAllCache ลบ cache ทั้งหมด (สำหรับ Admin)
func (s *WhitelistServiceImpl) InvalidateAllCache(ctx context.Context) (int64, error) {
	if s.redisClient == nil {
		return 0, nil // No Redis, no cache to invalidate
	}

	pattern := whitelistCachePrefix + "*"
	deleted, err := s.redisClient.ScanAndDelete(ctx, pattern)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to invalidate all cache",
			"pattern", pattern,
			"error", err,
		)
		return 0, err
	}

	logger.InfoContext(ctx, "All whitelist cache invalidated", "deleted", deleted)
	return deleted, nil
}

// invalidateProfileDomains ลบ cache ของ domains ทั้งหมดใน profile
func (s *WhitelistServiceImpl) invalidateProfileDomains(ctx context.Context, profileID uuid.UUID) {
	if s.redisClient == nil {
		return
	}

	domains, err := s.whitelistRepo.GetDomainsByProfileID(ctx, profileID)
	if err != nil {
		return
	}

	for _, d := range domains {
		s.InvalidateDomainCache(ctx, d.Domain)
	}
}
