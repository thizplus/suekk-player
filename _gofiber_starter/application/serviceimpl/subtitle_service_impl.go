package serviceimpl

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
)

// getTranslationTargets คืนค่าภาษาที่สามารถแปลได้จากภาษาต้นทาง
// กฎ: ถ้าไม่ใช่ไทย → แปลเป็นไทยได้ / ถ้าเป็นไทย → แปลเป็นอังกฤษได้
func getTranslationTargets(sourceLanguage string) []string {
	if sourceLanguage == "th" {
		return []string{"en"}
	}
	// ภาษาอื่นทั้งหมด → แปลเป็นไทยได้
	return []string{"th"}
}

type SubtitleServiceImpl struct {
	videoRepo    repositories.VideoRepository
	subtitleRepo repositories.SubtitleRepository
	jobPublisher services.SubtitleJobPublisher
}

func NewSubtitleService(
	videoRepo repositories.VideoRepository,
	subtitleRepo repositories.SubtitleRepository,
	jobPublisher services.SubtitleJobPublisher,
) services.SubtitleService {
	return &SubtitleServiceImpl{
		videoRepo:    videoRepo,
		subtitleRepo: subtitleRepo,
		jobPublisher: jobPublisher,
	}
}

// === Query Operations ===

// GetSubtitlesByVideoID ดึง subtitles ทั้งหมดของ video
func (s *SubtitleServiceImpl) GetSubtitlesByVideoID(ctx context.Context, videoID uuid.UUID) ([]*models.Subtitle, error) {
	return s.subtitleRepo.GetByVideoID(ctx, videoID)
}

// GetSubtitleByID ดึง subtitle ตาม ID
func (s *SubtitleServiceImpl) GetSubtitleByID(ctx context.Context, subtitleID uuid.UUID) (*models.Subtitle, error) {
	return s.subtitleRepo.GetByID(ctx, subtitleID)
}

// GetSupportedLanguages ดึงรายการภาษาที่รองรับ
func (s *SubtitleServiceImpl) GetSupportedLanguages(ctx context.Context) *dto.SupportedLanguagesResponse {
	return dto.GetSupportedLanguages()
}

// === Manual Trigger Operations ===

// TriggerDetectLanguage ส่ง detect language job ไปยัง NATS
func (s *SubtitleServiceImpl) TriggerDetectLanguage(ctx context.Context, videoID uuid.UUID) (*dto.DetectLanguageResponse, error) {
	logger.InfoContext(ctx, "Triggering language detection", "video_id", videoID)

	// 1. ดึง video
	video, err := s.videoRepo.GetByID(ctx, videoID)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get video", "video_id", videoID, "error", err)
		return nil, err
	}
	if video == nil {
		return nil, errors.New("video not found")
	}

	// 2. ตรวจสอบว่ามี audio หรือไม่
	if video.AudioPath == "" {
		return nil, errors.New("video does not have extracted audio")
	}

	// 3. ตรวจสอบว่า video ready หรือยัง
	if video.Status != models.VideoStatusReady {
		return nil, errors.New("video is not ready yet")
	}

	// 4. ตรวจสอบว่าตรวจจับภาษาไปแล้วหรือยัง
	if video.DetectedLanguage != "" {
		return nil, fmt.Errorf("language already detected: %s", video.DetectedLanguage)
	}

	// 5. ส่ง detect job
	if s.jobPublisher != nil {
		job := &services.DetectJob{
			VideoID:   video.ID.String(),
			VideoCode: video.Code,
			AudioPath: video.AudioPath,
		}
		if err := s.jobPublisher.PublishDetectJob(ctx, job); err != nil {
			logger.ErrorContext(ctx, "Failed to publish detect job", "video_id", videoID, "error", err)
			return nil, fmt.Errorf("failed to publish detect job: %w", err)
		}
	}

	logger.InfoContext(ctx, "Language detection job triggered", "video_id", videoID)

	return &dto.DetectLanguageResponse{
		VideoID:   videoID,
		Message:   "Language detection job submitted",
		AudioPath: video.AudioPath,
	}, nil
}

