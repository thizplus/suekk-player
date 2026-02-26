package ports

import (
	"context"

	"seo-worker/domain/models"
)

// SRTFetcherPort - Interface สำหรับดึง SRT จาก api.suekk.com
type SRTFetcherPort interface {
	// FetchSRT ดึง SRT content
	// หมายเหตุ: SRT ต้องมีอยู่แล้ว (pre-validated ที่ Admin UI)
	FetchSRT(ctx context.Context, videoCode string) (string, error)
}

// SuekkVideoFetcherPort - Interface สำหรับดึง Video Info จาก api.suekk.com
type SuekkVideoFetcherPort interface {
	// FetchVideoInfo ดึงข้อมูล video (duration, gallery)
	FetchVideoInfo(ctx context.Context, videoCode string) (*models.SuekkVideoInfo, error)

	// ListGalleryImages ดึงรายการ gallery images จาก storage (safe priority)
	ListGalleryImages(ctx context.Context, galleryPath string) ([]string, error)

	// ListAllGalleryImages ดึงรายการ gallery images จากทุก tier (safe, nsfw)
	ListAllGalleryImages(ctx context.Context, galleryPath string) (*models.TieredGalleryImages, error)
}

// ImageSelectorPort - Interface สำหรับเลือกภาพ cover และ gallery ที่เหมาะสม
type ImageSelectorPort interface {
	// SelectImages คัดเลือกภาพที่เหมาะสมจาก gallery
	// - กรอง NSFW ออก
	// - เลือก cover ที่เห็นหน้าชัด
	// - เลือก gallery 12-15 ภาพที่หลากหลาย
	SelectImages(ctx context.Context, imageURLs []string) (*models.ImageSelectionResult, error)
}

// MetadataFetcherPort - Interface สำหรับดึง Metadata จาก api.subth.com
type MetadataFetcherPort interface {
	// FetchVideoMetadataByCode ดึงข้อมูล video โดยใช้ video code (embed code)
	FetchVideoMetadataByCode(ctx context.Context, videoCode string) (*models.VideoMetadata, error)

	// FetchCasts ดึงข้อมูล casts
	FetchCasts(ctx context.Context, castIDs []string) ([]models.CastMetadata, error)

	// FetchMaker ดึงข้อมูล maker
	FetchMaker(ctx context.Context, makerID string) (*models.MakerMetadata, error)

	// FetchTags ดึงข้อมูล tags
	FetchTags(ctx context.Context, tagIDs []string) ([]models.TagMetadata, error)

	// FetchPreviousWorks ดึงผลงานก่อนหน้าของ cast (จาก articles ที่ publish แล้ว)
	// ใช้ castSlug (ไม่ใช่ ID) เพื่อเรียก /api/v1/articles/cast/{slug}
	FetchPreviousWorks(ctx context.Context, castSlug string, limit int) ([]models.PreviousWork, error)

	// FetchGalleryImages ดึงข้อมูล gallery
	FetchGalleryImages(ctx context.Context, videoID string) ([]models.GalleryImage, error)
}
