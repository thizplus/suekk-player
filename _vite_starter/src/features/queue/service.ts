import { apiClient, type PaginationMeta } from '@/lib/api-client'
import { WORKER_JOB_ROUTES, QUEUE_ROUTES } from '@/constants/api-routes'
import type {
  WorkerJob,
  WorkerJobDetail,
  WorkerJobStats,
  WorkerJobAllStats,
  WorkerJobType,
  WorkerJobStatus,
  WorkerJobListResponse,
  WarmCacheQueueItem,
  LegacyQueueStats,
  RetryResponse,
  ClearResponse,
  QueueMissingResponse,
  WarmCacheResponse,
  WarmAllResponse,
} from './types'

interface ListParams {
  page?: number
  limit?: number
  job_type?: WorkerJobType
  status?: WorkerJobStatus
  entity_type?: string
  entity_id?: string
}

interface PaginatedResult<T> {
  data: T[]
  meta: PaginationMeta
}

export const queueService = {
  // ==================== Stats (New API) ====================

  async getStats(jobType?: WorkerJobType): Promise<WorkerJobStats> {
    const params = jobType ? { job_type: jobType } : undefined
    return apiClient.get<WorkerJobStats>(WORKER_JOB_ROUTES.STATS, { params })
  },

  async getAllStats(): Promise<WorkerJobAllStats> {
    return apiClient.get<WorkerJobAllStats>(WORKER_JOB_ROUTES.STATS_ALL)
  },

  // ==================== List Jobs (New API) ====================

  async listJobs(params?: ListParams): Promise<WorkerJobListResponse> {
    return apiClient.get<WorkerJobListResponse>(WORKER_JOB_ROUTES.LIST, { params })
  },

  async getJob(id: string): Promise<WorkerJobDetail> {
    return apiClient.get<WorkerJobDetail>(WORKER_JOB_ROUTES.BY_ID(id))
  },

  async getJobsByEntity(entityType: string, entityId: string): Promise<WorkerJob[]> {
    return apiClient.get<WorkerJob[]>(WORKER_JOB_ROUTES.BY_ENTITY(entityType, entityId))
  },

  // ==================== Job Actions (New API) ====================

  async retryJob(id: string): Promise<{ message: string }> {
    return apiClient.post<{ message: string }>(WORKER_JOB_ROUTES.RETRY(id))
  },

  async cancelJob(id: string): Promise<{ message: string }> {
    return apiClient.post<{ message: string }>(WORKER_JOB_ROUTES.CANCEL(id))
  },

  // ==================== Cleanup Actions ====================

  async deleteJob(id: string): Promise<{ message: string }> {
    return apiClient.deleteWithResponse<{ message: string }>(WORKER_JOB_ROUTES.DELETE(id))
  },

  async deleteOrphanedJobs(): Promise<{ message: string; count: number }> {
    return apiClient.deleteWithResponse<{ message: string; count: number }>(
      WORKER_JOB_ROUTES.DELETE_ORPHANED
    )
  },

  async deleteCompletedJobs(days = 7): Promise<{ message: string; count: number; older_than_days: number }> {
    return apiClient.deleteWithResponse<{ message: string; count: number; older_than_days: number }>(
      `${WORKER_JOB_ROUTES.DELETE_COMPLETED}?days=${days}`
    )
  },

  async deleteFailedJobs(): Promise<{ message: string; count: number }> {
    return apiClient.deleteWithResponse<{ message: string; count: number }>(WORKER_JOB_ROUTES.DELETE_FAILED)
  },

  // ==================== Convenience Methods ====================

  // Get failed jobs by type
  async getFailedJobs(
    jobType: WorkerJobType,
    page = 1,
    limit = 20
  ): Promise<WorkerJobListResponse> {
    return this.listJobs({
      job_type: jobType,
      status: 'failed',
      page,
      limit,
    })
  },

  // Get processing jobs by type
  async getProcessingJobs(
    jobType: WorkerJobType,
    page = 1,
    limit = 20
  ): Promise<WorkerJobListResponse> {
    return this.listJobs({
      job_type: jobType,
      status: 'processing',
      page,
      limit,
    })
  },

  // Get queued jobs by type
  async getQueuedJobs(
    jobType: WorkerJobType,
    page = 1,
    limit = 20
  ): Promise<WorkerJobListResponse> {
    return this.listJobs({
      job_type: jobType,
      status: 'queued',
      page,
      limit,
    })
  },

  // ==================== Legacy Subtitle Actions ====================

  async retrySubtitleAll(): Promise<RetryResponse> {
    return apiClient.post<RetryResponse>(QUEUE_ROUTES.SUBTITLE_RETRY_ALL)
  },

  async clearSubtitleAll(): Promise<ClearResponse> {
    return apiClient.deleteWithResponse<ClearResponse>(QUEUE_ROUTES.SUBTITLE_CLEAR_ALL)
  },

  async queueMissingSubtitles(): Promise<QueueMissingResponse> {
    return apiClient.post<QueueMissingResponse>(QUEUE_ROUTES.SUBTITLE_QUEUE_MISSING)
  },

  // ==================== Legacy Stats (for WarmCache) ====================

  async getLegacyStats(): Promise<LegacyQueueStats> {
    return apiClient.get<LegacyQueueStats>(QUEUE_ROUTES.STATS)
  },

  // ==================== Legacy Warm Cache (uses Video.CacheStatus) ====================

  async getWarmCachePending(
    page = 1,
    limit = 20
  ): Promise<PaginatedResult<WarmCacheQueueItem>> {
    return apiClient.getPaginated<WarmCacheQueueItem>(QUEUE_ROUTES.WARM_CACHE_PENDING, {
      params: { page, limit },
    })
  },

  async getWarmCacheFailed(
    page = 1,
    limit = 20
  ): Promise<PaginatedResult<WarmCacheQueueItem>> {
    return apiClient.getPaginated<WarmCacheQueueItem>(QUEUE_ROUTES.WARM_CACHE_FAILED, {
      params: { page, limit },
    })
  },

  async warmCacheOne(id: string): Promise<WarmCacheResponse> {
    return apiClient.post<WarmCacheResponse>(QUEUE_ROUTES.WARM_CACHE_ONE(id))
  },

  async warmCacheAll(): Promise<WarmAllResponse> {
    return apiClient.post<WarmAllResponse>(QUEUE_ROUTES.WARM_CACHE_ALL)
  },

  // ==================== Stream Management ====================

  async retryAllTranscode(): Promise<RetryResponse> {
    return apiClient.post<RetryResponse>(QUEUE_ROUTES.TRANSCODE_RETRY_ALL)
  },

  async purgeTranscodeStream(): Promise<{ message: string }> {
    return apiClient.deleteWithResponse<{ message: string }>(QUEUE_ROUTES.TRANSCODE_PURGE)
  },
}
