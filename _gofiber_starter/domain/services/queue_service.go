package services

import (
	"context"

	"github.com/google/uuid"
	"gofiber-template/domain/dto"
)

// QueueService interface สำหรับจัดการ queue ทั้งหมด
type QueueService interface {
	// === Stats ===

	// GetQueueStats ดึงสถิติ queue ทั้งหมด
	GetQueueStats(ctx context.Context) (*dto.QueueStatsResponse, error)

	// === Transcode Queue ===

	// GetTranscodeFailed ดึงรายการ video ที่ transcode failed
	GetTranscodeFailed(ctx context.Context, page, limit int) ([]dto.TranscodeQueueItem, int64, error)

	// RetryTranscodeFailed retry video ที่ transcode failed ทั้งหมด
	RetryTranscodeFailed(ctx context.Context) (*dto.RetryResponse, error)

	// RetryTranscodeOne retry video 1 ตัว
	RetryTranscodeOne(ctx context.Context, videoID uuid.UUID) error

	// === Subtitle Queue ===

	// GetSubtitleStuck ดึงรายการ subtitle ที่ค้าง (queued)
	GetSubtitleStuck(ctx context.Context, page, limit int) ([]dto.SubtitleQueueItem, int64, error)

	// GetSubtitleFailed ดึงรายการ subtitle ที่ failed
	GetSubtitleFailed(ctx context.Context, page, limit int) ([]dto.SubtitleQueueItem, int64, error)

	// RetrySubtitleStuck retry subtitle ที่ค้างทั้งหมด (reuse existing logic)
	RetrySubtitleStuck(ctx context.Context) (*dto.RetryResponse, error)

	// ClearSubtitleStuck ลบ subtitle ที่ค้างทั้งหมด + purge NATS queue
	ClearSubtitleStuck(ctx context.Context) (*dto.ClearResponse, error)

	// QueueMissingSubtitles สแกน videos ที่ยังไม่มี subtitle แล้ว queue ใหม่
	QueueMissingSubtitles(ctx context.Context) (*dto.QueueMissingResponse, error)

	// === Warm Cache Queue ===

	// GetWarmCachePending ดึงรายการ video ที่ยังไม่ได้ warm cache
	GetWarmCachePending(ctx context.Context, page, limit int) ([]dto.WarmCacheQueueItem, int64, error)

	// GetWarmCacheFailed ดึงรายการ video ที่ warm cache failed
	GetWarmCacheFailed(ctx context.Context, page, limit int) ([]dto.WarmCacheQueueItem, int64, error)

	// WarmCacheOne warm cache video 1 ตัว
	WarmCacheOne(ctx context.Context, videoID uuid.UUID) (*dto.WarmCacheResponse, error)

	// WarmCacheAll warm cache ทุก video ที่ยังไม่ได้ warm
	WarmCacheAll(ctx context.Context) (*dto.WarmAllResponse, error)

	// === Gallery Queue ===

	// GetGalleryProcessing ดึงรายการ video ที่กำลังสร้าง gallery
	GetGalleryProcessing(ctx context.Context, page, limit int) ([]dto.GalleryQueueItem, int64, error)

	// GetGalleryFailed ดึงรายการ video ที่ gallery failed
	GetGalleryFailed(ctx context.Context, page, limit int) ([]dto.GalleryQueueItem, int64, error)

	// RetryGalleryAll retry gallery ที่ failed ทั้งหมด
	RetryGalleryAll(ctx context.Context) (*dto.RetryResponse, error)

	// === Reel Queue ===

	// GetReelExporting ดึงรายการ reel ที่กำลัง export
	GetReelExporting(ctx context.Context, page, limit int) ([]dto.ReelQueueItem, int64, error)

	// GetReelFailed ดึงรายการ reel ที่ export failed
	GetReelFailed(ctx context.Context, page, limit int) ([]dto.ReelQueueItem, int64, error)

	// RetryReelAll retry reel ที่ failed ทั้งหมด
	RetryReelAll(ctx context.Context) (*dto.RetryResponse, error)
}
