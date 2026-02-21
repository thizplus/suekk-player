package dto

import (
	"time"

	"github.com/google/uuid"
	"gofiber-template/domain/models"
)

// === Requests ===

// VideoSegmentRequest segment ใน request
type VideoSegmentRequest struct {
	Start float64 `json:"start" validate:"min=0"`
	End   float64 `json:"end" validate:"required,gtfield=Start"`
}

// CreateReelRequest สร้าง reel ใหม่
type CreateReelRequest struct {
	VideoID uuid.UUID `json:"videoId" validate:"required,uuid"`

	// Multi-segment support (preferred)
	Segments []VideoSegmentRequest `json:"segments" validate:"omitempty,min=1,max=10,dive"`

	// LEGACY: Single segment (still supported for backward compatibility)
	SegmentStart float64  `json:"segmentStart" validate:"min=0"`
	SegmentEnd   float64  `json:"segmentEnd" validate:"omitempty,gtfield=SegmentStart"`
	CoverTime    *float64 `json:"coverTime"` // nil = auto middle of segment

	// NEW: Style-based fields (preferred)
	Style    string `json:"style" validate:"omitempty,oneof=letterbox square fullcover"`
	Title    string `json:"title" validate:"omitempty,max=255"`
	Line1    string `json:"line1" validate:"omitempty,max=255"`
	Line2    string `json:"line2" validate:"omitempty,max=255"`
	ShowLogo *bool  `json:"showLogo"` // nil = true (default)

	// TTS (Text-to-Speech)
	TTSText  string `json:"ttsText" validate:"omitempty,max=5000"`  // ข้อความพากย์เสียง
	TTSVoice string `json:"ttsVoice" validate:"omitempty,max=50"`   // Voice ID (ถ้าว่าง = default)

	// LEGACY: Layer-based fields (deprecated but still supported)
	Description  string             `json:"description" validate:"omitempty,max=1000"` // deprecated, use line1
	OutputFormat string             `json:"outputFormat" validate:"omitempty,oneof=9:16 1:1 4:5 16:9"`
	VideoFit     string             `json:"videoFit" validate:"omitempty,oneof=fill fit crop-1:1 crop-4:3 crop-4:5"`
	CropX        float64            `json:"cropX" validate:"min=0,max=100"`
	CropY        float64            `json:"cropY" validate:"min=0,max=100"`
	TemplateID   *uuid.UUID         `json:"templateId" validate:"omitempty,uuid"`
	Layers       []ReelLayerRequest `json:"layers" validate:"dive"`
}

// UpdateReelRequest อัปเดต reel
type UpdateReelRequest struct {
	// Multi-segment support (preferred)
	Segments *[]VideoSegmentRequest `json:"segments" validate:"omitempty,min=1,max=10,dive"`

	// LEGACY: Single segment (still supported)
	SegmentStart *float64 `json:"segmentStart" validate:"omitempty,min=0"`
	SegmentEnd   *float64 `json:"segmentEnd" validate:"omitempty"`
	CoverTime    *float64 `json:"coverTime"` // nil = no change, -1 = auto middle

	// NEW: Style-based fields (preferred)
	Style    *string `json:"style" validate:"omitempty,oneof=letterbox square fullcover"`
	Title    *string `json:"title" validate:"omitempty,max=255"`
	Line1    *string `json:"line1" validate:"omitempty,max=255"`
	Line2    *string `json:"line2" validate:"omitempty,max=255"`
	ShowLogo *bool   `json:"showLogo"`

	// TTS (Text-to-Speech)
	TTSText  *string `json:"ttsText" validate:"omitempty,max=5000"`  // ข้อความพากย์เสียง
	TTSVoice *string `json:"ttsVoice" validate:"omitempty,max=50"`   // Voice ID (ถ้าว่าง = default)

	// LEGACY: Layer-based fields (deprecated but still supported)
	Description  *string             `json:"description" validate:"omitempty,max=1000"`
	OutputFormat *string             `json:"outputFormat" validate:"omitempty,oneof=9:16 1:1 4:5 16:9"`
	VideoFit     *string             `json:"videoFit" validate:"omitempty,oneof=fill fit crop-1:1 crop-4:3 crop-4:5"`
	CropX        *float64            `json:"cropX" validate:"omitempty,min=0,max=100"`
	CropY        *float64            `json:"cropY" validate:"omitempty,min=0,max=100"`
	TemplateID   *uuid.UUID          `json:"templateId" validate:"omitempty,uuid"`
	Layers       *[]ReelLayerRequest `json:"layers" validate:"omitempty,dive"`
}

