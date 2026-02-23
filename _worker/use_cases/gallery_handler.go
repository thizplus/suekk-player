package use_cases

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"suekk-worker/domain/models"
	"suekk-worker/infrastructure/classifier"
	"suekk-worker/ports"
)

// ═══════════════════════════════════════════════════════════════════════════════
// GalleryHandler - Use Case สำหรับจัดการ Gallery Jobs
// สร้าง gallery images จาก HLS ที่มีอยู่แล้ว โดยใช้ S3 presigned URLs
// ═══════════════════════════════════════════════════════════════════════════════

// GalleryHandlerConfig configuration สำหรับ GalleryHandler
type GalleryHandlerConfig struct {
	TempDir  string // Directory สำหรับเก็บ temp files
	APIURL   string // API URL สำหรับ update video
	APIToken string // API token
}

// GalleryHandler handles gallery generation jobs from NATS
type GalleryHandler struct {
	storage   ports.StoragePort
	messenger ports.MessengerPort
	config    GalleryHandlerConfig
	logger    *slog.Logger
}

// NewGalleryHandler สร้าง GalleryHandler instance
func NewGalleryHandler(
	storage ports.StoragePort,
	messenger ports.MessengerPort,
	config GalleryHandlerConfig,
) *GalleryHandler {
	return &GalleryHandler{
		storage:   storage,
		messenger: messenger,
		config:    config,
		logger:    slog.Default().With("component", "gallery-handler"),
	}
}

