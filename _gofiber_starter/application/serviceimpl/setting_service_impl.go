package serviceimpl

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
	"gofiber-template/pkg/settings"
)

type SettingServiceImpl struct {
	repo  repositories.SettingRepository
	cache *settings.SettingsCache
}

func NewSettingService(repo repositories.SettingRepository, cache *settings.SettingsCache) services.SettingService {
	return &SettingServiceImpl{
		repo:  repo,
		cache: cache,
	}
}

// GetAll ดึง settings ทั้งหมด grouped by category
func (s *SettingServiceImpl) GetAll(ctx context.Context) (map[string][]services.SettingResponse, error) {
	result := make(map[string][]services.SettingResponse)

	// ดึง settings จาก defaults ก่อน (เพื่อให้ได้ครบทุก key)
	for category, keys := range settings.DefaultSettings {
		for key, def := range keys {
			value, source := s.cache.GetWithSource(category, key)

			// Mask secret values
			displayValue := value
			if def.IsSecret && value != "" {
				displayValue = maskSecret(value)
			}

			resp := services.SettingResponse{
				Category:    category,
				Key:         key,
				Value:       displayValue,
				ValueType:   string(def.Type),
				Description: def.Description,
				IsSecret:    def.IsSecret,
				Source:      string(source),
				EnvKey:      s.cache.GetEnvKey(category, key),
				IsLocked:    s.cache.IsEnvOverridden(category, key),
			}
			result[category] = append(result[category], resp)
		}
		// Sort settings by key within each category
		sort.Slice(result[category], func(i, j int) bool {
			return result[category][i].Key < result[category][j].Key
		})
	}

	logger.InfoContext(ctx, "Settings retrieved", "categories", len(result))
	return result, nil
}

// GetByCategory ดึง settings ตาม category
func (s *SettingServiceImpl) GetByCategory(ctx context.Context, category string) ([]services.SettingResponse, error) {
	var result []services.SettingResponse

	catDefaults, ok := settings.DefaultSettings[category]
	if !ok {
		logger.WarnContext(ctx, "Category not found", "category", category)
		return result, nil
	}

	for key, def := range catDefaults {
		value, source := s.cache.GetWithSource(category, key)

		// Mask secret values
		displayValue := value
		if def.IsSecret && value != "" {
			displayValue = maskSecret(value)
		}

		resp := services.SettingResponse{
			Category:    category,
			Key:         key,
			Value:       displayValue,
			ValueType:   string(def.Type),
			Description: def.Description,
			IsSecret:    def.IsSecret,
			Source:      string(source),
			EnvKey:      s.cache.GetEnvKey(category, key),
			IsLocked:    s.cache.IsEnvOverridden(category, key),
		}
		result = append(result, resp)
	}

	// Sort settings by key
	sort.Slice(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})

	return result, nil
}

// Get ดึงค่า setting
func (s *SettingServiceImpl) Get(ctx context.Context, category, key string) (string, error) {
	return s.cache.Get(category, key), nil
}

// GetInt ดึงค่า setting เป็น int
func (s *SettingServiceImpl) GetInt(ctx context.Context, category, key string, fallback int) int {
	return s.cache.GetInt(category, key, fallback)
}

// GetBool ดึงค่า setting เป็น bool
func (s *SettingServiceImpl) GetBool(ctx context.Context, category, key string, fallback bool) bool {
	return s.cache.GetBool(category, key, fallback)
}

// Update อัพเดท settings หลายค่าพร้อมกัน
func (s *SettingServiceImpl) Update(ctx context.Context, category string, updates map[string]string, userID *uuid.UUID, reason, ipAddress string) error {
	// ตรวจสอบว่า category มีอยู่จริง
	catDefaults, ok := settings.DefaultSettings[category]
	if !ok {
		logger.WarnContext(ctx, "Invalid category for update", "category", category)
		return nil
	}

	for key, newValue := range updates {
		// ตรวจสอบว่า key มีอยู่จริง
		def, ok := catDefaults[key]
		if !ok {
			logger.WarnContext(ctx, "Invalid key for update", "category", category, "key", key)
			continue
		}

		// ข้าม settings ที่ถูก ENV override
		if s.cache.IsEnvOverridden(category, key) {
			logger.WarnContext(ctx, "Setting is locked by ENV", "category", category, "key", key)
			continue
		}

		// ดึงค่าเก่า
		oldValue := s.cache.Get(category, key)

		// ถ้าค่าเหมือนเดิมไม่ต้องอัพเดท
		if oldValue == newValue {
			continue
		}

		// บันทึกลง DB
		setting := &models.SystemSetting{
			Category:    category,
			Key:         key,
			Value:       newValue,
			ValueType:   string(def.Type),
			Description: def.Description,
			IsSecret:    def.IsSecret,
			UpdatedBy:   userID,
		}

		if err := s.repo.Upsert(ctx, setting); err != nil {
			logger.ErrorContext(ctx, "Failed to update setting",
				"category", category,
				"key", key,
				"error", err,
			)
			return err
		}

		// บันทึก Audit Log
		auditLog := &models.SettingAuditLog{
			Category:  category,
			Key:       key,
			OldValue:  maskIfSecret(oldValue, def.IsSecret),
			NewValue:  maskIfSecret(newValue, def.IsSecret),
			Reason:    reason,
			ChangedBy: userID,
			ChangedAt: time.Now(),
			IPAddress: ipAddress,
		}

		if err := s.repo.CreateAuditLog(ctx, auditLog); err != nil {
			logger.WarnContext(ctx, "Failed to create audit log",
				"category", category,
				"key", key,
				"error", err,
			)
			// ไม่ return error เพราะ audit log ไม่ critical
		}

		// อัพเดท cache
		s.cache.Set(category, key, newValue)

		logger.InfoContext(ctx, "Setting updated",
			"category", category,
			"key", key,
			"changed_by", userID,
			"reason", reason,
		)
	}

	return nil
}

