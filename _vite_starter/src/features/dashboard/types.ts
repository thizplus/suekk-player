export interface DashboardStats {
  totalUsers: number
  activeUsers: number
  totalSales: number
  pendingOrders: number
}

export interface DashboardData {
  stats: DashboardStats
  recentActivity: ActivityItem[]
}

export interface ActivityItem {
  id: string
  type: 'user' | 'order' | 'sale'
  message: string
  timestamp: string
}

// Storage usage (from /api/v1/storage/usage)
export interface StorageUsage {
  used: number        // bytes ที่ใช้ไป
  usedHuman: string   // เช่น "1.15 GB"
  quota: number       // bytes quota ทั้งหมด
  quotaHuman: string  // เช่น "5.00 TB"
  percent: number     // เปอร์เซ็นต์ที่ใช้ (0-100)
  unlimited: boolean  // ถ้า true = ไม่จำกัด
}
