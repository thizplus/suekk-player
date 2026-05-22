package application

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/suekk/my_worker/go/transcode/config"
	"github.com/suekk/my_worker/go/transcode/domain"
	"github.com/suekk/my_worker/go/transcode/domain/ports"
	"github.com/suekk/my_worker/go/transcode/infrastructure/progress"
	"github.com/suekk/my_worker/go/transcode/infrastructure/uploader"
)

// Handler handles transcode jobs
type Handler struct {
	config       *config.Config
	storage      ports.StoragePort
	transcoder   ports.TranscoderPort
	progress     ports.ProgressPort
	jobPublisher ports.JobPublisherPort
	throttler    *progress.Throttler
	logger       *slog.Logger
}

// allowedInputPrefixes defines valid path prefixes for input validation
var allowedInputPrefixes = []string{"videos/", "hls/"}

// allowedOutputPrefixes defines valid path prefixes for output validation
var allowedOutputPrefixes = []string{"hls/", "audio/", "thumbnails/"}

// validateInputPath checks for path traversal and validates allowed prefixes
func validateInputPath(path string) error {
	// Check for path traversal
	if strings.Contains(path, "..") {
		return fmt.Errorf("invalid path: contains '..'")
	}

	// Check allowed prefixes
	for _, prefix := range allowedInputPrefixes {
		if strings.HasPrefix(path, prefix) {
			return nil
		}
	}
	return fmt.Errorf("invalid path: must start with allowed prefix (videos/, hls/)")
}

// validateOutputPath checks for path traversal and validates allowed prefixes
func validateOutputPath(path string) error {
	// Check for path traversal
	if strings.Contains(path, "..") {
		return fmt.Errorf("invalid path: contains '..'")
	}

	// Check allowed prefixes
	for _, prefix := range allowedOutputPrefixes {
		if strings.HasPrefix(path, prefix) {
			return nil
		}
	}
	return fmt.Errorf("invalid path: must start with allowed prefix (hls/, audio/, thumbnails/)")
}

// NewHandler creates a new handler
func NewHandler(
	cfg *config.Config,
	storage ports.StoragePort,
	transcoder ports.TranscoderPort,
	progressPort ports.ProgressPort,
	jobPublisher ports.JobPublisherPort,
) *Handler {
	return &Handler{
		config:       cfg,
		storage:      storage,
		transcoder:   transcoder,
		progress:     progressPort,
		jobPublisher: jobPublisher,
		throttler:    progress.NewThrottler(progress.DefaultConfig()), // ทุก 2 วิ หรือ 5%
		logger:       slog.Default().With("component", "transcode-handler"),
	}
}

