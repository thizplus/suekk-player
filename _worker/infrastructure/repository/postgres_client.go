package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/lib/pq"

	"suekk-worker/ports"
)

// ═══════════════════════════════════════════════════════════════════════════════
// PostgresClient - Implementation ของ VideoRepository
// จัดการ video status ใน PostgreSQL database
// ═══════════════════════════════════════════════════════════════════════════════

// PostgresConfig configuration สำหรับ PostgreSQL connection
type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// PostgresClient implementation ของ ports.VideoRepository
type PostgresClient struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewPostgresClient สร้าง PostgresClient ใหม่
func NewPostgresClient(cfg PostgresConfig) (*PostgresClient, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Connection pool settings
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	logger := slog.Default().With("component", "postgres-client")
	logger.Info("database connected",
		"host", cfg.Host,
		"port", cfg.Port,
		"database", cfg.DBName,
	)

	return &PostgresClient{
		db:     db,
		logger: logger,
	}, nil
}

// NewPostgresClientFromDB สร้าง PostgresClient จาก existing db connection
// ใช้สำหรับ backward compatibility กับ code เดิม
func NewPostgresClientFromDB(db *sql.DB) *PostgresClient {
	return &PostgresClient{
		db:     db,
		logger: slog.Default().With("component", "postgres-client"),
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// VideoRepository Implementation
// ─────────────────────────────────────────────────────────────────────────────

// GetStatus ดึง status ปัจจุบันของวิดีโอ
func (p *PostgresClient) GetStatus(ctx context.Context, videoID string) (string, error) {
	if p.db == nil {
		return "", nil
	}

	var status string
	query := `SELECT status FROM videos WHERE id = $1`
	err := p.db.QueryRowContext(ctx, query, videoID).Scan(&status)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("video not found: %s", videoID)
		}
		return "", fmt.Errorf("failed to get video status: %w", err)
	}

	return status, nil
}

