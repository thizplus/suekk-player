package serviceimpl

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
	"gofiber-template/domain/ports"
	"gofiber-template/domain/repositories"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
)

type ReelServiceImpl struct {
	reelRepo     repositories.ReelRepository
	templateRepo repositories.ReelTemplateRepository
	videoRepo    repositories.VideoRepository
	jobPublisher services.ReelJobPublisher
	storage      ports.StoragePort
}

func NewReelService(
	reelRepo repositories.ReelRepository,
	templateRepo repositories.ReelTemplateRepository,
	videoRepo repositories.VideoRepository,
	jobPublisher services.ReelJobPublisher,
	storage ports.StoragePort,
) services.ReelService {
	return &ReelServiceImpl{
		reelRepo:     reelRepo,
		templateRepo: templateRepo,
		videoRepo:    videoRepo,
		jobPublisher: jobPublisher,
		storage:      storage,
	}
}

// Create สร้าง reel ใหม่
func (s *ReelServiceImpl) Create(ctx context.Context, userID uuid.UUID, req *dto.CreateReelRequest) (*models.Reel, error) {
	logger.InfoContext(ctx, "Creating new reel",
		"user_id", userID,
		"video_id", req.VideoID,
	)

	// 1. ตรวจสอบว่า video มีอยู่และ ready
	video, err := s.videoRepo.GetByID(ctx, req.VideoID)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get video", "video_id", req.VideoID, "error", err)
		return nil, errors.New("video not found")
	}
	if video == nil {
		return nil, errors.New("video not found")
	}
	if video.Status != models.VideoStatusReady {
		return nil, errors.New("video is not ready")
	}

	// 2. ตรวจสอบ segment time
	// ถ้ามี segments ใช้ segments, ถ้าไม่มีใช้ segmentStart/End
	var segments models.VideoSegments
	if len(req.Segments) > 0 {
		segments = dto.SegmentRequestsToModels(req.Segments)
		// Validate each segment
		for i, seg := range segments {
			if seg.End > float64(video.Duration) {
				return nil, fmt.Errorf("segment %d end (%v) exceeds video duration (%v)", i+1, seg.End, video.Duration)
			}
			if seg.End <= seg.Start {
				return nil, fmt.Errorf("segment %d: end time must be greater than start time", i+1)
			}
		}
		// Validate total duration (max 60 seconds)
		totalDuration := segments.TotalDuration()
		if totalDuration > 60 {
			return nil, fmt.Errorf("total duration (%v seconds) exceeds maximum (60 seconds)", totalDuration)
		}
	} else if req.SegmentEnd > 0 {
		// LEGACY: Single segment
		if req.SegmentEnd > float64(video.Duration) {
			return nil, fmt.Errorf("segment end (%v) exceeds video duration (%v)", req.SegmentEnd, video.Duration)
		}
		segments = models.VideoSegments{{Start: req.SegmentStart, End: req.SegmentEnd}}
	} else {
		return nil, errors.New("segments or segmentEnd is required")
	}

	// 3. ตรวจสอบ template (ถ้ามี)
	var template *models.ReelTemplate
	if req.TemplateID != nil {
		template, err = s.templateRepo.GetByID(ctx, *req.TemplateID)
		if err != nil || template == nil {
			return nil, errors.New("template not found")
		}
	}

	// 4. สร้าง layers
	layers := dto.LayerRequestsToModels(req.Layers)

	// 5. ใช้ default layers จาก template ถ้าไม่มี layers ที่ส่งมา
	if len(layers) == 0 && template != nil && len(template.DefaultLayers) > 0 {
		layers = template.DefaultLayers
	}

	// 6. สร้าง reel record
	coverTime := -1.0 // default: auto middle
	if req.CoverTime != nil {
		coverTime = *req.CoverTime
	}

	reel := &models.Reel{
		UserID:       userID,
		VideoID:      req.VideoID,
		Title:        req.Title,
		Segments:     segments,
		SegmentStart: segments[0].Start,                  // LEGACY: first segment start
		SegmentEnd:   segments[len(segments)-1].End,      // LEGACY: last segment end
		CoverTime:    coverTime,
		Status:       models.ReelStatusDraft,
	}

	// TTS (สำหรับทุก style)
	reel.TTSText = req.TTSText
	reel.TTSVoice = req.TTSVoice

	// Check if using new style-based system
	if req.Style != "" {
		// NEW: Style-based composition
		reel.Style = models.ReelStyle(req.Style)
		reel.Line1 = req.Line1
		reel.Line2 = req.Line2
		reel.ShowLogo = true // default
		if req.ShowLogo != nil {
			reel.ShowLogo = *req.ShowLogo
		}
		// Crop position for square/fullcover styles (default to center)
		reel.CropX = 50.0
		reel.CropY = 50.0
		if req.CropX > 0 {
			reel.CropX = req.CropX
		}
		if req.CropY > 0 {
			reel.CropY = req.CropY
		}
		logger.InfoContext(ctx, "Creating style-based reel",
			"style", req.Style,
			"title", req.Title,
			"crop_x", reel.CropX,
			"crop_y", reel.CropY,
			"has_tts", req.TTSText != "",
		)
	} else {
		// LEGACY: Layer-based composition
		outputFormat := models.OutputFormat9x16 // default
		if req.OutputFormat != "" {
			outputFormat = models.OutputFormat(req.OutputFormat)
		}
		videoFit := models.VideoFitFill // default
		if req.VideoFit != "" {
			videoFit = models.VideoFit(req.VideoFit)
		}
		cropX := 50.0 // default center
		if req.CropX > 0 {
			cropX = req.CropX
		}
		cropY := 50.0 // default center
		if req.CropY > 0 {
			cropY = req.CropY
		}

		reel.Description = req.Description
		reel.OutputFormat = outputFormat
		reel.VideoFit = videoFit
		reel.CropX = cropX
		reel.CropY = cropY
		reel.TemplateID = req.TemplateID
		reel.Layers = layers
	}

	if err := s.reelRepo.Create(ctx, reel); err != nil {
		logger.ErrorContext(ctx, "Failed to create reel", "error", err)
		return nil, err
	}

	// 7. Load relations สำหรับ response
	reel.Video = video
	reel.Template = template

	logger.InfoContext(ctx, "Reel created successfully",
		"reel_id", reel.ID,
		"video_code", video.Code,
	)

	return reel, nil
}

