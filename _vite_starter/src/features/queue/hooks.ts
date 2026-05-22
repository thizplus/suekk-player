import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { queueService } from './service'
import { toast } from 'sonner'
import type { WorkerJobType, WorkerJobStatus } from './types'

// Query Key Factory
export const queueKeys = {
  all: ['worker-jobs'] as const,
  stats: () => [...queueKeys.all, 'stats'] as const,
  statsAll: () => [...queueKeys.all, 'stats', 'all'] as const,
  list: (params?: {
    jobType?: WorkerJobType
    status?: WorkerJobStatus
    page?: number
    limit?: number
  }) => [...queueKeys.all, 'list', params] as const,
  detail: (id: string) => [...queueKeys.all, 'detail', id] as const,
  byEntity: (entityType: string, entityId: string) =>
    [...queueKeys.all, 'entity', entityType, entityId] as const,
}

// ==================== Stats ====================

export function useAllStats() {
  return useQuery({
    queryKey: queueKeys.statsAll(),
    queryFn: () => queueService.getAllStats(),
    refetchInterval: 10000, // Refresh every 10 seconds
  })
}

export function useStats(jobType?: WorkerJobType) {
  return useQuery({
    queryKey: [...queueKeys.stats(), jobType],
    queryFn: () => queueService.getStats(jobType),
    refetchInterval: 10000,
  })
}

// ==================== List Jobs ====================

export function useJobList(params: {
  jobType?: WorkerJobType
  status?: WorkerJobStatus
  page?: number
  limit?: number
}) {
  return useQuery({
    queryKey: queueKeys.list(params),
    queryFn: () =>
      queueService.listJobs({
        job_type: params.jobType,
        status: params.status,
        page: params.page,
        limit: params.limit,
      }),
    placeholderData: (prev) => prev,
  })
}

// Convenience hooks for common queries
export function useFailedJobs(jobType: WorkerJobType, page = 1, limit = 20) {
  return useJobList({ jobType, status: 'failed', page, limit })
}

export function useProcessingJobs(jobType: WorkerJobType, page = 1, limit = 20) {
  return useJobList({ jobType, status: 'processing', page, limit })
}

export function useQueuedJobs(jobType: WorkerJobType, page = 1, limit = 20) {
  return useJobList({ jobType, status: 'queued', page, limit })
}

// ==================== Job Detail ====================

export function useJobDetail(id: string, enabled = true) {
  return useQuery({
    queryKey: queueKeys.detail(id),
    queryFn: () => queueService.getJob(id),
    enabled,
  })
}

export function useJobsByEntity(entityType: string, entityId: string, enabled = true) {
  return useQuery({
    queryKey: queueKeys.byEntity(entityType, entityId),
    queryFn: () => queueService.getJobsByEntity(entityType, entityId),
    enabled,
  })
}

// ==================== Actions ====================

export function useRetryJob() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => queueService.retryJob(id),
    onSuccess: (data) => {
      toast.success(data.message)
      queryClient.invalidateQueries({ queryKey: queueKeys.all })
    },
    onError: () => {
      toast.error('Retry failed')
    },
  })
}

export function useCancelJob() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => queueService.cancelJob(id),
    onSuccess: (data) => {
      toast.success(data.message)
      queryClient.invalidateQueries({ queryKey: queueKeys.all })
    },
    onError: () => {
      toast.error('Cancel failed')
    },
  })
}

// ==================== Cleanup Actions ====================

export function useDeleteJob() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => queueService.deleteJob(id),
    onSuccess: (data) => {
      toast.success(data.message)
      queryClient.invalidateQueries({ queryKey: queueKeys.all })
    },
    onError: () => {
      toast.error('ลบ job ไม่สำเร็จ')
    },
  })
}

