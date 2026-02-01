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
