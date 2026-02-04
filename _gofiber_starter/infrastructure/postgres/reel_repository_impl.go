package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
	"gorm.io/gorm"
)

type reelRepository struct {
	db *gorm.DB
}

// NewReelRepository สร้าง reel repository
func NewReelRepository(db *gorm.DB) repositories.ReelRepository {
	return &reelRepository{db: db}
}

func (r *reelRepository) Create(ctx context.Context, reel *models.Reel) error {
	return r.db.WithContext(ctx).Create(reel).Error
}

func (r *reelRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Reel, error) {
	var reel models.Reel
	if err := r.db.WithContext(ctx).First(&reel, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &reel, nil
}

func (r *reelRepository) GetByIDWithRelations(ctx context.Context, id uuid.UUID) (*models.Reel, error) {
	var reel models.Reel
	if err := r.db.WithContext(ctx).
		Preload("Video").
		Preload("Template").
		First(&reel, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &reel, nil
}

func (r *reelRepository) Update(ctx context.Context, reel *models.Reel) error {
	return r.db.WithContext(ctx).Save(reel).Error
}

func (r *reelRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.Reel{}, "id = ?", id).Error
}

func (r *reelRepository) ListByUserID(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*models.Reel, int64, error) {
	var reels []*models.Reel
	var total int64

	// Count total
	if err := r.db.WithContext(ctx).Model(&models.Reel{}).
		Where("user_id = ?", userID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get list with relations
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Preload("Video").
		Preload("Template").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&reels).Error; err != nil {
		return nil, 0, err
	}

	return reels, total, nil
}

func (r *reelRepository) ListByVideoID(ctx context.Context, videoID uuid.UUID, offset, limit int) ([]*models.Reel, int64, error) {
	var reels []*models.Reel
	var total int64

	// Count total
	if err := r.db.WithContext(ctx).Model(&models.Reel{}).
		Where("video_id = ?", videoID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get list with relations
	if err := r.db.WithContext(ctx).
		Where("video_id = ?", videoID).
		Preload("Video").
		Preload("Template").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&reels).Error; err != nil {
		return nil, 0, err
	}

	return reels, total, nil
}

func (r *reelRepository) ListWithFilters(ctx context.Context, userID uuid.UUID, params *dto.ReelFilterRequest) ([]*models.Reel, int64, error) {
	var reels []*models.Reel
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Reel{}).Where("user_id = ?", userID)

	// Filter by video
	if params.VideoID != "" {
		videoID, err := uuid.Parse(params.VideoID)
		if err == nil {
			query = query.Where("video_id = ?", videoID)
		}
	}

	// Filter by status
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}

	// Search by title
	if params.Search != "" {
		query = query.Where("title ILIKE ?", "%"+params.Search+"%")
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Sort
	sortBy := "created_at"
	if params.SortBy != "" {
		sortBy = params.SortBy
	}
	sortOrder := "DESC"
	if params.SortOrder == "asc" {
		sortOrder = "ASC"
	}
	query = query.Order(sortBy + " " + sortOrder)

	// Pagination
	page := params.Page
	if page < 1 {
		page = 1
	}
	limit := params.Limit
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	// Get list with relations
	if err := query.
		Preload("Video").
		Preload("Template").
		Offset(offset).
		Limit(limit).
		Find(&reels).Error; err != nil {
		return nil, 0, err
	}

	return reels, total, nil
}

func (r *reelRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status models.ReelStatus, errorMsg string) error {
	updates := map[string]interface{}{
		"status":       status,
		"export_error": errorMsg,
	}
	if status == models.ReelStatusReady {
		now := time.Now()
		updates["exported_at"] = &now
	}
	return r.db.WithContext(ctx).Model(&models.Reel{}).Where("id = ?", id).Updates(updates).Error
}

func (r *reelRepository) GetByStatus(ctx context.Context, status models.ReelStatus, offset, limit int) ([]*models.Reel, error) {
	var reels []*models.Reel
	if err := r.db.WithContext(ctx).
		Where("status = ?", status).
		Preload("Video").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&reels).Error; err != nil {
		return nil, err
	}
	return reels, nil
}

func (r *reelRepository) UpdateOutput(ctx context.Context, id uuid.UUID, outputPath, thumbnailURL string, duration int, fileSize int64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&models.Reel{}).Where("id = ?", id).Updates(map[string]interface{}{
		"output_path":   outputPath,
		"thumbnail_url": thumbnailURL,
		"duration":      duration,
		"file_size":     fileSize,
		"status":        models.ReelStatusReady,
		"exported_at":   &now,
	}).Error
}

func (r *reelRepository) CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.Reel{}).
		Where("user_id = ?", userID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *reelRepository) CountByVideoID(ctx context.Context, videoID uuid.UUID) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.Reel{}).
		Where("video_id = ?", videoID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountByVideoIDs นับ reels สำหรับหลาย videos พร้อมกัน (batch query)
func (r *reelRepository) CountByVideoIDs(ctx context.Context, videoIDs []uuid.UUID) (map[uuid.UUID]int64, error) {
	result := make(map[uuid.UUID]int64)

	if len(videoIDs) == 0 {
		return result, nil
	}

	// Query: SELECT video_id, COUNT(*) FROM reels WHERE video_id IN (...) GROUP BY video_id
	type countResult struct {
		VideoID uuid.UUID `gorm:"column:video_id"`
		Count   int64     `gorm:"column:count"`
	}

	var counts []countResult
	if err := r.db.WithContext(ctx).
		Model(&models.Reel{}).
		Select("video_id, COUNT(*) as count").
		Where("video_id IN ?", videoIDs).
		Group("video_id").
		Scan(&counts).Error; err != nil {
		return nil, err
	}

	// Convert to map
	for _, c := range counts {
		result[c.VideoID] = c.Count
	}

	// Initialize missing video IDs with 0
	for _, id := range videoIDs {
		if _, exists := result[id]; !exists {
			result[id] = 0
		}
	}

	return result, nil
}

// === Reel Template Repository ===

type reelTemplateRepository struct {
	db *gorm.DB
}

// NewReelTemplateRepository สร้าง reel template repository
func NewReelTemplateRepository(db *gorm.DB) repositories.ReelTemplateRepository {
	return &reelTemplateRepository{db: db}
}

func (r *reelTemplateRepository) Create(ctx context.Context, template *models.ReelTemplate) error {
	return r.db.WithContext(ctx).Create(template).Error
}

func (r *reelTemplateRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ReelTemplate, error) {
	var template models.ReelTemplate
	if err := r.db.WithContext(ctx).First(&template, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &template, nil
}

func (r *reelTemplateRepository) Update(ctx context.Context, template *models.ReelTemplate) error {
	return r.db.WithContext(ctx).Save(template).Error
}

func (r *reelTemplateRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.ReelTemplate{}, "id = ?", id).Error
}

func (r *reelTemplateRepository) ListActive(ctx context.Context) ([]*models.ReelTemplate, error) {
	var templates []*models.ReelTemplate
	if err := r.db.WithContext(ctx).
		Where("is_active = ?", true).
		Order("sort_order ASC, name ASC").
		Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}

func (r *reelTemplateRepository) ListAll(ctx context.Context) ([]*models.ReelTemplate, error) {
	var templates []*models.ReelTemplate
	if err := r.db.WithContext(ctx).
		Order("sort_order ASC, name ASC").
		Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}
