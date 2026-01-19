// ==================== Setting Types ====================

export type SettingValueType = 'string' | 'number' | 'boolean' | 'json' | 'secret'

export type SettingSource = 'env' | 'database' | 'default'

export interface SettingResponse {
  category: string
  key: string
  value: string
  value_type: SettingValueType
  description: string
  is_secret: boolean
  source: SettingSource // env, database, default
  env_key?: string       // ชื่อ ENV variable (ถ้ามี)
  is_locked: boolean     // true ถ้า ENV override
}

export interface CategoryInfo {
  name: string
  label: string
  description: string
  settingCount?: number
}

export interface AuditLogResponse {
  id: string
  category: string
  key: string
  old_value: string
  new_value: string
  reason?: string
  changed_by_id?: string
  changed_by_name?: string
  changed_at: string
  ip_address?: string
}

// ==================== Request Types ====================

export interface UpdateSettingsRequest {
  settings: Record<string, string>
  reason?: string
}

export interface ResetSettingsRequest {
  reason?: string
}

// ==================== Response Types ====================

export type SettingsByCategory = Record<string, SettingResponse[]>

// ==================== Category Constants ====================

export const SETTING_CATEGORIES: CategoryInfo[] = [
  { name: 'general', label: 'ทั่วไป', description: 'ชื่อเว็บไซต์ คำอธิบาย และขนาดไฟล์สูงสุด' },
  { name: 'transcoding', label: 'แปลงวิดีโอ', description: 'กำหนดความละเอียดของวิดีโอที่ต้องการแปลง' },
  { name: 'alert', label: 'แจ้งเตือน', description: 'Discord, Telegram และเงื่อนไขการแจ้งเตือน' },
]

// Helper function to get category label
export function getCategoryLabel(categoryName: string): string {
  const category = SETTING_CATEGORIES.find((c) => c.name === categoryName)
  return category?.label || categoryName
}

// Helper function to get category description
export function getCategoryDescription(categoryName: string): string {
  const category = SETTING_CATEGORIES.find((c) => c.name === categoryName)
  return category?.description || ''
}
