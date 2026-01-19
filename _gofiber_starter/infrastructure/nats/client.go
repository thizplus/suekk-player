package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"gofiber-template/pkg/logger"
)

// Client wraps NATS connection with JetStream context
type Client struct {
	conn           *nats.Conn
	js             jetstream.JetStream
	stream         jetstream.Stream  // Transcode jobs stream
	subtitleStream jetstream.Stream  // Subtitle jobs stream

	// KV Buckets
	workerKV jetstream.KeyValue // Worker status (from heartbeat)
}

// ClientConfig configuration สำหรับ NATS Client
type ClientConfig struct {
	URL string // nats://localhost:4222
}

// NewClient สร้าง NATS Client พร้อม JetStream
func NewClient(cfg ClientConfig) (*Client, error) {
	// Connect to NATS
	nc, err := nats.Connect(cfg.URL,
		nats.MaxReconnects(-1),           // Reconnect forever
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				logger.Warn("NATS disconnected", "error", err)
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info("NATS reconnected", "url", nc.ConnectedUrl())
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create JetStream context
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	client := &Client{
		conn: nc,
		js:   js,
	}

	// Setup Stream
	if err := client.setupStream(context.Background()); err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to setup stream: %w", err)
	}

	// Setup KV Buckets for worker status
	if err := client.setupKVBuckets(context.Background()); err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to setup KV buckets: %w", err)
	}

	logger.Info("NATS client initialized", "url", cfg.URL, "stream", StreamName)
	return client, nil
}

// setupStream สร้างหรืออัปเดต Streams
func (c *Client) setupStream(ctx context.Context) error {
	// Transcode jobs stream
	transcodeCfg := jetstream.StreamConfig{
		Name:        StreamName,
		Subjects:    []string{SubjectJobs},
		Storage:     jetstream.FileStorage,      // Persistent storage
		Retention:   jetstream.WorkQueuePolicy,  // ลบ message หลัง Ack
		MaxAge:      24 * time.Hour,             // เก็บ message ไม่เกิน 24 ชม.
		Replicas:    1,
		Description: "Transcode job queue",
	}

	stream, err := c.js.CreateOrUpdateStream(ctx, transcodeCfg)
	if err != nil {
		return fmt.Errorf("failed to create/update transcode stream: %w", err)
	}
	c.stream = stream
	logger.Info("JetStream stream ready", "name", StreamName)

	// Subtitle jobs stream
	subtitleCfg := jetstream.StreamConfig{
		Name:     SubtitleStreamName,
		Subjects: []string{
			SubjectSubtitleDetect,
			SubjectSubtitleTranscribe,
			SubjectSubtitleTranslate,
		},
		Storage:     jetstream.FileStorage,
		Retention:   jetstream.WorkQueuePolicy,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
		Description: "Subtitle job queue (detect, transcribe, translate)",
	}

	subtitleStream, err := c.js.CreateOrUpdateStream(ctx, subtitleCfg)
	if err != nil {
		return fmt.Errorf("failed to create/update subtitle stream: %w", err)
	}
	c.subtitleStream = subtitleStream
	logger.Info("JetStream stream ready", "name", SubtitleStreamName)

	return nil
}

// setupKVBuckets สร้าง KV buckets
func (c *Client) setupKVBuckets(ctx context.Context) error {
	// Worker Status KV - อ่านจาก bucket ที่ Worker สร้าง (ไม่สร้างใหม่ ถ้าไม่มี)
	workerKV, err := c.js.KeyValue(ctx, "WORKER_STATUS")
	if err != nil {
		// KV อาจยังไม่มี ถ้า worker ยังไม่เริ่ม - ไม่ถือว่า error
		logger.Warn("Worker status KV not available (worker not started yet)", "error", err)
	} else {
		c.workerKV = workerKV
		logger.Info("NATS KV bucket ready", "bucket", "WORKER_STATUS")
	}

	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Getters
// ═══════════════════════════════════════════════════════════════════════════════

// Conn returns the underlying NATS connection
func (c *Client) Conn() *nats.Conn {
	return c.conn
}

// JetStream returns the JetStream context
func (c *Client) JetStream() jetstream.JetStream {
	return c.js
}

// Stream returns the configured stream
func (c *Client) Stream() jetstream.Stream {
	return c.stream
}

// WorkerKV returns the worker status KV bucket
func (c *Client) WorkerKV() jetstream.KeyValue {
	return c.workerKV
}

// RefreshWorkerKV พยายามเชื่อมต่อ Worker KV อีกครั้ง (กรณี worker เพิ่งเริ่ม)
func (c *Client) RefreshWorkerKV(ctx context.Context) error {
	workerKV, err := c.js.KeyValue(ctx, "WORKER_STATUS")
	if err != nil {
		return err
	}
	c.workerKV = workerKV
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// JetStream Status (สำหรับ Monitoring API)
// ═══════════════════════════════════════════════════════════════════════════════

// GetStatus ดึงสถานะของ JetStream stream และ consumer
func (c *Client) GetStatus(ctx context.Context) (*JetStreamStatus, error) {
	// Get stream info
	streamInfo, err := c.stream.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get stream info: %w", err)
	}

	// Get consumer info (may not exist if worker hasn't started)
	var consumerInfo ConsumerInfo
	consumer, err := c.stream.Consumer(ctx, ConsumerName)
	if err == nil {
		ci, err := consumer.Info(ctx)
		if err == nil {
			consumerInfo = ConsumerInfo{
				Name:          ci.Name,
				NumPending:    ci.NumPending,
				NumAckPending: ci.NumAckPending,
				Redelivered:   uint64(ci.NumRedelivered),
			}
		}
	}

	return &JetStreamStatus{
		Stream: StreamInfo{
			Name:     streamInfo.Config.Name,
			Messages: streamInfo.State.Msgs,
			Bytes:    streamInfo.State.Bytes,
			FirstSeq: streamInfo.State.FirstSeq,
			LastSeq:  streamInfo.State.LastSeq,
		},
		Consumer: consumerInfo,
	}, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Lifecycle
// ═══════════════════════════════════════════════════════════════════════════════

// Close ปิด NATS connection
func (c *Client) Close() error {
	if c.conn != nil {
		c.conn.Close()
		logger.Info("NATS connection closed")
	}
	return nil
}

// Ping ทดสอบ connection
func (c *Client) Ping() error {
	return c.conn.FlushTimeout(5 * time.Second)
}

// IsConnected ตรวจสอบว่าเชื่อมต่ออยู่หรือไม่
func (c *Client) IsConnected() bool {
	return c.conn != nil && c.conn.IsConnected()
}
