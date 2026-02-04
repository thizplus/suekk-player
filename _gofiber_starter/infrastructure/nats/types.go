package nats

import "time"

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
	ReelStreamName     = "REEL_JOBS"
	ReelConsumerName   = "REEL_WORKER"
	SubjectReelExport  = "jobs.reel.export"
	SubjectReelProgress = "progress.reel"
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
// ⚠️ โครงสร้างนี้ต้องตรงกับ Worker
// ═══════════════════════════════════════════════════════════════════════════════
type ProgressUpdate struct {
	VideoID    string  `json:"video_id"`
	VideoCode  string  `json:"video_code"`
	Status     string  `json:"status"`     // processing, completed, failed
	Stage      string  `json:"stage"`      // Subtitle stage: downloading, transcribing, generating, etc.
	Progress   float64 `json:"progress"`   // 0-100
	Quality    string  `json:"quality"`    // 1080p, 720p, 480p (transcode)
	Message    string  `json:"message"`    // Human readable message
	Error      string  `json:"error,omitempty"`
	OutputPath string  `json:"output_path,omitempty"`
	AudioPath  string  `json:"audio_path,omitempty"` // S3 path to extracted audio (WAV)
	WorkerID   string  `json:"worker_id,omitempty"`  // Worker ที่ส่ง message นี้

	// Subtitle-specific fields (from Python worker)
	SubtitleID      string `json:"subtitle_id,omitempty"`
	CurrentLanguage string `json:"current_language,omitempty"`
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
// ReelExportJob - API → Reel Worker (via JetStream)
// สำหรับ export reel เป็น MP4 แนวตั้ง (9:16) พร้อม layers composition
// ⚠️ โครงสร้างนี้ต้องตรงกับ _reel_worker
// ═══════════════════════════════════════════════════════════════════════════════
type ReelExportJob struct {
	ReelID       string  `json:"reel_id"`
	VideoID      string  `json:"video_id"`
	VideoCode    string  `json:"video_code"`
	HLSPath      string  `json:"hls_path"`       // S3 path: hls/{code}/master.m3u8
	VideoQuality string  `json:"video_quality"`  // Best available quality: 1080p, 720p, etc.
	SegmentStart float64 `json:"segment_start"`  // Start time in seconds
	SegmentEnd   float64 `json:"segment_end"`    // End time in seconds
	CoverTime    float64 `json:"cover_time"`     // Cover/thumbnail time (-1 = auto middle)

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
	TTSText string `json:"tts_text"` // ข้อความพากย์เสียง (ถ้าว่าง = ไม่มีเสียง)

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