// UpdateProcessingStarted อัพเดทว่าเริ่ม process แล้ว
// ตั้ง processing_started_at สำหรับ stuck detection
func (p *PostgresClient) UpdateProcessingStarted(ctx context.Context, videoID string) error {
	if p.db == nil {
		return nil
	}

	query := `UPDATE videos SET status = 'processing', processing_started_at = $1, updated_at = $1 WHERE id = $2`
	result, err := p.db.ExecContext(ctx, query, time.Now(), videoID)
	if err != nil {
		return fmt.Errorf("failed to update processing started: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		p.logger.Warn("no video updated for processing started", "video_id", videoID)
	}

	return nil
}

// UpdateCompleted อัพเดทเมื่อ transcode สำเร็จ
func (p *PostgresClient) UpdateCompleted(ctx context.Context, videoID string, info *ports.VideoCompletedInfo) error {
	if p.db == nil {
		return nil
	}

	// Convert qualitySizes to JSON
	qualitySizesJSON := "{}"
	if len(info.QualitySizes) > 0 {
		if jsonBytes, err := json.Marshal(info.QualitySizes); err == nil {
			qualitySizesJSON = string(jsonBytes)
		}
	}

	// Clear processing_started_at เมื่อเสร็จสมบูรณ์
	// Update disk_usage, hls_size, quality_sizes, duration และ quality
	// Set needs_retranscode = false เมื่อ transcode เสร็จ (สำหรับ batch re-transcode)
	query := `UPDATE videos SET
		status = $1,
		hls_path = $2,
		thumbnail_url = $3,
		disk_usage = $4,
		hls_size = $4,
		quality_sizes = $5,
		duration = $6,
		quality = $7,
		needs_retranscode = false,
		processing_started_at = NULL,
		updated_at = $8
		WHERE id = $9`

	result, err := p.db.ExecContext(ctx, query,
		"ready",
		info.HLSPath,
		info.ThumbnailURL,
		info.DiskUsage,
		qualitySizesJSON,
		info.Duration,
		info.Quality,
		time.Now(),
		videoID,
	)
	if err != nil {
		return fmt.Errorf("failed to update video completed: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		p.logger.Warn("no video updated for completed", "video_id", videoID)
	} else {
		p.logger.Info("video marked as completed",
			"video_id", videoID,
			"hls_path", info.HLSPath,
			"disk_usage", info.DiskUsage,
			"quality", info.Quality,
		)
	}

	return nil
}

// UpdateFailed อัพเดทเมื่อ transcode ล้มเหลว
func (p *PostgresClient) UpdateFailed(ctx context.Context, videoID, errorMsg string, retryCount int) error {
	if p.db == nil {
		return nil
	}

	// ถ้า retry เกิน 3 ครั้ง → dead_letter
	status := "failed"
	if retryCount >= 3 {
		status = "dead_letter"
		p.logger.Warn("video moved to dead letter queue",
			"video_id", videoID,
			"retry_count", retryCount,
		)
	}

	query := `UPDATE videos SET
		status = $1,
		last_error = $2,
		retry_count = $3,
		processing_started_at = NULL,
		updated_at = $4
		WHERE id = $5`

	result, err := p.db.ExecContext(ctx, query, status, errorMsg, retryCount, time.Now(), videoID)
	if err != nil {
		return fmt.Errorf("failed to update video failed: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		p.logger.Warn("no video updated for failed", "video_id", videoID)
	}

	return nil
}

// UpdateGallery อัพเดท gallery info หลัง generate สำเร็จ
func (p *PostgresClient) UpdateGallery(ctx context.Context, videoID, galleryPath string, galleryCount int) error {
	if p.db == nil {
		return nil
	}

	query := `UPDATE videos
		SET gallery_path = $1, gallery_count = $2, updated_at = NOW()
		WHERE id = $3`

	result, err := p.db.ExecContext(ctx, query, galleryPath, galleryCount, videoID)
	if err != nil {
		return fmt.Errorf("failed to update gallery: %w", err)
	}

	rows, _ := result.RowsAffected()
	p.logger.Info("gallery updated",
		"video_id", videoID,
		"gallery_path", galleryPath,
		"gallery_count", galleryCount,
		"rows", rows,
	)

	return nil
}

// UpdateGalleryProcessingStarted อัพเดทว่าเริ่ม gallery processing แล้ว
// ตั้ง gallery_status = 'processing'
func (p *PostgresClient) UpdateGalleryProcessingStarted(ctx context.Context, videoID string) error {
	if p.db == nil {
		return nil
	}

	query := `UPDATE videos SET gallery_status = 'processing', updated_at = NOW() WHERE id = $1`
	result, err := p.db.ExecContext(ctx, query, videoID)
	if err != nil {
		return fmt.Errorf("failed to update gallery processing started: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		p.logger.Warn("no video updated for gallery processing started", "video_id", videoID)
	}

	p.logger.Info("gallery processing started", "video_id", videoID, "gallery_status", "processing")
	return nil
}

// UpdateGalleryClassified อัพเดท gallery info พร้อม super_safe/safe/nsfw counts (Three-Tier)
// gallery_count = super_safe + safe (public-accessible images for backward compatibility)
// gallery_status = "pending_review" เพื่อให้ Admin ตรวจสอบก่อน publish
func (p *PostgresClient) UpdateGalleryClassified(ctx context.Context, videoID, galleryPath string, superSafeCount, safeCount, nsfwCount int) error {
	if p.db == nil {
		return nil
	}

	// gallery_count = super_safe + safe (public-accessible, ไม่รวม nsfw)
	totalCount := superSafeCount + safeCount
	// gallery_source_count = total ภาพที่สร้างมา (ก่อน classify)
	sourceCount := superSafeCount + safeCount + nsfwCount

	// อัพเดททั้ง columns ใหม่และ gallery_count สำหรับ backward compatibility
	// ตั้ง gallery_status = "pending_review" เพื่อให้ปุ่มจัดการ Gallery แสดง
	query := `UPDATE videos
		SET gallery_path = $1,
			gallery_count = $2,
			gallery_super_safe_count = $3,
			gallery_safe_count = $4,
			gallery_nsfw_count = $5,
			gallery_source_count = $6,
			gallery_status = 'pending_review',
			updated_at = NOW()
		WHERE id = $7`

	result, err := p.db.ExecContext(ctx, query, galleryPath, totalCount, superSafeCount, safeCount, nsfwCount, sourceCount, videoID)
	if err != nil {
		return fmt.Errorf("failed to update gallery classified: %w", err)
	}

	rows, _ := result.RowsAffected()
	p.logger.Info("gallery classified updated (three-tier)",
		"video_id", videoID,
		"gallery_path", galleryPath,
		"gallery_status", "pending_review",
		"source_count", sourceCount,
		"super_safe_count", superSafeCount,
		"safe_count", safeCount,
		"nsfw_count", nsfwCount,
		"total_count", totalCount,
		"rows", rows,
	)

	return nil
}

// UpdateGalleryManualSelection อัพเดท gallery info สำหรับ Manual Selection Flow
// ตั้ง gallery_status = "pending_review" และ gallery_source_count
func (p *PostgresClient) UpdateGalleryManualSelection(ctx context.Context, videoID, galleryPath string, sourceCount int) error {
	if p.db == nil {
		return nil
	}

	// Manual Selection Flow:
	// - gallery_source_count = จำนวนภาพใน source/ folder
	// - gallery_count = 0 (ยังไม่มี Admin เลือก)
	// - gallery_safe_count = 0
	// - gallery_nsfw_count = 0
	// - gallery_status = "pending_review"
	query := `UPDATE videos
		SET gallery_path = $1,
			gallery_source_count = $2,
			gallery_count = 0,
			gallery_super_safe_count = 0,
			gallery_safe_count = 0,
			gallery_nsfw_count = 0,
			gallery_status = 'pending_review',
			updated_at = NOW()
		WHERE id = $3`

	result, err := p.db.ExecContext(ctx, query, galleryPath, sourceCount, videoID)
	if err != nil {
		return fmt.Errorf("failed to update gallery manual selection: %w", err)
	}

	rows, _ := result.RowsAffected()
	p.logger.Info("gallery manual selection updated",
		"video_id", videoID,
		"gallery_path", galleryPath,
		"gallery_status", "pending_review",
		"source_count", sourceCount,
		"rows", rows,
	)

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Additional Methods
// ─────────────────────────────────────────────────────────────────────────────

// GetDB returns underlying database connection
// ใช้สำหรับ backward compatibility
func (p *PostgresClient) GetDB() *sql.DB {
	return p.db
}

// Close ปิด database connection
func (p *PostgresClient) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// Ping ทดสอบ connection
func (p *PostgresClient) Ping(ctx context.Context) error {
	if p.db == nil {
		return fmt.Errorf("database not connected")
	}
	return p.db.PingContext(ctx)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper Functions
// ─────────────────────────────────────────────────────────────────────────────

// getHighestQuality returns the highest quality from qualitySizes map
// Priority: 1080p > 720p > 480p > 360p
func getHighestQuality(qualitySizes map[string]int64) string {
	priorities := []string{"1080p", "720p", "480p", "360p"}
	for _, q := range priorities {
		if _, exists := qualitySizes[q]; exists {
			return q
		}
	}
	// Fallback: return first key if any
	for q := range qualitySizes {
		return q
	}
	return ""
}

// Ensure PostgresClient implements VideoRepository
var _ ports.VideoRepository = (*PostgresClient)(nil)
