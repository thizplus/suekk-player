package serviceimpl

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
	"gofiber-template/domain/services"
	"gofiber-template/infrastructure/nats"
	"gofiber-template/pkg/logger"
)

// WarmCachePublisher interface สำหรับส่ง warm cache jobs
type WarmCachePublisher interface {
	PublishWarmCacheJob(ctx context.Context, job *nats.WarmCacheJob) error
}

// SubtitleStreamPurger interface สำหรับ purge subtitle stream (แยกจาก transcode/warmcache)
type SubtitleStreamPurger interface {
	PurgeSubtitleStream(ctx context.Context) (uint64, error)
}

type QueueServiceImpl struct {
	videoRepo            repositories.VideoRepository
	subtitleRepo         repositories.SubtitleRepository
	reelRepo             repositories.ReelRepository
	transcodingService   services.TranscodingService
	subtitleService      services.SubtitleService
	warmCachePublisher   WarmCachePublisher
	subtitleStreamPurger SubtitleStreamPurger
}

func NewQueueService(
	videoRepo repositories.VideoRepository,
	subtitleRepo repositories.SubtitleRepository,
	reelRepo repositories.ReelRepository,
	transcodingService services.TranscodingService,
	subtitleService services.SubtitleService,
	warmCachePublisher WarmCachePublisher,
	subtitleStreamPurger SubtitleStreamPurger,
) services.QueueService {
	return &QueueServiceImpl{
		videoRepo:            videoRepo,
		subtitleRepo:         subtitleRepo,
		reelRepo:             reelRepo,
		transcodingService:   transcodingService,
		subtitleService:      subtitleService,
		warmCachePublisher:   warmCachePublisher,
		subtitleStreamPurger: subtitleStreamPurger,
	}
}

// === Stats ===

func (s *QueueServiceImpl) GetQueueStats(ctx context.Context) (*dto.QueueStatsResponse, error) {
	logger.InfoContext(ctx, "Getting queue stats")

	// Transcode stats
	pending, _ := s.videoRepo.CountByStatus(ctx, models.VideoStatusPending)
	queued, _ := s.videoRepo.CountByStatus(ctx, models.VideoStatusQueued)
	processing, _ := s.videoRepo.CountByStatus(ctx, models.VideoStatusProcessing)
	failed, _ := s.videoRepo.CountByStatus(ctx, models.VideoStatusFailed)
	deadLetter, _ := s.videoRepo.CountByStatus(ctx, models.VideoStatusDeadLetter)

	// Subtitle stats
	subtitleQueued, _ := s.countSubtitlesByStatus(ctx, models.SubtitleStatusQueued)
	subtitleProcessing, _ := s.countSubtitlesByProcessing(ctx)
	subtitleFailed, _ := s.countSubtitlesByStatus(ctx, models.SubtitleStatusFailed)

	// Warm cache stats
	notCached, warming, cached, warmFailed := s.countCacheStats(ctx)

	// Reel stats
	reelDraft, reelExporting, reelReady, reelFailed := s.countReelStats(ctx)

	return &dto.QueueStatsResponse{
		Transcode: dto.TranscodeStats{
			Pending:    pending,
			Queued:     queued,
			Processing: processing,
			Failed:     failed,
			DeadLetter: deadLetter,
		},
		Subtitle: dto.SubtitleStats{
			Queued:     subtitleQueued,
			Processing: subtitleProcessing,
			Failed:     subtitleFailed,
		},
		WarmCache: dto.WarmCacheStats{
			NotCached: notCached,
			Warming:   warming,
			Cached:    cached,
			Failed:    warmFailed,
		},
		Reel: dto.ReelStats{
			Draft:     reelDraft,
			Exporting: reelExporting,
			Ready:     reelReady,
			Failed:    reelFailed,
		},
	}, nil
}

func (s *QueueServiceImpl) countSubtitlesByStatus(ctx context.Context, status models.SubtitleStatus) (int64, error) {
	subs, err := s.subtitleRepo.GetByStatus(ctx, status)
	if err != nil {
		return 0, err
	}
	return int64(len(subs)), nil
}

func (s *QueueServiceImpl) countSubtitlesByProcessing(ctx context.Context) (int64, error) {
	var count int64
	for _, status := range []models.SubtitleStatus{
		models.SubtitleStatusProcessing,
		models.SubtitleStatusTranslating,
		models.SubtitleStatusDetecting,
	} {
		subs, _ := s.subtitleRepo.GetByStatus(ctx, status)
		count += int64(len(subs))
	}
	return count, nil
}

