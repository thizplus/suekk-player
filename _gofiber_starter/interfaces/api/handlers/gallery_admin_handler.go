package handlers

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"gofiber-template/domain/dto"
	"gofiber-template/domain/ports"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/utils"
)

// GalleryAdminHandler จัดการ gallery สำหรับ Admin
// ใช้ manual selection flow: source → safe/nsfw
type GalleryAdminHandler struct {
	videoService services.VideoService
	storage      ports.StoragePort
}

func NewGalleryAdminHandler(videoService services.VideoService, storage ports.StoragePort) *GalleryAdminHandler {
	return &GalleryAdminHandler{
		videoService: videoService,
		storage:      storage,
	}
}

// === Request/Response DTOs ===

// GalleryImage ข้อมูลภาพใน gallery
type GalleryImage struct {
	Filename string `json:"filename"`
	URL      string `json:"url"`    // Presigned URL
	Folder   string `json:"folder"` // source, safe, nsfw
}

// GalleryImagesResponse รายการภาพทั้งหมดใน gallery
type GalleryImagesResponse struct {
	VideoCode   string         `json:"videoCode"`
	Status      string         `json:"status"` // none, processing, pending_review, ready
	Source      []GalleryImage `json:"source"`
	Safe        []GalleryImage `json:"safe"`
	Nsfw        []GalleryImage `json:"nsfw"`
	SourceCount int            `json:"sourceCount"`
	SafeCount   int            `json:"safeCount"`
	NsfwCount   int            `json:"nsfwCount"`
}

// MoveImageRequest ย้ายภาพเดี่ยว
type MoveImageRequest struct {
	Filename string `json:"filename" validate:"required"`
	From     string `json:"from" validate:"required,oneof=source safe nsfw"`
	To       string `json:"to" validate:"required,oneof=source safe nsfw"`
}

// MoveBatchRequest ย้ายหลายภาพ
type MoveBatchRequest struct {
	Files []string `json:"files" validate:"required,min=1"`
	From  string   `json:"from" validate:"required,oneof=source safe nsfw"`
	To    string   `json:"to" validate:"required,oneof=source safe nsfw"`
}

// === Handlers ===

// GetGalleryImages ดึงรายการภาพทั้งหมดใน gallery (พร้อม presigned URLs)
// GET /api/v1/admin/videos/:id/gallery
func (h *GalleryAdminHandler) GetGalleryImages(c *fiber.Ctx) error {
	ctx := c.UserContext()
	idParam := c.Params("id")

	videoID, err := uuid.Parse(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid video ID")
	}

	// ดึงข้อมูล video
	video, err := h.videoService.GetByID(ctx, videoID)
	if err != nil {
		logger.WarnContext(ctx, "Video not found", "id", idParam)
		return utils.NotFoundResponse(c, "Video not found")
	}

	// Check ว่ามี gallery path หรือไม่
	if video.GalleryPath == "" {
		return utils.SuccessResponse(c, GalleryImagesResponse{
			VideoCode:   video.Code,
			Status:      video.GalleryStatus,
			Source:      []GalleryImage{},
			Safe:        []GalleryImage{},
			Nsfw:        []GalleryImage{},
			SourceCount: 0,
			SafeCount:   0,
			NsfwCount:   0,
		})
	}

	// List files จากแต่ละ folder
	expiry := 1 * time.Hour
	basePath := video.GalleryPath

	sourceImages := h.listFolderImages(basePath, "source", expiry)
	safeImages := h.listFolderImages(basePath, "safe", expiry)
	nsfwImages := h.listFolderImages(basePath, "nsfw", expiry)

	logger.InfoContext(ctx, "Gallery images listed",
		"video_id", videoID,
		"source", len(sourceImages),
		"safe", len(safeImages),
		"nsfw", len(nsfwImages),
	)

	return utils.SuccessResponse(c, GalleryImagesResponse{
		VideoCode:   video.Code,
		Status:      video.GalleryStatus,
		Source:      sourceImages,
		Safe:        safeImages,
		Nsfw:        nsfwImages,
		SourceCount: len(sourceImages),
		SafeCount:   len(safeImages),
		NsfwCount:   len(nsfwImages),
	})
}

