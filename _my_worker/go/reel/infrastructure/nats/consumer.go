package nats

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/suekk/my_worker/go/reel/config"
	"github.com/suekk/my_worker/go/reel/domain"
)

type JobHandler func(ctx context.Context, job *domain.ReelJob, startedAt time.Time) error

type Consumer struct {
	nc       *nats.Conn
	js       jetstream.JetStream
	consumer jetstream.Consumer
	config   *config.Config
	logger   *slog.Logger
}

func NewConsumer(cfg *config.Config) (*Consumer, error) {
	logger := slog.Default().With("component", "nats-consumer")

	nc, err := nats.Connect(cfg.NATSURL,
		nats.Name(cfg.WorkerID),
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, err
	}

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

	return &Consumer{nc: nc, js: js, consumer: consumer, config: cfg, logger: logger}, nil
}

func (c *Consumer) Start(ctx context.Context, handler JobHandler) error {
	c.logger.Info("Starting consumer", "stream", c.config.StreamName, "subject", c.config.Subject)

	msgs, err := c.consumer.Messages()
	if err != nil {
		return err
	}

	sem := make(chan struct{}, c.config.Concurrency)

	for {
		select {
		case <-ctx.Done():
			msgs.Stop()
			return nil
		default:
			msg, err := msgs.Next()
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				continue
			}

			sem <- struct{}{}
			go func(msg jetstream.Msg) {
				defer func() { <-sem }()
				c.processMessage(ctx, msg, handler)
			}(msg)
		}
	}
}

func (c *Consumer) processMessage(ctx context.Context, msg jetstream.Msg, handler JobHandler) {
	var job domain.ReelJob
	if err := json.Unmarshal(msg.Data(), &job); err != nil {
		c.logger.Error("Failed to parse job", "error", err)
		msg.Term()
		return
	}

	// Record job start time
	startedAt := time.Now()

	c.logger.Info("Processing job", "job_id", job.Meta.JobID, "entity_code", job.Meta.EntityCode, "started_at", startedAt)

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
		c.logger.Error("Job failed", "job_id", job.Meta.JobID, "error", err)
		metadata, _ := msg.Metadata()
		if metadata != nil && metadata.NumDelivered >= uint64(job.Meta.MaxRetries) {
			msg.Term()
		} else {
			msg.NakWithDelay(30 * time.Second)
		}
		return
	}

	msg.Ack()
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

func (c *Consumer) Connection() *nats.Conn { return c.nc }
func (c *Consumer) JetStream() jetstream.JetStream { return c.js }
func (c *Consumer) Close() { c.nc.Close() }
