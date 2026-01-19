package services

import (
	"context"
	"mime/multipart"

	"github.com/google/uuid"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
)

type VideoService interface {
	// Upload อัปโหลดวิดีโอใหม่ (ผ่าน Backend)
	Upload(ctx context.Context, userID uuid.UUID, file *multipart.FileHeader, req *dto.CreateVideoRequest) (*models.Video, error)

	// CreateVideo สร้าง video record โดยไม่ upload (สำหรับ Direct Upload)
	CreateVideo(ctx context.Context, video *models.Video) error

	// GetByID ดึง video ตาม ID
	GetByID(ctx context.Context, id uuid.UUID) (*models.Video, error)

	// GetByCode ดึง video ตาม code (สำหรับ embed)
	GetByCode(ctx context.Context, code string) (*models.Video, error)

	// GetUserVideos ดึง videos ของ user
	GetUserVideos(ctx context.Context, userID uuid.UUID, page, limit int) ([]*models.Video, int64, error)

	// GetByCategory ดึง videos ตาม category
	GetByCategory(ctx context.Context, categoryID uuid.UUID, page, limit int) ([]*models.Video, int64, error)

	// ListVideos ดึง videos ทั้งหมด (admin)
	ListVideos(ctx context.Context, page, limit int) ([]*models.Video, int64, error)

	// ListWithFilters ดึง videos พร้อม filter, search, sort, pagination
	ListWithFilters(ctx context.Context, params *dto.VideoFilterRequest) ([]*models.Video, int64, error)

	// ListVideosByStatus ดึง videos ตาม status (pending, processing, ready, failed)
	ListVideosByStatus(ctx context.Context, status models.VideoStatus, page, limit int) ([]*models.Video, int64, error)

	// ListReadyVideos ดึงเฉพาะ videos ที่พร้อม stream
	ListReadyVideos(ctx context.Context, page, limit int) ([]*models.Video, int64, error)

	// Update อัปเดต metadata
	Update(ctx context.Context, id uuid.UUID, req *dto.UpdateVideoRequest) (*models.Video, error)

	// Delete ลบ video
	Delete(ctx context.Context, id uuid.UUID) error

	// IncrementViews เพิ่มยอดวิว
	IncrementViews(ctx context.Context, id uuid.UUID) error

	// GetStats ดึง stats (สำหรับ dashboard)
	GetStats(ctx context.Context) (*VideoStats, error)

	// GetStuckVideos ดึง videos ที่ค้างสถานะ pending/processing นานเกินกำหนด
	GetStuckVideos(ctx context.Context, minutes int) ([]*models.Video, error)

	// UpdateVideoStatus อัปเดต status ของ video
	UpdateVideoStatus(ctx context.Context, id uuid.UUID, status models.VideoStatus) error

	// ResetVideoForRetry reset video สำหรับ retry จาก DLQ (ล้าง retry_count และ last_error)
	ResetVideoForRetry(ctx context.Context, id uuid.UUID) error

	// DeleteAll ลบ videos ทั้งหมด (สำหรับ testing)
	DeleteAll(ctx context.Context) (int64, error)

	// Storage Quota
	// CheckStorageQuota ตรวจสอบว่ายังอัพโหลดได้หรือไม่ (current_used < quota)
	CheckStorageQuota(ctx context.Context) error
	// GetStorageUsage ดึงข้อมูล storage usage
	GetStorageUsage(ctx context.Context) (*StorageUsage, error)
}

type VideoStats struct {
	TotalVideos      int64 `json:"totalVideos"`
	PendingVideos    int64 `json:"pendingVideos"`
	QueuedVideos     int64 `json:"queuedVideos"`     // รอคิว - job อยู่ใน NATS queue
	ProcessingVideos int64 `json:"processingVideos"`
	ReadyVideos      int64 `json:"readyVideos"`
	FailedVideos     int64 `json:"failedVideos"`
	DeadLetterVideos int64 `json:"deadLetterVideos"` // Poison pill - ต้องตรวจสอบ manual
}

// StorageUsage ข้อมูล storage usage
type StorageUsage struct {
	TotalUsed    int64   `json:"totalUsed"`    // Total storage used (bytes)
	TotalQuota   int64   `json:"totalQuota"`   // Total quota (0 = unlimited)
	TotalPercent float64 `json:"totalPercent"` // Usage percentage
	Unlimited    bool    `json:"unlimited"`    // true ถ้า quota = 0
}
