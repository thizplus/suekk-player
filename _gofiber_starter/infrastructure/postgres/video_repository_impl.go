package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"gofiber-template/domain/dto"
	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
)

type VideoRepositoryImpl struct {
	db *gorm.DB
}

func NewVideoRepository(db *gorm.DB) repositories.VideoRepository {
	return &VideoRepositoryImpl{db: db}
}

func (r *VideoRepositoryImpl) Create(ctx context.Context, video *models.Video) error {
	return r.db.WithContext(ctx).Create(video).Error
}

func (r *VideoRepositoryImpl) GetByID(ctx context.Context, id uuid.UUID) (*models.Video, error) {
	var video models.Video
	err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Category").
		Where("id = ?", id).
		First(&video).Error
	if err != nil {
		return nil, err
	}
	return &video, nil
}

func (r *VideoRepositoryImpl) GetByCode(ctx context.Context, code string) (*models.Video, error) {
	var video models.Video
	err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Category").
		Where("code = ?", code).
		First(&video).Error
	if err != nil {
		return nil, err
	}
	return &video, nil
}

func (r *VideoRepositoryImpl) GetByUserID(ctx context.Context, userID uuid.UUID, offset, limit int) ([]*models.Video, error) {
	var videos []*models.Video
	err := r.db.WithContext(ctx).
		Preload("Category").
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&videos).Error
	return videos, err
}

func (r *VideoRepositoryImpl) GetByCategory(ctx context.Context, categoryID uuid.UUID, offset, limit int) ([]*models.Video, error) {
	var videos []*models.Video
	err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Category").
		Where("category_id = ?", categoryID).
		Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&videos).Error
	return videos, err
}

func (r *VideoRepositoryImpl) GetByStatus(ctx context.Context, status models.VideoStatus, offset, limit int) ([]*models.Video, error) {
	var videos []*models.Video
	err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Category").
		Where("status = ?", status).
		Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&videos).Error
	return videos, err
}

func (r *VideoRepositoryImpl) Update(ctx context.Context, video *models.Video) error {
	return r.db.WithContext(ctx).Save(video).Error
}

func (r *VideoRepositoryImpl) UpdateStatus(ctx context.Context, id uuid.UUID, status models.VideoStatus) error {
	return r.db.WithContext(ctx).
		Model(&models.Video{}).
		Where("id = ?", id).
		Update("status", status).Error
}

func (r *VideoRepositoryImpl) UpdateHLSPath(ctx context.Context, id uuid.UUID, hlsPath string) error {
	return r.db.WithContext(ctx).
		Model(&models.Video{}).
		Where("id = ?", id).
		Update("hls_path", hlsPath).Error
}

func (r *VideoRepositoryImpl) ClearOriginalPath(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.Video{}).
		Where("id = ?", id).
		Update("original_path", "").Error
}

func (r *VideoRepositoryImpl) IncrementViews(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.Video{}).
		Where("id = ?", id).
		UpdateColumn("views", gorm.Expr("views + ?", 1)).Error
}

func (r *VideoRepositoryImpl) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.Video{}).Error
}

func (r *VideoRepositoryImpl) List(ctx context.Context, offset, limit int) ([]*models.Video, error) {
	var videos []*models.Video
	err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Category").
		Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&videos).Error
	return videos, err
}

func (r *VideoRepositoryImpl) ListReady(ctx context.Context, offset, limit int) ([]*models.Video, error) {
	var videos []*models.Video
	err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Category").
		Where("status = ?", models.VideoStatusReady).
		Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&videos).Error
	return videos, err
}

// ListWithFilters ดึง videos พร้อม filter, search, sort, pagination
func (r *VideoRepositoryImpl) ListWithFilters(ctx context.Context, params *dto.VideoFilterRequest) ([]*models.Video, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.Video{}).
		Preload("User").
		Preload("Category").
		Preload("Subtitles")

	// Search (title หรือ code)
	if params.Search != "" {
		searchTerm := "%" + params.Search + "%"
		query = query.Where("title ILIKE ? OR code ILIKE ?", searchTerm, searchTerm)
	}

	// Filter by status
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}

	// Filter by category
	if params.CategoryID != "" {
		query = query.Where("category_id = ?", params.CategoryID)
	}

	// Filter by user
	if params.UserID != "" {
		query = query.Where("user_id = ?", params.UserID)
	}

	// Filter by date range
	if params.DateFrom != "" {
		query = query.Where("created_at >= ?", params.DateFrom)
	}
	if params.DateTo != "" {
		query = query.Where("created_at <= ?", params.DateTo+" 23:59:59")
	}

	// Count total (before pagination)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Sort
	sortBy := "created_at"
	sortOrder := "DESC"
	if params.SortBy != "" {
		sortBy = params.SortBy
	}
	if params.SortOrder == "asc" {
		sortOrder = "ASC"
	}
	query = query.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))

	// Pagination
	page := params.Page
	if page < 1 {
		page = 1
	}
	limit := params.Limit
	if limit < 1 {
		limit = 20
	}
	offset := (page - 1) * limit
	query = query.Offset(offset).Limit(limit)

	var videos []*models.Video
	if err := query.Find(&videos).Error; err != nil {
		return nil, 0, err
	}

	return videos, total, nil
}

