package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/utils"
)

type ReelHandler struct {
	reelService services.ReelService
}

func NewReelHandler(reelService services.ReelService) *ReelHandler {
	return &ReelHandler{
		reelService: reelService,
	}
}

// getUserIDFromContext ดึง user ID จาก fiber context
func getUserIDFromContext(c *fiber.Ctx) (uuid.UUID, error) {
	uid := c.Locals("user_id")
	if uid == nil {
		return uuid.Nil, fiber.ErrUnauthorized
	}
	userID, ok := uid.(uuid.UUID)
	if !ok {
		return uuid.Nil, fiber.ErrUnauthorized
	}
	return userID, nil
}

// Create สร้าง reel ใหม่
// POST /api/v1/reels
func (h *ReelHandler) Create(c *fiber.Ctx) error {
	ctx := c.UserContext()

	// 1. Get user ID from context
	userID, err := getUserIDFromContext(c)
	if err != nil {
		logger.WarnContext(ctx, "Unauthorized: no user ID in context")
		return utils.UnauthorizedResponse(c, "Unauthorized")
	}

	// 2. Parse request body
	var req dto.CreateReelRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	// 3. Validate
	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	// 4. Create reel
	reel, err := h.reelService.Create(ctx, userID, &req)
	if err != nil {
		logger.WarnContext(ctx, "Failed to create reel", "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Reel created", "reel_id", reel.ID)
	return utils.CreatedResponse(c, dto.ReelToResponse(reel))
}

// GetByID ดึง reel ตาม ID
// GET /api/v1/reels/:id
func (h *ReelHandler) GetByID(c *fiber.Ctx) error {
	ctx := c.UserContext()

	// 1. Get user ID
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return utils.UnauthorizedResponse(c, "Unauthorized")
	}

	// 2. Parse reel ID
	reelIDStr := c.Params("id")
	reelID, err := uuid.Parse(reelIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid reel ID", "reel_id", reelIDStr)
		return utils.BadRequestResponse(c, "Invalid reel ID")
	}

	// 3. Get reel (with ownership check)
	reel, err := h.reelService.GetByIDForUser(ctx, reelID, userID)
	if err != nil {
		logger.WarnContext(ctx, "Failed to get reel", "reel_id", reelID, "error", err)
		return utils.NotFoundResponse(c, err.Error())
	}

	return utils.SuccessResponse(c, dto.ReelToResponse(reel))
}

