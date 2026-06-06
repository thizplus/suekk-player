package nats

import (
	"time"

	"github.com/google/uuid"
	"gofiber-template/domain/models"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Standard Job Message Format with _meta wrapper
// ตาม ___WORKER_INTERFACE_STANDARD.md และ ___NATS_MESSAGE_STANDARD.md
// ═══════════════════════════════════════════════════════════════════════════════

// JobMeta contains metadata for all job types
// Workers MUST read this to get job_id for progress tracking
type JobMeta struct {
	JobID      uuid.UUID               `json:"job_id"`      // WorkerJob.ID - ใช้สำหรับ progress tracking
	JobType    models.WorkerJobType    `json:"job_type"`    // transcode, gallery, warmcache, etc.
	EntityType models.WorkerEntityType `json:"entity_type"` // video, subtitle, reel
	EntityID   uuid.UUID               `json:"entity_id"`   // FK to entity
	EntityCode string                  `json:"entity_code"` // human-readable code (for logging)
	Priority   int                     `json:"priority"`    // 1=high, 2=normal, 3=low
	RetryCount int                     `json:"retry_count"` // current retry attempt
	MaxRetries int                     `json:"max_retries"` // max allowed retries
	TimeoutSec int                     `json:"timeout_sec"` // job timeout in seconds
	CreatedAt  int64                   `json:"created_at"`  // unix timestamp
}

// JobMessage is the standard wrapper for all job messages
// Format: { "_meta": {...}, "input": {...} }
type JobMessage[T any] struct {
	Meta  JobMeta `json:"_meta"`
	Input T       `json:"input"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// Standard Input Types for each Job
// Workers รับ input เหล่านี้ใน field "input"
// ═══════════════════════════════════════════════════════════════════════════════

// TranscodeInput - Input for transcode job
type TranscodeInput struct {
	InputPath    string   `json:"input_path"`     // S3 path: videos/{code}/original.mp4
	OutputPath   string   `json:"output_path"`    // S3 path: hls/{code}/
	Codec        string   `json:"codec"`          // h264 or h265
	Qualities    []string `json:"qualities"`      // ["1080p", "720p", "480p"]
	UseByteRange bool     `json:"use_byte_range"` // Single file HLS
}

// GalleryInput - Input for gallery job
type GalleryInput struct {
	HLSPath      string `json:"hls_path"`       // hls/{code}/master.m3u8
	VideoQuality string `json:"video_quality"`  // Best quality: 1080p, 720p, etc.
	Duration     int    `json:"duration"`       // Video duration in seconds
	OutputPath   string `json:"output_path"`    // gallery/{code}/
	ImageCount   int    `json:"image_count"`    // Number of images (default 100)
}

// WarmCacheInput - Input for warm cache job
type WarmCacheInput struct {
	HLSPath       string         `json:"hls_path"`       // hls/{code}/
	SegmentCounts map[string]int `json:"segment_counts"` // {"1080p": 150, "720p": 150}
	Priority      int            `json:"priority"`       // 1=new, 2=popular, 3=backfill
}

// SubtitleDetectInput - Input for subtitle detect job
type SubtitleDetectInput struct {
	AudioPath string `json:"audio_path"` // S3 path to audio file
}

// SubtitleTranscribeInput - Input for subtitle transcribe job
type SubtitleTranscribeInput struct {
	VideoID       string `json:"video_id"`       // UUID of video
	VideoCode     string `json:"video_code"`     // video code for logging
	AudioPath     string `json:"audio_path"`     // S3 path to audio file
	Language      string `json:"language"`       // Detected language
	OutputPath    string `json:"output_path"`    // S3 path for SRT output
	RefineWithLLM bool   `json:"refine_with_llm"`
	Context       string `json:"context,omitempty"` // Video description
}

// SubtitleTranslateInput - Input for subtitle translate job
type SubtitleTranslateInput struct {
	SubtitleIDs     []string `json:"subtitle_ids"`     // IDs of subtitle records
	VideoID         string   `json:"video_id"`         // UUID of video
	VideoCode       string   `json:"video_code"`       // video code for logging
	SourceSRTPath   string   `json:"source_srt_path"`  // S3 path to original SRT
	SourceLanguage  string   `json:"source_language"`
	TargetLanguages []string `json:"target_languages"`
	OutputPath      string   `json:"output_path"`       // S3 directory for translated SRTs
	Context         string   `json:"context,omitempty"` // Video description
}

// VideoSegment - Time range for reel segments
type VideoSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// ReelInput - Input for reel export job
type ReelInput struct {
	VideoID      string `json:"video_id"`
	VideoCode    string `json:"video_code"`
	HLSPath      string `json:"hls_path"`      // S3 path: hls/{code}/master.m3u8
	VideoQuality string `json:"video_quality"` // Best quality: 1080p, 720p, etc.

	// Multi-segment support
	Segments []VideoSegment `json:"segments"` // Multiple time ranges

	// Legacy single segment (backward compat)
	SegmentStart float64 `json:"segment_start,omitempty"`
	SegmentEnd   float64 `json:"segment_end,omitempty"`
	CoverTime    float64 `json:"cover_time,omitempty"`

	// Style-based composition
	Style        string  `json:"style"` // letterbox, square, fullcover
	Title        string  `json:"title,omitempty"`
	Line1        string  `json:"line1,omitempty"`
	Line2        string  `json:"line2,omitempty"`
	ShowLogo     bool    `json:"show_logo"`
	LogoPath     string  `json:"logo_path,omitempty"`
	GradientPath string  `json:"gradient_path,omitempty"`
	CropX        float64 `json:"crop_x,omitempty"`
	CropY        float64 `json:"crop_y,omitempty"`

	// TTS
	TTSText  string `json:"tts_text,omitempty"`
	TTSVoice string `json:"tts_voice,omitempty"`

	OutputPath string `json:"output_path"` // S3 path: reels/{reel_id}/output.mp4
}

// ═══════════════════════════════════════════════════════════════════════════════
// Standard Progress Update Format
// Workers ส่ง progress ในรูปแบบนี้
// ═══════════════════════════════════════════════════════════════════════════════

// StandardProgressUpdate - Progress update with job_id
type StandardProgressUpdate struct {
	// Required - Worker MUST include job_id
	JobID      uuid.UUID `json:"job_id"`      // WorkerJob.ID จาก _meta
	EntityID   uuid.UUID `json:"entity_id"`   // entity_id จาก _meta
	EntityCode string    `json:"entity_code"` // entity_code จาก _meta
	WorkerID   string    `json:"worker_id"`   // Worker identifier

	// Status
	Status   string  `json:"status"`   // processing, completed, failed
	Progress float64 `json:"progress"` // 0-100
	Stage    string  `json:"stage"`    // Current stage: downloading, transcoding, etc.
	Message  string  `json:"message"`  // Human readable message

	// Optional
	Error      string `json:"error,omitempty"`       // Error message if failed
	OutputPath string `json:"output_path,omitempty"` // Output path if completed

	// Job-specific (optional)
	Quality         string `json:"quality,omitempty"`          // For transcode: 1080p, 720p
	AudioPath       string `json:"audio_path,omitempty"`       // For transcode: extracted audio
	SubtitleID      string `json:"subtitle_id,omitempty"`      // For subtitle jobs
	CurrentLanguage string `json:"current_language,omitempty"` // For subtitle jobs
	FileSize        int64  `json:"file_size,omitempty"`        // For reel jobs
}

// ═══════════════════════════════════════════════════════════════════════════════
// Stream and Consumer names
// ═══════════════════════════════════════════════════════════════════════════════

// Stream and Consumer names
const (
	StreamName   = "TRANSCODE_JOBS"
	ConsumerName = "WORKER"
	SubjectJobs  = "jobs.transcode"

	// Pub/Sub subject for progress updates
	SubjectProgress = "progress"

	// Subtitle Jobs Stream and Subjects
	SubtitleStreamName         = "SUBTITLE_JOBS"
	SubtitleConsumerName       = "SUBTITLE_WORKER"
	SubjectSubtitleDetect      = "jobs.subtitle.detect"
	SubjectSubtitleTranscribe  = "jobs.subtitle.transcribe"
	SubjectSubtitleTranslate   = "jobs.subtitle.translate"
	SubjectSubtitleProgress    = "progress.subtitle"

	// Warm Cache Jobs Stream and Subjects
	WarmCacheStreamName   = "WARM_CACHE_JOBS"
	WarmCacheConsumerName = "WARM_CACHE_WORKER"
	SubjectWarmCache      = "jobs.warmcache"

	// Reel Export Jobs Stream and Subjects
	ReelStreamName      = "REEL_JOBS"
	ReelConsumerName    = "REEL_WORKER"
	SubjectReelExport   = "jobs.reel.export"
	SubjectReelProgress = "progress.reel"

	// Gallery Jobs Stream and Subjects
	GalleryStreamName      = "GALLERY_JOBS"
	GalleryConsumerName    = "GALLERY_WORKER"
	SubjectGalleryGenerate = "jobs.gallery.generate"
	SubjectGalleryProgress = "progress.gallery"
)

// ═══════════════════════════════════════════════════════════════════════════════
// TranscodeJob - API → Worker (via JetStream)
// ⚠️ โครงสร้างนี้ต้องตรงกับ Worker
// ═══════════════════════════════════════════════════════════════════════════════
type TranscodeJob struct {
	VideoID      string   `json:"video_id"`
	VideoCode    string   `json:"video_code"`
	InputPath    string   `json:"input_path"`     // S3 path: videos/{code}/original.mp4
	OutputPath   string   `json:"output_path"`    // S3 path: hls/{code}/
	Codec        string   `json:"codec"`          // h264 or h265
	Qualities    []string `json:"qualities"`      // ["1080p", "720p", "480p"]
	UseByteRange bool     `json:"use_byte_range"` // Single file HLS
	CreatedAt    int64    `json:"created_at"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// ProgressUpdate - Worker → API (via Pub/Sub)
// รองรับทั้ง legacy format (video_id) และ standard format (entity_id)
// ═══════════════════════════════════════════════════════════════════════════════
type ProgressUpdate struct {
	// Standard format (from new workers) - เอา priority
	EntityID   string `json:"entity_id,omitempty"`
	EntityCode string `json:"entity_code,omitempty"`
	EntityType string `json:"entity_type,omitempty"` // transcode, gallery, warmcache, etc.
	JobType    string `json:"job_type,omitempty"`    // subtitle_detect, subtitle_transcribe, subtitle_translate, etc.

	// Legacy format (for backward compatibility)
	VideoID   string `json:"video_id,omitempty"`
	VideoCode string `json:"video_code,omitempty"`

	Status   string  `json:"status"`   // processing, completed, failed
	Stage    string  `json:"stage"`    // Subtitle stage: downloading, transcribing, generating, etc.
	Progress float64 `json:"progress"` // 0-100
	Quality  string  `json:"quality"`  // 1080p, 720p, 480p (transcode)
	Message  string  `json:"message"`  // Human readable message

	Error      string `json:"error,omitempty"`
	OutputPath string `json:"output_path,omitempty"`
	AudioPath  string `json:"audio_path,omitempty"` // S3 path to extracted audio (WAV)
	WorkerID   string `json:"worker_id,omitempty"`  // Worker ที่ส่ง message นี้

	// Subtitle-specific fields (from Python worker)
	SubtitleID      string `json:"subtitle_id,omitempty"`
	CurrentLanguage string `json:"current_language,omitempty"`

	// Reel-specific fields (from reel worker - uses different field names)
	ReelID   string `json:"reel_id,omitempty"`
	FileSize int64  `json:"file_size,omitempty"`
	// Note: reel worker ใช้ output_url แทน output_path แต่เรา map เป็น OutputPath

	// Transcode output fields (from new worker - nested in "output" object)
	Output *TranscodeOutputData `json:"output,omitempty"`

	// Raw output for non-transcode jobs (subtitle detect/transcribe/translate)
	// Go จะ parse output เป็น TranscodeOutputData ก่อน แต่ถ้าเป็น subtitle job
	// ให้ใช้ RawOutput แทน (populated ใน handleMessage)
	RawOutput map[string]interface{} `json:"-"` // ไม่ serialize - populated manually
}

// TranscodeOutputData contains detailed output from transcode worker
type TranscodeOutputData struct {
	HLSPath       string           `json:"hls_path,omitempty"`
	ThumbnailPath string           `json:"thumbnail_path,omitempty"`
	AudioPath     string           `json:"audio_path,omitempty"`
	Duration      float64          `json:"duration,omitempty"`
	DiskUsage     int64            `json:"disk_usage,omitempty"`
	QualitySizes  map[string]int64 `json:"quality_sizes,omitempty"`
	SegmentCounts map[string]int   `json:"segment_counts,omitempty"`
}

// GetVideoID returns video_id (prefers entity_id if available)
func (p *ProgressUpdate) GetVideoID() string {
	if p.EntityID != "" {
		return p.EntityID
	}
	return p.VideoID
}

// GetVideoCode returns video_code (prefers entity_code if available)
func (p *ProgressUpdate) GetVideoCode() string {
	if p.EntityCode != "" {
		return p.EntityCode
	}
	return p.VideoCode
}

// ═══════════════════════════════════════════════════════════════════════════════
// JetStream Status - สำหรับ Monitoring API
// ═══════════════════════════════════════════════════════════════════════════════
type JetStreamStatus struct {
	Stream   StreamInfo   `json:"stream"`
	Consumer ConsumerInfo `json:"consumer"`
}

type StreamInfo struct {
	Name     string `json:"name"`      // TRANSCODE_JOBS
	Messages uint64 `json:"messages"`  // จำนวน messages ในคิว
	Bytes    uint64 `json:"bytes"`     // ขนาด data ทั้งหมด
	FirstSeq uint64 `json:"first_seq"` // Sequence แรก
	LastSeq  uint64 `json:"last_seq"`  // Sequence ล่าสุด
}

type ConsumerInfo struct {
	Name        string `json:"name"`         // WORKER
	NumPending  uint64 `json:"num_pending"`  // รอ process
	NumAckPending int  `json:"ack_pending"`  // รอ ack
	Redelivered uint64 `json:"redelivered"`  // ส่งซ้ำกี่ครั้ง
}

// NewTranscodeJob สร้าง TranscodeJob ใหม่
func NewTranscodeJob(videoID, videoCode, inputPath, outputPath, codec string, qualities []string, useByteRange bool) *TranscodeJob {
	return &TranscodeJob{
		VideoID:      videoID,
		VideoCode:    videoCode,
		InputPath:    inputPath,
		OutputPath:   outputPath,
		Codec:        codec,
		Qualities:    qualities,
		UseByteRange: useByteRange,
		CreatedAt:    time.Now().Unix(),
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// WarmCacheJob - API → Warm Cache Worker (via JetStream)
// ⚠️ โครงสร้างนี้ต้องตรงกับ _warm_cache Worker
// ═══════════════════════════════════════════════════════════════════════════════
type WarmCacheJob struct {
	VideoID       string         `json:"video_id"`
	VideoCode     string         `json:"video_code"`
	HLSPath       string         `json:"hls_path"`        // hls/{code}/
	SegmentCounts map[string]int `json:"segment_counts"`  // {"1080p": 150, "720p": 150, ...}
	Priority      int            `json:"priority"`        // 1=new, 2=popular, 3=backfill
	CreatedAt     int64          `json:"created_at"`
}

// NewWarmCacheJob สร้าง WarmCacheJob ใหม่
func NewWarmCacheJob(videoID, videoCode, hlsPath string, segmentCounts map[string]int, priority int) *WarmCacheJob {
	return &WarmCacheJob{
		VideoID:       videoID,
		VideoCode:     videoCode,
		HLSPath:       hlsPath,
		SegmentCounts: segmentCounts,
		Priority:      priority,
		CreatedAt:     time.Now().Unix(),
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// VideoSegmentJob - แต่ละช่วงเวลาใน multi-segment reel
// ═══════════════════════════════════════════════════════════════════════════════
type VideoSegmentJob struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// ═══════════════════════════════════════════════════════════════════════════════
// ReelExportJob - API → Reel Worker (via JetStream)
// สำหรับ export reel เป็น MP4 แนวตั้ง (9:16) พร้อม layers composition
// ⚠️ โครงสร้างนี้ต้องตรงกับ _reel_worker
// ═══════════════════════════════════════════════════════════════════════════════
type ReelExportJob struct {
	ReelID       string  `json:"reel_id"`
	VideoID      string  `json:"video_id"`
	VideoCode    string  `json:"video_code"`
	HLSPath      string  `json:"hls_path"`      // S3 path: hls/{code}/master.m3u8
	VideoQuality string  `json:"video_quality"` // Best available quality: 1080p, 720p, etc.

	// ═══ Multi-segment support ═══
	Segments []VideoSegmentJob `json:"segments"` // หลายช่วงเวลา

	// ═══ LEGACY: Single segment (for backward compatibility) ═══
	SegmentStart float64 `json:"segment_start"` // Start time in seconds
	SegmentEnd   float64 `json:"segment_end"`   // End time in seconds
	CoverTime    float64 `json:"cover_time"`    // Cover/thumbnail time (-1 = auto middle)

	// ═══ NEW: Style-based composition (simplified) ═══
	Style        string  `json:"style"`         // letterbox, square, fullcover
	Title        string  `json:"title"`         // Main title text
	Line1        string  `json:"line1"`         // Secondary line 1
	Line2        string  `json:"line2"`         // Secondary line 2
	ShowLogo     bool    `json:"show_logo"`     // Show logo overlay
	LogoPath     string  `json:"logo_path"`     // S3 path to logo PNG (optional)
	GradientPath string  `json:"gradient_path"` // S3 path to gradient PNG (for fullcover)
	CropX        float64 `json:"crop_x"`        // 0-100 crop position X (for square/fullcover)
	CropY        float64 `json:"crop_y"`        // 0-100 crop position Y (for square)

	// ═══ TTS (Text-to-Speech) ═══
	TTSText  string `json:"tts_text"`  // ข้อความพากย์เสียง (ถ้าว่าง = ไม่มีเสียง)
	TTSVoice string `json:"tts_voice"` // Voice ID (ถ้าว่าง = ใช้ default)

	// ═══ LEGACY: Layer-based composition (deprecated) ═══
	OutputFormat string         `json:"output_format"` // 9:16, 1:1, 4:5, 16:9
	VideoFit     string         `json:"video_fit"`     // fill, fit, crop-1:1, crop-4:3, crop-4:5
	Layers       []ReelLayerJob `json:"layers"`        // Composition layers

	OutputPath string `json:"output_path"` // S3 path: reels/{reel_id}/output.mp4
	CreatedAt  int64  `json:"created_at"`
}

// ReelLayerJob layer ใน export job
type ReelLayerJob struct {
	Type       string  `json:"type"`                 // text, image, shape, background
	Content    string  `json:"content,omitempty"`    // ข้อความ หรือ URL รูปภาพ
	FontFamily string  `json:"font_family,omitempty"`
	FontSize   int     `json:"font_size,omitempty"`
	FontColor  string  `json:"font_color,omitempty"`
	FontWeight string  `json:"font_weight,omitempty"`
	X          float64 `json:"x"`                    // ตำแหน่ง X (0-100%)
	Y          float64 `json:"y"`                    // ตำแหน่ง Y (0-100%)
	Width      float64 `json:"width,omitempty"`      // ความกว้าง (0-100%)
	Height     float64 `json:"height,omitempty"`     // ความสูง (0-100%)
	Opacity    float64 `json:"opacity,omitempty"`    // ความโปร่งใส (0-1)
	ZIndex     int     `json:"z_index,omitempty"`    // ลำดับ layer
	Style      string  `json:"style,omitempty"`      // สไตล์เพิ่มเติม (gradient, shape type)
}

// NewReelExportJob สร้าง ReelExportJob ใหม่
func NewReelExportJob(reelID, videoID, videoCode, hlsPath string, segmentStart, segmentEnd float64, layers []ReelLayerJob, outputPath string) *ReelExportJob {
	return &ReelExportJob{
		ReelID:       reelID,
		VideoID:      videoID,
		VideoCode:    videoCode,
		HLSPath:      hlsPath,
		SegmentStart: segmentStart,
		SegmentEnd:   segmentEnd,
		Layers:       layers,
		OutputPath:   outputPath,
		CreatedAt:    time.Now().Unix(),
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// GalleryJob - API → Worker (via JetStream)
// สำหรับ generate gallery images จาก HLS ที่มีอยู่แล้ว
// ⚠️ โครงสร้างนี้ต้องตรงกับ _worker
// ═══════════════════════════════════════════════════════════════════════════════
type GalleryJob struct {
	VideoID      string `json:"video_id"`
	VideoCode    string `json:"video_code"`
	HLSPath      string `json:"hls_path"`       // hls/{code}/master.m3u8
	VideoQuality string `json:"video_quality"`  // Best quality: 1080p, 720p, etc.
	Duration     int    `json:"duration"`       // Video duration in seconds
	OutputPath   string `json:"output_path"`    // gallery/{code}/
	ImageCount   int    `json:"image_count"`    // Number of images to generate (default 100)
	CreatedAt    int64  `json:"created_at"`
}

// NewGalleryJob สร้าง GalleryJob ใหม่
func NewGalleryJob(videoID, videoCode, hlsPath, videoQuality string, duration int, outputPath string, imageCount int) *GalleryJob {
	if imageCount <= 0 {
		imageCount = 100 // default 100 images
	}
	return &GalleryJob{
		VideoID:      videoID,
		VideoCode:    videoCode,
		HLSPath:      hlsPath,
		VideoQuality: videoQuality,
		Duration:     duration,
		OutputPath:   outputPath,
		ImageCount:   imageCount,
		CreatedAt:    time.Now().Unix(),
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Job Message Builders
// สร้าง JobMessage พร้อม _meta wrapper สำหรับแต่ละ job type
// ═══════════════════════════════════════════════════════════════════════════════

// Default timeouts per job type (seconds)
const (
	TimeoutTranscode         = 3600  // 1 hour
	TimeoutGallery           = 1800  // 30 minutes
	TimeoutWarmCache         = 600   // 10 minutes
	TimeoutSubtitleDetect    = 300   // 5 minutes
	TimeoutSubtitleTranscribe = 1800 // 30 minutes
	TimeoutSubtitleTranslate = 900   // 15 minutes
	TimeoutReel              = 1200  // 20 minutes
)

// NewTranscodeMessage creates a transcode job message with _meta wrapper
func NewTranscodeMessage(jobID, entityID uuid.UUID, entityCode string, input TranscodeInput, priority, retryCount, maxRetries int) *JobMessage[TranscodeInput] {
	return &JobMessage[TranscodeInput]{
		Meta: JobMeta{
			JobID:      jobID,
			JobType:    models.WorkerJobTypeTranscode,
			EntityType: models.WorkerEntityTypeVideo,
			EntityID:   entityID,
			EntityCode: entityCode,
			Priority:   priority,
			RetryCount: retryCount,
			MaxRetries: maxRetries,
			TimeoutSec: TimeoutTranscode,
			CreatedAt:  time.Now().Unix(),
		},
		Input: input,
	}
}

// NewGalleryMessage creates a gallery job message with _meta wrapper
func NewGalleryMessage(jobID, entityID uuid.UUID, entityCode string, input GalleryInput, priority, retryCount, maxRetries int) *JobMessage[GalleryInput] {
	return &JobMessage[GalleryInput]{
		Meta: JobMeta{
			JobID:      jobID,
			JobType:    models.WorkerJobTypeGallery,
			EntityType: models.WorkerEntityTypeVideo,
			EntityID:   entityID,
			EntityCode: entityCode,
			Priority:   priority,
			RetryCount: retryCount,
			MaxRetries: maxRetries,
			TimeoutSec: TimeoutGallery,
			CreatedAt:  time.Now().Unix(),
		},
		Input: input,
	}
}

// NewWarmCacheMessage creates a warm cache job message with _meta wrapper
func NewWarmCacheMessage(jobID, entityID uuid.UUID, entityCode string, input WarmCacheInput, priority, retryCount, maxRetries int) *JobMessage[WarmCacheInput] {
	return &JobMessage[WarmCacheInput]{
		Meta: JobMeta{
			JobID:      jobID,
			JobType:    models.WorkerJobTypeWarmCache,
			EntityType: models.WorkerEntityTypeVideo,
			EntityID:   entityID,
			EntityCode: entityCode,
			Priority:   priority,
			RetryCount: retryCount,
			MaxRetries: maxRetries,
			TimeoutSec: TimeoutWarmCache,
			CreatedAt:  time.Now().Unix(),
		},
		Input: input,
	}
}

// NewSubtitleDetectMessage creates a subtitle detect job message with _meta wrapper
func NewSubtitleDetectMessage(jobID, entityID uuid.UUID, entityCode string, input SubtitleDetectInput, priority, retryCount, maxRetries int) *JobMessage[SubtitleDetectInput] {
	return &JobMessage[SubtitleDetectInput]{
		Meta: JobMeta{
			JobID:      jobID,
			JobType:    models.WorkerJobTypeSubtitleDetect,
			EntityType: models.WorkerEntityTypeVideo,
			EntityID:   entityID,
			EntityCode: entityCode,
			Priority:   priority,
			RetryCount: retryCount,
			MaxRetries: maxRetries,
			TimeoutSec: TimeoutSubtitleDetect,
			CreatedAt:  time.Now().Unix(),
		},
		Input: input,
	}
}

// NewSubtitleTranscribeMessage creates a subtitle transcribe job message with _meta wrapper
func NewSubtitleTranscribeMessage(jobID, entityID uuid.UUID, entityCode string, input SubtitleTranscribeInput, priority, retryCount, maxRetries int) *JobMessage[SubtitleTranscribeInput] {
	return &JobMessage[SubtitleTranscribeInput]{
		Meta: JobMeta{
			JobID:      jobID,
			JobType:    models.WorkerJobTypeSubtitleTranscribe,
			EntityType: models.WorkerEntityTypeSubtitle,
			EntityID:   entityID,
			EntityCode: entityCode,
			Priority:   priority,
			RetryCount: retryCount,
			MaxRetries: maxRetries,
			TimeoutSec: TimeoutSubtitleTranscribe,
			CreatedAt:  time.Now().Unix(),
		},
		Input: input,
	}
}

// NewSubtitleTranslateMessage creates a subtitle translate job message with _meta wrapper
func NewSubtitleTranslateMessage(jobID, entityID uuid.UUID, entityCode string, input SubtitleTranslateInput, priority, retryCount, maxRetries int) *JobMessage[SubtitleTranslateInput] {
	return &JobMessage[SubtitleTranslateInput]{
		Meta: JobMeta{
			JobID:      jobID,
			JobType:    models.WorkerJobTypeSubtitleTranslate,
			EntityType: models.WorkerEntityTypeVideo,
			EntityID:   entityID,
			EntityCode: entityCode,
			Priority:   priority,
			RetryCount: retryCount,
			MaxRetries: maxRetries,
			TimeoutSec: TimeoutSubtitleTranslate,
			CreatedAt:  time.Now().Unix(),
		},
		Input: input,
	}
}

// NewReelMessage creates a reel export job message with _meta wrapper
func NewReelMessage(jobID, entityID uuid.UUID, entityCode string, input ReelInput, priority, retryCount, maxRetries int) *JobMessage[ReelInput] {
	return &JobMessage[ReelInput]{
		Meta: JobMeta{
			JobID:      jobID,
			JobType:    models.WorkerJobTypeReel,
			EntityType: models.WorkerEntityTypeReel,
			EntityID:   entityID,
			EntityCode: entityCode,
			Priority:   priority,
			RetryCount: retryCount,
			MaxRetries: maxRetries,
			TimeoutSec: TimeoutReel,
			CreatedAt:  time.Now().Unix(),
		},
		Input: input,
	}
}
