package settings

import "gofiber-template/domain/models"

// DefaultSettings - Business Settings สำหรับ Admin UI
// ค่า System/Infrastructure อยู่ใน .env หรือ config โดยตรง
var DefaultSettings = map[string]map[string]SettingDefinition{
	// ทั่วไป - Branding และ Limits
	"general": {
		"site_title":       {Value: "Suekk Stream", Type: models.SettingTypeString, Description: "ชื่อเว็บไซต์"},
		"site_description": {Value: "ระบบจัดการวิดีโอสตรีมมิ่ง", Type: models.SettingTypeString, Description: "คำอธิบายเว็บไซต์"},
		"max_upload_size":  {Value: "10", Type: models.SettingTypeNumber, Description: "ขนาดไฟล์สูงสุดที่อัปโหลดได้ (GB)"},
	},
	// การแปลงวิดีโอ - Transcoding settings
	"transcoding": {
		"default_qualities": {Value: "1080p,720p,480p", Type: models.SettingTypeString, Description: "ความละเอียดที่ต้องการแปลง (คั่นด้วย ,)"},
		"auto_queue":        {Value: "true", Type: models.SettingTypeBoolean, Description: "เข้าคิวอัตโนมัติหลังอัปโหลด"},
		"max_queue_size":    {Value: "100", Type: models.SettingTypeNumber, Description: "จำนวน jobs สูงสุดในคิว (0 = ไม่จำกัด)"},
	},
	// การแจ้งเตือน - Notification settings
	"alert": {
		"enabled":               {Value: "false", Type: models.SettingTypeBoolean, Description: "เปิดใช้งานการแจ้งเตือน"},
		"telegram_bot_token":    {Value: "", Type: models.SettingTypeSecret, Description: "Telegram Bot Token", IsSecret: true},
		"telegram_chat_id":      {Value: "", Type: models.SettingTypeString, Description: "Telegram Chat ID"},
		"on_transcode_complete": {Value: "false", Type: models.SettingTypeBoolean, Description: "แจ้งเตือนเมื่อแปลงไฟล์สำเร็จ"},
		"on_transcode_fail":     {Value: "true", Type: models.SettingTypeBoolean, Description: "แจ้งเตือนเมื่อแปลงไฟล์ล้มเหลว"},
		"on_worker_offline":     {Value: "true", Type: models.SettingTypeBoolean, Description: "แจ้งเตือนเมื่อ Worker ออฟไลน์"},
		"on_dlq":                {Value: "true", Type: models.SettingTypeBoolean, Description: "แจ้งเตือนเมื่อวิดีโอเข้า Dead Letter Queue"},
	},
}

// EnvMapping mapping จาก setting key ไปยัง ENV variable (Level 1 - Override สูงสุด)
var EnvMapping = map[string]string{
	// General
	"general.site_title": "APP_NAME",
}

// SettingDefinition คำอธิบายของ setting
type SettingDefinition struct {
	Value       string
	Type        models.SettingValueType
	Description string
	IsSecret    bool
}

// GetDefaultModels แปลง DefaultSettings เป็น models สำหรับ insert
func GetDefaultModels() []*models.SystemSetting {
	var result []*models.SystemSetting

	for category, keys := range DefaultSettings {
		for key, def := range keys {
			result = append(result, &models.SystemSetting{
				Category:    category,
				Key:         key,
				Value:       def.Value,
				ValueType:   string(def.Type),
				Description: def.Description,
				IsSecret:    def.IsSecret,
			})
		}
	}

	return result
}
