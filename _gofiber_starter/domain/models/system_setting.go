package models

import (
	"time"

	"github.com/google/uuid"
)

// SystemSetting เก็บค่าตั้งค่าระบบแบบ key-value
type SystemSetting struct {
	ID          uuid.UUID  `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Category    string     `gorm:"size:50;not null;uniqueIndex:idx_settings_category_key"`
	Key         string     `gorm:"size:100;not null;uniqueIndex:idx_settings_category_key"`
	Value       string     `gorm:"type:text;not null"`
	ValueType   string     `gorm:"size:20;not null"` // string, number, boolean, json, secret
	Description string     `gorm:"type:text"`
	IsSecret    bool       `gorm:"default:false"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`
	UpdatedBy   *uuid.UUID `gorm:"type:uuid"`
}

func (SystemSetting) TableName() string {
	return "system_settings"
}

// SettingAuditLog บันทึกการเปลี่ยนแปลง settings
type SettingAuditLog struct {
	ID        uuid.UUID  `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Category  string     `gorm:"size:50;not null;index"`
	Key       string     `gorm:"size:100;not null"`
	OldValue  string     `gorm:"type:text"`
	NewValue  string     `gorm:"type:text"`
	Reason    string     `gorm:"type:text"` // เหตุผลที่แก้ไข (optional)
	ChangedBy *uuid.UUID `gorm:"type:uuid"`
	ChangedAt time.Time  `gorm:"default:now()"`
	IPAddress string     `gorm:"size:45"`
}

func (SettingAuditLog) TableName() string {
	return "setting_audit_logs"
}

// SettingValueType ประเภทของค่า
type SettingValueType string

const (
	SettingTypeString  SettingValueType = "string"
	SettingTypeNumber  SettingValueType = "number"
	SettingTypeBoolean SettingValueType = "boolean"
	SettingTypeJSON    SettingValueType = "json"
	SettingTypeSecret  SettingValueType = "secret"
)

// SettingCategory หมวดหมู่ของ settings (Business Settings เท่านั้น)
type SettingCategory string

const (
	SettingCategoryGeneral     SettingCategory = "general"     // ทั่วไป
	SettingCategoryTranscoding SettingCategory = "transcoding" // การแปลงวิดีโอ
	SettingCategoryAlert       SettingCategory = "alert"       // แจ้งเตือน
)

// ValidCategories รายการ categories ที่ถูกต้อง
var ValidCategories = []SettingCategory{
	SettingCategoryGeneral,
	SettingCategoryTranscoding,
	SettingCategoryAlert,
}

// IsValidCategory ตรวจสอบว่า category ถูกต้องหรือไม่
func IsValidCategory(cat string) bool {
	for _, valid := range ValidCategories {
		if string(valid) == cat {
			return true
		}
	}
	return false
}

// SettingSource แหล่งที่มาของค่า (สำหรับ API response)
type SettingSource string

const (
	SettingSourceEnv      SettingSource = "env"      // จาก .env file
	SettingSourceDatabase SettingSource = "database" // จาก database
	SettingSourceDefault  SettingSource = "default"  // จาก hardcoded default
)
