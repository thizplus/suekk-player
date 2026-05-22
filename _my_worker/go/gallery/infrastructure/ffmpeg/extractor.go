package ffmpeg

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/suekk/my_worker/go/gallery/config"
	"github.com/suekk/my_worker/go/gallery/domain/ports"
)

// Extractor implements ExtractorPort using FFmpeg
type Extractor struct {
	config *config.Config
	logger *slog.Logger
}

// NewExtractor creates a new FFmpeg extractor
func NewExtractor(cfg *config.Config) *Extractor {
	return &Extractor{
		config: cfg,
		logger: slog.Default().With("component", "ffmpeg-extractor"),
	}
}

// ExtractFrames extracts frames from a video file
func (e *Extractor) ExtractFrames(ctx context.Context, inputPath, outputDir string, count int, duration int, onProgress ports.ProgressCallback) ([]string, error) {
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Get actual duration if not provided
	if duration <= 0 {
		var err error
		duration, err = e.getVideoDuration(inputPath)
		if err != nil {
			e.logger.Warn("Failed to get video duration", "error", err)
			duration = 3600 // Default to 1 hour
		}
	}

	// Apply skip intro/outro
	effectiveDuration := e.getEffectiveDuration(duration)
	startOffset := e.getStartOffset(duration)

	// Calculate frame interval
	interval := float64(effectiveDuration) / float64(count)
	if interval < float64(e.config.MinTimestampGap) {
		interval = float64(e.config.MinTimestampGap)
	}

	e.logger.Info("Extracting frames",
		"input", inputPath,
		"output", outputDir,
		"count", count,
		"duration", duration,
		"effective_duration", effectiveDuration,
		"start_offset", startOffset,
		"interval", interval,
	)

	var extractedFiles []string

	for i := 0; i < count; i++ {
		select {
		case <-ctx.Done():
			return extractedFiles, ctx.Err()
		default:
		}

		// Calculate timestamp with offset
		timestamp := startOffset + float64(i)*interval
		if timestamp >= float64(duration)-float64(duration)*e.config.SkipOutro {
			break
		}

		// Output filename: 001.jpg, 002.jpg, ...
		outputFile := filepath.Join(outputDir, fmt.Sprintf("%03d.jpg", i+1))

		if err := e.extractSingleFrame(ctx, inputPath, outputFile, timestamp); err != nil {
			e.logger.Warn("Failed to extract frame",
				"index", i+1,
				"timestamp", timestamp,
				"error", err,
			)
			continue // Skip failed frames
		}

		extractedFiles = append(extractedFiles, outputFile)

		// Progress callback
		if onProgress != nil {
			onProgress(i+1, count, fmt.Sprintf("ตัดภาพ %d/%d", i+1, count))
		}
	}

	e.logger.Info("Frame extraction completed",
		"requested", count,
		"extracted", len(extractedFiles),
	)

	return extractedFiles, nil
}

// ExtractFromHLS extracts frames from HLS playlist
func (e *Extractor) ExtractFromHLS(ctx context.Context, playlistPath, outputDir string, count int, duration int, onProgress ports.ProgressCallback) ([]string, error) {
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Apply skip intro/outro
	if duration <= 0 {
		duration = 3600
	}
	effectiveDuration := e.getEffectiveDuration(duration)
	startOffset := e.getStartOffset(duration)

	// Calculate interval with MinTimestampGap
	interval := float64(effectiveDuration) / float64(count)
	if interval < float64(e.config.MinTimestampGap) {
		interval = float64(e.config.MinTimestampGap)
	}

	e.logger.Info("Extracting frames from HLS",
		"playlist", playlistPath,
		"output", outputDir,
		"count", count,
		"effective_duration", effectiveDuration,
		"interval", interval,
	)

	var extractedFiles []string

	for i := 0; i < count; i++ {
		select {
		case <-ctx.Done():
			return extractedFiles, ctx.Err()
		default:
		}

		timestamp := startOffset + float64(i)*interval
		if timestamp >= float64(duration)-float64(duration)*e.config.SkipOutro {
			break
		}

		outputFile := filepath.Join(outputDir, fmt.Sprintf("%03d.jpg", i+1))

		if err := e.extractSingleFrame(ctx, playlistPath, outputFile, timestamp); err != nil {
			e.logger.Warn("Failed to extract frame from HLS",
				"index", i+1,
				"timestamp", timestamp,
				"error", err,
			)
			continue
		}

		extractedFiles = append(extractedFiles, outputFile)

		if onProgress != nil {
			onProgress(i+1, count, fmt.Sprintf("ตัดภาพ %d/%d", i+1, count))
		}
	}

	e.logger.Info("HLS frame extraction completed",
		"requested", count,
		"extracted", len(extractedFiles),
	)

	return extractedFiles, nil
}

