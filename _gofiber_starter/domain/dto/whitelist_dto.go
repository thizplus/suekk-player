package dto

import (
	"time"

	"github.com/google/uuid"
	"gofiber-template/domain/models"
)

// ==================== Request DTOs ====================

// CreateWhitelistProfileRequest สำหรับสร้าง profile ใหม่
type CreateWhitelistProfileRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=100"`
	Description string `json:"description" validate:"max=500"`
	IsActive    bool   `json:"isActive"`

	// Thumbnail Settings (แสดงก่อนกด play)
	ThumbnailURL string `json:"thumbnailUrl" validate:"omitempty,url"`

	// Watermark Settings
	WatermarkEnabled  bool    `json:"watermarkEnabled"`
	WatermarkURL      string  `json:"watermarkUrl" validate:"omitempty,url"`
	WatermarkPosition string  `json:"watermarkPosition" validate:"omitempty,oneof=top-left top-right bottom-left bottom-right"`
	WatermarkOpacity  float64 `json:"watermarkOpacity" validate:"omitempty,min=0,max=1"`
	WatermarkSize     int     `json:"watermarkSize" validate:"omitempty,min=10,max=500"`
	WatermarkOffsetY  int     `json:"watermarkOffsetY" validate:"omitempty,min=0,max=200"`

	// Pre-roll Ads Settings
	PrerollEnabled   bool   `json:"prerollEnabled"`
	PrerollURL       string `json:"prerollUrl" validate:"omitempty,url"`
	PrerollSkipAfter int    `json:"prerollSkipAfter" validate:"omitempty,min=0,max=120"` // 0 = ไม่ให้ skip

	// Initial domains (optional)
	Domains []string `json:"domains" validate:"omitempty,dive,min=1,max=255"`
}

// UpdateWhitelistProfileRequest สำหรับอัพเดท profile
type UpdateWhitelistProfileRequest struct {
	Name        *string `json:"name" validate:"omitempty,min=1,max=100"`
	Description *string `json:"description" validate:"omitempty,max=500"`
	IsActive    *bool   `json:"isActive"`

	// Thumbnail Settings (แสดงก่อนกด play)
	ThumbnailURL *string `json:"thumbnailUrl" validate:"omitempty"`

	// Watermark Settings
	WatermarkEnabled  *bool    `json:"watermarkEnabled"`
	WatermarkURL      *string  `json:"watermarkUrl" validate:"omitempty,url"`
	WatermarkPosition *string  `json:"watermarkPosition" validate:"omitempty,oneof=top-left top-right bottom-left bottom-right"`
	WatermarkOpacity  *float64 `json:"watermarkOpacity" validate:"omitempty,min=0,max=1"`
	WatermarkSize     *int     `json:"watermarkSize" validate:"omitempty,min=10,max=500"`
	WatermarkOffsetY  *int     `json:"watermarkOffsetY" validate:"omitempty,min=0,max=200"`

	// Pre-roll Ads Settings
	PrerollEnabled   *bool   `json:"prerollEnabled"`
	PrerollURL       *string `json:"prerollUrl" validate:"omitempty,url"`
	PrerollSkipAfter *int    `json:"prerollSkipAfter" validate:"omitempty,min=0,max=120"`
}

// AddDomainRequest สำหรับเพิ่ม domain
type AddDomainRequest struct {
	Domain string `json:"domain" validate:"required,min=1,max=255"`
}

// AddPrerollAdRequest สำหรับเพิ่ม preroll ad
type AddPrerollAdRequest struct {
	// Ad Type & Content
	Type     string `json:"type" validate:"required,oneof=video image"` // video หรือ image
	URL      string `json:"url" validate:"required,url"`                // URL ของ video หรือ image
	Duration int    `json:"duration" validate:"min=0,max=120"`          // ระยะเวลา (วินาที) - ใช้กับ image

	// Skip Settings
	SkipAfter int `json:"skipAfter" validate:"min=0,max=120"` // 0 = บังคับดูจบ

	// Click/Link Settings (optional)
	ClickURL  string `json:"clickUrl" validate:"omitempty,url"` // URL เมื่อคลิกโฆษณา
	ClickText string `json:"clickText" validate:"max=100"`      // ข้อความปุ่ม เช่น "ดูรายละเอียด"

	// Display Settings (optional)
	Title string `json:"title" validate:"max=255"` // ชื่อโฆษณา/ผู้สนับสนุน
}

// UpdatePrerollAdRequest สำหรับอัพเดท preroll ad
type UpdatePrerollAdRequest struct {
	// Ad Type & Content
	Type     string `json:"type" validate:"required,oneof=video image"`
	URL      string `json:"url" validate:"required,url"`
	Duration int    `json:"duration" validate:"min=0,max=120"`

	// Skip Settings
	SkipAfter int `json:"skipAfter" validate:"min=0,max=120"`

	// Click/Link Settings
	ClickURL  string `json:"clickUrl" validate:"omitempty,url"`
	ClickText string `json:"clickText" validate:"max=100"`

	// Display Settings
	Title string `json:"title" validate:"max=255"`
}

