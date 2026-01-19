package models

import (
	"time"

	"github.com/google/uuid"
)

// AdType ประเภทของโฆษณา
type AdType string

const (
	AdTypeVideo AdType = "video"
	AdTypeImage AdType = "image"
)

// PrerollAd เก็บข้อมูลโฆษณา pre-roll แต่ละตัว
// 1 Profile สามารถมีหลาย PrerollAd
type PrerollAd struct {
	ID        uuid.UUID `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	ProfileID uuid.UUID `gorm:"type:uuid;not null;index"`

	// Ad Type & Content
	Type     AdType `gorm:"type:varchar(10);default:'video'"` // video หรือ image
	URL      string `gorm:"type:text;not null"`               // URL ของ video หรือ image
	Duration int    `gorm:"default:0"`                        // ระยะเวลาแสดง (วินาที) - ใช้กับ image, 0 = ใช้ความยาว video

	// Skip Settings
	SkipAfter int `gorm:"default:5"` // วินาทีก่อนแสดงปุ่ม Skip (0 = บังคับดูจบ)

	// Click/Link Settings
	ClickURL  string `gorm:"type:text"`         // URL เมื่อคลิกโฆษณา
	ClickText string `gorm:"type:varchar(100)"` // ข้อความปุ่ม เช่น "ดูรายละเอียด"

	// Display Settings
	Title string `gorm:"type:varchar(255)"` // ชื่อโฆษณา/ผู้สนับสนุน

	// Order
	SortOrder int `gorm:"default:0"` // ลำดับการเล่น (0, 1, 2, ...)

	CreatedAt time.Time
	UpdatedAt time.Time

	// Relations
	Profile WhitelistProfile `gorm:"foreignKey:ProfileID;constraint:OnDelete:CASCADE"`
}

func (PrerollAd) TableName() string {
	return "preroll_ads"
}

// CanSkip ตรวจสอบว่าสามารถ skip ได้หรือไม่
func (p *PrerollAd) CanSkip() bool {
	return p.SkipAfter > 0
}

// IsVideo ตรวจสอบว่าเป็น video ad
func (p *PrerollAd) IsVideo() bool {
	return p.Type == AdTypeVideo || p.Type == ""
}

// IsImage ตรวจสอบว่าเป็น image ad
func (p *PrerollAd) IsImage() bool {
	return p.Type == AdTypeImage
}

// GetDuration คืนค่าระยะเวลา (สำหรับ image จะใช้ Duration, video จะคืน 0)
func (p *PrerollAd) GetDuration() int {
	if p.IsImage() && p.Duration > 0 {
		return p.Duration
	}
	return 0
}
