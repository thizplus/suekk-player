package websocket

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"gofiber-template/domain/ports"
	"gofiber-template/domain/repositories"
	"gofiber-template/pkg/logger"
)

// ProgressBroadcaster รับ progress จาก messaging และ broadcast ไปยัง WebSocket clients
// ใช้ ports.ProgressSubscriberPort เพื่อ decouple จาก NATS implementation
type ProgressBroadcaster struct {
	progressSub ports.ProgressSubscriberPort
	manager     *WebSocketManager
	videoRepo   repositories.VideoRepository
	notifier    ports.NotifierPort // สำหรับส่ง notification เมื่อ completed/failed
	titleCache  map[string]string  // cache video title เพื่อไม่ต้อง query ทุกครั้ง
	cacheMu     sync.RWMutex
	running     bool
	runningMu   sync.Mutex
	cancelCtx   context.CancelFunc
}

// NewProgressBroadcaster สร้าง ProgressBroadcaster ใหม่
func NewProgressBroadcaster(progressSub ports.ProgressSubscriberPort, videoRepo repositories.VideoRepository) *ProgressBroadcaster {
	return &ProgressBroadcaster{
		progressSub: progressSub,
		manager:     Manager, // ใช้ global Manager
		videoRepo:   videoRepo,
		titleCache:  make(map[string]string),
	}
}

// SetNotifier ตั้งค่า notifier สำหรับส่ง notification เมื่อ transcode completed/failed
func (pb *ProgressBroadcaster) SetNotifier(notifier ports.NotifierPort) {
	pb.notifier = notifier
}

// Start เริ่ม broadcaster
func (pb *ProgressBroadcaster) Start() error {
	pb.runningMu.Lock()
	if pb.running {
		pb.runningMu.Unlock()
		return nil
	}
	pb.running = true
	pb.runningMu.Unlock()

	// สร้าง context สำหรับ cancel
	ctx, cancel := context.WithCancel(context.Background())
	pb.cancelCtx = cancel

	// Subscribe ผ่าน interface พร้อม handler
	if err := pb.progressSub.Subscribe(ctx, pb.handleProgressUpdate); err != nil {
		pb.running = false
		return err
	}

	logger.Info("Progress broadcaster started")
	return nil
}

