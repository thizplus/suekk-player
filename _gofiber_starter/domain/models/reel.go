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

// OutputFormat รูปแบบ output (aspect ratio)
type OutputFormat string

const (
	OutputFormat9x16 OutputFormat = "9:16"  // Reels/TikTok
	OutputFormat1x1  OutputFormat = "1:1"   // Square
	OutputFormat4x5  OutputFormat = "4:5"   // Instagram
	OutputFormat16x9 OutputFormat = "16:9"  // YouTube
)

// VideoFit วิธีการ fit video ในกรอบ (DEPRECATED - use ReelStyle instead)
type VideoFit string

const (
	VideoFitFill    VideoFit = "fill"      // Crop ให้เต็มกรอบ
	VideoFitFit     VideoFit = "fit"       // มี letterbox
	VideoFitCrop1x1 VideoFit = "crop-1:1"  // Crop เป็น 1:1
	VideoFitCrop4x3 VideoFit = "crop-4:3"  // Crop เป็น 4:3
	VideoFitCrop4x5 VideoFit = "crop-4:5"  // Crop เป็น 4:5
)

// ReelStyle สไตล์การแสดงผล reel (NEW simplified system)
type ReelStyle string

const (
	ReelStyleLetterbox ReelStyle = "letterbox" // 16:9 video centered with black bars
	ReelStyleSquare    ReelStyle = "square"    // 1:1 video centered with black bars
	ReelStyleFullcover ReelStyle = "fullcover" // Video fills entire 9:16 frame, sides cropped
)

// ValidReelStyles รายการสไตล์ที่รองรับ
var ValidReelStyles = []ReelStyle{
	ReelStyleLetterbox,
	ReelStyleSquare,
	ReelStyleFullcover,
}

// IsValidStyle ตรวจสอบว่า style ถูกต้อง
func IsValidStyle(style string) bool {
	for _, valid := range ValidReelStyles {
		if string(valid) == style {
			return true
		}
	}
	return false
}

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

// ═══════════════════════════════════════════════════════════════════════════════
// VideoSegment - Multi-segment support
// ═══════════════════════════════════════════════════════════════════════════════

// VideoSegment แต่ละช่วงเวลาใน reel
type VideoSegment struct {
	Start float64 `json:"start"` // start time (seconds)
	End   float64 `json:"end"`   // end time (seconds)
}

// VideoSegments custom type สำหรับเก็บ segments ใน JSONB
type VideoSegments []VideoSegment

// Scan implements sql.Scanner for VideoSegments
func (s *VideoSegments) Scan(value interface{}) error {
	if value == nil {
		*s = VideoSegments{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}

	return json.Unmarshal(bytes, s)
}

// Value implements driver.Valuer for VideoSegments
func (s VideoSegments) Value() (driver.Value, error) {
	if s == nil || len(s) == 0 {
		return "[]", nil
	}
	return json.Marshal(s)
}

// TotalDuration คำนวณ duration รวมของทุก segments
func (s VideoSegments) TotalDuration() float64 {
	total := 0.0
	for _, seg := range s {
		total += seg.End - seg.Start
	}
	return total
}

// Reel แต่ละ reel ที่สร้างจาก video
type Reel struct {
	ID        uuid.UUID `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	VideoID   uuid.UUID `gorm:"type:uuid;not null;index"`

	// Basic Info
	Title       string `gorm:"size:255"`
	Description string `gorm:"type:text"` // DEPRECATED - use Line1 instead

	// NEW: Style-based text fields
	Line1 string `gorm:"size:255"` // Secondary line 1
	Line2 string `gorm:"size:255"` // Secondary line 2

	// TTS (Text-to-Speech)
	TTSText string `gorm:"type:text"` // ข้อความสำหรับพากย์เสียง (ถ้าว่าง = ไม่มีเสียง)

	// Video Segments (Multi-segment support)
	Segments     VideoSegments `gorm:"type:jsonb;default:'[]'"` // หลายช่วงเวลา
	SegmentStart float64       `gorm:"default:0"`               // LEGACY: start time (seconds)
	SegmentEnd   float64       `gorm:"default:60"`              // LEGACY: end time (seconds)
	CoverTime    float64       `gorm:"default:-1"`              // cover/thumbnail time (-1 = auto middle)

	// NEW: Style-based display (simplified)
	Style    ReelStyle `gorm:"size:20;default:'letterbox'"` // letterbox, square, fullcover
	ShowLogo bool      `gorm:"default:true"`                // show logo overlay

	// LEGACY: Display Options (deprecated - kept for backward compatibility)
	OutputFormat OutputFormat `gorm:"size:10;default:'9:16'"` // output aspect ratio
	VideoFit     VideoFit     `gorm:"size:20;default:'fill'"` // how video fits in frame
	CropX        float64      `gorm:"default:50"`             // crop position X (0-100%)
	CropY        float64      `gorm:"default:50"`             // crop position Y (0-100%)

	// Template (optional) - DEPRECATED
	TemplateID *uuid.UUID `gorm:"type:uuid"` // null = custom

	// Composition Layers (JSONB) - DEPRECATED for style-based, kept for legacy
	Layers ReelLayers `gorm:"type:jsonb;default:'[]'"`

	// Output
	OutputPath   string     `gorm:"type:text"` // S3 path to MP4
	ThumbnailURL string     `gorm:"type:text"` // thumbnail image (9:16)
	CoverURL     string     `gorm:"type:text"` // cover image (1:1 with gradient bg)
	Duration     int        `gorm:"default:0"` // seconds
	FileSize     int64      `gorm:"default:0"` // bytes

	// Status
	Status      ReelStatus `gorm:"size:20;default:'draft'"`
	ExportError string     `gorm:"type:text"`
	ExportedAt  *time.Time `gorm:"type:timestamptz"`

	// Timestamps
	CreatedAt time.Time
	UpdatedAt time.Time

	// Relations
	User     *User         `gorm:"foreignKey:UserID"`
	Video    *Video        `gorm:"foreignKey:VideoID"`
	Template *ReelTemplate `gorm:"foreignKey:TemplateID"`
}

// IsStyleBased ตรวจสอบว่าใช้ระบบ style-based หรือไม่
func (r *Reel) IsStyleBased() bool {
	return r.Style != ""
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

// GetSegments returns segments (backward compatible)
// ถ้ามี Segments ใช้ Segments, ถ้าไม่มีใช้ SegmentStart/End
func (r *Reel) GetSegments() []VideoSegment {
	if len(r.Segments) > 0 {
		return r.Segments
	}
	// Fallback to legacy single segment
	return []VideoSegment{{Start: r.SegmentStart, End: r.SegmentEnd}}
}

// GetDuration คำนวณ duration รวมของทุก segments
func (r *Reel) GetDuration() float64 {
	if len(r.Segments) > 0 {
		return r.Segments.TotalDuration()
	}
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
