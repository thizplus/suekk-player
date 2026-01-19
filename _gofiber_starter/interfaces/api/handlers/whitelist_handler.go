package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"gofiber-template/domain/dto"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/utils"
)

type WhitelistHandler struct {
	whitelistService    services.WhitelistService
	streamCookieService services.StreamCookieService
	streamURL           string
}

func NewWhitelistHandler(
	whitelistService services.WhitelistService,
	streamCookieService services.StreamCookieService,
	streamURL string,
) *WhitelistHandler {
	return &WhitelistHandler{
		whitelistService:    whitelistService,
		streamCookieService: streamCookieService,
		streamURL:           streamURL,
	}
}

// GetWhitelistService returns the whitelist service (for middleware)
func (h *WhitelistHandler) GetWhitelistService() services.WhitelistService {
	return h.whitelistService
}

// ==================== Profile Management ====================

// CreateProfile สร้าง whitelist profile ใหม่
// POST /api/v1/whitelist/profiles
func (h *WhitelistHandler) CreateProfile(c *fiber.Ctx) error {
	ctx := c.UserContext()

	var req dto.CreateWhitelistProfileRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	logger.InfoContext(ctx, "Creating whitelist profile", "name", req.Name)

	profile, err := h.whitelistService.CreateProfile(ctx, &req)
	if err != nil {
		logger.WarnContext(ctx, "Failed to create whitelist profile", "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	return utils.CreatedResponse(c, dto.ProfileToResponse(profile))
}

// GetProfile ดึงข้อมูล profile
// GET /api/v1/whitelist/profiles/:id
func (h *WhitelistHandler) GetProfile(c *fiber.Ctx) error {
	ctx := c.UserContext()

	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid profile ID")
	}

	profile, err := h.whitelistService.GetProfileWithDomains(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "Profile not found", "profile_id", id, "error", err)
		return utils.NotFoundResponse(c, "Profile not found")
	}

	return utils.SuccessResponse(c, dto.ProfileToResponse(profile))
}

// ListProfiles ดึง profiles ทั้งหมดพร้อม pagination
// GET /api/v1/whitelist/profiles
func (h *WhitelistHandler) ListProfiles(c *fiber.Ctx) error {
	ctx := c.UserContext()

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	profiles, total, err := h.whitelistService.ListProfiles(ctx, page, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to list profiles", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	responses := dto.ProfilesToResponses(profiles)
	return utils.PaginatedSuccessResponse(c, responses, total, page, limit)
}

// UpdateProfile อัพเดท profile
// PUT /api/v1/whitelist/profiles/:id
func (h *WhitelistHandler) UpdateProfile(c *fiber.Ctx) error {
	ctx := c.UserContext()

	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid profile ID")
	}

	var req dto.UpdateWhitelistProfileRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	logger.InfoContext(ctx, "Updating whitelist profile", "profile_id", id)

	profile, err := h.whitelistService.UpdateProfile(ctx, id, &req)
	if err != nil {
		logger.WarnContext(ctx, "Failed to update profile", "profile_id", id, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	return utils.SuccessResponse(c, dto.ProfileToResponse(profile))
}

// DeleteProfile ลบ profile
// DELETE /api/v1/whitelist/profiles/:id
func (h *WhitelistHandler) DeleteProfile(c *fiber.Ctx) error {
	ctx := c.UserContext()

	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid profile ID")
	}

	logger.InfoContext(ctx, "Deleting whitelist profile", "profile_id", id)

	if err := h.whitelistService.DeleteProfile(ctx, id); err != nil {
		logger.WarnContext(ctx, "Failed to delete profile", "profile_id", id, "error", err)
		return utils.NotFoundResponse(c, "Profile not found")
	}

	return utils.SuccessResponse(c, fiber.Map{"message": "Profile deleted successfully"})
}

// ==================== Domain Management ====================

// AddDomain เพิ่ม domain ให้ profile
// POST /api/v1/whitelist/profiles/:id/domains
func (h *WhitelistHandler) AddDomain(c *fiber.Ctx) error {
	ctx := c.UserContext()

	profileIDStr := c.Params("id")
	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid profile ID")
	}

	var req dto.AddDomainRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	logger.InfoContext(ctx, "Adding domain to profile",
		"profile_id", profileID,
		"domain", req.Domain,
	)

	domain, err := h.whitelistService.AddDomain(ctx, profileID, req.Domain)
	if err != nil {
		logger.WarnContext(ctx, "Failed to add domain",
			"profile_id", profileID,
			"domain", req.Domain,
			"error", err,
		)
		return utils.BadRequestResponse(c, err.Error())
	}

	return utils.CreatedResponse(c, dto.ProfileDomainResponse{
		ID:        domain.ID,
		ProfileID: domain.ProfileID,
		Domain:    domain.Domain,
		CreatedAt: domain.CreatedAt,
	})
}