// ProcessJob handles the gallery job from NATS JetStream
func (h *GalleryHandler) ProcessJob(ctx context.Context, job *models.GalleryJob) error {
	h.logger.Info("processing gallery job",
		"video_id", job.VideoID,
		"video_code", job.VideoCode,
		"quality", job.VideoQuality,
		"duration", job.Duration,
		"image_count", job.ImageCount,
	)

	// Publish initial progress
	h.publishProgress(ctx, job, 0, "เริ่มสร้าง Gallery...")

	// 1. Create temp directory
	outputDir := filepath.Join(h.config.TempDir, "gallery", job.VideoCode)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		h.publishFailed(ctx, job, err.Error())
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(outputDir) // Cleanup after done

	h.publishProgress(ctx, job, 5, "กำลังวิเคราะห์ HLS playlist...")

	// 2. Extract frames using FFmpeg with presigned URLs
	h.logger.Info("extracting frames from HLS", "hls_path", job.HLSPath)

	// Progress callback for frame extraction (5% - 85%)
	progressCallback := func(current, total int) {
		pct := 5.0 + (float64(current)/float64(total))*80.0
		msg := fmt.Sprintf("กำลังสร้างภาพ %d/%d...", current, total)
		h.publishProgress(ctx, job, pct, msg)
	}

	if err := h.extractFramesFromHLS(ctx, job, outputDir, progressCallback); err != nil {
		h.publishFailed(ctx, job, err.Error())
		return fmt.Errorf("extract frames: %w", err)
	}

	h.publishProgress(ctx, job, 85, "กำลังอัพโหลดภาพ...")

	// 4. Upload images to S3
	uploadedCount, err := h.uploadGalleryImages(ctx, outputDir, job.OutputPath, job.VideoCode)
	if err != nil {
		h.publishFailed(ctx, job, err.Error())
		return fmt.Errorf("upload gallery: %w", err)
	}

	h.logger.Info("gallery images uploaded",
		"video_code", job.VideoCode,
		"uploaded_count", uploadedCount,
	)

	h.publishProgress(ctx, job, 95, "กำลังบันทึกข้อมูล...")

	// 5. Update video in database via API
	if err := h.updateVideoGallery(ctx, job.VideoID, job.OutputPath, uploadedCount); err != nil {
		h.logger.Warn("failed to update video gallery in DB",
			"video_id", job.VideoID,
			"error", err,
		)
		// Don't fail the job - images are uploaded successfully
	}

	// Publish completed
	h.publishCompleted(ctx, job)

	h.logger.Info("gallery job completed",
		"video_id", job.VideoID,
		"video_code", job.VideoCode,
		"images", uploadedCount,
	)

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// ProcessJobWithClassification - Gallery Generation with NSFW Classification
// ═══════════════════════════════════════════════════════════════════════════════

// ProcessJobWithClassification handles gallery job with NSFW classification
// Uses Multi-Round extraction to ensure minimum safe images
func (h *GalleryHandler) ProcessJobWithClassification(ctx context.Context, job *models.GalleryJob) error {
	h.logger.Info("processing gallery job with classification",
		"video_id", job.VideoID,
		"video_code", job.VideoCode,
		"quality", job.VideoQuality,
		"duration", job.Duration,
	)

	// Publish initial progress
	h.publishProgress(ctx, job, 0, "เริ่มสร้าง Gallery + NSFW Classification...")

	// 1. Create temp directories (Three-Tier)
	baseDir := filepath.Join(h.config.TempDir, "gallery", job.VideoCode)
	allFramesDir := filepath.Join(baseDir, "all")
	superSafeDir := filepath.Join(baseDir, "super_safe") // < 0.15 + face (Public SEO)
	safeDir := filepath.Join(baseDir, "safe")            // 0.15-0.3 (Lazy load)
	nsfwDir := filepath.Join(baseDir, "nsfw")            // >= 0.3 (Member only)

	for _, dir := range []string{allFramesDir, superSafeDir, safeDir, nsfwDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			h.publishFailed(ctx, job, err.Error())
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}
	defer os.RemoveAll(baseDir) // Cleanup after done

	h.publishProgress(ctx, job, 5, "กำลังวิเคราะห์ HLS playlist...")

	// 2. Parse HLS playlist
	segments, err := h.parseHLSPlaylist(ctx, job.HLSPath)
	if err != nil {
		h.publishFailed(ctx, job, err.Error())
		return fmt.Errorf("parse playlist: %w", err)
	}

	if len(segments) == 0 {
		h.publishFailed(ctx, job, "no segments found in playlist")
		return fmt.Errorf("no segments found in playlist")
	}

	// 3. Initialize classifier (Three-Tier config)
	classifierConfig := classifier.ClassifierConfig{
		PythonPath:         "python",
		ScriptPath:         "infrastructure/classifier/classify_batch.py",
		NsfwThreshold:      0.3,
		SuperSafeThreshold: 0.15,
		MinFaceScore:       0.1,
		Timeout:            90,
		MaxNsfwImages:      30,
		MinSafeImages:      12,
		MinSuperSafeImages: 10,
	}
	nsfwClassifier := classifier.NewNSFWClassifier(classifierConfig, h.logger)

	// 4. Multi-Round extraction with classification (Three-Tier)
	var allSuperSafeResults []classifier.ClassificationResult
	var allSafeResults []classifier.ClassificationResult
	var allNsfwResults []classifier.ClassificationResult
	totalFrames := 0
	roundsUsed := 0

	extractionRounds := []struct {
		name       string
		startPct   float64
		endPct     float64
		frameCount int
		offset     float64
	}{
		{"standard", 0.05, 0.95, 100, 0},
		{"intro", 0.00, 0.15, 20, 0},
		{"outro", 0.90, 1.00, 15, 0},
		{"gap_fill", 0.05, 0.95, 30, 0.5},
		{"dense_intro", 0.00, 0.10, 30, 0.25},
	}

	timestampTracker := make(map[int]bool)
	minGap := 3 // minimum 3 seconds between frames

	for _, round := range extractionRounds {
		// Stop if we have enough super_safe AND safe images (Three-Tier)
		hasEnoughSuperSafe := len(allSuperSafeResults) >= classifierConfig.MinSuperSafeImages
		totalSafeCount := len(allSuperSafeResults) + len(allSafeResults)
		hasEnoughSafe := totalSafeCount >= classifierConfig.MinSafeImages

		if hasEnoughSuperSafe && hasEnoughSafe {
			break
		}

		roundsUsed++
		h.publishProgress(ctx, job, 10+float64(roundsUsed)*15, fmt.Sprintf("Round %d: %s...", roundsUsed, round.name))

		h.logger.Info("extraction round",
			"round", round.name,
			"current_super_safe", len(allSuperSafeResults),
			"current_safe", len(allSafeResults),
			"target_super_safe", classifierConfig.MinSuperSafeImages,
			"target_safe", classifierConfig.MinSafeImages,
		)

		// Extract frames for this round
		frameCount := h.extractRoundFramesFromHLS(
			ctx, job, segments, allFramesDir,
			round.startPct, round.endPct, round.frameCount, round.offset,
			timestampTracker, minGap, totalFrames,
		)

		if frameCount == 0 {
			continue
		}

		totalFrames += frameCount

		// Classify all frames
		result, err := nsfwClassifier.ClassifyBatch(ctx, allFramesDir)
		if err != nil {
			h.logger.Warn("classification failed", "round", round.name, "error", err)
			continue
		}

		// Separate and move files (Three-Tier)
		separated := nsfwClassifier.SeparateResults(result.Results)
		h.moveClassifiedFilesThreeTier(allFramesDir, superSafeDir, safeDir, nsfwDir, separated)

		allSuperSafeResults = append(allSuperSafeResults, separated.SuperSafe...)
		allSafeResults = append(allSafeResults, separated.Safe...)
		allNsfwResults = append(allNsfwResults, separated.Nsfw...)

		h.logger.Info("round complete",
			"round", round.name,
			"super_safe_found", len(separated.SuperSafe),
			"safe_found", len(separated.Safe),
			"total_super_safe", len(allSuperSafeResults),
			"total_safe", len(allSafeResults),
		)
	}

	// 5. Limit NSFW images to top 30 by quality
	nsfwClassifier.SortByQuality(allNsfwResults)
	if len(allNsfwResults) > classifierConfig.MaxNsfwImages {
		// Delete excess files
		for i := classifierConfig.MaxNsfwImages; i < len(allNsfwResults); i++ {
			os.Remove(filepath.Join(nsfwDir, allNsfwResults[i].Filename))
		}
		allNsfwResults = allNsfwResults[:classifierConfig.MaxNsfwImages]
	}

	// Sort safe images by quality
	nsfwClassifier.SortByQuality(allSafeResults)

	h.publishProgress(ctx, job, 85, "กำลังอัพโหลดภาพ...")

	// 6. Upload super_safe, safe, and nsfw folders (Three-Tier)
	superSafeUploaded, err := h.uploadGalleryImages(ctx, superSafeDir, job.OutputPath+"/super_safe", job.VideoCode)
	if err != nil {
		h.logger.Warn("failed to upload super_safe images", "error", err)
	}

	safeUploaded, err := h.uploadGalleryImages(ctx, safeDir, job.OutputPath+"/safe", job.VideoCode)
	if err != nil {
		h.logger.Warn("failed to upload safe images", "error", err)
	}

	nsfwUploaded, err := h.uploadGalleryImages(ctx, nsfwDir, job.OutputPath+"/nsfw", job.VideoCode)
	if err != nil {
		h.logger.Warn("failed to upload nsfw images", "error", err)
	}

	h.logger.Info("three-tier gallery uploaded",
		"video_code", job.VideoCode,
		"super_safe_uploaded", superSafeUploaded,
		"safe_uploaded", safeUploaded,
		"nsfw_uploaded", nsfwUploaded,
	)

	h.publishProgress(ctx, job, 95, "กำลังบันทึกข้อมูล...")

	// 7. Update video in database via API (Three-Tier)
	if err := h.updateVideoGalleryClassifiedThreeTier(ctx, job.VideoID, job.OutputPath, superSafeUploaded, safeUploaded, nsfwUploaded); err != nil {
		h.logger.Warn("failed to update classified gallery in DB",
			"video_id", job.VideoID,
			"error", err,
		)
	}

	// 8. Log classification stats (Three-Tier)
	h.logger.Info("classification_stats",
		"video_code", job.VideoCode,
		"total_frames", totalFrames,
		"super_safe_count", len(allSuperSafeResults),
		"safe_count", len(allSafeResults),
		"nsfw_count", len(allNsfwResults),
		"rounds_used", roundsUsed,
		"super_safe_target_met", len(allSuperSafeResults) >= classifierConfig.MinSuperSafeImages,
	)

	// Publish completed
	h.publishCompleted(ctx, job)

	h.logger.Info("classified gallery job completed (three-tier)",
		"video_id", job.VideoID,
		"video_code", job.VideoCode,
		"super_safe_images", superSafeUploaded,
		"safe_images", safeUploaded,
		"nsfw_images", nsfwUploaded,
	)

	return nil
}

// extractRoundFramesFromHLS extracts frames for a specific round from HLS
func (h *GalleryHandler) extractRoundFramesFromHLS(
	ctx context.Context,
	job *models.GalleryJob,
	segments []hlsSegment,
	outputDir string,
	startPct, endPct float64,
	frameCount int,
	offset float64,
	timestampTracker map[int]bool,
	minGap int,
	filenameOffset int,
) int {
	duration := float64(job.Duration)
	startTime := duration * startPct
	endTime := duration * endPct
	usableDuration := endTime - startTime

	if usableDuration <= 0 || frameCount <= 0 {
		return 0
	}

	interval := usableDuration / float64(frameCount)
	extracted := 0

	for i := 0; i < frameCount; i++ {
		select {
		case <-ctx.Done():
			return extracted
		default:
		}

		timestamp := startTime + (float64(i)+offset)*interval
		sec := int(timestamp)

		// Skip if timestamp already used (with minGap)
		used := false
		for t := sec - minGap; t <= sec+minGap; t++ {
			if timestampTracker[t] {
				used = true
				break
			}
		}
		if used {
			continue
		}

		// Find segment for this timestamp
		segment := h.findSegmentForTimestamp(segments, timestamp)
		if segment == nil {
			continue
		}

		// Get presigned URL
		segmentPath := filepath.Dir(job.HLSPath) + "/" + segment.filename
		segmentPath = strings.ReplaceAll(segmentPath, "\\", "/")

		presignedURL, err := h.storage.GetPresignedURL(ctx, segmentPath, 5*time.Minute)
		if err != nil {
			continue
		}

		// Capture frame
		frameNum := filenameOffset + extracted + 1
		outputPath := filepath.Join(outputDir, fmt.Sprintf("%03d.jpg", frameNum))

		if err := h.captureFrameFromSegment(ctx, presignedURL, outputPath, timestamp-segment.startTime); err != nil {
			continue
		}

		if _, err := os.Stat(outputPath); err == nil {
			timestampTracker[sec] = true
			extracted++
		}
	}

	return extracted
}

// moveClassifiedFilesThreeTier moves files to appropriate directories based on classification (Three-Tier)
func (h *GalleryHandler) moveClassifiedFilesThreeTier(srcDir, superSafeDir, safeDir, nsfwDir string, separated *classifier.SeparatedImages) {
	// Move super_safe files (< 0.15 + face) - สำหรับ Public SEO
	for _, img := range separated.SuperSafe {
		src := filepath.Join(srcDir, img.Filename)
		dst := filepath.Join(superSafeDir, img.Filename)
		if err := os.Rename(src, dst); err != nil {
			h.logger.Warn("failed to move super_safe image", "file", img.Filename, "error", err)
		}
	}

	// Move safe files (0.15-0.3) - Lazy load
	for _, img := range separated.Safe {
		src := filepath.Join(srcDir, img.Filename)
		dst := filepath.Join(safeDir, img.Filename)
		if err := os.Rename(src, dst); err != nil {
			h.logger.Warn("failed to move safe image", "file", img.Filename, "error", err)
		}
	}

	// Move nsfw files (>= 0.3) - Member only
	for _, img := range separated.Nsfw {
		src := filepath.Join(srcDir, img.Filename)
		dst := filepath.Join(nsfwDir, img.Filename)
		if err := os.Rename(src, dst); err != nil {
			h.logger.Warn("failed to move nsfw image", "file", img.Filename, "error", err)
		}
	}

	// Move error files to nsfw (safety first)
	for _, img := range separated.Error {
		src := filepath.Join(srcDir, img.Filename)
		dst := filepath.Join(nsfwDir, img.Filename)
		if err := os.Rename(src, dst); err != nil {
			h.logger.Warn("failed to move error image", "file", img.Filename, "error", err)
		}
	}
}

// updateVideoGalleryClassified updates video with safe/nsfw counts via API (deprecated, use Three-Tier)
func (h *GalleryHandler) updateVideoGalleryClassified(ctx context.Context, videoID, galleryPath string, safeCount, nsfwCount int) error {
	if h.config.APIURL == "" {
		return nil
	}

	url := fmt.Sprintf("%s/api/v1/internal/videos/%s/gallery", h.config.APIURL, videoID)

	payload := map[string]interface{}{
		"gallery_path":       galleryPath,
		"gallery_count":      safeCount, // Use safe_count as main gallery_count
		"gallery_safe_count": safeCount,
		"gallery_nsfw_count": nsfwCount,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if h.config.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+h.config.APIToken)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API returned %d", resp.StatusCode)
	}

	return nil
}