// TriggerTranscribe สร้าง original subtitle record และส่ง transcribe job
// ถ้ายังไม่ได้ตรวจจับภาษา worker จะ auto-detect ให้
func (s *SubtitleServiceImpl) TriggerTranscribe(ctx context.Context, videoID uuid.UUID) (*dto.TranscribeResponse, error) {
	logger.InfoContext(ctx, "Triggering transcription", "video_id", videoID)

	// 1. ดึง video
	video, err := s.videoRepo.GetByID(ctx, videoID)
	if err != nil {
		return nil, err
	}
	if video == nil {
		return nil, errors.New("video not found")
	}

	// 2. ตรวจสอบว่ามี audio หรือไม่
	if video.AudioPath == "" {
		return nil, errors.New("video does not have extracted audio")
	}

	// 3. ตรวจสอบว่ามี original subtitle อยู่แล้วหรือไม่
	existingOriginal, _ := s.subtitleRepo.GetOriginalByVideoID(ctx, videoID)
	if existingOriginal != nil {
		if existingOriginal.Status == models.SubtitleStatusReady {
			return nil, errors.New("original subtitle already exists")
		}
		if existingOriginal.IsInProgress() {
			return nil, errors.New("transcription already in progress")
		}
		// ถ้า failed ก็ลองใหม่ได้ - ลบอันเก่าก่อน
		if err := s.subtitleRepo.Delete(ctx, existingOriginal.ID); err != nil {
			logger.WarnContext(ctx, "Failed to delete failed subtitle", "subtitle_id", existingOriginal.ID, "error", err)
		}
	}

	// 4. กำหนดภาษา - ถ้ายังไม่ได้ detect ให้ใช้ "auto" แล้ว worker จะ detect ให้
	language := video.DetectedLanguage
	if language == "" {
		language = "auto"
	}

	// 5. สร้าง subtitle record ใหม่ (status = queued รอ worker มารับ)
	subtitle := &models.Subtitle{
		VideoID:    videoID,
		Language:   language, // อาจเป็น "auto" ซึ่ง worker จะอัปเดตภายหลัง
		Type:       models.SubtitleTypeOriginal,
		Confidence: 0,
		Status:     models.SubtitleStatusQueued, // รอใน queue จนกว่า worker จะหยิบไปทำ
	}
	if err := s.subtitleRepo.Create(ctx, subtitle); err != nil {
		logger.ErrorContext(ctx, "Failed to create subtitle record", "video_id", videoID, "error", err)
		return nil, err
	}

	// 6. ส่ง transcribe job
	if s.jobPublisher != nil {
		// ถ้า language เป็น "auto" output_path จะใช้ชั่วคราว - worker จะอัปเดตให้ถูกต้อง
		outputPath := fmt.Sprintf("subtitles/%s/%s.srt", video.Code, language)

		job := &services.TranscribeJob{
			SubtitleID:    subtitle.ID.String(),
			VideoID:       video.ID.String(),
			VideoCode:     video.Code,
			AudioPath:     video.AudioPath,
			Language:      language,
			OutputPath:    outputPath,
			RefineWithLLM: true,
		}
		if err := s.jobPublisher.PublishTranscribeJob(ctx, job); err != nil {
			// Rollback: ลบ subtitle record ที่สร้างไป
			s.subtitleRepo.Delete(ctx, subtitle.ID)
			logger.ErrorContext(ctx, "Failed to publish transcribe job", "video_id", videoID, "error", err)
			return nil, fmt.Errorf("failed to publish transcribe job: %w", err)
		}
	}

	logger.InfoContext(ctx, "Transcription job triggered",
		"video_id", videoID,
		"subtitle_id", subtitle.ID,
		"language", language,
	)

	return &dto.TranscribeResponse{
		VideoID:    videoID,
		SubtitleID: subtitle.ID,
		Language:   language,
		Message:    "Transcription job submitted",
	}, nil
}

