package services

import (
	"context"

	"github.com/google/uuid"
)

// SettingService interface สำหรับจัดการ settings
type SettingService interface {
	// Get settings
	GetAll(ctx context.Context) (map[string][]SettingResponse, error)
	GetByCategory(ctx context.Context, category string) ([]SettingResponse, error)
	Get(ctx context.Context, category, key string) (string, error)
	GetInt(ctx context.Context, category, key string, fallback int) int
	GetBool(ctx context.Context, category, key string, fallback bool) bool

	// Update settings
	Update(ctx context.Context, category string, updates map[string]string, userID *uuid.UUID, reason, ipAddress string) error
	ResetToDefaults(ctx context.Context, category string, userID *uuid.UUID, reason, ipAddress string) error

	// Audit logs
	GetAuditLogs(ctx context.Context, limit, offset int) ([]AuditLogResponse, int64, error)

	// Cache management
	InvalidateCache(category string)
	ReloadCache(ctx context.Context) error

	// Initialize defaults
	InitializeDefaults(ctx context.Context) error
}

// SettingResponse response สำหรับ setting
type SettingResponse struct {
	Category    string `json:"category"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	ValueType   string `json:"value_type"`
	Description string `json:"description"`
	IsSecret    bool   `json:"is_secret"`
	Source      string `json:"source"`      // env, database, default
	EnvKey      string `json:"env_key,omitempty"` // ชื่อ ENV variable (ถ้ามี)
	IsLocked    bool   `json:"is_locked"`  // true ถ้า ENV override
}

// AuditLogResponse response สำหรับ audit log
type AuditLogResponse struct {
	ID            uuid.UUID `json:"id"`
	Category      string    `json:"category"`
	Key           string    `json:"key"`
	OldValue      string    `json:"old_value"`
	NewValue      string    `json:"new_value"`
	Reason        string    `json:"reason,omitempty"`
	ChangedByID   *uuid.UUID `json:"changed_by_id,omitempty"`
	ChangedByName string    `json:"changed_by_name,omitempty"`
	ChangedAt     string    `json:"changed_at"`
	IPAddress     string    `json:"ip_address,omitempty"`
}

// CategoryInfo ข้อมูล category
type CategoryInfo struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Description string `json:"description"`
	SettingCount int   `json:"setting_count"`
}

// GetCategoryInfo ดึงข้อมูล categories ทั้งหมด
func GetCategoryInfo() []CategoryInfo {
	return []CategoryInfo{
		{Name: "general", Label: "ทั่วไป", Description: "ตั้งค่าทั่วไปของระบบ"},
		{Name: "p2p", Label: "P2P", Description: "ตั้งค่า P2P Streaming"},
		{Name: "transcoding", Label: "แปลงไฟล์", Description: "ตั้งค่าการแปลงวิดีโอ"},
		{Name: "worker", Label: "Worker", Description: "ตั้งค่า Worker"},
		{Name: "disk_monitor", Label: "Disk Monitor", Description: "ตั้งค่าการตรวจสอบพื้นที่ดิสก์"},
		{Name: "storage", Label: "Storage", Description: "ตั้งค่าการเก็บไฟล์"},
		{Name: "alert", Label: "แจ้งเตือน", Description: "ตั้งค่าการแจ้งเตือน"},
		{Name: "logging", Label: "Log", Description: "ตั้งค่าการบันทึก Log"},
	}
}