// ReorderPrerollAdsRequest สำหรับจัดลำดับ preroll ads
type ReorderPrerollAdsRequest struct {
	PrerollIDs []uuid.UUID `json:"prerollIds" validate:"required,min=1"`
}

// RecordAdImpressionRequest สำหรับบันทึก ad impression
type RecordAdImpressionRequest struct {
	ProfileID     *uuid.UUID `json:"profileId"`
	VideoCode     string     `json:"videoCode" validate:"required"`
	Domain        string     `json:"domain"`
	AdURL         string     `json:"adUrl"`
	AdDuration    int        `json:"adDuration"`
	WatchDuration int        `json:"watchDuration"`
	Completed     bool       `json:"completed"`
	Skipped       bool       `json:"skipped"`
	SkippedAt     int        `json:"skippedAt"`
	ErrorOccurred bool       `json:"errorOccurred"`
	UserAgent     string     `json:"userAgent"`
	IPAddress     string     `json:"ipAddress"`
}

// ==================== Response DTOs ====================

// WhitelistProfileResponse response DTO สำหรับ profile
type WhitelistProfileResponse struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsActive    bool      `json:"isActive"`

	// Thumbnail Settings
	ThumbnailURL string `json:"thumbnailUrl"`

	// Watermark Settings
	WatermarkEnabled  bool    `json:"watermarkEnabled"`
	WatermarkURL      string  `json:"watermarkUrl"`
	WatermarkPosition string  `json:"watermarkPosition"`
	WatermarkOpacity  float64 `json:"watermarkOpacity"`
	WatermarkSize     int     `json:"watermarkSize"`
	WatermarkOffsetY  int     `json:"watermarkOffsetY"`

	// Pre-roll Ads Settings
	PrerollEnabled   bool   `json:"prerollEnabled"`
	PrerollURL       string `json:"prerollUrl"`
	PrerollSkipAfter int    `json:"prerollSkipAfter"`

	// Relations
	Domains    []ProfileDomainResponse `json:"domains,omitempty"`
	PrerollAds []PrerollAdResponse     `json:"prerollAds,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ProfileDomainResponse response DTO สำหรับ domain
type ProfileDomainResponse struct {
	ID        uuid.UUID `json:"id"`
	ProfileID uuid.UUID `json:"profileId"`
	Domain    string    `json:"domain"`
	CreatedAt time.Time `json:"createdAt"`
}

// PrerollAdResponse response DTO สำหรับ preroll ad
type PrerollAdResponse struct {
	ID        uuid.UUID `json:"id"`
	ProfileID uuid.UUID `json:"profileId"`

	// Ad Type & Content
	Type     string `json:"type"` // video หรือ image
	URL      string `json:"url"`
	Duration int    `json:"duration"` // ระยะเวลา (วินาที)

	// Skip Settings
	SkipAfter int `json:"skipAfter"`

	// Click/Link Settings
	ClickURL  string `json:"clickUrl,omitempty"`
	ClickText string `json:"clickText,omitempty"`

	// Display Settings
	Title string `json:"title,omitempty"`

	// Order & Timestamps
	SortOrder int       `json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// AdImpressionStatsResponse response DTO สำหรับสถิติ ad
type AdImpressionStatsResponse struct {
	TotalImpressions int64   `json:"totalImpressions"`
	Completed        int64   `json:"completed"`
	Skipped          int64   `json:"skipped"`
	Errors           int64   `json:"errors"`
	CompletionRate   float64 `json:"completionRate"`
	SkipRate         float64 `json:"skipRate"`
	ErrorRate        float64 `json:"errorRate"`
	AvgWatchDuration float64 `json:"avgWatchDuration"`
	AvgSkipTime      float64 `json:"avgSkipTime"`
}

// DeviceStatsResponse response DTO สำหรับสถิติอุปกรณ์
type DeviceStatsResponse struct {
	Mobile  int64 `json:"mobile"`
	Desktop int64 `json:"desktop"`
	Tablet  int64 `json:"tablet"`
}

// ProfileRankingResponse response DTO สำหรับ ranking
type ProfileRankingResponse struct {
	ProfileID      uuid.UUID `json:"profileId"`
	ProfileName    string    `json:"profileName"`
	TotalViews     int64     `json:"totalViews"`
	CompletionRate float64   `json:"completionRate"`
}

