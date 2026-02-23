package dto

import (
	"time"

	"github.com/google/uuid"
	"gofiber-template/domain/models"
)

// === Requests ===

type CreateVideoRequest struct {
	Title       string     `json:"title" validate:"required,min=1,max=255"`
	Description string     `json:"description" validate:"omitempty,max=5000"`
	CategoryID  *uuid.UUID `json:"categoryId" validate:"omitempty,uuid"`
}

type UpdateVideoRequest struct {
	Title            *string    `json:"title" validate:"omitempty,min=1,max=255"`
	Description      *string    `json:"description" validate:"omitempty,max=5000"`
	CategoryID       *uuid.UUID `json:"categoryId" validate:"omitempty,uuid"`
	GalleryPath      *string    `json:"gallery_path"`       // S3 path prefix (worker callback)
	GalleryCount     *int       `json:"gallery_count"`      // Total gallery images (worker callback)
	GallerySafeCount *int       `json:"gallery_safe_count"` // Safe (SFW) images count
	GalleryNsfwCount *int       `json:"gallery_nsfw_count"` // NSFW images count
}

type VideoFilterRequest struct {
	Search     string `query:"search"`                                                           // ค้นหา title/code
	Status     string `query:"status" validate:"omitempty,oneof=pending queued processing ready failed"` // เพิ่ม queued
	CategoryID string `query:"categoryId" validate:"omitempty,uuid"`
	UserID     string `query:"userId" validate:"omitempty,uuid"`
	DateFrom   string `query:"dateFrom"`                              // วันที่เริ่มต้น (YYYY-MM-DD)
	DateTo     string `query:"dateTo"`                                // วันที่สิ้นสุด (YYYY-MM-DD)
	SortBy     string `query:"sortBy" validate:"omitempty,oneof=created_at title views"` // เรียงตาม
	SortOrder  string `query:"sortOrder" validate:"omitempty,oneof=asc desc"`            // asc หรือ desc
	Page       int    `query:"page" validate:"omitempty,min=1"`
	Limit      int    `query:"limit" validate:"omitempty,min=1,max=100"`
}

// === Responses ===

type VideoResponse struct {
	ID           uuid.UUID          `json:"id"`
	Code         string             `json:"code"`
	Title        string             `json:"title"`
	Description  string             `json:"description"`
	Duration     int                `json:"duration"`
	Quality      string             `json:"quality"`
	ThumbnailURL string             `json:"thumbnailUrl"`
	HLSPath      string             `json:"hlsPath,omitempty"`       // H.265 master playlist
	HLSPathH264  string             `json:"hlsPathH264,omitempty"`   // H.264 fallback playlist
	DiskUsage    int64              `json:"diskUsage,omitempty"`     // ขนาดไฟล์รวม (bytes)
	QualitySizes map[string]int64   `json:"qualitySizes,omitempty"`  // ขนาดแยกตาม quality {"1080p": bytes}
	Status       models.VideoStatus `json:"status"`
	Views        int64              `json:"views"`
	Category     *CategoryResponse  `json:"category,omitempty"`
	User         *UserBasicResponse `json:"user,omitempty"`

	// Audio/Subtitle info
	HasAudio         bool               `json:"hasAudio"`                   // มี audio ที่ตัดไว้หรือไม่
	DetectedLanguage string             `json:"detectedLanguage,omitempty"` // ภาษาที่ตรวจพบ
	SubtitleSummary  *SubtitleSummary   `json:"subtitleSummary,omitempty"`  // สรุป subtitle
	Subtitles        []SubtitleResponse `json:"subtitles,omitempty"`        // Full subtitle list (สำหรับ embed/preview)

	// Reel info
	ReelCount int64 `json:"reelCount"` // จำนวน reels ที่สร้างจาก video นี้

	// Gallery info (สำหรับ video > 20 นาที)
	GalleryPath      string `json:"galleryPath,omitempty"`      // S3 path prefix e.g., "gallery/ABC123"
	GalleryCount     int    `json:"galleryCount,omitempty"`     // จำนวนภาพทั้งหมด (0 = ไม่มี)
	GallerySafeCount int    `json:"gallerySafeCount,omitempty"` // จำนวนภาพ safe (SFW)
	GalleryNsfwCount int    `json:"galleryNsfwCount,omitempty"` // จำนวนภาพ nsfw

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type VideoListResponse struct {
	Videos []VideoResponse `json:"videos"`
	Meta   PaginationMeta  `json:"meta"`
}

type VideoUploadResponse struct {
	ID           uuid.UUID `json:"id"`
	Code         string    `json:"code"`
	Title        string    `json:"title"`
	Status       string    `json:"status"`
	AutoEnqueued bool      `json:"autoEnqueued"` // ถูกส่งเข้า queue โดยอัตโนมัติหรือไม่
}

type EmbedVideoResponse struct {
	Code         string `json:"code"`
	Title        string `json:"title"`
	HLSPath      string `json:"hlsPath"`
	ThumbnailURL string `json:"thumbnailUrl"`
	Duration     int    `json:"duration"`
}

// ErrorRecordResponse สำหรับแสดง error record
type ErrorRecordResponse struct {
	Attempt   int    `json:"attempt"`
	Error     string `json:"error"`
	WorkerID  string `json:"workerId"`
	Stage     string `json:"stage"`
	Timestamp string `json:"timestamp"`
}

// DLQVideoResponse สำหรับแสดง video ใน Dead Letter Queue
type DLQVideoResponse struct {
	ID           uuid.UUID             `json:"id"`
	Code         string                `json:"code"`
	Title        string                `json:"title"`
	RetryCount   int                   `json:"retryCount"`
	LastError    string                `json:"lastError"`
	ErrorHistory []ErrorRecordResponse `json:"errorHistory,omitempty"`
	CreatedAt    time.Time             `json:"createdAt"`
	UpdatedAt    time.Time             `json:"updatedAt"`
	UserID       uuid.UUID             `json:"userId"`
}

// === Helper Types ===

// SubtitleSummary สรุปข้อมูล subtitle สำหรับแสดงใน video list
type SubtitleSummary struct {
	Original     *SubtitleBrief   `json:"original,omitempty"`     // Original subtitle (null if none)
	Translations []SubtitleBrief  `json:"translations,omitempty"` // Translated subtitles
}

// SubtitleBrief ข้อมูล subtitle แบบย่อ
type SubtitleBrief struct {
	Language string `json:"language"`
	Status   string `json:"status"`
}

type UserBasicResponse struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Avatar   string    `json:"avatar,omitempty"`
}