func (s *QueueServiceImpl) countCacheStats(ctx context.Context) (notCached, warming, cached, failed int64) {
	// Query videos with ready status and check cache_status
	// Note: This is simplified - in production you might want dedicated repo methods
	readyVideos, _ := s.videoRepo.GetByStatus(ctx, models.VideoStatusReady, 0, 10000)
	for _, v := range readyVideos {
		switch v.CacheStatus {
		case "pending", "":
			notCached++
		case "warming":
			warming++
		case "cached":
			cached++
		case "failed":
			failed++
		default:
			notCached++
		}
	}
	return
}

func (s *QueueServiceImpl) countReelStats(ctx context.Context) (draft, exporting, ready, failed int64) {
	if s.reelRepo == nil {
		return 0, 0, 0, 0
	}

	// Count by each status
	draftReels, _ := s.reelRepo.GetByStatus(ctx, models.ReelStatusDraft, 0, 10000)
	draft = int64(len(draftReels))

	exportingReels, _ := s.reelRepo.GetByStatus(ctx, models.ReelStatusExporting, 0, 10000)
	exporting = int64(len(exportingReels))

	readyReels, _ := s.reelRepo.GetByStatus(ctx, models.ReelStatusReady, 0, 10000)
	ready = int64(len(readyReels))

	failedReels, _ := s.reelRepo.GetByStatus(ctx, models.ReelStatusFailed, 0, 10000)
	failed = int64(len(failedReels))

	return
}

// === Transcode Queue ===

func (s *QueueServiceImpl) GetTranscodeFailed(ctx context.Context, page, limit int) ([]dto.TranscodeQueueItem, int64, error) {
	offset := (page - 1) * limit
	videos, err := s.videoRepo.GetByStatus(ctx, models.VideoStatusFailed, offset, limit)
	if err != nil {
		return nil, 0, err
	}

	total, _ := s.videoRepo.CountByStatus(ctx, models.VideoStatusFailed)

	items := make([]dto.TranscodeQueueItem, len(videos))
	for i, v := range videos {
		items[i] = dto.TranscodeQueueItem{
			ID:         v.ID,
			Code:       v.Code,
			Title:      v.Title,
			Status:     string(v.Status),
			Error:      v.LastError,
			RetryCount: v.RetryCount,
			CreatedAt:  v.CreatedAt,
			UpdatedAt:  v.UpdatedAt,
		}
	}

	return items, total, nil
}

func (s *QueueServiceImpl) RetryTranscodeFailed(ctx context.Context) (*dto.RetryResponse, error) {
	logger.InfoContext(ctx, "Retrying all transcode failed videos")

	videos, err := s.videoRepo.GetByStatus(ctx, models.VideoStatusFailed, 0, 1000)
	if err != nil {
		return nil, err
	}

	response := &dto.RetryResponse{
		TotalFound: len(videos),
	}

	if len(videos) == 0 {
		response.Message = "No failed videos found"
		return response, nil
	}

	var errors []string
	for _, v := range videos {
		if err := s.transcodingService.QueueTranscoding(ctx, v.ID); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", v.Code, err))
			response.Skipped++
		} else {
			response.TotalRetried++
		}
	}

	response.Errors = errors
	response.Message = fmt.Sprintf("Retried %d/%d failed videos", response.TotalRetried, response.TotalFound)

	logger.InfoContext(ctx, "Retry transcode failed completed",
		"total_found", response.TotalFound,
		"total_retried", response.TotalRetried,
	)

	return response, nil
}

func (s *QueueServiceImpl) RetryTranscodeOne(ctx context.Context, videoID uuid.UUID) error {
	return s.transcodingService.QueueTranscoding(ctx, videoID)
}

// === Subtitle Queue ===

func (s *QueueServiceImpl) GetSubtitleStuck(ctx context.Context, page, limit int) ([]dto.SubtitleQueueItem, int64, error) {
	subs, err := s.subtitleRepo.GetByStatus(ctx, models.SubtitleStatusQueued)
	if err != nil {
		return nil, 0, err
	}

	total := int64(len(subs))

	// Simple pagination
	start := (page - 1) * limit
	end := start + limit
	if start > len(subs) {
		return []dto.SubtitleQueueItem{}, total, nil
	}
	if end > len(subs) {
		end = len(subs)
	}
	pagedSubs := subs[start:end]

	items := make([]dto.SubtitleQueueItem, len(pagedSubs))
	for i, sub := range pagedSubs {
		videoCode := ""
		videoTitle := ""
		if sub.Video != nil {
			videoCode = sub.Video.Code
			videoTitle = sub.Video.Title
		}

		items[i] = dto.SubtitleQueueItem{
			ID:         sub.ID,
			VideoID:    sub.VideoID,
			VideoCode:  videoCode,
			VideoTitle: videoTitle,
			Language:   sub.Language,
			Type:       string(sub.Type),
			Status:     string(sub.Status),
			Error:      sub.Error,
			CreatedAt:  sub.CreatedAt,
			UpdatedAt:  sub.UpdatedAt,
		}
	}

	return items, total, nil
}

