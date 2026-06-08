package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"gofiber-template/domain/dto"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/utils"
)

type SeriesHandler struct {
	seriesService   services.SeriesService
	categoryService services.SeriesCategoryService
}

func NewSeriesHandler(
	seriesService services.SeriesService,
	categoryService services.SeriesCategoryService,
) *SeriesHandler {
	return &SeriesHandler{
		seriesService:   seriesService,
		categoryService: categoryService,
	}
}

// ═══════════════════════════════════════════
// Series
// ═══════════════════════════════════════════

func (h *SeriesHandler) List(c *fiber.Ctx) error {
	ctx := c.UserContext()

	var filter dto.SeriesFilterRequest
	if err := c.QueryParser(&filter); err != nil {
		return utils.BadRequestResponse(c, "Invalid query parameters")
	}

	series, total, err := h.seriesService.List(ctx, &filter)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to list series", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	responses := dto.SeriesToSeriesResponses(series)
	page := filter.Page
	if page < 1 {
		page = 1
	}
	limit := filter.Limit
	if limit < 1 {
		limit = 24
	}

	return utils.PaginatedSuccessResponse(c, responses, total, page, limit)
}

func (h *SeriesHandler) GetByCode(c *fiber.Ctx) error {
	ctx := c.UserContext()
	code := c.Params("code")

	series, err := h.seriesService.GetByCode(ctx, code)
	if err != nil {
		return utils.NotFoundResponse(c, "Series not found")
	}

	return utils.SuccessResponse(c, dto.SeriesToSeriesResponse(series))
}

func (h *SeriesHandler) GetBySlug(c *fiber.Ctx) error {
	ctx := c.UserContext()
	slug := c.Params("slug")

	series, err := h.seriesService.GetBySlug(ctx, slug)
	if err != nil {
		return utils.NotFoundResponse(c, "Series not found")
	}

	return utils.SuccessResponse(c, dto.SeriesToSeriesResponse(series))
}

func (h *SeriesHandler) Create(c *fiber.Ctx) error {
	ctx := c.UserContext()

	var req dto.CreateSeriesRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		return utils.ValidationErrorResponse(c, errors)
	}

	series, err := h.seriesService.Create(ctx, &req)
	if err != nil {
		logger.WarnContext(ctx, "Series creation failed", "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	return utils.CreatedResponse(c, dto.SeriesToSeriesResponse(series))
}

func (h *SeriesHandler) Update(c *fiber.Ctx) error {
	ctx := c.UserContext()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid series ID")
	}

	var req dto.UpdateSeriesRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	series, err := h.seriesService.Update(ctx, id, &req)
	if err != nil {
		return utils.BadRequestResponse(c, err.Error())
	}

	return utils.SuccessResponse(c, dto.SeriesToSeriesResponse(series))
}

func (h *SeriesHandler) Delete(c *fiber.Ctx) error {
	ctx := c.UserContext()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid series ID")
	}

	if err := h.seriesService.Delete(ctx, id); err != nil {
		return utils.BadRequestResponse(c, err.Error())
	}

	return utils.SuccessResponse(c, fiber.Map{"message": "Series deleted"})
}

// Upsert: Bot ใช้ — สร้างหรืออัพเดท by source_site + source_id
func (h *SeriesHandler) Upsert(c *fiber.Ctx) error {
	ctx := c.UserContext()

	var req dto.UpsertSeriesRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		return utils.ValidationErrorResponse(c, errors)
	}

	series, isNew, err := h.seriesService.Upsert(ctx, &req)
	if err != nil {
		logger.WarnContext(ctx, "Series upsert failed", "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	if isNew {
		return utils.CreatedResponse(c, dto.SeriesToSeriesResponse(series))
	}
	return utils.SuccessResponse(c, dto.SeriesToSeriesResponse(series))
}

// ═══════════════════════════════════════════
// Episodes
// ═══════════════════════════════════════════

func (h *SeriesHandler) AddEpisodes(c *fiber.Ctx) error {
	ctx := c.UserContext()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid series ID")
	}

	var req dto.AddEpisodesRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	episodes, err := h.seriesService.AddEpisodes(ctx, id, &req)
	if err != nil {
		return utils.BadRequestResponse(c, err.Error())
	}

	return utils.CreatedResponse(c, fiber.Map{
		"newEpisodes": len(episodes),
	})
}

func (h *SeriesHandler) UpdateEpisode(c *fiber.Ctx) error {
	ctx := c.UserContext()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid series ID")
	}

	epNum, err := c.ParamsInt("episode")
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid episode number")
	}

	var req dto.UpdateEpisodeRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	episode, err := h.seriesService.UpdateEpisode(ctx, id, epNum, &req)
	if err != nil {
		return utils.BadRequestResponse(c, err.Error())
	}

	return utils.SuccessResponse(c, dto.SeriesEpisodeToResponse(episode))
}

func (h *SeriesHandler) ListEpisodes(c *fiber.Ctx) error {
	ctx := c.UserContext()

	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return utils.BadRequestResponse(c, "Invalid series ID")
	}

	episodes, err := h.seriesService.ListEpisodes(ctx, id)
	if err != nil {
		return utils.BadRequestResponse(c, err.Error())
	}

	responses := make([]dto.SeriesEpisodeResponse, len(episodes))
	for i, ep := range episodes {
		responses[i] = *dto.SeriesEpisodeToResponse(&ep)
	}

	return utils.SuccessResponse(c, responses)
}

// ═══════════════════════════════════════════
// Categories
// ═══════════════════════════════════════════

func (h *SeriesHandler) ListCategories(c *fiber.Ctx) error {
	ctx := c.UserContext()

	cats, err := h.categoryService.List(ctx)
	if err != nil {
		return utils.InternalServerErrorResponse(c)
	}

	counts, _ := h.categoryService.GetSeriesCounts(ctx)

	responses := make([]fiber.Map, len(cats))
	for i, cat := range cats {
		resp := fiber.Map{
			"id":          cat.ID,
			"name":        cat.Name,
			"slug":        cat.Slug,
			"seriesCount": counts[cat.ID],
		}
		responses[i] = resp
	}

	return utils.SuccessResponse(c, responses)
}

func (h *SeriesHandler) CreateCategory(c *fiber.Ctx) error {
	ctx := c.UserContext()

	var req struct {
		Name     string     `json:"name" validate:"required"`
		Slug     string     `json:"slug" validate:"required"`
		ParentID *uuid.UUID `json:"parentId"`
	}
	if err := c.BodyParser(&req); err != nil {
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	cat, err := h.categoryService.Create(ctx, req.Name, req.Slug, req.ParentID)
	if err != nil {
		return utils.BadRequestResponse(c, err.Error())
	}

	return utils.CreatedResponse(c, dto.SeriesCategoryToResponse(cat))
}
