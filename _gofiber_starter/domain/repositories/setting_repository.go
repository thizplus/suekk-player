package repositories

import (
	"context"

	"github.com/google/uuid"
	"gofiber-template/domain/models"
)

type SettingRepository interface {
	// Settings CRUD
	GetAll(ctx context.Context) ([]*models.SystemSetting, error)
	GetByCategory(ctx context.Context, category string) ([]*models.SystemSetting, error)
	GetByKey(ctx context.Context, category, key string) (*models.SystemSetting, error)
	Upsert(ctx context.Context, setting *models.SystemSetting) error
	UpsertMany(ctx context.Context, settings []*models.SystemSetting) error
	Delete(ctx context.Context, category, key string) error
	DeleteByCategory(ctx context.Context, category string) error

	// Audit Logs
	CreateAuditLog(ctx context.Context, log *models.SettingAuditLog) error
	GetAuditLogs(ctx context.Context, limit int, offset int) ([]*models.SettingAuditLog, int64, error)
	GetAuditLogsByCategory(ctx context.Context, category string, limit int) ([]*models.SettingAuditLog, error)

	// Bulk operations
	InsertDefaults(ctx context.Context, settings []*models.SystemSetting) error
	GetAllGroupedByCategory(ctx context.Context) (map[string][]*models.SystemSetting, error)
}

// SettingWithSource เพิ่ม source information
type SettingWithSource struct {
	*models.SystemSetting
	Source models.SettingSource `json:"source"`
}

// AuditLogWithUser รวมข้อมูล user
type AuditLogWithUser struct {
	*models.SettingAuditLog
	ChangedByName  string `json:"changed_by_name,omitempty"`
	ChangedByEmail string `json:"changed_by_email,omitempty"`
}

// GetAuditLogsWithUser ดึง audit logs พร้อมข้อมูล user
func GetAuditLogsWithUserQuery(ctx context.Context, userRepo UserRepository, logs []*models.SettingAuditLog) ([]*AuditLogWithUser, error) {
	result := make([]*AuditLogWithUser, len(logs))
	userCache := make(map[uuid.UUID]*models.User)

	for i, log := range logs {
		result[i] = &AuditLogWithUser{SettingAuditLog: log}

		if log.ChangedBy != nil {
			if user, ok := userCache[*log.ChangedBy]; ok {
				result[i].ChangedByName = user.Username
				result[i].ChangedByEmail = user.Email
			} else {
				user, err := userRepo.GetByID(ctx, *log.ChangedBy)
				if err == nil && user != nil {
					userCache[*log.ChangedBy] = user
					result[i].ChangedByName = user.Username
					result[i].ChangedByEmail = user.Email
				}
			}
		}
	}

	return result, nil
}
