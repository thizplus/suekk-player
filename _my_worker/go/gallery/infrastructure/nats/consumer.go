package nats

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/suekk/my_worker/go/gallery/config"
	"github.com/suekk/my_worker/go/gallery/domain"
)

// JobHandler is the function signature for processing jobs with startedAt
type JobHandler func(ctx context.Context, job *domain.GalleryJob, startedAt time.Time) error

// Consumer handles NATS JetStream consumption
type Consumer struct {
	nc       *nats.Conn
	js       jetstream.JetStream
	consumer jetstream.Consumer
	config   *config.Config
	logger   *slog.Logger
}

// NewConsumer creates a new NATS JetStream consumer
// Note: Stream and Consumer should be pre-created by Backend (API Server)
func NewConsumer(cfg *config.Config) (*Consumer, error) {
	logger := slog.Default().With("component", "nats-consumer")

	// Connect to NATS
	nc, err := nats.Connect(cfg.NATSURL,
		nats.Name(cfg.WorkerID),
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				logger.Warn("NATS disconnected", "error", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info("NATS reconnected", "url", nc.ConnectedUrl())
		}),
	)
	if err != nil {
		return nil, err
	}

	logger.Info("Connected to NATS", "url", cfg.NATSURL)

	// Create JetStream context
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, err
	}

	// Get existing consumer (should be created by Backend/API)
	ctx := context.Background()
	consumer, err := js.Consumer(ctx, cfg.StreamName, cfg.ConsumerName)
	if err != nil {
		nc.Close()
		return nil, err
	}

	logger.Info("Consumer ready",
		"stream", cfg.StreamName,
		"consumer", cfg.ConsumerName,
		"subject", cfg.Subject,
	)

	return &Consumer{
		nc:       nc,
		js:       js,
		consumer: consumer,
		config:   cfg,
		logger:   logger,
	}, nil
}

// Start begins consuming messages
func (c *Consumer) Start(ctx context.Context, handler JobHandler) error {
	c.logger.Info("Starting consumer",
		"stream", c.config.StreamName,
		"subject", c.config.Subject,
		"concurrency", c.config.Concurrency,
	)

	// Create message handler
	msgs, err := c.consumer.Messages()
	if err != nil {
		return err
	}

	// Semaphore for concurrency control
	sem := make(chan struct{}, c.config.Concurrency)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Context cancelled, stopping consumer")
			msgs.Stop()
			return nil

		default:
			msg, err := msgs.Next()
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				c.logger.Warn("Error fetching message", "error", err)
				continue
			}

			// Acquire semaphore
			sem <- struct{}{}

			go func(msg jetstream.Msg) {
				defer func() { <-sem }()
				c.processMessage(ctx, msg, handler)
			}(msg)
		}
	}
}

// processMessage handles a single message
func (c *Consumer) processMessage(ctx context.Context, msg jetstream.Msg, handler JobHandler) {
	// Parse job
	var job domain.GalleryJob
	if err := json.Unmarshal(msg.Data(), &job); err != nil {
		c.logger.Error("Failed to parse job", "error", err)
		msg.Term() // Don't retry invalid messages
		return
	}

	// Record job start time
	startedAt := time.Now()

	c.logger.Info("Processing job",
		"job_id", job.Meta.JobID,
		"entity_code", job.Meta.EntityCode,
		"image_count", job.Input.ImageCount,
		"started_at", startedAt,
	)

	// Create job context with timeout
	timeout := time.Duration(job.Meta.TimeoutSec) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	jobCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Start heartbeat goroutine for InProgress (every 30 seconds per standard)
	heartbeatCtx, cancelHeartbeat := context.WithCancel(jobCtx)
	go c.heartbeat(heartbeatCtx, msg)

	// Process job with startedAt
	err := handler(jobCtx, &job, startedAt)
	cancelHeartbeat()

	if err != nil {
		c.logger.Error("Job failed",
			"job_id", job.Meta.JobID,
			"error", err,
		)

		// Check if we should retry
		metadata, _ := msg.Metadata()
		if metadata != nil && metadata.NumDelivered >= uint64(job.Meta.MaxRetries) {
			c.logger.Warn("Max retries reached, terminating job",
				"job_id", job.Meta.JobID,
				"attempts", metadata.NumDelivered,
			)
			msg.Term()
		} else {
			// Nak with delay for retry
			msg.NakWithDelay(30 * time.Second)
		}
		return
	}

	// Success
	msg.Ack()
	c.logger.Info("Job completed successfully", "job_id", job.Meta.JobID)
}

// heartbeat sends InProgress to extend ack wait every 30 seconds (per standard)
func (c *Consumer) heartbeat(ctx context.Context, msg jetstream.Msg) {
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

// Connection returns the NATS connection for progress publisher
func (c *Consumer) Connection() *nats.Conn {
	return c.nc
}

// JetStream returns the JetStream context
func (c *Consumer) JetStream() jetstream.JetStream {
	return c.js
}

// Close closes the NATS connection
func (c *Consumer) Close() {
	if c.nc != nil {
		c.nc.Close()
	}
}
