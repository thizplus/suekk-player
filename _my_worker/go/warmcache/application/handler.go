package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/suekk/my_worker/go/warmcache/config"
	"github.com/suekk/my_worker/go/warmcache/domain"
	"github.com/suekk/my_worker/go/warmcache/domain/ports"
	"github.com/suekk/my_worker/go/warmcache/infrastructure/progress"
)

// Stage constants ตาม ___WORKER_INTERFACE_STANDARD.md
const (
	StageInitializing = "initializing" // 0-5%
	StageWarming      = "warming"      // 5-80%
	StageVerifying    = "verifying"    // 80-95%
	StageFinalizing   = "finalizing"   // 95-99%
	StageCompleted    = "completed"    // 100%
	StageFailed       = "failed"
)

// Handler handles warmcache jobs
type Handler struct {
	config    *config.Config
	cdn       ports.CDNPort
	progress  ports.ProgressPort
	throttler *progress.Throttler
	logger    *slog.Logger
}

// NewHandler creates a new handler
func NewHandler(cfg *config.Config, cdn ports.CDNPort, progressPort ports.ProgressPort) *Handler {
	return &Handler{
		config:    cfg,
		cdn:       cdn,
		progress:  progressPort,
		throttler: progress.NewThrottler(progress.DefaultConfig()),
		logger:    slog.Default().With("component", "warmcache-handler"),
	}
}

// HandleJob processes a warmcache job with startedAt timestamp from consumer
func (h *Handler) HandleJob(ctx context.Context, job *domain.WarmCacheJob, startedAt time.Time) error {
	meta := job.Meta
	input := job.Input

	h.logger.Info("Starting warmcache job",
		"job_id", meta.JobID,
		"entity_code", meta.EntityCode,
		"hls_path", input.HLSPath,
		"segment_counts", input.SegmentCounts,
		"started_at", startedAt,
	)

	// Use override threshold if provided
	verifyThreshold := h.config.VerifyThreshold
	if input.VerifyThreshold > 0 {
		verifyThreshold = input.VerifyThreshold
	}

	// Publish initializing
	h.publishProgress(ctx, &meta, StageInitializing, 0, "เริ่มต้น warm cache")

	var lastErr error
	for attempt := 1; attempt <= h.config.MaxRetries; attempt++ {
		// 1. Warm all segments (5-80%)
		h.publishProgress(ctx, &meta, StageWarming, 5, fmt.Sprintf("อุ่น cache (รอบ %d/%d)", attempt, h.config.MaxRetries))

		warmResult, err := h.cdn.WarmCache(ctx, meta.EntityCode, input.HLSPath, input.SegmentCounts, func(percent float64, completed, total int) {
			// Scale warming progress to 5-80%
			scaledProgress := 5 + (percent * 0.75)
			h.publishProgress(ctx, &meta, StageWarming, scaledProgress, fmt.Sprintf("อุ่น cache (%d/%d segments)", completed, total))
		})
		if err != nil {
			lastErr = err
			h.logger.Warn("Warming failed", "error", err, "attempt", attempt)
			continue
		}

		// 2. Verify cache (80-95%)
		h.publishProgress(ctx, &meta, StageVerifying, 80, fmt.Sprintf("ตรวจสอบ cache (รอบ %d/%d)", attempt, h.config.MaxRetries))

		verifyResult, err := h.cdn.VerifyCache(ctx, meta.EntityCode, input.HLSPath, input.SegmentCounts)
		if err != nil {
			lastErr = err
			h.logger.Warn("Verification failed", "error", err, "attempt", attempt)
			continue
		}

		// Scale verify progress
		verifyProgress := 80 + (float64(attempt) / float64(h.config.MaxRetries) * 15)
		h.publishProgress(ctx, &meta, StageVerifying, verifyProgress, fmt.Sprintf("Cache hit: %.1f%%", verifyResult.HitPercent))

		h.logger.Info("Attempt completed",
			"job_id", meta.JobID,
			"attempt", fmt.Sprintf("%d/%d", attempt, h.config.MaxRetries),
			"warm_total", warmResult.TotalSegments,
			"warm_hit", warmResult.CachedHit,
			"warm_miss", warmResult.CachedMiss,
			"verify_hit_pct", fmt.Sprintf("%.1f%%", verifyResult.HitPercent),
			"verify_passed", verifyResult.Passed,
		)

		if verifyResult.HitPercent >= float64(verifyThreshold) {
			// Success!
			duration := time.Since(startedAt).Seconds()

			output := &domain.WarmCacheOutput{
				TotalSegments:      warmResult.TotalSegments,
				CachedHit:          warmResult.CachedHit,
				CachedMiss:         warmResult.CachedMiss,
				Failed:             warmResult.Failed,
				HitPercent:         verifyResult.HitPercent,
				VerificationPassed: true,
			}

			h.publishCompleted(ctx, &meta, duration, output)
			h.logger.Info("Job completed successfully",
				"job_id", meta.JobID,
				"entity_code", meta.EntityCode,
				"cache_hit_percent", fmt.Sprintf("%.1f%%", verifyResult.HitPercent),
				"duration_sec", fmt.Sprintf("%.1f", duration),
			)
			return nil
		}

		lastErr = fmt.Errorf("verification failed: %.1f%% < %d%% threshold",
			verifyResult.HitPercent, verifyThreshold)

		if attempt < h.config.MaxRetries {
			delay := time.Duration(attempt*2) * time.Second
			h.logger.Warn("Verification below threshold, retrying",
				"hit_percent", verifyResult.HitPercent,
				"threshold", verifyThreshold,
				"delay", delay,
			)
			time.Sleep(delay)
		}
	}

	// All retries failed
	duration := time.Since(startedAt).Seconds()
	h.publishFailed(ctx, &meta, lastErr, duration)

	h.logger.Error("Job failed after all retries",
		"job_id", meta.JobID,
		"entity_code", meta.EntityCode,
		"error", lastErr,
	)

	return lastErr
}

