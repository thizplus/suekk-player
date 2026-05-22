package domain

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Job Message Format (ตาม ___WORKER_INPUT_STANDARD.md)
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

// TranscodeInput defines input for transcode job
type TranscodeInput struct {
	InputPath    string   `json:"input_path"`     // S3 path: videos/{code}/original.mp4
	OutputPath   string   `json:"output_path"`    // S3 path prefix: hls/{code}/
	Codec        string   `json:"codec"`          // h264 or h265
	Qualities    []string `json:"qualities"`      // ["1080p", "720p", "480p"]
	HLSTime      int      `json:"hls_time"`       // Segment duration (default: 2)
	UseByteRange bool     `json:"use_byte_range"` // Single file HLS (optional)
}

// TranscodeJob is the full job message with _meta wrapper
type TranscodeJob struct {
	Meta  JobMeta        `json:"_meta"`
	Input TranscodeInput `json:"input"`
}

// TranscodeOutput defines output for transcode job
type TranscodeOutput struct {
	HLSPath       string         `json:"hls_path"`        // hls/{code}/master.m3u8
	ThumbnailPath string         `json:"thumbnail_path"`  // hls/{code}/thumb.jpg
	AudioPath     string         `json:"audio_path"`      // audio/{code}/audio.wav
	Duration      int            `json:"duration"`        // Video duration in seconds
	DiskUsage     int64          `json:"disk_usage"`      // Total bytes uploaded
	QualitySizes  map[string]int64 `json:"quality_sizes"` // {"1080p": 123456, ...}
	SegmentCounts map[string]int `json:"segment_counts"`  // {"1080p": 150, ...}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Quality Definition
// ═══════════════════════════════════════════════════════════════════════════════

// Quality defines video quality settings
type Quality struct {
	Name         string
	Width        int
	Height       int
	VideoBitrate string
	AudioBitrate string
}

// VideoInfo contains source video metadata
type VideoInfo struct {
	Width       int     // Source width
	Height      int     // Source height
	Duration    float64 // Duration in seconds
	Bitrate     int     // Video bitrate in bps
	FPS         float64 // Frames per second
	Codec       string  // Video codec (h264, hevc, etc.)
	AudioCodec  string  // Audio codec
	HasAudio    bool    // Has audio track
}

// DefaultQualities returns default quality presets
func DefaultQualities() []Quality {
	return []Quality{
		{Name: "1080p", Width: 1920, Height: 1080, VideoBitrate: "5M", AudioBitrate: "192k"},
		{Name: "720p", Width: 1280, Height: 720, VideoBitrate: "3M", AudioBitrate: "128k"},
		{Name: "480p", Width: 854, Height: 480, VideoBitrate: "1.5M", AudioBitrate: "96k"},
	}
}

// GetQualities returns Quality structs from names
func GetQualities(names []string) []Quality {
	qualityMap := map[string]Quality{
		"1080p": {Name: "1080p", Width: 1920, Height: 1080, VideoBitrate: "5M", AudioBitrate: "192k"},
		"720p":  {Name: "720p", Width: 1280, Height: 720, VideoBitrate: "3M", AudioBitrate: "128k"},
		"480p":  {Name: "480p", Width: 854, Height: 480, VideoBitrate: "1.5M", AudioBitrate: "96k"},
		"360p":  {Name: "360p", Width: 640, Height: 360, VideoBitrate: "800K", AudioBitrate: "64k"},
	}

	if len(names) == 0 {
		return DefaultQualities()
	}

	var qualities []Quality
	for _, name := range names {
		if q, ok := qualityMap[name]; ok {
			qualities = append(qualities, q)
		}
	}

	if len(qualities) == 0 {
		return DefaultQualities()
	}

	return qualities
}

// FilterQualitiesBySource กรอง qualities ตาม source resolution
// ไม่ output quality ที่สูงกว่า source
func FilterQualitiesBySource(qualities []Quality, sourceHeight int) []Quality {
	if sourceHeight <= 0 {
		return qualities
	}

	var filtered []Quality
	for _, q := range qualities {
		// เลือก quality ที่ height ไม่เกิน source
		if q.Height <= sourceHeight {
			filtered = append(filtered, q)
		}
	}

	// ต้องมีอย่างน้อย 1 quality
	if len(filtered) == 0 && len(qualities) > 0 {
		// ใช้ quality ต่ำสุด
		lowest := qualities[len(qualities)-1]
		filtered = append(filtered, lowest)
	}

	return filtered
}

// CapBitrate ปรับ bitrate ไม่ให้เกิน source
// sourceBitrate ในหน่วย bps
func CapBitrate(quality *Quality, sourceBitrate int) {
	if sourceBitrate <= 0 {
		return
	}

	// Cap ที่ 110% ของ source bitrate
	maxBitrate := int(float64(sourceBitrate) * 1.1)

	targetBitrate := parseBitrateValue(quality.VideoBitrate)
	if targetBitrate > maxBitrate {
		// ปรับลง
		quality.VideoBitrate = formatBitrate(maxBitrate)
	}
}

// parseBitrateValue แปลง "5M" หรือ "3000K" เป็น bps
func parseBitrateValue(s string) int {
	s = strings.ToUpper(strings.TrimSpace(s))
	multiplier := 1

	if strings.HasSuffix(s, "M") {
		multiplier = 1000000
		s = strings.TrimSuffix(s, "M")
	} else if strings.HasSuffix(s, "K") {
		multiplier = 1000
		s = strings.TrimSuffix(s, "K")
	}

	val, _ := strconv.ParseFloat(s, 64)
	return int(val * float64(multiplier))
}

// formatBitrate แปลง bps เป็น "5M" format
func formatBitrate(bps int) string {
	if bps >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(bps)/1000000)
	}
	return fmt.Sprintf("%dK", bps/1000)
}

