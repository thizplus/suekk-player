package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
	"github.com/suekk/my_worker/go/gallery/domain/ports"
)

// ProgressPublisher publishes progress updates via NATS
type ProgressPublisher struct {
	nc       *nats.Conn
	workerID string
	logger   *slog.Logger
}

// NewProgressPublisher creates a new progress publisher
func NewProgressPublisher(nc *nats.Conn, workerID string) *ProgressPublisher {
	return &ProgressPublisher{
		nc:       nc,
		workerID: workerID,
		logger:   slog.Default().With("component", "progress-publisher"),
	}
}

// Publish sends a progress update
func (p *ProgressPublisher) Publish(ctx context.Context, update *ports.ProgressUpdate) error {
	// Set worker ID
	update.WorkerID = p.workerID

	// Serialize
	data, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("failed to marshal progress: %w", err)
	}

	// Subject format: progress.{entity_type}.{entity_id}
	subject := fmt.Sprintf("progress.%s.%s", update.EntityType, update.EntityID.String())

	// Publish
	if err := p.nc.Publish(subject, data); err != nil {
		return fmt.Errorf("failed to publish progress: %w", err)
	}

	p.logger.Debug("Progress published",
		"subject", subject,
		"status", update.Status,
		"progress", update.Progress,
		"stage", update.Stage,
	)

	return nil
}
