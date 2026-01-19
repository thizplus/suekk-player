package serviceimpl

import (
	"context"
	"errors"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
	"time"

	"github.com/google/uuid"
)

type TaskServiceImpl struct {
	taskRepo repositories.TaskRepository
	userRepo repositories.UserRepository
}

func NewTaskService(taskRepo repositories.TaskRepository, userRepo repositories.UserRepository) services.TaskService {
	return &TaskServiceImpl{
		taskRepo: taskRepo,
		userRepo: userRepo,
	}
}

func (s *TaskServiceImpl) CreateTask(ctx context.Context, userID uuid.UUID, req *dto.CreateTaskRequest) (*models.Task, error) {
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		logger.WarnContext(ctx, "User not found for task creation", "user_id", userID)
		return nil, errors.New("user not found")
	}

	task := &models.Task{
		ID:          uuid.New(),
		Title:       req.Title,
		Description: req.Description,
		Status:      "pending",
		Priority:    req.Priority,
		DueDate:     req.DueDate,
		UserID:      userID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if task.Priority == 0 {
		task.Priority = 1
	}

	err = s.taskRepo.Create(ctx, task)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to create task", "user_id", userID, "error", err)
		return nil, err
	}

	logger.InfoContext(ctx, "Task created successfully", "task_id", task.ID, "user_id", userID)

	return task, nil
}

func (s *TaskServiceImpl) GetTask(ctx context.Context, taskID uuid.UUID) (*models.Task, error) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, errors.New("task not found")
	}
	return task, nil
}

func (s *TaskServiceImpl) GetUserTasks(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*models.Task, int64, error) {
	tasks, err := s.taskRepo.GetByUserID(ctx, userID, offset, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get user tasks", "user_id", userID, "error", err)
		return nil, 0, err
	}

	count, err := s.taskRepo.CountByUserID(ctx, userID)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to count user tasks", "user_id", userID, "error", err)
		return nil, 0, err
	}

	return tasks, count, nil
}

func (s *TaskServiceImpl) UpdateTask(ctx context.Context, taskID uuid.UUID, req *dto.UpdateTaskRequest) (*models.Task, error) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		logger.WarnContext(ctx, "Task not found for update", "task_id", taskID)
		return nil, errors.New("task not found")
	}

	if req.Title != "" {
		task.Title = req.Title
	}
	if req.Description != "" {
		task.Description = req.Description
	}
	if req.Status != "" {
		task.Status = req.Status
	}
	if req.Priority > 0 {
		task.Priority = req.Priority
	}
	if req.DueDate != nil {
		task.DueDate = req.DueDate
	}

	task.UpdatedAt = time.Now()

	err = s.taskRepo.Update(ctx, taskID, task)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to update task", "task_id", taskID, "error", err)
		return nil, err
	}

	logger.InfoContext(ctx, "Task updated successfully", "task_id", taskID)

	return task, nil
}

func (s *TaskServiceImpl) DeleteTask(ctx context.Context, taskID uuid.UUID) error {
	_, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		logger.WarnContext(ctx, "Task not found for deletion", "task_id", taskID)
		return errors.New("task not found")
	}

	err = s.taskRepo.Delete(ctx, taskID)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to delete task", "task_id", taskID, "error", err)
		return err
	}

	logger.InfoContext(ctx, "Task deleted successfully", "task_id", taskID)
	return nil
}

func (s *TaskServiceImpl) ListTasks(ctx context.Context, offset, limit int) ([]*models.Task, int64, error) {
	tasks, err := s.taskRepo.List(ctx, offset, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to list tasks", "offset", offset, "limit", limit, "error", err)
		return nil, 0, err
	}

	count, err := s.taskRepo.Count(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to count tasks", "error", err)
		return nil, 0, err
	}

	return tasks, count, nil
}