func (s *QueueServiceImpl) GetSubtitleFailed(ctx context.Context, page, limit int) ([]dto.SubtitleQueueItem, int64, error) {
	subs, err := s.subtitleRepo.GetByStatus(ctx, models.SubtitleStatusFailed)
	if err != nil {
		return nil, 0, err
	}

	total := int64(len(subs))

	// Simple pagination
	start := (page - 1) * limit
	end := start + limit
	if start > len(subs) {
		return []dto.SubtitleQueueItem{}, total, nil
	}
	if end > len(subs) {
		end = len(subs)
	}
	pagedSubs := subs[start:end]

	items := make([]dto.SubtitleQueueItem, len(pagedSubs))
	for i, sub := range pagedSubs {
		videoCode := ""
		videoTitle := ""
		if sub.Video != nil {
			videoCode = sub.Video.Code
			videoTitle = sub.Video.Title
		}

		items[i] = dto.SubtitleQueueItem{
			ID:         sub.ID,
			VideoID:    sub.VideoID,
			VideoCode:  videoCode,
			VideoTitle: videoTitle,
			Language:   sub.Language,
			Type:       string(sub.Type),
			Status:     string(sub.Status),
			Error:      sub.Error,
			CreatedAt:  sub.CreatedAt,
			UpdatedAt:  sub.UpdatedAt,
		}
	}

	return items, total, nil
}

func (s *QueueServiceImpl) RetrySubtitleStuck(ctx context.Context) (*dto.RetryResponse, error) {
	// Reuse existing subtitle service method
	result, err := s.subtitleService.RetryStuckSubtitles(ctx)
	if err != nil {
		return nil, err
	}

	return &dto.RetryResponse{
		TotalFound:   result.TotalFound,
		TotalRetried: result.TotalRetried,
		Skipped:      result.Skipped,
		Message:      result.Message,
		Errors:       result.Errors,
	}, nil
}

func (s *QueueServiceImpl) ClearSubtitleStuck(ctx context.Context) (*dto.ClearResponse, error) {
	logger.InfoContext(ctx, "Clearing all stuck subtitles and purging NATS queue")

	response := &dto.ClearResponse{}

	// 1. Purge NATS subtitle stream (ไม่กระทบ transcode/warmcache)
	if s.subtitleStreamPurger != nil {
		purgedCount, err := s.subtitleStreamPurger.PurgeSubtitleStream(ctx)
		if err != nil {
			logger.WarnContext(ctx, "Failed to purge subtitle stream", "error", err)
			// Continue anyway - still delete DB records
		} else {
			response.NATSJobsPurged = int(purgedCount)
			logger.InfoContext(ctx, "Purged NATS subtitle stream", "jobs_purged", purgedCount)
		}
	}

	// 2. ดึง subtitles ที่ status = queued จาก DB
	stuckSubtitles, err := s.subtitleRepo.GetByStatus(ctx, models.SubtitleStatusQueued)
	if err != nil {
		return nil, err
	}

	response.TotalFound = len(stuckSubtitles)

	// 3. ลบ subtitle records ใน DB
	for _, subtitle := range stuckSubtitles {
		if err := s.subtitleRepo.Delete(ctx, subtitle.ID); err != nil {
			logger.WarnContext(ctx, "Failed to delete stuck subtitle",
				"subtitle_id", subtitle.ID,
				"error", err,
			)
			response.Skipped++
			continue
		}
		response.TotalDeleted++
	}

	response.Message = fmt.Sprintf("Purged %d NATS jobs, deleted %d/%d DB records",
		response.NATSJobsPurged, response.TotalDeleted, response.TotalFound)

	logger.InfoContext(ctx, "Clear subtitle queue completed",
		"nats_purged", response.NATSJobsPurged,
		"db_deleted", response.TotalDeleted,
	)

	return response, nil
}

