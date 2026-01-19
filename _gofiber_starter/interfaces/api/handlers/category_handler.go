package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"gofiber-template/domain/dto"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/utils"
)

type CategoryHandler struct {
	categoryService services.CategoryService
}

func NewCategoryHandler(categoryService services.CategoryService) *CategoryHandler {
	return &CategoryHandler{
		categoryService: categoryService,
	}
}

// Create สร้าง category ใหม่
func (h *CategoryHandler) Create(c *fiber.Ctx) error {
	ctx := c.UserContext()

	var req dto.CreateCategoryRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	category, err := h.categoryService.Create(ctx, &req)
	if err != nil {
		logger.WarnContext(ctx, "Category creation failed", "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Category created", "category_id", category.ID, "name", category.Name)
	return utils.CreatedResponse(c, dto.CategoryToCategoryResponse(category))
}

// GetByID ดึง category ตาม ID
func (h *CategoryHandler) GetByID(c *fiber.Ctx) error {
	ctx := c.UserContext()
	idParam := c.Params("id")

	id, err := uuid.Parse(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid category ID")
	}

	category, err := h.categoryService.GetByID(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "Category not found", "category_id", id)
		return utils.NotFoundResponse(c, "Category not found")
	}

	return utils.SuccessResponse(c, dto.CategoryToCategoryResponse(category))
}

// GetBySlug ดึง category ตาม slug
func (h *CategoryHandler) GetBySlug(c *fiber.Ctx) error {
	ctx := c.UserContext()
	slug := c.Params("slug")

	if slug == "" {
		return utils.BadRequestResponse(c, "Category slug is required")
	}

	category, err := h.categoryService.GetBySlug(ctx, slug)
	if err != nil {
		logger.WarnContext(ctx, "Category not found", "slug", slug)
		return utils.NotFoundResponse(c, "Category not found")
	}

	return utils.SuccessResponse(c, dto.CategoryToCategoryResponse(category))
}

// List ดึง categories ทั้งหมด (flat list)
func (h *CategoryHandler) List(c *fiber.Ctx) error {
	ctx := c.UserContext()

	categories, err := h.categoryService.List(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to list categories", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	// ดึงจำนวนวิดีโอในแต่ละ category
	videoCounts, _ := h.categoryService.GetVideoCounts(ctx)

	responses := dto.CategoriesToCategoryResponses(categories)
	// เพิ่ม video count
	for i := range responses {
		if count, ok := videoCounts[responses[i].ID]; ok {
			responses[i].VideoCount = count
		}
	}

	return utils.SuccessResponse(c, dto.CategoryListResponse{
		Categories: responses,
	})
}

// ListTree ดึง categories แบบ tree structure
func (h *CategoryHandler) ListTree(c *fiber.Ctx) error {
	ctx := c.UserContext()

	categories, err := h.categoryService.ListTree(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to list categories tree", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	// ดึงจำนวนวิดีโอในแต่ละ category
	videoCounts, _ := h.categoryService.GetVideoCounts(ctx)

	responses := dto.CategoriesToTreeResponses(categories)
	// เพิ่ม video count (รวม children ด้วย)
	addVideoCountsToTree(responses, videoCounts)

	return utils.SuccessResponse(c, dto.CategoryListResponse{
		Categories: responses,
	})
}

// addVideoCountsToTree เพิ่ม video count ให้ tree structure
func addVideoCountsToTree(categories []dto.CategoryResponse, counts map[uuid.UUID]int64) {
	for i := range categories {
		if count, ok := counts[categories[i].ID]; ok {
			categories[i].VideoCount = count
		}
		if len(categories[i].Children) > 0 {
			childResponses := make([]dto.CategoryResponse, len(categories[i].Children))
			for j, child := range categories[i].Children {
				childResponses[j] = *child
			}
			addVideoCountsToTree(childResponses, counts)
			for j := range categories[i].Children {
				categories[i].Children[j].VideoCount = childResponses[j].VideoCount
			}
		}
	}
}

// Reorder จัดเรียง categories ใหม่
func (h *CategoryHandler) Reorder(c *fiber.Ctx) error {
	ctx := c.UserContext()

	var req dto.ReorderCategoriesRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	if err := h.categoryService.Reorder(ctx, &req); err != nil {
		logger.WarnContext(ctx, "Failed to reorder categories", "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Categories reordered")
	return utils.SuccessResponse(c, fiber.Map{"message": "Categories reordered successfully"})
}

// Update อัปเดต category
func (h *CategoryHandler) Update(c *fiber.Ctx) error {
	ctx := c.UserContext()
	idParam := c.Params("id")

	id, err := uuid.Parse(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid category ID")
	}

	var req dto.UpdateCategoryRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	category, err := h.categoryService.Update(ctx, id, &req)
	if err != nil {
		logger.WarnContext(ctx, "Category update failed", "category_id", id, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Category updated", "category_id", id)
	return utils.SuccessResponse(c, dto.CategoryToCategoryResponse(category))
}

// Delete ลบ category
func (h *CategoryHandler) Delete(c *fiber.Ctx) error {
	ctx := c.UserContext()
	idParam := c.Params("id")

	id, err := uuid.Parse(idParam)
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid category ID")
	}

	if err := h.categoryService.Delete(ctx, id); err != nil {
		logger.WarnContext(ctx, "Category delete failed", "category_id", id, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Category deleted", "category_id", id)
	return utils.SuccessResponse(c, fiber.Map{"message": "Category deleted successfully"})
}
