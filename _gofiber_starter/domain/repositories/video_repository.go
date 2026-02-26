package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
)

type VideoRepository interface {
	Create(ctx context.Context, video *models.Video) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Video, error)
	GetByCode(ctx context.Context, code string) (*models.Video, error)
	GetByUserID(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*models.Video, error)
	GetByCategory(ctx context.Context, categoryID uuid.UUID, offset, limit int) ([]*models.Video, error)
	GetByStatus(ctx context.Context, status models.VideoStatus, offset, limit int) ([]*models.Video, error)
	Update(ctx context.Context, video *models.Video) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status models.VideoStatus) error
	UpdateHLSPath(ctx context.Context, id uuid.UUID, hlsPath string) error
	ClearOriginalPath(ctx context.Context, id uuid.UUID) error
	IncrementViews(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, offset, limit int) ([]*models.Video, error)
	ListReady(ctx context.Context, offset, limit int) ([]*models.Video, error)
	// ListWithFilters ดึง videos พร้อม filter, search, sort, pagination
	ListWithFilters(ctx context.Context, params *dto.VideoFilterRequest) ([]*models.Video, int64, error)
	Count(ctx context.Context) (int64, error)
	CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
	CountByStatus(ctx context.Context, status models.VideoStatus) (int64, error)
	// GetStuckByStatus ดึง videos ที่ค้างสถานะนานเกิน threshold
	GetStuckByStatus(ctx context.Context, status models.VideoStatus, threshold time.Time) ([]*models.Video, error)
	// GetStuckProcessing ดึง videos ที่ processing_started_at เกิน threshold (สำหรับ fast stuck detection)
	GetStuckProcessing(ctx context.Context, threshold time.Time) ([]*models.Video, error)
	// MarkVideoFailed อัพเดท video เป็น failed พร้อม error message และ increment retry_count
	MarkVideoFailed(ctx context.Context, id uuid.UUID, errorMsg string) error
	// ResetForRetry reset retry_count และ last_error สำหรับ retry จาก DLQ
	ResetForRetry(ctx context.Context, id uuid.UUID) error
	// UpdateProcessingTimestamp อัพเดท processing_started_at เพื่อ reset stuck detection timer
	UpdateProcessingTimestamp(ctx context.Context, id uuid.UUID) error
	// AppendErrorHistory เพิ่ม error record ลงใน error_history
	AppendErrorHistory(ctx context.Context, id uuid.UUID, record models.ErrorRecord) error
	// DeleteAll ลบ videos ทั้งหมด
	DeleteAll(ctx context.Context) (int64, error)

	// Storage Quota Methods
	// GetTotalStorageUsed คำนวณ disk_usage รวมทุก video (bytes)
	GetTotalStorageUsed(ctx context.Context) (int64, error)

	// Gallery Queue Methods
	// GetByGalleryStatus ดึง videos ตาม gallery_status
	GetByGalleryStatus(ctx context.Context, galleryStatus string, offset, limit int) ([]*models.Video, error)
	// CountByGalleryStatus นับ videos ตาม gallery_status
	CountByGalleryStatus(ctx context.Context, galleryStatus string) (int64, error)
	// GetGalleryFailed ดึง videos ที่ gallery failed (status=ready, gallery_status=none, last_error not empty)
	GetGalleryFailed(ctx context.Context, offset, limit int) ([]*models.Video, int64, error)
}