// HandleJob processes a transcode job
func (h *Handler) HandleJob(ctx context.Context, job *domain.TranscodeJob, startedAt time.Time) error {
	startTime := time.Now()
	meta := job.Meta
	input := job.Input

	h.logger.Info("Starting transcode job",
		"job_id", meta.JobID,
		"entity_code", meta.EntityCode,
		"input_path", input.InputPath,
		"qualities", input.Qualities,
	)

	// 0a. Input path validation - Security check
	if err := validateInputPath(input.InputPath); err != nil {
		return h.handleError(ctx, &meta, startedAt, domain.StageInitializing, err)
	}
	if err := validateOutputPath(input.OutputPath); err != nil {
		return h.handleError(ctx, &meta, startedAt, domain.StageInitializing, err)
	}

	// 0. Idempotency Check - ตรวจว่า transcode ไปแล้วหรือยัง
	remotePath := strings.TrimSuffix(input.OutputPath, "/")
	masterPlaylist := remotePath + "/master.m3u8"

	exists, _ := h.storage.Exists(ctx, masterPlaylist)
	if exists {
		h.logger.Info("Transcode already completed (idempotency check)",
			"job_id", meta.JobID,
			"entity_code", meta.EntityCode,
			"master_playlist", masterPlaylist,
		)
		// Skip แต่ไม่ error - publish completed เพื่อให้ downstream jobs ทำงานต่อ
		h.publishCompleted(ctx, &meta, startedAt, 0, &domain.TranscodeOutput{
			HLSPath: masterPlaylist,
		})
		return nil
	}

	// Create job temp directory
	jobDir := filepath.Join(h.config.TempDir, meta.JobID.String())
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		return h.handleError(ctx, &meta, startedAt, domain.StageInitializing, err)
	}
	defer h.cleanup(jobDir, meta.JobID.String())

	// 1. Initialize
	h.publishProgress(ctx, &meta, startedAt, domain.StageInitializing, 0, "เริ่มต้น transcode")

	// Check VRAM
	if !h.transcoder.HasAvailableVRAM() {
		return h.handleError(ctx, &meta, startedAt, domain.StageInitializing, fmt.Errorf("GPU VRAM not available"))
	}

	// 2. Download source file
	h.publishProgress(ctx, &meta, startedAt, domain.StageDownloading, 2, "กำลังดาวน์โหลด...")
	localInputPath := filepath.Join(jobDir, "input"+filepath.Ext(input.InputPath))

	if err := h.storage.Download(ctx, input.InputPath, localInputPath, func(downloaded, total int64) {
		if total > 0 {
			pct := float64(downloaded) / float64(total) * 100
			scaledPct := 2 + (pct * 0.08) // 2-10%
			h.publishProgress(ctx, &meta, startedAt, domain.StageDownloading, scaledPct,
				fmt.Sprintf("ดาวน์โหลด: %.1f%%", pct))
		}
	}); err != nil {
		return h.handleError(ctx, &meta, startedAt, domain.StageDownloading, err)
	}

	h.logger.Info("Source file downloaded", "path", localInputPath)

	// 2b. Get video info for quality filtering
	videoInfo, err := h.transcoder.GetVideoInfo(localInputPath)
	if err != nil {
		h.logger.Warn("Failed to get video info, using default qualities", "error", err)
		videoInfo = &domain.VideoInfo{Height: 1080, Bitrate: 10000000, Duration: 0}
	}

	h.logger.Info("Video info",
		"width", videoInfo.Width,
		"height", videoInfo.Height,
		"duration", videoInfo.Duration,
		"bitrate", videoInfo.Bitrate,
		"fps", videoInfo.FPS,
	)

	// 3. Prepare qualities
	qualities := domain.GetQualities(input.Qualities)

	// Filter qualities by source resolution (ไม่ output 1080p ถ้า source เป็น 720p)
	qualities = domain.FilterQualitiesBySource(qualities, videoInfo.Height)

	// Cap bitrates based on source bitrate
	for i := range qualities {
		domain.CapBitrate(&qualities[i], videoInfo.Bitrate)
	}

	h.logger.Info("Filtered qualities",
		"original", input.Qualities,
		"filtered", qualityNames(qualities),
	)

	outputDir := filepath.Join(jobDir, "output")

	// 4. Prepare HLS settings
	hlsTime := input.HLSTime
	if hlsTime <= 0 {
		hlsTime = h.config.HLSTime
	}

	useByteRange := input.UseByteRange
	h.logger.Info("HLS mode",
		"use_byte_range", useByteRange,
		"hls_time", hlsTime,
	)

	// Variables for upload stats (used later for completed message)
	var uploadedCount int64
	var uploadedBytes int64
	var qualitySizes map[string]int64
	var qualitySegmentCounts map[string]int

	if useByteRange {
		// ═══════════════════════════════════════════════════════
		// Byte-Range Mode: single file per quality
		// ไม่ต้องใช้ SegmentWatcher - upload ทั้งไฟล์หลัง transcode เสร็จ
		// ═══════════════════════════════════════════════════════

		// 5. Transcode to HLS (byte-range)
		h.publishProgress(ctx, &meta, startedAt, domain.StageTranscoding, 12, "กำลังแปลงวิดีโอ (byte-range)...")

		segmentCounts, err := h.transcoder.TranscodeToHLS(ctx, meta.JobID.String(), localInputPath, outputDir, qualities, hlsTime, true,
			func(quality string, pct float64, message string) {
				scaledPct := 12 + (pct * 0.68) // 12-80%
				h.publishProgress(ctx, &meta, startedAt, domain.StageTranscoding, scaledPct, message)
			})
		if err != nil {
			return h.handleError(ctx, &meta, startedAt, domain.StageTranscoding, err)
		}

		qualitySegmentCounts = segmentCounts
		h.logger.Info("Transcode completed (byte-range)", "segment_counts", segmentCounts)

		// 6. Upload all output files (stream.ts + playlist.m3u8 per quality + master.m3u8)
		h.publishProgress(ctx, &meta, startedAt, domain.StageUploadingSegments, 82, "กำลังอัพโหลด...")

		qualitySizes = make(map[string]int64)
		for i, q := range qualities {
			qualityDir := filepath.Join(outputDir, q.Name)

			// Upload stream.ts (single file per quality)
			streamFile := filepath.Join(qualityDir, "stream.ts")
			remoteStream := fmt.Sprintf("%s/%s/stream.ts", remotePath, q.Name)
			if err := h.storage.Upload(ctx, remoteStream, streamFile); err != nil {
				return h.handleError(ctx, &meta, startedAt, domain.StageUploading, fmt.Errorf("failed to upload %s stream.ts: %w", q.Name, err))
			}

			// Track size
			if fi, err := os.Stat(streamFile); err == nil {
				qualitySizes[q.Name] = fi.Size()
				uploadedBytes += fi.Size()
			}
			uploadedCount++

			// Upload playlist.m3u8
			playlistFile := filepath.Join(qualityDir, "playlist.m3u8")
			remotePlaylist := fmt.Sprintf("%s/%s/playlist.m3u8", remotePath, q.Name)
			if err := h.storage.Upload(ctx, remotePlaylist, playlistFile); err != nil {
				return h.handleError(ctx, &meta, startedAt, domain.StageUploading, fmt.Errorf("failed to upload %s playlist: %w", q.Name, err))
			}
			uploadedCount++

			pct := 82 + float64(i+1)/float64(len(qualities))*6 // 82-88%
			h.publishProgress(ctx, &meta, startedAt, domain.StageUploadingSegments, pct,
				fmt.Sprintf("อัพโหลด %s เสร็จ", q.Name))
		}

		// Upload master.m3u8
		h.publishProgress(ctx, &meta, startedAt, domain.StageUploadingPlaylists, 88, "กำลังอัพโหลด master playlist...")
		masterFile := filepath.Join(outputDir, "master.m3u8")
		remoteMaster := remotePath + "/master.m3u8"
		if err := h.storage.Upload(ctx, remoteMaster, masterFile); err != nil {
			return h.handleError(ctx, &meta, startedAt, domain.StageUploading, fmt.Errorf("failed to upload master playlist: %w", err))
		}

		h.logger.Info("Byte-range upload completed",
			"files_uploaded", uploadedCount,
			"bytes_uploaded", uploadedBytes,
			"quality_sizes", qualitySizes,
		)

	} else {
		// ═══════════════════════════════════════════════════════
		// Segment Mode: separate .ts files (เดิม)
		// ใช้ SegmentWatcher สำหรับ parallel upload ขณะ transcode
		// ═══════════════════════════════════════════════════════

		// 4b. Start SegmentWatcher for parallel upload
		watcherConfig := domain.GetAdaptiveWatcherConfig(videoInfo.Duration, len(qualities))
		h.logger.Info("Using adaptive watcher config",
			"video_duration_min", videoInfo.Duration/60,
			"mode", watcherConfig.Mode,
			"workers", watcherConfig.Workers,
		)

		h.publishProgress(ctx, &meta, startedAt, domain.StageTranscoding, 11, "กำลังเตรียม upload watcher...")

		segmentWatcher, err := uploader.NewSegmentWatcher(h.storage, uploader.Config{
			Workers:        watcherConfig.Workers,
			UseTempFile:    true,
			MaxUncommitted: watcherConfig.MaxUncommitted,
			BaseRateLimit:  watcherConfig.BaseRateLimit,
			RenditionCount: len(qualities),
			WindowSize:     watcherConfig.WindowSize,
			FlushInterval:  watcherConfig.FlushInterval,
		})
		if err != nil {
			return h.handleError(ctx, &meta, startedAt, domain.StageTranscoding, err)
		}

		if err := segmentWatcher.Start(outputDir, remotePath); err != nil {
			return h.handleError(ctx, &meta, startedAt, domain.StageTranscoding, err)
		}

		// 5. Transcode to HLS (segment mode)
		h.publishProgress(ctx, &meta, startedAt, domain.StageTranscoding, 12, "กำลังแปลงวิดีโอ...")

		segmentCounts, err := h.transcoder.TranscodeToHLS(ctx, meta.JobID.String(), localInputPath, outputDir, qualities, hlsTime, false,
			func(quality string, pct float64, message string) {
				scaledPct := 12 + (pct * 0.68)
				h.publishProgress(ctx, &meta, startedAt, domain.StageTranscoding, scaledPct, message)
			})
		if err != nil {
			segmentWatcher.Stop()
			return h.handleError(ctx, &meta, startedAt, domain.StageTranscoding, err)
		}

		qualitySegmentCounts = segmentCounts
		h.logger.Info("Transcode completed", "segment_counts", segmentCounts)

		// 6. Wait for remaining segment uploads
		h.publishProgress(ctx, &meta, startedAt, domain.StageUploadingSegments, 82, "กำลังอัพโหลดไฟล์ที่เหลือ...")
		segmentWatcher.FlushNow()

		if err := segmentWatcher.Wait(); err != nil {
			h.logger.Warn("Some segment uploads failed", "error", err)
		}

		// 7. Upload playlists LAST
		h.publishProgress(ctx, &meta, startedAt, domain.StageUploadingPlaylists, 86, "กำลังอัพโหลด playlists...")
		if err := segmentWatcher.UploadPlaylists(ctx, outputDir, remotePath); err != nil {
			return h.handleError(ctx, &meta, startedAt, domain.StageUploading, err)
		}

		// 8. Cleanup local segments
		deletedCount, freedBytes := segmentWatcher.CleanupUploadedSegments()
		if deletedCount > 0 {
			h.logger.Info("Cleaned up local segments",
				"deleted", deletedCount,
				"freed_mb", float64(freedBytes)/(1024*1024),
			)
		}

		uploadedCount, uploadedBytes = segmentWatcher.GetStats()
		qualitySizes = segmentWatcher.GetQualitySizes()
		h.logger.Info("Parallel upload completed",
			"segments_uploaded", uploadedCount,
			"bytes_uploaded", uploadedBytes,
			"quality_sizes", qualitySizes,
		)
	}

	// 9. Generate and upload thumbnail
	h.publishProgress(ctx, &meta, startedAt, domain.StageGeneratingThumbnail, 88, "กำลังสร้าง thumbnail...")
	thumbnailLocalPath := filepath.Join(jobDir, "thumb.jpg")
	thumbnailRemotePath := remotePath + "/thumb.jpg"

	if err := h.transcoder.GenerateThumbnail(ctx, localInputPath, thumbnailLocalPath); err != nil {
		h.logger.Warn("Failed to generate thumbnail", "error", err)
	} else {
		if err := h.storage.Upload(ctx, thumbnailRemotePath, thumbnailLocalPath); err != nil {
			h.logger.Warn("Failed to upload thumbnail", "error", err)
		}
	}

	// 10. Extract audio
	h.publishProgress(ctx, &meta, startedAt, domain.StageExtractingAudio, 90, "กำลังตัดเสียง...")
	audioLocalPath := filepath.Join(jobDir, "audio.wav")
	audioRemotePath := fmt.Sprintf("audio/%s/audio.wav", meta.EntityCode)
	audioSuccess := false

	if err := h.transcoder.ExtractAudio(ctx, localInputPath, audioLocalPath); err != nil {
		h.logger.Warn("Failed to extract audio", "error", err)
	} else {
		if err := h.storage.Upload(ctx, audioRemotePath, audioLocalPath); err != nil {
			h.logger.Warn("Failed to upload audio", "error", err)
		} else {
			audioSuccess = true
			h.logger.Info("Audio extracted and uploaded", "path", audioRemotePath)
		}
	}

	// Get video duration
	duration := videoInfo.Duration
	if duration == 0 {
		duration, _ = h.transcoder.GetVideoDuration(localInputPath)
	}

	// 11. Publish downstream jobs
	h.publishProgress(ctx, &meta, startedAt, domain.StagePublishing, 93, "กำลังส่งงานต่อ...")

	// 11a. Gallery job
	if err := h.jobPublisher.PublishGalleryJob(ctx, &meta, &domain.GalleryJobInput{
		HLSPath:      remotePath,
		VideoQuality: h.getHighestQuality(qualitySizes),
		Duration:     int(duration),
		OutputPath:   fmt.Sprintf("gallery/%s/", meta.EntityCode),
		ImageCount:   100,
	}); err != nil {
		h.logger.Warn("Failed to publish gallery job", "error", err)
	}

	// 11b. WarmCache job (byte-range mode ไม่ต้อง warm cache เพราะ CDN cache range requests เอง)
	if err := h.jobPublisher.PublishWarmCacheJob(ctx, &meta, &domain.WarmCacheJobInput{
		HLSPath:         remotePath,
		CDNBaseURL:      h.config.CDNBaseURL,
		SegmentCounts:   qualitySegmentCounts,
		VerifyThreshold: 95,
	}); err != nil {
		h.logger.Warn("Failed to publish warmcache job", "error", err)
	}

	// 11c. Subtitle job (if audio extracted successfully)
	if audioSuccess && h.config.AutoSubtitleEnabled {
		if err := h.jobPublisher.PublishSubtitleDetectJob(ctx, &meta, &domain.SubtitleDetectJobInput{
			AudioPath:  audioRemotePath,
			OutputPath: fmt.Sprintf("subtitles/%s/", meta.EntityCode),
		}); err != nil {
			h.logger.Warn("Failed to publish subtitle job", "error", err)
		}
	}

	// 12. Delete original file (if audio succeeded)
	h.publishProgress(ctx, &meta, startedAt, domain.StageFinalizing, 97, "กำลังลบไฟล์ต้นฉบับ...")
	if audioSuccess {
		if err := h.storage.Delete(ctx, input.InputPath); err != nil {
			h.logger.Warn("Failed to delete original file", "error", err)
		} else {
			h.logger.Info("Original file deleted", "path", input.InputPath)
		}
	} else {
		h.logger.Warn("Keeping original file because audio extraction failed")
	}

	// 13. Publish completed
	durationSec := time.Since(startTime).Seconds()
	output := &domain.TranscodeOutput{
		HLSPath:       remotePath + "/master.m3u8",
		ThumbnailPath: thumbnailRemotePath,
		AudioPath:     audioRemotePath,
		Duration:      int(duration),
		DiskUsage:     uploadedBytes,
		QualitySizes:  qualitySizes,
		SegmentCounts: qualitySegmentCounts,
	}

	h.publishCompleted(ctx, &meta, startedAt, durationSec, output)

	h.logger.Info("Transcode job completed",
		"job_id", meta.JobID,
		"entity_code", meta.EntityCode,
		"duration_sec", durationSec,
		"segments_uploaded", uploadedCount,
		"bytes_uploaded", uploadedBytes,
	)

	return nil
}

