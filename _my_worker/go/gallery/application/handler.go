package application

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/suekk/my_worker/go/gallery/config"
	"github.com/suekk/my_worker/go/gallery/domain"
	"github.com/suekk/my_worker/go/gallery/domain/ports"
	"github.com/suekk/my_worker/go/gallery/infrastructure/progress"
)

// Handler handles gallery jobs
type Handler struct {
	config     *config.Config
	storage    ports.StoragePort
	extractor  ports.ExtractorPort
	classifier ports.ClassifierPort
	progress   ports.ProgressPort
	throttler  *progress.Throttler
	logger     *slog.Logger
}

// allowedInputPrefixes defines valid path prefixes for input validation
var allowedInputPrefixes = []string{"hls/", "videos/"}

// allowedOutputPrefixes defines valid path prefixes for output validation
var allowedOutputPrefixes = []string{"gallery/"}

// validatePath checks for path traversal and validates allowed prefixes
func validatePath(path string, allowedPrefixes []string) error {
	// Check for path traversal
	if strings.Contains(path, "..") {
		return fmt.Errorf("invalid path: contains '..'")
	}

	// Check allowed prefixes
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return nil
		}
	}
	return fmt.Errorf("invalid path: must start with allowed prefix")
}

// NewHandler creates a new handler
func NewHandler(
	cfg *config.Config,
	storage ports.StoragePort,
	extractor ports.ExtractorPort,
	classifier ports.ClassifierPort,
	progressPort ports.ProgressPort,
) *Handler {
	return &Handler{
		config:     cfg,
		storage:    storage,
		extractor:  extractor,
		classifier: classifier,
		progress:   progressPort,
		throttler:  progress.NewThrottler(progress.DefaultConfig()),
		logger:     slog.Default().With("component", "gallery-handler"),
	}
}

