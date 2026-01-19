package progress

import (
	"sync"

	"github.com/google/uuid"
	websocketManager "gofiber-template/infrastructure/websocket"
	"gofiber-template/pkg/logger"
)

// ProgressType ประเภทของ progress
type ProgressType string

const (
	ProgressTypeUpload     ProgressType = "upload"
	ProgressTypeTranscode  ProgressType = "transcode"
)

// ProgressStatus สถานะของ progress
type ProgressStatus string

const (
	ProgressStatusStarted    ProgressStatus = "started"
	ProgressStatusProcessing ProgressStatus = "processing"
	ProgressStatusCompleted  ProgressStatus = "completed"
	ProgressStatusFailed     ProgressStatus = "failed"
)

// ProgressData ข้อมูล progress ที่ส่งไปให้ client
type ProgressData struct {
	VideoID      string         `json:"videoId"`
	VideoCode    string         `json:"videoCode"`
	VideoTitle   string         `json:"videoTitle"`
	Type         ProgressType   `json:"type"`
	Status       ProgressStatus `json:"status"`
	Progress     int            `json:"progress"`     // 0-100
	CurrentStep  string         `json:"currentStep"`  // เช่น "uploading", "transcoding", "generating_thumbnail"
	Message      string         `json:"message"`
	ErrorMessage string         `json:"errorMessage,omitempty"`
}

// ProgressTracker จัดการ progress tracking
type ProgressTracker struct {
	mutex    sync.RWMutex
	progress map[string]*ProgressData // key: videoID
}

var tracker *ProgressTracker
var once sync.Once

// GetTracker returns singleton instance of ProgressTracker
func GetTracker() *ProgressTracker {
	once.Do(func() {
		tracker = &ProgressTracker{
			progress: make(map[string]*ProgressData),
		}
	})
	return tracker
}

// StartUpload เริ่มต้น tracking upload
func (t *ProgressTracker) StartUpload(userID uuid.UUID, videoID uuid.UUID, videoCode, videoTitle string) {
	data := &ProgressData{
		VideoID:     videoID.String(),
		VideoCode:   videoCode,
		VideoTitle:  videoTitle,
		Type:        ProgressTypeUpload,
		Status:      ProgressStatusStarted,
		Progress:    0,
		CurrentStep: "กำลังอัพโหลด",
		Message:     "เริ่มอัพโหลด",
	}

	t.mutex.Lock()
	t.progress[videoID.String()] = data
	t.mutex.Unlock()

	t.notifyUser(userID, data)
}

// UpdateUploadProgress อัพเดท upload progress
func (t *ProgressTracker) UpdateUploadProgress(userID uuid.UUID, videoID uuid.UUID, progress int, message string) {
	t.mutex.Lock()
	if data, ok := t.progress[videoID.String()]; ok {
		data.Progress = progress
		data.Status = ProgressStatusProcessing
		data.Message = message
	}
	t.mutex.Unlock()

	t.mutex.RLock()
	data := t.progress[videoID.String()]
	t.mutex.RUnlock()

	if data != nil {
		t.notifyUser(userID, data)
	}
}

// CompleteUpload upload เสร็จสิ้น
func (t *ProgressTracker) CompleteUpload(userID uuid.UUID, videoID uuid.UUID) {
	t.mutex.Lock()
	if data, ok := t.progress[videoID.String()]; ok {
		data.Progress = 100
		data.Status = ProgressStatusCompleted
		data.CurrentStep = "อัพโหลดเสร็จ"
		data.Message = "อัพโหลดเสร็จสิ้น"
	}
	t.mutex.Unlock()

	t.mutex.RLock()
	data := t.progress[videoID.String()]
	t.mutex.RUnlock()

	if data != nil {
		t.notifyUser(userID, data)
	}
}

