# Sprint 5: SEO Worker - ใช้ Gallery ที่ Classify ไว้แล้ว

## Overview

เปลี่ยน SEO Worker จากการโหลดภาพทั้งหมดแล้วมา classify ที่ runtime
เป็นการใช้ภาพจาก `gallery/{code}/safe/` โดยตรง ซึ่ง Worker (_worker) ได้ classify ไว้แล้วตอน generate gallery

## ประโยชน์
- **เร็วขึ้น**: ไม่ต้องโหลดภาพ NSFW มา แล้ว classify ที่ runtime
- **ประหยัด GPU**: ไม่ต้องรัน NSFW detection model ที่ SEO Worker
- **เสถียร**: ใช้ผลลัพธ์เดียวกันทุกครั้ง (deterministic)

---

## สถานะปัจจุบัน

### API Response (api.suekk.com)
```json
{
  "success": true,
  "data": {
    "code": "ABC123",
    "galleryPath": "gallery/ABC123",
    "galleryCount": 45,
    "gallerySafeCount": 32,      // ✅ เพิ่มใหม่ (Sprint 4)
    "galleryNsfwCount": 13       // ✅ เพิ่มใหม่ (Sprint 4)
  }
}
```

### Storage Structure (R2)
```
gallery/
└── ABC123/
    ├── safe/           # ✅ ภาพ SFW (ใช้ตรงๆ)
    │   ├── frame_001.jpg
    │   ├── frame_005.jpg
    │   └── ...
    ├── nsfw/           # ภาพ NSFW (ไม่ใช้)
    │   └── ...
    ├── frame_001.jpg   # Legacy (อาจมีใน videos เก่า)
    └── ...
```

---

## งานที่ต้องทำ

### Task 1: Update `SuekkVideoInfo` Model

**ไฟล์**: `domain/models/article.go`

```go
// SuekkVideoInfo - ข้อมูล video จาก api.suekk.com
type SuekkVideoInfo struct {
	Code             string `json:"code"`
	Duration         int    `json:"duration"`
	ThumbnailURL     string `json:"thumbnailUrl"`
	GalleryPath      string `json:"galleryPath"`
	GalleryCount     int    `json:"galleryCount"`
	GallerySafeCount int    `json:"gallerySafeCount"`  // ✅ เพิ่มใหม่
	GalleryNsfwCount int    `json:"galleryNsfwCount"`  // ✅ เพิ่มใหม่
}
```

---

### Task 2: Update Fetcher Response Struct

**ไฟล์**: `infrastructure/fetcher/suekk_video_fetcher.go`

```go
type suekkVideoResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Code             string `json:"code"`
		Duration         int    `json:"duration"`
		ThumbnailURL     string `json:"thumbnailUrl"`
		GalleryPath      string `json:"galleryPath"`
		GalleryCount     int    `json:"galleryCount"`
		GallerySafeCount int    `json:"gallerySafeCount"`  // ✅ เพิ่มใหม่
		GalleryNsfwCount int    `json:"galleryNsfwCount"`  // ✅ เพิ่มใหม่
	} `json:"data"`
	Error string `json:"error,omitempty"`
}
```

อัพเดต `FetchVideoInfo()` ให้ map ค่าใหม่:
```go
return &models.SuekkVideoInfo{
	Code:             result.Data.Code,
	Duration:         result.Data.Duration,
	ThumbnailURL:     result.Data.ThumbnailURL,
	GalleryPath:      result.Data.GalleryPath,
	GalleryCount:     result.Data.GalleryCount,
	GallerySafeCount: result.Data.GallerySafeCount,  // ✅ เพิ่มใหม่
	GalleryNsfwCount: result.Data.GalleryNsfwCount,  // ✅ เพิ่มใหม่
}, nil
```

---

### Task 3: Update `ListGalleryImages()` - ใช้ `/safe/` subfolder

**ไฟล์**: `infrastructure/fetcher/suekk_video_fetcher.go`

**Before**:
```go
func (f *SuekkVideoFetcher) ListGalleryImages(ctx context.Context, galleryPath string) ([]string, error) {
	// List files from storage (ทั้งหมด)
	files, err := f.storage.ListFiles(galleryPath)
	// ...
}
```

