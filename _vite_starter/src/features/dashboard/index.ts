// Barrel exports for dashboard feature
export { AdminDashboard } from './pages/AdminDashboard'
export { AgentDashboard } from './pages/AgentDashboard'
export { SalesDashboard } from './pages/SalesDashboard'
export {
  useDashboardStats,
  useAdminDashboard,
  useAgentDashboard,
  useSalesDashboard,
  useStorageUsage,
  dashboardKeys,
} from './hooks'
export { dashboardService } from './service'
export type { DashboardStats, DashboardData, ActivityItem, StorageUsage } from './types'