// TriggerTranslation สร้าง translated subtitle records และส่ง translate job
func (s *SubtitleServiceImpl) TriggerTranslation(ctx context.Context, videoID uuid.UUID, req *dto.TranslateRequest) (*dto.TranslateJobResponse, error) {
	logger.InfoContext(ctx, "Triggering translation",
		"video_id", videoID,
		"target_languages", req.TargetLanguages,
	)

	// 1. ดึง video
	video, err := s.videoRepo.GetByID(ctx, videoID)
	if err != nil {
		return nil, err
	}
	if video == nil {
		return nil, errors.New("video not found")
	}

	// 2. ดึง original subtitle
	original, err := s.subtitleRepo.GetOriginalByVideoID(ctx, videoID)
	if err != nil {
		return nil, errors.New("original subtitle not found")
	}
	if original.Status != models.SubtitleStatusReady {
		return nil, errors.New("original subtitle is not ready")
	}

	// 3. ตรวจสอบภาษาที่รองรับ
	validTargets, invalidTargets := s.CanTranslate(original.Language, req.TargetLanguages)
	if len(validTargets) == 0 {
		return nil, fmt.Errorf("no valid target languages for source language '%s', unsupported: %v",
			original.Language, invalidTargets)
	}

	// 4. สร้าง subtitle records สำหรับแต่ละภาษา
	var subtitleIDs []uuid.UUID
	var createdSubtitles []*models.Subtitle

	for _, lang := range validTargets {
		// ตรวจสอบว่ามีอยู่แล้วหรือไม่
		existing, _ := s.subtitleRepo.GetByVideoIDAndLanguage(ctx, videoID, lang)
		if existing != nil {
			if existing.Status == models.SubtitleStatusReady {
				logger.WarnContext(ctx, "Translation already exists, skipping", "language", lang)
				continue
			}
			if existing.IsInProgress() {
				logger.WarnContext(ctx, "Translation already in progress, skipping", "language", lang)
				continue
			}
			// ถ้า failed ก็ลบแล้วสร้างใหม่
			s.subtitleRepo.Delete(ctx, existing.ID)
		}

		// สร้าง subtitle record ใหม่ (status = queued รอ worker มารับ)
		subtitle := &models.Subtitle{
			VideoID:        videoID,
			Language:       lang,
			Type:           models.SubtitleTypeTranslated,
			SourceLanguage: original.Language,
			Status:         models.SubtitleStatusQueued, // รอใน queue จนกว่า worker จะหยิบไปทำ
		}
		if err := s.subtitleRepo.Create(ctx, subtitle); err != nil {
			logger.WarnContext(ctx, "Failed to create subtitle record", "language", lang, "error", err)
			continue
		}
		subtitleIDs = append(subtitleIDs, subtitle.ID)
		createdSubtitles = append(createdSubtitles, subtitle)
	}

	if len(subtitleIDs) == 0 {
		return nil, errors.New("no new translations to create")
	}

	// 5. ส่ง translate job
	if s.jobPublisher != nil {
		// แปลง UUIDs เป็น strings
		subtitleIDStrings := make([]string, len(subtitleIDs))
		targetLangs := make([]string, len(createdSubtitles))
		for i, sub := range createdSubtitles {
			subtitleIDStrings[i] = sub.ID.String()
			targetLangs[i] = sub.Language
		}

		job := &services.TranslateJob{
			SubtitleIDs:     subtitleIDStrings,
			VideoID:         video.ID.String(),
			VideoCode:       video.Code,
			SourceSRTPath:   original.SRTPath,
			SourceLanguage:  original.Language,
			TargetLanguages: targetLangs,
			OutputPath:      fmt.Sprintf("subtitles/%s", video.Code),
		}
		if err := s.jobPublisher.PublishTranslateJob(ctx, job); err != nil {
			// Rollback: ลบ subtitle records ที่สร้างไป
			for _, id := range subtitleIDs {
				s.subtitleRepo.Delete(ctx, id)
			}
			logger.ErrorContext(ctx, "Failed to publish translate job", "video_id", videoID, "error", err)
			return nil, fmt.Errorf("failed to publish translate job: %w", err)
		}
	}

	// สร้าง target languages list สำหรับ response
	finalTargets := make([]string, len(createdSubtitles))
	for i, sub := range createdSubtitles {
		finalTargets[i] = sub.Language
	}

	response := &dto.TranslateJobResponse{
		VideoID:         videoID,
		SubtitleIDs:     subtitleIDs,
		SourceLanguage:  original.Language,
		TargetLanguages: finalTargets,
		Message:         fmt.Sprintf("Translation job submitted for %d language(s)", len(finalTargets)),
	}

	if len(invalidTargets) > 0 {
		response.Message += fmt.Sprintf(". Skipped unsupported: %v", invalidTargets)
	}

	logger.InfoContext(ctx, "Translation job triggered",
		"video_id", videoID,
		"source_language", original.Language,
		"target_languages", finalTargets,
	)

	return response, nil
}

// === Worker Callbacks ===

