package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ReelStatus สถานะของ reel
type ReelStatus string

const (
	ReelStatusDraft      ReelStatus = "draft"      // กำลังแก้ไข
	ReelStatusExporting  ReelStatus = "exporting"  // กำลัง export
	ReelStatusReady      ReelStatus = "ready"      // export สำเร็จ
	ReelStatusFailed     ReelStatus = "failed"     // export ล้มเหลว
)

// ReelLayerType ประเภทของ layer
type ReelLayerType string

const (
	ReelLayerTypeText       ReelLayerType = "text"
	ReelLayerTypeImage      ReelLayerType = "image"
	ReelLayerTypeShape      ReelLayerType = "shape"
	ReelLayerTypeBackground ReelLayerType = "background"
)

// ReelLayer แต่ละ layer ใน reel composition
type ReelLayer struct {
	Type       ReelLayerType `json:"type"`                 // text, image, shape, background
	Content    string        `json:"content,omitempty"`    // text content หรือ image URL
	FontFamily string        `json:"fontFamily,omitempty"` // font family (สำหรับ text)
	FontSize   int           `json:"fontSize,omitempty"`   // font size (สำหรับ text)
	FontColor  string        `json:"fontColor,omitempty"`  // font color (สำหรับ text)
	FontWeight string        `json:"fontWeight,omitempty"` // normal, bold
	X          float64       `json:"x"`                    // position X (0-100%)
	Y          float64       `json:"y"`                    // position Y (0-100%)
	Width      float64       `json:"width,omitempty"`      // width (0-100%)
	Height     float64       `json:"height,omitempty"`     // height (0-100%)
	Opacity    float64       `json:"opacity,omitempty"`    // 0-1 (default 1)
	ZIndex     int           `json:"zIndex,omitempty"`     // layer order
	Style      string        `json:"style,omitempty"`      // gradient style, shape type, etc.
}

// ReelLayers custom type สำหรับเก็บ layers ใน JSONB
type ReelLayers []ReelLayer

// Scan implements sql.Scanner for ReelLayers
func (l *ReelLayers) Scan(value interface{}) error {
	if value == nil {
		*l = ReelLayers{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}

	return json.Unmarshal(bytes, l)
}

// Value implements driver.Valuer for ReelLayers
func (l ReelLayers) Value() (driver.Value, error) {
	if l == nil {
		return "[]", nil
	}
	return json.Marshal(l)
}

// Reel แต่ละ reel ที่สร้างจาก video
type Reel struct {
	ID        uuid.UUID  `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null;index"`
	VideoID   uuid.UUID  `gorm:"type:uuid;not null;index"`

	// Basic Info
	Title       string     `gorm:"size:255"`
	Description string     `gorm:"type:text"`

	// Video Segment
	SegmentStart float64 `gorm:"default:0"`  // start time (seconds)
	SegmentEnd   float64 `gorm:"default:60"` // end time (seconds)

	// Template (optional)
	TemplateID *uuid.UUID `gorm:"type:uuid"` // null = custom

	// Composition Layers (JSONB)
	Layers ReelLayers `gorm:"type:jsonb;default:'[]'"`

	// Output
	OutputPath   string     `gorm:"type:text"` // S3 path to MP4
	ThumbnailURL string     `gorm:"type:text"` // thumbnail image
	Duration     int        `gorm:"default:0"` // seconds
	FileSize     int64      `gorm:"default:0"` // bytes

	// Status
	Status       ReelStatus `gorm:"size:20;default:'draft'"`
	ExportError  string     `gorm:"type:text"`
	ExportedAt   *time.Time `gorm:"type:timestamptz"`

	// Timestamps
	CreatedAt time.Time
	UpdatedAt time.Time

	// Relations
	User     *User         `gorm:"foreignKey:UserID"`
	Video    *Video        `gorm:"foreignKey:VideoID"`
	Template *ReelTemplate `gorm:"foreignKey:TemplateID"`
}

func (Reel) TableName() string {
	return "reels"
}

// IsDraft ตรวจสอบว่า reel ยังอยู่ใน draft mode
func (r *Reel) IsDraft() bool {
	return r.Status == ReelStatusDraft
}

// IsReady ตรวจสอบว่า reel export สำเร็จ
func (r *Reel) IsReady() bool {
	return r.Status == ReelStatusReady
}

// IsExporting ตรวจสอบว่า reel กำลัง export
func (r *Reel) IsExporting() bool {
	return r.Status == ReelStatusExporting
}

// IsFailed ตรวจสอบว่า reel export ล้มเหลว
func (r *Reel) IsFailed() bool {
	return r.Status == ReelStatusFailed
}

// GetDuration คำนวณ duration ของ segment
func (r *Reel) GetDuration() float64 {
	return r.SegmentEnd - r.SegmentStart
}

// ReelTemplate template สำเร็จรูปสำหรับสร้าง reel
type ReelTemplate struct {
	ID          uuid.UUID  `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Name        string     `gorm:"size:100;not null"`
	Description string     `gorm:"type:text"`
	Thumbnail   string     `gorm:"type:text"` // preview image

	// Default Layers
	DefaultLayers ReelLayers `gorm:"type:jsonb;default:'[]'"`

	// Styling
	BackgroundStyle string `gorm:"size:50"` // gradient-1, blur, solid-black, etc.
	FontFamily      string `gorm:"size:50;default:'Google Sans'"`
	PrimaryColor    string `gorm:"size:20"` // main accent color
	SecondaryColor  string `gorm:"size:20"` // secondary color

	// Metadata
	IsActive  bool `gorm:"default:true"`
	SortOrder int  `gorm:"default:0"`

	// Timestamps
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (ReelTemplate) TableName() string {
	return "reel_templates"
}
