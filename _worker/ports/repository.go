package ports

import "context"

// ═══════════════════════════════════════════════════════════════════════════════
// VideoRepository - Interface สำหรับ Video Status Management (PostgreSQL)
// ═══════════════════════════════════════════════════════════════════════════════

// VideoCompletedInfo ข้อมูลที่ต้องอัพเดทเมื่อ transcode สำเร็จ
type VideoCompletedInfo struct {
	HLSPath      string            // path ของ HLS files บน storage
	ThumbnailURL string            // URL ของ thumbnail
	DiskUsage    int64             // ขนาดรวมของ HLS files (bytes)
	QualitySizes map[string]int64  // ขนาดแยกตาม quality {"1080p": 123456, ...}
	Duration     int               // ความยาววิดีโอ (วินาที)
	Quality      string            // highest quality ที่มี (e.g. "1080p")
}

type VideoRepository interface {
	// GetStatus ดึง status ปัจจุบันของวิดีโอ
	GetStatus(ctx context.Context, videoID string) (string, error)

	// UpdateProcessingStarted อัพเดทว่าเริ่ม process แล้ว
	UpdateProcessingStarted(ctx context.Context, videoID string) error

	// UpdateGalleryProcessingStarted อัพเดทว่าเริ่ม gallery processing แล้ว
	UpdateGalleryProcessingStarted(ctx context.Context, videoID string) error

	// UpdateCompleted อัพเดทเมื่อ transcode สำเร็จ
	UpdateCompleted(ctx context.Context, videoID string, info *VideoCompletedInfo) error

	// UpdateFailed อัพเดทเมื่อ transcode ล้มเหลว
	UpdateFailed(ctx context.Context, videoID, errorMsg string, retryCount int) error

	// UpdateGallery อัพเดท gallery info หลัง generate สำเร็จ
	UpdateGallery(ctx context.Context, videoID, galleryPath string, galleryCount int) error

	// UpdateGalleryClassified อัพเดท gallery info พร้อม super_safe/safe/nsfw counts (Three-Tier)
	UpdateGalleryClassified(ctx context.Context, videoID, galleryPath string, superSafeCount, safeCount, nsfwCount int) error

	// UpdateGalleryManualSelection อัพเดท gallery info สำหรับ Manual Selection Flow
	// ตั้ง gallery_status = "pending_review" และ gallery_source_count
	UpdateGalleryManualSelection(ctx context.Context, videoID, galleryPath string, sourceCount int) error
}