// HandleDetectComplete callback จาก worker เมื่อ detect language เสร็จ
func (s *SubtitleServiceImpl) HandleDetectComplete(ctx context.Context, videoID uuid.UUID, req *dto.DetectCompleteRequest) error {
	logger.InfoContext(ctx, "Handling detect complete callback",
		"video_id", videoID,
		"language", req.Language,
		"confidence", req.Confidence,
		"worker_id", req.WorkerID,
	)

	// อัปเดต video.DetectedLanguage
	video, err := s.videoRepo.GetByID(ctx, videoID)
	if err != nil {
		return err
	}
	if video == nil {
		return errors.New("video not found")
	}

	video.DetectedLanguage = req.Language
	if err := s.videoRepo.Update(ctx, video); err != nil {
		logger.ErrorContext(ctx, "Failed to update video detected language", "video_id", videoID, "error", err)
		return err
	}

	logger.InfoContext(ctx, "Language detection completed", "video_id", videoID, "language", req.Language)
	return nil
}

// HandleTranscribeComplete callback จาก worker เมื่อ transcribe เสร็จ
func (s *SubtitleServiceImpl) HandleTranscribeComplete(ctx context.Context, subtitleID uuid.UUID, req *dto.TranscribeCompleteRequest) error {
	logger.InfoContext(ctx, "Handling transcribe complete callback",
		"subtitle_id", subtitleID,
		"srt_path", req.SRTPath,
		"language", req.Language,
		"worker_id", req.WorkerID,
	)

	subtitle, err := s.subtitleRepo.GetByID(ctx, subtitleID)
	if err != nil {
		return err
	}
	if subtitle == nil {
		return errors.New("subtitle not found")
	}

	subtitle.SRTPath = req.SRTPath
	subtitle.Status = models.SubtitleStatusReady
	subtitle.Error = ""

	// อัปเดตภาษาถ้า worker ส่งมา (กรณี auto-detect)
	if req.Language != "" && req.Language != "auto" {
		subtitle.Language = req.Language
	}

	if err := s.subtitleRepo.Update(ctx, subtitle); err != nil {
		logger.ErrorContext(ctx, "Failed to update subtitle", "subtitle_id", subtitleID, "error", err)
		return err
	}

	logger.InfoContext(ctx, "Transcription completed", "subtitle_id", subtitleID, "language", subtitle.Language)

	// === Auto-translate ===
	// หลัง transcribe เสร็จ → trigger translate อัตโนมัติ
	// ภาษาไทย → แปลเป็นอังกฤษ, ภาษาอื่น → แปลเป็นไทย
	go func() {
		autoCtx := context.Background()
		var targetLang string
		if subtitle.Language == "th" {
			targetLang = "en"
		} else {
			targetLang = "th"
		}

		logger.InfoContext(autoCtx, "Auto-triggering translation",
			"video_id", subtitle.VideoID,
			"source_language", subtitle.Language,
			"target_language", targetLang,
		)

		translateReq := &dto.TranslateRequest{
			TargetLanguages: []string{targetLang},
		}

		_, err := s.TriggerTranslation(autoCtx, subtitle.VideoID, translateReq)
		if err != nil {
			logger.WarnContext(autoCtx, "Auto-translate failed (non-critical)",
				"video_id", subtitle.VideoID,
				"target_language", targetLang,
				"error", err,
			)
		} else {
			logger.InfoContext(autoCtx, "Auto-translate triggered successfully",
				"video_id", subtitle.VideoID,
				"target_language", targetLang,
			)
		}
	}()

	return nil
}

// HandleTranslationComplete callback จาก worker เมื่อ translate เสร็จ (per language)
func (s *SubtitleServiceImpl) HandleTranslationComplete(ctx context.Context, subtitleID uuid.UUID, req *dto.TranslationCompleteRequest) error {
	logger.InfoContext(ctx, "Handling translation complete callback",
		"subtitle_id", subtitleID,
		"language", req.Language,
		"srt_path", req.SRTPath,
		"worker_id", req.WorkerID,
	)

	subtitle, err := s.subtitleRepo.GetByID(ctx, subtitleID)
	if err != nil {
		return err
	}
	if subtitle == nil {
		return errors.New("subtitle not found")
	}

	subtitle.SRTPath = req.SRTPath
	subtitle.Status = models.SubtitleStatusReady
	subtitle.Error = ""

	if err := s.subtitleRepo.Update(ctx, subtitle); err != nil {
		logger.ErrorContext(ctx, "Failed to update subtitle", "subtitle_id", subtitleID, "error", err)
		return err
	}

	logger.InfoContext(ctx, "Translation completed", "subtitle_id", subtitleID, "language", req.Language)
	return nil
}

