package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/suekk/my_worker/go/warmcache/config"
	"github.com/suekk/my_worker/go/warmcache/domain"
)

// JobHandler is a function that handles a job with startedAt timestamp
type JobHandler func(ctx context.Context, job *domain.WarmCacheJob, startedAt time.Time) error

// Consumer consumes jobs from NATS JetStream
type Consumer struct {
	conn     *nats.Conn
	js       jetstream.JetStream
	consumer jetstream.Consumer
	config   *config.Config
	logger   *slog.Logger
}

// NewConsumer creates a new NATS JetStream consumer
func NewConsumer(cfg *config.Config) (*Consumer, error) {
	logger := slog.Default().With("component", "nats-consumer")

	// Connect to NATS
	nc, err := nats.Connect(cfg.NATSURL,
		nats.Name(cfg.WorkerID),
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	logger.Info("Connected to NATS", "url", cfg.NATSURL)

	// Create JetStream context
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	// Get existing consumer (should be created by API/infra)
	ctx := context.Background()
	consumer, err := js.Consumer(ctx, cfg.StreamName, cfg.ConsumerName)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to get consumer %s/%s: %w", cfg.StreamName, cfg.ConsumerName, err)
	}

	logger.Info("Consumer ready",
		"stream", cfg.StreamName,
		"consumer", cfg.ConsumerName,
		"subject", cfg.Subject,
	)

	return &Consumer{
		conn:     nc,
		js:       js,
		consumer: consumer,
		config:   cfg,
		logger:   logger,
	}, nil
}

// Start begins consuming jobs
func (c *Consumer) Start(ctx context.Context, handler JobHandler) error {
	c.logger.Info("Starting job consumer")

	// Consume messages
	iter, err := c.consumer.Messages(
		jetstream.PullMaxMessages(c.config.Concurrency),
	)
	if err != nil {
		return fmt.Errorf("failed to create message iterator: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Context cancelled, stopping consumer")
			iter.Stop()
			return nil
		default:
		}

		msg, err := iter.Next()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			c.logger.Warn("Failed to get next message", "error", err)
			time.Sleep(time.Second)
			continue
		}

		c.processMessage(ctx, msg, handler)
	}
}

// processMessage handles a single message
func (c *Consumer) processMessage(ctx context.Context, msg jetstream.Msg, handler JobHandler) {
	// Parse job
	var job domain.WarmCacheJob
	if err := json.Unmarshal(msg.Data(), &job); err != nil {
		c.logger.Error("Failed to parse job", "error", err)
		msg.Term()
		return
	}

	// Record job start time
	startedAt := time.Now()

	c.logger.Info("Processing job",
		"job_id", job.Meta.JobID,
		"entity_code", job.Meta.EntityCode,
		"entity_type", job.Meta.EntityType,
		"started_at", startedAt,
	)

	// Start heartbeat goroutine for InProgress (every 30 seconds per standard)
	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	go c.heartbeat(heartbeatCtx, msg)

	// Process job with startedAt
	err := handler(ctx, &job, startedAt)
	cancelHeartbeat()

	if err != nil {
		c.logger.Error("Job failed",
			"job_id", job.Meta.JobID,
			"error", err,
			"retry_count", job.Meta.RetryCount,
			"max_retries", job.Meta.MaxRetries,
		)

		// Nack with delay for retry
		if job.Meta.RetryCount < job.Meta.MaxRetries {
			msg.NakWithDelay(time.Duration(job.Meta.RetryCount+1) * 30 * time.Second)
		} else {
			msg.Term()
		}
		return
	}

	c.logger.Info("Job completed", "job_id", job.Meta.JobID)
	msg.Ack()
}

// heartbeat sends InProgress to extend ack wait every 30 seconds (per standard)
func (c *Consumer) heartbeat(ctx context.Context, msg jetstream.Msg) {
	// Send heartbeat every 30 seconds per ___WORKER_INTERFACE_STANDARD.md
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := msg.InProgress(); err != nil {
				c.logger.Warn("Failed to send InProgress", "error", err)
			} else {
				c.logger.Debug("InProgress heartbeat sent")
			}
		}
	}
}

// Connection returns the NATS connection (for progress publisher)
func (c *Consumer) Connection() *nats.Conn {
	return c.conn
}

// Close closes the consumer and connection
func (c *Consumer) Close() error {
	c.conn.Close()
	return nil
}