// HandleJob processes a gallery job with Two-Phase extraction
func (h *Handler) HandleJob(ctx context.Context, job *domain.GalleryJob, startedAt time.Time) error {
	startTime := time.Now()
	meta := job.Meta
	input := job.Input

	h.logger.Info("Starting gallery job",
		"job_id", meta.JobID,
		"entity_code", meta.EntityCode,
		"hls_path", input.HLSPath,
		"duration", input.Duration,
		"started_at", startedAt,
	)

	// ═══════════════════════════════════════════════════════════════════════════════
	// 0. Input path validation - Security check
	// ═══════════════════════════════════════════════════════════════════════════════
	if err := validatePath(input.HLSPath, allowedInputPrefixes); err != nil {
		return h.handleError(ctx, &meta, startedAt, domain.StageInitializing, err, domain.ErrCodeInputValidation)
	}
	if err := validatePath(input.OutputPath, allowedOutputPrefixes); err != nil {
		return h.handleError(ctx, &meta, startedAt, domain.StageInitializing, err, domain.ErrCodeInputValidation)
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// 1. Validate video duration
	// ═══════════════════════════════════════════════════════════════════════════════
	if input.Duration < h.config.MinVideoDuration {
		err := fmt.Errorf("video too short: %d sec (min: %d sec)", input.Duration, h.config.MinVideoDuration)
		h.logger.Warn("Video too short for gallery", "duration", input.Duration)
		return h.handleError(ctx, &meta, startedAt, domain.StageInitializing, err, domain.ErrCodeVideoTooShort)
	}

	// Create job temp directory
	jobDir := filepath.Join(h.config.TempDir, meta.JobID.String())
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		return h.handleError(ctx, &meta, startedAt, domain.StageInitializing, err, domain.ErrCodeExtractionFailed)
	}
	defer h.cleanup(jobDir)

	h.publishProgress(ctx, &meta, startedAt, domain.StageInitializing, 0, "เริ่มต้นสร้าง gallery")

	// Get HLS playlist URL
	playlistPath := strings.TrimSuffix(input.HLSPath, "/")
	quality := input.VideoQuality
	if quality == "" {
		quality = "1080p"
	}
	hlsPlaylist := fmt.Sprintf("%s/%s/playlist.m3u8", playlistPath, quality)

	presignedURL, err := h.storage.GetPresignedURL(ctx, hlsPlaylist)
	if err != nil {
		h.logger.Warn("Failed to get presigned URL, trying direct path", "error", err)
		presignedURL = hlsPlaylist
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// 2. Two-Phase Extraction
	// ═══════════════════════════════════════════════════════════════════════════════

	h.publishProgress(ctx, &meta, startedAt, domain.StageDownloading, 5, "กำลังเตรียมวิดีโอ...")

	// Calculate time ranges based on video duration
	durationMinutes := input.Duration / 60
	phase1EndMin := h.config.Phase1EndMinute
	phase2EndMin := h.config.Phase2EndMinute

	// Adjust phases if video is shorter than expected
	if durationMinutes < phase1EndMin {
		phase1EndMin = durationMinutes
	}
	if durationMinutes < phase2EndMin {
		phase2EndMin = durationMinutes
	}

	// Initialize separated images
	separated := &domain.SeparatedImages{
		SuperSafe: []domain.ClassificationResult{},
		Safe:      []domain.ClassificationResult{},
		Nsfw:      []domain.ClassificationResult{},
		Error:     []domain.ClassificationResult{},
	}

	roundsUsed := 0

	// ═══════════════════════════════════════════════════════════════════════════════
	// Phase 1: Extract super_safe + safe (minute 0-10)
	// ═══════════════════════════════════════════════════════════════════════════════

	h.publishProgress(ctx, &meta, startedAt, domain.StageExtractingFrames, 10, "Phase 1: ตัดภาพ (นาที 0-10)...")

	phase1Dir := filepath.Join(jobDir, "phase1")
	phase1StartSec := h.config.Phase1StartMinute * 60
	phase1EndSec := phase1EndMin * 60

	if phase1EndSec > phase1StartSec {
		phase1Files, err := h.extractor.ExtractRange(ctx, presignedURL, phase1Dir,
			phase1StartSec, phase1EndSec, h.config.FramesPerMinute,
			func(current, total int, msg string) {
				pct := 10 + (float64(current)/float64(total))*15
				h.publishProgress(ctx, &meta, startedAt, domain.StageExtractingFrames, pct, fmt.Sprintf("Phase 1: %s", msg))
			})
		if err != nil {
			h.logger.Warn("Phase 1 extraction failed", "error", err)
		} else if len(phase1Files) > 0 {
			roundsUsed++
			h.logger.Info("Phase 1 extraction completed", "count", len(phase1Files))

			// Classify phase 1
			h.publishProgress(ctx, &meta, startedAt, domain.StageClassifying, 25, "Phase 1: จัดประเภทภาพ...")
			results, err := h.classifier.ClassifyBatch(ctx, phase1Files, func(current, total int, msg string) {
				pct := 25 + (float64(current)/float64(total))*15
				h.publishProgress(ctx, &meta, startedAt, domain.StageClassifying, pct, fmt.Sprintf("Phase 1: %s", msg))
			})
			if err != nil {
				h.logger.Warn("Phase 1 classification failed", "error", err)
			} else {
				// Separate into tiers (only keep super_safe + safe from phase 1)
				for _, r := range results {
					r.Filename = filepath.Join(phase1Dir, filepath.Base(r.Filename))
					switch r.Classification {
					case string(domain.ClassificationSuperSafe):
						separated.SuperSafe = append(separated.SuperSafe, r)
					case string(domain.ClassificationSafe):
						separated.Safe = append(separated.Safe, r)
					// NSFW from phase 1 is ignored (we get NSFW from phase 2)
					}
				}
			}
		}
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// Phase 2: Extract NSFW (minute 10-30)
	// ═══════════════════════════════════════════════════════════════════════════════

	h.publishProgress(ctx, &meta, startedAt, domain.StageExtractingFrames, 40, "Phase 2: ตัดภาพ (นาที 10-30)...")

	phase2Dir := filepath.Join(jobDir, "phase2")
	phase2StartSec := h.config.Phase2StartMinute * 60
	phase2EndSec := phase2EndMin * 60

	if phase2EndSec > phase2StartSec && durationMinutes >= h.config.Phase2StartMinute {
		phase2Files, err := h.extractor.ExtractRange(ctx, presignedURL, phase2Dir,
			phase2StartSec, phase2EndSec, h.config.FramesPerMinute,
			func(current, total int, msg string) {
				pct := 40 + (float64(current)/float64(total))*15
				h.publishProgress(ctx, &meta, startedAt, domain.StageExtractingFrames, pct, fmt.Sprintf("Phase 2: %s", msg))
			})
		if err != nil {
			h.logger.Warn("Phase 2 extraction failed", "error", err)
		} else if len(phase2Files) > 0 {
			roundsUsed++
			h.logger.Info("Phase 2 extraction completed", "count", len(phase2Files))

			// Classify phase 2
			h.publishProgress(ctx, &meta, startedAt, domain.StageClassifying, 55, "Phase 2: จัดประเภทภาพ...")
			results, err := h.classifier.ClassifyBatch(ctx, phase2Files, func(current, total int, msg string) {
				pct := 55 + (float64(current)/float64(total))*15
				h.publishProgress(ctx, &meta, startedAt, domain.StageClassifying, pct, fmt.Sprintf("Phase 2: %s", msg))
			})
			if err != nil {
				h.logger.Warn("Phase 2 classification failed", "error", err)
			} else {
				// Only keep NSFW from phase 2
				for _, r := range results {
					r.Filename = filepath.Join(phase2Dir, filepath.Base(r.Filename))
					if r.Classification == string(domain.ClassificationNSFW) {
						separated.Nsfw = append(separated.Nsfw, r)
					}
				}
			}
		}
	}

	h.logger.Info("Classification summary",
		"super_safe", len(separated.SuperSafe),
		"safe", len(separated.Safe),
		"nsfw", len(separated.Nsfw),
		"rounds_used", roundsUsed,
	)

	// Validate we have at least some images
	totalExtracted := len(separated.SuperSafe) + len(separated.Safe) + len(separated.Nsfw)
	if totalExtracted == 0 {
		return h.handleError(ctx, &meta, startedAt, domain.StageClassifying,
			fmt.Errorf("no images extracted from video (both phases returned 0 frames)"),
			domain.ErrCodeExtractionFailed)
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// 3. Apply Limits & Sort by Score
	// ═══════════════════════════════════════════════════════════════════════════════

	h.publishProgress(ctx, &meta, startedAt, domain.StageUploading, 70, "กำลังเลือกภาพที่ดีที่สุด...")

	// Sort by NSFW score ascending (lowest = safest first)
	sortByScore := func(results []domain.ClassificationResult) {
		sort.Slice(results, func(i, j int) bool {
			return results[i].NsfwScore < results[j].NsfwScore
		})
	}

	sortByScore(separated.SuperSafe)
	sortByScore(separated.Safe)
	// For NSFW, sort by score descending (highest NSFW score first)
	sort.Slice(separated.Nsfw, func(i, j int) bool {
		return separated.Nsfw[i].NsfwScore > separated.Nsfw[j].NsfwScore
	})

	// Apply limits
	if len(separated.SuperSafe) > h.config.MaxSuperSafeImages {
		separated.SuperSafe = separated.SuperSafe[:h.config.MaxSuperSafeImages]
	}
	if len(separated.Safe) > h.config.MaxSafeImages {
		separated.Safe = separated.Safe[:h.config.MaxSafeImages]
	}
	if len(separated.Nsfw) > h.config.MaxNsfwImages {
		separated.Nsfw = separated.Nsfw[:h.config.MaxNsfwImages]
	}

	h.logger.Info("After applying limits",
		"super_safe", len(separated.SuperSafe),
		"safe", len(separated.Safe),
		"nsfw", len(separated.Nsfw),
	)

	// ═══════════════════════════════════════════════════════════════════════════════
	// 4. Upload to Three-Tier Folders
	// ═══════════════════════════════════════════════════════════════════════════════

	h.publishProgress(ctx, &meta, startedAt, domain.StageUploading, 75, "กำลังอัพโหลด gallery...")

	remotePath := strings.TrimSuffix(input.OutputPath, "/")
	output := &domain.GalleryOutput{
		GalleryPath:     remotePath,
		Classifications: make(map[string]string),
		RoundsUsed:      roundsUsed,
	}

	// Upload super_safe to gallery/{code}/super_safe/
	superSafeRemote := fmt.Sprintf("%s/super_safe", remotePath)
	for i, r := range separated.SuperSafe {
		remoteFile := fmt.Sprintf("%s/%03d.jpg", superSafeRemote, i+1)
		if err := h.storage.Upload(ctx, remoteFile, r.Filename); err != nil {
			h.logger.Warn("Failed to upload super_safe", "file", r.Filename, "error", err)
			continue
		}
		output.SuperSafeCount++
		output.Classifications[fmt.Sprintf("super_safe/%03d.jpg", i+1)] = string(domain.ClassificationSuperSafe)
	}

	h.publishProgress(ctx, &meta, startedAt, domain.StageUploading, 82, fmt.Sprintf("อัพโหลด super_safe: %d", output.SuperSafeCount))

	// Upload safe to gallery/{code}/safe/
	safeRemote := fmt.Sprintf("%s/safe", remotePath)
	for i, r := range separated.Safe {
		remoteFile := fmt.Sprintf("%s/%03d.jpg", safeRemote, i+1)
		if err := h.storage.Upload(ctx, remoteFile, r.Filename); err != nil {
			h.logger.Warn("Failed to upload safe", "file", r.Filename, "error", err)
			continue
		}
		output.SafeCount++
		output.Classifications[fmt.Sprintf("safe/%03d.jpg", i+1)] = string(domain.ClassificationSafe)
	}

	h.publishProgress(ctx, &meta, startedAt, domain.StageUploading, 89, fmt.Sprintf("อัพโหลด safe: %d", output.SafeCount))

	// Upload nsfw to gallery/{code}/nsfw/
	nsfwRemote := fmt.Sprintf("%s/nsfw", remotePath)
	for i, r := range separated.Nsfw {
		remoteFile := fmt.Sprintf("%s/%03d.jpg", nsfwRemote, i+1)
		if err := h.storage.Upload(ctx, remoteFile, r.Filename); err != nil {
			h.logger.Warn("Failed to upload nsfw", "file", r.Filename, "error", err)
			continue
		}
		output.NSFWCount++
		output.Classifications[fmt.Sprintf("nsfw/%03d.jpg", i+1)] = string(domain.ClassificationNSFW)
	}

	h.publishProgress(ctx, &meta, startedAt, domain.StageUploading, 95, fmt.Sprintf("อัพโหลด nsfw: %d", output.NSFWCount))

	output.TotalImages = output.SuperSafeCount + output.SafeCount + output.NSFWCount

	h.logger.Info("Gallery uploaded",
		"path", remotePath,
		"super_safe", output.SuperSafeCount,
		"safe", output.SafeCount,
		"nsfw", output.NSFWCount,
		"total", output.TotalImages,
	)

	// ═══════════════════════════════════════════════════════════════════════════════
	// 5. Finalize
	// ═══════════════════════════════════════════════════════════════════════════════

	h.publishProgress(ctx, &meta, startedAt, domain.StageFinalizing, 98, "กำลังสรุปผล...")

	durationSec := time.Since(startTime).Seconds()
	h.publishCompleted(ctx, &meta, startedAt, durationSec, output)

	h.logger.Info("Gallery job completed",
		"job_id", meta.JobID,
		"entity_code", meta.EntityCode,
		"total_images", output.TotalImages,
		"rounds_used", output.RoundsUsed,
		"duration_sec", durationSec,
	)

	return nil
}

// publishProgress sends a progress update (with throttling)
func (h *Handler) publishProgress(ctx context.Context, meta *domain.JobMeta, startedAt time.Time, stage string, pct float64, message string) {
	// Throttle check
	if !h.throttler.ShouldSend(meta.JobID.String(), pct, "processing") {
		return
	}

	now := time.Now()
	update := &ports.ProgressUpdate{
		JobID:      meta.JobID,
		JobType:    meta.JobType,
		EntityType: meta.EntityType,
		EntityID:   meta.EntityID,
		EntityCode: meta.EntityCode,
		WorkerID:   h.config.WorkerID,
		Status:     "processing",
		Progress:   pct,
		Stage:      stage,
		Message:    message,
		StartedAt:  startedAt,
		UpdatedAt:  now,
	}

	if err := h.progress.Publish(ctx, update); err != nil {
		h.logger.Warn("Failed to publish progress", "error", err)
	}
}

// publishCompleted sends completion status (always sends, bypasses throttle)
func (h *Handler) publishCompleted(ctx context.Context, meta *domain.JobMeta, startedAt time.Time, durationSec float64, output *domain.GalleryOutput) {
	h.throttler.Cleanup(meta.JobID.String())
	now := time.Now()
	update := &ports.ProgressUpdate{
		JobID:       meta.JobID,
		JobType:     meta.JobType,
		EntityType:  meta.EntityType,
		EntityID:    meta.EntityID,
		EntityCode:  meta.EntityCode,
		WorkerID:    h.config.WorkerID,
		Status:      "completed",
		Progress:    100,
		Stage:       domain.StageCompleted,
		Message:     "Gallery สร้างสำเร็จ",
		StartedAt:   startedAt,
		UpdatedAt:   now,
		DurationSec: &durationSec,
		Output:      output,
	}

	if err := h.progress.Publish(ctx, update); err != nil {
		h.logger.Warn("Failed to publish completed", "error", err)
	}
}

// handleError handles job error with specific error code (always sends, bypasses throttle)
func (h *Handler) handleError(ctx context.Context, meta *domain.JobMeta, startedAt time.Time, stage string, err error, errorCode string) error {
	h.logger.Error("Job failed",
		"job_id", meta.JobID,
		"stage", stage,
		"error", err,
	)

	h.throttler.Cleanup(meta.JobID.String())

	if errorCode == "" {
		errorCode = h.getErrorCode(stage)
	}

	now := time.Now()
	update := &ports.ProgressUpdate{
		JobID:      meta.JobID,
		JobType:    meta.JobType,
		EntityType: meta.EntityType,
		EntityID:   meta.EntityID,
		EntityCode: meta.EntityCode,
		WorkerID:   h.config.WorkerID,
		Status:     "failed",
		Progress:   0,
		Stage:      domain.StageFailed,
		Message:    "Gallery ล้มเหลว",
		Error:      err.Error(),
		StartedAt:  startedAt,
		UpdatedAt:  now,
		ErrorDetails: &ports.ErrorInfo{
			Code:        errorCode,
			Stage:       stage,
			Attempt:     meta.RetryCount + 1,
			IsRetryable: h.isRetryable(stage, err),
		},
	}

	h.progress.Publish(ctx, update)

	return err
}

// cleanup removes temp files
func (h *Handler) cleanup(jobDir string) {
	if h.config.KeepLocalFiles {
		h.logger.Debug("Keeping local files", "path", jobDir)
		return
	}

	if err := os.RemoveAll(jobDir); err != nil {
		h.logger.Warn("Failed to cleanup job directory", "path", jobDir, "error", err)
	} else {
		h.logger.Debug("Cleaned up job directory", "path", jobDir)
	}
}

// getErrorCode returns error code based on stage
func (h *Handler) getErrorCode(stage string) string {
	switch stage {
	case domain.StageInitializing:
		return domain.ErrCodeInputValidation
	case domain.StageDownloading:
		return domain.ErrCodeDownloadFailed
	case domain.StageExtractingFrames:
		return domain.ErrCodeExtractionFailed
	case domain.StageClassifying:
		return domain.ErrCodeClassificationFailed
	case domain.StageUploading:
		return domain.ErrCodeUploadFailed
	default:
		return domain.ErrCodeExtractionFailed
	}
}

// isRetryable checks if error should be retried
func (h *Handler) isRetryable(stage string, err error) bool {
	errStr := strings.ToLower(err.Error())

	// Not found errors are permanent
	if strings.Contains(errStr, "not found") || strings.Contains(errStr, "no such key") {
		return false
	}

	// Video too short is permanent
	if strings.Contains(errStr, "video too short") {
		return false
	}

	// Python errors might be temporary (model loading, etc.)
	if stage == domain.StageClassifying {
		return true
	}

	return true
}
