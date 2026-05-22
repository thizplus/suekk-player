package ports

import (
	"context"

	"github.com/suekk/my_worker/go/transcode/domain"
)

// ProgressCallback is called during transcoding with progress updates
type ProgressCallback func(quality string, percent float64, message string)

// TranscoderPort defines interface for video transcoding operations
type TranscoderPort interface {
	// TranscodeToHLS transcodes video to HLS format with multiple qualities
	// Returns segment counts per quality
	// useByteRange: true = single file HLS (EXT-X-BYTERANGE), false = separate .ts segments
	TranscodeToHLS(ctx context.Context, jobID string, inputPath, outputDir string, qualities []domain.Quality, hlsTime int, useByteRange bool, onProgress ProgressCallback) (map[string]int, error)

	// GenerateThumbnail creates a thumbnail from video
	GenerateThumbnail(ctx context.Context, inputPath, outputPath string) error

	// ExtractAudio extracts audio track to WAV format
	ExtractAudio(ctx context.Context, inputPath, outputPath string) error

	// GetVideoDuration returns video duration in seconds
	GetVideoDuration(inputPath string) (float64, error)

	// GetVideoInfo returns comprehensive video metadata
	GetVideoInfo(inputPath string) (*domain.VideoInfo, error)

	// HasAvailableVRAM checks if GPU has enough VRAM
	HasAvailableVRAM() bool

	// CleanupZombieProcesses kills any stuck FFmpeg processes
	CleanupZombieProcesses() int

	// KillProcess kills a specific job's FFmpeg process
	KillProcess(jobID string)
}