// QueueMissingSubtitles สแกน videos ที่ยังไม่มี subtitle แล้ว queue ใหม่
func (s *QueueServiceImpl) QueueMissingSubtitles(ctx context.Context) (*dto.QueueMissingResponse, error) {
	logger.InfoContext(ctx, "Scanning videos for missing subtitles")

	response := &dto.QueueMissingResponse{}

	// 1. ดึง videos ทั้งหมดที่ ready
	videos, err := s.videoRepo.GetByStatus(ctx, models.VideoStatusReady, 0, 10000)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get ready videos", "error", err)
		return nil, err
	}

	response.TotalVideos = len(videos)
	logger.InfoContext(ctx, "Found ready videos", "count", len(videos))

	// 2. วนลูปตรวจสอบแต่ละ video
	for _, video := range videos {
		// 2.1 ต้องมี audio
		if video.AudioPath == "" {
			response.Skipped++
			continue
		}

		// 2.2 ตรวจสอบว่ามี original subtitle ที่ ready หรือกำลังทำอยู่หรือไม่
		existingOriginal, _ := s.subtitleRepo.GetOriginalByVideoID(ctx, video.ID)
		if existingOriginal != nil {
			// มี subtitle อยู่แล้ว
			if existingOriginal.Status == models.SubtitleStatusReady {
				// ready แล้ว ไม่ต้องทำอะไร
				continue
			}
			if existingOriginal.IsInProgress() {
				// กำลังทำอยู่ ไม่ต้อง queue ซ้ำ
				continue
			}
			// failed → ลบแล้ว queue ใหม่
			s.subtitleRepo.Delete(ctx, existingOriginal.ID)
		}

		// 2.3 Video นี้ต้องการ subtitle → Queue ผ่าน SubtitleService
		response.TotalMissing++

		_, err := s.subtitleService.TriggerTranscribe(ctx, video.ID)
		if err != nil {
			logger.WarnContext(ctx, "Failed to queue transcribe for video",
				"video_id", video.ID,
				"video_code", video.Code,
				"error", err,
			)
			response.Skipped++
			continue
		}
		response.TotalQueued++
	}

	response.Message = fmt.Sprintf("Queued %d/%d videos for transcription (%d skipped)",
		response.TotalQueued, response.TotalMissing, response.Skipped)

	logger.InfoContext(ctx, "Queue missing subtitles completed",
		"total_videos", response.TotalVideos,
		"total_missing", response.TotalMissing,
		"total_queued", response.TotalQueued,
		"skipped", response.Skipped,
	)

	return response, nil
}

// === Warm Cache Queue ===

func (s *QueueServiceImpl) GetWarmCachePending(ctx context.Context, page, limit int) ([]dto.WarmCacheQueueItem, int64, error) {
	// Get all ready videos
	videos, err := s.videoRepo.GetByStatus(ctx, models.VideoStatusReady, 0, 10000)
	if err != nil {
		return nil, 0, err
	}

	// Filter to only pending cache status
	var pendingVideos []*models.Video
	for _, v := range videos {
		if v.CacheStatus == "pending" || v.CacheStatus == "" {
			pendingVideos = append(pendingVideos, v)
		}
	}

	total := int64(len(pendingVideos))

	// Pagination
	start := (page - 1) * limit
	end := start + limit
	if start > len(pendingVideos) {
		return []dto.WarmCacheQueueItem{}, total, nil
	}
	if end > len(pendingVideos) {
		end = len(pendingVideos)
	}
	pagedVideos := pendingVideos[start:end]

	items := make([]dto.WarmCacheQueueItem, len(pagedVideos))
	for i, v := range pagedVideos {
		var lastWarmed *string
		if v.LastWarmedAt != nil {
			t := v.LastWarmedAt.Format("2006-01-02T15:04:05Z")
			lastWarmed = &t
		}

		items[i] = dto.WarmCacheQueueItem{
			ID:              v.ID,
			Code:            v.Code,
			Title:           v.Title,
			CacheStatus:     v.CacheStatus,
			CachePercentage: v.CachePercentage,
			Qualities:       v.GetQualities(),
			Error:           v.CacheError,
			LastWarmedAt:    lastWarmed,
		}
	}

	return items, total, nil
}

