package ports

import (
	"context"

	"github.com/suekk/my_worker/go/reel/domain"
)

// ProcessorPort defines interface for reel video processing
type ProcessorPort interface {
	// ProcessSegments cuts video segments and applies style transformation
	// Returns path to processed video
	ProcessSegments(ctx context.Context, inputPath, outputPath string, segments []domain.Segment, style domain.ReelStyle, cropX, cropY int, onProgress ProgressCallback) error

	// AddTextOverlay adds text overlays (title, line1, line2) to video
	AddTextOverlay(ctx context.Context, inputPath, outputPath string, title, line1, line2 string, onProgress ProgressCallback) error

	// AddLogoOverlay adds logo overlay to video
	AddLogoOverlay(ctx context.Context, inputPath, outputPath, logoPath string, onProgress ProgressCallback) error

	// GenerateTTS generates TTS audio from text using ElevenLabs API
	GenerateTTS(ctx context.Context, text, voice, outputPath string) error

	// MergeAudioVideo merges TTS audio with video
	MergeAudioVideo(ctx context.Context, videoPath, audioPath, outputPath string) error

	// GenerateThumbnail creates a thumbnail at specific time
	// coverTime -1 = auto middle
	GenerateThumbnail(ctx context.Context, videoPath, outputPath string, coverTime float64) error

	// GetVideoDuration returns video duration in seconds
	GetVideoDuration(videoPath string) (float64, error)

	// GetVideoInfo returns video metadata
	GetVideoInfo(videoPath string) (*VideoInfo, error)
}

// VideoInfo contains video metadata
type VideoInfo struct {
	Width    int
	Height   int
	Duration float64
	Bitrate  int
	FPS      float64
}

// ProgressCallback for progress updates
type ProgressCallback func(pct float64, msg string)
