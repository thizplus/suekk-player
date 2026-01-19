package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/repositories"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/utils"
)

type SubtitleHandler struct {
	subtitleService services.SubtitleService
	videoRepo       repositories.VideoRepository
}

func NewSubtitleHandler(
	subtitleService services.SubtitleService,
	videoRepo repositories.VideoRepository,
) *SubtitleHandler {
	return &SubtitleHandler{
		subtitleService: subtitleService,
		videoRepo:       videoRepo,
	}
}

// GetSubtitles ดึงข้อมูล subtitles ของ video
// GET /api/v1/videos/:id/subtitles
func (h *SubtitleHandler) GetSubtitles(c *fiber.Ctx) error {
	ctx := c.UserContext()

	videoIDStr := c.Params("id")
	videoID, err := uuid.Parse(videoIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid video ID", "video_id", videoIDStr)
		return utils.BadRequestResponse(c, "Invalid video ID")
	}

	// ดึง video เพื่อเช็ค DetectedLanguage และ AudioPath
	video, err := h.videoRepo.GetByID(ctx, videoID)
	if err != nil {
		logger.WarnContext(ctx, "Failed to get video", "video_id", videoID, "error", err)
		return utils.NotFoundResponse(c, "Video not found")
	}
	if video == nil {
		return utils.NotFoundResponse(c, "Video not found")
	}

	// ดึง subtitles
	subtitles, err := h.subtitleService.GetSubtitlesByVideoID(ctx, videoID)
	if err != nil {
		logger.WarnContext(ctx, "Failed to get subtitles", "video_id", videoID, "error", err)
		subtitles = nil // ถ้าดึงไม่ได้ก็ให้ empty array
	}

	response := &dto.SubtitlesResponse{
		VideoID:            videoID,
		DetectedLanguage:   video.DetectedLanguage,
		HasAudio:           video.AudioPath != "",
		Subtitles:          dto.SubtitlesToResponses(subtitles),
		AvailableLanguages: dto.GetAvailableLanguages(subtitles),
	}

	return utils.SuccessResponse(c, response)
}

// GetSubtitlesByCode ดึงข้อมูล subtitles ของ video โดยใช้ code (สำหรับ embed - ไม่ต้อง auth)
// GET /api/v1/embed/videos/:code/subtitles
func (h *SubtitleHandler) GetSubtitlesByCode(c *fiber.Ctx) error {
	ctx := c.UserContext()

	code := c.Params("code")
	if code == "" {
		return utils.BadRequestResponse(c, "Video code is required")
	}

	// ดึง video โดยใช้ code
	video, err := h.videoRepo.GetByCode(ctx, code)
	if err != nil {
		logger.WarnContext(ctx, "Failed to get video by code", "code", code, "error", err)
		return utils.NotFoundResponse(c, "Video not found")
	}
	if video == nil {
		return utils.NotFoundResponse(c, "Video not found")
	}

	// ดึง subtitles
	subtitles, err := h.subtitleService.GetSubtitlesByVideoID(ctx, video.ID)
	if err != nil {
		logger.WarnContext(ctx, "Failed to get subtitles", "video_id", video.ID, "error", err)
		subtitles = nil // ถ้าดึงไม่ได้ก็ให้ empty array
	}

	response := &dto.SubtitlesResponse{
		VideoID:            video.ID,
		DetectedLanguage:   video.DetectedLanguage,
		HasAudio:           video.AudioPath != "",
		Subtitles:          dto.SubtitlesToResponses(subtitles),
		AvailableLanguages: dto.GetAvailableLanguages(subtitles),
	}

	logger.InfoContext(ctx, "Subtitles fetched by code", "code", code, "count", len(subtitles))
	return utils.SuccessResponse(c, response)
}

// GetSupportedLanguages ดึงรายการภาษาที่รองรับ
// GET /api/v1/subtitles/languages
func (h *SubtitleHandler) GetSupportedLanguages(c *fiber.Ctx) error {
	ctx := c.UserContext()

	languages := h.subtitleService.GetSupportedLanguages(ctx)
	return utils.SuccessResponse(c, languages)
}