// RemoveDomain ลบ domain
// DELETE /api/v1/whitelist/domains/:id
func (h *WhitelistHandler) RemoveDomain(c *fiber.Ctx) error {
	ctx := c.UserContext()

	domainIDStr := c.Params("id")
	domainID, err := uuid.Parse(domainIDStr)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid domain ID")
	}

	logger.InfoContext(ctx, "Removing domain", "domain_id", domainID)

	if err := h.whitelistService.RemoveDomain(ctx, domainID); err != nil {
		logger.WarnContext(ctx, "Failed to remove domain", "domain_id", domainID, "error", err)
		return utils.NotFoundResponse(c, "Domain not found")
	}

	return utils.SuccessResponse(c, fiber.Map{"message": "Domain removed successfully"})
}

// ==================== Preroll Ads Management ====================

// AddPrerollAd เพิ่ม preroll ad ให้ profile
// POST /api/v1/whitelist/profiles/:id/prerolls
func (h *WhitelistHandler) AddPrerollAd(c *fiber.Ctx) error {
	ctx := c.UserContext()

	profileIDStr := c.Params("id")
	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid profile ID")
	}

	var req dto.AddPrerollAdRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	logger.InfoContext(ctx, "Adding preroll ad to profile",
		"profile_id", profileID,
		"type", req.Type,
		"url", req.URL,
	)

	preroll, err := h.whitelistService.AddPrerollAd(ctx, profileID, &req)
	if err != nil {
		logger.WarnContext(ctx, "Failed to add preroll ad",
			"profile_id", profileID,
			"error", err,
		)
		return utils.BadRequestResponse(c, err.Error())
	}

	return utils.CreatedResponse(c, dto.PrerollAdToResponse(preroll))
}

// GetPrerollAdsByProfile ดึง preroll ads ของ profile
// GET /api/v1/whitelist/profiles/:id/prerolls
func (h *WhitelistHandler) GetPrerollAdsByProfile(c *fiber.Ctx) error {
	ctx := c.UserContext()

	profileIDStr := c.Params("id")
	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid profile ID")
	}

	prerolls, err := h.whitelistService.GetPrerollAdsByProfile(ctx, profileID)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get preroll ads", "profile_id", profileID, "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, dto.PrerollAdsToResponses(prerolls))
}

// UpdatePrerollAd อัพเดท preroll ad
// PUT /api/v1/whitelist/prerolls/:id
func (h *WhitelistHandler) UpdatePrerollAd(c *fiber.Ctx) error {
	ctx := c.UserContext()

	prerollIDStr := c.Params("id")
	prerollID, err := uuid.Parse(prerollIDStr)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid preroll ID")
	}

	var req dto.UpdatePrerollAdRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	logger.InfoContext(ctx, "Updating preroll ad", "preroll_id", prerollID, "type", req.Type)

	preroll, err := h.whitelistService.UpdatePrerollAd(ctx, prerollID, &req)
	if err != nil {
		logger.WarnContext(ctx, "Failed to update preroll ad", "preroll_id", prerollID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	return utils.SuccessResponse(c, dto.PrerollAdToResponse(preroll))
}

// DeletePrerollAd ลบ preroll ad
// DELETE /api/v1/whitelist/prerolls/:id
func (h *WhitelistHandler) DeletePrerollAd(c *fiber.Ctx) error {
	ctx := c.UserContext()

	prerollIDStr := c.Params("id")
	prerollID, err := uuid.Parse(prerollIDStr)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid preroll ID")
	}

	logger.InfoContext(ctx, "Deleting preroll ad", "preroll_id", prerollID)

	if err := h.whitelistService.DeletePrerollAd(ctx, prerollID); err != nil {
		logger.WarnContext(ctx, "Failed to delete preroll ad", "preroll_id", prerollID, "error", err)
		return utils.NotFoundResponse(c, "Preroll ad not found")
	}

	return utils.SuccessResponse(c, fiber.Map{"message": "Preroll ad deleted successfully"})
}

