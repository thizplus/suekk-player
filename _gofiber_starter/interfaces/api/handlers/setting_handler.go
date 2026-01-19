package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/utils"
)

type SettingHandler struct {
	settingService services.SettingService
}

func NewSettingHandler(settingService services.SettingService) *SettingHandler {
	return &SettingHandler{settingService: settingService}
}

// GetAll ดึง settings ทั้งหมด grouped by category
// GET /api/v1/settings
func (h *SettingHandler) GetAll(c *fiber.Ctx) error {
	ctx := c.UserContext()

	settings, err := h.settingService.GetAll(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get all settings", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	logger.InfoContext(ctx, "Settings retrieved", "categories", len(settings))
	return utils.SuccessResponse(c, settings)
}

// GetByCategory ดึง settings ตาม category
// GET /api/v1/settings/:category
func (h *SettingHandler) GetByCategory(c *fiber.Ctx) error {
	ctx := c.UserContext()
	category := c.Params("category")

	if category == "" {
		return utils.BadRequestResponse(c, "Category is required")
	}

	settings, err := h.settingService.GetByCategory(ctx, category)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get settings by category", "category", category, "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, settings)
}

// GetCategories ดึงรายชื่อ categories ทั้งหมด
// GET /api/v1/settings/categories
func (h *SettingHandler) GetCategories(c *fiber.Ctx) error {
	categories := services.GetCategoryInfo()
	return utils.SuccessResponse(c, categories)
}

// UpdateRequest request สำหรับ update settings
type UpdateSettingsRequest struct {
	Settings map[string]string `json:"settings" validate:"required"`
	Reason   string            `json:"reason"` // เหตุผลที่แก้ไข (optional)
}

// Update อัพเดท settings ของ category
// PUT /api/v1/settings/:category
func (h *SettingHandler) Update(c *fiber.Ctx) error {
	ctx := c.UserContext()
	category := c.Params("category")

	if category == "" {
		return utils.BadRequestResponse(c, "Category is required")
	}

	var req UpdateSettingsRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if len(req.Settings) == 0 {
		return utils.BadRequestResponse(c, "No settings provided")
	}

	// ดึง user ID จาก context (จาก AuthMiddleware)
	var userID *uuid.UUID
	if uid := c.Locals("user_id"); uid != nil {
		if id, ok := uid.(uuid.UUID); ok {
			userID = &id
		}
	}

	// ดึง IP address
	ipAddress := c.IP()

	logger.InfoContext(ctx, "Updating settings",
		"category", category,
		"settings_count", len(req.Settings),
		"user_id", userID,
		"reason", req.Reason,
	)

	if err := h.settingService.Update(ctx, category, req.Settings, userID, req.Reason, ipAddress); err != nil {
		logger.ErrorContext(ctx, "Failed to update settings",
			"category", category,
			"error", err,
		)
		return utils.InternalServerErrorResponse(c)
	}

	// Return updated settings
	settings, err := h.settingService.GetByCategory(ctx, category)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get updated settings", "category", category, "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	logger.InfoContext(ctx, "Settings updated successfully",
		"category", category,
		"settings_count", len(req.Settings),
	)

	return utils.SuccessResponse(c, settings)
}

// ResetRequest request สำหรับ reset to defaults
type ResetSettingsRequest struct {
	Reason string `json:"reason"` // เหตุผลที่ reset (optional)
}

// ResetToDefaults รีเซ็ต settings ของ category กลับเป็นค่า default
// POST /api/v1/settings/:category/reset
func (h *SettingHandler) ResetToDefaults(c *fiber.Ctx) error {
	ctx := c.UserContext()
	category := c.Params("category")

	if category == "" {
		return utils.BadRequestResponse(c, "Category is required")
	}

	var req ResetSettingsRequest
	c.BodyParser(&req) // Optional body

	// ดึง user ID จาก context
	var userID *uuid.UUID
	if uid := c.Locals("user_id"); uid != nil {
		if id, ok := uid.(uuid.UUID); ok {
			userID = &id
		}
	}

	ipAddress := c.IP()

	logger.InfoContext(ctx, "Resetting settings to defaults",
		"category", category,
		"user_id", userID,
		"reason", req.Reason,
	)

	if err := h.settingService.ResetToDefaults(ctx, category, userID, req.Reason, ipAddress); err != nil {
		logger.ErrorContext(ctx, "Failed to reset settings",
			"category", category,
			"error", err,
		)
		return utils.InternalServerErrorResponse(c)
	}

	// Return reset settings
	settings, err := h.settingService.GetByCategory(ctx, category)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get reset settings", "category", category, "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	logger.InfoContext(ctx, "Settings reset to defaults",
		"category", category,
	)

	return utils.SuccessResponse(c, settings)
}

// GetAuditLogs ดึง audit logs
// GET /api/v1/settings/audit-logs
func (h *SettingHandler) GetAuditLogs(c *fiber.Ctx) error {
	ctx := c.UserContext()

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	logs, total, err := h.settingService.GetAuditLogs(ctx, limit, offset)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get audit logs", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.PaginatedSuccessResponse(c, logs, total, page, limit)
}

// ReloadCache โหลด cache ใหม่
// POST /api/v1/settings/reload-cache
func (h *SettingHandler) ReloadCache(c *fiber.Ctx) error {
	ctx := c.UserContext()

	logger.InfoContext(ctx, "Reloading settings cache")

	if err := h.settingService.ReloadCache(ctx); err != nil {
		logger.ErrorContext(ctx, "Failed to reload cache", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	logger.InfoContext(ctx, "Settings cache reloaded successfully")
	return utils.SuccessResponse(c, fiber.Map{"message": "Cache reloaded successfully"})
}
