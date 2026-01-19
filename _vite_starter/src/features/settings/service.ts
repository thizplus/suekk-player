import { apiClient, type PaginationMeta } from '@/lib/api-client'
import { SETTINGS_ROUTES } from '@/constants/api-routes'
import type {
  SettingsByCategory,
  SettingResponse,
  CategoryInfo,
  AuditLogResponse,
  UpdateSettingsRequest,
  ResetSettingsRequest,
} from './types'

export const settingsService = {
  // ==================== Get Settings ====================

  /**
   * ดึง settings ทั้งหมด grouped by category
   */
  async getAll(): Promise<SettingsByCategory> {
    return apiClient.get<SettingsByCategory>(SETTINGS_ROUTES.ALL)
  },

  /**
   * ดึง settings ตาม category
   */
  async getByCategory(category: string): Promise<SettingResponse[]> {
    return apiClient.get<SettingResponse[]>(SETTINGS_ROUTES.BY_CATEGORY(category))
  },

  /**
   * ดึงรายชื่อ categories
   */
  async getCategories(): Promise<CategoryInfo[]> {
    return apiClient.get<CategoryInfo[]>(SETTINGS_ROUTES.CATEGORIES)
  },

  // ==================== Update Settings ====================

  /**
   * อัพเดท settings ของ category
   */
  async updateCategory(
    category: string,
    data: UpdateSettingsRequest
  ): Promise<SettingResponse[]> {
    return apiClient.put<SettingResponse[]>(SETTINGS_ROUTES.BY_CATEGORY(category), data)
  },

  /**
   * รีเซ็ต settings ของ category กลับเป็นค่า default
   */
  async resetCategory(
    category: string,
    data?: ResetSettingsRequest
  ): Promise<SettingResponse[]> {
    return apiClient.post<SettingResponse[]>(SETTINGS_ROUTES.RESET_CATEGORY(category), data || {})
  },

  // ==================== Audit Logs ====================

  /**
   * ดึง audit logs
   */
  async getAuditLogs(params?: { page?: number; limit?: number }): Promise<{
    data: AuditLogResponse[]
    meta: PaginationMeta
  }> {
    return apiClient.getPaginated<AuditLogResponse>(SETTINGS_ROUTES.AUDIT_LOGS, { params })
  },

  // ==================== Cache Management ====================

  /**
   * โหลด cache ใหม่
   */
  async reloadCache(): Promise<{ message: string }> {
    return apiClient.post<{ message: string }>(SETTINGS_ROUTES.RELOAD_CACHE, {})
  },
}