// TriggerDetectLanguage trigger ตรวจจับภาษา (manual)
// POST /api/v1/videos/:id/subtitle/detect
func (h *SubtitleHandler) TriggerDetectLanguage(c *fiber.Ctx) error {
	ctx := c.UserContext()

	videoIDStr := c.Params("id")
	videoID, err := uuid.Parse(videoIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid video ID", "video_id", videoIDStr)
		return utils.BadRequestResponse(c, "Invalid video ID")
	}

	logger.InfoContext(ctx, "Detect language trigger request", "video_id", videoID)

	response, err := h.subtitleService.TriggerDetectLanguage(ctx, videoID)
	if err != nil {
		logger.WarnContext(ctx, "Failed to trigger detect language", "video_id", videoID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	return utils.SuccessResponse(c, response)
}

// TriggerTranscribe trigger สร้าง original subtitle (manual)
// POST /api/v1/videos/:id/subtitle/transcribe
func (h *SubtitleHandler) TriggerTranscribe(c *fiber.Ctx) error {
	ctx := c.UserContext()

	videoIDStr := c.Params("id")
	videoID, err := uuid.Parse(videoIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid video ID", "video_id", videoIDStr)
		return utils.BadRequestResponse(c, "Invalid video ID")
	}

	logger.InfoContext(ctx, "Transcribe trigger request", "video_id", videoID)

	response, err := h.subtitleService.TriggerTranscribe(ctx, videoID)
	if err != nil {
		logger.WarnContext(ctx, "Failed to trigger transcribe", "video_id", videoID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	return utils.SuccessResponse(c, response)
}

// TriggerTranslation ส่ง translation job (manual trigger จาก UI)
// POST /api/v1/videos/:id/subtitle/translate
func (h *SubtitleHandler) TriggerTranslation(c *fiber.Ctx) error {
	ctx := c.UserContext()

	videoIDStr := c.Params("id")
	videoID, err := uuid.Parse(videoIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid video ID", "video_id", videoIDStr)
		return utils.BadRequestResponse(c, "Invalid video ID")
	}

	var req dto.TranslateRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	logger.InfoContext(ctx, "Translation trigger request",
		"video_id", videoID,
		"target_languages", req.TargetLanguages,
	)

	response, err := h.subtitleService.TriggerTranslation(ctx, videoID, &req)
	if err != nil {
		logger.WarnContext(ctx, "Failed to trigger translation", "video_id", videoID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Translation triggered",
		"video_id", videoID,
		"target_languages", response.TargetLanguages,
	)

	return utils.SuccessResponse(c, response)
}

// === Worker Callbacks ===

// DetectComplete callback จาก worker เมื่อตรวจจับภาษาเสร็จ
// POST /api/v1/videos/:id/subtitle/callback/detect
func (h *SubtitleHandler) DetectComplete(c *fiber.Ctx) error {
	ctx := c.UserContext()

	videoIDStr := c.Params("id")
	videoID, err := uuid.Parse(videoIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid video ID", "video_id", videoIDStr)
		return utils.BadRequestResponse(c, "Invalid video ID")
	}

	var req dto.DetectCompleteRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	logger.InfoContext(ctx, "Detect complete callback received",
		"video_id", videoID,
		"language", req.Language,
		"confidence", req.Confidence,
		"worker_id", req.WorkerID,
	)

	if err := h.subtitleService.HandleDetectComplete(ctx, videoID, &req); err != nil {
		logger.ErrorContext(ctx, "Failed to handle detect complete", "video_id", videoID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	return utils.SuccessResponse(c, fiber.Map{
		"message": "Detect complete callback processed",
	})
}

// TranscribeComplete callback จาก worker เมื่อ transcribe เสร็จ
// POST /api/v1/subtitles/:id/callback/transcribe
func (h *SubtitleHandler) TranscribeComplete(c *fiber.Ctx) error {
	ctx := c.UserContext()

	subtitleIDStr := c.Params("id")
	subtitleID, err := uuid.Parse(subtitleIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid subtitle ID", "subtitle_id", subtitleIDStr)
		return utils.BadRequestResponse(c, "Invalid subtitle ID")
	}

	var req dto.TranscribeCompleteRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	logger.InfoContext(ctx, "Transcribe complete callback received",
		"subtitle_id", subtitleID,
		"srt_path", req.SRTPath,
		"worker_id", req.WorkerID,
	)

	if err := h.subtitleService.HandleTranscribeComplete(ctx, subtitleID, &req); err != nil {
		logger.ErrorContext(ctx, "Failed to handle transcribe complete", "subtitle_id", subtitleID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	return utils.SuccessResponse(c, fiber.Map{
		"message": "Transcribe complete callback processed",
	})
}

// TranslationComplete callback จาก worker เมื่อแปลเสร็จ
// POST /api/v1/subtitles/:id/callback/translate
func (h *SubtitleHandler) TranslationComplete(c *fiber.Ctx) error {
	ctx := c.UserContext()

	subtitleIDStr := c.Params("id")
	subtitleID, err := uuid.Parse(subtitleIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid subtitle ID", "subtitle_id", subtitleIDStr)
		return utils.BadRequestResponse(c, "Invalid subtitle ID")
	}

	var req dto.TranslationCompleteRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	logger.InfoContext(ctx, "Translation complete callback received",
		"subtitle_id", subtitleID,
		"language", req.Language,
		"worker_id", req.WorkerID,
	)

	if err := h.subtitleService.HandleTranslationComplete(ctx, subtitleID, &req); err != nil {
		logger.ErrorContext(ctx, "Failed to handle translation complete", "subtitle_id", subtitleID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	return utils.SuccessResponse(c, fiber.Map{
		"message": "Translation complete callback processed",
	})
}

// SubtitleFailed callback จาก worker เมื่อ job ล้มเหลว
// POST /api/v1/subtitles/:id/callback/failed
func (h *SubtitleHandler) SubtitleFailed(c *fiber.Ctx) error {
	ctx := c.UserContext()

	subtitleIDStr := c.Params("id")
	subtitleID, err := uuid.Parse(subtitleIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid subtitle ID", "subtitle_id", subtitleIDStr)
		return utils.BadRequestResponse(c, "Invalid subtitle ID")
	}

	var req dto.SubtitleFailedRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	logger.WarnContext(ctx, "Subtitle failed callback received",
		"subtitle_id", subtitleID,
		"error", req.Error,
		"worker_id", req.WorkerID,
	)

	if err := h.subtitleService.HandleSubtitleFailed(ctx, subtitleID, &req); err != nil {
		logger.ErrorContext(ctx, "Failed to handle subtitle failed", "subtitle_id", subtitleID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	return utils.SuccessResponse(c, fiber.Map{
		"message": "Subtitle failed callback processed",
	})
}

// JobStarted callback จาก worker เมื่อเริ่มทำ job
// เปลี่ยน status จาก queued → processing/translating และบันทึก processing_started_at
// POST /api/v1/internal/subtitles/job-started
func (h *SubtitleHandler) JobStarted(c *fiber.Ctx) error {
	ctx := c.UserContext()

	var req dto.JobStartedRequest
	if err := c.BodyParser(&req); err != nil {
		logger.WarnContext(ctx, "Invalid request body", "error", err)
		return utils.BadRequestResponse(c, "Invalid request body")
	}

	if err := utils.ValidateStruct(&req); err != nil {
		errors := utils.GetValidationErrors(err)
		logger.WarnContext(ctx, "Validation failed", "errors", errors)
		return utils.ValidationErrorResponse(c, errors)
	}

	subtitleID, err := uuid.Parse(req.SubtitleID)
	if err != nil {
		logger.WarnContext(ctx, "Invalid subtitle ID", "subtitle_id", req.SubtitleID)
		return utils.BadRequestResponse(c, "Invalid subtitle ID")
	}

	logger.InfoContext(ctx, "Job started callback received",
		"subtitle_id", subtitleID,
		"job_type", req.JobType,
		"worker_id", req.WorkerID,
	)

	if err := h.subtitleService.MarkJobStarted(ctx, subtitleID, req.JobType); err != nil {
		logger.ErrorContext(ctx, "Failed to mark job started", "subtitle_id", subtitleID, "error", err)
		return utils.InternalServerErrorResponse(c)
	}

	return utils.SuccessResponse(c, fiber.Map{
		"message": "Job started callback processed",
	})
}

// === User Actions ===

// DeleteSubtitle ลบ subtitle
// DELETE /api/v1/subtitles/:id
func (h *SubtitleHandler) DeleteSubtitle(c *fiber.Ctx) error {
	ctx := c.UserContext()

	subtitleIDStr := c.Params("id")
	subtitleID, err := uuid.Parse(subtitleIDStr)
	if err != nil {
		logger.WarnContext(ctx, "Invalid subtitle ID", "subtitle_id", subtitleIDStr)
		return utils.BadRequestResponse(c, "Invalid subtitle ID")
	}

	logger.InfoContext(ctx, "Delete subtitle request", "subtitle_id", subtitleID)

	if err := h.subtitleService.DeleteSubtitle(ctx, subtitleID); err != nil {
		logger.WarnContext(ctx, "Failed to delete subtitle", "subtitle_id", subtitleID, "error", err)
		return utils.BadRequestResponse(c, err.Error())
	}

	logger.InfoContext(ctx, "Subtitle deleted", "subtitle_id", subtitleID)

	return utils.SuccessResponse(c, fiber.Map{
		"message": "Subtitle deleted successfully",
	})
}