// GetByID ดึง reel ตาม ID
func (s *ReelServiceImpl) GetByID(ctx context.Context, id uuid.UUID) (*models.Reel, error) {
	return s.reelRepo.GetByIDWithRelations(ctx, id)
}

// GetByIDForUser ดึง reel ตาม ID (ตรวจสอบ ownership)
func (s *ReelServiceImpl) GetByIDForUser(ctx context.Context, id, userID uuid.UUID) (*models.Reel, error) {
	reel, err := s.reelRepo.GetByIDWithRelations(ctx, id)
	if err != nil {
		return nil, err
	}
	if reel == nil {
		return nil, errors.New("reel not found")
	}
	if reel.UserID != userID {
		return nil, errors.New("access denied")
	}
	return reel, nil
}

// Update อัปเดต reel
func (s *ReelServiceImpl) Update(ctx context.Context, id, userID uuid.UUID, req *dto.UpdateReelRequest) (*models.Reel, error) {
	logger.InfoContext(ctx, "Updating reel",
		"reel_id", id,
		"user_id", userID,
	)

	// 1. ดึง reel และตรวจสอบ ownership
	reel, err := s.reelRepo.GetByIDWithRelations(ctx, id)
	if err != nil {
		return nil, err
	}
	if reel == nil {
		return nil, errors.New("reel not found")
	}
	if reel.UserID != userID {
		return nil, errors.New("access denied")
	}

	// 2. ตรวจสอบว่าไม่ได้กำลัง export อยู่
	if reel.Status == models.ReelStatusExporting {
		return nil, errors.New("cannot update reel that is being exported")
	}

	// 3. อัปเดตฟิลด์ทั่วไป
	if req.Title != nil {
		reel.Title = *req.Title
	}

	// Multi-segment support
	if req.Segments != nil && len(*req.Segments) > 0 {
		segments := dto.SegmentRequestsToModels(*req.Segments)
		// Validate each segment
		for i, seg := range segments {
			if reel.Video != nil && seg.End > float64(reel.Video.Duration) {
				return nil, fmt.Errorf("segment %d end (%v) exceeds video duration (%v)", i+1, seg.End, reel.Video.Duration)
			}
			if seg.End <= seg.Start {
				return nil, fmt.Errorf("segment %d: end time must be greater than start time", i+1)
			}
		}
		// Validate total duration
		totalDuration := segments.TotalDuration()
		if totalDuration > 60 {
			return nil, fmt.Errorf("total duration (%v seconds) exceeds maximum (60 seconds)", totalDuration)
		}
		reel.Segments = segments
		reel.SegmentStart = segments[0].Start
		reel.SegmentEnd = segments[len(segments)-1].End
	} else {
		// LEGACY: Single segment update
		if req.SegmentStart != nil {
			reel.SegmentStart = *req.SegmentStart
		}
		if req.SegmentEnd != nil {
			if reel.Video != nil && *req.SegmentEnd > float64(reel.Video.Duration) {
				return nil, fmt.Errorf("segment end (%v) exceeds video duration (%v)", *req.SegmentEnd, reel.Video.Duration)
			}
			reel.SegmentEnd = *req.SegmentEnd
		}
		// Update legacy segments array if using single segment
		if req.SegmentStart != nil || req.SegmentEnd != nil {
			reel.Segments = models.VideoSegments{{Start: reel.SegmentStart, End: reel.SegmentEnd}}
		}
	}

	if req.CoverTime != nil {
		reel.CoverTime = *req.CoverTime
	}

	// NEW: Style-based fields
	if req.Style != nil {
		reel.Style = models.ReelStyle(*req.Style)
	}
	if req.Line1 != nil {
		reel.Line1 = *req.Line1
	}
	if req.Line2 != nil {
		reel.Line2 = *req.Line2
	}
	if req.ShowLogo != nil {
		reel.ShowLogo = *req.ShowLogo
	}

	// TTS
	if req.TTSText != nil {
		reel.TTSText = *req.TTSText
	}
	if req.TTSVoice != nil {
		reel.TTSVoice = *req.TTSVoice
	}

	// LEGACY: Layer-based fields
	if req.Description != nil {
		reel.Description = *req.Description
	}
	if req.OutputFormat != nil {
		reel.OutputFormat = models.OutputFormat(*req.OutputFormat)
	}
	if req.VideoFit != nil {
		reel.VideoFit = models.VideoFit(*req.VideoFit)
	}
	if req.CropX != nil {
		reel.CropX = *req.CropX
	}
	if req.CropY != nil {
		reel.CropY = *req.CropY
	}
	if req.TemplateID != nil {
		reel.TemplateID = req.TemplateID
		// Load new template
		if *req.TemplateID != uuid.Nil {
			template, err := s.templateRepo.GetByID(ctx, *req.TemplateID)
			if err == nil {
				reel.Template = template
			}
		}
	}
	if req.Layers != nil {
		reel.Layers = dto.LayerRequestsToModels(*req.Layers)
	}

	// 4. Reset status to draft if was failed
	if reel.Status == models.ReelStatusFailed {
		reel.Status = models.ReelStatusDraft
		reel.ExportError = ""
	}

	// 5. Save
	if err := s.reelRepo.Update(ctx, reel); err != nil {
		logger.ErrorContext(ctx, "Failed to update reel", "reel_id", id, "error", err)
		return nil, err
	}

	logger.InfoContext(ctx, "Reel updated successfully", "reel_id", id)

	return reel, nil
}

