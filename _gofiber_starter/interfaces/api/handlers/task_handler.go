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

type TaskHandler struct {
	taskService services.TaskService
}

func NewTaskHandler(taskService services.TaskService) *TaskHandler {
	return &TaskHandler{
		taskService: taskService,
	}
}

func (h *TaskHandler) CreateTask(c *fiber.Ctx) error {
	ctx := c.UserContext()

	user, err := utils.GetUserFromContext(c)
	if err != nil {
		logger.WarnContext(ctx, "Unauthorized access attempt")
		return utils.UnauthorizedResponse(c, "")
	}

	var req dto.CreateTaskRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	logger.InfoContext(ctx, "Task creation attempt", "user_id", user.ID, "title", req.Title)

	task, err := h.taskService.CreateTask(ctx, user.ID, &req)
	if err != nil {
		logger.WarnContext(ctx, "Task creation failed", "user_id", user.ID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Task created", "task_id", task.ID, "user_id", user.ID)

	taskResponse := dto.TaskToTaskResponse(task, nil)
	return utils.CreatedResponse(c, taskResponse)
}

func (h *TaskHandler) GetTask(c *fiber.Ctx) error {
	ctx := c.UserContext()

	taskIDStr := c.Params("id")
	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid task ID", "task_id", taskIDStr)
		return utils.BadRequestResponse(c, "Invalid task ID")
	}

	task, err := h.taskService.GetTask(ctx, taskID)
	if err != nil {
		logger.WarnContext(ctx, "Task not found", "task_id", taskID)
		return utils.NotFoundResponse(c, "Task not found")
	}

	taskResponse := dto.TaskToTaskResponse(task, &task.User)
	return utils.SuccessResponse(c, taskResponse)
}

func (h *TaskHandler) UpdateTask(c *fiber.Ctx) error {
	ctx := c.UserContext()

	taskIDStr := c.Params("id")
	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid task ID", "task_id", taskIDStr)
		return utils.BadRequestResponse(c, "Invalid task ID")
	}

	var req dto.UpdateTaskRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	logger.InfoContext(ctx, "Task update attempt", "task_id", taskID)

	task, err := h.taskService.UpdateTask(ctx, taskID, &req)
	if err != nil {
		logger.WarnContext(ctx, "Task update failed", "task_id", taskID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Task updated", "task_id", taskID)

	taskResponse := dto.TaskToTaskResponse(task, &task.User)
	return utils.SuccessResponse(c, taskResponse)
}

func (h *TaskHandler) DeleteTask(c *fiber.Ctx) error {
	ctx := c.UserContext()

	taskIDStr := c.Params("id")
	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid task ID", "task_id", taskIDStr)
		return utils.BadRequestResponse(c, "Invalid task ID")
	}

	logger.InfoContext(ctx, "Task deletion attempt", "task_id", taskID)

	err = h.taskService.DeleteTask(ctx, taskID)
	if err != nil {
		logger.WarnContext(ctx, "Task deletion failed", "task_id", taskID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Task deleted", "task_id", taskID)

	return utils.NoContentResponse(c)
}

func (h *TaskHandler) GetUserTasks(c *fiber.Ctx) error {
	ctx := c.UserContext()

	user, err := utils.GetUserFromContext(c)
	if err != nil {
		logger.WarnContext(ctx, "Unauthorized access attempt")
		return utils.UnauthorizedResponse(c, "")
	}

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
	tasks, total, err := h.taskService.GetUserTasks(ctx, user.ID, offset, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to retrieve user tasks", "user_id", user.ID, "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	taskResponses := make([]dto.TaskResponse, len(tasks))
	for i, task := range tasks {
		taskResponses[i] = *dto.TaskToTaskResponse(task, &task.User)
	}

	return utils.PaginatedSuccessResponse(c, taskResponses, total, page, limit)
}

func (h *TaskHandler) ListTasks(c *fiber.Ctx) error {
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
	tasks, total, err := h.taskService.ListTasks(ctx, offset, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to retrieve tasks", "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	taskResponses := make([]dto.TaskResponse, len(tasks))
	for i, task := range tasks {
		taskResponses[i] = *dto.TaskToTaskResponse(task, &task.User)
	}

	return utils.PaginatedSuccessResponse(c, taskResponses, total, page, limit)
}
