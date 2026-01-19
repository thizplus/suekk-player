package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Config สำหรับ logger
type Config struct {
	Level      string // debug, info, warn, error
	Format     string // json, text
	Output     string // stdout, file, both
	FilePath   string // logs/app.log
	MaxSize    int    // MB
	MaxBackups int    // จำนวน backup files
	MaxAge     int    // วัน
	Compress   bool   // บีบอัด backup
}

// DefaultConfig ค่า default
func DefaultConfig() Config {
	return Config{
		Level:      "info",
		Format:     "json",
		Output:     "both",
		FilePath:   "logs/app.log",
		MaxSize:    100,
		MaxBackups: 5,
		MaxAge:     30,
		Compress:   true,
	}
}

// context key สำหรับ request ID
type contextKey string

const RequestIDKey contextKey = "request_id"

var defaultLogger *slog.Logger

// Init สร้าง logger จาก config
func Init(cfg Config) error {
	// Parse log level
	level := parseLevel(cfg.Level)

	// สร้าง writers ตาม output config
	writers := []io.Writer{}

	if cfg.Output == "stdout" || cfg.Output == "both" {
		writers = append(writers, os.Stdout)
	}

	if cfg.Output == "file" || cfg.Output == "both" {
		// สร้าง directory ถ้าไม่มี
		dir := filepath.Dir(cfg.FilePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}

		// สร้าง lumberjack logger สำหรับ rotation
		fileWriter := &lumberjack.Logger{
			Filename:   cfg.FilePath,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
		}
		writers = append(writers, fileWriter)
	}

	// รวม writers
	var writer io.Writer
	if len(writers) == 1 {
		writer = writers[0]
	} else {
		writer = io.MultiWriter(writers...)
	}

	// สร้าง handler ตาม format
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true, // เปิด source เพื่อดูว่า log มาจากไฟล์/บรรทัดไหน
	}

	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		handler = slog.NewTextHandler(writer, opts)
	}

	// สร้าง logger
	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)

	return nil
}

// parseLevel แปลง string เป็น slog.Level
func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// GetLogger return default logger
func GetLogger() *slog.Logger {
	if defaultLogger == nil {
		// fallback ถ้ายังไม่ได้ init
		return slog.Default()
	}
	return defaultLogger
}

// WithRequestID สร้าง logger ที่มี request ID
func WithRequestID(ctx context.Context) *slog.Logger {
	logger := GetLogger()
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return logger.With("request_id", requestID)
	}
	return logger
}

// ContextWithRequestID ใส่ request ID ลงใน context
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// GetRequestID ดึง request ID จาก context
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// ========== Convenience functions ==========

// Debug log debug level
func Debug(msg string, args ...any) {
	GetLogger().Debug(msg, args...)
}

// Info log info level
func Info(msg string, args ...any) {
	GetLogger().Info(msg, args...)
}

// Warn log warn level
func Warn(msg string, args ...any) {
	GetLogger().Warn(msg, args...)
}

// Error log error level
func Error(msg string, args ...any) {
	GetLogger().Error(msg, args...)
}

// ========== Context-aware functions ==========

// DebugContext log debug with context (request ID)
func DebugContext(ctx context.Context, msg string, args ...any) {
	WithRequestID(ctx).Debug(msg, args...)
}

// InfoContext log info with context (request ID)
func InfoContext(ctx context.Context, msg string, args ...any) {
	WithRequestID(ctx).Info(msg, args...)
}

// WarnContext log warn with context (request ID)
func WarnContext(ctx context.Context, msg string, args ...any) {
	WithRequestID(ctx).Warn(msg, args...)
}

// ErrorContext log error with context (request ID)
func ErrorContext(ctx context.Context, msg string, args ...any) {
	WithRequestID(ctx).Error(msg, args...)
}
