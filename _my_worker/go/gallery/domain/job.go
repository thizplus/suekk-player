package domain

import (
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

// GalleryInput defines input for gallery job
type GalleryInput struct {
	HLSPath      string `json:"hls_path"`      // S3 path: hls/{code}/
	VideoQuality string `json:"video_quality"` // Best quality: 1080p
	Duration     int    `json:"duration"`      // Video duration in seconds
	OutputPath   string `json:"output_path"`   // S3 path: gallery/{code}/
	ImageCount   int    `json:"image_count"`   // Number of images (default 100)
}

// GalleryJob is the full job message with _meta wrapper
type GalleryJob struct {
	Meta  JobMeta      `json:"_meta"`
	Input GalleryInput `json:"input"`
}

// GalleryOutput defines output for gallery job (Three-tier)
type GalleryOutput struct {
	GalleryPath     string            `json:"gallery_path"`       // gallery/{code}/
	TotalImages     int               `json:"total_images"`       // Total frames extracted
	SuperSafeCount  int               `json:"super_safe_count"`   // Uploaded to super_safe/
	SafeCount       int               `json:"safe_count"`         // Uploaded to safe/
	NSFWCount       int               `json:"nsfw_count"`         // Uploaded to nsfw/
	Classifications map[string]string `json:"classifications"`    // {filename: classification}
	RoundsUsed      int               `json:"rounds_used"`        // Number of extraction rounds
}

// ═══════════════════════════════════════════════════════════════════════════════
// Classification Types
// ═══════════════════════════════════════════════════════════════════════════════

// ClassificationType represents NSFW classification tier
type ClassificationType string

const (
	// ClassificationSuperSafe - score < 0.15 AND has face detected
	ClassificationSuperSafe ClassificationType = "super_safe"
	// ClassificationSafe - score 0.15-0.3 OR (score < 0.15 without face)
	ClassificationSafe ClassificationType = "safe"
	// ClassificationNSFW - score >= 0.3
	ClassificationNSFW ClassificationType = "nsfw"
)

// ClassificationResult holds result from Python classifier
type ClassificationResult struct {
	Filename       string  `json:"filename"`
	Classification string  `json:"classification"` // super_safe, safe, nsfw
	NsfwScore      float64 `json:"nsfw_score"`
	FaceScore      float64 `json:"face_score"`
	HasFace        bool    `json:"has_face"`
	Reason         string  `json:"reason"` // Why this classification
}

// SeparatedImages holds images separated by classification
type SeparatedImages struct {
	SuperSafe []ClassificationResult
	Safe      []ClassificationResult
	Nsfw      []ClassificationResult
	Error     []ClassificationResult
}

// ═══════════════════════════════════════════════════════════════════════════════
// Stages (ตาม ___WORKER_INTERFACE_STANDARD.md)
// ═══════════════════════════════════════════════════════════════════════════════

const (
	StageInitializing     = "initializing"      // 0-5%
	StageDownloading      = "downloading"       // 5-15% - Download HLS segments
	StageExtractingFrames = "extracting_frames" // 15-70% - Extract frames from video
	StageExtracting       = "extracting_frames" // legacy alias
	StageClassifying      = "classifying"       // 70-85% - NSFW classification
	StageUploading        = "uploading"         // 85-95% - Upload to S3
	StageFinalizing       = "finalizing"        // 95-99%
	StageCompleted        = "completed"         // 100%
	StageFailed           = "failed"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Error Codes
// ═══════════════════════════════════════════════════════════════════════════════

const (
	ErrCodeDownloadFailed       = "GALLERY_DOWNLOAD_FAILED"
	ErrCodeExtractionFailed     = "GALLERY_EXTRACTION_FAILED"
	ErrCodeClassificationFailed = "GALLERY_CLASSIFICATION_FAILED"
	ErrCodeUploadFailed         = "GALLERY_UPLOAD_FAILED"
	ErrCodeInputValidation      = "INPUT_VALIDATION_FAILED"
	ErrCodePythonError          = "PYTHON_CLASSIFIER_ERROR"
	ErrCodeVideoTooShort        = "VIDEO_TOO_SHORT"
)
