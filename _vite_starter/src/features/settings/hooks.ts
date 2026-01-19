import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { settingsService } from './service'
import type { UpdateSettingsRequest, ResetSettingsRequest } from './types'

// ==================== Query Keys ====================

export const settingsKeys = {
  all: ['settings'] as const,
  list: () => [...settingsKeys.all, 'list'] as const,
  byCategory: (category: string) => [...settingsKeys.all, 'category', category] as const,
  categories: () => [...settingsKeys.all, 'categories'] as const,
  auditLogs: (params?: { page?: number; limit?: number }) =>
    [...settingsKeys.all, 'auditLogs', params] as const,
}

// ==================== Get Settings ====================

/**
 * ดึง settings ทั้งหมด grouped by category
 */
export function useAllSettings() {
  return useQuery({
    queryKey: settingsKeys.list(),
    queryFn: () => settingsService.getAll(),
    staleTime: 30 * 1000, // 30 seconds
  })
}

/**
 * ดึง settings ตาม category
 */
export function useSettingsByCategory(category: string) {
  return useQuery({
    queryKey: settingsKeys.byCategory(category),
    queryFn: () => settingsService.getByCategory(category),
    enabled: !!category,
    staleTime: 30 * 1000,
  })
}

/**
 * ดึงรายชื่อ categories
 */
export function useSettingCategories() {
  return useQuery({
    queryKey: settingsKeys.categories(),
    queryFn: () => settingsService.getCategories(),
    staleTime: 5 * 60 * 1000, // 5 minutes
  })
}

// ==================== Update Settings ====================

/**
 * อัพเดท settings ของ category
 */
export function useUpdateSettings() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ category, data }: { category: string; data: UpdateSettingsRequest }) =>
      settingsService.updateCategory(category, data),
    onSuccess: (_, { category }) => {
      // Invalidate specific category and all settings list
      queryClient.invalidateQueries({ queryKey: settingsKeys.byCategory(category) })
      queryClient.invalidateQueries({ queryKey: settingsKeys.list() })
      // Also invalidate audit logs since we added new entries
      queryClient.invalidateQueries({ queryKey: settingsKeys.all })
    },
  })
}

/**
 * รีเซ็ต settings ของ category กลับเป็นค่า default
 */
export function useResetSettings() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ category, data }: { category: string; data?: ResetSettingsRequest }) =>
      settingsService.resetCategory(category, data),
    onSuccess: (_, { category }) => {
      queryClient.invalidateQueries({ queryKey: settingsKeys.byCategory(category) })
      queryClient.invalidateQueries({ queryKey: settingsKeys.list() })
      queryClient.invalidateQueries({ queryKey: settingsKeys.all })
    },
  })
}

// ==================== Audit Logs ====================

/**
 * ดึง audit logs
 */
export function useAuditLogs(params?: { page?: number; limit?: number }) {
  return useQuery({
    queryKey: settingsKeys.auditLogs(params),
    queryFn: () => settingsService.getAuditLogs(params),
    staleTime: 10 * 1000, // 10 seconds
  })
}

// ==================== Cache Management ====================

/**
 * โหลด cache ใหม่
 */
export function useReloadCache() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: () => settingsService.reloadCache(),
    onSuccess: () => {
      // Invalidate all settings after cache reload
      queryClient.invalidateQueries({ queryKey: settingsKeys.all })
    },
  })
}
