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

// ═══════════════════════════════════════════════════════════════════════════════
// WorkerJobHandler - HTTP handlers for Worker Queue Jobs
// ═══════════════════════════════════════════════════════════════════════════════

type WorkerJobHandler struct {
	service services.WorkerJobService
}

func NewWorkerJobHandler(service services.WorkerJobService) *WorkerJobHandler {
	return &WorkerJobHandler{
		service: service,
	}
}

// ═══════════════════════════════════════════════════════════════════════════════
// Job Query
// ═══════════════════════════════════════════════════════════════════════════════

// ListJobs - GET /api/v1/worker-jobs
func (h *WorkerJobHandler) ListJobs(c *fiber.Ctx) error {
	ctx := c.UserContext()

	var filter dto.WorkerJobFilterRequest
	if err := c.QueryParser(&filter); err != nil {
		logger.WarnContext(ctx, "Invalid query parameters", "error", err)
		return utils.BadRequestResponse(c, "Invalid query parameters")
	}

	jobs, total, err := h.service.ListJobs(ctx, &filter)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to list worker jobs", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	// Convert to responses
	offset := 0
	if filter.Page > 1 {
		offset = (filter.Page - 1) * filter.Limit
	}

	listResponse := dto.WorkerJobsToListResponse(jobs, total, offset, filter.Limit)
	return utils.SuccessResponse(c, listResponse)
}

// GetJob - GET /api/v1/worker-jobs/:id
func (h *WorkerJobHandler) GetJob(c *fiber.Ctx) error {
	ctx := c.UserContext()

	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid job ID", "id", idStr)
		return utils.BadRequestResponse(c, "Invalid job ID")
	}

	job, err := h.service.GetJob(ctx, id)
	if err != nil {
		logger.WarnContext(ctx, "Worker job not found", "id", id)
		return utils.NotFoundResponse(c, "Worker job not found")
	}

	response := dto.WorkerJobToDetailResponse(job)
	return utils.SuccessResponse(c, response)
}

// GetJobsByEntity - GET /api/v1/worker-jobs/entity/:type/:id
func (h *WorkerJobHandler) GetJobsByEntity(c *fiber.Ctx) error {
	ctx := c.UserContext()

	entityType := c.Params("type")
	entityIDStr := c.Params("id")

	entityID, err := uuid.Parse(entityIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid entity ID", "entity_id", entityIDStr)
		return utils.BadRequestResponse(c, "Invalid entity ID")
	}

	jobs, err := h.service.GetJobsByEntity(ctx, models.WorkerEntityType(entityType), entityID)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get jobs by entity", "entity_type", entityType, "entity_id", entityID, "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	// Convert to responses
	responses := make([]*dto.WorkerJobResponse, len(jobs))
	for i, job := range jobs {
		responses[i] = dto.WorkerJobToResponse(job)
	}

	return utils.SuccessResponse(c, responses)
}

// ═══════════════════════════════════════════════════════════════════════════════
// Job Actions
// ═══════════════════════════════════════════════════════════════════════════════

