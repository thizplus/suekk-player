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
	Level         string // debug, info, warn, error
	Format        string // json, text
	Output        string // stdout, file, both
	FilePath      string // path to log file
	ErrorFilePath string // path to error log file (WARN + ERROR only)
	MaxSizeMB     int    // max size in MB before rotation
	MaxBackups    int    // max number of old log files
	MaxAgeDays    int    // max days to retain old log files
	Compress      bool   // compress rotated files
}

// DefaultConfig returns default logger config
func DefaultConfig() Config {
	return Config{
		Level:         "info",
		Format:        "json",
		Output:        "both",
		FilePath:      "logs/worker.log",
		ErrorFilePath: "logs/errors.log",
		MaxSizeMB:     100,
		MaxBackups:    5,
		MaxAgeDays:    30,
		Compress:      true,
	}
}

var defaultLogger *slog.Logger

// multiHandler เขียน log ไปหลาย handler พร้อมกัน
type multiHandler struct {
	handlers []slog.Handler
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, r.Level) {
			if err := handler.Handle(ctx, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}

// Init initializes the logger with config
func Init(cfg Config) error {
	// Parse log level
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Setup writers for main log
	var writers []io.Writer

	// Stdout writer
	if cfg.Output == "stdout" || cfg.Output == "both" {
		writers = append(writers, os.Stdout)
	}

	// File writer with rotation
	if cfg.Output == "file" || cfg.Output == "both" {
		// Ensure log directory exists
		logDir := filepath.Dir(cfg.FilePath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return err
		}

		fileWriter := &lumberjack.Logger{
			Filename:   cfg.FilePath,
			MaxSize:    cfg.MaxSizeMB,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAgeDays,
			Compress:   cfg.Compress,
		}
		writers = append(writers, fileWriter)
	}

	// Combine writers
	var writer io.Writer
	if len(writers) == 1 {
		writer = writers[0]
	} else {
		writer = io.MultiWriter(writers...)
	}

	// Create main handler
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug,
	}

	var mainHandler slog.Handler
	if cfg.Format == "text" {
		mainHandler = slog.NewTextHandler(writer, opts)
	} else {
		mainHandler = slog.NewJSONHandler(writer, opts)
	}

	// Create error handler (WARN + ERROR only)
	var handlers []slog.Handler
	handlers = append(handlers, mainHandler)

	if cfg.ErrorFilePath != "" && (cfg.Output == "file" || cfg.Output == "both") {
		// Ensure error log directory exists
		errorLogDir := filepath.Dir(cfg.ErrorFilePath)
		if err := os.MkdirAll(errorLogDir, 0755); err != nil {
			return err
		}

		errorFileWriter := &lumberjack.Logger{
			Filename:   cfg.ErrorFilePath,
			MaxSize:    cfg.MaxSizeMB,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAgeDays,
			Compress:   cfg.Compress,
		}

		errorOpts := &slog.HandlerOptions{
			Level:     slog.LevelWarn, // WARN และ ERROR เท่านั้น
			AddSource: true,           // แสดง source เสมอสำหรับ error log
		}

		var errorHandler slog.Handler
		if cfg.Format == "text" {
			errorHandler = slog.NewTextHandler(errorFileWriter, errorOpts)
		} else {
			errorHandler = slog.NewJSONHandler(errorFileWriter, errorOpts)
		}

		handlers = append(handlers, errorHandler)
	}

	// Set default logger with multi handler
	defaultLogger = slog.New(&multiHandler{handlers: handlers})
	slog.SetDefault(defaultLogger)

	return nil
}

// GetLogger returns the default logger
func GetLogger() *slog.Logger {
	if defaultLogger == nil {
		return slog.Default()
	}
	return defaultLogger
}

// Convenience functions
func Debug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

func Info(msg string, args ...any) {
	slog.Info(msg, args...)
}

func Warn(msg string, args ...any) {
	slog.Warn(msg, args...)
}

func Error(msg string, args ...any) {
	slog.Error(msg, args...)
}