// ReelLayerRequest layer ใน request
type ReelLayerRequest struct {
	Type       string  `json:"type" validate:"required,oneof=text image shape background"`
	Content    string  `json:"content" validate:"omitempty,max=500"`
	FontFamily string  `json:"fontFamily" validate:"omitempty,max=50"`
	FontSize   int     `json:"fontSize" validate:"omitempty,min=8,max=200"`
	FontColor  string  `json:"fontColor" validate:"omitempty,max=20"`
	FontWeight string  `json:"fontWeight" validate:"omitempty,oneof=normal bold"`
	X          float64 `json:"x" validate:"min=0,max=100"`
	Y          float64 `json:"y" validate:"min=0,max=100"`
	Width      float64 `json:"width" validate:"omitempty,min=0,max=100"`
	Height     float64 `json:"height" validate:"omitempty,min=0,max=100"`
	Opacity    float64 `json:"opacity" validate:"omitempty,min=0,max=1"`
	ZIndex     int     `json:"zIndex" validate:"omitempty,min=0,max=100"`
	Style      string  `json:"style" validate:"omitempty,max=50"`
}

// ReelFilterRequest filter สำหรับ list reels
type ReelFilterRequest struct {
	VideoID   string `query:"videoId" validate:"omitempty,uuid"`
	Status    string `query:"status" validate:"omitempty,oneof=draft exporting ready failed"`
	Search    string `query:"search"`
	SortBy    string `query:"sortBy" validate:"omitempty,oneof=created_at updated_at title"`
	SortOrder string `query:"sortOrder" validate:"omitempty,oneof=asc desc"`
	Page      int    `query:"page" validate:"omitempty,min=1"`
	Limit     int    `query:"limit" validate:"omitempty,min=1,max=100"`
}

// === Responses ===

// VideoSegmentResponse segment ใน response
type VideoSegmentResponse struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// ReelResponse reel response
type ReelResponse struct {
	ID       uuid.UUID         `json:"id"`
	Duration int               `json:"duration"`
	Status   models.ReelStatus `json:"status"`

	// Multi-segment support
	Segments []VideoSegmentResponse `json:"segments"`

	// LEGACY: Single segment (for backward compatibility)
	SegmentStart float64 `json:"segmentStart"`
	SegmentEnd   float64 `json:"segmentEnd"`
	CoverTime    float64 `json:"coverTime"` // -1 = auto middle

	// NEW: Style-based fields
	Style    string `json:"style,omitempty"`
	Title    string `json:"title"`
	Line1    string `json:"line1,omitempty"`
	Line2    string `json:"line2,omitempty"`
	ShowLogo bool   `json:"showLogo"`

	// TTS (Text-to-Speech)
	TTSText  string `json:"ttsText,omitempty"`  // ข้อความพากย์เสียง
	TTSVoice string `json:"ttsVoice,omitempty"` // Voice ID

	// Output
	OutputURL    string     `json:"outputUrl,omitempty"`
	ThumbnailURL string     `json:"thumbnailUrl,omitempty"`
	FileSize     int64      `json:"fileSize,omitempty"`
	ExportError  string     `json:"exportError,omitempty"`
	ExportedAt   *time.Time `json:"exportedAt,omitempty"`

	// LEGACY: Layer-based fields (for backward compatibility)
	Description  string              `json:"description,omitempty"`
	OutputFormat string              `json:"outputFormat,omitempty"`
	VideoFit     string              `json:"videoFit,omitempty"`
	CropX        float64             `json:"cropX,omitempty"`
	CropY        float64             `json:"cropY,omitempty"`
	Layers       []ReelLayerResponse `json:"layers,omitempty"`
	Template     *ReelTemplateBasic  `json:"template,omitempty"`

	// Relations
	Video     *VideoBasicResponse `json:"video,omitempty"`
	CreatedAt time.Time           `json:"createdAt"`
	UpdatedAt time.Time           `json:"updatedAt"`
}