// StartTranscoding เริ่มต้น tracking transcoding
func (t *ProgressTracker) StartTranscoding(userID uuid.UUID, videoID uuid.UUID, videoCode, videoTitle string) {
	data := &ProgressData{
		VideoID:     videoID.String(),
		VideoCode:   videoCode,
		VideoTitle:  videoTitle,
		Type:        ProgressTypeTranscode,
		Status:      ProgressStatusStarted,
		Progress:    0,
		CurrentStep: "กำลังแปลงไฟล์",
		Message:     "เริ่มแปลงไฟล์",
	}

	t.mutex.Lock()
	t.progress[videoID.String()] = data
	t.mutex.Unlock()

	t.notifyUser(userID, data)
}

// UpdateTranscodingProgress อัพเดท transcoding progress
func (t *ProgressTracker) UpdateTranscodingProgress(userID uuid.UUID, videoID uuid.UUID, progress int, step, message string) {
	t.mutex.Lock()
	if data, ok := t.progress[videoID.String()]; ok {
		data.Progress = progress
		data.Status = ProgressStatusProcessing
		data.CurrentStep = step
		data.Message = message
	}
	t.mutex.Unlock()

	t.mutex.RLock()
	data := t.progress[videoID.String()]
	t.mutex.RUnlock()

	if data != nil {
		t.notifyUser(userID, data)
	}
}

// CompleteTranscoding transcoding เสร็จสิ้น
func (t *ProgressTracker) CompleteTranscoding(userID uuid.UUID, videoID uuid.UUID) {
	t.mutex.Lock()
	if data, ok := t.progress[videoID.String()]; ok {
		data.Progress = 100
		data.Status = ProgressStatusCompleted
		data.CurrentStep = "เสร็จสิ้น"
		data.Message = "แปลงไฟล์เสร็จสิ้น"
	}
	t.mutex.Unlock()

	t.mutex.RLock()
	data := t.progress[videoID.String()]
	t.mutex.RUnlock()

	if data != nil {
		t.notifyUser(userID, data)
	}

	// Clean up after completion
	t.cleanupProgress(videoID.String())
}

// FailProgress mark progress as failed
func (t *ProgressTracker) FailProgress(userID uuid.UUID, videoID uuid.UUID, errorMessage string) {
	t.mutex.Lock()
	if data, ok := t.progress[videoID.String()]; ok {
		data.Status = ProgressStatusFailed
		data.ErrorMessage = errorMessage
		data.Message = "ล้มเหลว"
	}
	t.mutex.Unlock()

	t.mutex.RLock()
	data := t.progress[videoID.String()]
	t.mutex.RUnlock()

	if data != nil {
		t.notifyUser(userID, data)
	}

	// Clean up after failure
	t.cleanupProgress(videoID.String())
}

// GetProgress ดึง progress ปัจจุบัน
func (t *ProgressTracker) GetProgress(videoID string) *ProgressData {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	if data, ok := t.progress[videoID]; ok {
		return data
	}
	return nil
}

// notifyUser ส่ง notification ไปให้ user ผ่าน WebSocket
// Broadcast ไปทั้ง user เฉพาะและ room "analytics" เพื่อให้ admin dashboard รับได้
func (t *ProgressTracker) notifyUser(userID uuid.UUID, data *ProgressData) {
	if websocketManager.Manager != nil {
		logger.Info("Broadcasting video progress",
			"video_id", data.VideoID,
			"video_code", data.VideoCode,
			"type", data.Type,
			"status", data.Status,
			"progress", data.Progress,
			"step", data.CurrentStep,
			"room", "analytics",
		)

		// Broadcast ไป analytics room (admin dashboard ทุกคนที่เปิดอยู่จะได้รับ)
		websocketManager.Manager.BroadcastToRoom("analytics", "video_progress", data)

		// ส่งให้ user เฉพาะด้วย (ถ้า user มี WebSocket connection)
		websocketManager.Manager.BroadcastToUser(userID, "video_progress", data)
	} else {
		logger.Warn("WebSocket Manager is nil, cannot broadcast progress",
			"video_id", data.VideoID,
		)
	}
}

// cleanupProgress ลบ progress data หลังจากเสร็จหรือ fail
func (t *ProgressTracker) cleanupProgress(videoID string) {
	// Delay cleanup to ensure last message is sent
	go func() {
		// Wait a bit before cleaning up
		t.mutex.Lock()
		delete(t.progress, videoID)
		t.mutex.Unlock()
	}()
}