// updateVideoGalleryClassifiedThreeTier updates video with super_safe/safe/nsfw counts via API (Three-Tier)
func (h *GalleryHandler) updateVideoGalleryClassifiedThreeTier(ctx context.Context, videoID, galleryPath string, superSafeCount, safeCount, nsfwCount int) error {
	if h.config.APIURL == "" {
		return nil
	}

	url := fmt.Sprintf("%s/api/v1/internal/videos/%s/gallery", h.config.APIURL, videoID)

	// gallery_count = super_safe + safe (total public-accessible images)
	totalPublicCount := superSafeCount + safeCount

	payload := map[string]interface{}{
		"gallery_path":             galleryPath,
		"gallery_count":            totalPublicCount,  // Total safe images (backward compatible)
		"gallery_super_safe_count": superSafeCount,    // super_safe (< 0.15 + face) for Public SEO
		"gallery_safe_count":       safeCount,         // borderline (0.15-0.3)
		"gallery_nsfw_count":       nsfwCount,         // nsfw (>= 0.3)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if h.config.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+h.config.APIToken)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API returned %d", resp.StatusCode)
	}

	return nil
}

// publishProgress ส่ง progress update ไปยัง API
func (h *GalleryHandler) publishProgress(ctx context.Context, job *models.GalleryJob, progress float64, message string) {
	h.logger.Info("gallery progress",
		"video_code", job.VideoCode,
		"progress", progress,
		"message", message,
		"has_messenger", h.messenger != nil,
	)
	if h.messenger != nil {
		h.messenger.PublishGalleryProgress(ctx, job.VideoID, job.VideoCode, progress, message)
	}
}