func (s *QueueServiceImpl) GetWarmCacheFailed(ctx context.Context, page, limit int) ([]dto.WarmCacheQueueItem, int64, error) {
	// Get all ready videos
	videos, err := s.videoRepo.GetByStatus(ctx, models.VideoStatusReady, 0, 10000)
	if err != nil {
		return nil, 0, err
	}

	// Filter to only failed cache status
	var failedVideos []*models.Video
	for _, v := range videos {
		if v.CacheStatus == "failed" {
			failedVideos = append(failedVideos, v)
		}
	}

	total := int64(len(failedVideos))

	// Pagination
	start := (page - 1) * limit
	end := start + limit
	if start > len(failedVideos) {
		return []dto.WarmCacheQueueItem{}, total, nil
	}
	if end > len(failedVideos) {
		end = len(failedVideos)
	}
	pagedVideos := failedVideos[start:end]

	items := make([]dto.WarmCacheQueueItem, len(pagedVideos))
	for i, v := range pagedVideos {
		var lastWarmed *string
		if v.LastWarmedAt != nil {
			t := v.LastWarmedAt.Format("2006-01-02T15:04:05Z")
			lastWarmed = &t
		}

		items[i] = dto.WarmCacheQueueItem{
			ID:              v.ID,
			Code:            v.Code,
			Title:           v.Title,
			CacheStatus:     v.CacheStatus,
			CachePercentage: v.CachePercentage,
			Qualities:       v.GetQualities(),
			Error:           v.CacheError,
			LastWarmedAt:    lastWarmed,
		}
	}

	return items, total, nil
}

func (s *QueueServiceImpl) WarmCacheOne(ctx context.Context, videoID uuid.UUID) (*dto.WarmCacheResponse, error) {
	video, err := s.videoRepo.GetByID(ctx, videoID)
	if err != nil {
		return nil, err
	}
	if video == nil {
		return nil, fmt.Errorf("video not found")
	}

	if video.Status != models.VideoStatusReady {
		return nil, fmt.Errorf("video is not ready")
	}

	// Build segment counts from quality sizes
	segmentCounts := make(map[string]int)
	for quality := range video.QualitySizes {
		// Estimate segment count from size (rough estimate: 10MB per 10sec segment)
		// This is a placeholder - in production you'd get actual segment count
		segmentCounts[quality] = 100 // Default estimate
	}

	// Publish warm cache job
	if s.warmCachePublisher == nil {
		return nil, fmt.Errorf("warm cache publisher not available")
	}

	job := nats.NewWarmCacheJob(
		video.ID.String(),
		video.Code,
		fmt.Sprintf("hls/%s", video.Code),
		segmentCounts,
		3, // Priority 3 = manual/backfill
	)

	if err := s.warmCachePublisher.PublishWarmCacheJob(ctx, job); err != nil {
		return nil, err
	}

	// Update cache status to warming
	video.CacheStatus = "warming"
	if err := s.videoRepo.Update(ctx, video); err != nil {
		logger.WarnContext(ctx, "Failed to update cache status", "video_id", videoID, "error", err)
	}

	logger.InfoContext(ctx, "Warm cache job published",
		"video_id", videoID,
		"video_code", video.Code,
	)

	return &dto.WarmCacheResponse{
		VideoID: video.ID.String(),
		Code:    video.Code,
		Message: "Warm cache job published",
	}, nil
}

func (s *QueueServiceImpl) WarmCacheAll(ctx context.Context) (*dto.WarmAllResponse, error) {
	logger.InfoContext(ctx, "Warming all pending cache")

	// Get all ready videos with pending cache
	videos, err := s.videoRepo.GetByStatus(ctx, models.VideoStatusReady, 0, 10000)
	if err != nil {
		return nil, err
	}

	var pendingVideos []*models.Video
	for _, v := range videos {
		if v.CacheStatus == "pending" || v.CacheStatus == "" {
			pendingVideos = append(pendingVideos, v)
		}
	}

	response := &dto.WarmAllResponse{
		TotalFound: len(pendingVideos),
	}

	if len(pendingVideos) == 0 {
		response.Message = "No videos pending cache warming"
		return response, nil
	}

	if s.warmCachePublisher == nil {
		return nil, fmt.Errorf("warm cache publisher not available")
	}

	for _, v := range pendingVideos {
		segmentCounts := make(map[string]int)
		for quality := range v.QualitySizes {
			segmentCounts[quality] = 100
		}

		job := nats.NewWarmCacheJob(
			v.ID.String(),
			v.Code,
			fmt.Sprintf("hls/%s", v.Code),
			segmentCounts,
			3,
		)

		if err := s.warmCachePublisher.PublishWarmCacheJob(ctx, job); err != nil {
			logger.WarnContext(ctx, "Failed to publish warm cache job",
				"video_id", v.ID,
				"error", err,
			)
			continue
		}

		// Update cache status
		v.CacheStatus = "warming"
		s.videoRepo.Update(ctx, v)

		response.TotalQueued++
	}

	response.Message = fmt.Sprintf("Queued %d/%d videos for cache warming", response.TotalQueued, response.TotalFound)

	logger.InfoContext(ctx, "Warm cache all completed",
		"total_found", response.TotalFound,
		"total_queued", response.TotalQueued,
	)

	return response, nil
}