**After**:
```go
func (f *SuekkVideoFetcher) ListGalleryImages(ctx context.Context, galleryPath string) ([]string, error) {
	if galleryPath == "" {
		return nil, nil
	}

	// ใช้ safe subfolder เท่านั้น (pre-classified by _worker)
	safePath := galleryPath + "/safe"

	// List files from storage
	files, err := f.storage.ListFiles(safePath)
	if err != nil {
		// Fallback: ถ้าไม่มี /safe subfolder (legacy videos) ใช้ path เดิม
		f.logger.WarnContext(ctx, "Safe gallery not found, falling back to main gallery",
			"safe_path", safePath,
			"error", err,
		)
		files, err = f.storage.ListFiles(galleryPath)
		if err != nil {
			return nil, err
		}
	}

	// Filter only image files and build presigned URLs
	var imageURLs []string
	for _, file := range files {
		if isImageFile(file) {
			url, err := f.storage.GetPresignedDownloadURL(file, galleryURLExpiry)
			if err != nil {
				continue
			}
			imageURLs = append(imageURLs, url)
		}
	}

	f.logger.InfoContext(ctx, "Gallery images listed (safe only)",
		"path", safePath,
		"count", len(imageURLs),
	)

	return imageURLs, nil
}
```

---

### Task 4: Simplify SEO Handler - ไม่ต้อง classify ที่ runtime

**ไฟล์**: `use_cases/seo_handler.go`

**Before** (Lines 137-204):
```go
if suekkVideoInfo.GalleryPath != "" {
	imageURLs, err := h.suekkVideoFetcher.ListGalleryImages(ctx, suekkVideoInfo.GalleryPath)
	// ...

	// Image Selector: คัดเลือกภาพที่เหมาะสม (กรอง NSFW) ⬅️ ไม่จำเป็นแล้ว
	if h.imageSelector != nil && len(imageURLs) > 0 && !skipImageSelector {
		selectionResult, err := h.imageSelector.SelectImages(ctx, imageURLs)
		// ... complex logic
	}
}
```

**After**:
```go
if suekkVideoInfo.GalleryPath != "" {
	// ดึงเฉพาะภาพ safe (pre-classified by _worker)
	imageURLs, err := h.suekkVideoFetcher.ListGalleryImages(ctx, suekkVideoInfo.GalleryPath)
	if err != nil {
		h.logger.WarnContext(ctx, "Failed to list gallery images",
			"gallery_path", suekkVideoInfo.GalleryPath,
			"error", err,
		)
	} else {
		h.logger.InfoContext(ctx, "Gallery images fetched (safe only)",
			"count", len(imageURLs),
			"expected", suekkVideoInfo.GallerySafeCount,
		)
	}

	// ไม่ต้อง classify - ภาพใน /safe/ เป็น SFW ทั้งหมดแล้ว
	for _, url := range imageURLs {
		galleryImages = append(galleryImages, models.GalleryImage{
			URL: url,
		})
	}

	// Cover image: ใช้ภาพแรก (TODO: อาจเพิ่ม cover selection ในอนาคต)
	if len(galleryImages) > 0 {
		coverImage = &models.ImageScore{
			URL:   galleryImages[0].URL,
			IsSafe: true,
		}
	}
}
```

---

### Task 5: Optional Cleanup

1. **ลบหรือ disable ImageSelector** (ถ้าไม่ใช้ที่อื่น):
   - `infrastructure/imageselector/python_selector.go`
   - `python/image_selector.py` และ dependencies

2. **ลบ ImageSelectorPort** จาก `domain/ports/fetcher_port.go` (ถ้าไม่ใช้)

3. **อัพเดต Container** - ไม่ต้อง inject ImageSelector เข้า SEOHandler

---

## Testing Checklist

- [ ] API `/videos/code/{code}` return `gallerySafeCount` และ `galleryNsfwCount` ถูกต้อง
- [ ] `ListGalleryImages()` list จาก `/safe/` subfolder
- [ ] Fallback ทำงานถูกต้องสำหรับ videos เก่าที่ไม่มี `/safe/` subfolder
- [ ] SEO article ใช้แต่ภาพ safe เท่านั้น
- [ ] Processing time ลดลง (ไม่ต้องรัน Python NSFW classifier)

---

## Rollback Plan

ถ้ามีปัญหา:
1. Revert `ListGalleryImages()` กลับไปใช้ `galleryPath` ตรงๆ
2. Enable `imageSelector` กลับมา
3. Videos ใหม่จะยังมี `/safe/` subfolder ไว้ใช้ในอนาคต

---

## Dependencies

- **Sprint 4 Complete**: `gallery_safe_count` และ `gallery_nsfw_count` columns added to DB
- **_worker Updated**: Gallery generator สร้าง `/safe/` และ `/nsfw/` subfolders
- **API Updated**: `/videos/code/{code}` return new fields

---

*Sprint 5 - SEO Worker Pre-classified Gallery*
*Last updated: 2026-02-23*