// publishProgress sends a progress update (with throttling)
func (h *Handler) publishProgress(ctx context.Context, meta *domain.JobMeta, stage string, pct float64, message string) {
	// Throttle check
	if !h.throttler.ShouldSend(meta.JobID.String(), pct, "processing") {
		return
	}

	now := time.Now()
	update := &ports.ProgressUpdate{
		JobID:      meta.JobID,
		JobType:    meta.JobType,
		EntityType: meta.EntityType,
		EntityID:   meta.EntityID,
		EntityCode: meta.EntityCode,
		Status:     "processing",
		Progress:   pct,
		Stage:      stage,
		Message:    message,
		UpdatedAt:  now,
	}

	if err := h.progress.Publish(ctx, update); err != nil {
		h.logger.Warn("Failed to publish progress", "error", err)
	}
}

// publishCompleted sends completion status (always sends, bypasses throttle)
func (h *Handler) publishCompleted(ctx context.Context, meta *domain.JobMeta, durationSec float64, output *domain.WarmCacheOutput) {
	h.throttler.Cleanup(meta.JobID.String())
	now := time.Now()
	update := &ports.ProgressUpdate{
		JobID:       meta.JobID,
		JobType:     meta.JobType,
		EntityType:  meta.EntityType,
		EntityID:    meta.EntityID,
		EntityCode:  meta.EntityCode,
		Status:      "completed",
		Progress:    100,
		Stage:       StageCompleted,
		Message:     fmt.Sprintf("Cache อุ่นแล้ว (%.1f%% HIT)", output.HitPercent),
		UpdatedAt:   now,
		DurationSec: &durationSec,
		Output:      output,
	}

	if err := h.progress.Publish(ctx, update); err != nil {
		h.logger.Warn("Failed to publish completed", "error", err)
	}
}

// publishFailed sends failure status (always sends, bypasses throttle)
func (h *Handler) publishFailed(ctx context.Context, meta *domain.JobMeta, jobErr error, durationSec float64) {
	h.throttler.Cleanup(meta.JobID.String())
	now := time.Now()
	update := &ports.ProgressUpdate{
		JobID:       meta.JobID,
		JobType:     meta.JobType,
		EntityType:  meta.EntityType,
		EntityID:    meta.EntityID,
		EntityCode:  meta.EntityCode,
		Status:      "failed",
		Progress:    0,
		Stage:       StageFailed,
		Message:     "อุ่น cache ล้มเหลว",
		Error:       jobErr.Error(),
		UpdatedAt:   now,
		DurationSec: &durationSec,
		ErrorDetails: &ports.ErrorInfo{
			Code:        domain.ErrCodeWarmCacheVerifyFailed,
			Stage:       StageVerifying,
			IsRetryable: true,
		},
	}

	if err := h.progress.Publish(ctx, update); err != nil {
		h.logger.Warn("Failed to publish failed", "error", err)
	}
}