// Delete ลบ reel
func (s *ReelServiceImpl) Delete(ctx context.Context, id, userID uuid.UUID) error {
	logger.InfoContext(ctx, "Deleting reel",
		"reel_id", id,
		"user_id", userID,
	)

	// 1. ดึง reel และตรวจสอบ ownership
	reel, err := s.reelRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if reel == nil {
		return errors.New("reel not found")
	}
	if reel.UserID != userID {
		return errors.New("access denied")
	}

	// 2. ตรวจสอบว่าไม่ได้กำลัง export
	if reel.Status == models.ReelStatusExporting {
		return errors.New("cannot delete reel that is being exported")
	}

	// 3. ลบไฟล์จาก storage (E2/S3)
	if s.storage != nil && reel.Status == models.ReelStatusReady {
		// ใช้ DeleteFolder เพื่อลบ prefix reels/{reel_id}/ ทั้งหมด
		// รวมถึง output.mp4, thumb.jpg, cover.jpg
		folderPath := fmt.Sprintf("reels/%s", id.String())
		if err := s.storage.DeleteFolder(folderPath); err != nil {
			logger.WarnContext(ctx, "Failed to delete reel files from storage",
				"reel_id", id,
				"folder", folderPath,
				"error", err,
			)
			// ไม่ return error เพราะยังต้องลบ record
		} else {
			logger.InfoContext(ctx, "Reel files deleted from storage",
				"reel_id", id,
				"folder", folderPath,
			)
		}
	}

	// 4. ลบ record
	if err := s.reelRepo.Delete(ctx, id); err != nil {
		logger.ErrorContext(ctx, "Failed to delete reel", "reel_id", id, "error", err)
		return err
	}

	logger.InfoContext(ctx, "Reel deleted successfully", "reel_id", id)

	return nil
}

