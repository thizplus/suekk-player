import { apiClient, type PaginationMeta } from '@/lib/api-client'
import { QUEUE_ROUTES } from '@/constants/api-routes'
import type {
  QueueStatsResponse,
  TranscodeQueueItem,
  SubtitleQueueItem,
  WarmCacheQueueItem,
  RetryResponse,
  ClearResponse,
  QueueMissingResponse,
  WarmCacheResponse,
  WarmAllResponse,
} from './types'

interface ListParams {
  page?: number
  limit?: number
}

interface PaginatedResult<T> {
  data: T[]
  meta: PaginationMeta
}

export const queueService = {
  // === Stats ===
  async getStats(): Promise<QueueStatsResponse> {
    return apiClient.get<QueueStatsResponse>(QUEUE_ROUTES.STATS)
  },

  // === Transcode Queue ===
  async getTranscodeFailed(params?: ListParams): Promise<PaginatedResult<TranscodeQueueItem>> {
    return apiClient.getPaginated<TranscodeQueueItem>(
      QUEUE_ROUTES.TRANSCODE_FAILED,
      { params }
    )
  },

  async retryTranscodeAll(): Promise<RetryResponse> {
    return apiClient.post<RetryResponse>(QUEUE_ROUTES.TRANSCODE_RETRY_ALL)
  },

  async retryTranscodeOne(id: string): Promise<void> {
    await apiClient.postVoid(QUEUE_ROUTES.TRANSCODE_RETRY_ONE(id))
  },

  // === Subtitle Queue ===
  async getSubtitleStuck(params?: ListParams): Promise<PaginatedResult<SubtitleQueueItem>> {
    return apiClient.getPaginated<SubtitleQueueItem>(
      QUEUE_ROUTES.SUBTITLE_STUCK,
      { params }
    )
  },

  async getSubtitleFailed(params?: ListParams): Promise<PaginatedResult<SubtitleQueueItem>> {
    return apiClient.getPaginated<SubtitleQueueItem>(
      QUEUE_ROUTES.SUBTITLE_FAILED,
      { params }
    )
  },

  async retrySubtitleAll(): Promise<RetryResponse> {
    return apiClient.post<RetryResponse>(QUEUE_ROUTES.SUBTITLE_RETRY_ALL)
  },

  async clearSubtitleAll(): Promise<ClearResponse> {
    return apiClient.deleteWithResponse<ClearResponse>(QUEUE_ROUTES.SUBTITLE_CLEAR_ALL)
  },

  async queueMissingSubtitles(): Promise<QueueMissingResponse> {
    return apiClient.post<QueueMissingResponse>(QUEUE_ROUTES.SUBTITLE_QUEUE_MISSING)
  },

  // === Warm Cache Queue ===
  async getWarmCachePending(params?: ListParams): Promise<PaginatedResult<WarmCacheQueueItem>> {
    return apiClient.getPaginated<WarmCacheQueueItem>(
      QUEUE_ROUTES.WARM_CACHE_PENDING,
      { params }
    )
  },

  async getWarmCacheFailed(params?: ListParams): Promise<PaginatedResult<WarmCacheQueueItem>> {
    return apiClient.getPaginated<WarmCacheQueueItem>(
      QUEUE_ROUTES.WARM_CACHE_FAILED,
      { params }
    )
  },

  async warmCacheOne(id: string): Promise<WarmCacheResponse> {
    return apiClient.post<WarmCacheResponse>(QUEUE_ROUTES.WARM_CACHE_ONE(id))
  },

  async warmCacheAll(): Promise<WarmAllResponse> {
    return apiClient.post<WarmAllResponse>(QUEUE_ROUTES.WARM_CACHE_ALL)
  },
}
