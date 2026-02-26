import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { queueService } from './service'
import { toast } from 'sonner'

// Query Key Factory
export const queueKeys = {
  all: ['queue'] as const,
  stats: () => [...queueKeys.all, 'stats'] as const,
  transcode: () => [...queueKeys.all, 'transcode'] as const,
  transcodeFailed: (page: number, limit: number) =>
    [...queueKeys.transcode(), 'failed', { page, limit }] as const,
  subtitle: () => [...queueKeys.all, 'subtitle'] as const,
  subtitleStuck: (page: number, limit: number) =>
    [...queueKeys.subtitle(), 'stuck', { page, limit }] as const,
  subtitleFailed: (page: number, limit: number) =>
    [...queueKeys.subtitle(), 'failed', { page, limit }] as const,
  warmCache: () => [...queueKeys.all, 'warmCache'] as const,
  warmCachePending: (page: number, limit: number) =>
    [...queueKeys.warmCache(), 'pending', { page, limit }] as const,
  warmCacheFailed: (page: number, limit: number) =>
    [...queueKeys.warmCache(), 'failed', { page, limit }] as const,
  gallery: () => [...queueKeys.all, 'gallery'] as const,
  galleryProcessing: (page: number, limit: number) =>
    [...queueKeys.gallery(), 'processing', { page, limit }] as const,
  galleryFailed: (page: number, limit: number) =>
    [...queueKeys.gallery(), 'failed', { page, limit }] as const,
  reel: () => [...queueKeys.all, 'reel'] as const,
  reelExporting: (page: number, limit: number) =>
    [...queueKeys.reel(), 'exporting', { page, limit }] as const,
  reelFailed: (page: number, limit: number) =>
    [...queueKeys.reel(), 'failed', { page, limit }] as const,
}

// ==================== Stats ====================

export function useQueueStats() {
  return useQuery({
    queryKey: queueKeys.stats(),
    queryFn: () => queueService.getStats(),
    refetchInterval: 10000, // Refresh every 10 seconds
  })
}

// ==================== Transcode Queue ====================

export function useTranscodeFailed(page = 1, limit = 20) {
  return useQuery({
    queryKey: queueKeys.transcodeFailed(page, limit),
    queryFn: () => queueService.getTranscodeFailed({ page, limit }),
  })
}

export function useRetryTranscodeAll() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => queueService.retryTranscodeAll(),
    onSuccess: (data) => {
      toast.success(data.message)
      queryClient.invalidateQueries({ queryKey: queueKeys.all })
    },
    onError: () => {
      toast.error('Retry failed')
    },
  })
}

export function useRetryTranscodeOne() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => queueService.retryTranscodeOne(id),
    onSuccess: () => {
      toast.success('Retry queued')
      queryClient.invalidateQueries({ queryKey: queueKeys.all })
    },
    onError: () => {
      toast.error('Retry failed')
    },
  })
}

// ==================== Subtitle Queue ====================

export function useSubtitleStuck(page = 1, limit = 20) {
  return useQuery({
    queryKey: queueKeys.subtitleStuck(page, limit),
    queryFn: () => queueService.getSubtitleStuck({ page, limit }),
  })
}

export function useSubtitleFailed(page = 1, limit = 20) {
  return useQuery({
    queryKey: queueKeys.subtitleFailed(page, limit),
    queryFn: () => queueService.getSubtitleFailed({ page, limit }),
  })
}

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

// ==================== Warm Cache Queue ====================

export function useWarmCachePending(page = 1, limit = 20) {
  return useQuery({
    queryKey: queueKeys.warmCachePending(page, limit),
    queryFn: () => queueService.getWarmCachePending({ page, limit }),
  })
}

export function useWarmCacheFailed(page = 1, limit = 20) {
  return useQuery({
    queryKey: queueKeys.warmCacheFailed(page, limit),
    queryFn: () => queueService.getWarmCacheFailed({ page, limit }),
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

// ==================== Gallery Queue ====================

export function useGalleryProcessing(page = 1, limit = 20) {
  return useQuery({
    queryKey: queueKeys.galleryProcessing(page, limit),
    queryFn: () => queueService.getGalleryProcessing({ page, limit }),
  })
}

export function useGalleryFailed(page = 1, limit = 20) {
  return useQuery({
    queryKey: queueKeys.galleryFailed(page, limit),
    queryFn: () => queueService.getGalleryFailed({ page, limit }),
  })
}

export function useRetryGalleryAll() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => queueService.retryGalleryAll(),
    onSuccess: (data) => {
      toast.success(data.message)
      queryClient.invalidateQueries({ queryKey: queueKeys.all })
    },
    onError: () => {
      toast.error('Retry gallery failed')
    },
  })
}

// ==================== Reel Queue ====================

export function useReelExporting(page = 1, limit = 20) {
  return useQuery({
    queryKey: queueKeys.reelExporting(page, limit),
    queryFn: () => queueService.getReelExporting({ page, limit }),
  })
}

export function useReelFailed(page = 1, limit = 20) {
  return useQuery({
    queryKey: queueKeys.reelFailed(page, limit),
    queryFn: () => queueService.getReelFailed({ page, limit }),
  })
}

export function useRetryReelAll() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => queueService.retryReelAll(),
    onSuccess: (data) => {
      toast.success(data.message)
      queryClient.invalidateQueries({ queryKey: queueKeys.all })
    },
    onError: () => {
      toast.error('Retry reel failed')
    },
  })
}