// ReorderPrerollAds จัดลำดับ preroll ads ใหม่
// PUT /api/v1/whitelist/profiles/:id/prerolls/reorder
func (h *WhitelistHandler) ReorderPrerollAds(c *fiber.Ctx) error {
	ctx := c.UserContext()

	profileIDStr := c.Params("id")
	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid profile ID")
	}

	var req dto.ReorderPrerollAdsRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	logger.InfoContext(ctx, "Reordering preroll ads",
		"profile_id", profileID,
		"preroll_ids_count", len(req.PrerollIDs),
	)

	if err := h.whitelistService.ReorderPrerollAds(ctx, profileID, req.PrerollIDs); err != nil {
		logger.WarnContext(ctx, "Failed to reorder preroll ads", "profile_id", profileID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	// Return updated prerolls
	prerolls, err := h.whitelistService.GetPrerollAdsByProfile(ctx, profileID)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get preroll ads after reorder", "profile_id", profileID, "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, dto.PrerollAdsToResponses(prerolls))
}

// ==================== Ad Statistics ====================

// RecordAdImpression บันทึก ad impression
// POST /api/v1/ads/impression
func (h *WhitelistHandler) RecordAdImpression(c *fiber.Ctx) error {
	ctx := c.UserContext()

	var req dto.RecordAdImpressionRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	// Auto-fill client info
	req.UserAgent = c.Get("User-Agent")
	req.IPAddress = c.IP()

	if err := h.whitelistService.RecordAdImpression(ctx, &req); err != nil {
		logger.ErrorContext(ctx, "Failed to record ad impression", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.CreatedResponse(c, fiber.Map{"message": "Impression recorded"})
}

// GetAdStats ดึงสถิติ ads
// GET /api/v1/ads/stats
func (h *WhitelistHandler) GetAdStats(c *fiber.Ctx) error {
	ctx := c.UserContext()

	// Parse date range (default: last 7 days)
	start, end := h.parseDateRange(c)

	stats, err := h.whitelistService.GetAdStats(ctx, start, end)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get ad stats", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, dto.AdStatsToResponse(stats))
}

// GetAdStatsByProfile ดึงสถิติ ads ของ profile
// GET /api/v1/ads/stats/profile/:id
func (h *WhitelistHandler) GetAdStatsByProfile(c *fiber.Ctx) error {
	ctx := c.UserContext()

	profileIDStr := c.Params("id")
	profileID, err := uuid.Parse(profileIDStr)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid profile ID")
	}

	start, end := h.parseDateRange(c)

	stats, err := h.whitelistService.GetAdStatsByProfile(ctx, profileID, start, end)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get ad stats by profile", "profile_id", profileID, "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, dto.AdStatsToResponse(stats))
}

// GetDeviceStats ดึงสถิติแยกตามอุปกรณ์
// GET /api/v1/ads/stats/devices
func (h *WhitelistHandler) GetDeviceStats(c *fiber.Ctx) error {
	ctx := c.UserContext()

	start, end := h.parseDateRange(c)

	stats, err := h.whitelistService.GetDeviceStats(ctx, start, end)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get device stats", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, dto.DeviceStatsToResponse(stats))
}

