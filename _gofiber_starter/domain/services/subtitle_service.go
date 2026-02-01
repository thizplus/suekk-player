package services

import (
	"context"

	"github.com/google/uuid"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
)

// SubtitleService interface สำหรับจัดการ subtitle
type SubtitleService interface {
	// === Query Operations ===

	// GetSubtitlesByVideoID ดึง subtitles ทั้งหมดของ video
	GetSubtitlesByVideoID(ctx context.Context, videoID uuid.UUID) ([]*models.Subtitle, error)

	// GetSubtitleByID ดึง subtitle ตาม ID
	GetSubtitleByID(ctx context.Context, subtitleID uuid.UUID) (*models.Subtitle, error)

	// GetSupportedLanguages ดึงรายการภาษาที่รองรับ
	GetSupportedLanguages(ctx context.Context) *dto.SupportedLanguagesResponse

	// === Manual Trigger Operations (User Actions) ===

	// TriggerDetectLanguage ส่ง detect language job ไปยัง NATS
	// ใช้ audio จาก video.AudioPath
	// อัปเดต video.DetectedLanguage เมื่อเสร็จ
	TriggerDetectLanguage(ctx context.Context, videoID uuid.UUID) (*dto.DetectLanguageResponse, error)

	// TriggerTranscribe สร้าง original subtitle record และส่ง transcribe job
	// ต้องมี video.DetectedLanguage ก่อน
	TriggerTranscribe(ctx context.Context, videoID uuid.UUID) (*dto.TranscribeResponse, error)

	// TriggerTranslation สร้าง translated subtitle records และส่ง translate job
	// ต้องมี original subtitle ที่ ready ก่อน
	TriggerTranslation(ctx context.Context, videoID uuid.UUID, req *dto.TranslateRequest) (*dto.TranslateJobResponse, error)

	// === Worker Callbacks ===

	// HandleDetectComplete callback จาก worker เมื่อ detect language เสร็จ
	HandleDetectComplete(ctx context.Context, videoID uuid.UUID, req *dto.DetectCompleteRequest) error

	// HandleTranscribeComplete callback จาก worker เมื่อ transcribe เสร็จ
	HandleTranscribeComplete(ctx context.Context, subtitleID uuid.UUID, req *dto.TranscribeCompleteRequest) error

	// HandleTranslationComplete callback จาก worker เมื่อ translate เสร็จ (per language)
	HandleTranslationComplete(ctx context.Context, subtitleID uuid.UUID, req *dto.TranslationCompleteRequest) error

	// HandleSubtitleFailed callback จาก worker เมื่อ job ล้มเหลว
	HandleSubtitleFailed(ctx context.Context, subtitleID uuid.UUID, req *dto.SubtitleFailedRequest) error

	// MarkJobStarted callback จาก worker เมื่อเริ่มทำ job
	// เปลี่ยน status จาก queued → processing/translating และบันทึก processing_started_at
	MarkJobStarted(ctx context.Context, subtitleID uuid.UUID, jobType string) error

	// === Content Edit Operations ===

	// GetSubtitleContent ดึง content ของ subtitle (SRT file)
	GetSubtitleContent(ctx context.Context, subtitleID uuid.UUID) (*dto.SubtitleContentResponse, error)

	// UpdateSubtitleContent อัปเดต content ของ subtitle (SRT file)
	UpdateSubtitleContent(ctx context.Context, subtitleID uuid.UUID, content string) error

	// === Utility ===

	// CanTranslate ตรวจสอบว่าสามารถแปลจากภาษาต้นทางเป็นภาษาเป้าหมายได้หรือไม่
	CanTranslate(sourceLanguage string, targetLanguages []string) ([]string, []string)

	// DeleteSubtitle ลบ subtitle (ลบไฟล์ด้วย)
	DeleteSubtitle(ctx context.Context, subtitleID uuid.UUID) error

	// DeleteAllSubtitlesByVideo ลบ subtitles ทั้งหมดของ video
	DeleteAllSubtitlesByVideo(ctx context.Context, videoID uuid.UUID) error

	// RetryStuckSubtitles retry subtitles ที่ค้างอยู่ใน queue (status = queued)
	RetryStuckSubtitles(ctx context.Context) (*dto.RetryStuckResponse, error)
}

// SubtitleJobPublisher interface สำหรับส่ง subtitle jobs ไปยัง NATS
type SubtitleJobPublisher interface {
	// PublishDetectJob ส่ง detect language job
	PublishDetectJob(ctx context.Context, job *DetectJob) error

	// PublishTranscribeJob ส่ง transcribe job
	PublishTranscribeJob(ctx context.Context, job *TranscribeJob) error

	// PublishTranslateJob ส่ง translate job
	PublishTranslateJob(ctx context.Context, job *TranslateJob) error
}

// DetectJob job สำหรับ detect language
type DetectJob struct {
	VideoID   string `json:"video_id"`
	VideoCode string `json:"video_code"`
	AudioPath string `json:"audio_path"` // S3 path to audio file
}

// TranscribeJob job สำหรับ transcribe (สร้าง original SRT)
type TranscribeJob struct {
	SubtitleID    string `json:"subtitle_id"`
	VideoID       string `json:"video_id"`
	VideoCode     string `json:"video_code"`
	AudioPath     string `json:"audio_path"`     // S3 path to audio file
	Language      string `json:"language"`       // Detected language
	OutputPath    string `json:"output_path"`    // S3 path for SRT output
	RefineWithLLM bool   `json:"refine_with_llm"`
}

// TranslateJob job สำหรับ translate
type TranslateJob struct {
	SubtitleIDs        []string `json:"subtitle_ids"`         // IDs of subtitle records to update
	VideoID            string   `json:"video_id"`
	VideoCode          string   `json:"video_code"`
	SourceSRTPath      string   `json:"source_srt_path"`      // S3 path to original SRT
	SourceLanguage     string   `json:"source_language"`
	TargetLanguages    []string `json:"target_languages"`
	OutputPath         string   `json:"output_path"`          // S3 directory for translated SRTs
}