// MoveImage ย้ายภาพเดี่ยวระหว่าง folders
// POST /api/v1/admin/videos/:id/gallery/move
func (h *GalleryAdminHandler) MoveImage(c *fiber.Ctx) error {
	ctx := c.UserContext()
	idParam := c.Params("id")

	videoID, err := uuid.Parse(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid video ID")
	}

	var req MoveImageRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		return utils.ValidationErrorResponse(c, utils.GetValidationErrors(err))
	}

	// ไม่อนุญาตย้ายไปที่เดิม
	if req.From == req.To {
		return utils.BadRequestResponse(c, "Source and destination folders are the same")
	}

	video, err := h.videoService.GetByID(ctx, videoID)
	if err != nil {
		return utils.NotFoundResponse(c, "Video not found")
	}

	if video.GalleryPath == "" {
		return utils.BadRequestResponse(c, "Video has no gallery")
	}

	// ทำการย้ายไฟล์
	err = h.moveFile(video.GalleryPath, req.Filename, req.From, req.To)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to move image",
			"video_id", videoID,
			"filename", req.Filename,
			"from", req.From,
			"to", req.To,
			"error", err,
		)
		return utils.InternalServerErrorResponse(c)
	}

	// อัพเดท counts ใน database
	if err := h.updateGalleryCounts(ctx, videoID, video.GalleryPath); err != nil {
		logger.WarnContext(ctx, "Failed to update gallery counts", "error", err)
	}

	logger.InfoContext(ctx, "Image moved",
		"video_id", videoID,
		"filename", req.Filename,
		"from", req.From,
		"to", req.To,
	)

	return utils.SuccessResponse(c, fiber.Map{
		"message": "Image moved successfully",
	})
}

// MoveBatch ย้ายหลายภาพพร้อมกัน
// POST /api/v1/admin/videos/:id/gallery/move-batch
func (h *GalleryAdminHandler) MoveBatch(c *fiber.Ctx) error {
	ctx := c.UserContext()
	idParam := c.Params("id")

	videoID, err := uuid.Parse(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid video ID")
	}

	var req MoveBatchRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		return utils.ValidationErrorResponse(c, utils.GetValidationErrors(err))
	}

	if req.From == req.To {
		return utils.BadRequestResponse(c, "Source and destination folders are the same")
	}

	video, err := h.videoService.GetByID(ctx, videoID)
	if err != nil {
		return utils.NotFoundResponse(c, "Video not found")
	}

	if video.GalleryPath == "" {
		return utils.BadRequestResponse(c, "Video has no gallery")
	}

	// ย้ายทีละไฟล์
	var movedCount int
	var failedFiles []string

	for _, filename := range req.Files {
		err := h.moveFile(video.GalleryPath, filename, req.From, req.To)
		if err != nil {
			logger.WarnContext(ctx, "Failed to move file",
				"filename", filename,
				"error", err,
			)
			failedFiles = append(failedFiles, filename)
		} else {
			movedCount++
		}
	}

	// อัพเดท counts
	if err := h.updateGalleryCounts(ctx, videoID, video.GalleryPath); err != nil {
		logger.WarnContext(ctx, "Failed to update gallery counts", "error", err)
	}

	logger.InfoContext(ctx, "Batch move completed",
		"video_id", videoID,
		"requested", len(req.Files),
		"moved", movedCount,
		"failed", len(failedFiles),
	)

	return utils.SuccessResponse(c, fiber.Map{
		"message":     fmt.Sprintf("Moved %d of %d images", movedCount, len(req.Files)),
		"movedCount":  movedCount,
		"failedFiles": failedFiles,
	})
}