// publishCompleted ส่ง completion status
func (h *GalleryHandler) publishCompleted(ctx context.Context, job *models.GalleryJob) {
	if h.messenger != nil {
		h.messenger.PublishGalleryCompleted(ctx, job.VideoID, job.VideoCode)
	}
}

// publishFailed ส่ง failure status
func (h *GalleryHandler) publishFailed(ctx context.Context, job *models.GalleryJob, errMsg string) {
	if h.messenger != nil {
		h.messenger.PublishGalleryFailed(ctx, job.VideoID, job.VideoCode, errMsg)
	}
}

// hlsSegment represents an HLS segment with timing info
type hlsSegment struct {
	filename  string
	duration  float64
	startTime float64 // cumulative start time
}

// GalleryProgressCallback callback สำหรับ report progress
type GalleryProgressCallback func(current, total int)

// extractFramesFromHLS extracts frames from HLS using S3 presigned URLs
func (h *GalleryHandler) extractFramesFromHLS(ctx context.Context, job *models.GalleryJob, outputDir string, progressCallback GalleryProgressCallback) error {
	hlsPath := job.HLSPath
	duration := job.Duration
	imageCount := job.ImageCount

	if imageCount <= 0 {
		imageCount = 100
	}

	// 1. Download and parse HLS playlist from S3
	segments, err := h.parseHLSPlaylist(ctx, hlsPath)
	if err != nil {
		return fmt.Errorf("parse playlist: %w", err)
	}

	if len(segments) == 0 {
		return fmt.Errorf("no segments found in playlist")
	}

	h.logger.Info("parsed HLS playlist",
		"segments", len(segments),
		"total_duration", segments[len(segments)-1].startTime+segments[len(segments)-1].duration,
	)

	// Calculate frame interval
	// ข้าม 5% แรกและ 5% หลัง
	startTime := float64(duration) * 0.05
	endTime := float64(duration) * 0.95
	usableDuration := endTime - startTime
	interval := usableDuration / float64(imageCount)

	h.logger.Info("ffmpeg extract params",
		"start_time", startTime,
		"end_time", endTime,
		"interval", interval,
		"image_count", imageCount,
	)

	// Extract each frame individually
	for i := 0; i < imageCount; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		timestamp := startTime + (float64(i) * interval)
		outputPath := filepath.Join(outputDir, fmt.Sprintf("%03d.jpg", i+1))

		// Find the segment that contains this timestamp
		segment := h.findSegmentForTimestamp(segments, timestamp)
		if segment == nil {
			h.logger.Warn("no segment found for timestamp",
				"frame", i+1,
				"timestamp", timestamp,
			)
			continue
		}

		// Get presigned URL for this segment
		segmentPath := filepath.Dir(hlsPath) + "/" + segment.filename
		segmentPath = strings.ReplaceAll(segmentPath, "\\", "/")

		presignedURL, err := h.storage.GetPresignedURL(ctx, segmentPath, 5*time.Minute)
		if err != nil {
			h.logger.Warn("failed to get presigned URL",
				"segment", segmentPath,
				"error", err,
			)
			continue
		}

		// Calculate seek time within segment
		seekInSegment := timestamp - segment.startTime
		if seekInSegment < 0 {
			seekInSegment = 0
		}

		if err := h.captureFrameFromSegment(ctx, presignedURL, outputPath, seekInSegment); err != nil {
			h.logger.Warn("failed to capture frame",
				"frame", i+1,
				"timestamp", timestamp,
				"segment", segment.filename,
				"error", err,
			)
			continue // Continue with other frames
		}

		// Report progress every 5 frames
		if progressCallback != nil && (i+1)%5 == 0 {
			progressCallback(i+1, imageCount)
		}
	}

	// Final progress callback
	if progressCallback != nil {
		progressCallback(imageCount, imageCount)
	}

	return nil
}

