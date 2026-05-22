package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/suekk/my_worker/go/transcode/domain"
)

// JobPublisher publishes downstream jobs to NATS JetStream
type JobPublisher struct {
	js     jetstream.JetStream
	logger *slog.Logger
}

// NewJobPublisher creates a new job publisher
func NewJobPublisher(js jetstream.JetStream) *JobPublisher {
	return &JobPublisher{
		js:     js,
		logger: slog.Default().With("component", "job-publisher"),
	}
}

// DownstreamJob wraps _meta and input for downstream jobs
type DownstreamJob struct {
	Meta  domain.JobMeta `json:"_meta"`
	Input interface{}    `json:"input"`
}

// PublishGalleryJob publishes a job to gallery worker
func (p *JobPublisher) PublishGalleryJob(ctx context.Context, meta *domain.JobMeta, input *domain.GalleryJobInput) error {
	job := DownstreamJob{
		Meta: domain.JobMeta{
			JobID:      uuid.New(),
			JobType:    "gallery",
			EntityType: meta.EntityType,
			EntityID:   meta.EntityID,
			EntityCode: meta.EntityCode,
			Priority:   meta.Priority,
			RetryCount: 0,
			MaxRetries: 3,
			TimeoutSec: 600,
			CreatedAt:  time.Now().Unix(),
		},
		Input: input,
	}

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal gallery job: %w", err)
	}

	_, err = p.js.Publish(ctx, "jobs.gallery", data)
	if err != nil {
		p.logger.Error("Failed to publish gallery job",
			"error", err,
			"entity_code", meta.EntityCode,
		)
		return err
	}

	p.logger.Info("Gallery job published",
		"entity_code", meta.EntityCode,
		"job_id", job.Meta.JobID,
	)
	return nil
}

// PublishWarmCacheJob publishes a job to warmcache worker
func (p *JobPublisher) PublishWarmCacheJob(ctx context.Context, meta *domain.JobMeta, input *domain.WarmCacheJobInput) error {
	job := DownstreamJob{
		Meta: domain.JobMeta{
			JobID:      uuid.New(),
			JobType:    "warmcache",
			EntityType: meta.EntityType,
			EntityID:   meta.EntityID,
			EntityCode: meta.EntityCode,
			Priority:   1, // High priority for new uploads
			RetryCount: 0,
			MaxRetries: 3,
			TimeoutSec: 600,
			CreatedAt:  time.Now().Unix(),
		},
		Input: input,
	}

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal warmcache job: %w", err)
	}

	_, err = p.js.Publish(ctx, "jobs.warmcache", data)
	if err != nil {
		p.logger.Error("Failed to publish warmcache job",
			"error", err,
			"entity_code", meta.EntityCode,
		)
		return err
	}

	p.logger.Info("WarmCache job published",
		"entity_code", meta.EntityCode,
		"job_id", job.Meta.JobID,
		"segment_counts", input.SegmentCounts,
	)
	return nil
}

// PublishSubtitleDetectJob publishes a job to subtitle_detect worker
func (p *JobPublisher) PublishSubtitleDetectJob(ctx context.Context, meta *domain.JobMeta, input *domain.SubtitleDetectJobInput) error {
	job := DownstreamJob{
		Meta: domain.JobMeta{
			JobID:      uuid.New(),
			JobType:    "subtitle_detect",
			EntityType: meta.EntityType,
			EntityID:   meta.EntityID,
			EntityCode: meta.EntityCode,
			Priority:   meta.Priority,
			RetryCount: 0,
			MaxRetries: 3,
			TimeoutSec: 1800, // 30 minutes for subtitle
			CreatedAt:  time.Now().Unix(),
		},
		Input: input,
	}

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal subtitle_detect job: %w", err)
	}

	_, err = p.js.Publish(ctx, "jobs.subtitle.detect", data)
	if err != nil {
		p.logger.Error("Failed to publish subtitle_detect job",
			"error", err,
			"entity_code", meta.EntityCode,
		)
		return err
	}

	p.logger.Info("SubtitleDetect job published",
		"entity_code", meta.EntityCode,
		"job_id", job.Meta.JobID,
		"audio_path", input.AudioPath,
	)
	return nil
}