func (r *VideoRepositoryImpl) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.Video{}).Count(&count).Error
	return count, err
}

func (r *VideoRepositoryImpl) CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Video{}).
		Where("user_id = ?", userID).
		Count(&count).Error
	return count, err
}

func (r *VideoRepositoryImpl) CountByStatus(ctx context.Context, status models.VideoStatus) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Video{}).
		Where("status = ?", status).
		Count(&count).Error
	return count, err
}

// GetStuckByStatus ดึง videos ที่ค้างสถานะนานเกิน threshold
func (r *VideoRepositoryImpl) GetStuckByStatus(ctx context.Context, status models.VideoStatus, threshold time.Time) ([]*models.Video, error) {
	var videos []*models.Video
	err := r.db.WithContext(ctx).
		Preload("User").
		Where("status = ? AND updated_at < ?", status, threshold).
		Order("updated_at ASC").
		Find(&videos).Error
	return videos, err
}

// GetStuckProcessing ดึง videos ที่ processing_started_at เกิน threshold (fast stuck detection)
// ใช้สำหรับตรวจจับ job ที่ค้างใน processing นานเกิน 1 นาที
func (r *VideoRepositoryImpl) GetStuckProcessing(ctx context.Context, threshold time.Time) ([]*models.Video, error) {
	var videos []*models.Video
	err := r.db.WithContext(ctx).
		Preload("User").
		Where("status = ? AND processing_started_at IS NOT NULL AND processing_started_at < ?", models.VideoStatusProcessing, threshold).
		Order("processing_started_at ASC").
		Find(&videos).Error
	return videos, err
}

// MarkVideoFailed อัพเดท video เป็น failed พร้อม error message และ increment retry_count
func (r *VideoRepositoryImpl) MarkVideoFailed(ctx context.Context, id uuid.UUID, errorMsg string) error {
	return r.db.WithContext(ctx).
		Model(&models.Video{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":                "failed",
			"last_error":            errorMsg,
			"retry_count":           gorm.Expr("retry_count + ?", 1),
			"processing_started_at": nil,
			"updated_at":            time.Now(),
		}).Error
}

// ResetForRetry reset retry_count และ last_error สำหรับ retry จาก DLQ
func (r *VideoRepositoryImpl) ResetForRetry(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.Video{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"retry_count":           0,
			"last_error":            nil,
			"processing_started_at": nil,
			"status":                "pending",
			"updated_at":            time.Now(),
		}).Error
}

// UpdateProcessingTimestamp อัพเดท processing_started_at เป็นเวลาปัจจุบัน
// ใช้เพื่อ reset stuck detection timer เมื่อ worker ส่ง progress update มา
func (r *VideoRepositoryImpl) UpdateProcessingTimestamp(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.Video{}).
		Where("id = ? AND status = ?", id, models.VideoStatusProcessing).
		Update("processing_started_at", time.Now()).Error
}

// AppendErrorHistory เพิ่ม error record ลงใน error_history JSONB array
func (r *VideoRepositoryImpl) AppendErrorHistory(ctx context.Context, id uuid.UUID, record models.ErrorRecord) error {
	// ใช้ PostgreSQL jsonb_array_append หรือ || operator
	return r.db.WithContext(ctx).
		Model(&models.Video{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"error_history": gorm.Expr("COALESCE(error_history, '[]'::jsonb) || ?::jsonb", record),
			"last_error":    record.Error,
			"updated_at":    time.Now(),
		}).Error
}

// DeleteAll ลบ videos ทั้งหมด
func (r *VideoRepositoryImpl) DeleteAll(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).Exec("DELETE FROM videos")
	return result.RowsAffected, result.Error
}

// GetTotalStorageUsed คำนวณ disk_usage รวมทุก video (bytes)
func (r *VideoRepositoryImpl) GetTotalStorageUsed(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Model(&models.Video{}).
		Select("COALESCE(SUM(disk_usage), 0)").
		Scan(&total).Error
	return total, err
}