// parseHLSPlaylist downloads and parses HLS playlist to get segment info
func (h *GalleryHandler) parseHLSPlaylist(ctx context.Context, hlsPath string) ([]hlsSegment, error) {
	// Download playlist from S3
	localPlaylist := filepath.Join(h.config.TempDir, "temp_playlist.m3u8")
	if err := h.storage.Download(ctx, hlsPath, localPlaylist, nil); err != nil {
		return nil, fmt.Errorf("download playlist: %w", err)
	}
	defer os.Remove(localPlaylist)

	// Parse the playlist
	file, err := os.Open(localPlaylist)
	if err != nil {
		return nil, fmt.Errorf("open playlist: %w", err)
	}
	defer file.Close()

	var segments []hlsSegment
	var currentDuration float64
	var cumulativeTime float64

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Parse EXTINF duration
		if strings.HasPrefix(line, "#EXTINF:") {
			// Format: #EXTINF:2.000000,
			durStr := strings.TrimPrefix(line, "#EXTINF:")
			durStr = strings.TrimSuffix(durStr, ",")
			if idx := strings.Index(durStr, ","); idx > 0 {
				durStr = durStr[:idx]
			}
			if dur, err := strconv.ParseFloat(durStr, 64); err == nil {
				currentDuration = dur
			}
		} else if !strings.HasPrefix(line, "#") && line != "" {
			// This is a segment filename
			segments = append(segments, hlsSegment{
				filename:  line,
				duration:  currentDuration,
				startTime: cumulativeTime,
			})
			cumulativeTime += currentDuration
			currentDuration = 0
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan playlist: %w", err)
	}

	return segments, nil
}

