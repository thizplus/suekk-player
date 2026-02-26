package imagecopier

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"seo-worker/domain/models"
	"seo-worker/domain/ports"
)

// ImageCopier - Copy images from e2 (suekk) to r2 (subth)
type ImageCopier struct {
	sourceStorage ports.StoragePort // e2 (suekk)
	destStorage   ports.StoragePort // r2 (subth)
	httpClient    *http.Client
	logger        *slog.Logger
}

func NewImageCopier(sourceStorage, destStorage ports.StoragePort) *ImageCopier {
	return &ImageCopier{
		sourceStorage: sourceStorage,
		destStorage:   destStorage,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: slog.Default().With("component", "image_copier"),
	}
}

// CopyGalleryImages copy ภาพ gallery จาก e2 ไป r2 (parallel)
func (c *ImageCopier) CopyGalleryImages(ctx context.Context, videoCode string, images []models.GalleryImage) ([]models.GalleryImage, error) {
	if len(images) == 0 {
		return images, nil
	}

	c.logger.InfoContext(ctx, "Starting gallery copy",
		"video_code", videoCode,
		"image_count", len(images),
	)

	// Copy in parallel with semaphore (max 5 concurrent)
	const maxConcurrent = 5
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []error

	result := make([]models.GalleryImage, len(images))

	for i, img := range images {
		wg.Add(1)
		go func(idx int, image models.GalleryImage) {
			defer wg.Done()

			sem <- struct{}{}        // acquire
			defer func() { <-sem }() // release

			// Extract filename from URL (strip query params)
			filename := extractFilename(image.URL, idx)

			// Copy the image
			newURL, err := c.CopyImage(ctx, videoCode, image.URL, filename)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("failed to copy %s: %w", image.URL, err))
				mu.Unlock()

				// Keep original URL on error
				result[idx] = image
				return
			}

			// Update with new URL
			mu.Lock()
			result[idx] = models.GalleryImage{
				URL:    newURL,
				Alt:    image.Alt,
				Width:  image.Width,
				Height: image.Height,
			}
			mu.Unlock()

		}(i, img)
	}

	wg.Wait()

	if len(errors) > 0 {
		c.logger.WarnContext(ctx, "Some images failed to copy",
			"video_code", videoCode,
			"error_count", len(errors),
			"total_count", len(images),
		)
		// Log first few errors
		for i, err := range errors {
			if i >= 3 {
				break
			}
			c.logger.WarnContext(ctx, "Copy error", "error", err)
		}
	}

	// Count successful copies
	successCount := 0
	for _, img := range result {
		if strings.Contains(img.URL, "files.subth.com") {
			successCount++
		}
	}

	c.logger.InfoContext(ctx, "Gallery copy completed",
		"video_code", videoCode,
		"success_count", successCount,
		"total_count", len(images),
	)

	return result, nil
}

// CopyImage copy ภาพเดี่ยวจาก e2 ไป r2
func (c *ImageCopier) CopyImage(ctx context.Context, videoCode string, srcURL string, filename string) (string, error) {
	// Destination path: articles/{videoCode}/gallery/{filename}
	destPath := fmt.Sprintf("articles/%s/gallery/%s", videoCode, filename)

	// Check if already exists in destination
	exists, _ := c.destStorage.Exists(ctx, destPath)
	if exists {
		c.logger.DebugContext(ctx, "Image already exists in r2, skipping",
			"path", destPath,
		)
		return c.destStorage.GetPublicURL(destPath), nil
	}

	// Download from source (could be URL or storage path)
	var data []byte
	var err error

	if strings.HasPrefix(srcURL, "http://") || strings.HasPrefix(srcURL, "https://") {
		// Download from HTTP URL
		data, err = c.downloadFromURL(ctx, srcURL)
	} else {
		// Read from source storage
		data, err = c.readFromStorage(ctx, srcURL)
	}

	if err != nil {
		return "", fmt.Errorf("failed to download image: %w", err)
	}

	// Detect content type
	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") {
		contentType = "image/jpeg" // default
	}

	// Upload to destination
	if err := c.destStorage.Upload(ctx, destPath, data, contentType); err != nil {
		return "", fmt.Errorf("failed to upload to r2: %w", err)
	}

	newURL := c.destStorage.GetPublicURL(destPath)

	c.logger.DebugContext(ctx, "Image copied",
		"src", srcURL,
		"dest", destPath,
		"size", len(data),
	)

	return newURL, nil
}

