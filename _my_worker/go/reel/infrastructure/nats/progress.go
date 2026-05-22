package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
	"github.com/suekk/my_worker/go/reel/domain/ports"
)

type ProgressPublisher struct {
	nc       *nats.Conn
	workerID string
	logger   *slog.Logger
}

func NewProgressPublisher(nc *nats.Conn, workerID string) *ProgressPublisher {
	return &ProgressPublisher{
		nc:       nc,
		workerID: workerID,
		logger:   slog.Default().With("component", "progress-publisher"),
	}
}

func (p *ProgressPublisher) Publish(ctx context.Context, update *ports.ProgressUpdate) error {
	update.WorkerID = p.workerID

	data, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("failed to marshal progress: %w", err)
	}

	subject := fmt.Sprintf("progress.%s.%s", update.EntityType, update.EntityID.String())

	if err := p.nc.Publish(subject, data); err != nil {
		return fmt.Errorf("failed to publish progress: %w", err)
	}

	return nil
}