// publishProgress sends a progress update (with throttling)
func (h *Handler) publishProgress(ctx context.Context, meta *domain.JobMeta, startedAt time.Time, stage string, pct float64, message string) {
	// Throttle check: ส่งเมื่อผ่านไป 2 วิ หรือ progress เปลี่ยน 5%
	if !h.throttler.ShouldSend(meta.JobID.String(), pct, "processing") {
		return // Skip this update
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
func (h *Handler) publishCompleted(ctx context.Context, meta *domain.JobMeta, startedAt time.Time, durationSec float64, output *domain.TranscodeOutput) {
	// Cleanup throttler tracking for this job
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
		Message:     "Transcode สำเร็จ",
		StartedAt:   startedAt,
		UpdatedAt:   now,
		DurationSec: &durationSec,
		Output:      output,
		// Top-level fields for backward compatibility with API
		OutputPath: output.HLSPath,
		AudioPath:  output.AudioPath,
	}

	if err := h.progress.Publish(ctx, update); err != nil {
		h.logger.Warn("Failed to publish completed", "error", err)
	}
}

// handleError handles job error (always sends, bypasses throttle)
func (h *Handler) handleError(ctx context.Context, meta *domain.JobMeta, startedAt time.Time, stage string, err error) error {
	h.logger.Error("Job failed",
		"job_id", meta.JobID,
		"stage", stage,
		"error", err,
	)

	// Cleanup throttler tracking for this job
	h.throttler.Cleanup(meta.JobID.String())

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
		Message:    "Transcode ล้มเหลว",
		Error:      err.Error(),
		StartedAt:  startedAt,
		UpdatedAt:  now,
		ErrorDetails: &ports.ErrorInfo{
			Code:        h.getErrorCode(stage),
			Stage:       stage,
			Attempt:     meta.RetryCount + 1,
			IsRetryable: h.isRetryable(stage, err),
		},
	}

	h.progress.Publish(ctx, update)

	return err
}