// GetProfileRanking ดึง ranking ของ profiles
// GET /api/v1/ads/stats/ranking
func (h *WhitelistHandler) GetProfileRanking(c *fiber.Ctx) error {
	ctx := c.UserContext()

	start, end := h.parseDateRange(c)
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	if limit < 1 || limit > 50 {
		limit = 10
	}

	rankings, err := h.whitelistService.GetProfileRanking(ctx, start, end, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get profile ranking", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, dto.ProfileRankingsToResponses(rankings))
}

// GetSkipTimeDistribution ดึง distribution ของ skip time
// GET /api/v1/ads/stats/skip-distribution
func (h *WhitelistHandler) GetSkipTimeDistribution(c *fiber.Ctx) error {
	ctx := c.UserContext()

	start, end := h.parseDateRange(c)

	distribution, err := h.whitelistService.GetSkipTimeDistribution(ctx, start, end)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get skip time distribution", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, distribution)
}

// ==================== Cache Management ====================

// ClearAllCache ลบ cache ทั้งหมด
// POST /api/v1/whitelist/cache/clear
func (h *WhitelistHandler) ClearAllCache(c *fiber.Ctx) error {
	ctx := c.UserContext()

	logger.InfoContext(ctx, "Clearing all whitelist cache")

	deleted, err := h.whitelistService.InvalidateAllCache(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to clear whitelist cache", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	logger.InfoContext(ctx, "Whitelist cache cleared", "deleted_keys", deleted)

	return utils.SuccessResponse(c, fiber.Map{
		"message":     "Cache cleared successfully",
		"deletedKeys": deleted,
	})
}

// ClearDomainCache ลบ cache ของ domain เดียว
// DELETE /api/v1/whitelist/cache/domain/:domain
func (h *WhitelistHandler) ClearDomainCache(c *fiber.Ctx) error {
	ctx := c.UserContext()

	domain := c.Params("domain")
	if domain == "" {
		return utils.BadRequestResponse(c, "Domain is required")
	}

	logger.InfoContext(ctx, "Clearing cache for domain", "domain", domain)

	if err := h.whitelistService.InvalidateDomainCache(ctx, domain); err != nil {
		logger.ErrorContext(ctx, "Failed to clear domain cache", "domain", domain, "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	logger.InfoContext(ctx, "Domain cache cleared", "domain", domain)

	return utils.SuccessResponse(c, fiber.Map{
		"message": "Domain cache cleared",
		"domain":  domain,
	})
}

// ==================== Embed Config (Public) ====================

// GetEmbedConfig ดึง config สำหรับ embed player
// GET /api/v1/embed/config (ต้องมี Origin/Referer header)
func (h *WhitelistHandler) GetEmbedConfig(c *fiber.Ctx) error {
	ctx := c.UserContext()

	// รับ domain จาก Origin หรือ Referer header
	domain := c.Get("Origin")
	if domain == "" {
		domain = c.Get("Referer")
	}

	// Clean domain (remove protocol and path)
	domain = cleanDomain(domain)

	if domain == "" {
		return utils.BadRequestResponse(c, "Origin or Referer header required")
	}

	// ค้นหา profile ที่ match domain
	allowed, profile, err := h.whitelistService.IsDomainAllowed(ctx, domain)
	if err != nil || !allowed {
		logger.WarnContext(ctx, "Domain not allowed", "domain", domain)
		return utils.ForbiddenResponse(c, "Domain not in whitelist")
	}

	// Generate stream token (Hybrid Shield - ป้องกัน IDM)
	config := dto.ProfileToEmbedConfig(profile)
	if h.streamCookieService != nil {
		config.StreamToken = h.streamCookieService.GenerateToken(domain)
	}
	if h.streamURL != "" {
		config.StreamURL = h.streamURL
	}

	return utils.SuccessResponse(c, config)
}

// ==================== Helper Functions ====================

// parseDateRange parse start/end date จาก query params
// Default: last 7 days
func (h *WhitelistHandler) parseDateRange(c *fiber.Ctx) (time.Time, time.Time) {
	now := time.Now()
	defaultEnd := now
	defaultStart := now.AddDate(0, 0, -7) // 7 days ago

	startStr := c.Query("start")
	endStr := c.Query("end")

	start := defaultStart
	end := defaultEnd

	if startStr != "" {
		if parsed, err := time.Parse("2006-01-02", startStr); err == nil {
			start = parsed
		}
	}

	if endStr != "" {
		if parsed, err := time.Parse("2006-01-02", endStr); err == nil {
			// Set to end of day
			end = parsed.Add(24*time.Hour - time.Second)
		}
	}

	return start, end
}

// cleanDomain extract domain from URL
func cleanDomain(urlStr string) string {
	if urlStr == "" {
		return ""
	}

	// Remove protocol
	domain := urlStr
	if idx := len("https://"); len(domain) > idx && domain[:idx] == "https://" {
		domain = domain[idx:]
	} else if idx := len("http://"); len(domain) > idx && domain[:idx] == "http://" {
		domain = domain[idx:]
	}

	// Remove path
	if idx := indexOf(domain, '/'); idx != -1 {
		domain = domain[:idx]
	}

	// Remove port
	if idx := indexOf(domain, ':'); idx != -1 {
		domain = domain[:idx]
	}

	return domain
}

func indexOf(s string, char rune) int {
	for i, c := range s {
		if c == char {
			return i
		}
	}
	return -1
}