// downloadFromURL downloads image from HTTP URL
func (c *ImageCopier) downloadFromURL(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Limit to 10MB
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, err
	}

	return data, nil
}

// readFromStorage reads from source storage
func (c *ImageCopier) readFromStorage(ctx context.Context, path string) ([]byte, error) {
	reader, _, err := c.sourceStorage.GetFileContent(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

// extractFilename extracts clean filename from URL (strips query params)
func extractFilename(rawURL string, fallbackIdx int) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Sprintf("gallery_%d.jpg", fallbackIdx)
	}

	filename := path.Base(parsed.Path)
	if filename == "" || filename == "." || filename == "/" {
		return fmt.Sprintf("gallery_%d.jpg", fallbackIdx)
	}

	return filename
}

// CopyTieredGallery copy ภาพจากทุก tier ไป r2 แยก path
// Two-Tier System (Admin Manual Selection):
// - articles/{code}/gallery/public/  = safe (admin approved - SEO safe)
// - articles/{code}/gallery/member/  = nsfw (admin approved - members only)
func (c *ImageCopier) CopyTieredGallery(ctx context.Context, videoCode string, tiered *models.TieredGalleryImages) (*ports.CopiedGalleryResult, error) {
	if tiered == nil {
		return nil, nil
	}

	result := &ports.CopiedGalleryResult{
		PublicImages: []models.GalleryImage{},
		MemberImages: []models.GalleryImage{},
	}

	c.logger.InfoContext(ctx, "Starting tiered gallery copy",
		"video_code", videoCode,
		"safe", len(tiered.Safe),
		"nsfw", len(tiered.NSFW),
	)

	// Copy safe → public/ (admin approved for SEO)
	for i, srcURL := range tiered.Safe {
		filename := fmt.Sprintf("%03d.jpg", i+1)
		destPath := fmt.Sprintf("articles/%s/gallery/public/%s", videoCode, filename)

		newURL, err := c.copyToPath(ctx, srcURL, destPath)
		if err != nil {
			c.logger.WarnContext(ctx, "Failed to copy safe image", "error", err)
			continue
		}

		result.PublicImages = append(result.PublicImages, models.GalleryImage{
			URL:    newURL,
			Width:  1280,
			Height: 720,
		})

		// ใช้ภาพแรกเป็น cover
		if i == 0 {
			coverPath := fmt.Sprintf("articles/%s/gallery/cover.jpg", videoCode)
			coverURL, err := c.copyToPath(ctx, srcURL, coverPath)
			if err == nil {
				result.CoverURL = coverURL
			}
		}
	}

	// Copy nsfw → member/ (members only)
	for i, srcURL := range tiered.NSFW {
		filename := fmt.Sprintf("%03d.jpg", i+1)
		destPath := fmt.Sprintf("articles/%s/gallery/member/%s", videoCode, filename)

		newURL, err := c.copyToPath(ctx, srcURL, destPath)
		if err != nil {
			c.logger.WarnContext(ctx, "Failed to copy nsfw image", "error", err)
			continue
		}

		result.MemberImages = append(result.MemberImages, models.GalleryImage{
			URL:    newURL,
			Width:  1280,
			Height: 720,
		})
	}

	c.logger.InfoContext(ctx, "Tiered gallery copy completed",
		"video_code", videoCode,
		"public_count", len(result.PublicImages),
		"member_count", len(result.MemberImages),
		"has_cover", result.CoverURL != "",
	)

	return result, nil
}

// copyToPath copy ภาพไปยัง path ที่กำหนด
func (c *ImageCopier) copyToPath(ctx context.Context, srcURL string, destPath string) (string, error) {
	// Check if already exists
	exists, _ := c.destStorage.Exists(ctx, destPath)
	if exists {
		return c.destStorage.GetPublicURL(destPath), nil
	}

	// Download
	var data []byte
	var err error

	if strings.HasPrefix(srcURL, "http://") || strings.HasPrefix(srcURL, "https://") {
		data, err = c.downloadFromURL(ctx, srcURL)
	} else {
		data, err = c.readFromStorage(ctx, srcURL)
	}

	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}

	// Detect content type
	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") {
		contentType = "image/jpeg"
	}

	// Upload
	if err := c.destStorage.Upload(ctx, destPath, data, contentType); err != nil {
		return "", fmt.Errorf("failed to upload: %w", err)
	}

	return c.destStorage.GetPublicURL(destPath), nil
}

// Verify interface implementation
var _ ports.ImageCopierPort = (*ImageCopier)(nil)
