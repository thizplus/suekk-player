package serviceimpl

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
)

type ReelServiceImpl struct {
	reelRepo     repositories.ReelRepository
	templateRepo repositories.ReelTemplateRepository
	videoRepo    repositories.VideoRepository
	jobPublisher services.ReelJobPublisher
}

func NewReelService(
	reelRepo repositories.ReelRepository,
	templateRepo repositories.ReelTemplateRepository,
	videoRepo repositories.VideoRepository,
	jobPublisher services.ReelJobPublisher,
) services.ReelService {
	return &ReelServiceImpl{
		reelRepo:     reelRepo,
		templateRepo: templateRepo,
		videoRepo:    videoRepo,
		jobPublisher: jobPublisher,
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
	if req.SegmentEnd > float64(video.Duration) {
		return nil, fmt.Errorf("segment end (%v) exceeds video duration (%v)", req.SegmentEnd, video.Duration)
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
	reel := &models.Reel{
		UserID:       userID,
		VideoID:      req.VideoID,
		Title:        req.Title,
		SegmentStart: req.SegmentStart,
		SegmentEnd:   req.SegmentEnd,
		Status:       models.ReelStatusDraft,
	}

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
		logger.InfoContext(ctx, "Creating style-based reel",
			"style", req.Style,
			"title", req.Title,
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

	// 2. ตรวจสอบว่ายังอยู่ใน draft mode
	if reel.Status != models.ReelStatusDraft && reel.Status != models.ReelStatusFailed {
		return nil, errors.New("cannot update reel that is being exported or already exported")
	}

	// 3. อัปเดตฟิลด์ทั่วไป
	if req.Title != nil {
		reel.Title = *req.Title
	}
	if req.SegmentStart != nil {
		reel.SegmentStart = *req.SegmentStart
	}
	if req.SegmentEnd != nil {
		// ตรวจสอบว่าไม่เกิน video duration
		if reel.Video != nil && *req.SegmentEnd > float64(reel.Video.Duration) {
			return nil, fmt.Errorf("segment end (%v) exceeds video duration (%v)", *req.SegmentEnd, reel.Video.Duration)
		}
		reel.SegmentEnd = *req.SegmentEnd
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

	// TODO: ลบไฟล์ output จาก S3 ถ้ามี

	// 3. ลบ record
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

	// 2. ตรวจสอบ status
	if reel.Status == models.ReelStatusExporting {
		return errors.New("reel is already being exported")
	}
	if reel.Status == models.ReelStatusReady {
		return errors.New("reel is already exported")
	}

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
		job := &services.ReelExportJob{
			ReelID:       reel.ID.String(),
			VideoID:      reel.VideoID.String(),
			VideoCode:    reel.Video.Code,
			HLSPath:      reel.Video.HLSPath,
			VideoQuality: reel.Video.Quality,
			SegmentStart: reel.SegmentStart,
			SegmentEnd:   reel.SegmentEnd,
			OutputPath:   fmt.Sprintf("reels/%s/%s.mp4", reel.Video.Code, reel.ID.String()),
		}

		// Check if using style-based or legacy layer-based
		if reel.IsStyleBased() {
			// NEW: Style-based job
			job.Style = string(reel.Style)
			job.Title = reel.Title
			job.Line1 = reel.Line1
			job.Line2 = reel.Line2
			job.ShowLogo = reel.ShowLogo
			logger.InfoContext(ctx, "Publishing style-based reel export job",
				"reel_id", id,
				"style", reel.Style,
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
