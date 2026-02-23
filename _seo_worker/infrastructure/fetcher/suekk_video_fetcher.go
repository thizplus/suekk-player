package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"seo-worker/domain/models"
	"seo-worker/domain/ports"
	"seo-worker/infrastructure/auth"
)

// Presigned URL expiry สำหรับ gallery images (1 ชั่วโมง)
const galleryURLExpiry = 1 * time.Hour

// SuekkVideoFetcher ดึงข้อมูล video จาก api.suekk.com
type SuekkVideoFetcher struct {
	apiURL     string
	authClient *auth.AuthClient
	httpClient *http.Client
	storage    ports.StoragePort
	logger     *slog.Logger
}

func NewSuekkVideoFetcher(apiURL string, authClient *auth.AuthClient, storage ports.StoragePort) *SuekkVideoFetcher {
	return &SuekkVideoFetcher{
		apiURL:     apiURL,
		authClient: authClient,
		storage:    storage,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: slog.Default().With("component", "suekk_video_fetcher"),
	}
}

type suekkVideoResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Code             string `json:"code"`
		Duration         int    `json:"duration"`
		ThumbnailURL     string `json:"thumbnailUrl"`
		GalleryPath      string `json:"galleryPath"`
		GalleryCount     int    `json:"galleryCount"`
		GallerySafeCount int    `json:"gallerySafeCount"` // จำนวนภาพ safe (pre-classified)
		GalleryNsfwCount int    `json:"galleryNsfwCount"` // จำนวนภาพ nsfw (pre-classified)
	} `json:"data"`
	Error string `json:"error,omitempty"`
}

// FetchVideoInfo ดึงข้อมูล video จาก api.suekk.com
func (f *SuekkVideoFetcher) FetchVideoInfo(ctx context.Context, videoCode string) (*models.SuekkVideoInfo, error) {
	url := fmt.Sprintf("%s/api/v1/videos/code/%s", f.apiURL, videoCode)

	// Get token
	token, err := f.authClient.GetToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle 401 - retry with new token
	if resp.StatusCode == http.StatusUnauthorized {
		f.authClient.InvalidateToken()
		return f.FetchVideoInfo(ctx, videoCode)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var result suekkVideoResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("API error: %s", result.Error)
	}

	f.logger.InfoContext(ctx, "[DEBUG] Suekk video info fetched",
		"video_code", videoCode,
		"duration", result.Data.Duration,
		"gallery_path", result.Data.GalleryPath,
		"gallery_count", result.Data.GalleryCount,
		"gallery_safe_count", result.Data.GallerySafeCount,
		"gallery_nsfw_count", result.Data.GalleryNsfwCount,
		"thumbnail_url", result.Data.ThumbnailURL,
	)

	return &models.SuekkVideoInfo{
		Code:             result.Data.Code,
		Duration:         result.Data.Duration,
		ThumbnailURL:     result.Data.ThumbnailURL,
		GalleryPath:      result.Data.GalleryPath,
		GalleryCount:     result.Data.GalleryCount,
		GallerySafeCount: result.Data.GallerySafeCount,
		GalleryNsfwCount: result.Data.GalleryNsfwCount,
	}, nil
}

// ListGalleryImages ดึงรายการ gallery images จาก storage (ใช้ presigned URLs)
// ใช้ภาพจาก /safe/ subfolder ที่ _worker classify ไว้แล้ว
func (f *SuekkVideoFetcher) ListGalleryImages(ctx context.Context, galleryPath string) ([]string, error) {
	if galleryPath == "" {
		return nil, nil
	}

	// ใช้ safe subfolder เท่านั้น (pre-classified by _worker)
	safePath := galleryPath + "/safe"

	// List files from storage
	files, err := f.storage.ListFiles(safePath)
	if err != nil || len(files) == 0 {
		// Fallback: ถ้าไม่มี /safe subfolder (legacy videos) ใช้ path เดิม
		f.logger.WarnContext(ctx, "Safe gallery not found, falling back to main gallery",
			"safe_path", safePath,
			"error", err,
		)
		files, err = f.storage.ListFiles(galleryPath)
		if err != nil {
			return nil, err
		}
	}

	// Filter only image files and build presigned URLs
	// IMPORTANT: ต้องกรอง /nsfw/ ออก เพราะ fallback อาจ list ทั้ง folder
	var imageURLs []string
	for _, file := range files {
		// Skip NSFW files (ป้องกัน fallback ดึงภาพจาก /nsfw/ subfolder มา)
		if strings.Contains(file, "/nsfw/") {
			continue
		}

		// Check if it's an image
		if isImageFile(file) {
			// ใช้ presigned URL เพราะ E2 bucket เป็น private
			url, err := f.storage.GetPresignedDownloadURL(file, galleryURLExpiry)
			if err != nil {
				f.logger.WarnContext(ctx, "Failed to get presigned URL",
					"file", file,
					"error", err,
				)
				continue
			}
			imageURLs = append(imageURLs, url)
		}
	}

	f.logger.InfoContext(ctx, "Gallery images listed (safe only)",
		"path", safePath,
		"count", len(imageURLs),
	)

	return imageURLs, nil
}

// isImageFile checks if the file is an image
func isImageFile(filename string) bool {
	extensions := []string{".jpg", ".jpeg", ".png", ".webp", ".gif"}
	for _, ext := range extensions {
		if len(filename) > len(ext) && filename[len(filename)-len(ext):] == ext {
			return true
		}
	}
	return false
}

// Verify interface implementation
var _ ports.SuekkVideoFetcherPort = (*SuekkVideoFetcher)(nil)
