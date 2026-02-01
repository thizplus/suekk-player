import { apiClient } from '@/lib/api-client'
import { QUEUE_ROUTES } from '@/constants/api-routes'
import type {
  QueueStatsResponse,
  TranscodeQueueItem,
  SubtitleQueueItem,
  WarmCacheQueueItem,
  RetryResponse,
  WarmCacheResponse,
  WarmAllResponse,
  PaginatedResponse,
} from './types'

interface ListParams {
  page?: number
  limit?: number
}

export const queueService = {
  // === Stats ===
  async getStats(): Promise<QueueStatsResponse> {
    const res = await apiClient.get<{ data: QueueStatsResponse }>(QUEUE_ROUTES.STATS)
    return res.data.data
  },

  // === Transcode Queue ===
  async getTranscodeFailed(params?: ListParams): Promise<PaginatedResponse<TranscodeQueueItem>> {
    const res = await apiClient.get<PaginatedResponse<TranscodeQueueItem>>(
      QUEUE_ROUTES.TRANSCODE_FAILED,
      { params }
    )
    return res.data
  },

  async retryTranscodeAll(): Promise<RetryResponse> {
    const res = await apiClient.post<{ data: RetryResponse }>(QUEUE_ROUTES.TRANSCODE_RETRY_ALL)
    return res.data.data
  },

  async retryTranscodeOne(id: string): Promise<void> {
    await apiClient.post(QUEUE_ROUTES.TRANSCODE_RETRY_ONE(id))
  },

  // === Subtitle Queue ===
  async getSubtitleStuck(params?: ListParams): Promise<PaginatedResponse<SubtitleQueueItem>> {
    const res = await apiClient.get<PaginatedResponse<SubtitleQueueItem>>(
      QUEUE_ROUTES.SUBTITLE_STUCK,
      { params }
    )
    return res.data
  },

  async getSubtitleFailed(params?: ListParams): Promise<PaginatedResponse<SubtitleQueueItem>> {
    const res = await apiClient.get<PaginatedResponse<SubtitleQueueItem>>(
      QUEUE_ROUTES.SUBTITLE_FAILED,
      { params }
    )
    return res.data
  },

  async retrySubtitleAll(): Promise<RetryResponse> {
    const res = await apiClient.post<{ data: RetryResponse }>(QUEUE_ROUTES.SUBTITLE_RETRY_ALL)
    return res.data.data
  },

  // === Warm Cache Queue ===
  async getWarmCachePending(params?: ListParams): Promise<PaginatedResponse<WarmCacheQueueItem>> {
    const res = await apiClient.get<PaginatedResponse<WarmCacheQueueItem>>(
      QUEUE_ROUTES.WARM_CACHE_PENDING,
      { params }
    )
    return res.data
  },

  async getWarmCacheFailed(params?: ListParams): Promise<PaginatedResponse<WarmCacheQueueItem>> {
    const res = await apiClient.get<PaginatedResponse<WarmCacheQueueItem>>(
      QUEUE_ROUTES.WARM_CACHE_FAILED,
      { params }
    )
    return res.data
  },

  async warmCacheOne(id: string): Promise<WarmCacheResponse> {
    const res = await apiClient.post<{ data: WarmCacheResponse }>(QUEUE_ROUTES.WARM_CACHE_ONE(id))
    return res.data.data
  },

  async warmCacheAll(): Promise<WarmAllResponse> {
    const res = await apiClient.post<{ data: WarmAllResponse }>(QUEUE_ROUTES.WARM_CACHE_ALL)
    return res.data.data
  },
}
