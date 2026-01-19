import { apiClient } from '@/lib/api-client'
import { DASHBOARD_ROUTES, STORAGE_ROUTES } from '@/constants/api-routes'
import type { DashboardStats, StorageUsage } from './types'

export const dashboardService = {
  async getStats(): Promise<DashboardStats> {
    // Response: { success: true, data: DashboardStats }
    return apiClient.get<DashboardStats>(DASHBOARD_ROUTES.STATS)
  },

  async getAdminDashboard(): Promise<DashboardStats> {
    // Response: { success: true, data: DashboardStats }
    return apiClient.get<DashboardStats>(DASHBOARD_ROUTES.ADMIN)
  },

  async getAgentDashboard(): Promise<DashboardStats> {
    // Response: { success: true, data: DashboardStats }
    return apiClient.get<DashboardStats>(DASHBOARD_ROUTES.AGENT)
  },

  async getSalesDashboard(): Promise<DashboardStats> {
    // Response: { success: true, data: DashboardStats }
    return apiClient.get<DashboardStats>(DASHBOARD_ROUTES.SALES)
  },

  async getStorageUsage(): Promise<StorageUsage> {
    // Response: { success: true, data: StorageUsage }
    return apiClient.get<StorageUsage>(STORAGE_ROUTES.USAGE)
  },
}