// HandleSubtitleFailed callback จาก worker เมื่อ job ล้มเหลว
func (s *SubtitleServiceImpl) HandleSubtitleFailed(ctx context.Context, subtitleID uuid.UUID, req *dto.SubtitleFailedRequest) error {
	logger.WarnContext(ctx, "Handling subtitle failed callback",
		"subtitle_id", subtitleID,
		"error", req.Error,
		"worker_id", req.WorkerID,
	)

	if err := s.subtitleRepo.UpdateStatus(ctx, subtitleID, models.SubtitleStatusFailed, req.Error); err != nil {
		logger.ErrorContext(ctx, "Failed to update subtitle status", "subtitle_id", subtitleID, "error", err)
		return err
	}

	logger.InfoContext(ctx, "Subtitle marked as failed", "subtitle_id", subtitleID)
	return nil
}

// MarkJobStarted callback จาก worker เมื่อเริ่มทำ job
// เปลี่ยน status จาก queued → processing/translating และบันทึก processing_started_at
func (s *SubtitleServiceImpl) MarkJobStarted(ctx context.Context, subtitleID uuid.UUID, jobType string) error {
	logger.InfoContext(ctx, "Marking job as started",
		"subtitle_id", subtitleID,
		"job_type", jobType,
	)

	// กำหนด status ตาม job type
	var newStatus models.SubtitleStatus
	switch jobType {
	case "transcribe":
		newStatus = models.SubtitleStatusProcessing
	case "translate":
		newStatus = models.SubtitleStatusTranslating
	case "detect":
		newStatus = models.SubtitleStatusDetecting
	default:
		return fmt.Errorf("unknown job type: %s", jobType)
	}

	// อัปเดต status
	if err := s.subtitleRepo.UpdateStatus(ctx, subtitleID, newStatus, ""); err != nil {
		logger.ErrorContext(ctx, "Failed to update subtitle status", "subtitle_id", subtitleID, "error", err)
		return err
	}

	// บันทึก processing_started_at สำหรับ stuck detection
	now := time.Now()
	if err := s.subtitleRepo.UpdateProcessingStartedAt(ctx, subtitleID, now); err != nil {
		logger.ErrorContext(ctx, "Failed to update processing_started_at", "subtitle_id", subtitleID, "error", err)
		return err
	}

	logger.InfoContext(ctx, "Job marked as started",
		"subtitle_id", subtitleID,
		"new_status", newStatus,
		"processing_started_at", now,
	)
	return nil
}

// === Utility ===

// CanTranslate ตรวจสอบว่าสามารถแปลจากภาษาต้นทางเป็นภาษาเป้าหมายได้หรือไม่
func (s *SubtitleServiceImpl) CanTranslate(sourceLanguage string, targetLanguages []string) ([]string, []string) {
	supported := getTranslationTargets(sourceLanguage)

	supportedSet := make(map[string]bool)
	for _, lang := range supported {
		supportedSet[lang] = true
	}

	var valid, invalid []string
	for _, target := range targetLanguages {
		if supportedSet[target] {
			valid = append(valid, target)
		} else {
			invalid = append(invalid, target)
		}
	}

	return valid, invalid
}

// DeleteSubtitle ลบ subtitle (ลบไฟล์ด้วย - TODO: implement S3 delete)
func (s *SubtitleServiceImpl) DeleteSubtitle(ctx context.Context, subtitleID uuid.UUID) error {
	// TODO: ลบไฟล์ SRT จาก S3 ก่อนลบ record
	return s.subtitleRepo.Delete(ctx, subtitleID)
}

// DeleteAllSubtitlesByVideo ลบ subtitles ทั้งหมดของ video
func (s *SubtitleServiceImpl) DeleteAllSubtitlesByVideo(ctx context.Context, videoID uuid.UUID) error {
	// TODO: ลบไฟล์ SRT จาก S3 ก่อนลบ records
	return s.subtitleRepo.DeleteByVideoID(ctx, videoID)
}
