import { apiClient } from '@/lib/api-client'
import { TRANSCODING_ROUTES } from '@/constants/api-routes'
import type { OnlineWorkersResponse } from './types'

export const workerService = {
  // ดึง online workers จาก NATS KV (Auto-Discovery)
  async getOnlineWorkers(): Promise<OnlineWorkersResponse> {
    return apiClient.get<OnlineWorkersResponse>(TRANSCODING_ROUTES.WORKERS)
  },
}
