package models

import (
	"time"

	"github.com/google/uuid"
)

// WhitelistProfile เก็บการตั้งค่า Whitelist แยกตามกลุ่ม
// 1 Profile สามารถมีหลาย allowed domains
type WhitelistProfile struct {
	ID          uuid.UUID `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Name        string    `gorm:"size:100;not null"`
	Description string    `gorm:"type:text"`
	IsActive    bool      `gorm:"default:true"`

	// Thumbnail Settings (แสดงก่อนกด play)
	ThumbnailURL string `gorm:"type:text"` // Custom thumbnail URL, ถ้าว่างใช้ thumbnail ของวิดีโอ

	// Watermark Settings
	WatermarkEnabled  bool    `gorm:"default:false"`
	WatermarkURL      string  `gorm:"type:text"`
	WatermarkPosition string  `gorm:"size:20;default:'bottom-right'"` // top-left, top-right, bottom-left, bottom-right
	WatermarkOpacity  float64 `gorm:"type:decimal(3,2);default:0.7"`  // 0.0 - 1.0
	WatermarkSize     int     `gorm:"default:100"`                    // pixel width
	WatermarkOffsetY  int     `gorm:"default:64"`                     // pixel offset จากขอบ (Safe Zone สำหรับ Mobile)

	// Pre-roll Ads Settings (deprecated - ใช้ PrerollAds แทน)
	PrerollEnabled   bool   `gorm:"default:false"`
	PrerollURL       string `gorm:"type:text"`        // URL ของ Ad video (.mp4, .m3u8)
	PrerollSkipAfter int    `gorm:"default:5"`        // วินาทีก่อนแสดงปุ่ม Skip (0 = ไม่มี Skip)

	CreatedAt time.Time
	UpdatedAt time.Time

	// Relations
	Domains    []ProfileDomain `gorm:"foreignKey:ProfileID;constraint:OnDelete:CASCADE"`
	PrerollAds []PrerollAd     `gorm:"foreignKey:ProfileID;constraint:OnDelete:CASCADE"`
}

func (WhitelistProfile) TableName() string {
	return "whitelist_profiles"
}

// HasWatermark ตรวจสอบว่ามี watermark หรือไม่
func (p *WhitelistProfile) HasWatermark() bool {
	return p.WatermarkEnabled && p.WatermarkURL != ""
}

// HasPreroll ตรวจสอบว่ามี pre-roll ad หรือไม่
func (p *WhitelistProfile) HasPreroll() bool {
	// ตรวจสอบ PrerollAds array ก่อน
	if len(p.PrerollAds) > 0 {
		return true
	}
	// Fallback to legacy field
	return p.PrerollEnabled && p.PrerollURL != ""
}

// CanSkipAd ตรวจสอบว่าสามารถ skip ad ได้หรือไม่
func (p *WhitelistProfile) CanSkipAd() bool {
	return p.PrerollSkipAfter > 0
}

// WatermarkPositionType ประเภทตำแหน่ง watermark
type WatermarkPositionType string

const (
	WatermarkTopLeft     WatermarkPositionType = "top-left"
	WatermarkTopRight    WatermarkPositionType = "top-right"
	WatermarkBottomLeft  WatermarkPositionType = "bottom-left"
	WatermarkBottomRight WatermarkPositionType = "bottom-right"
)

// ValidWatermarkPositions รายการตำแหน่งที่ถูกต้อง
var ValidWatermarkPositions = []WatermarkPositionType{
	WatermarkTopLeft,
	WatermarkTopRight,
	WatermarkBottomLeft,
	WatermarkBottomRight,
}

// IsValidWatermarkPosition ตรวจสอบว่าตำแหน่งถูกต้องหรือไม่
func IsValidWatermarkPosition(pos string) bool {
	for _, valid := range ValidWatermarkPositions {
		if string(valid) == pos {
			return true
		}
	}
	return false
}
