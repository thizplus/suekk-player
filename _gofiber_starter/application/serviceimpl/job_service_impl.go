package serviceimpl

import (
	"context"
	"errors"
	"fmt"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/scheduler"
	"time"

	"github.com/google/uuid"
)

type JobServiceImpl struct {
	jobRepo   repositories.JobRepository
	scheduler scheduler.EventScheduler
}

func NewJobService(jobRepo repositories.JobRepository, scheduler scheduler.EventScheduler) services.JobService {
	return &JobServiceImpl{
		jobRepo:   jobRepo,
		scheduler: scheduler,
	}
}

func (s *JobServiceImpl) CreateJob(ctx context.Context, req *dto.CreateJobRequest) (*models.Job, error) {
	if err := scheduler.ValidateCronExpression(req.CronExpr); err != nil {
		logger.WarnContext(ctx, "Invalid cron expression", "cron_expr", req.CronExpr, "error", err)
		return nil, fmt.Errorf("invalid cron expression: %v", err)
	}

	existingJob, _ := s.jobRepo.GetByName(ctx, req.Name)
	if existingJob != nil {
		logger.WarnContext(ctx, "Job name already exists", "name", req.Name)
		return nil, errors.New("job with this name already exists")
	}

	nextRun, err := scheduler.GetNextRunTime(req.CronExpr)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to calculate next run time", "cron_expr", req.CronExpr, "error", err)
		return nil, fmt.Errorf("failed to calculate next run time: %v", err)
	}

	job := &models.Job{
		ID:        uuid.New(),
		Name:      req.Name,
		CronExpr:  req.CronExpr,
		Payload:   req.Payload,
		Status:    "active",
		NextRun:   nextRun,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = s.jobRepo.Create(ctx, job)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to create job record", "name", req.Name, "error", err)
		return nil, err
	}

	logger.InfoContext(ctx, "Scheduling job", "job_id", job.ID, "name", job.Name, "cron_expr", job.CronExpr)

	err = s.scheduler.AddJob(job.ID.String(), req.CronExpr, func() {
		s.ExecuteJob(context.Background(), job)
	})
	if err != nil {
		logger.ErrorContext(ctx, "Failed to schedule job, rolling back", "job_id", job.ID, "error", err)
		s.jobRepo.Delete(ctx, job.ID)
		return nil, fmt.Errorf("failed to schedule job: %v", err)
	}

	logger.InfoContext(ctx, "Job created and scheduled successfully", "job_id", job.ID, "name", job.Name, "next_run", nextRun)

	return job, nil
}

func (s *JobServiceImpl) GetJob(ctx context.Context, jobID uuid.UUID) (*models.Job, error) {
	job, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		return nil, errors.New("job not found")
	}
	return job, nil
}

func (s *JobServiceImpl) UpdateJob(ctx context.Context, jobID uuid.UUID, req *dto.UpdateJobRequest) (*models.Job, error) {
	job, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		logger.WarnContext(ctx, "Job not found for update", "job_id", jobID)
		return nil, errors.New("job not found")
	}

	needsReschedule := false

	if req.Name != "" {
		job.Name = req.Name
	}
	if req.CronExpr != "" {
		if err := scheduler.ValidateCronExpression(req.CronExpr); err != nil {
			logger.WarnContext(ctx, "Invalid cron expression for update", "job_id", jobID, "cron_expr", req.CronExpr, "error", err)
			return nil, fmt.Errorf("invalid cron expression: %v", err)
		}
		job.CronExpr = req.CronExpr
		needsReschedule = true
	}
	if req.Payload != "" {
		job.Payload = req.Payload
	}
	if req.IsActive != job.IsActive {
		job.IsActive = req.IsActive
		needsReschedule = true
	}

	if needsReschedule {
		logger.InfoContext(ctx, "Rescheduling job", "job_id", jobID, "is_active", job.IsActive)
		s.scheduler.RemoveJob(jobID.String())
		if job.IsActive {
			nextRun, err := scheduler.GetNextRunTime(job.CronExpr)
			if err != nil {
				logger.ErrorContext(ctx, "Failed to calculate next run time for update", "job_id", jobID, "error", err)
				return nil, fmt.Errorf("failed to calculate next run time: %v", err)
			}
			job.NextRun = nextRun

			err = s.scheduler.AddJob(jobID.String(), job.CronExpr, func() {
				s.ExecuteJob(context.Background(), job)
			})
			if err != nil {
				logger.ErrorContext(ctx, "Failed to reschedule job", "job_id", jobID, "error", err)
				return nil, fmt.Errorf("failed to reschedule job: %v", err)
			}
		}
	}

	job.UpdatedAt = time.Now()

	err = s.jobRepo.Update(ctx, jobID, job)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to update job record", "job_id", jobID, "error", err)
		return nil, err
	}

	logger.InfoContext(ctx, "Job updated successfully", "job_id", jobID, "name", job.Name)

	return job, nil
}

