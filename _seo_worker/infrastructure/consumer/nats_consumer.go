package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"seo-worker/domain/models"
	"seo-worker/domain/ports"
)

type NATSConsumer struct {
	nc       *nats.Conn
	js       jetstream.JetStream
	consumer jetstream.Consumer
	handler  ports.JobHandler
	logger   *slog.Logger

	// State
	running atomic.Bool
	paused  atomic.Bool
	wg      sync.WaitGroup

	// Config
	config NATSConsumerConfig
}

type NATSConsumerConfig struct {
	URL             string
	Stream          string
	Subject         string
	ConsumerName    string
	Concurrency     int
	ShutdownTimeout time.Duration
}

func NewNATSConsumer(cfg NATSConsumerConfig) (*NATSConsumer, error) {
	nc, err := nats.Connect(cfg.URL,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	return &NATSConsumer{
		nc:     nc,
		js:     js,
		config: cfg,
		logger: slog.Default().With("component", "nats_consumer"),
	}, nil
}

func (c *NATSConsumer) SetHandler(handler ports.JobHandler) {
	c.handler = handler
}

func (c *NATSConsumer) Start(ctx context.Context) error {
	if c.handler == nil {
		return fmt.Errorf("handler not set")
	}

	// Create or get stream
	stream, err := c.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      c.config.Stream,
		Subjects:  []string{c.config.Subject},
		Retention: jetstream.WorkQueuePolicy,
		MaxAge:    7 * 24 * time.Hour, // 7 days
	})
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}

	// Create or get consumer
	consumer, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Name:          c.config.ConsumerName,
		Durable:       c.config.ConsumerName,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    3, // Retry 3 times then DLQ
		AckWait:       5 * time.Minute,
		FilterSubject: c.config.Subject,
	})
	if err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}
	c.consumer = consumer

	c.running.Store(true)
	c.logger.Info("Consumer started",
		"stream", c.config.Stream,
		"consumer", c.config.ConsumerName,
		"concurrency", c.config.Concurrency,
	)

	// Start consuming with Consume API
	consumeCtx, err := consumer.Consume(func(msg jetstream.Msg) {
		if c.paused.Load() {
			// Requeue if paused
			msg.Nak()
			return
		}

		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			c.processMessage(ctx, msg)
		}()
	})
	if err != nil {
		return fmt.Errorf("failed to start consuming: %w", err)
	}
	defer consumeCtx.Stop()

	// Wait for context cancellation
	<-ctx.Done()
	c.logger.Info("Context cancelled, stopping consumer")
	c.running.Store(false)
	c.wg.Wait()
	return nil
}

func (c *NATSConsumer) processMessage(ctx context.Context, msg jetstream.Msg) {
	var job models.SEOArticleJob
	if err := json.Unmarshal(msg.Data(), &job); err != nil {
		c.logger.Error("Failed to unmarshal job", "error", err)
		msg.Term() // Terminal error, don't retry
		return
	}

	c.logger.Info("Processing job",
		"video_id", job.VideoID,
		"video_code", job.VideoCode,
	)

	// Process job
	if err := c.handler(ctx, &job); err != nil {
		c.logger.Error("Job failed",
			"video_id", job.VideoID,
			"error", err,
		)
		// NAK to retry (or send to DLQ after max retries)
		msg.Nak()
		return
	}

	// Success
	msg.Ack()
	c.logger.Info("Job completed",
		"video_id", job.VideoID,
	)
}

func (c *NATSConsumer) Stop() {
	c.running.Store(false)
	c.wg.Wait()
	if c.nc != nil {
		c.nc.Close()
	}
	c.logger.Info("Consumer stopped")
}

func (c *NATSConsumer) IsRunning() bool {
	return c.running.Load()
}

func (c *NATSConsumer) IsPaused() bool {
	return c.paused.Load()
}

func (c *NATSConsumer) Pause() {
	c.paused.Store(true)
	c.logger.Info("Consumer paused")
}

func (c *NATSConsumer) Resume() {
	c.paused.Store(false)
	c.logger.Info("Consumer resumed")
}

// Verify interface implementation
var _ ports.ConsumerPort = (*NATSConsumer)(nil)