// ListByUser ดึง reels ของ user พร้อม pagination
func (s *ReelServiceImpl) ListByUser(ctx context.Context, userID uuid.UUID, page, limit int) ([]*models.Reel, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	return s.reelRepo.ListByUserID(ctx, userID, offset, limit)
}

// ListByVideo ดึง reels ที่สร้างจาก video นี้
func (s *ReelServiceImpl) ListByVideo(ctx context.Context, videoID uuid.UUID, page, limit int) ([]*models.Reel, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	return s.reelRepo.ListByVideoID(ctx, videoID, offset, limit)
}

// ListWithFilters ดึง reels พร้อม filters
func (s *ReelServiceImpl) ListWithFilters(ctx context.Context, userID uuid.UUID, params *dto.ReelFilterRequest) ([]*models.Reel, int64, error) {
	return s.reelRepo.ListWithFilters(ctx, userID, params)
}

// Export ส่ง reel ไป export queue
func (s *ReelServiceImpl) Export(ctx context.Context, id, userID uuid.UUID) error {
	logger.InfoContext(ctx, "Exporting reel",
		"reel_id", id,
		"user_id", userID,
	)

	// 1. ดึง reel และตรวจสอบ ownership
	reel, err := s.reelRepo.GetByIDWithRelations(ctx, id)
	if err != nil {
		return err
	}
	if reel == nil {
		return errors.New("reel not found")
	}
	if reel.UserID != userID {
		return errors.New("access denied")
	}

	// 2. ตรวจสอบ status (อนุญาตให้ re-export ได้ถ้าไม่ได้กำลัง export อยู่)
	if reel.Status == models.ReelStatusExporting {
		return errors.New("reel is already being exported")
	}
	// NOTE: อนุญาตให้ re-export reel ที่ ready แล้วได้ (เพื่อ update style/settings)

	// 3. ตรวจสอบว่ามี video
	if reel.Video == nil || reel.Video.HLSPath == "" {
		return errors.New("video HLS path not available")
	}

	// 4. อัปเดต status เป็น exporting
	if err := s.reelRepo.UpdateStatus(ctx, id, models.ReelStatusExporting, ""); err != nil {
		logger.ErrorContext(ctx, "Failed to update reel status", "reel_id", id, "error", err)
		return err
	}

	// 5. Publish job ไป NATS (ถ้ามี publisher)
	if s.jobPublisher != nil {
		// Convert segments to job format
		segments := make([]services.VideoSegmentJob, len(reel.GetSegments()))
		for i, seg := range reel.GetSegments() {
			segments[i] = services.VideoSegmentJob{
				Start: seg.Start,
				End:   seg.End,
			}
		}

		job := &services.ReelExportJob{
			ReelID:       reel.ID.String(),
			VideoID:      reel.VideoID.String(),
			VideoCode:    reel.Video.Code,
			HLSPath:      reel.Video.HLSPath,
			VideoQuality: reel.Video.Quality,
			Segments:     segments,
			SegmentStart: reel.SegmentStart, // LEGACY
			SegmentEnd:   reel.SegmentEnd,   // LEGACY
			CoverTime:    reel.CoverTime,
			OutputPath:   fmt.Sprintf("reels/%s/output.mp4", reel.ID.String()),
		}

		// TTS (สำหรับทุก style)
		job.TTSText = reel.TTSText
		job.TTSVoice = reel.TTSVoice

		// Check if using style-based or legacy layer-based
		if reel.IsStyleBased() {
			// NEW: Style-based job
			job.Style = string(reel.Style)
			job.Title = reel.Title
			job.Line1 = reel.Line1
			job.Line2 = reel.Line2
			job.ShowLogo = reel.ShowLogo
			job.CropX = reel.CropX // Crop position X (0-100)
			job.CropY = reel.CropY // Crop position Y (0-100)
			logger.InfoContext(ctx, "Publishing style-based reel export job",
				"reel_id", id,
				"style", reel.Style,
				"crop_x", reel.CropX,
				"crop_y", reel.CropY,
				"has_tts", reel.TTSText != "",
			)
		} else {
			// LEGACY: Layer-based job
			job.OutputFormat = string(reel.OutputFormat)
			job.VideoFit = string(reel.VideoFit)
			job.CropX = reel.CropX
			job.CropY = reel.CropY
			job.Layers = convertLayersToJobFormat(reel.Layers)
			logger.InfoContext(ctx, "Publishing legacy layer-based reel export job",
				"reel_id", id,
				"output_format", reel.OutputFormat,
			)
		}

		if err := s.jobPublisher.PublishReelExportJob(ctx, job); err != nil {
			// Rollback status
			s.reelRepo.UpdateStatus(ctx, id, models.ReelStatusFailed, "Failed to publish export job")
			logger.ErrorContext(ctx, "Failed to publish reel export job", "reel_id", id, "error", err)
			return fmt.Errorf("failed to publish export job: %w", err)
		}
	}

	logger.InfoContext(ctx, "Reel export job published",
		"reel_id", id,
		"video_code", reel.Video.Code,
	)

	return nil
}

// GetTemplates ดึง templates ทั้งหมด (active)
func (s *ReelServiceImpl) GetTemplates(ctx context.Context) ([]*models.ReelTemplate, error) {
	return s.templateRepo.ListActive(ctx)
}

// GetTemplateByID ดึง template ตาม ID
func (s *ReelServiceImpl) GetTemplateByID(ctx context.Context, id uuid.UUID) (*models.ReelTemplate, error) {
	return s.templateRepo.GetByID(ctx, id)
}

// convertLayersToJobFormat แปลง layers สำหรับ job
func convertLayersToJobFormat(layers models.ReelLayers) []services.ReelLayerJob {
	result := make([]services.ReelLayerJob, len(layers))
	for i, l := range layers {
		result[i] = services.ReelLayerJob{
			Type:       string(l.Type),
			Content:    l.Content,
			FontFamily: l.FontFamily,
			FontSize:   l.FontSize,
			FontColor:  l.FontColor,
			FontWeight: l.FontWeight,
			X:          l.X,
			Y:          l.Y,
			Width:      l.Width,
			Height:     l.Height,
			Opacity:    l.Opacity,
			ZIndex:     l.ZIndex,
			Style:      l.Style,
		}
	}
	return result
}