// handleProgressUpdate จัดการ progress update จาก messaging (ผ่าน interface)
func (pb *ProgressBroadcaster) handleProgressUpdate(update *ports.ProgressData) {
	// Validate input
	if update == nil || update.VideoID == "" {
		logger.Warn("Invalid progress data received")
		return
	}

	// ตรวจสอบว่าเป็น subtitle progress หรือ transcode progress หรือ gallery progress
	isSubtitleProgress := update.SubtitleID != "" || update.Stage != ""
	isGalleryProgress := update.Quality == "gallery"

	if isSubtitleProgress {
		pb.handleSubtitleProgress(update)
		return
	}

	if isGalleryProgress {
		pb.handleGalleryProgress(update)
		return
	}

	// === Transcode Progress ===
	// Map status จาก worker เป็น frontend status
	// Worker: "processing", "completed", "failed"
	// Frontend: "started", "processing", "completed", "failed"
	status := update.Status
	if status == "processing" && update.Progress == 0 {
		status = "started"
	}

	// กำหนด currentStep จาก Message ที่ worker ส่งมา (ละเอียดกว่า)
	// หรือ fallback เป็น stage ตาม progress range
	currentStep := update.Message
	if currentStep == "" {
		// Fallback ถ้าไม่มี message
		if update.Progress < 2 {
			currentStep = "เริ่มต้น"
		} else if update.Progress < 10 {
			currentStep = "ดาวน์โหลด"
		} else if update.Progress < 12 {
			currentStep = "วิเคราะห์"
		} else if update.Progress < 87 {
			currentStep = "แปลงไฟล์"
		} else if update.Progress < 92 {
			currentStep = "อัพโหลด"
		} else {
			currentStep = "สรุปผล"
		}
	}
	if status == "completed" {
		currentStep = "เสร็จสิ้น"
	} else if status == "failed" {
		currentStep = "ล้มเหลว"
	}

	// ดึง video title จาก cache หรือ database
	videoTitle := pb.getVideoTitle(update.VideoID, update.VideoCode)

	// สร้าง WebSocket message ตรงกับ frontend VideoProgress interface
	wsMessage := ProgressMessage{
		VideoID:      update.VideoID,
		VideoCode:    update.VideoCode,
		VideoTitle:   videoTitle,
		Type:         "transcode",
		Status:       status,
		Progress:     update.Progress,
		CurrentStep:  currentStep,
		Message:      update.Message,
		ErrorMessage: update.Error,
		Quality:      update.Quality,
		OutputPath:   update.OutputPath,
	}

	// Broadcast ไปยังทุก client
	// ใช้ "video_progress" เพื่อให้ตรงกับ frontend WebSocket handler
	pb.manager.BroadcastToAll("video_progress", wsMessage)

	logger.Info("Progress broadcasted to WebSocket",
		"video_id", update.VideoID,
		"status", update.Status,
		"progress", update.Progress,
		"worker_id", update.WorkerID,
		"clients_count", pb.manager.GetTotalClients(),
	)

	// อัพเดท Database เมื่อ status เปลี่ยน (processing, completed, failed)
	if update.Status == "processing" || update.Status == "completed" || update.Status == "failed" {
		pb.updateVideoStatus(update)
	}

	// ถ้า completed หรือ failed ให้ส่ง notification พิเศษ
	if update.Status == "completed" {
		pb.manager.BroadcastToAll("transcode:completed", wsMessage)
		logger.Info("Transcode completed, notification sent", "video_id", update.VideoID)

		// ส่ง Telegram notification (ถ้าเปิดใช้งาน)
		if pb.notifier != nil {
			go func() {
				ctx := context.Background()
				if err := pb.notifier.SendTranscodeCompleteAlert(ctx, update.VideoCode, videoTitle); err != nil {
					logger.Warn("Failed to send transcode complete notification", "video_id", update.VideoID, "error", err)
				}
			}()
		}
	} else if update.Status == "failed" {
		pb.manager.BroadcastToAll("transcode:failed", wsMessage)
		logger.Warn("Transcode failed, notification sent", "video_id", update.VideoID, "error", update.Error)

		// ส่ง Telegram notification (ถ้าเปิดใช้งาน)
		if pb.notifier != nil {
			go func() {
				ctx := context.Background()
				if err := pb.notifier.SendTranscodeFailAlert(ctx, update.VideoCode, videoTitle, update.Error); err != nil {
					logger.Warn("Failed to send transcode fail notification", "video_id", update.VideoID, "error", err)
				}
			}()
		}
	}
}

