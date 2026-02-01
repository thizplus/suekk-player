package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gofiber-template/domain/models"
)

// SubtitleRepository interface สำหรับ subtitle operations
type SubtitleRepository interface {
	// Create สร้าง subtitle record ใหม่
	Create(ctx context.Context, subtitle *models.Subtitle) error

	// GetByID ดึง subtitle ตาม ID
	GetByID(ctx context.Context, id uuid.UUID) (*models.Subtitle, error)

	// GetByVideoID ดึง subtitles ทั้งหมดของ video
	GetByVideoID(ctx context.Context, videoID uuid.UUID) ([]*models.Subtitle, error)

	// GetByVideoIDAndLanguage ดึง subtitle ตาม video และ language
	GetByVideoIDAndLanguage(ctx context.Context, videoID uuid.UUID, language string) (*models.Subtitle, error)

	// GetOriginalByVideoID ดึง original subtitle ของ video (type = original)
	GetOriginalByVideoID(ctx context.Context, videoID uuid.UUID) (*models.Subtitle, error)

	// Update อัพเดท subtitle
	Update(ctx context.Context, subtitle *models.Subtitle) error

	// UpdateStatus อัพเดทเฉพาะ status
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.SubtitleStatus, errMsg string) error

	// Delete ลบ subtitle
	Delete(ctx context.Context, id uuid.UUID) error

	// DeleteByVideoID ลบ subtitles ทั้งหมดของ video
	DeleteByVideoID(ctx context.Context, videoID uuid.UUID) error

	// Exists ตรวจสอบว่ามี subtitle อยู่แล้วหรือไม่
	Exists(ctx context.Context, videoID uuid.UUID, language string) (bool, error)

	// CountByVideoID นับจำนวน subtitles ของ video
	CountByVideoID(ctx context.Context, videoID uuid.UUID) (int64, error)

	// GetReadyByVideoID ดึงเฉพาะ subtitles ที่ ready ของ video
	GetReadyByVideoID(ctx context.Context, videoID uuid.UUID) ([]*models.Subtitle, error)

	// === Stuck Detection Methods ===

	// GetByStatus ดึง subtitles ตาม status
	GetByStatus(ctx context.Context, status models.SubtitleStatus) ([]*models.Subtitle, error)

	// GetStuckProcessing หา subtitles ที่ processing นานเกินไป (worker crash)
	GetStuckProcessing(ctx context.Context, threshold time.Time) ([]*models.Subtitle, error)

	// MarkSubtitleFailed mark subtitle as failed พร้อม error message
	MarkSubtitleFailed(ctx context.Context, id uuid.UUID, errorMsg string) error

	// UpdateProcessingStartedAt บันทึกเวลาที่ worker เริ่มทำจริง
	UpdateProcessingStartedAt(ctx context.Context, id uuid.UUID, startedAt time.Time) error
}
