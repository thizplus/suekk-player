package use_cases

import (
	"bufio"
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
	"suekk-worker/infrastructure/gallery"
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
	TestMode bool   // TEST_MODE: skip upload & DB update, keep files locally
}

// GalleryAuthClientPort interface สำหรับ auth client
type GalleryAuthClientPort interface {
	DoRequestWithAuth(ctx context.Context, method, url string, body []byte) (*http.Response, error)
	IsConfigured() bool
}

// GalleryHandler handles gallery generation jobs from NATS
type GalleryHandler struct {
	storage         ports.StoragePort
	messenger       ports.MessengerPort
	repository      ports.VideoRepository
	authClient      GalleryAuthClientPort
	galleryService  *gallery.Service
	galleryUploader *gallery.Uploader
	config          GalleryHandlerConfig
	logger          *slog.Logger
}

// NewGalleryHandler สร้าง GalleryHandler instance
func NewGalleryHandler(
	storage ports.StoragePort,
	messenger ports.MessengerPort,
	repository ports.VideoRepository,
	authClient GalleryAuthClientPort,
	galleryService *gallery.Service,
	galleryUploader *gallery.Uploader,
	config GalleryHandlerConfig,
) *GalleryHandler {
	return &GalleryHandler{
		storage:         storage,
		messenger:       messenger,
		repository:      repository,
		authClient:      authClient,
		galleryService:  galleryService,
		galleryUploader: galleryUploader,
		config:          config,
		logger:          slog.Default().With("component", "gallery-handler"),
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
	// defer os.RemoveAll(outputDir) // Cleanup after done - DISABLED for debugging

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
// Uses shared GalleryService for consistent logic with TranscodeHandler
// ═══════════════════════════════════════════════════════════════════════════════

// ProcessJobWithClassification handles gallery job with classification or manual selection
// Uses shared GalleryService เพื่อให้ logic เหมือนกับ TranscodeHandler
func (h *GalleryHandler) ProcessJobWithClassification(ctx context.Context, job *models.GalleryJob) error {
	h.logger.Info("processing gallery job (shared service)",
		"video_id", job.VideoID,
		"video_code", job.VideoCode,
		"quality", job.VideoQuality,
		"duration", job.Duration,
	)

	// Update gallery_status to 'processing'
	if h.repository != nil {
		if err := h.repository.UpdateGalleryProcessingStarted(ctx, job.VideoID); err != nil {
			h.logger.Warn("failed to update gallery processing started", "error", err)
		}
	}

	// Publish initial progress
	h.publishProgress(ctx, job, 0, "เริ่มสร้าง Gallery...")

	// Use shared gallery service (createDirectories will add videoCode)
	outputDir := filepath.Join(h.config.TempDir, "gallery")

	h.logger.Info("ProcessJobWithClassification",
		"TempDir", h.config.TempDir,
		"outputDir", outputDir,
		"video_code", job.VideoCode,
	)

	h.publishProgress(ctx, job, 10, "กำลังดึงภาพจาก HLS...")

	// Generate gallery using shared service
	result, err := h.galleryService.GenerateFromHLS(ctx,
		job.HLSPath,
		job.VideoCode,
		job.Duration,
		outputDir,
		h.storage, // StoragePort for presigned URLs
	)
	if err != nil {
		h.publishFailed(ctx, job, err.Error())
		return fmt.Errorf("generate gallery: %w", err)
	}

	if result == nil {
		h.logger.Info("gallery skipped (video too short)",
			"video_id", job.VideoID,
			"video_code", job.VideoCode,
			"duration", job.Duration,
		)
		h.publishCompleted(ctx, job)
		return nil
	}

	// TEST_MODE: Skip upload and DB update, keep files locally
	if h.config.TestMode {
		h.logger.Info("========================================")
		h.logger.Info("TEST MODE - Skipping upload & DB update")
		h.logger.Info("========================================")
		h.logger.Info("test mode results",
			"video_code", job.VideoCode,
			"source_dir", result.SourceDir,
			"source_count", result.SourceCount,
			"is_manual_selection", result.IsManualSelection,
			"total_frames", result.TotalFrames,
		)
		h.logger.Info("Files kept at", "base_dir", result.BaseDir)
		h.logger.Info("TEST MODE COMPLETE - Check files manually")
		h.publishCompleted(ctx, job)
		return nil
	}

	h.publishProgress(ctx, job, 85, "กำลังอัพโหลดภาพ...")

	// Manual Selection Flow: Upload to source/ only
	if result.IsManualSelection {
		return h.handleManualSelectionUpload(ctx, job, result)
	}

	// Legacy: Three-tier classification flow
	return h.handleThreeTierUpload(ctx, job, result)
}

// handleManualSelectionUpload uploads source/ และ update DB สำหรับ Manual Selection Flow
func (h *GalleryHandler) handleManualSelectionUpload(ctx context.Context, job *models.GalleryJob, result *gallery.Result) error {
	// Upload source/ only
	uploadResult, err := h.galleryUploader.UploadManualSelection(ctx, result, job.OutputPath)
	if err != nil {
		h.logger.Warn("failed to upload gallery", "error", err)
	}

	h.logger.Info("manual selection gallery uploaded",
		"video_code", job.VideoCode,
		"source_uploaded", uploadResult.SuperSafeUploaded, // source count stored in SuperSafeUploaded
	)

	h.publishProgress(ctx, job, 95, "กำลังบันทึกข้อมูล...")

	// Update database with manual selection flow fields
	if err := h.updateVideoGalleryManualSelection(ctx, job.VideoID, job.OutputPath, result.SourceCount); err != nil {
		h.logger.Warn("failed to update gallery in DB",
			"video_id", job.VideoID,
			"error", err,
		)
	}

	// Cleanup
	h.galleryService.Cleanup(result)

	// Publish completed
	h.publishCompleted(ctx, job)

	h.logger.Info("manual selection gallery job completed",
		"video_id", job.VideoID,
		"video_code", job.VideoCode,
		"source_count", result.SourceCount,
		"total_frames", result.TotalFrames,
	)

	return nil
}

// handleThreeTierUpload handles legacy three-tier classification upload
func (h *GalleryHandler) handleThreeTierUpload(ctx context.Context, job *models.GalleryJob, result *gallery.Result) error {
	// Upload using shared uploader
	uploadResult, err := h.galleryUploader.UploadClassified(ctx, result, job.OutputPath)
	if err != nil {
		h.logger.Warn("failed to upload gallery", "error", err)
	}

	h.logger.Info("three-tier gallery uploaded",
		"video_code", job.VideoCode,
		"super_safe_uploaded", uploadResult.SuperSafeUploaded,
		"safe_uploaded", uploadResult.SafeUploaded,
		"nsfw_uploaded", uploadResult.NsfwUploaded,
	)

	h.publishProgress(ctx, job, 95, "กำลังบันทึกข้อมูล...")

	// Update database
	if err := h.updateVideoGalleryClassifiedThreeTier(ctx, job.VideoID, job.OutputPath,
		uploadResult.SuperSafeUploaded, uploadResult.SafeUploaded, uploadResult.NsfwUploaded); err != nil {
		h.logger.Warn("failed to update classified gallery in DB",
			"video_id", job.VideoID,
			"error", err,
		)
	}

	// Log classification stats
	h.logger.Info("classification_stats",
		"video_code", job.VideoCode,
		"total_frames", result.TotalFrames,
		"super_safe_count", result.SuperSafeCount,
		"safe_count", result.SafeCount,
		"nsfw_count", result.NsfwCount,
		"rounds_used", result.RoundsUsed,
	)

	// Cleanup using shared service
	h.galleryService.Cleanup(result)

	// Publish completed
	h.publishCompleted(ctx, job)

	h.logger.Info("classified gallery job completed (three-tier)",
		"video_id", job.VideoID,
		"video_code", job.VideoCode,
		"super_safe_images", uploadResult.SuperSafeUploaded,
		"safe_images", uploadResult.SafeUploaded,
		"nsfw_images", uploadResult.NsfwUploaded,
	)

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Legacy ProcessJobWithClassification (inline logic) - DEPRECATED
// Kept for reference, will be removed in future version
// ═══════════════════════════════════════════════════════════════════════════════

// ProcessJobWithClassificationLegacy handles gallery job with inline classification logic
// DEPRECATED: Use ProcessJobWithClassification instead
func (h *GalleryHandler) ProcessJobWithClassificationLegacy(ctx context.Context, job *models.GalleryJob) error {
	h.logger.Info("processing gallery job with classification (legacy)",
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
	// defer os.RemoveAll(baseDir) // Cleanup after done - DISABLED for debugging

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
	// Verbose mode เปิดตลอดเพื่อ debug ปัญหา super_safe images
	classifierConfig := classifier.ClassifierConfig{
		PythonPath:         "python",
		ScriptPath:         "infrastructure/classifier/classify_batch.py",
		NsfwThreshold:      0.3,
		SuperSafeThreshold: 0.15,
		MinFaceScore:       0.1,
		Timeout:            300, // 5 minutes for POV + Mosaic detection
		MaxNsfwImages:      20,  // จำกัด NSFW 20 ภาพ
		MaxSafeImages:      10,  // จำกัด Safe 10 ภาพ
		MinSafeImages:      12,
		MinSuperSafeImages: 10,
		Verbose:            true, // Enable detailed per-image logging
		SkipMosaic:         true, // Skip slow mosaic detection (temporarily)
		SkipPOV:            true, // Skip slow POV detection (temporarily)
	}
	nsfwClassifier := classifier.NewNSFWClassifier(classifierConfig, h.logger)

	// 4. Two-Phase Extraction:
	// Phase 1 (นาทีที่ 1-10): หา super_safe + safe
	// Phase 2 (นาทีที่ 11-20): หา nsfw
	var allSuperSafeResults []classifier.ClassificationResult
	var allSafeResults []classifier.ClassificationResult
	var allNsfwResults []classifier.ClassificationResult
	totalFrames := 0

	videoDurationMin := job.Duration / 60 // Duration in minutes
	framesPerMinute := 10

	h.logger.Info("starting two-phase extraction",
		"video_duration_min", videoDurationMin,
		"frames_per_minute", framesPerMinute,
	)

	timestampTracker := make(map[int]bool)

	// ═══════════════════════════════════════════════════════════════
	// Phase 1: นาทีที่ 1-10 → หา super_safe + safe
	// ═══════════════════════════════════════════════════════════════
	phase1Start := 0
	phase1End := 10
	if phase1End > videoDurationMin {
		phase1End = videoDurationMin
	}

	h.publishProgress(ctx, job, 20, fmt.Sprintf("Phase 1: นาทีที่ %d-%d (หา super_safe)...", phase1Start+1, phase1End))
	h.logger.Info("phase 1: extracting super_safe candidates",
		"start_minute", phase1Start+1,
		"end_minute", phase1End,
	)

	frameCount1 := h.extractTimeBasedFrames(
		ctx, job, segments, allFramesDir,
		phase1Start, phase1End, framesPerMinute,
		timestampTracker, totalFrames,
	)

	if frameCount1 > 0 {
		totalFrames += frameCount1

		result1, err := nsfwClassifier.ClassifyBatch(ctx, allFramesDir)
		if err != nil {
			h.logger.Warn("phase 1 classification failed", "error", err)
		} else {
			h.logger.Info("phase 1 classification complete",
				"total_images", result1.Stats.TotalImages,
				"super_safe", result1.Stats.SuperSafeCount,
				"safe", result1.Stats.SafeCount,
				"nsfw", result1.Stats.NsfwCount,
			)

			separated1 := nsfwClassifier.SeparateResults(result1.Results)
			h.moveClassifiedFilesThreeTier(allFramesDir, superSafeDir, safeDir, nsfwDir, separated1)

			// Phase 1: เก็บ super_safe และ safe เท่านั้น (ไม่เก็บ nsfw จาก phase นี้)
			allSuperSafeResults = append(allSuperSafeResults, separated1.SuperSafe...)
			allSafeResults = append(allSafeResults, separated1.Safe...)
			// ลบ nsfw จาก phase 1 ออก (เพราะอาจไม่ใช่ nsfw จริง)
			for _, r := range separated1.Nsfw {
				os.Remove(filepath.Join(nsfwDir, r.Filename))
			}

			h.logger.Info("phase 1 complete",
				"super_safe_found", len(separated1.SuperSafe),
				"safe_found", len(separated1.Safe),
				"nsfw_discarded", len(separated1.Nsfw),
			)
		}
	}

	// ═══════════════════════════════════════════════════════════════
	// Phase 2: นาทีที่ 11-30 → หา nsfw (20 นาที = 200 frames, เลือก 20 ภาพ)
	// ═══════════════════════════════════════════════════════════════
	phase2Start := 10
	phase2End := 30
	if phase2Start >= videoDurationMin {
		h.logger.Warn("video too short for phase 2, skipping nsfw extraction",
			"video_duration_min", videoDurationMin,
			"phase2_start_min", phase2Start,
		)
	} else {
		if phase2End > videoDurationMin {
			phase2End = videoDurationMin
		}

		h.publishProgress(ctx, job, 50, fmt.Sprintf("Phase 2: นาทีที่ %d-%d (หา nsfw)...", phase2Start+1, phase2End))
		h.logger.Info("phase 2: extracting nsfw candidates",
			"start_minute", phase2Start+1,
			"end_minute", phase2End,
		)

		frameCount2 := h.extractTimeBasedFrames(
			ctx, job, segments, allFramesDir,
			phase2Start, phase2End, framesPerMinute,
			timestampTracker, totalFrames,
		)

		if frameCount2 > 0 {
			totalFrames += frameCount2

			result2, err := nsfwClassifier.ClassifyBatch(ctx, allFramesDir)
			if err != nil {
				h.logger.Warn("phase 2 classification failed", "error", err)
			} else {
				h.logger.Info("phase 2 classification complete",
					"total_images", result2.Stats.TotalImages,
					"super_safe", result2.Stats.SuperSafeCount,
					"safe", result2.Stats.SafeCount,
					"nsfw", result2.Stats.NsfwCount,
				)

				separated2 := nsfwClassifier.SeparateResults(result2.Results)
				h.moveClassifiedFilesThreeTier(allFramesDir, superSafeDir, safeDir, nsfwDir, separated2)

				// Phase 2: เก็บ nsfw เท่านั้น (super_safe และ safe จาก phase นี้ไม่ค่อยน่าสนใจ)
				allNsfwResults = append(allNsfwResults, separated2.Nsfw...)
				// ลบ super_safe และ safe จาก phase 2 ออก
				for _, r := range separated2.SuperSafe {
					os.Remove(filepath.Join(superSafeDir, r.Filename))
				}
				for _, r := range separated2.Safe {
					os.Remove(filepath.Join(safeDir, r.Filename))
				}

				h.logger.Info("phase 2 complete",
					"nsfw_found", len(separated2.Nsfw),
					"super_safe_discarded", len(separated2.SuperSafe),
					"safe_discarded", len(separated2.Safe),
				)
			}
		}
	}

	// 5. Limit NSFW and Safe images to top 10 by quality
	nsfwClassifier.SortByQuality(allNsfwResults)
	if len(allNsfwResults) > classifierConfig.MaxNsfwImages {
		// Delete excess NSFW files
		for i := classifierConfig.MaxNsfwImages; i < len(allNsfwResults); i++ {
			os.Remove(filepath.Join(nsfwDir, allNsfwResults[i].Filename))
		}
		allNsfwResults = allNsfwResults[:classifierConfig.MaxNsfwImages]
	}

	// Limit Safe images to top 10 by quality
	nsfwClassifier.SortByQuality(allSafeResults)
	if len(allSafeResults) > classifierConfig.MaxSafeImages {
		// Delete excess Safe files
		for i := classifierConfig.MaxSafeImages; i < len(allSafeResults); i++ {
			os.Remove(filepath.Join(safeDir, allSafeResults[i].Filename))
		}
		allSafeResults = allSafeResults[:classifierConfig.MaxSafeImages]
	}

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

	// 8. Log classification stats (Two-Phase)
	h.logger.Info("classification_stats",
		"video_code", job.VideoCode,
		"total_frames", totalFrames,
		"super_safe_count", len(allSuperSafeResults),
		"safe_count", len(allSafeResults),
		"nsfw_count", len(allNsfwResults),
		"phases_used", 2,
		"super_safe_target_met", len(allSuperSafeResults) >= classifierConfig.MinSuperSafeImages,
	)

	// Log all super_safe images with their scores (for debugging NSFW leakage)
	for _, img := range allSuperSafeResults {
		h.logger.Info("super_safe_image",
			"video_code", job.VideoCode,
			"filename", img.Filename,
			"nsfw_score", img.NsfwScore,
			"falconsai_score", img.FalconsaiScore,
			"nudenet_score", img.NudenetScore,
			"face_score", img.FaceScore,
			"reason", img.Reason,
		)
	}

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

// extractTimeBasedFrames extracts frames based on time: 10 frames per minute
// startMinute and endMinute are 0-indexed (0 = first minute)
func (h *GalleryHandler) extractTimeBasedFrames(
	ctx context.Context,
	job *models.GalleryJob,
	segments []hlsSegment,
	outputDir string,
	startMinute, endMinute int,
	framesPerMinute int,
	timestampTracker map[int]bool,
	filenameOffset int,
) int {
	extracted := 0
	secondsPerFrame := 60 / framesPerMinute // 6 seconds per frame for 10 frames/minute

	for minute := startMinute; minute < endMinute; minute++ {
		for frameInMinute := 0; frameInMinute < framesPerMinute; frameInMinute++ {
			select {
			case <-ctx.Done():
				return extracted
			default:
			}

			// Calculate timestamp: minute * 60 + frame offset
			timestamp := float64(minute*60 + frameInMinute*secondsPerFrame)
			sec := int(timestamp)

			// Skip if this second was already used
			if timestampTracker[sec] {
				continue
			}

			// Check if timestamp is within video duration
			if sec >= job.Duration {
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
	}

	h.logger.Info("time-based extraction complete",
		"start_minute", startMinute+1,
		"end_minute", endMinute,
		"frames_extracted", extracted,
	)

	return extracted
}

// updateVideoGalleryManualSelection updates video for Manual Selection Flow via API
// Sets gallery_status = "pending_review" และ gallery_source_count
func (h *GalleryHandler) updateVideoGalleryManualSelection(ctx context.Context, videoID, galleryPath string, sourceCount int) error {
	h.logger.Info("updateVideoGalleryManualSelection called",
		"video_id", videoID,
		"gallery_path", galleryPath,
		"source_count", sourceCount,
		"api_url", h.config.APIURL,
	)

	if h.config.APIURL == "" {
		h.logger.Warn("skipping gallery DB update: APIURL is empty")
		return nil
	}
	if h.authClient == nil {
		h.logger.Warn("skipping gallery DB update: authClient is nil")
		return nil
	}
	if !h.authClient.IsConfigured() {
		h.logger.Warn("skipping gallery DB update: authClient not configured")
		return nil
	}

	url := fmt.Sprintf("%s/api/v1/internal/videos/%s/gallery", h.config.APIURL, videoID)

	payload := map[string]interface{}{
		"gallery_path":         galleryPath,
		"gallery_status":       "pending_review", // รอ Admin เลือกภาพ
		"gallery_source_count": sourceCount,      // จำนวนภาพใน source/
		"gallery_safe_count":   0,                // ยังไม่มี Admin เลือก
		"gallery_nsfw_count":   0,                // ยังไม่มี Admin เลือก
		"gallery_count":        0,                // Total = safe + nsfw (ยังไม่มี)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	h.logger.Info("calling API to update gallery (manual selection)",
		"url", url,
		"payload", string(data),
	)

	resp, err := h.authClient.DoRequestWithAuth(ctx, "PATCH", url, data)
	if err != nil {
		h.logger.Error("API call failed", "error", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		h.logger.Error("API returned error status", "status_code", resp.StatusCode)
		return fmt.Errorf("API returned %d", resp.StatusCode)
	}

	h.logger.Info("gallery DB updated successfully (manual selection)",
		"video_id", videoID,
		"status", "pending_review",
		"source_count", sourceCount,
	)
	return nil
}

// updateVideoGalleryClassified updates video with safe/nsfw counts via API (deprecated, use Three-Tier)
func (h *GalleryHandler) updateVideoGalleryClassified(ctx context.Context, videoID, galleryPath string, safeCount, nsfwCount int) error {
	if h.config.APIURL == "" || h.authClient == nil || !h.authClient.IsConfigured() {
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

	resp, err := h.authClient.DoRequestWithAuth(ctx, "PATCH", url, data)
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
	h.logger.Info("updateVideoGalleryClassifiedThreeTier called",
		"video_id", videoID,
		"gallery_path", galleryPath,
		"super_safe_count", superSafeCount,
		"safe_count", safeCount,
		"nsfw_count", nsfwCount,
		"api_url", h.config.APIURL,
		"auth_client_nil", h.authClient == nil,
		"auth_configured", h.authClient != nil && h.authClient.IsConfigured(),
	)

	if h.config.APIURL == "" {
		h.logger.Warn("skipping gallery DB update: APIURL is empty")
		return nil
	}
	if h.authClient == nil {
		h.logger.Warn("skipping gallery DB update: authClient is nil")
		return nil
	}
	if !h.authClient.IsConfigured() {
		h.logger.Warn("skipping gallery DB update: authClient not configured")
		return nil
	}

	url := fmt.Sprintf("%s/api/v1/internal/videos/%s/gallery", h.config.APIURL, videoID)

	// gallery_count = super_safe + safe (total public-accessible images)
	totalPublicCount := superSafeCount + safeCount

	payload := map[string]interface{}{
		"gallery_path":             galleryPath,
		"gallery_count":            totalPublicCount, // Total safe images (backward compatible)
		"gallery_super_safe_count": superSafeCount,   // super_safe (< 0.15 + face) for Public SEO
		"gallery_safe_count":       safeCount,        // borderline (0.15-0.3)
		"gallery_nsfw_count":       nsfwCount,        // nsfw (>= 0.3)
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	h.logger.Info("calling API to update gallery",
		"url", url,
		"payload", string(data),
	)

	resp, err := h.authClient.DoRequestWithAuth(ctx, "PATCH", url, data)
	if err != nil {
		h.logger.Error("API call failed", "error", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		h.logger.Error("API returned error status", "status_code", resp.StatusCode)
		return fmt.Errorf("API returned %d", resp.StatusCode)
	}

	h.logger.Info("gallery DB updated successfully via API",
		"video_id", videoID,
		"status_code", resp.StatusCode,
	)
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
	if h.config.APIURL == "" || h.authClient == nil || !h.authClient.IsConfigured() {
		return nil // Skip if API URL or auth not configured
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

	resp, err := h.authClient.DoRequestWithAuth(ctx, "PATCH", url, data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API returned %d", resp.StatusCode)
	}

	return nil
}