// findSegmentForTimestamp finds the segment that contains the given timestamp
func (h *GalleryHandler) findSegmentForTimestamp(segments []hlsSegment, timestamp float64) *hlsSegment {
	for i := range segments {
		seg := &segments[i]
		if timestamp >= seg.startTime && timestamp < seg.startTime+seg.duration {
			return seg
		}
	}

	// If timestamp is beyond all segments, return the last one
	if len(segments) > 0 && timestamp >= segments[len(segments)-1].startTime {
		return &segments[len(segments)-1]
	}

	return nil
}

// captureFrameFromSegment captures a frame from a single segment using presigned URL
func (h *GalleryHandler) captureFrameFromSegment(ctx context.Context, segmentURL, outputPath string, seekTime float64) error {
	// Always extract first frame (no seeking) - segment selection already gives us the right time
	// Seeking within HLS segments is unreliable due to timestamp discontinuities
	_ = seekTime // unused, we always use first frame

	// FFmpeg command: extract first frame from segment
	args := []string{
		"-i", segmentURL,
		"-frames:v", "1",
		"-vf", "scale=1280:720:force_original_aspect_ratio=decrease,pad=1280:720:(ow-iw)/2:(oh-ih)/2",
		"-q:v", "2", // High quality JPEG
		"-y",        // Overwrite
		outputPath,
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "ffmpeg", args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("ffmpeg: %w, output: %s", err, string(output))
	}

	return nil
}


