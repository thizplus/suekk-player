package handlers

import (
	"github.com/gofiber/fiber/v2"

	"gofiber-template/application/serviceimpl"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/utils"
)

type StorageHandler struct {
	storageService services.StorageService
	videoService   services.VideoService
}

func NewStorageHandler(storageService services.StorageService, videoService services.VideoService) *StorageHandler {
	return &StorageHandler{
		storageService: storageService,
		videoService:   videoService,
	}
}

// GetStorageStats ดึงสถิติ storage สำหรับ dashboard
func (h *StorageHandler) GetStorageStats(c *fiber.Ctx) error {
	ctx := c.UserContext()

	if h.storageService == nil {
		logger.WarnContext(ctx, "Storage service not available")
		return utils.BadRequestResponse(c, "Storage service is not available")
	}

	stats, err := h.storageService.GetStorageStats(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get storage stats", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	// Format for display
	formatted := serviceimpl.FormatStorageStats(stats)

	return utils.SuccessResponse(c, fiber.Map{
		"raw":       stats,
		"formatted": formatted,
	})
}

// TriggerCleanup รัน cleanup manually (admin only)
func (h *StorageHandler) TriggerCleanup(c *fiber.Ctx) error {
	ctx := c.UserContext()

	if h.storageService == nil {
		logger.WarnContext(ctx, "Storage service not available")
		return utils.BadRequestResponse(c, "Storage service is not available")
	}

	logger.InfoContext(ctx, "Manual storage cleanup triggered")

	// Run cleanup in background
	go h.storageService.RunCleanup(ctx)

	return utils.SuccessResponse(c, fiber.Map{
		"message": "Cleanup started in background",
	})
}

// GetStorageUsage ดึงข้อมูล storage usage สำหรับ quota
// GET /api/v1/storage/usage
func (h *StorageHandler) GetStorageUsage(c *fiber.Ctx) error {
	ctx := c.UserContext()

	if h.videoService == nil {
		logger.WarnContext(ctx, "Video service not available")
		return utils.BadRequestResponse(c, "Video service is not available")
	}

	usage, err := h.videoService.GetStorageUsage(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get storage usage", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, fiber.Map{
		"used":       usage.TotalUsed,
		"usedHuman":  utils.FormatBytes(uint64(usage.TotalUsed)),
		"quota":      usage.TotalQuota,
		"quotaHuman": utils.FormatBytes(uint64(usage.TotalQuota)),
		"percent":    usage.TotalPercent,
		"unlimited":  usage.Unlimited,
	})
}