// ExtractRange extracts frames from a specific time range (Two-Phase extraction)
// Used for two-phase gallery: Phase1 (min 1-10), Phase2 (min 11-30)
func (e *Extractor) ExtractRange(ctx context.Context, playlistPath, outputDir string, startSec, endSec int, framesPerMinute int, onProgress ports.ProgressCallback) ([]string, error) {
	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Calculate how many frames to extract
	rangeDuration := endSec - startSec
	if rangeDuration <= 0 {
		return nil, fmt.Errorf("invalid range: start=%d end=%d", startSec, endSec)
	}

	rangeMinutes := float64(rangeDuration) / 60.0
	totalFrames := int(rangeMinutes * float64(framesPerMinute))
	if totalFrames <= 0 {
		totalFrames = 1
	}

	// Calculate interval between frames (respect MinTimestampGap)
	interval := float64(rangeDuration) / float64(totalFrames)
	if interval < float64(e.config.MinTimestampGap) {
		interval = float64(e.config.MinTimestampGap)
		totalFrames = rangeDuration / e.config.MinTimestampGap
		if totalFrames <= 0 {
			totalFrames = 1
		}
	}

	e.logger.Info("ExtractRange starting",
		"playlist", playlistPath,
		"start_sec", startSec,
		"end_sec", endSec,
		"range_minutes", rangeMinutes,
		"frames_per_minute", framesPerMinute,
		"total_frames", totalFrames,
		"interval", interval,
	)

	var extractedFiles []string
	frameIndex := 0

	for i := 0; i < totalFrames; i++ {
		select {
		case <-ctx.Done():
			return extractedFiles, ctx.Err()
		default:
		}

		timestamp := float64(startSec) + float64(i)*interval
		if timestamp >= float64(endSec) {
			break
		}

		// Output filename: phase_001.jpg, phase_002.jpg, ...
		outputFile := filepath.Join(outputDir, fmt.Sprintf("phase_%03d.jpg", frameIndex+1))

		if err := e.extractSingleFrame(ctx, playlistPath, outputFile, timestamp); err != nil {
			e.logger.Warn("Failed to extract frame in range",
				"frame_index", frameIndex+1,
				"timestamp", timestamp,
				"error", err,
			)
			continue
		}

		extractedFiles = append(extractedFiles, outputFile)
		frameIndex++

		if onProgress != nil {
			onProgress(i+1, totalFrames, fmt.Sprintf("ตัดภาพ %d/%d", i+1, totalFrames))
		}
	}

	e.logger.Info("ExtractRange completed",
		"range", fmt.Sprintf("%d-%d sec", startSec, endSec),
		"expected", totalFrames,
		"extracted", len(extractedFiles),
	)

	return extractedFiles, nil
}

// extractSingleFrame extracts a single frame at timestamp using config dimensions
func (e *Extractor) extractSingleFrame(ctx context.Context, inputPath, outputFile string, timestamp float64) error {
	// Use config dimensions (default: 1280x720) and quality (default: 2)
	scaleFilter := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2",
		e.config.FrameWidth, e.config.FrameHeight,
		e.config.FrameWidth, e.config.FrameHeight,
	)

	args := []string{
		"-y",
		"-ss", fmt.Sprintf("%.2f", timestamp),
		"-i", inputPath,
		"-vframes", "1",
		"-vf", scaleFilter,
		"-q:v", strconv.Itoa(e.config.JpegQuality),
		outputFile,
	}

	cmd := exec.CommandContext(ctx, e.config.FFmpegPath, args...)
	return cmd.Run()
}

// getEffectiveDuration returns duration after skip intro/outro
func (e *Extractor) getEffectiveDuration(duration int) int {
	skipTotal := e.config.SkipIntro + e.config.SkipOutro
	return int(float64(duration) * (1.0 - skipTotal))
}

// getStartOffset returns where to start extraction (after skip intro)
func (e *Extractor) getStartOffset(duration int) float64 {
	return float64(duration) * e.config.SkipIntro
}

// getVideoDuration returns video duration in seconds
func (e *Extractor) getVideoDuration(inputPath string) (int, error) {
	args := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		inputPath,
	}

	cmd := exec.Command(e.config.FFprobePath, args...)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get duration: %w", err)
	}

	durationStr := strings.TrimSpace(string(output))
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return int(duration), nil
}