// ReelLayerResponse layer ใน response
type ReelLayerResponse struct {
	Type       string  `json:"type"`
	Content    string  `json:"content,omitempty"`
	FontFamily string  `json:"fontFamily,omitempty"`
	FontSize   int     `json:"fontSize,omitempty"`
	FontColor  string  `json:"fontColor,omitempty"`
	FontWeight string  `json:"fontWeight,omitempty"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Width      float64 `json:"width,omitempty"`
	Height     float64 `json:"height,omitempty"`
	Opacity    float64 `json:"opacity,omitempty"`
	ZIndex     int     `json:"zIndex,omitempty"`
	Style      string  `json:"style,omitempty"`
}

// VideoBasicResponse ข้อมูล video แบบย่อ
type VideoBasicResponse struct {
	ID           uuid.UUID           `json:"id"`
	Code         string              `json:"code"`
	Title        string              `json:"title"`
	Duration     int                 `json:"duration"`
	Status       models.VideoStatus  `json:"status"`
	ThumbnailURL string              `json:"thumbnailUrl,omitempty"`
}

// ReelTemplateResponse template response
type ReelTemplateResponse struct {
	ID              uuid.UUID           `json:"id"`
	Name            string              `json:"name"`
	Description     string              `json:"description"`
	Thumbnail       string              `json:"thumbnail,omitempty"`
	BackgroundStyle string              `json:"backgroundStyle,omitempty"`
	FontFamily      string              `json:"fontFamily,omitempty"`
	PrimaryColor    string              `json:"primaryColor,omitempty"`
	SecondaryColor  string              `json:"secondaryColor,omitempty"`
	DefaultLayers   []ReelLayerResponse `json:"defaultLayers"`
	IsActive        bool                `json:"isActive"`
	SortOrder       int                 `json:"sortOrder"`
	CreatedAt       time.Time           `json:"createdAt"`
}

// ReelTemplateBasic template แบบย่อ
type ReelTemplateBasic struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// ReelListResponse list response พร้อม pagination
type ReelListResponse struct {
	Reels []ReelResponse `json:"reels"`
	Meta  PaginationMeta `json:"meta"`
}

// ReelExportResponse response หลัง export
type ReelExportResponse struct {
	ID        uuid.UUID         `json:"id"`
	Status    models.ReelStatus `json:"status"`
	Message   string            `json:"message"`
}

// === Mappers ===

// ReelToResponse แปลง model เป็น response
func ReelToResponse(reel *models.Reel) *ReelResponse {
	if reel == nil {
		return nil
	}

	resp := &ReelResponse{
		ID:       reel.ID,
		Duration: reel.Duration,
		Status:   reel.Status,

		// Multi-segment support
		Segments: segmentsToResponse(reel.GetSegments()),

		// LEGACY: Single segment (first segment start, last segment end)
		SegmentStart: reel.SegmentStart,
		SegmentEnd:   reel.SegmentEnd,
		CoverTime:    reel.CoverTime,

		// Style-based fields
		Style:    string(reel.Style),
		Title:    reel.Title,
		Line1:    reel.Line1,
		Line2:    reel.Line2,
		ShowLogo: reel.ShowLogo,

		// TTS
		TTSText:  reel.TTSText,
		TTSVoice: reel.TTSVoice,

		// Output
		OutputURL:    reel.OutputPath,
		ThumbnailURL: reel.ThumbnailURL,
		FileSize:     reel.FileSize,
		ExportError:  reel.ExportError,
		ExportedAt:   reel.ExportedAt,

		// LEGACY: For backward compatibility
		Description:  reel.Description,
		OutputFormat: string(reel.OutputFormat),
		VideoFit:     string(reel.VideoFit),
		CropX:        reel.CropX,
		CropY:        reel.CropY,
		Layers:       layersToResponse(reel.Layers),

		CreatedAt: reel.CreatedAt,
		UpdatedAt: reel.UpdatedAt,
	}

	// Video info
	if reel.Video != nil {
		resp.Video = &VideoBasicResponse{
			ID:           reel.Video.ID,
			Code:         reel.Video.Code,
			Title:        reel.Video.Title,
			Duration:     reel.Video.Duration,
			Status:       reel.Video.Status,
			ThumbnailURL: reel.Video.ThumbnailURL,
		}
	}

	// Template info
	if reel.Template != nil {
		resp.Template = &ReelTemplateBasic{
			ID:   reel.Template.ID,
			Name: reel.Template.Name,
		}
	}

	return resp
}

// ReelsToResponses แปลง models เป็น responses
func ReelsToResponses(reels []*models.Reel) []ReelResponse {
	responses := make([]ReelResponse, len(reels))
	for i, reel := range reels {
		responses[i] = *ReelToResponse(reel)
	}
	return responses
}

// ReelTemplateToResponse แปลง template model เป็น response
func ReelTemplateToResponse(t *models.ReelTemplate) *ReelTemplateResponse {
	if t == nil {
		return nil
	}

	return &ReelTemplateResponse{
		ID:              t.ID,
		Name:            t.Name,
		Description:     t.Description,
		Thumbnail:       t.Thumbnail,
		BackgroundStyle: t.BackgroundStyle,
		FontFamily:      t.FontFamily,
		PrimaryColor:    t.PrimaryColor,
		SecondaryColor:  t.SecondaryColor,
		DefaultLayers:   layersToResponse(t.DefaultLayers),
		IsActive:        t.IsActive,
		SortOrder:       t.SortOrder,
		CreatedAt:       t.CreatedAt,
	}
}

// ReelTemplatesToResponses แปลง templates เป็น responses
func ReelTemplatesToResponses(templates []*models.ReelTemplate) []ReelTemplateResponse {
	responses := make([]ReelTemplateResponse, len(templates))
	for i, t := range templates {
		responses[i] = *ReelTemplateToResponse(t)
	}
	return responses
}

// LayerRequestsToModels แปลง layer requests เป็น models
func LayerRequestsToModels(layers []ReelLayerRequest) models.ReelLayers {
	result := make(models.ReelLayers, len(layers))
	for i, l := range layers {
		result[i] = models.ReelLayer{
			Type:       models.ReelLayerType(l.Type),
			Content:    l.Content,
			FontFamily: l.FontFamily,
			FontSize:   l.FontSize,
			FontColor:  l.FontColor,
			FontWeight: l.FontWeight,
			X:          l.X,
			Y:          l.Y,
			Width:      l.Width,
			Height:     l.Height,
			Opacity:    l.Opacity,
			ZIndex:     l.ZIndex,
			Style:      l.Style,
		}
	}
	return result
}

// layersToResponse แปลง model layers เป็น response
func layersToResponse(layers models.ReelLayers) []ReelLayerResponse {
	result := make([]ReelLayerResponse, len(layers))
	for i, l := range layers {
		result[i] = ReelLayerResponse{
			Type:       string(l.Type),
			Content:    l.Content,
			FontFamily: l.FontFamily,
			FontSize:   l.FontSize,
			FontColor:  l.FontColor,
			FontWeight: l.FontWeight,
			X:          l.X,
			Y:          l.Y,
			Width:      l.Width,
			Height:     l.Height,
			Opacity:    l.Opacity,
			ZIndex:     l.ZIndex,
			Style:      l.Style,
		}
	}
	return result
}

// === Segment Mappers ===

// SegmentRequestsToModels แปลง segment requests เป็น models
func SegmentRequestsToModels(segments []VideoSegmentRequest) models.VideoSegments {
	result := make(models.VideoSegments, len(segments))
	for i, s := range segments {
		result[i] = models.VideoSegment{
			Start: s.Start,
			End:   s.End,
		}
	}
	return result
}

// segmentsToResponse แปลง model segments เป็น response
func segmentsToResponse(segments []models.VideoSegment) []VideoSegmentResponse {
	result := make([]VideoSegmentResponse, len(segments))
	for i, s := range segments {
		result[i] = VideoSegmentResponse{
			Start: s.Start,
			End:   s.End,
		}
	}
	return result
}