func (s *JobServiceImpl) DeleteJob(ctx context.Context, jobID uuid.UUID) error {
	_, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		logger.WarnContext(ctx, "Job not found for deletion", "job_id", jobID)
		return errors.New("job not found")
	}

	logger.InfoContext(ctx, "Removing job from scheduler", "job_id", jobID)
	s.scheduler.RemoveJob(jobID.String())

	err = s.jobRepo.Delete(ctx, jobID)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to delete job record", "job_id", jobID, "error", err)
		return err
	}

	logger.InfoContext(ctx, "Job deleted successfully", "job_id", jobID)
	return nil
}

func (s *JobServiceImpl) ListJobs(ctx context.Context, offset, limit int) ([]*models.Job, int64, error) {
	jobs, err := s.jobRepo.List(ctx, offset, limit)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to list jobs", "offset", offset, "limit", limit, "error", err)
		return nil, 0, err
	}

	count, err := s.jobRepo.Count(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to count jobs", "error", err)
		return nil, 0, err
	}

	return jobs, count, nil
}

func (s *JobServiceImpl) StartJob(ctx context.Context, jobID uuid.UUID) error {
	job, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		logger.WarnContext(ctx, "Job not found for start", "job_id", jobID)
		return errors.New("job not found")
	}

	if job.IsActive {
		logger.WarnContext(ctx, "Job is already active", "job_id", jobID)
		return errors.New("job is already active")
	}

	job.IsActive = true
	job.UpdatedAt = time.Now()

	nextRun, err := scheduler.GetNextRunTime(job.CronExpr)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to calculate next run time for start", "job_id", jobID, "error", err)
		return fmt.Errorf("failed to calculate next run time: %v", err)
	}
	job.NextRun = nextRun

	logger.InfoContext(ctx, "Starting job", "job_id", jobID, "name", job.Name, "next_run", nextRun)

	err = s.scheduler.AddJob(jobID.String(), job.CronExpr, func() {
		s.ExecuteJob(context.Background(), job)
	})
	if err != nil {
		logger.ErrorContext(ctx, "Failed to add job to scheduler", "job_id", jobID, "error", err)
		return fmt.Errorf("failed to start job: %v", err)
	}

	err = s.jobRepo.Update(ctx, jobID, job)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to update job after start", "job_id", jobID, "error", err)
		return err
	}

	logger.InfoContext(ctx, "Job started successfully", "job_id", jobID, "name", job.Name)
	return nil
}

func (s *JobServiceImpl) StopJob(ctx context.Context, jobID uuid.UUID) error {
	job, err := s.jobRepo.GetByID(ctx, jobID)
	if err != nil {
		logger.WarnContext(ctx, "Job not found for stop", "job_id", jobID)
		return errors.New("job not found")
	}

	if !job.IsActive {
		logger.WarnContext(ctx, "Job is already inactive", "job_id", jobID)
		return errors.New("job is already inactive")
	}

	job.IsActive = false
	job.UpdatedAt = time.Now()

	logger.InfoContext(ctx, "Stopping job", "job_id", jobID, "name", job.Name)
	s.scheduler.RemoveJob(jobID.String())

	err = s.jobRepo.Update(ctx, jobID, job)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to update job after stop", "job_id", jobID, "error", err)
		return err
	}

	logger.InfoContext(ctx, "Job stopped successfully", "job_id", jobID, "name", job.Name)
	return nil
}

func (s *JobServiceImpl) ExecuteJob(ctx context.Context, job *models.Job) error {
	now := time.Now()

	logger.InfoContext(ctx, "Executing job", "job_id", job.ID, "name", job.Name, "started_at", now.Format(time.RFC3339))

	job.LastRun = &now
	job.Status = "running"

	nextRun, err := scheduler.GetNextRunTime(job.CronExpr)
	if err == nil {
		job.NextRun = nextRun
	}

	s.jobRepo.Update(ctx, job.ID, job)

	job.Status = "completed"
	job.UpdatedAt = time.Now()

	err = s.jobRepo.Update(ctx, job.ID, job)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to update job after execution", "job_id", job.ID, "error", err)
		return err
	}

	logger.InfoContext(ctx, "Job executed successfully", "job_id", job.ID, "name", job.Name, "next_run", nextRun)
	return nil
}