// updateVideoStatus อัพเดท video status ใน Database
func (pb *ProgressBroadcaster) updateVideoStatus(update *ports.ProgressData) {
	if pb.videoRepo == nil {
		logger.Warn("VideoRepository not available, cannot update status")
		return
	}

	videoUUID, err := uuid.Parse(update.VideoID)
	if err != nil {
		logger.Warn("Invalid video ID", "video_id", update.VideoID, "error", err)
		return
	}

	ctx := context.Background()

	// ดึง video ปัจจุบัน
	video, err := pb.videoRepo.GetByID(ctx, videoUUID)
	if err != nil {
		logger.Warn("Failed to get video for status update", "video_id", update.VideoID, "error", err)
		return
	}

	// อัพเดท status ตาม progress status
	if update.Status == "processing" {
		// อัพเดทเป็น processing เฉพาะเมื่อยังเป็น pending หรือ queued
		if video.Status == "pending" || video.Status == "queued" {
			oldStatus := video.Status
			video.Status = "processing"
			logger.Info("Updating video status to processing",
				"video_id", update.VideoID,
				"old_status", oldStatus,
				"new_status", "processing",
				"worker_id", update.WorkerID,
			)
		} else if video.Status == "processing" {
			// ถ้าเป็น processing อยู่แล้ว → reset stuck detection timer
			// เพื่อป้องกันไม่ให้ถูก mark เป็น failed ระหว่าง download ไฟล์ใหญ่
			if err := pb.videoRepo.UpdateProcessingTimestamp(ctx, videoUUID); err != nil {
				logger.Warn("Failed to update processing timestamp", "video_id", update.VideoID, "error", err)
			}
			return
		} else {
			// status อื่นๆ (ready, failed) → ไม่ต้องทำอะไร
			return
		}
	} else if update.Status == "completed" {
		video.Status = "ready"
		video.HLSPath = update.OutputPath
		if update.AudioPath != "" {
			video.AudioPath = update.AudioPath
		}
		logger.Info("Updating video status to ready",
			"video_id", update.VideoID,
			"audio_path", update.AudioPath,
			"worker_id", update.WorkerID,
		)
	} else if update.Status == "failed" {
		video.Status = "failed"
		logger.Info("Updating video status to failed",
			"video_id", update.VideoID,
			"worker_id", update.WorkerID,
		)
	}

	// บันทึกลง database
	if err := pb.videoRepo.Update(ctx, video); err != nil {
		logger.Error("Failed to update video status", "video_id", update.VideoID, "error", err)
		return
	}

	logger.Info("Video status updated in database",
		"video_id", update.VideoID,
		"status", video.Status,
	)

	// Clear cache
	pb.cacheMu.Lock()
	delete(pb.titleCache, update.VideoID)
	pb.cacheMu.Unlock()
}

// Stop หยุด broadcaster
func (pb *ProgressBroadcaster) Stop() {
	pb.runningMu.Lock()
	defer pb.runningMu.Unlock()

	if !pb.running {
		return
	}

	pb.running = false

	// Cancel context first
	if pb.cancelCtx != nil {
		pb.cancelCtx()
	}

	// Unsubscribe จาก messaging
	if pb.progressSub != nil {
		if err := pb.progressSub.Unsubscribe(); err != nil {
			logger.Warn("Failed to unsubscribe progress", "error", err)
		}
	}

	logger.Info("Progress broadcaster stopped")
}

// IsRunning ตรวจสอบว่า broadcaster กำลังทำงานอยู่หรือไม่
func (pb *ProgressBroadcaster) IsRunning() bool {
	pb.runningMu.Lock()
	defer pb.runningMu.Unlock()
	return pb.running
}

// handleSubtitleProgress จัดการ subtitle progress update
func (pb *ProgressBroadcaster) handleSubtitleProgress(update *ports.ProgressData) {
	// Map stage to Thai message
	currentStep := update.Message
	if currentStep == "" {
		switch update.Stage {
		case "downloading":
			currentStep = "กำลังดาวน์โหลดเสียง"
		case "detecting":
			currentStep = "กำลังตรวจจับภาษา"
		case "transcribing":
			currentStep = "กำลังถอดเสียง"
		case "vad":
			currentStep = "กำลังวิเคราะห์เสียง"
		case "fixing":
			currentStep = "กำลังแก้ไขช่องว่าง"
		case "refining":
			currentStep = "กำลังปรับปรุงด้วย AI"
		case "generating":
			currentStep = "กำลังสร้างไฟล์ SRT"
		case "uploading":
			currentStep = "กำลังอัพโหลด"
		case "translating":
			currentStep = "กำลังแปลภาษา"
		case "completed":
			currentStep = "เสร็จสิ้น"
		case "failed":
			currentStep = "ล้มเหลว"
		default:
			currentStep = update.Stage
		}
	}

	// Map status
	status := update.Stage
	if status == "" {
		status = "processing"
	}

	// ดึง video title จาก cache หรือ database
	videoTitle := pb.getVideoTitle(update.VideoID, update.VideoCode)

	// สร้าง WebSocket message สำหรับ subtitle
	wsMessage := ProgressMessage{
		VideoID:      update.VideoID,
		VideoCode:    update.VideoCode,
		VideoTitle:   videoTitle,
		Type:         "subtitle",
		Status:       status,
		Progress:     update.Progress,
		CurrentStep:  currentStep,
		Message:      update.Message,
		ErrorMessage: update.Error,
		SubtitleID:   update.SubtitleID,
		Language:     update.CurrentLanguage,
	}

	// Broadcast ไปยังทุก client
	pb.manager.BroadcastToAll("subtitle_progress", wsMessage)

	logger.Info("Subtitle progress broadcasted to WebSocket",
		"video_id", update.VideoID,
		"stage", update.Stage,
		"progress", update.Progress,
		"subtitle_id", update.SubtitleID,
		"language", update.CurrentLanguage,
		"clients_count", pb.manager.GetTotalClients(),
	)
}

