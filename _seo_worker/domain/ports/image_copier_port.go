package ports

import (
	"context"

	"seo-worker/domain/models"
)

// ImageCopierPort - Interface สำหรับ copy ภาพจาก source storage ไป destination storage
type ImageCopierPort interface {
	// CopyGalleryImages copy ภาพ gallery จาก e2 (suekk) ไป r2 (subth)
	// คืนค่า URLs ใหม่ที่ชี้ไป r2
	CopyGalleryImages(ctx context.Context, videoCode string, images []models.GalleryImage) ([]models.GalleryImage, error)

	// CopyImage copy ภาพเดี่ยว
	// srcURL = URL จาก e2, returns URL ใหม่จาก r2
	CopyImage(ctx context.Context, videoCode string, srcURL string, filename string) (string, error)

	// CopyTieredGallery copy ภาพจากทุก tier ไป r2 แยก path
	// - public/  = safe (admin approved - Google-safe)
	// - member/  = nsfw (admin approved - members only)
	CopyTieredGallery(ctx context.Context, videoCode string, tiered *models.TieredGalleryImages) (*CopiedGalleryResult, error)
}

// CopiedGalleryResult - ผลลัพธ์จาก CopyTieredGallery
type CopiedGalleryResult struct {
	PublicImages []models.GalleryImage // R2 URLs for safe (admin approved)
	MemberImages []models.GalleryImage // R2 URLs for nsfw
	CoverURL     string                // Best cover image URL
}