// === Mappers ===

func VideoToVideoResponse(video *models.Video) *VideoResponse {
	if video == nil {
		return nil
	}

	response := &VideoResponse{
		ID:               video.ID,
		Code:             video.Code,
		Title:            video.Title,
		Description:      video.Description,
		Duration:         video.Duration,
		Quality:          video.Quality,
		ThumbnailURL:     video.ThumbnailURL,
		HLSPath:          video.HLSPath,
		HLSPathH264:      video.HLSPathH264,
		DiskUsage:        video.DiskUsage,
		QualitySizes:     video.QualitySizes,
		Status:           video.Status,
		Views:            video.Views,
		HasAudio:         video.AudioPath != "",
		DetectedLanguage: video.DetectedLanguage,
		GalleryPath:      video.GalleryPath,
		GalleryCount:     video.GalleryCount,
		GallerySafeCount: video.GallerySafeCount,
		GalleryNsfwCount: video.GalleryNsfwCount,
		CreatedAt:        video.CreatedAt,
		UpdatedAt:        video.UpdatedAt,
	}

	if video.Category != nil {
		response.Category = CategoryToCategoryResponse(video.Category)
	}

	if video.User != nil {
		response.User = &UserBasicResponse{
			ID:       video.User.ID,
			Username: video.User.Username,
			Avatar:   video.User.Avatar,
		}
	}

	// Build subtitle summary and full list if subtitles are loaded
	if len(video.Subtitles) > 0 {
		response.SubtitleSummary = buildSubtitleSummary(video.Subtitles)
		response.Subtitles = SubtitlesToResponses(video.Subtitles)
	}

	return response
}

// buildSubtitleSummary สร้าง SubtitleSummary จาก subtitles
func buildSubtitleSummary(subtitles []*models.Subtitle) *SubtitleSummary {
	if len(subtitles) == 0 {
		return nil
	}

	summary := &SubtitleSummary{
		Translations: []SubtitleBrief{},
	}

	for _, sub := range subtitles {
		brief := SubtitleBrief{
			Language: sub.Language,
			Status:   string(sub.Status),
		}

		if sub.Type == models.SubtitleTypeOriginal {
			summary.Original = &brief
		} else {
			summary.Translations = append(summary.Translations, brief)
		}
	}

	return summary
}

func VideosToVideoResponses(videos []*models.Video) []VideoResponse {
	responses := make([]VideoResponse, len(videos))
	for i, video := range videos {
		responses[i] = *VideoToVideoResponse(video)
	}
	return responses
}

// VideosToVideoResponsesWithReelCounts แปลง videos พร้อม reel counts
func VideosToVideoResponsesWithReelCounts(videos []*models.Video, reelCounts map[uuid.UUID]int64) []VideoResponse {
	responses := make([]VideoResponse, len(videos))
	for i, video := range videos {
		resp := VideoToVideoResponse(video)
		if reelCounts != nil {
			resp.ReelCount = reelCounts[video.ID]
		}
		responses[i] = *resp
	}
	return responses
}

func VideoToEmbedResponse(video *models.Video) *EmbedVideoResponse {
	if video == nil {
		return nil
	}
	return &EmbedVideoResponse{
		Code:         video.Code,
		Title:        video.Title,
		HLSPath:      video.HLSPath,
		ThumbnailURL: video.ThumbnailURL,
		Duration:     video.Duration,
	}
}