// PublishGallery เปลี่ยน status เป็น ready
// POST /api/v1/admin/videos/:id/gallery/publish
func (h *GalleryAdminHandler) PublishGallery(c *fiber.Ctx) error {
	ctx := c.UserContext()
	idParam := c.Params("id")

	videoID, err := uuid.Parse(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid video ID")
	}

	video, err := h.videoService.GetByID(ctx, videoID)
	if err != nil {
		return utils.NotFoundResponse(c, "Video not found")
	}

	if video.GalleryPath == "" {
		return utils.BadRequestResponse(c, "Video has no gallery")
	}

	// ตรวจสอบว่ามีภาพใน safe หรือ nsfw หรือไม่
	basePath := strings.TrimSuffix(video.GalleryPath, "/")
	safeFiles, _ := h.storage.ListFiles(fmt.Sprintf("%s/safe", basePath))
	nsfwFiles, _ := h.storage.ListFiles(fmt.Sprintf("%s/nsfw", basePath))

	if len(safeFiles) == 0 && len(nsfwFiles) == 0 {
		return utils.BadRequestResponse(c, "Cannot publish: No images in safe or nsfw folders")
	}

	// อัพเดท status และ counts ผ่าน Update ของ VideoService
	gallerySafeCount := len(safeFiles)
	galleryNsfwCount := len(nsfwFiles)
	galleryCount := gallerySafeCount + galleryNsfwCount
	galleryStatus := "ready"

	updateReq := &dto.UpdateVideoRequest{
		GalleryStatus:    &galleryStatus,
		GalleryCount:     &galleryCount,
		GallerySafeCount: &gallerySafeCount,
		GalleryNsfwCount: &galleryNsfwCount,
	}

	_, err = h.videoService.Update(ctx, videoID, updateReq)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to publish gallery", "video_id", videoID, "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	logger.InfoContext(ctx, "Gallery published",
		"video_id", videoID,
		"safe_count", gallerySafeCount,
		"nsfw_count", galleryNsfwCount,
	)

	return utils.SuccessResponse(c, fiber.Map{
		"message":   "Gallery published successfully",
		"status":    galleryStatus,
		"safeCount": gallerySafeCount,
		"nsfwCount": galleryNsfwCount,
		"total":     galleryCount,
	})
}

// === Helper Functions ===

// listFolderImages list ภาพใน folder และสร้าง presigned URLs
func (h *GalleryAdminHandler) listFolderImages(basePath, folder string, expiry time.Duration) []GalleryImage {
	// Remove trailing slash from basePath to avoid double slash
	basePath = strings.TrimSuffix(basePath, "/")
	folderPath := fmt.Sprintf("%s/%s", basePath, folder)
	files, err := h.storage.ListFiles(folderPath)
	if err != nil {
		return []GalleryImage{}
	}

	images := make([]GalleryImage, 0, len(files))
	for _, filePath := range files {
		// Skip non-image files
		ext := strings.ToLower(filepath.Ext(filePath))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
			continue
		}

		presignedURL, err := h.storage.GetPresignedDownloadURL(filePath, expiry)
		if err != nil {
			continue
		}

		filename := filepath.Base(filePath)
		images = append(images, GalleryImage{
			Filename: filename,
			URL:      presignedURL,
			Folder:   folder,
		})
	}

	return images
}

// moveFile ย้ายไฟล์ระหว่าง folders โดยใช้ copy + delete
func (h *GalleryAdminHandler) moveFile(basePath, filename, fromFolder, toFolder string) error {
	basePath = strings.TrimSuffix(basePath, "/")
	srcPath := fmt.Sprintf("%s/%s/%s", basePath, fromFolder, filename)
	dstPath := fmt.Sprintf("%s/%s/%s", basePath, toFolder, filename)

	// อ่านไฟล์ต้นทาง
	reader, contentType, err := h.storage.GetFileContent(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}
	defer reader.Close()

	// Upload ไปปลายทาง
	_, err = h.storage.UploadFile(reader, dstPath, contentType)
	if err != nil {
		return fmt.Errorf("failed to upload to destination: %w", err)
	}

	// ลบไฟล์ต้นทาง
	err = h.storage.DeleteFile(srcPath)
	if err != nil {
		// ถ้าลบไม่ได้ ให้ลบไฟล์ที่ upload ไปแล้ว (rollback)
		_ = h.storage.DeleteFile(dstPath)
		return fmt.Errorf("failed to delete source file: %w", err)
	}

	return nil
}

// updateGalleryCounts อัพเดท counts ใน database จากไฟล์จริง
func (h *GalleryAdminHandler) updateGalleryCounts(ctx context.Context, videoID uuid.UUID, galleryPath string) error {
	basePath := strings.TrimSuffix(galleryPath, "/")
	sourceFiles, _ := h.storage.ListFiles(fmt.Sprintf("%s/source", basePath))
	safeFiles, _ := h.storage.ListFiles(fmt.Sprintf("%s/safe", basePath))
	nsfwFiles, _ := h.storage.ListFiles(fmt.Sprintf("%s/nsfw", basePath))

	sourceCount := len(sourceFiles)
	safeCount := len(safeFiles)
	nsfwCount := len(nsfwFiles)
	totalCount := safeCount + nsfwCount

	updateReq := &dto.UpdateVideoRequest{
		GallerySourceCount: &sourceCount,
		GallerySafeCount:   &safeCount,
		GalleryNsfwCount:   &nsfwCount,
		GalleryCount:       &totalCount,
	}

	_, err := h.videoService.Update(ctx, videoID, updateReq)
	return err
}