export function useDeleteOrphanedJobs() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => queueService.deleteOrphanedJobs(),
    onSuccess: (data) => {
      toast.success(`ลบ ${data.count} orphaned jobs สำเร็จ`)
      queryClient.invalidateQueries({ queryKey: queueKeys.all })
    },
    onError: () => {
      toast.error('ลบ orphaned jobs ไม่สำเร็จ')
    },
  })
}

export function useDeleteCompletedJobs() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (days = 7) => queueService.deleteCompletedJobs(days),
    onSuccess: (data) => {
      toast.success(`ลบ ${data.count} completed jobs (เก่ากว่า ${data.older_than_days} วัน) สำเร็จ`)
      queryClient.invalidateQueries({ queryKey: queueKeys.all })
    },
    onError: () => {
      toast.error('ลบ completed jobs ไม่สำเร็จ')
    },
  })
}

export function useDeleteFailedJobs() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => queueService.deleteFailedJobs(),
    onSuccess: (data) => {
      toast.success(`ลบ ${data.count} failed jobs สำเร็จ`)
      queryClient.invalidateQueries({ queryKey: queueKeys.all })
    },
    onError: () => {
      toast.error('ลบ failed jobs ไม่สำเร็จ')
    },
  })
}

// ==================== Legacy Subtitle Actions ====================

export function useRetrySubtitleAll() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => queueService.retrySubtitleAll(),
    onSuccess: (data) => {
      toast.success(data.message)
      queryClient.invalidateQueries({ queryKey: queueKeys.all })
    },
    onError: () => {
      toast.error('Retry failed')
    },
  })
}

export function useClearSubtitleAll() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => queueService.clearSubtitleAll(),
    onSuccess: (data) => {
      toast.success(data.message)
      queryClient.invalidateQueries({ queryKey: queueKeys.all })
    },
    onError: () => {
      toast.error('ลบไม่สำเร็จ')
    },
  })
}

export function useQueueMissingSubtitles() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => queueService.queueMissingSubtitles(),
    onSuccess: (data) => {
      toast.success(data.message)
      queryClient.invalidateQueries({ queryKey: queueKeys.all })
    },
    onError: () => {
      toast.error('Queue ไม่สำเร็จ')
    },
  })
}

// ==================== Legacy Stats (for WarmCache) ====================

export function useLegacyStats() {
  return useQuery({
    queryKey: [...queueKeys.all, 'legacy-stats'],
    queryFn: () => queueService.getLegacyStats(),
    refetchInterval: 10000,
  })
}

// ==================== Legacy Warm Cache (uses Video.CacheStatus) ====================

export function useWarmCachePending(page = 1, limit = 20) {
  return useQuery({
    queryKey: [...queueKeys.all, 'warmcache', 'pending', { page, limit }],
    queryFn: () => queueService.getWarmCachePending(page, limit),
  })
}

export function useWarmCacheFailed(page = 1, limit = 20) {
  return useQuery({
    queryKey: [...queueKeys.all, 'warmcache', 'failed', { page, limit }],
    queryFn: () => queueService.getWarmCacheFailed(page, limit),
  })
}

export function useWarmCacheOne() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => queueService.warmCacheOne(id),
    onSuccess: (data) => {
      toast.success(data.message)
      queryClient.invalidateQueries({ queryKey: queueKeys.all })
    },
    onError: () => {
      toast.error('Warm cache failed')
    },
  })
}

export function useWarmCacheAll() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => queueService.warmCacheAll(),
    onSuccess: (data) => {
      toast.success(data.message)
      queryClient.invalidateQueries({ queryKey: queueKeys.all })
    },
    onError: () => {
      toast.error('Warm cache failed')
    },
  })
}

// ==================== Stream Management ====================

export function usePurgeTranscodeStream() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => queueService.purgeTranscodeStream(),
    onSuccess: (data) => {
      toast.success(data.message)
      queryClient.invalidateQueries({ queryKey: queueKeys.all })
    },
    onError: () => {
      toast.error('Purge transcode stream failed')
    },
  })
}
