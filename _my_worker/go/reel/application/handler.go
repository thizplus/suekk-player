package application

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/suekk/my_worker/go/reel/config"
	"github.com/suekk/my_worker/go/reel/domain"
	"github.com/suekk/my_worker/go/reel/domain/ports"
	"github.com/suekk/my_worker/go/reel/infrastructure/progress"
)

// Handler handles reel jobs
type Handler struct {
	config    *config.Config
	storage   ports.StoragePort
	processor ports.ProcessorPort
	progress  ports.ProgressPort
	throttler *progress.Throttler
	logger    *slog.Logger
}

// allowedInputPrefixes defines valid path prefixes for input validation
var allowedInputPrefixes = []string{"hls/", "videos/"}

// allowedOutputPrefixes defines valid path prefixes for output validation
var allowedOutputPrefixes = []string{"reels/"}

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
	processor ports.ProcessorPort,
	progressPort ports.ProgressPort,
) *Handler {
	return &Handler{
		config:    cfg,
		storage:   storage,
		processor: processor,
		progress:  progressPort,
		throttler: progress.NewThrottler(progress.DefaultConfig()),
		logger:    slog.Default().With("component", "reel-handler"),
	}
}

// HandleJob processes a reel job with full production logic
func (h *Handler) HandleJob(ctx context.Context, job *domain.ReelJob, startedAt time.Time) error {
	startTime := time.Now()
	meta := job.Meta
	input := job.Input

	h.logger.Info("Starting reel job",
		"job_id", meta.JobID,
		"entity_code", meta.EntityCode,
		"style", input.Style,
		"segments", len(input.GetSegments()),
		"add_tts", input.AddTTS,
		"started_at", startedAt,
	)

	// ═══════════════════════════════════════════════════════════════════════════════
	// 0. Input path validation - Security check
	// ═══════════════════════════════════════════════════════════════════════════════
	if err := validatePath(input.VideoPath, allowedInputPrefixes); err != nil {
		return h.handleError(ctx, &meta, startedAt, domain.StageInitializing, err, domain.ErrCodeInputValidation)
	}
	if err := validatePath(input.OutputPath, allowedOutputPrefixes); err != nil {
		return h.handleError(ctx, &meta, startedAt, domain.StageInitializing, err, domain.ErrCodeInputValidation)
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// 1. Validate Input
	// ═══════════════════════════════════════════════════════════════════════════════

	// Validate segments
	if err := input.ValidateSegments(); err != nil {
		return h.handleError(ctx, &meta, startedAt, domain.StageInitializing, err, domain.ErrCodeInputValidation)
	}

	segments := input.GetSegments()
	if len(segments) > h.config.MaxSegments {
		return h.handleError(ctx, &meta, startedAt, domain.StageInitializing,
			fmt.Errorf("too many segments: %d (max: %d)", len(segments), h.config.MaxSegments),
			domain.ErrCodeInputValidation)
	}

	totalDuration := input.GetTotalDuration()
	if totalDuration > h.config.MaxDuration {
		return h.handleError(ctx, &meta, startedAt, domain.StageInitializing,
			fmt.Errorf("total duration too long: %.1fs (max: %.1fs)", totalDuration, h.config.MaxDuration),
			domain.ErrCodeSegmentsTooLong)
	}

	// Create job temp directory
	jobDir := filepath.Join(h.config.TempDir, meta.JobID.String())
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		return h.handleError(ctx, &meta, startedAt, domain.StageInitializing, err, domain.ErrCodeProcessFailed)
	}
	defer h.cleanup(jobDir)

	h.publishProgress(ctx, &meta, startedAt, domain.StageInitializing, 0, "เริ่มสร้าง Reel")

	// ═══════════════════════════════════════════════════════════════════════════════
	// 2. Download Source Video
	// ═══════════════════════════════════════════════════════════════════════════════

	h.publishProgress(ctx, &meta, startedAt, domain.StageDownloading, 5, "กำลังดาวน์โหลด...")

	localInput := filepath.Join(jobDir, "input"+filepath.Ext(input.VideoPath))
	if err := h.storage.Download(ctx, input.VideoPath, localInput, func(downloaded, total int64) {
		if total > 0 {
			pct := float64(downloaded) / float64(total) * 100
			h.publishProgress(ctx, &meta, startedAt, domain.StageDownloading, 5+pct*0.1, fmt.Sprintf("ดาวน์โหลด %.0f%%", pct))
		}
	}); err != nil {
		return h.handleError(ctx, &meta, startedAt, domain.StageDownloading, err, domain.ErrCodeDownloadFailed)
	}

	h.logger.Info("Video downloaded", "path", localInput)

	// ═══════════════════════════════════════════════════════════════════════════════
	// 3. Process Segments (Cut + Style Transform)
	// ═══════════════════════════════════════════════════════════════════════════════

	h.publishProgress(ctx, &meta, startedAt, domain.StageComposing, 15, "กำลังตัดต่อ...")

	style := input.Style
	if style == "" {
		style = domain.StyleLetterbox
	}

	cropX := input.CropX
	if cropX <= 0 {
		cropX = 50 // Center
	}
	cropY := input.CropY
	if cropY <= 0 {
		cropY = 50 // Center
	}

	processedVideo := filepath.Join(jobDir, "processed.mp4")
	if err := h.processor.ProcessSegments(ctx, localInput, processedVideo, segments, style, cropX, cropY,
		func(pct float64, msg string) {
			h.publishProgress(ctx, &meta, startedAt, domain.StageComposing, 15+pct*0.35, msg)
		}); err != nil {
		return h.handleError(ctx, &meta, startedAt, domain.StageComposing, err, domain.ErrCodeProcessFailed)
	}

	currentVideo := processedVideo
	h.logger.Info("Segments processed", "output", processedVideo)

	// ═══════════════════════════════════════════════════════════════════════════════
	// 4. Add Text Overlays (if any)
	// ═══════════════════════════════════════════════════════════════════════════════

	if input.Title != "" || input.Line1 != "" || input.Line2 != "" {
		h.publishProgress(ctx, &meta, startedAt, domain.StageOverlays, 50, "กำลังเพิ่มข้อความ...")

		overlayVideo := filepath.Join(jobDir, "overlay.mp4")
		if err := h.processor.AddTextOverlay(ctx, currentVideo, overlayVideo, input.Title, input.Line1, input.Line2, nil); err != nil {
			h.logger.Warn("Text overlay failed, continuing without", "error", err)
		} else {
			currentVideo = overlayVideo
			h.logger.Info("Text overlay added")
		}
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// 5. Add Logo Overlay (if enabled)
	// ═══════════════════════════════════════════════════════════════════════════════

	if input.ShowLogo && h.config.LogoPath != "" {
		h.publishProgress(ctx, &meta, startedAt, domain.StageOverlays, 55, "กำลังเพิ่มโลโก้...")

		logoVideo := filepath.Join(jobDir, "logo.mp4")
		if err := h.processor.AddLogoOverlay(ctx, currentVideo, logoVideo, h.config.LogoPath, nil); err != nil {
			h.logger.Warn("Logo overlay failed, continuing without", "error", err)
		} else {
			currentVideo = logoVideo
			h.logger.Info("Logo overlay added")
		}
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// 6. TTS (if enabled)
	// ═══════════════════════════════════════════════════════════════════════════════

	if input.AddTTS && input.TTSText != "" {
		h.publishProgress(ctx, &meta, startedAt, domain.StageGeneratingTTS, 60, "กำลังสร้างเสียงพากย์...")

		ttsAudio := filepath.Join(jobDir, "tts.mp3")
		if err := h.processor.GenerateTTS(ctx, input.TTSText, input.TTSVoice, ttsAudio); err != nil {
			h.logger.Warn("TTS generation failed, continuing without", "error", err)
		} else {
			// Merge audio with video
			h.publishProgress(ctx, &meta, startedAt, domain.StageMergingAudio, 70, "กำลังรวมเสียง...")

			mergedVideo := filepath.Join(jobDir, "merged.mp4")
			if err := h.processor.MergeAudioVideo(ctx, currentVideo, ttsAudio, mergedVideo); err != nil {
				h.logger.Warn("Audio merge failed, using video without TTS", "error", err)
			} else {
				currentVideo = mergedVideo
				h.logger.Info("TTS audio merged")
			}
		}
	}

	finalVideo := currentVideo

	// ═══════════════════════════════════════════════════════════════════════════════
	// 7. Generate Thumbnail
	// ═══════════════════════════════════════════════════════════════════════════════

	h.publishProgress(ctx, &meta, startedAt, domain.StageGeneratingThumbnail, 80, "กำลังสร้าง thumbnail...")

	thumbnailPath := filepath.Join(jobDir, "thumb.jpg")
	coverTime := input.CoverTime
	if err := h.processor.GenerateThumbnail(ctx, finalVideo, thumbnailPath, coverTime); err != nil {
		h.logger.Warn("Thumbnail generation failed", "error", err)
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// 8. Upload to S3
	// ═══════════════════════════════════════════════════════════════════════════════

	h.publishProgress(ctx, &meta, startedAt, domain.StageUploading, 85, "กำลังอัพโหลด...")

	remotePath := strings.TrimSuffix(input.OutputPath, "/")
	remoteVideo := fmt.Sprintf("%s/reel.mp4", remotePath)
	remoteThumbnail := fmt.Sprintf("%s/thumb.jpg", remotePath)

	if err := h.storage.Upload(ctx, remoteVideo, finalVideo); err != nil {
		return h.handleError(ctx, &meta, startedAt, domain.StageUploading, err, domain.ErrCodeUploadFailed)
	}

	h.publishProgress(ctx, &meta, startedAt, domain.StageUploading, 95, "อัพโหลดวิดีโอสำเร็จ")

	// Upload thumbnail (best effort)
	if _, err := os.Stat(thumbnailPath); err == nil {
		if err := h.storage.Upload(ctx, remoteThumbnail, thumbnailPath); err != nil {
			h.logger.Warn("Thumbnail upload failed", "error", err)
		}
	}

	// ═══════════════════════════════════════════════════════════════════════════════
	// 9. Get Output Info & Complete
	// ═══════════════════════════════════════════════════════════════════════════════

	duration, _ := h.processor.GetVideoDuration(finalVideo)
	fileInfo, _ := os.Stat(finalVideo)

	var fileSize int64
	if fileInfo != nil {
		fileSize = fileInfo.Size()
	}

	// Get output dimensions
	outWidth, outHeight := style.GetOutputDimensions()

	output := &domain.ReelOutput{
		VideoPath:     remoteVideo,
		ThumbnailPath: remoteThumbnail,
		Duration:      duration,
		FileSize:      fileSize,
		Width:         outWidth,
		Height:        outHeight,
	}

	durationSec := time.Since(startTime).Seconds()
	h.publishCompleted(ctx, &meta, startedAt, durationSec, output)

	h.logger.Info("Reel job completed",
		"job_id", meta.JobID,
		"duration_sec", durationSec,
		"output_path", remoteVideo,
		"file_size", fileSize,
	)

	return nil
}

func (h *Handler) publishProgress(ctx context.Context, meta *domain.JobMeta, startedAt time.Time, stage string, pct float64, message string) {
	// Throttle check
	if !h.throttler.ShouldSend(meta.JobID.String(), pct, "processing") {
		return
	}

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
		UpdatedAt:  time.Now(),
	}
	if err := h.progress.Publish(ctx, update); err != nil {
		h.logger.Warn("Failed to publish progress", "error", err)
	}
}

func (h *Handler) publishCompleted(ctx context.Context, meta *domain.JobMeta, startedAt time.Time, durationSec float64, output *domain.ReelOutput) {
	h.throttler.Cleanup(meta.JobID.String())
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
		Message:     "Reel สร้างสำเร็จ",
		StartedAt:   startedAt,
		UpdatedAt:   time.Now(),
		DurationSec: &durationSec,
		Output:      output,
	}
	if err := h.progress.Publish(ctx, update); err != nil {
		h.logger.Warn("Failed to publish completed", "error", err)
	}
}

func (h *Handler) handleError(ctx context.Context, meta *domain.JobMeta, startedAt time.Time, stage string, err error, errorCode string) error {
	h.logger.Error("Job failed", "job_id", meta.JobID, "stage", stage, "error", err)

	h.throttler.Cleanup(meta.JobID.String())

	if errorCode == "" {
		errorCode = h.getErrorCode(stage)
	}

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
		Message:    "Reel ล้มเหลว",
		Error:      err.Error(),
		StartedAt:  startedAt,
		UpdatedAt:  time.Now(),
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

func (h *Handler) getErrorCode(stage string) string {
	switch stage {
	case domain.StageInitializing:
		return domain.ErrCodeInputValidation
	case domain.StageDownloading:
		return domain.ErrCodeDownloadFailed
	case domain.StageComposing:
		return domain.ErrCodeProcessFailed
	case domain.StageOverlays:
		return domain.ErrCodeOverlayFailed
	case domain.StageGeneratingTTS:
		return domain.ErrCodeTTSFailed
	case domain.StageMergingAudio:
		return domain.ErrCodeProcessFailed
	case domain.StageGeneratingThumbnail:
		return domain.ErrCodeProcessFailed
	case domain.StageUploading:
		return domain.ErrCodeUploadFailed
	default:
		return domain.ErrCodeProcessFailed
	}
}

func (h *Handler) isRetryable(stage string, err error) bool {
	errStr := strings.ToLower(err.Error())

	// Not found errors are permanent
	if strings.Contains(errStr, "not found") || strings.Contains(errStr, "no such key") {
		return false
	}

	// Validation errors are permanent
	if strings.Contains(errStr, "too long") || strings.Contains(errStr, "too many") {
		return false
	}

	// TTS errors might be temporary (rate limits, etc.)
	if stage == domain.StageTTS {
		return true
	}

	return true
}

func (h *Handler) cleanup(jobDir string) {
	if h.config.KeepLocalFiles {
		h.logger.Debug("Keeping local files", "path", jobDir)
		return
	}

	if err := os.RemoveAll(jobDir); err != nil {
		h.logger.Warn("Failed to cleanup", "path", jobDir, "error", err)
	}
}
