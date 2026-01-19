package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// AdImpression เก็บข้อมูลการแสดงโฆษณา (Ad Statistics)
type AdImpression struct {
	ID        uuid.UUID  `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	ProfileID *uuid.UUID `gorm:"type:uuid;index"` // nullable - อาจไม่มี profile
	VideoCode string     `gorm:"size:50;not null;index"`
	Domain    string     `gorm:"size:255"` // Domain ที่เล่น

	// Ad Playback Data
	AdURL         string `gorm:"type:text"`    // URL ของ Ad ที่เล่น
	AdDuration    int    `gorm:"default:0"`    // ความยาว Ad (วินาที)
	WatchDuration int    `gorm:"default:0"`    // ดูไปกี่วินาที
	Completed     bool   `gorm:"default:false"` // ดูจนจบหรือไม่
	Skipped       bool   `gorm:"default:false"` // กด Skip หรือไม่
	SkippedAt     int    `gorm:"default:0"`    // Skip ตอนวินาทีที่เท่าไหร่
	ErrorOccurred bool   `gorm:"default:false"` // Ad โหลดไม่ได้ (fallback to main video)

	// Client Info
	UserAgent  string `gorm:"type:text"`
	DeviceType string `gorm:"size:20;index"` // 'mobile' | 'desktop' | 'tablet'
	IPAddress  string `gorm:"size:45"`

	CreatedAt time.Time `gorm:"index"`

	// Relations
	Profile *WhitelistProfile `gorm:"foreignKey:ProfileID"`
}

func (AdImpression) TableName() string {
	return "ad_impressions"
}

// DeviceTypeEnum ประเภทอุปกรณ์
type DeviceTypeEnum string

const (
	DeviceMobile  DeviceTypeEnum = "mobile"
	DeviceDesktop DeviceTypeEnum = "desktop"
	DeviceTablet  DeviceTypeEnum = "tablet"
)

// ParseDeviceType วิเคราะห์ประเภทอุปกรณ์จาก User-Agent
func ParseDeviceType(userAgent string) string {
	ua := strings.ToLower(userAgent)

	// Mobile patterns
	mobileKeywords := []string{
		"mobile", "android", "iphone", "ipod", "blackberry",
		"windows phone", "opera mini", "opera mobi", "iemobile",
	}
	for _, keyword := range mobileKeywords {
		if strings.Contains(ua, keyword) {
			return string(DeviceMobile)
		}
	}

	// Tablet patterns (ต้องเช็คก่อน desktop เพราะบาง tablet มี "android" ด้วย)
	tabletKeywords := []string{"ipad", "tablet", "kindle", "silk", "playbook"}
	for _, keyword := range tabletKeywords {
		if strings.Contains(ua, keyword) {
			return string(DeviceTablet)
		}
	}

	// Android without "mobile" is likely tablet
	if strings.Contains(ua, "android") && !strings.Contains(ua, "mobile") {
		return string(DeviceTablet)
	}

	return string(DeviceDesktop)
}

// AdImpressionStats สถิติ Ad Impressions
type AdImpressionStats struct {
	TotalImpressions int64   `json:"totalImpressions"`
	Completed        int64   `json:"completed"`
	Skipped          int64   `json:"skipped"`
	Errors           int64   `json:"errors"`
	CompletionRate   float64 `json:"completionRate"` // percentage
	SkipRate         float64 `json:"skipRate"`       // percentage
	ErrorRate        float64 `json:"errorRate"`      // percentage
	AvgWatchDuration float64 `json:"avgWatchDuration"` // seconds
	AvgSkipTime      float64 `json:"avgSkipTime"`      // seconds (เฉลี่ย skip ตอนวินาทีที่เท่าไหร่)
}

// DeviceStats สถิติแยกตามอุปกรณ์
type DeviceStats struct {
	Mobile  int64 `json:"mobile"`
	Desktop int64 `json:"desktop"`
	Tablet  int64 `json:"tablet"`
}

// ProfileAdStats สถิติแยกตาม Profile
type ProfileAdStats struct {
	ProfileID      uuid.UUID `json:"profileId"`
	ProfileName    string    `json:"profileName"`
	TotalViews     int64     `json:"totalViews"`
	CompletionRate float64   `json:"completionRate"`
}