// uploadGalleryImages uploads all images in directory to S3
func (h *GalleryHandler) uploadGalleryImages(ctx context.Context, localDir, remotePrefix, videoCode string) (int, error) {
	uploadedCount := 0

	// Walk through directory
	err := filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Only upload .jpg files
		if filepath.Ext(path) != ".jpg" {
			return nil
		}

		// Calculate remote path: gallery/{code}/001.jpg
		filename := filepath.Base(path)
		remotePath := filepath.Join(remotePrefix, filename)
		remotePath = filepath.ToSlash(remotePath) // Convert to forward slashes for S3

		// Upload to S3 using UploadWithOptions (file path based)
		if err := h.storage.UploadWithOptions(ctx, remotePath, path, "image/jpeg", "public, max-age=31536000"); err != nil {
			h.logger.Warn("failed to upload image", "path", remotePath, "error", err)
			return nil
		}

		uploadedCount++
		return nil
	})

	if err != nil {
		return uploadedCount, fmt.Errorf("walk dir: %w", err)
	}

	return uploadedCount, nil
}

// updateVideoGallery updates video gallery info in database via API
func (h *GalleryHandler) updateVideoGallery(ctx context.Context, videoID, galleryPath string, galleryCount int) error {
	if h.config.APIURL == "" {
		return nil // Skip if API URL not configured
	}

	// PATCH /api/v1/videos/{id}
	url := fmt.Sprintf("%s/api/v1/internal/videos/%s/gallery", h.config.APIURL, videoID)

	payload := map[string]interface{}{
		"gallery_path":  galleryPath,
		"gallery_count": galleryCount,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if h.config.APIToken != "" {
		req.Header.Set("Authorization", "Bearer "+h.config.APIToken)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API returned %d", resp.StatusCode)
	}

	return nil
}
