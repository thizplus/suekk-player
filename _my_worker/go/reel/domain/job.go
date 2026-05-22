package domain

import (
	"fmt"

	"github.com/google/uuid"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Job Message Format
// ═══════════════════════════════════════════════════════════════════════════════

// JobMeta contains standard metadata for all job types
type JobMeta struct {
	JobID      uuid.UUID `json:"job_id"`
	JobType    string    `json:"job_type"`
	EntityType string    `json:"entity_type"`
	EntityID   uuid.UUID `json:"entity_id"`
	EntityCode string    `json:"entity_code"`
	Priority   int       `json:"priority"`
	RetryCount int       `json:"retry_count"`
	MaxRetries int       `json:"max_retries"`
	TimeoutSec int       `json:"timeout_sec"`
	CreatedAt  int64     `json:"created_at"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// Reel Style Types
// ═══════════════════════════════════════════════════════════════════════════════

// ReelStyle defines output style/aspect ratio
type ReelStyle string

const (
	StyleLetterbox ReelStyle = "letterbox" // 9:16 vertical with letterbox
	StyleSquare    ReelStyle = "square"    // 1:1 square
	StyleFullCover ReelStyle = "fullcover" // 9:16 cropped to fill
)

// ═══════════════════════════════════════════════════════════════════════════════
// Job Input/Output
// ═══════════════════════════════════════════════════════════════════════════════

// ReelInput defines input for reel processing job
type ReelInput struct {
	// Source video
	VideoPath  string `json:"video_path"`  // S3 path to source video (HLS or MP4)
	OutputPath string `json:"output_path"` // S3 output prefix: reel/{reel_id}/

	// Multi-segment support (preferred)
	Segments []Segment `json:"segments"` // Video segments to process

	// Legacy single segment (fallback)
	SegmentStart float64 `json:"segment_start"` // Start time in seconds
	SegmentEnd   float64 `json:"segment_end"`   // End time in seconds

	// Style configuration
	Style    ReelStyle `json:"style"`     // letterbox, square, fullcover
	CropX    int       `json:"crop_x"`    // Crop X position 0-100 (50 = center)
	CropY    int       `json:"crop_y"`    // Crop Y position 0-100 (50 = center)
	ShowLogo bool      `json:"show_logo"` // Add logo overlay

	// Text overlays
	Title string `json:"title"` // Main title text
	Line1 string `json:"line1"` // Secondary line 1
	Line2 string `json:"line2"` // Secondary line 2

	// TTS (Text-to-Speech)
	AddTTS   bool   `json:"add_tts"`   // Enable TTS narration
	TTSText  string `json:"tts_text"`  // TTS script
	TTSVoice string `json:"tts_voice"` // TTS voice ID (empty = default)

	// Thumbnail
	CoverTime float64 `json:"cover_time"` // Thumbnail time (-1 = auto middle)

	// Legacy fields (backward compatibility)
	OutputCodec string `json:"output_codec"` // h264, h265
	Quality     string `json:"quality"`      // 1080p, 720p
}

// Segment defines a video segment
type Segment struct {
	StartTime float64 `json:"start_time"` // Start in seconds
	EndTime   float64 `json:"end_time"`   // End in seconds
	Text      string  `json:"text"`       // Caption/subtitle text (optional)
}

// ReelJob is the full job message
type ReelJob struct {
	Meta  JobMeta   `json:"_meta"`
	Input ReelInput `json:"input"`
}

// ReelOutput defines output for reel job
type ReelOutput struct {
	VideoPath     string  `json:"video_path"`     // Output video path
	ThumbnailPath string  `json:"thumbnail_path"` // Thumbnail path
	Duration      float64 `json:"duration"`       // Output duration
	FileSize      int64   `json:"file_size"`      // Output file size
	Width         int     `json:"width"`          // Output width
	Height        int     `json:"height"`         // Output height
}

// ═══════════════════════════════════════════════════════════════════════════════
// Helper Methods
// ═══════════════════════════════════════════════════════════════════════════════

// GetSegments returns segments (converts legacy single segment if needed)
func (i *ReelInput) GetSegments() []Segment {
	if len(i.Segments) > 0 {
		return i.Segments
	}
	// Convert legacy single segment
	if i.SegmentEnd > i.SegmentStart {
		return []Segment{{
			StartTime: i.SegmentStart,
			EndTime:   i.SegmentEnd,
		}}
	}
	return nil
}

// ValidateSegments checks if segments are valid
func (i *ReelInput) ValidateSegments() error {
	segments := i.GetSegments()
	if len(segments) == 0 {
		return fmt.Errorf("no segments provided")
	}

	for idx, seg := range segments {
		// Check start < end
		if seg.StartTime >= seg.EndTime {
			return fmt.Errorf("segment %d: start time (%.2f) must be less than end time (%.2f)",
				idx+1, seg.StartTime, seg.EndTime)
		}
		// Check non-negative
		if seg.StartTime < 0 {
			return fmt.Errorf("segment %d: start time cannot be negative", idx+1)
		}
		// Check duration is reasonable (at least 0.1 seconds)
		if seg.EndTime-seg.StartTime < 0.1 {
			return fmt.Errorf("segment %d: duration too short (min 0.1 seconds)", idx+1)
		}
	}

	return nil
}

// GetTotalDuration returns total duration of all segments
func (i *ReelInput) GetTotalDuration() float64 {
	var total float64
	for _, seg := range i.GetSegments() {
		total += seg.EndTime - seg.StartTime
	}
	return total
}

// GetOutputDimensions returns output dimensions based on style
func (s ReelStyle) GetOutputDimensions() (width, height int) {
	switch s {
	case StyleSquare:
		return 1080, 1080 // 1:1
	case StyleLetterbox, StyleFullCover:
		return 1080, 1920 // 9:16
	default:
		return 1080, 1920
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Stages
// ═══════════════════════════════════════════════════════════════════════════════

const (
	StageInitializing        = "initializing"          // 0-5%
	StageDownloading         = "downloading"           // 5-25%
	StageComposing           = "composing"             // 25-55% - FFmpeg composition
	StageProcessing          = "composing"             // legacy alias
	StageGeneratingTTS       = "generating_tts"        // 55-65%
	StageTTS                 = "generating_tts"        // legacy alias
	StageMergingAudio        = "merging_audio"         // 65-70%
	StageMerging             = "merging_audio"         // legacy alias
	StageGeneratingThumbnail = "generating_thumbnail"  // 70-80%
	StageOverlays            = "overlays"              // legacy - Add text/logo overlays
	StageUploading           = "uploading"             // 80-95%
	StageFinalizing          = "finalizing"            // 95-99%
	StageCompleted           = "completed"             // 100%
	StageFailed              = "failed"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Error Codes
// ═══════════════════════════════════════════════════════════════════════════════

const (
	ErrCodeDownloadFailed   = "REEL_DOWNLOAD_FAILED"
	ErrCodeProcessFailed    = "REEL_PROCESS_FAILED"
	ErrCodeOverlayFailed    = "REEL_OVERLAY_FAILED"
	ErrCodeTTSFailed        = "REEL_TTS_FAILED"
	ErrCodeUploadFailed     = "REEL_UPLOAD_FAILED"
	ErrCodeInputValidation  = "INPUT_VALIDATION_FAILED"
	ErrCodeSegmentsTooLong  = "SEGMENTS_TOO_LONG"
	ErrCodeNoSegments       = "NO_SEGMENTS"
)
