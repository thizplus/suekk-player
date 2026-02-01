import { apiClient, type PaginationMeta } from '@/lib/api-client'
import { USER_ROUTES } from '@/constants/api-routes'
import type { UserProfile, UserListItem, UpdateProfilePayload, UserListParams, SetPasswordPayload } from './types'

export const userService = {
  async getList(params?: UserListParams): Promise<{ data: UserListItem[]; meta: PaginationMeta }> {
    // Response: { success: true, data: [...], meta: { total, page, limit, ... } }
    return apiClient.getPaginated<UserListItem>(USER_ROUTES.LIST, { params })
  },

  async getById(id: string): Promise<UserProfile> {
    // Response: { success: true, data: UserProfile }
    return apiClient.get<UserProfile>(USER_ROUTES.BY_ID(id))
  },

  async getProfile(): Promise<UserProfile> {
    // Response: { success: true, data: UserProfile }
    return apiClient.get<UserProfile>(USER_ROUTES.PROFILE)
  },

  async updateProfile(payload: UpdateProfilePayload): Promise<UserProfile> {
    // Response: { success: true, data: UserProfile }
    return apiClient.patch<UserProfile>(USER_ROUTES.PROFILE, payload)
  },

  // ตั้ง password สำหรับ Google users
  async setPassword(payload: SetPasswordPayload): Promise<void> {
    return apiClient.post(USER_ROUTES.SET_PASSWORD, payload)
  },
}