// cleanup removes temp files
func (h *Handler) cleanup(jobDir, jobID string) {
	h.transcoder.KillProcess(jobID)

	if err := os.RemoveAll(jobDir); err != nil {
		h.logger.Warn("Failed to cleanup job directory", "path", jobDir, "error", err)
	} else {
		h.logger.Debug("Cleaned up job directory", "path", jobDir)
	}
}

// getHighestQuality returns the highest quality from sizes map
func (h *Handler) getHighestQuality(qualitySizes map[string]int64) string {
	priorities := []string{"1080p", "720p", "480p", "360p"}
	for _, q := range priorities {
		if _, exists := qualitySizes[q]; exists {
			return q
		}
	}
	for q := range qualitySizes {
		return q
	}
	return "720p"
}

// getErrorCode returns error code based on stage
func (h *Handler) getErrorCode(stage string) string {
	switch stage {
	case domain.StageInitializing:
		return domain.ErrCodeInputValidation
	case domain.StageDownloading:
		return domain.ErrCodeDownloadFailed
	case domain.StageAnalyzing, domain.StageTranscoding, domain.StageTranscoding1080p, domain.StageTranscoding720p, domain.StageTranscoding480p:
		return domain.ErrCodeTranscodeFailed
	case domain.StageUploading, domain.StageUploadingSegments, domain.StageUploadingPlaylists:
		return domain.ErrCodeUploadFailed
	case domain.StageGeneratingThumbnail:
		return domain.ErrCodeThumbnailFailed
	case domain.StageExtractingAudio:
		return domain.ErrCodeAudioFailed
	default:
		return domain.ErrCodeTranscodeFailed
	}
}

// isRetryable checks if error should be retried
func (h *Handler) isRetryable(stage string, err error) bool {
	errStr := strings.ToLower(err.Error())

	// Not found errors are permanent
	if strings.Contains(errStr, "not found") || strings.Contains(errStr, "no such key") {
		return false
	}

	// Transcode errors might be permanent (bad file)
	if stage == domain.StageTranscoding {
		if strings.Contains(errStr, "invalid data") || strings.Contains(errStr, "moov atom not found") {
			return false
		}
	}

	return true
}

// qualityNames extracts quality names for logging
func qualityNames(qualities []domain.Quality) []string {
	names := make([]string, len(qualities))
	for i, q := range qualities {
		names[i] = q.Name
	}
	return names
}
