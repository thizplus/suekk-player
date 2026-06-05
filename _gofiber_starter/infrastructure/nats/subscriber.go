package nats

import (
	"encoding/json"
	"sync"

	"github.com/nats-io/nats.go"
	"gofiber-template/pkg/logger"
)

// ProgressHandler callback function เมื่อได้รับ progress update
type ProgressHandler func(update *ProgressUpdate)

// Subscriber NATS Pub/Sub subscriber สำหรับ progress updates
type Subscriber struct {
	conn       *nats.Conn
	sub        *nats.Subscription
	handlers   []ProgressHandler
	handlersMu sync.RWMutex
	running    bool
	runningMu  sync.Mutex
}

// NewSubscriber สร้าง NATS Subscriber ใหม่
func NewSubscriber(conn *nats.Conn) *Subscriber {
	return &Subscriber{
		conn:     conn,
		handlers: make([]ProgressHandler, 0),
	}
}

// OnProgress ลงทะเบียน handler สำหรับ progress updates
func (s *Subscriber) OnProgress(handler ProgressHandler) {
	s.handlersMu.Lock()
	defer s.handlersMu.Unlock()
	s.handlers = append(s.handlers, handler)
}

// Start เริ่ม subscribe และรับข้อมูล
func (s *Subscriber) Start() error {
	s.runningMu.Lock()
	if s.running {
		s.runningMu.Unlock()
		return nil
	}
	s.running = true
	s.runningMu.Unlock()

	// Subscribe to progress.> (> matches all descendant tokens)
	// รองรับทั้ง progress.{video_id} และ progress.subtitle.{video_id}
	sub, err := s.conn.Subscribe(SubjectProgress+".>", s.handleMessage)
	if err != nil {
		return err
	}
	s.sub = sub

	logger.Info("NATS subscriber started", "subject", SubjectProgress+".>")
	return nil
}

// handleMessage จัดการ message ที่ได้รับ
func (s *Subscriber) handleMessage(msg *nats.Msg) {
	var update ProgressUpdate
	if err := json.Unmarshal(msg.Data, &update); err != nil {
		logger.Error("Failed to parse progress update", "error", err)
		return
	}

	// Parse raw output for subtitle jobs (output fields ไม่ตรงกับ TranscodeOutputData)
	if update.Status == "completed" {
		var rawMsg map[string]interface{}
		if err := json.Unmarshal(msg.Data, &rawMsg); err == nil {
			if outputRaw, ok := rawMsg["output"].(map[string]interface{}); ok {
				update.RawOutput = outputRaw
			}
		}
	}

	// Normalize: map entity_id/entity_code based on entity_type
	// subtitle entity → SubtitleID, video/other entity → VideoID
	if update.EntityID != "" {
		switch update.EntityType {
		case "subtitle":
			// Transcribe jobs: entity_id = subtitle UUID
			if update.SubtitleID == "" {
				update.SubtitleID = update.EntityID
			}
			// VideoID ต้องมาจาก field อื่น (video_id ที่ worker ส่งมา)
			// ถ้าไม่มี video_id ให้ใช้ entity_code สำหรับ lookup
		default:
			// video, reel, etc: entity_id = video UUID
			if update.VideoID == "" {
				update.VideoID = update.EntityID
			}
			if update.EntityCode != "" && update.VideoCode == "" {
				update.VideoCode = update.EntityCode
			}
		}
	}

	// Call handlers
	s.handlersMu.RLock()
	handlers := s.handlers
	s.handlersMu.RUnlock()

	for _, handler := range handlers {
		// Run synchronously to maintain message order
		// (WebSocket broadcast is fast, no need for goroutine)
		func(h ProgressHandler, u ProgressUpdate) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Progress handler panicked", "error", r)
				}
			}()
			h(&u)
		}(handler, update)
	}

	logger.Info("Progress update received from NATS",
		"video_id", update.GetVideoID(),
		"entity_type", update.EntityType,
		"status", update.Status,
		"progress", update.Progress,
		"handlers_count", len(handlers),
	)
}

// Stop หยุด subscriber
func (s *Subscriber) Stop() error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false

	if s.sub != nil {
		if err := s.sub.Unsubscribe(); err != nil {
			logger.Warn("Failed to unsubscribe", "error", err)
		}
	}

	logger.Info("NATS subscriber stopped")
	return nil
}

// IsRunning ตรวจสอบว่า subscriber กำลังทำงานอยู่หรือไม่
func (s *Subscriber) IsRunning() bool {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()
	return s.running
}
