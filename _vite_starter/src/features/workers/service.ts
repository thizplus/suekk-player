import { apiClient } from '@/lib/api-client'
import { QUEUE_ROUTES } from '@/constants/api-routes'
import type { OnlineWorkersResponse } from './types'

export const workerService = {
  // ดึง online workers จาก NATS KV (Auto-Discovery)
  async getOnlineWorkers(): Promise<OnlineWorkersResponse> {
    return apiClient.get<OnlineWorkersResponse>(QUEUE_ROUTES.WORKERS_ONLINE)
  },
}
