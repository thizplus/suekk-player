package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for gallery worker
type Config struct {
	// Worker identity
	WorkerID string

	// NATS configuration
	NATSURL      string
	StreamName   string
	ConsumerName string
	Subject      string

	// S3 configuration
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
	S3Region    string

	// CDN for presigned URLs
	CDNBaseURL      string
	StreamJWTSecret string

	// FFmpeg paths
	FFmpegPath  string
	FFprobePath string

	// Python classifier
	PythonPath       string
	ClassifierScript string

	// Processing settings
	TempDir       string
	Concurrency   int
	DefaultImages int // Default number of images to extract

	// NSFW thresholds
	SuperSafeThreshold float64 // Below this + face = super_safe
	SafeThreshold      float64 // Below this = safe, above = nsfw

	// ═══════════════════════════════════════════════════════════════════════════════
	// Two-Phase Extraction Settings (ตาม production logic)
	// ═══════════════════════════════════════════════════════════════════════════════

	// Phase 1: Extract super_safe + safe (minute 1-10)
	Phase1StartMinute int
	Phase1EndMinute   int

	// Phase 2: Extract nsfw (minute 11-30)
	Phase2StartMinute int
	Phase2EndMinute   int

	// Frame extraction
	FramesPerMinute int     // 10 frames per minute
	FrameWidth      int     // Output image width
	FrameHeight     int     // Output image height
	JpegQuality     int     // JPEG quality (1-31, lower is better)
	SkipIntro       float64 // Skip first N% of video
	SkipOutro       float64 // Skip last N% of video
	MinTimestampGap int     // Min seconds between frames

	// Image limits
	MaxSuperSafeImages int
	MaxSafeImages      int
	MaxNsfwImages      int
	MinSuperSafeImages int
	MinSafeImages      int

	// Min video duration to generate gallery (seconds)
	MinVideoDuration int

	// Manual selection mode (gender filter instead of NSFW classification)
	ManualSelectionMode bool
	GenderScriptPath    string
	GenderThreshold     float64

	// Debug
	KeepLocalFiles bool
	Verbose        bool
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		// Worker
		WorkerID: getEnvOrDefault("WORKER_ID", fmt.Sprintf("gallery-%s", getHostname())),

		// NATS (ตาม ___JOBS_TABLE_REFACTORING_PLAN.md)
		NATSURL:      getEnvOrDefault("NATS_URL", "nats://localhost:4222"),
		StreamName:   getEnvOrDefault("GALLERY_STREAM_NAME", "GALLERY_JOBS"),
		ConsumerName: getEnvOrDefault("GALLERY_CONSUMER_NAME", "GALLERY_WORKER"),
		Subject:      getEnvOrDefault("GALLERY_SUBJECT", "jobs.gallery.generate"),

		// S3
		S3Endpoint:  os.Getenv("S3_ENDPOINT"),
		S3AccessKey: os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey: os.Getenv("S3_SECRET_KEY"),
		S3Bucket:    getEnvOrDefault("S3_BUCKET", "suekk"),
		S3Region:    getEnvOrDefault("S3_REGION", "us-east-1"),

		// CDN
		CDNBaseURL:      os.Getenv("CDN_BASE_URL"),
		StreamJWTSecret: os.Getenv("STREAM_JWT_SECRET"),

		// FFmpeg
		FFmpegPath:  getEnvOrDefault("FFMPEG_PATH", "ffmpeg"),
		FFprobePath: getEnvOrDefault("FFPROBE_PATH", "ffprobe"),

		// Python
		PythonPath:       getEnvOrDefault("PYTHON_PATH", "python"),
		ClassifierScript: getEnvOrDefault("CLASSIFIER_SCRIPT", "../../python/classify_batch.py"),

		// Processing
		TempDir:       getEnvOrDefault("TEMP_DIR", os.TempDir()+"/gallery"),
		Concurrency:   getEnvInt("GALLERY_CONCURRENCY", 2),
		DefaultImages: getEnvInt("GALLERY_DEFAULT_IMAGES", 100),

		// NSFW thresholds
		SuperSafeThreshold: getEnvFloat("NSFW_SUPER_SAFE_THRESHOLD", 0.15),
		SafeThreshold:      getEnvFloat("NSFW_SAFE_THRESHOLD", 0.30),

		// Two-Phase Extraction
		Phase1StartMinute: getEnvInt("GALLERY_PHASE1_START", 0),
		Phase1EndMinute:   getEnvInt("GALLERY_PHASE1_END", 10),
		Phase2StartMinute: getEnvInt("GALLERY_PHASE2_START", 10),
		Phase2EndMinute:   getEnvInt("GALLERY_PHASE2_END", 30),

		// Frame extraction
		FramesPerMinute: getEnvInt("GALLERY_FRAMES_PER_MINUTE", 10),
		FrameWidth:      getEnvInt("GALLERY_FRAME_WIDTH", 1280),
		FrameHeight:     getEnvInt("GALLERY_FRAME_HEIGHT", 720),
		JpegQuality:     getEnvInt("GALLERY_JPEG_QUALITY", 2),
		SkipIntro:       getEnvFloat("GALLERY_SKIP_INTRO", 0.05),
		SkipOutro:       getEnvFloat("GALLERY_SKIP_OUTRO", 0.05),
		MinTimestampGap: getEnvInt("GALLERY_MIN_TIMESTAMP_GAP", 3),

		// Image limits
		MaxSuperSafeImages: getEnvInt("GALLERY_MAX_SUPER_SAFE", 10),
		MaxSafeImages:      getEnvInt("GALLERY_MAX_SAFE", 10),
		MaxNsfwImages:      getEnvInt("GALLERY_MAX_NSFW", 20),
		MinSuperSafeImages: getEnvInt("GALLERY_MIN_SUPER_SAFE", 10),
		MinSafeImages:      getEnvInt("GALLERY_MIN_SAFE", 12),

		// Min duration
		MinVideoDuration: getEnvInt("GALLERY_MIN_DURATION", 60),

		// Manual selection
		ManualSelectionMode: getEnvBool("GALLERY_MANUAL_SELECTION", false),
		GenderScriptPath:    getEnvOrDefault("GENDER_SCRIPT_PATH", "../../python/gender_gpu.py"),
		GenderThreshold:     getEnvFloat("GENDER_THRESHOLD", 85.0),

		// Debug
		KeepLocalFiles: getEnvBool("GALLERY_KEEP_LOCAL_FILES", false),
		Verbose:        getEnvBool("GALLERY_VERBOSE", false),
	}

	// Validate required fields
	if cfg.S3Endpoint == "" {
		return nil, fmt.Errorf("S3_ENDPOINT is required")
	}
	if cfg.S3AccessKey == "" {
		return nil, fmt.Errorf("S3_ACCESS_KEY is required")
	}
	if cfg.S3SecretKey == "" {
		return nil, fmt.Errorf("S3_SECRET_KEY is required")
	}

	return cfg, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			return floatVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}
