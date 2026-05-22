package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"

	"github.com/nats-io/nats.go"
	"github.com/suekk/my_worker/go/transcode/domain/ports"
)

// ProgressPublisher publishes progress updates via NATS Pub/Sub
type ProgressPublisher struct {
	conn     *nats.Conn
	workerID string
	logger   *slog.Logger
}

// NewProgressPublisher creates a new progress publisher
func NewProgressPublisher(conn *nats.Conn, workerID string) *ProgressPublisher {
	return &ProgressPublisher{
		conn:     conn,
		workerID: workerID,
		logger:   slog.Default().With("component", "progress-publisher", "worker_id", workerID),
	}
}

// Publish sends a progress update to NATS
// Subject format: progress.{entity_type}.{entity_id}
func (p *ProgressPublisher) Publish(ctx context.Context, update *ports.ProgressUpdate) error {
	update.WorkerID = p.workerID

	// Round progress to 2 decimal places
	update.Progress = math.Round(update.Progress*100) / 100

	data, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("failed to marshal progress update: %w", err)
	}

	// Publish to progress.{entity_type}.{entity_id}
	subject := fmt.Sprintf("progress.%s.%s", update.EntityType, update.EntityID)
	if err := p.conn.Publish(subject, data); err != nil {
		p.logger.Error("Failed to publish progress",
			"error", err,
			"job_id", update.JobID,
			"entity_id", update.EntityID,
		)
		return err
	}

	p.logger.Info("Progress published",
		"job_id", update.JobID,
		"progress", update.Progress,
		"stage", update.Stage,
		"message", update.Message,
	)

	return nil
}
