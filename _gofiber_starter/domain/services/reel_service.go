package services

import (
	"context"

	"github.com/google/uuid"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
)

type ReelService interface {
	// Create สร้าง reel ใหม่
	Create(ctx context.Context, userID uuid.UUID, req *dto.CreateReelRequest) (*models.Reel, error)

	// GetByID ดึง reel ตาม ID
	GetByID(ctx context.Context, id uuid.UUID) (*models.Reel, error)

	// GetByIDForUser ดึง reel ตาม ID (ตรวจสอบ ownership)
	GetByIDForUser(ctx context.Context, id, userID uuid.UUID) (*models.Reel, error)

	// Update อัปเดต reel
	Update(ctx context.Context, id, userID uuid.UUID, req *dto.UpdateReelRequest) (*models.Reel, error)

	// Delete ลบ reel
	Delete(ctx context.Context, id, userID uuid.UUID) error

	// List ดึง reels ของ user พร้อม pagination
	ListByUser(ctx context.Context, userID uuid.UUID, page, limit int) ([]*models.Reel, int64, error)

	// ListByVideo ดึง reels ที่สร้างจาก video นี้
	ListByVideo(ctx context.Context, videoID uuid.UUID, page, limit int) ([]*models.Reel, int64, error)

	// ListWithFilters ดึง reels พร้อม filters
	ListWithFilters(ctx context.Context, userID uuid.UUID, params *dto.ReelFilterRequest) ([]*models.Reel, int64, error)

	// Export ส่ง reel ไป export queue
	Export(ctx context.Context, id, userID uuid.UUID) error

	// GetTemplates ดึง templates ทั้งหมด (active)
	GetTemplates(ctx context.Context) ([]*models.ReelTemplate, error)

	// GetTemplateByID ดึง template ตาม ID
	GetTemplateByID(ctx context.Context, id uuid.UUID) (*models.ReelTemplate, error)
}

// ReelJobPublisher interface สำหรับส่ง reel jobs ไปยัง NATS
type ReelJobPublisher interface {
	// PublishReelExportJob ส่ง reel export job
	PublishReelExportJob(ctx context.Context, job *ReelExportJob) error
}

// VideoSegmentJob segment ใน export job
type VideoSegmentJob struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// ReelExportJob job สำหรับ export reel เป็น MP4
type ReelExportJob struct {
	ReelID       string  `json:"reel_id"`
	VideoID      string  `json:"video_id"`
	VideoCode    string  `json:"video_code"`
	HLSPath      string  `json:"hls_path"`      // S3 path to HLS master playlist
	VideoQuality string  `json:"video_quality"` // Best available quality: 1080p, 720p, etc.

	// Multi-segment support
	Segments []VideoSegmentJob `json:"segments"` // หลายช่วงเวลา

	// LEGACY: Single segment (for backward compatibility)
	SegmentStart float64 `json:"segment_start"` // Start time in seconds
	SegmentEnd   float64 `json:"segment_end"`   // End time in seconds
	CoverTime    float64 `json:"cover_time"`    // Cover/thumbnail time (-1 = auto middle)

	// NEW: Style-based composition (simplified)
	Style        string  `json:"style"`         // letterbox, square, fullcover
	Title        string  `json:"title"`         // Main title text
	Line1        string  `json:"line1"`         // Secondary line 1
	Line2        string  `json:"line2"`         // Secondary line 2
	ShowLogo     bool    `json:"show_logo"`     // Show logo overlay
	LogoPath     string  `json:"logo_path"`     // S3 path to logo PNG (optional)
	GradientPath string  `json:"gradient_path"` // S3 path to gradient PNG (for fullcover)
	CropX        float64 `json:"crop_x"`        // 0-100 crop position X (for square/fullcover)
	CropY        float64 `json:"crop_y"`        // 0-100 crop position Y (for square)

	// TTS (Text-to-Speech)
	TTSText string `json:"tts_text"` // ข้อความพากย์เสียง (ถ้าว่าง = ไม่มีเสียง)

	// LEGACY: Layer-based composition (deprecated)
	OutputFormat string         `json:"output_format"` // 9:16, 1:1, 4:5, 16:9
	VideoFit     string         `json:"video_fit"`     // fill, fit, crop-1:1, crop-4:3, crop-4:5
	Layers       []ReelLayerJob `json:"layers"`        // Composition layers

	OutputPath string `json:"output_path"` // S3 path for MP4 output
}

// ReelLayerJob layer ใน export job
type ReelLayerJob struct {
	Type       string  `json:"type"`
	Content    string  `json:"content,omitempty"`
	FontFamily string  `json:"font_family,omitempty"`
	FontSize   int     `json:"font_size,omitempty"`
	FontColor  string  `json:"font_color,omitempty"`
	FontWeight string  `json:"font_weight,omitempty"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Width      float64 `json:"width,omitempty"`
	Height     float64 `json:"height,omitempty"`
	Opacity    float64 `json:"opacity,omitempty"`
	ZIndex     int     `json:"z_index,omitempty"`
	Style      string  `json:"style,omitempty"`
}