// handleGalleryProgress จัดการ gallery progress update
func (pb *ProgressBroadcaster) handleGalleryProgress(update *ports.ProgressData) {
	// Map status
	status := update.Status
	if status == "processing" && update.Progress == 0 {
		status = "started"
	}

	// ดึง video title จาก cache หรือ database
	videoTitle := pb.getVideoTitle(update.VideoID, update.VideoCode)

	// สร้าง WebSocket message สำหรับ gallery
	wsMessage := ProgressMessage{
		VideoID:      update.VideoID,
		VideoCode:    update.VideoCode,
		VideoTitle:   videoTitle,
		Type:         "gallery",
		Status:       status,
		Progress:     update.Progress,
		CurrentStep:  update.Message,
		Message:      update.Message,
		ErrorMessage: update.Error,
	}

	// Broadcast ไปยังทุก client
	pb.manager.BroadcastToAll("video_progress", wsMessage)

	logger.Info("Gallery progress broadcasted to WebSocket",
		"video_id", update.VideoID,
		"status", update.Status,
		"progress", update.Progress,
		"worker_id", update.WorkerID,
		"clients_count", pb.manager.GetTotalClients(),
	)
}

// getVideoTitle ดึง video title จาก cache หรือ database
func (pb *ProgressBroadcaster) getVideoTitle(videoID, videoCode string) string {
	// ลอง cache ก่อน
	pb.cacheMu.RLock()
	if title, ok := pb.titleCache[videoID]; ok {
		pb.cacheMu.RUnlock()
		return title
	}
	pb.cacheMu.RUnlock()

	// ถ้าไม่มี videoRepo ให้ใช้ videoCode
	if pb.videoRepo == nil {
		return videoCode
	}

	// Query จาก database
	videoUUID, err := uuid.Parse(videoID)
	if err != nil {
		return videoCode
	}

	video, err := pb.videoRepo.GetByID(context.Background(), videoUUID)
	if err != nil || video == nil {
		return videoCode
	}

	// Cache title
	pb.cacheMu.Lock()
	pb.titleCache[videoID] = video.Title
	pb.cacheMu.Unlock()

	return video.Title
}

// ProgressMessage โครงสร้าง message ที่ส่งไป WebSocket (ตรงกับ frontend VideoProgress interface)
type ProgressMessage struct {
	VideoID      string  `json:"videoId"`
	VideoCode    string  `json:"videoCode"`
	VideoTitle   string  `json:"videoTitle"`
	Type         string  `json:"type"`         // "upload", "transcode", or "subtitle"
	Status       string  `json:"status"`       // "started", "processing", "completed", "failed"
	Progress     float64 `json:"progress"`     // 0-100
	CurrentStep  string  `json:"currentStep"`  // เช่น "uploading", "transcoding", "generating_thumbnail"
	Message      string  `json:"message"`
	ErrorMessage string  `json:"errorMessage,omitempty"`
	Quality      string  `json:"quality,omitempty"`
	OutputPath   string  `json:"outputPath,omitempty"`

	// Subtitle-specific fields
	SubtitleID string `json:"subtitleId,omitempty"`
	Language   string `json:"language,omitempty"`
}
