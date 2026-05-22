package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/suekk/my_worker/go/transcode/config"
	"github.com/suekk/my_worker/go/transcode/domain"
)

// JobHandler is a function that handles a job with startedAt timestamp
type JobHandler func(ctx context.Context, job *domain.TranscodeJob, startedAt time.Time) error

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
	c.logger.Info("Starting job consumer", "concurrency", c.config.Concurrency)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Context cancelled, stopping consumer")
			return nil
		default:
		}

		// Fetch with timeout (30 seconds) - same pattern as old worker
		// This allows graceful shutdown by letting the loop check ctx.Done()
		msgs, err := c.consumer.Fetch(1, jetstream.FetchMaxWait(30*time.Second))
		if err != nil {
			// Timeout is normal - just continue to check ctx.Done()
			if err.Error() == "nats: timeout" {
				continue
			}
			if ctx.Err() != nil {
				return nil
			}
			c.logger.Warn("Failed to fetch messages", "error", err)
			time.Sleep(time.Second)
			continue
		}

		// Process fetched messages
		for msg := range msgs.Messages() {
			// Check context before processing
			select {
			case <-ctx.Done():
				c.logger.Info("Context cancelled during message processing")
				return nil
			default:
			}

			c.processMessage(ctx, msg, handler)
		}
	}
}

// processMessage handles a single message
func (c *Consumer) processMessage(ctx context.Context, msg jetstream.Msg, handler JobHandler) {
	// Parse job
	var job domain.TranscodeJob
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
		"input_path", job.Input.InputPath,
		"qualities", job.Input.Qualities,
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

		// Check if error is permanent or retryable
		if isPermanentError(err) {
			c.logger.Warn("Permanent error, terminating job", "job_id", job.Meta.JobID)
			msg.Term()
		} else if job.Meta.RetryCount < job.Meta.MaxRetries {
			// Nack with delay for retry
			delay := time.Duration(job.Meta.RetryCount+1) * 30 * time.Second
			msg.NakWithDelay(delay)
		} else {
			c.logger.Warn("Max retries exceeded, terminating job", "job_id", job.Meta.JobID)
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

// Connection returns the NATS connection
func (c *Consumer) Connection() *nats.Conn {
	return c.conn
}

// JetStream returns the JetStream context
func (c *Consumer) JetStream() jetstream.JetStream {
	return c.js
}

// Close closes the consumer and connection
func (c *Consumer) Close() error {
	c.conn.Close()
	return nil
}

// isPermanentError checks if error should not be retried
func isPermanentError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Permanent errors (don't retry)
	permanentPatterns := []string{
		"not found",
		"no such key",
		"does not exist",
		"invalid input",
		"validation failed",
		"unsupported format",
		"codec not found",
		"invalid data",
		"access denied",
		"forbidden",
		"unauthorized",
		"no video stream",
		"no audio stream",
	}

	for _, pattern := range permanentPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}
