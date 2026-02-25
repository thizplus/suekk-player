package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Worker        WorkerConfig
	NATS          NATSConfig
	Database      DatabaseConfig
	SuekkAPI      APIConfig
	SubthAPI      APIConfig
	Gemini        GeminiConfig
	ElevenLabs    ElevenLabsConfig
	ImageSelector ImageSelectorConfig
	SuekkStorage  StorageConfig // IDrive - for reading SRT files
	SubthStorage  StorageConfig // R2 - for uploading audio files
	Alert         AlertConfig
}

type WorkerConfig struct {
	ID          string
	Concurrency int
}

type NATSConfig struct {
	URL             string
	Stream          string
	Subject         string
	Consumer        string
	ShutdownTimeout time.Duration
}

type DatabaseConfig struct {
	URL string
}

type APIConfig struct {
	URL      string
	Email    string
	Password string
}

type GeminiConfig struct {
	APIKey string
	Model  string // gemini-1.5-flash or gemini-1.5-pro
}

type ElevenLabsConfig struct {
	APIKey  string
	VoiceID string
	Model   string // eleven_v3, eleven_multilingual_v2
}

type ImageSelectorConfig struct {
	PythonPath string // e.g., "python" or "/usr/bin/python3"
	ScriptPath string // e.g., "python/image_selector.py"
	Device     string // "cuda" or "cpu"
}

type StorageConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	PublicURL string
}

type AlertConfig struct {
	Enabled        bool
	DiscordWebhook string
}

func Load() (*Config, error) {
	// Load .env file if exists
	_ = godotenv.Load()

	concurrency, _ := strconv.Atoi(getEnv("WORKER_CONCURRENCY", "2"))
	alertEnabled, _ := strconv.ParseBool(getEnv("ALERT_ENABLED", "false"))

	workerID := getEnv("WORKER_ID", "seo-worker-1")

	return &Config{
		Worker: WorkerConfig{
			ID:          workerID,
			Concurrency: concurrency,
		},
		NATS: NATSConfig{
			URL:             getEnv("NATS_URL", "nats://localhost:4222"),
			Stream:          getEnv("NATS_STREAM", "SEO_ARTICLES"),
			Subject:         "seo.article.generate",
			Consumer:        "seo-worker-" + workerID,
			ShutdownTimeout: 60 * time.Second,
		},
		Database: DatabaseConfig{
			URL: getEnv("DATABASE_URL", ""),
		},
		SuekkAPI: APIConfig{
			URL:      getEnv("SUEKK_API_URL", "https://api.suekk.com"),
			Email:    getEnv("SUEKK_API_EMAIL", ""),
			Password: getEnv("SUEKK_API_PASSWORD", ""),
		},
		SubthAPI: APIConfig{
			URL:      getEnv("SUBTH_API_URL", "https://api.subth.com"),
			Email:    getEnv("SUBTH_API_EMAIL", ""),
			Password: getEnv("SUBTH_API_PASSWORD", ""),
		},
		Gemini: GeminiConfig{
			APIKey: getEnv("GEMINI_API_KEY", ""),
			Model:  getEnv("GEMINI_MODEL", "gemini-1.5-flash"),
		},
		ElevenLabs: ElevenLabsConfig{
			APIKey:  getEnv("ELEVENLABS_API_KEY", ""),
			VoiceID: getEnv("ELEVENLABS_VOICE_ID", "q0IMILNRPxOgtBTS4taI"),
			Model:   getEnv("ELEVENLABS_MODEL", "eleven_v3"),
		},
		// Image Selector (Python) - NSFW filter, face detection, aesthetic scoring
		ImageSelector: ImageSelectorConfig{
			PythonPath: getEnv("IMAGE_SELECTOR_PYTHON", "python"),
			ScriptPath: getEnv("IMAGE_SELECTOR_SCRIPT", "python/image_selector.py"),
			Device:     getEnv("IMAGE_SELECTOR_DEVICE", "cuda"),
		},
		// Suekk Storage (IDrive) - for reading SRT files
		SuekkStorage: StorageConfig{
			Endpoint:  getEnv("SUEKK_STORAGE_ENDPOINT", ""),
			AccessKey: getEnv("SUEKK_STORAGE_ACCESS_KEY", ""),
			SecretKey: getEnv("SUEKK_STORAGE_SECRET_KEY", ""),
			Bucket:    getEnv("SUEKK_STORAGE_BUCKET", "suekk-01"),
			PublicURL: getEnv("SUEKK_STORAGE_PUBLIC_URL", ""),
		},
		// Subth Storage (R2) - for uploading audio files
		SubthStorage: StorageConfig{
			Endpoint:  getEnv("SUBTH_STORAGE_ENDPOINT", ""),
			AccessKey: getEnv("SUBTH_STORAGE_ACCESS_KEY", ""),
			SecretKey: getEnv("SUBTH_STORAGE_SECRET_KEY", ""),
			Bucket:    getEnv("SUBTH_STORAGE_BUCKET", "r2-subth"),
			PublicURL: getEnv("SUBTH_STORAGE_PUBLIC_URL", ""),
		},
		Alert: AlertConfig{
			Enabled:        alertEnabled,
			DiscordWebhook: getEnv("DISCORD_WEBHOOK_URL", ""),
		},
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
