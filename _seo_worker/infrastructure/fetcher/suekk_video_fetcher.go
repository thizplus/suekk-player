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
// Three-Tier Priority: super_safe → safe → fallback to main gallery
// super_safe = NSFW < 0.15 + มีหน้าคน (ดีที่สุดสำหรับ SEO)
func (f *SuekkVideoFetcher) ListGalleryImages(ctx context.Context, galleryPath string) ([]string, error) {
	if galleryPath == "" {
		return nil, nil
	}

	// Trim trailing slash เพื่อป้องกัน double slash
	galleryPath = strings.TrimSuffix(galleryPath, "/")

	var files []string
	var err error
	var usedPath string

	// Priority 1: super_safe (NSFW < 0.15 + face) - ดีที่สุดสำหรับ SEO
	superSafePath := galleryPath + "/super_safe"
	files, err = f.storage.ListFiles(superSafePath)
	if err == nil && len(files) > 0 {
		usedPath = superSafePath
		f.logger.InfoContext(ctx, "Using super_safe gallery (Three-Tier)",
			"path", superSafePath,
			"count", len(files),
		)
	} else {
		// Priority 2: safe (NSFW 0.15-0.3)
		safePath := galleryPath + "/safe"
		files, err = f.storage.ListFiles(safePath)
		if err == nil && len(files) > 0 {
			usedPath = safePath
			f.logger.InfoContext(ctx, "Using safe gallery (fallback from super_safe)",
				"path", safePath,
				"count", len(files),
			)
		} else {
			// Priority 3: Fallback to main gallery (legacy videos)
			f.logger.WarnContext(ctx, "No classified gallery found, falling back to main gallery",
				"super_safe_path", superSafePath,
				"safe_path", safePath,
			)
			files, err = f.storage.ListFiles(galleryPath)
			if err != nil {
				return nil, err
			}
			usedPath = galleryPath
		}
	}

	// Filter only image files and build presigned URLs
	// IMPORTANT: ต้องกรอง subfolder ออกเฉพาะเมื่อใช้ main gallery (fallback)
	var imageURLs []string
	isUsingMainGallery := usedPath == galleryPath
	for _, file := range files {
		// Skip subfolders เฉพาะเมื่อใช้ main gallery (ป้องกัน fallback ดึงภาพจาก subfolder มา)
		if isUsingMainGallery {
			if strings.Contains(file, "/nsfw/") || strings.Contains(file, "/safe/") || strings.Contains(file, "/super_safe/") {
				continue
			}
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

	f.logger.InfoContext(ctx, "Gallery images listed (SEO-safe)",
		"path", usedPath,
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
