package dto

import (
	"time"

	"github.com/google/uuid"
	"gofiber-template/domain/models"
)

// === Requests ===

// CreateReelRequest สร้าง reel ใหม่
type CreateReelRequest struct {
	VideoID      uuid.UUID           `json:"videoId" validate:"required,uuid"`
	Title        string              `json:"title" validate:"omitempty,max=255"`
	Description  string              `json:"description" validate:"omitempty,max=1000"`
	SegmentStart float64             `json:"segmentStart" validate:"min=0"`
	SegmentEnd   float64             `json:"segmentEnd" validate:"required,gtfield=SegmentStart"`
	OutputFormat string              `json:"outputFormat" validate:"omitempty,oneof=9:16 1:1 4:5 16:9"`
	VideoFit     string              `json:"videoFit" validate:"omitempty,oneof=fill fit crop-1:1 crop-4:3 crop-4:5"`
	CropX        float64             `json:"cropX" validate:"min=0,max=100"`
	CropY        float64             `json:"cropY" validate:"min=0,max=100"`
	TemplateID   *uuid.UUID          `json:"templateId" validate:"omitempty,uuid"`
	Layers       []ReelLayerRequest  `json:"layers" validate:"dive"`
}

// UpdateReelRequest อัปเดต reel
type UpdateReelRequest struct {
	Title        *string             `json:"title" validate:"omitempty,max=255"`
	Description  *string             `json:"description" validate:"omitempty,max=1000"`
	SegmentStart *float64            `json:"segmentStart" validate:"omitempty,min=0"`
	SegmentEnd   *float64            `json:"segmentEnd" validate:"omitempty"`
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

// ReelResponse reel response
type ReelResponse struct {
	ID           uuid.UUID           `json:"id"`
	Title        string              `json:"title"`
	Description  string              `json:"description"`
	SegmentStart float64             `json:"segmentStart"`
	SegmentEnd   float64             `json:"segmentEnd"`
	Duration     int                 `json:"duration"`
	OutputFormat string              `json:"outputFormat"`
	VideoFit     string              `json:"videoFit"`
	CropX        float64             `json:"cropX"`
	CropY        float64             `json:"cropY"`
	Status       models.ReelStatus   `json:"status"`
	OutputURL    string              `json:"outputUrl,omitempty"`
	ThumbnailURL string              `json:"thumbnailUrl,omitempty"`
	FileSize     int64               `json:"fileSize,omitempty"`
	ExportError  string              `json:"exportError,omitempty"`
	ExportedAt   *time.Time          `json:"exportedAt,omitempty"`
	Layers       []ReelLayerResponse `json:"layers"`
	Video        *VideoBasicResponse `json:"video,omitempty"`
	Template     *ReelTemplateBasic  `json:"template,omitempty"`
	CreatedAt    time.Time           `json:"createdAt"`
	UpdatedAt    time.Time           `json:"updatedAt"`
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
	ID           uuid.UUID `json:"id"`
	Code         string    `json:"code"`
	Title        string    `json:"title"`
	Duration     int       `json:"duration"`
	ThumbnailURL string    `json:"thumbnailUrl,omitempty"`
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
		ID:           reel.ID,
		Title:        reel.Title,
		Description:  reel.Description,
		SegmentStart: reel.SegmentStart,
		SegmentEnd:   reel.SegmentEnd,
		Duration:     reel.Duration,
		OutputFormat: string(reel.OutputFormat),
		VideoFit:     string(reel.VideoFit),
		CropX:        reel.CropX,
		CropY:        reel.CropY,
		Status:       reel.Status,
		OutputURL:    reel.OutputPath,
		ThumbnailURL: reel.ThumbnailURL,
		FileSize:     reel.FileSize,
		ExportError:  reel.ExportError,
		ExportedAt:   reel.ExportedAt,
		Layers:       layersToResponse(reel.Layers),
		CreatedAt:    reel.CreatedAt,
		UpdatedAt:    reel.UpdatedAt,
	}

	// Video info
	if reel.Video != nil {
		resp.Video = &VideoBasicResponse{
			ID:           reel.Video.ID,
			Code:         reel.Video.Code,
			Title:        reel.Video.Title,
			Duration:     reel.Video.Duration,
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
