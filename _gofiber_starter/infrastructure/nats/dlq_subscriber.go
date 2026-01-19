package nats

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"gofiber-template/domain/ports"
	"gofiber-template/pkg/logger"
)

const (
	// DLQ Stream and Consumer names
	StreamNameDLQ    = "TRANSCODE_DLQ"
	SubjectDLQ       = "dlq.transcode"
	ConsumerNameDLQ  = "DLQ_NOTIFIER"
)

// DLQJob - Job ที่ถูกย้ายไป Dead Letter Queue (ต้องตรงกับ worker)
type DLQJob struct {
	OriginalJob  TranscodeJobData `json:"original_job"`
	Error        string           `json:"error"`
	Attempts     int              `json:"attempts"`
	WorkerID     string           `json:"worker_id"`
	FailedAt     int64            `json:"failed_at"`
	Stage        string           `json:"stage"` // download, transcode, upload
}

// TranscodeJobData - (ต้องตรงกับ worker)
type TranscodeJobData struct {
	VideoID      string   `json:"video_id"`
	VideoCode    string   `json:"video_code"`
	InputPath    string   `json:"input_path"`
	OutputPath   string   `json:"output_path"`
	Codec        string   `json:"codec"`
	Qualities    []string `json:"qualities"`
	UseByteRange bool     `json:"use_byte_range"`
	RetryCount   int      `json:"retry_count"`
	CreatedAt    int64    `json:"created_at"`
}

// DLQSubscriber - Subscribes to DLQ and sends notifications
type DLQSubscriber struct {
	js         jetstream.JetStream
	notifier   ports.NotifierPort
	consumer   jetstream.Consumer
	cancelFunc context.CancelFunc
	running    bool
}

// NewDLQSubscriber สร้าง DLQSubscriber
func NewDLQSubscriber(nc *nats.Conn, notifier ports.NotifierPort) (*DLQSubscriber, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, err
	}

	return &DLQSubscriber{
		js:       js,
		notifier: notifier,
	}, nil
}

// Start เริ่ม subscribe และส่ง notifications
func (s *DLQSubscriber) Start(ctx context.Context) error {
	if s.running {
		return nil
	}

	// สร้าง DLQ stream ถ้ายังไม่มี (ในกรณีที่ worker ยังไม่เคย publish)
	_, err := s.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:        StreamNameDLQ,
		Description: "Dead Letter Queue for failed transcode jobs",
		Subjects:    []string{SubjectDLQ},
		Retention:   jetstream.LimitsPolicy,
		MaxAge:      30 * 24 * time.Hour, // 30 days
		Storage:     jetstream.FileStorage,
		Replicas:    1,
	})
	if err != nil {
		logger.Error("Failed to create DLQ stream", "error", err)
		return err
	}

	// สร้าง durable consumer
	consumer, err := s.js.CreateOrUpdateConsumer(ctx, StreamNameDLQ, jetstream.ConsumerConfig{
		Durable:       ConsumerNameDLQ,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverNewPolicy, // Only new messages after start
		AckWait:       30 * time.Second,
		MaxDeliver:    1, // Don't retry notification failures
	})
	if err != nil {
		logger.Error("Failed to create DLQ consumer", "error", err)
		return err
	}
	s.consumer = consumer

	// Start consuming in background
	subCtx, cancel := context.WithCancel(ctx)
	s.cancelFunc = cancel
	s.running = true

	go s.consume(subCtx)

	logger.Info("DLQ Subscriber started")
	return nil
}

// consume รับ messages และส่ง notifications
func (s *DLQSubscriber) consume(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			logger.Info("DLQ Subscriber stopping...")
			return
		default:
			// Fetch messages with timeout
			msgs, err := s.consumer.Fetch(1, jetstream.FetchMaxWait(5*time.Second))
			if err != nil {
				// ไม่ใช่ error ถ้า timeout
				continue
			}

			for msg := range msgs.Messages() {
				s.handleMessage(ctx, msg)
			}
		}
	}
}

// handleMessage ประมวลผล DLQ message และส่ง notification
func (s *DLQSubscriber) handleMessage(ctx context.Context, msg jetstream.Msg) {
	var dlqJob DLQJob
	if err := json.Unmarshal(msg.Data(), &dlqJob); err != nil {
		logger.Error("Failed to unmarshal DLQ job", "error", err)
		msg.Ack() // Ack anyway to prevent redelivery
		return
	}

	logger.Info("Processing DLQ notification",
		"video_id", dlqJob.OriginalJob.VideoID,
		"video_code", dlqJob.OriginalJob.VideoCode,
		"attempts", dlqJob.Attempts,
		"stage", dlqJob.Stage,
	)

	// ส่ง Telegram notification
	notification := &ports.DLQNotification{
		VideoID:   dlqJob.OriginalJob.VideoID,
		VideoCode: dlqJob.OriginalJob.VideoCode,
		Title:     dlqJob.OriginalJob.VideoCode, // ไม่มี title ใน job, ใช้ code แทน
		Error:     dlqJob.Error,
		Attempts:  dlqJob.Attempts,
		WorkerID:  dlqJob.WorkerID,
		Stage:     dlqJob.Stage,
		FailedAt:  time.Unix(dlqJob.FailedAt, 0).Format("2006-01-02 15:04:05"),
	}

	if err := s.notifier.SendDLQAlert(ctx, notification); err != nil {
		logger.Warn("Failed to send DLQ notification", "error", err)
	}

	// Ack message
	msg.Ack()
}

// Stop หยุด subscriber
func (s *DLQSubscriber) Stop() {
	if !s.running {
		return
	}

	if s.cancelFunc != nil {
		s.cancelFunc()
	}
	s.running = false

	logger.Info("DLQ Subscriber stopped")
}
