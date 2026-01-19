package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	App      AppConfig
	Database DatabaseConfig
	NATS     NATSConfig // NATS JetStream (แทน Redis/Asynq)
	Redis    RedisConfig
	JWT      JWTConfig
	Log      LogConfig
	Google   GoogleOAuthConfig
	Storage  StorageConfig
	Stream   StreamConfig // Stream cookie และ R2 settings
}

// RedisConfig สำหรับ cache whitelist lookups
type RedisConfig struct {
	URL      string // redis://localhost:6379
	Password string
	DB       int
}

// StreamConfig สำหรับ video streaming security
type StreamConfig struct {
	R2PublicURL string // R2 Public URL (e.g., https://cdn.suekk.com)
	CookieKey   string // Secret key สำหรับ sign cookie (32+ chars)
	CookieDomain string // Domain สำหรับ cookie (e.g., .suekk.com)
	CookieMaxAge int    // Cookie lifetime in seconds (default: 7200 = 2 hours)
}

type AppConfig struct {
	Name string
	Port string
	Env  string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// NATSConfig configuration สำหรับ NATS JetStream
type NATSConfig struct {
	URL string // nats://localhost:4222
}

type JWTConfig struct {
	Secret string
}

type LogConfig struct {
	Level      string // debug, info, warn, error
	Format     string // json, text
	Output     string // stdout, file, both
	FilePath   string // logs/app.log
	MaxSize    int    // MB
	MaxBackups int    // จำนวน backup files
	MaxAge     int    // วัน
	Compress   bool   // บีบอัด backup
}

type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	FrontendURL  string // URL ของ frontend สำหรับ redirect หลัง OAuth
}

type StorageConfig struct {
	Type            string // local, s3
	BasePath        string // สำหรับ local: ./uploads
	BaseURL         string // URL สำหรับเข้าถึงไฟล์ (เช่น http://localhost:8080/files)
	VideoPath       string // เก็บไฟล์วิดีโอ
	TempPath        string // เก็บไฟล์ชั่วคราว
	FFmpegPath      string // path ถึง ffmpeg binary
	MaxUploadSize   int64  // ขนาดสูงสุดที่อัปโหลดได้ (bytes)
	CleanupOriginal bool   // ลบไฟล์ต้นฉบับหลัง transcode

	// Storage Quota (bytes) - 0 = unlimited
	QuotaTotal int64 // จำกัด storage ทั้งระบบ (เช่น 5TB = 5497558138880)

	// Transcoding Settings
	TranscodeQualities []string // ความละเอียดที่ต้องการ ["1080p", "720p", "480p"]

	// CDN/Cloudflare Worker สำหรับ HLS streaming
	CDNBaseURL string // URL ของ Cloudflare Worker (เช่น https://hls.yourdomain.com)

	// S3-Compatible Storage (MinIO / Cloudflare R2)
	S3 S3Config
}

type S3Config struct {
	Endpoint        string // minio:9000 หรือ xxx.r2.cloudflarestorage.com
	AccessKey       string
	SecretKey       string
	Bucket          string
	UseSSL          bool   // false สำหรับ MinIO local, true สำหรับ R2
	Region          string // auto สำหรับ R2
	PublicURL       string // URL สำหรับเข้าถึงไฟล์ public (optional)
}