// CancelJob - POST /api/v1/worker-jobs/:id/cancel
func (h *WorkerJobHandler) CancelJob(c *fiber.Ctx) error {
	ctx := c.UserContext()

	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid job ID", "id", idStr)
		return utils.BadRequestResponse(c, "Invalid job ID")
	}

	logger.InfoContext(ctx, "Worker job cancel attempt", "job_id", id)

	if err := h.service.CancelJob(ctx, id); err != nil {
		logger.WarnContext(ctx, "Worker job cancel failed", "job_id", id, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Worker job cancelled", "job_id", id)
	return utils.SuccessResponse(c, map[string]string{"message": "Job cancelled"})
}

// RetryJob - POST /api/v1/worker-jobs/:id/retry
func (h *WorkerJobHandler) RetryJob(c *fiber.Ctx) error {
	ctx := c.UserContext()

	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid job ID", "id", idStr)
		return utils.BadRequestResponse(c, "Invalid job ID")
	}

	logger.InfoContext(ctx, "Worker job retry attempt", "job_id", id)

	if err := h.service.RetryJob(ctx, id); err != nil {
		logger.WarnContext(ctx, "Worker job retry failed", "job_id", id, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Worker job retry scheduled", "job_id", id)
	return utils.SuccessResponse(c, map[string]string{"message": "Retry scheduled"})
}

// ═══════════════════════════════════════════════════════════════════════════════
// Stats
// ═══════════════════════════════════════════════════════════════════════════════

// GetStats - GET /api/v1/worker-jobs/stats
func (h *WorkerJobHandler) GetStats(c *fiber.Ctx) error {
	ctx := c.UserContext()

	jobTypeStr := c.Query("job_type")

	var jobType *models.WorkerJobType
	if jobTypeStr != "" {
		jt := models.WorkerJobType(jobTypeStr)
		if jt.IsValid() {
			jobType = &jt
		}
	}

	stats, err := h.service.GetStats(ctx, jobType)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get worker job stats", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, stats)
}

// GetAllStats - GET /api/v1/worker-jobs/stats/all
func (h *WorkerJobHandler) GetAllStats(c *fiber.Ctx) error {
	ctx := c.UserContext()

	stats, err := h.service.GetAllStats(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get all worker job stats", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, stats)
}

// ═══════════════════════════════════════════════════════════════════════════════
// Cleanup
// ═══════════════════════════════════════════════════════════════════════════════

// DeleteJob - DELETE /api/v1/worker-jobs/:id
func (h *WorkerJobHandler) DeleteJob(c *fiber.Ctx) error {
	ctx := c.UserContext()

	idStr := c.Params("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid job ID", "id", idStr)
		return utils.BadRequestResponse(c, "Invalid job ID")
	}

	logger.InfoContext(ctx, "Worker job delete attempt", "job_id", id)

	if err := h.service.DeleteJob(ctx, id); err != nil {
		logger.WarnContext(ctx, "Worker job delete failed", "job_id", id, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Worker job deleted", "job_id", id)
	return utils.SuccessResponse(c, map[string]string{"message": "Job deleted"})
}

// DeleteOrphanedJobs - DELETE /api/v1/worker-jobs/orphaned
func (h *WorkerJobHandler) DeleteOrphanedJobs(c *fiber.Ctx) error {
	ctx := c.UserContext()

	logger.InfoContext(ctx, "Delete orphaned jobs attempt")

	count, err := h.service.DeleteOrphanedJobs(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to delete orphaned jobs", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	logger.InfoContext(ctx, "Orphaned jobs deleted", "count", count)
	return utils.SuccessResponse(c, map[string]interface{}{
		"message": "Orphaned jobs deleted",
		"count":   count,
	})
}

// DeleteCompletedJobs - DELETE /api/v1/worker-jobs/completed
func (h *WorkerJobHandler) DeleteCompletedJobs(c *fiber.Ctx) error {
	ctx := c.UserContext()

	// Get days parameter (default 7)
	days := c.QueryInt("days", 7)
	if days < 1 {
		days = 7
	}

	logger.InfoContext(ctx, "Delete completed jobs attempt", "older_than_days", days)

	count, err := h.service.DeleteCompletedJobs(ctx, days)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to delete completed jobs", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	logger.InfoContext(ctx, "Completed jobs deleted", "count", count, "older_than_days", days)
	return utils.SuccessResponse(c, map[string]interface{}{
		"message":         "Completed jobs deleted",
		"count":           count,
		"older_than_days": days,
	})
}

// DeleteFailedJobs - DELETE /api/v1/worker-jobs/failed
func (h *WorkerJobHandler) DeleteFailedJobs(c *fiber.Ctx) error {
	ctx := c.UserContext()

	logger.InfoContext(ctx, "Delete failed jobs attempt")

	count, err := h.service.DeleteFailedJobs(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to delete failed jobs", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	logger.InfoContext(ctx, "Failed jobs deleted", "count", count)
	return utils.SuccessResponse(c, map[string]interface{}{
		"message": "Failed jobs deleted",
		"count":   count,
	})
}