// EmbedConfigResponse config สำหรับ embed player
type EmbedConfigResponse struct {
	ProfileID uuid.UUID `json:"profileId"`
	IsAllowed bool      `json:"isAllowed"`

	// Stream Token (Hybrid Shield - ป้องกัน IDM)
	StreamToken string `json:"streamToken,omitempty"`
	StreamURL   string `json:"streamUrl,omitempty"`

	// Thumbnail (แสดงก่อนกด play)
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`

	// Watermark
	Watermark *WatermarkConfig `json:"watermark,omitempty"`

	// Pre-roll (legacy - single preroll)
	Preroll *PrerollConfig `json:"preroll,omitempty"`

	// Pre-roll Ads (multiple prerolls)
	PrerollAds []PrerollConfig `json:"prerollAds,omitempty"`
}

// WatermarkConfig config สำหรับ watermark
type WatermarkConfig struct {
	Enabled  bool    `json:"enabled"`
	URL      string  `json:"url"`
	Position string  `json:"position"`
	Opacity  float64 `json:"opacity"`
	Size     int     `json:"size"`
	OffsetY  int     `json:"offsetY"`
}

// PrerollConfig config สำหรับ pre-roll ad (ส่งไป embed player)
type PrerollConfig struct {
	Enabled bool `json:"enabled"`

	// Ad Type & Content
	Type     string `json:"type"` // video หรือ image
	URL      string `json:"url"`
	Duration int    `json:"duration"` // ระยะเวลา (วินาที) - ใช้กับ image

	// Skip Settings
	SkipAfter int `json:"skipAfter"` // 0 = ไม่ให้ skip

	// Click/Link Settings
	ClickURL  string `json:"clickUrl,omitempty"`
	ClickText string `json:"clickText,omitempty"`

	// Display Settings
	Title string `json:"title,omitempty"`
}

// ==================== Mappers ====================

// ProfileToResponse แปลง model เป็น response DTO
func ProfileToResponse(p *models.WhitelistProfile) *WhitelistProfileResponse {
	if p == nil {
		return nil
	}

	resp := &WhitelistProfileResponse{
		ID:                p.ID,
		Name:              p.Name,
		Description:       p.Description,
		IsActive:          p.IsActive,
		ThumbnailURL:      p.ThumbnailURL,
		WatermarkEnabled:  p.WatermarkEnabled,
		WatermarkURL:      p.WatermarkURL,
		WatermarkPosition: p.WatermarkPosition,
		WatermarkOpacity:  p.WatermarkOpacity,
		WatermarkSize:     p.WatermarkSize,
		WatermarkOffsetY:  p.WatermarkOffsetY,
		PrerollEnabled:    p.PrerollEnabled,
		PrerollURL:        p.PrerollURL,
		PrerollSkipAfter:  p.PrerollSkipAfter,
		CreatedAt:         p.CreatedAt,
		UpdatedAt:         p.UpdatedAt,
	}

	// Map domains if loaded
	if len(p.Domains) > 0 {
		resp.Domains = make([]ProfileDomainResponse, len(p.Domains))
		for i, d := range p.Domains {
			resp.Domains[i] = ProfileDomainResponse{
				ID:        d.ID,
				ProfileID: d.ProfileID,
				Domain:    d.Domain,
				CreatedAt: d.CreatedAt,
			}
		}
	}

	// Map preroll ads if loaded
	if len(p.PrerollAds) > 0 {
		resp.PrerollAds = make([]PrerollAdResponse, len(p.PrerollAds))
		for i, ad := range p.PrerollAds {
			adType := string(ad.Type)
			if adType == "" {
				adType = "video" // default
			}
			resp.PrerollAds[i] = PrerollAdResponse{
				ID:        ad.ID,
				ProfileID: ad.ProfileID,
				Type:      adType,
				URL:       ad.URL,
				Duration:  ad.Duration,
				SkipAfter: ad.SkipAfter,
				ClickURL:  ad.ClickURL,
				ClickText: ad.ClickText,
				Title:     ad.Title,
				SortOrder: ad.SortOrder,
				CreatedAt: ad.CreatedAt,
				UpdatedAt: ad.UpdatedAt,
			}
		}
	}

	return resp
}

// ProfileToEmbedConfig แปลง profile เป็น embed config
func ProfileToEmbedConfig(p *models.WhitelistProfile) *EmbedConfigResponse {
	if p == nil {
		return &EmbedConfigResponse{IsAllowed: false}
	}

	config := &EmbedConfigResponse{
		ProfileID:    p.ID,
		IsAllowed:    true,
		ThumbnailURL: p.ThumbnailURL,
	}

	// Watermark config
	if p.WatermarkEnabled && p.WatermarkURL != "" {
		config.Watermark = &WatermarkConfig{
			Enabled:  true,
			URL:      p.WatermarkURL,
			Position: p.WatermarkPosition,
			Opacity:  p.WatermarkOpacity,
			Size:     p.WatermarkSize,
			OffsetY:  p.WatermarkOffsetY,
		}
	}

	// Preroll Ads (multiple) - ใช้ PrerollAds array ก่อน
	if len(p.PrerollAds) > 0 {
		config.PrerollAds = make([]PrerollConfig, len(p.PrerollAds))
		for i, ad := range p.PrerollAds {
			adType := string(ad.Type)
			if adType == "" {
				adType = "video" // default
			}
			config.PrerollAds[i] = PrerollConfig{
				Enabled:   true,
				Type:      adType,
				URL:       ad.URL,
				Duration:  ad.Duration,
				SkipAfter: ad.SkipAfter,
				ClickURL:  ad.ClickURL,
				ClickText: ad.ClickText,
				Title:     ad.Title,
			}
		}
	} else if p.PrerollEnabled && p.PrerollURL != "" {
		// Legacy: Fallback to single preroll field
		config.Preroll = &PrerollConfig{
			Enabled:   true,
			Type:      "video",
			URL:       p.PrerollURL,
			SkipAfter: p.PrerollSkipAfter,
		}
	}

	return config
}

// ProfilesToResponses แปลง slice ของ models เป็น slice ของ response DTOs
func ProfilesToResponses(profiles []*models.WhitelistProfile) []*WhitelistProfileResponse {
	responses := make([]*WhitelistProfileResponse, len(profiles))
	for i, p := range profiles {
		responses[i] = ProfileToResponse(p)
	}
	return responses
}

// AdStatsToResponse แปลง model เป็น response DTO
func AdStatsToResponse(s *models.AdImpressionStats) *AdImpressionStatsResponse {
	if s == nil {
		return nil
	}
	return &AdImpressionStatsResponse{
		TotalImpressions: s.TotalImpressions,
		Completed:        s.Completed,
		Skipped:          s.Skipped,
		Errors:           s.Errors,
		CompletionRate:   s.CompletionRate,
		SkipRate:         s.SkipRate,
		ErrorRate:        s.ErrorRate,
		AvgWatchDuration: s.AvgWatchDuration,
		AvgSkipTime:      s.AvgSkipTime,
	}
}

// DeviceStatsToResponse แปลง model เป็น response DTO
func DeviceStatsToResponse(s *models.DeviceStats) *DeviceStatsResponse {
	if s == nil {
		return nil
	}
	return &DeviceStatsResponse{
		Mobile:  s.Mobile,
		Desktop: s.Desktop,
		Tablet:  s.Tablet,
	}
}

// ProfileRankingsToResponses แปลง slice ของ rankings เป็น response DTOs
func ProfileRankingsToResponses(rankings []*models.ProfileAdStats) []*ProfileRankingResponse {
	responses := make([]*ProfileRankingResponse, len(rankings))
	for i, r := range rankings {
		responses[i] = &ProfileRankingResponse{
			ProfileID:      r.ProfileID,
			ProfileName:    r.ProfileName,
			TotalViews:     r.TotalViews,
			CompletionRate: r.CompletionRate,
		}
	}
	return responses
}

// PrerollAdToResponse แปลง preroll ad model เป็น response DTO
func PrerollAdToResponse(ad *models.PrerollAd) *PrerollAdResponse {
	if ad == nil {
		return nil
	}
	adType := string(ad.Type)
	if adType == "" {
		adType = "video" // default
	}
	return &PrerollAdResponse{
		ID:        ad.ID,
		ProfileID: ad.ProfileID,
		Type:      adType,
		URL:       ad.URL,
		Duration:  ad.Duration,
		SkipAfter: ad.SkipAfter,
		ClickURL:  ad.ClickURL,
		ClickText: ad.ClickText,
		Title:     ad.Title,
		SortOrder: ad.SortOrder,
		CreatedAt: ad.CreatedAt,
		UpdatedAt: ad.UpdatedAt,
	}
}

// PrerollAdsToResponses แปลง slice ของ preroll ads เป็น response DTOs
func PrerollAdsToResponses(ads []*models.PrerollAd) []PrerollAdResponse {
	responses := make([]PrerollAdResponse, len(ads))
	for i, ad := range ads {
		adType := string(ad.Type)
		if adType == "" {
			adType = "video" // default
		}
		responses[i] = PrerollAdResponse{
			ID:        ad.ID,
			ProfileID: ad.ProfileID,
			Type:      adType,
			URL:       ad.URL,
			Duration:  ad.Duration,
			SkipAfter: ad.SkipAfter,
			ClickURL:  ad.ClickURL,
			ClickText: ad.ClickText,
			Title:     ad.Title,
			SortOrder: ad.SortOrder,
			CreatedAt: ad.CreatedAt,
			UpdatedAt: ad.UpdatedAt,
		}
	}
	return responses
}
