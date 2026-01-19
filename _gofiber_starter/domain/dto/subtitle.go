package dto

import (
	"time"

	"github.com/google/uuid"
	"gofiber-template/domain/models"
)

// === Requests ===

// TranslateRequest request สำหรับ trigger translation (manual)
type TranslateRequest struct {
	TargetLanguages []string `json:"targetLanguages" validate:"required,min=1,dive,required"`
}

// DetectCompleteRequest callback จาก worker เมื่อ detect language เสร็จ
type DetectCompleteRequest struct {
	Language   string  `json:"language" validate:"required"`
	Confidence float64 `json:"confidence" validate:"required,min=0,max=1"`
	WorkerID   string  `json:"worker_id"`
}

// TranscribeCompleteRequest callback จาก worker เมื่อ transcribe เสร็จ
type TranscribeCompleteRequest struct {
	SRTPath  string `json:"srt_path" validate:"required"`
	Language string `json:"language"` // ภาษาที่ตรวจพบ (กรณี auto-detect)
	WorkerID string `json:"worker_id"`
}

// TranslationCompleteRequest callback จาก subtitle worker เมื่อแปลเสร็จ
type TranslationCompleteRequest struct {
	Language string `json:"language" validate:"required"`
	SRTPath  string `json:"srt_path" validate:"required"`
	WorkerID string `json:"worker_id"`
}

// SubtitleFailedRequest callback จาก subtitle worker เมื่อ job ล้มเหลว
type SubtitleFailedRequest struct {
	Error    string `json:"error" validate:"required"`
	WorkerID string `json:"worker_id"`
}

// JobStartedRequest callback จาก worker เมื่อเริ่มทำ job
// ใช้สำหรับเปลี่ยน status จาก queued → processing/translating
type JobStartedRequest struct {
	SubtitleID string `json:"subtitle_id" validate:"required,uuid"`
	JobType    string `json:"job_type" validate:"required,oneof=detect transcribe translate"`
	WorkerID   string `json:"worker_id"`
}

// === Responses ===

// SubtitleResponse ข้อมูล subtitle แต่ละ record
type SubtitleResponse struct {
	ID             uuid.UUID            `json:"id"`
	VideoID        uuid.UUID            `json:"videoId"`
	Language       string               `json:"language"`
	Type           models.SubtitleType  `json:"type"`
	SourceLanguage string               `json:"sourceLanguage,omitempty"`
	Confidence     float64              `json:"confidence,omitempty"`
	SRTPath        string               `json:"srtPath,omitempty"`
	Status         models.SubtitleStatus `json:"status"`
	Error          string               `json:"error,omitempty"`
	CreatedAt      time.Time            `json:"createdAt"`
	UpdatedAt      time.Time            `json:"updatedAt"`
}

// SubtitlesResponse รายการ subtitles ของ video
type SubtitlesResponse struct {
	VideoID            uuid.UUID          `json:"videoId"`
	DetectedLanguage   string             `json:"detectedLanguage,omitempty"`
	HasAudio           bool               `json:"hasAudio"`
	Subtitles          []SubtitleResponse `json:"subtitles"`
	AvailableLanguages []string           `json:"availableLanguages"`
}

// DetectLanguageResponse response หลัง trigger detect
type DetectLanguageResponse struct {
	VideoID  uuid.UUID `json:"videoId"`
	Message  string    `json:"message"`
	AudioPath string   `json:"audioPath,omitempty"`
}

// TranscribeResponse response หลัง trigger transcribe
type TranscribeResponse struct {
	VideoID    uuid.UUID `json:"videoId"`
	SubtitleID uuid.UUID `json:"subtitleId"`
	Language   string    `json:"language"`
	Message    string    `json:"message"`
}

// TranslateJobResponse response หลังจาก trigger translation
type TranslateJobResponse struct {
	VideoID         uuid.UUID   `json:"videoId"`
	SubtitleIDs     []uuid.UUID `json:"subtitleIds"`
	SourceLanguage  string      `json:"sourceLanguage"`
	TargetLanguages []string    `json:"targetLanguages"`
	Message         string      `json:"message"`
}

// SupportedLanguagesResponse รายการภาษาที่รองรับ
type SupportedLanguagesResponse struct {
	SourceLanguages  []LanguageInfo      `json:"sourceLanguages"`
	TranslationPairs map[string][]string `json:"translationPairs"`
}

// LanguageInfo ข้อมูลภาษา
type LanguageInfo struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// === Mappers ===

// SubtitleToResponse แปลง Subtitle model เป็น SubtitleResponse
func SubtitleToResponse(subtitle *models.Subtitle) *SubtitleResponse {
	if subtitle == nil {
		return nil
	}
	return &SubtitleResponse{
		ID:             subtitle.ID,
		VideoID:        subtitle.VideoID,
		Language:       subtitle.Language,
		Type:           subtitle.Type,
		SourceLanguage: subtitle.SourceLanguage,
		Confidence:     subtitle.Confidence,
		SRTPath:        subtitle.SRTPath,
		Status:         subtitle.Status,
		Error:          subtitle.Error,
		CreatedAt:      subtitle.CreatedAt,
		UpdatedAt:      subtitle.UpdatedAt,
	}
}

// SubtitlesToResponses แปลง slice of Subtitle models เป็น slice of SubtitleResponse
func SubtitlesToResponses(subtitles []*models.Subtitle) []SubtitleResponse {
	responses := make([]SubtitleResponse, len(subtitles))
	for i, subtitle := range subtitles {
		responses[i] = *SubtitleToResponse(subtitle)
	}
	return responses
}

// GetAvailableLanguages ดึง languages ที่ ready จาก subtitles
func GetAvailableLanguages(subtitles []*models.Subtitle) []string {
	var languages []string
	for _, sub := range subtitles {
		if sub.Status == models.SubtitleStatusReady {
			languages = append(languages, sub.Language)
		}
	}
	return languages
}

// GetSupportedLanguages ดึงรายการภาษาที่รองรับ
// กฎ: ภาษาใดก็ได้ (ยกเว้นไทย) → แปลเป็นไทย / ไทย → แปลเป็นอังกฤษ
func GetSupportedLanguages() *SupportedLanguagesResponse {
	sourceLanguages := []LanguageInfo{
		{Code: "ja", Name: "Japanese"},
		{Code: "en", Name: "English"},
		{Code: "zh", Name: "Chinese"},
		{Code: "ko", Name: "Korean"},
		{Code: "th", Name: "Thai"},
		{Code: "ru", Name: "Russian"},
	}

	// สร้าง translation pairs แบบ dynamic
	// ภาษาอื่น → ไทย, ไทย → อังกฤษ
	translationPairs := make(map[string][]string)
	for _, lang := range sourceLanguages {
		if lang.Code == "th" {
			translationPairs[lang.Code] = []string{"en"}
		} else {
			translationPairs[lang.Code] = []string{"th"}
		}
	}

	return &SupportedLanguagesResponse{
		SourceLanguages:  sourceLanguages,
		TranslationPairs: translationPairs,
	}
}