// ═══════════════════════════════════════════════════════════════════════════════
// Downstream Job Types (jobs to publish after transcode)
// ═══════════════════════════════════════════════════════════════════════════════

// GalleryJobInput input for gallery worker
type GalleryJobInput struct {
	HLSPath      string `json:"hls_path"`      // hls/{code}/
	VideoQuality string `json:"video_quality"` // Best quality: 1080p
	Duration     int    `json:"duration"`      // Video duration in seconds
	OutputPath   string `json:"output_path"`   // gallery/{code}/
	ImageCount   int    `json:"image_count"`   // Number of images (default 100)
}

// WarmCacheJobInput input for warmcache worker
type WarmCacheJobInput struct {
	HLSPath         string         `json:"hls_path"`
	CDNBaseURL      string         `json:"cdn_base_url"`
	SegmentCounts   map[string]int `json:"segment_counts"`
	VerifyThreshold int            `json:"verify_threshold"`
}

// SubtitleDetectJobInput input for subtitle_detect worker
type SubtitleDetectJobInput struct {
	AudioPath  string `json:"audio_path"`  // audio/{code}/audio.wav
	OutputPath string `json:"output_path"` // subtitles/{code}/
}

// ═══════════════════════════════════════════════════════════════════════════════
// Stages (ตาม ___WORKER_INTERFACE_STANDARD.md)
// ═══════════════════════════════════════════════════════════════════════════════

const (
	StageInitializing        = "initializing"         // 0-2%
	StageDownloading         = "downloading"          // 2-10%
	StageAnalyzing           = "analyzing"            // 10-12%
	StageTranscoding         = "transcoding"          // 12-80% (generic)
	StageTranscoding1080p    = "transcoding_1080p"    // 12-45%
	StageTranscoding720p     = "transcoding_720p"     // 45-65%
	StageTranscoding480p     = "transcoding_480p"     // 65-80%
	StageUploadingSegments   = "uploading_segments"   // 80-90%
	StageUploadingPlaylists  = "uploading_playlists"  // 90-92%
	StageGeneratingThumbnail = "generating_thumbnail" // 92-94%
	StageExtractingAudio     = "extracting_audio"     // 94-97%
	StageUploading           = "uploading"            // 80-90% (legacy alias)
	StageThumbnail           = "generating_thumbnail" // legacy alias
	StageAudio               = "extracting_audio"     // legacy alias
	StagePublishing          = "publishing"           // 95-98%
	StageFinalizing          = "finalizing"           // 97-99%
	StageCompleted           = "completed"            // 100%
	StageFailed              = "failed"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Error Codes
// ═══════════════════════════════════════════════════════════════════════════════

const (
	ErrCodeDownloadFailed   = "TRANSCODE_DOWNLOAD_FAILED"
	ErrCodeTranscodeFailed  = "TRANSCODE_FAILED"
	ErrCodeUploadFailed     = "TRANSCODE_UPLOAD_FAILED"
	ErrCodeThumbnailFailed  = "TRANSCODE_THUMBNAIL_FAILED"
	ErrCodeAudioFailed      = "TRANSCODE_AUDIO_FAILED"
	ErrCodeInputValidation  = "INPUT_VALIDATION_FAILED"
	ErrCodeSourceNotFound   = "SOURCE_NOT_FOUND"
	ErrCodeVRAMUnavailable  = "VRAM_UNAVAILABLE"
)