// Update อัปเดต reel
// PUT /api/v1/reels/:id
func (h *ReelHandler) Update(c *fiber.Ctx) error {
	ctx := c.UserContext()

	// 1. Get user ID
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return utils.UnauthorizedResponse(c, "Unauthorized")
	}

	// 2. Parse reel ID
	reelIDStr := c.Params("id")
	reelID, err := uuid.Parse(reelIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid reel ID", "reel_id", reelIDStr)
		return utils.BadRequestResponse(c, "Invalid reel ID")
	}

	// 3. Parse request body
	var req dto.UpdateReelRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	// 4. Validate
	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	// 5. Update reel
	reel, err := h.reelService.Update(ctx, reelID, userID, &req)
	if err != nil {
		logger.WarnContext(ctx, "Failed to update reel", "reel_id", reelID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Reel updated", "reel_id", reelID)
	return utils.SuccessResponse(c, dto.ReelToResponse(reel))
}

// Delete ลบ reel
// DELETE /api/v1/reels/:id
func (h *ReelHandler) Delete(c *fiber.Ctx) error {
	ctx := c.UserContext()

	// 1. Get user ID
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return utils.UnauthorizedResponse(c, "Unauthorized")
	}

	// 2. Parse reel ID
	reelIDStr := c.Params("id")
	reelID, err := uuid.Parse(reelIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid reel ID", "reel_id", reelIDStr)
		return utils.BadRequestResponse(c, "Invalid reel ID")
	}

	// 3. Delete reel
	if err := h.reelService.Delete(ctx, reelID, userID); err != nil {
		logger.WarnContext(ctx, "Failed to delete reel", "reel_id", reelID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Reel deleted", "reel_id", reelID)
	return utils.SuccessResponse(c, map[string]string{"message": "Reel deleted successfully"})
}

// List ดึง reels ของ user
// GET /api/v1/reels
func (h *ReelHandler) List(c *fiber.Ctx) error {
	ctx := c.UserContext()

	// 1. Get user ID
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return utils.UnauthorizedResponse(c, "Unauthorized")
	}

	// 2. Parse query params
	var params dto.ReelFilterRequest
	if err := c.QueryParser(&params); err != nil {
		logger.WarnContext(ctx, "Invalid query params", "error", err)
		return utils.BadRequestResponse(c, "Invalid query parameters")
	}

	// 3. Get reels
	reels, total, err := h.reelService.ListWithFilters(ctx, userID, &params)
	if err != nil {
		logger.WarnContext(ctx, "Failed to list reels", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	// 4. Build pagination
	page := params.Page
	if page < 1 {
		page = 1
	}
	limit := params.Limit
	if limit < 1 || limit > 100 {
		limit = 20
	}

	return utils.PaginatedSuccessResponse(c, dto.ReelsToResponses(reels), total, page, limit)
}

// ListByVideo ดึง reels ที่สร้างจาก video
// GET /api/v1/videos/:id/reels
func (h *ReelHandler) ListByVideo(c *fiber.Ctx) error {
	ctx := c.UserContext()

	// 1. Parse video ID
	videoIDStr := c.Params("id")
	videoID, err := uuid.Parse(videoIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid video ID", "video_id", videoIDStr)
		return utils.BadRequestResponse(c, "Invalid video ID")
	}

	// 2. Parse pagination
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)

	// 3. Get reels
	reels, total, err := h.reelService.ListByVideo(ctx, videoID, page, limit)
	if err != nil {
		logger.WarnContext(ctx, "Failed to list reels by video", "video_id", videoID, "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.PaginatedSuccessResponse(c, dto.ReelsToResponses(reels), total, page, limit)
}

// Export ส่ง reel ไป export
// POST /api/v1/reels/:id/export
func (h *ReelHandler) Export(c *fiber.Ctx) error {
	ctx := c.UserContext()

	// 1. Get user ID
	userID, err := getUserIDFromContext(c)
	if err != nil {
		return utils.UnauthorizedResponse(c, "Unauthorized")
	}

	// 2. Parse reel ID
	reelIDStr := c.Params("id")
	reelID, err := uuid.Parse(reelIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid reel ID", "reel_id", reelIDStr)
		return utils.BadRequestResponse(c, "Invalid reel ID")
	}

	// 3. Export reel
	if err := h.reelService.Export(ctx, reelID, userID); err != nil {
		logger.WarnContext(ctx, "Failed to export reel", "reel_id", reelID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Reel export started", "reel_id", reelID)
	return utils.SuccessResponse(c, dto.ReelExportResponse{
		ID:      reelID,
		Status:  models.ReelStatusExporting,
		Message: "Reel export job submitted",
	})
}

// GetTemplates ดึง templates ทั้งหมด
// GET /api/v1/reels/templates
func (h *ReelHandler) GetTemplates(c *fiber.Ctx) error {
	ctx := c.UserContext()

	templates, err := h.reelService.GetTemplates(ctx)
	if err != nil {
		logger.WarnContext(ctx, "Failed to get templates", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, dto.ReelTemplatesToResponses(templates))
}

// GetTemplateByID ดึง template ตาม ID
// GET /api/v1/reels/templates/:id
func (h *ReelHandler) GetTemplateByID(c *fiber.Ctx) error {
	ctx := c.UserContext()

	templateIDStr := c.Params("id")
	templateID, err := uuid.Parse(templateIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid template ID", "template_id", templateIDStr)
		return utils.BadRequestResponse(c, "Invalid template ID")
	}

	template, err := h.reelService.GetTemplateByID(ctx, templateID)
	if err != nil {
		logger.WarnContext(ctx, "Failed to get template", "template_id", templateID, "error", err)
		return utils.NotFoundResponse(c, "Template not found")
	}

	return utils.SuccessResponse(c, dto.ReelTemplateToResponse(template))
}
