package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for reel worker
type Config struct {
	WorkerID string

	// NATS
	NATSURL      string
	StreamName   string
	ConsumerName string
	Subject      string

	// S3
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
	S3Region    string

	// CDN for presigned URLs
	CDNBaseURL      string
	StreamJWTSecret string

	// FFmpeg
	FFmpegPath  string
	FFprobePath string
	GPUEnabled  bool

	// TTS (ElevenLabs)
	TTSAPIUrl       string
	TTSAPIKey       string
	TTSDefaultVoice string

	// Processing
	TempDir     string
	Concurrency int

	// Output settings
	OutputWidth    int
	OutputHeight   int
	OutputBitrate  string // e.g., "4M"
	OutputFPS      int
	OutputCodec    string // h264, h265
	JpegQuality    int

	// Text overlay
	FontPath     string
	FontSize     int
	FontColor    string
	TitleFontSize int

	// Logo
	LogoPath    string
	LogoWidth   int
	LogoOpacity float64
	LogoX       int // Position X (pixels from right edge)
	LogoY       int // Position Y (pixels from bottom edge)

	// Limits
	MaxDuration float64 // Max total duration in seconds
	MaxSegments int     // Max number of segments

	// Debug
	KeepLocalFiles bool
	Verbose        bool
}

// Load loads configuration from environment
func Load() (*Config, error) {
	cfg := &Config{
		WorkerID: getEnvOrDefault("WORKER_ID", fmt.Sprintf("reel-%s", getHostname())),

		// NATS (ตาม ___JOBS_TABLE_REFACTORING_PLAN.md)
		NATSURL:      getEnvOrDefault("NATS_URL", "nats://localhost:4222"),
		StreamName:   getEnvOrDefault("REEL_STREAM_NAME", "REEL_JOBS"),
		ConsumerName: getEnvOrDefault("REEL_CONSUMER_NAME", "REEL_WORKER"),
		Subject:      getEnvOrDefault("REEL_SUBJECT", "jobs.reel.export"),

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
		GPUEnabled:  os.Getenv("USE_NVENC") == "true" || os.Getenv("GPU_ENABLED") == "true",

		// TTS (ElevenLabs)
		TTSAPIUrl:       getEnvOrDefault("TTS_API_URL", "https://api.elevenlabs.io/v1"),
		TTSAPIKey:       os.Getenv("ELEVENLABS_API_KEY"),
		TTSDefaultVoice: getEnvOrDefault("TTS_DEFAULT_VOICE", "21m00Tcm4TlvDq8ikWAM"), // Rachel

		// Processing
		TempDir:     getEnvOrDefault("TEMP_DIR", os.TempDir()+"/reel"),
		Concurrency: getEnvInt("REEL_CONCURRENCY", 2),

		// Output settings
		OutputWidth:   getEnvInt("REEL_OUTPUT_WIDTH", 1080),
		OutputHeight:  getEnvInt("REEL_OUTPUT_HEIGHT", 1920),
		OutputBitrate: getEnvOrDefault("REEL_OUTPUT_BITRATE", "4M"),
		OutputFPS:     getEnvInt("REEL_OUTPUT_FPS", 30),
		OutputCodec:   getEnvOrDefault("REEL_OUTPUT_CODEC", "h264"),
		JpegQuality:   getEnvInt("REEL_JPEG_QUALITY", 2),

		// Text overlay
		FontPath:      getEnvOrDefault("REEL_FONT_PATH", "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf"),
		FontSize:      getEnvInt("REEL_FONT_SIZE", 48),
		FontColor:     getEnvOrDefault("REEL_FONT_COLOR", "white"),
		TitleFontSize: getEnvInt("REEL_TITLE_FONT_SIZE", 64),

		// Logo
		LogoPath:    getEnvOrDefault("REEL_LOGO_PATH", ""),
		LogoWidth:   getEnvInt("REEL_LOGO_WIDTH", 150),
		LogoOpacity: getEnvFloat("REEL_LOGO_OPACITY", 0.8),
		LogoX:       getEnvInt("REEL_LOGO_X", 30),
		LogoY:       getEnvInt("REEL_LOGO_Y", 30),

		// Limits
		MaxDuration: getEnvFloat("REEL_MAX_DURATION", 60.0),
		MaxSegments: getEnvInt("REEL_MAX_SEGMENTS", 10),

		// Debug
		KeepLocalFiles: getEnvBool("REEL_KEEP_LOCAL_FILES", false),
		Verbose:        getEnvBool("REEL_VERBOSE", false),
	}

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

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func getEnvFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func getHostname() string {
	h, _ := os.Hostname()
	if h == "" {
		return "unknown"
	}
	return h
}