// ResetToDefaults รีเซ็ต settings ของ category กลับเป็นค่า default
func (s *SettingServiceImpl) ResetToDefaults(ctx context.Context, category string, userID *uuid.UUID, reason, ipAddress string) error {
	catDefaults, ok := settings.DefaultSettings[category]
	if !ok {
		logger.WarnContext(ctx, "Invalid category for reset", "category", category)
		return nil
	}

	for key, def := range catDefaults {
		// ข้าม settings ที่ถูก ENV override
		if s.cache.IsEnvOverridden(category, key) {
			continue
		}

		oldValue := s.cache.Get(category, key)

		// ถ้าค่าเหมือน default อยู่แล้วไม่ต้องทำอะไร
		if oldValue == def.Value {
			continue
		}

		// ลบออกจาก DB (ให้ใช้ default)
		if err := s.repo.Delete(ctx, category, key); err != nil {
			logger.ErrorContext(ctx, "Failed to delete setting for reset",
				"category", category,
				"key", key,
				"error", err,
			)
			continue
		}

		// บันทึก Audit Log
		auditLog := &models.SettingAuditLog{
			Category:  category,
			Key:       key,
			OldValue:  maskIfSecret(oldValue, def.IsSecret),
			NewValue:  maskIfSecret(def.Value, def.IsSecret) + " (default)",
			Reason:    reason,
			ChangedBy: userID,
			ChangedAt: time.Now(),
			IPAddress: ipAddress,
		}
		s.repo.CreateAuditLog(ctx, auditLog)

		logger.InfoContext(ctx, "Setting reset to default",
			"category", category,
			"key", key,
		)
	}

	// Invalidate cache for category
	s.cache.Invalidate(category)

	// Reload cache
	s.cache.Reload(ctx)

	return nil
}

// GetAuditLogs ดึง audit logs
func (s *SettingServiceImpl) GetAuditLogs(ctx context.Context, limit, offset int) ([]services.AuditLogResponse, int64, error) {
	logs, total, err := s.repo.GetAuditLogs(ctx, limit, offset)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get audit logs", "error", err)
		return nil, 0, err
	}

	result := make([]services.AuditLogResponse, len(logs))
	for i, log := range logs {
		result[i] = services.AuditLogResponse{
			ID:          log.ID,
			Category:    log.Category,
			Key:         log.Key,
			OldValue:    log.OldValue,
			NewValue:    log.NewValue,
			Reason:      log.Reason,
			ChangedByID: log.ChangedBy,
			ChangedAt:   log.ChangedAt.Format(time.RFC3339),
			IPAddress:   log.IPAddress,
		}
	}

	return result, total, nil
}

// InvalidateCache ล้าง cache ของ category
func (s *SettingServiceImpl) InvalidateCache(category string) {
	s.cache.Invalidate(category)
}

// ReloadCache โหลด cache ใหม่
func (s *SettingServiceImpl) ReloadCache(ctx context.Context) error {
	return s.cache.Reload(ctx)
}

// InitializeDefaults สร้างค่า default ใน DB ถ้ายังไม่มี
func (s *SettingServiceImpl) InitializeDefaults(ctx context.Context) error {
	defaults := settings.GetDefaultModels()

	if err := s.repo.InsertDefaults(ctx, defaults); err != nil {
		logger.ErrorContext(ctx, "Failed to initialize defaults", "error", err)
		return err
	}

	logger.InfoContext(ctx, "Settings defaults initialized", "count", len(defaults))
	return nil
}

// Helper functions

// maskSecret ซ่อนค่า secret โดยแสดงแค่บางส่วน
func maskSecret(value string) string {
	if len(value) <= 8 {
		return "********"
	}
	// แสดง 4 ตัวแรกและ 4 ตัวท้าย
	return value[:4] + strings.Repeat("*", len(value)-8) + value[len(value)-4:]
}

// maskIfSecret ซ่อนค่าถ้าเป็น secret
func maskIfSecret(value string, isSecret bool) string {
	if isSecret && value != "" {
		return maskSecret(value)
	}
	return value
}
