package postgres

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
)

type SettingRepositoryImpl struct {
	db *gorm.DB
}

func NewSettingRepository(db *gorm.DB) repositories.SettingRepository {
	return &SettingRepositoryImpl{db: db}
}

// GetAll ดึง settings ทั้งหมด
func (r *SettingRepositoryImpl) GetAll(ctx context.Context) ([]*models.SystemSetting, error) {
	var settings []*models.SystemSetting
	err := r.db.WithContext(ctx).Order("category ASC, key ASC").Find(&settings).Error
	return settings, err
}

// GetByCategory ดึง settings ตาม category
func (r *SettingRepositoryImpl) GetByCategory(ctx context.Context, category string) ([]*models.SystemSetting, error) {
	var settings []*models.SystemSetting
	err := r.db.WithContext(ctx).
		Where("category = ?", category).
		Order("key ASC").
		Find(&settings).Error
	return settings, err
}

// GetByKey ดึง setting ตาม category และ key
func (r *SettingRepositoryImpl) GetByKey(ctx context.Context, category, key string) (*models.SystemSetting, error) {
	var setting models.SystemSetting
	err := r.db.WithContext(ctx).
		Where("category = ? AND key = ?", category, key).
		First(&setting).Error
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

// Upsert สร้างหรืออัพเดท setting
func (r *SettingRepositoryImpl) Upsert(ctx context.Context, setting *models.SystemSetting) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "category"}, {Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at", "updated_by"}),
		}).
		Create(setting).Error
}

// UpsertMany สร้างหรืออัพเดทหลาย settings
func (r *SettingRepositoryImpl) UpsertMany(ctx context.Context, settings []*models.SystemSetting) error {
	if len(settings) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, setting := range settings {
			err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "category"}, {Name: "key"}},
				DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at", "updated_by"}),
			}).Create(setting).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// Delete ลบ setting
func (r *SettingRepositoryImpl) Delete(ctx context.Context, category, key string) error {
	return r.db.WithContext(ctx).
		Where("category = ? AND key = ?", category, key).
		Delete(&models.SystemSetting{}).Error
}

// DeleteByCategory ลบ settings ทั้ง category
func (r *SettingRepositoryImpl) DeleteByCategory(ctx context.Context, category string) error {
	return r.db.WithContext(ctx).
		Where("category = ?", category).
		Delete(&models.SystemSetting{}).Error
}

// CreateAuditLog สร้าง audit log
func (r *SettingRepositoryImpl) CreateAuditLog(ctx context.Context, log *models.SettingAuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// GetAuditLogs ดึง audit logs
func (r *SettingRepositoryImpl) GetAuditLogs(ctx context.Context, limit int, offset int) ([]*models.SettingAuditLog, int64, error) {
	var logs []*models.SettingAuditLog
	var total int64

	// Count total
	if err := r.db.WithContext(ctx).Model(&models.SettingAuditLog{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get logs
	err := r.db.WithContext(ctx).
		Order("changed_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&logs).Error

	return logs, total, err
}

// GetAuditLogsByCategory ดึง audit logs ตาม category
func (r *SettingRepositoryImpl) GetAuditLogsByCategory(ctx context.Context, category string, limit int) ([]*models.SettingAuditLog, error) {
	var logs []*models.SettingAuditLog
	err := r.db.WithContext(ctx).
		Where("category = ?", category).
		Order("changed_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

// InsertDefaults ใส่ค่า default (ไม่ overwrite ถ้ามีอยู่แล้ว)
func (r *SettingRepositoryImpl) InsertDefaults(ctx context.Context, settings []*models.SystemSetting) error {
	if len(settings) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, setting := range settings {
			// Insert only if not exists
			err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "category"}, {Name: "key"}},
				DoNothing: true,
			}).Create(setting).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// GetAllGroupedByCategory ดึง settings ทั้งหมดและจัดกลุ่มตาม category
func (r *SettingRepositoryImpl) GetAllGroupedByCategory(ctx context.Context) (map[string][]*models.SystemSetting, error) {
	settings, err := r.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]*models.SystemSetting)
	for _, setting := range settings {
		result[setting.Category] = append(result[setting.Category], setting)
	}

	return result, nil
}
