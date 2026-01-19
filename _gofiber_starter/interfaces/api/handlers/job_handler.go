package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/utils"
)

type JobHandler struct {
	jobService services.JobService
}

func NewJobHandler(jobService services.JobService) *JobHandler {
	return &JobHandler{
		jobService: jobService,
	}
}

func (h *JobHandler) CreateJob(c *fiber.Ctx) error {
	ctx := c.UserContext()

	var req dto.CreateJobRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	logger.InfoContext(ctx, "Job creation attempt", "name", req.Name, "cron_expr", req.CronExpr)

	job, err := h.jobService.CreateJob(ctx, &req)
	if err != nil {
		logger.WarnContext(ctx, "Job creation failed", "name", req.Name, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Job created", "job_id", job.ID, "name", job.Name)

	jobResponse := dto.JobToJobResponse(job)
	return utils.CreatedResponse(c, jobResponse)
}

func (h *JobHandler) GetJob(c *fiber.Ctx) error {
	ctx := c.UserContext()

	jobIDStr := c.Params("id")
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid job ID", "job_id", jobIDStr)
		return utils.BadRequestResponse(c, "Invalid job ID")
	}

	job, err := h.jobService.GetJob(ctx, jobID)
	if err != nil {
		logger.WarnContext(ctx, "Job not found", "job_id", jobID)
		return utils.NotFoundResponse(c, "Job not found")
	}

	jobResponse := dto.JobToJobResponse(job)
	return utils.SuccessResponse(c, jobResponse)
}

func (h *JobHandler) UpdateJob(c *fiber.Ctx) error {
	ctx := c.UserContext()

	jobIDStr := c.Params("id")
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid job ID", "job_id", jobIDStr)
		return utils.BadRequestResponse(c, "Invalid job ID")
	}

	var req dto.UpdateJobRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	logger.InfoContext(ctx, "Job update attempt", "job_id", jobID)

	job, err := h.jobService.UpdateJob(ctx, jobID, &req)
	if err != nil {
		logger.WarnContext(ctx, "Job update failed", "job_id", jobID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Job updated", "job_id", jobID)

	jobResponse := dto.JobToJobResponse(job)
	return utils.SuccessResponse(c, jobResponse)
}

func (h *JobHandler) DeleteJob(c *fiber.Ctx) error {
	ctx := c.UserContext()

	jobIDStr := c.Params("id")
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid job ID", "job_id", jobIDStr)
		return utils.BadRequestResponse(c, "Invalid job ID")
	}

	logger.InfoContext(ctx, "Job deletion attempt", "job_id", jobID)

	err = h.jobService.DeleteJob(ctx, jobID)
	if err != nil {
		logger.WarnContext(ctx, "Job deletion failed", "job_id", jobID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Job deleted", "job_id", jobID)

	return utils.NoContentResponse(c)
}

func (h *JobHandler) StartJob(c *fiber.Ctx) error {
	ctx := c.UserContext()

	jobIDStr := c.Params("id")
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid job ID", "job_id", jobIDStr)
		return utils.BadRequestResponse(c, "Invalid job ID")
	}

	logger.InfoContext(ctx, "Job start attempt", "job_id", jobID)

	err = h.jobService.StartJob(ctx, jobID)
	if err != nil {
		logger.WarnContext(ctx, "Job start failed", "job_id", jobID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Job started", "job_id", jobID)

	return utils.SuccessResponse(c, nil)
}

func (h *JobHandler) StopJob(c *fiber.Ctx) error {
	ctx := c.UserContext()

	jobIDStr := c.Params("id")
	jobID, err := uuid.Parse(jobIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid job ID", "job_id", jobIDStr)
		return utils.BadRequestResponse(c, "Invalid job ID")
	}

	logger.InfoContext(ctx, "Job stop attempt", "job_id", jobID)

	err = h.jobService.StopJob(ctx, jobID)
	if err != nil {
		logger.WarnContext(ctx, "Job stop failed", "job_id", jobID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Job stopped", "job_id", jobID)

	return utils.SuccessResponse(c, nil)
}

func (h *JobHandler) ListJobs(c *fiber.Ctx) error {
	ctx := c.UserContext()

	pageStr := c.Query("page", "1")
	limitStr := c.Query("limit", "10")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		logger.WarnContext(ctx, "Invalid page parameter", "page", pageStr)
		return utils.BadRequestResponse(c, "Invalid page parameter")
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		logger.WarnContext(ctx, "Invalid limit parameter", "limit", limitStr)
		return utils.BadRequestResponse(c, "Invalid limit parameter")
	}

	offset := (page - 1) * limit
	jobs, total, err := h.jobService.ListJobs(ctx, offset, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to retrieve jobs", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	jobResponses := make([]dto.JobResponse, len(jobs))
	for i, job := range jobs {
		jobResponses[i] = *dto.JobToJobResponse(job)
	}

	return utils.PaginatedSuccessResponse(c, jobResponses, total, page, limit)
}