func LoadConfig() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		// ไม่ error ถ้าไม่มี .env file (ใช้ environment variables แทน)
	}

	logMaxSize, _ := strconv.Atoi(getEnv("LOG_MAX_SIZE", "100"))
	logMaxBackups, _ := strconv.Atoi(getEnv("LOG_MAX_BACKUPS", "5"))
	logMaxAge, _ := strconv.Atoi(getEnv("LOG_MAX_AGE", "30"))
	logCompress := getEnv("LOG_COMPRESS", "true") == "true"

	maxUploadSize, _ := strconv.ParseInt(getEnv("STORAGE_MAX_UPLOAD_SIZE", "5368709120"), 10, 64) // 5GB default
	cleanupOriginal := getEnv("STORAGE_CLEANUP_ORIGINAL", "true") == "true"
	quotaTotal, _ := strconv.ParseInt(getEnv("STORAGE_QUOTA_TOTAL", "0"), 10, 64) // 0 = unlimited
	s3UseSSL := getEnv("S3_USE_SSL", "false") == "true"
	transcodeQualities := parseQualities(getEnv("TRANSCODE_QUALITIES", "1080p,720p,480p"))

	// Redis config
	redisDB, _ := strconv.Atoi(getEnv("REDIS_DB", "0"))

	// Stream cookie config
	cookieMaxAge, _ := strconv.Atoi(getEnv("STREAM_COOKIE_MAX_AGE", "7200")) // 2 hours default

	config := &Config{
		App: AppConfig{
			Name: getEnv("APP_NAME", "Suekk Stream"),
			Port: getEnv("APP_PORT", "8080"),
			Env:  getEnv("APP_ENV", "development"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			DBName:   getEnv("DB_NAME", "suekk_stream"),
			SSLMode:  getEnv("DB_SSL_MODE", "disable"),
		},
		NATS: NATSConfig{
			URL: getEnv("NATS_URL", "nats://localhost:4222"),
		},
		Redis: RedisConfig{
			URL:      getEnv("REDIS_URL", "redis://localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       redisDB,
		},
		Stream: StreamConfig{
			R2PublicURL:  getEnv("STREAM_R2_PUBLIC_URL", getEnv("S3_PUBLIC_URL", "")), // fallback to S3_PUBLIC_URL
			CookieKey:    getEnv("STREAM_COOKIE_KEY", "change-this-to-a-secure-32-char-key"),
			CookieDomain: getEnv("STREAM_COOKIE_DOMAIN", ".suekk.com"),
			CookieMaxAge: cookieMaxAge,
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", "your-secret-key"),
		},
		Log: LogConfig{
			Level:      getEnv("LOG_LEVEL", "info"),
			Format:     getEnv("LOG_FORMAT", "json"),
			Output:     getEnv("LOG_OUTPUT", "both"),
			FilePath:   getEnv("LOG_FILE", "logs/app.log"),
			MaxSize:    logMaxSize,
			MaxBackups: logMaxBackups,
			MaxAge:     logMaxAge,
			Compress:   logCompress,
		},
		Google: GoogleOAuthConfig{
			ClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
			ClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
			RedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/v1/auth/google/callback"),
			FrontendURL:  getEnv("FRONTEND_URL", "http://localhost:5173"),
		},
		Storage: StorageConfig{
			Type:               getEnv("STORAGE_TYPE", "local"),
			BasePath:           getEnv("STORAGE_BASE_PATH", "./uploads"),
			BaseURL:            getEnv("STORAGE_BASE_URL", "http://localhost:8080/files"),
			VideoPath:          getEnv("STORAGE_VIDEO_PATH", "./videos"),
			TempPath:           getEnv("STORAGE_TEMP_PATH", "./temp"),
			FFmpegPath:         getEnv("FFMPEG_PATH", "ffmpeg"),
			MaxUploadSize:      maxUploadSize,
			CleanupOriginal:    cleanupOriginal,
			QuotaTotal:         quotaTotal,
			TranscodeQualities: transcodeQualities,
			CDNBaseURL:         getEnv("CDN_BASE_URL", ""), // Cloudflare Worker URL
			S3: S3Config{
				Endpoint:  getEnv("S3_ENDPOINT", "localhost:9000"),
				AccessKey: getEnv("S3_ACCESS_KEY", "minioadmin"),
				SecretKey: getEnv("S3_SECRET_KEY", "minioadmin"),
				Bucket:    getEnv("S3_BUCKET", "videos"),
				UseSSL:    s3UseSSL,
				Region:    getEnv("S3_REGION", "auto"),
				PublicURL: getEnv("S3_PUBLIC_URL", ""),
			},
		},
	}

	return config, nil
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// parseQualities แปลง comma-separated string เป็น slice
// เช่น "1080p,720p,480p" -> ["1080p", "720p", "480p"]
func parseQualities(s string) []string {
	if s == "" {
		return []string{"1080p", "720p", "480p"}
	}
	parts := strings.Split(s, ",")
	var qualities []string
	for _, p := range parts {
		q := strings.TrimSpace(p)
		if q != "" {
			qualities = append(qualities, q)
		}
	}
	if len(qualities) == 0 {
		return []string{"1080p", "720p", "480p"}
	}
	return qualities
}

// IsDevelopment ตรวจสอบว่าเป็น development mode
func (c *Config) IsDevelopment() bool {
	return c.App.Env == "development"
}

// IsProduction ตรวจสอบว่าเป็น production mode
func (c *Config) IsProduction() bool {
	return c.App.Env == "production"
}
