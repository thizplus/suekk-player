package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config contains all configuration for warmcache worker
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

	// CDN
	CDNURL          string
	StreamJWTSecret string

	// Worker Settings
	Concurrency        int // จำนวน concurrent jobs
	SegmentConcurrency int // concurrent segment requests per job
	TimeoutSec         int // Per-segment timeout

	// Verification
	VerifySamplePercent int // Sample X% for verification
	VerifyThreshold     int // Must be >= X% HIT to pass
	MaxRetries          int // Max retries if verification fails

	// Debug
	Debug bool
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	hostname, _ := os.Hostname()
	workerID := getEnv("WORKER_ID", "")
	if workerID == "" {
		workerID = fmt.Sprintf("warmcache-%s-%d", hostname, os.Getpid())
	}

	cfg := &Config{
		// Worker Identity
		WorkerType: "warmcache",
		WorkerID:   workerID,
		Hostname:   hostname,

		// NATS (ตาม ___WORKER_INPUT_STANDARD.md)
		NATSURL:        getEnv("NATS_URL", "nats://localhost:4222"),
		StreamName:     getEnv("NATS_STREAM_NAME", "WARM_CACHE_JOBS"),
		Subject:        getEnv("NATS_SUBJECT", "jobs.warmcache"),
		ConsumerName:   getEnv("NATS_CONSUMER_NAME", "WARM_CACHE_WORKER"),
		AckWaitMinutes: getEnvInt("NATS_ACK_WAIT_MINUTES", 10),

		// CDN
		CDNURL:          getEnv("CDN_BASE_URL", ""),
		StreamJWTSecret: getEnv("STREAM_JWT_SECRET", ""),

		// Worker Settings
		Concurrency:        getEnvInt("WARM_CONCURRENCY", 1),
		SegmentConcurrency: getEnvInt("WARM_SEGMENT_CONCURRENCY", 20),
		TimeoutSec:         getEnvInt("WARM_TIMEOUT_SEC", 30),

		// Verification
		VerifySamplePercent: getEnvInt("VERIFY_SAMPLE_PERCENT", 10),
		VerifyThreshold:     getEnvInt("VERIFY_THRESHOLD", 95),
		MaxRetries:          getEnvInt("MAX_RETRIES", 3),

		// Debug
		Debug: getEnvBool("DEBUG", false),
	}

	// Validate required config
	if cfg.CDNURL == "" {
		return nil, fmt.Errorf("CDN_BASE_URL is required")
	}
	if cfg.StreamJWTSecret == "" {
		return nil, fmt.Errorf("STREAM_JWT_SECRET is required")
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
