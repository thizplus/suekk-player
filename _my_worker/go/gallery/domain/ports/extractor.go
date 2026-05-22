package ports

import (
	"context"
)

// ExtractorPort defines interface for frame extraction
type ExtractorPort interface {
	// ExtractFrames extracts frames from a video/HLS at specified intervals
	// Returns paths to extracted frame files
	ExtractFrames(ctx context.Context, inputPath, outputDir string, count int, duration int, onProgress ProgressCallback) ([]string, error)

	// ExtractFromHLS extracts frames from HLS playlist
	ExtractFromHLS(ctx context.Context, playlistPath, outputDir string, count int, duration int, onProgress ProgressCallback) ([]string, error)

	// ExtractRange extracts frames from specific time range (Two-Phase extraction)
	// startSec/endSec: time range in seconds
	// framesPerMinute: how many frames to extract per minute
	// Returns paths to extracted frame files
	ExtractRange(ctx context.Context, playlistPath, outputDir string, startSec, endSec int, framesPerMinute int, onProgress ProgressCallback) ([]string, error)
}
