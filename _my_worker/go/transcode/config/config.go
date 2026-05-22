package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config contains all configuration for transcode worker
type Config struct {
	// Worker Identity
	WorkerType string
	WorkerID   string
	Hostname   string

	// NATS
	NATSURL        string
	StreamName     string
	Subject        string
	ConsumerName   string
	AckWaitMinutes int

	// S3 Storage
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
	S3Region    string
	S3UseSSL    bool

	// CDN (for downstream jobs)
	CDNBaseURL      string
	StreamJWTSecret string

	// Transcoder Settings
	TempDir            string
	FFmpegPath         string
	FFprobePath        string
	GPUEnabled         bool
	Preset             string // NVENC preset: p1-p7
	CRF                int
	HLSTime            int
	SegmentConcurrency int // Concurrent segment uploads

	// Worker Settings
	Concurrency int

	// Auto Subtitle
	AutoSubtitleEnabled bool
	SubtitleAPIURL      string

	// Debug
	Debug bool

	// Logging
	LogLevel      string // debug, info, warn, error
	LogFormat     string // json, text
	LogOutput     string // stdout, file, both
	LogFilePath   string // logs/worker.log
	LogErrorPath  string // logs/errors.log
	LogMaxSizeMB  int    // 100
	LogMaxBackups int    // 5
	LogMaxAgeDays int    // 30
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	hostname, _ := os.Hostname()
	workerID := getEnv("WORKER_ID", "")
	if workerID == "" {
		workerID = fmt.Sprintf("transcode-%s-%d", hostname, os.Getpid())
	}

	cfg := &Config{
		// Worker Identity
		WorkerType: "transcode",
		WorkerID:   workerID,
		Hostname:   hostname,

		// NATS
		NATSURL:        getEnv("NATS_URL", "nats://localhost:4222"),
		StreamName:     getEnv("NATS_STREAM_NAME", "TRANSCODE_JOBS"),
		Subject:        getEnv("NATS_SUBJECT", "jobs.transcode"),
		ConsumerName:   getEnv("NATS_CONSUMER_NAME", "TRANSCODE_WORKER"),
		AckWaitMinutes: getEnvInt("NATS_ACK_WAIT_MINUTES", 30),

		// S3 Storage
		S3Endpoint:  getEnv("S3_ENDPOINT", ""),
		S3AccessKey: getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey: getEnv("S3_SECRET_KEY", ""),
		S3Bucket:    getEnv("S3_BUCKET", ""),
		S3Region:    getEnv("S3_REGION", "us-east-1"),
		S3UseSSL:    getEnvBool("S3_USE_SSL", true),

		// CDN
		CDNBaseURL:      getEnv("CDN_BASE_URL", ""),
		StreamJWTSecret: getEnv("STREAM_JWT_SECRET", ""),

		// Transcoder Settings
		TempDir:            getEnv("TEMP_DIR", "/tmp/transcode"),
		FFmpegPath:         getEnv("FFMPEG_PATH", "ffmpeg"),
		FFprobePath:        getEnv("FFPROBE_PATH", "ffprobe"),
		GPUEnabled:         getEnvBool("USE_NVENC", true),
		Preset:             getEnv("NVENC_PRESET", "p4"),
		CRF:                getEnvInt("FFMPEG_CRF", 23),
		HLSTime:            getEnvInt("HLS_TIME", 2),
		SegmentConcurrency: getEnvInt("SEGMENT_CONCURRENCY", 20),

		// Worker Settings
		Concurrency: getEnvInt("TRANSCODE_CONCURRENCY", 1),

		// Auto Subtitle
		AutoSubtitleEnabled: getEnvBool("AUTO_SUBTITLE_ENABLED", false),
		SubtitleAPIURL:      getEnv("SUBTITLE_API_URL", ""),

		// Debug
		Debug: getEnvBool("DEBUG", false),

		// Logging
		LogLevel:      getEnv("LOG_LEVEL", "info"),
		LogFormat:     getEnv("LOG_FORMAT", "json"),
		LogOutput:     getEnv("LOG_OUTPUT", "both"),
		LogFilePath:   getEnv("LOG_FILE", "logs/transcode.log"),
		LogErrorPath:  getEnv("LOG_ERROR_FILE", "logs/transcode_errors.log"),
		LogMaxSizeMB:  getEnvInt("LOG_MAX_SIZE_MB", 100),
		LogMaxBackups: getEnvInt("LOG_MAX_BACKUPS", 5),
		LogMaxAgeDays: getEnvInt("LOG_MAX_AGE_DAYS", 30),
	}

	// Validate required config
	if cfg.S3Endpoint == "" {
		return nil, fmt.Errorf("S3_ENDPOINT is required")
	}
	if cfg.S3AccessKey == "" {
		return nil, fmt.Errorf("S3_ACCESS_KEY is required")
	}
	if cfg.S3SecretKey == "" {
		return nil, fmt.Errorf("S3_SECRET_KEY is required")
	}
	if cfg.S3Bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET is required")
	}

	return cfg, nil
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1"
	}
	return defaultValue
}
