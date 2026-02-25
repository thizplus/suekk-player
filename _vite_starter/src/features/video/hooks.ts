import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { videoService } from './service'
import type { VideoFilterParams, UpdateVideoRequest } from './types'

// Query Key Factory
export const videoKeys = {
  all: ['videos'] as const,
  lists: () => [...videoKeys.all, 'list'] as const,
  list: (params?: VideoFilterParams) => [...videoKeys.lists(), params] as const,
  details: () => [...videoKeys.all, 'detail'] as const,
  detail: (id: string) => [...videoKeys.details(), id] as const,
  byCode: (code: string) => [...videoKeys.all, 'code', code] as const,
  transcoding: () => [...videoKeys.all, 'transcoding'] as const,
  transcodingStatus: (videoId: string) => [...videoKeys.transcoding(), 'status', videoId] as const,
  transcodingStats: () => [...videoKeys.transcoding(), 'stats'] as const,
  workers: () => [...videoKeys.all, 'workers'] as const,
  dlq: () => [...videoKeys.all, 'dlq'] as const,
  dlqList: (params?: { page?: number; limit?: number }) => [...videoKeys.dlq(), 'list', params] as const,
}

// ดึงรายการวิดีโอ
export function useVideos(params?: VideoFilterParams, options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: videoKeys.list(params),
    queryFn: () => videoService.getList(params),
    enabled: options?.enabled ?? true,
  })
}

// ดึงวิดีโอตาม ID
export function useVideo(id: string) {
  return useQuery({
    queryKey: videoKeys.detail(id),
    queryFn: () => videoService.getById(id),
    enabled: !!id,
  })
}

// ดึงวิดีโอตาม Code
export function useVideoByCode(code: string) {
  return useQuery({
    queryKey: videoKeys.byCode(code),
    queryFn: () => videoService.getByCode(code),
    enabled: !!code,
  })
}

// อัปโหลดวิดีโอ
export function useUploadVideo() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ file, title, description, categoryId }: {
      file: File
      title: string
      description?: string
      categoryId?: string
    }) => videoService.upload(file, title, description, categoryId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: videoKeys.lists() })
    },
  })
}

// อัปเดตวิดีโอ
export function useUpdateVideo() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateVideoRequest }) =>
      videoService.update(id, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: videoKeys.detail(variables.id) })
      queryClient.invalidateQueries({ queryKey: videoKeys.lists() })
    },
  })
}

// ลบวิดีโอ (optimistic update - ลบออกจาก UI ทันที)
export function useDeleteVideo() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => videoService.delete(id),
    onMutate: async (deletedId) => {
      // Cancel any outgoing refetches
      await queryClient.cancelQueries({ queryKey: videoKeys.lists() })

      // Snapshot previous value
      const previousVideos = queryClient.getQueriesData({ queryKey: videoKeys.lists() })

      // Optimistically remove from all video lists
      queryClient.setQueriesData({ queryKey: videoKeys.lists() }, (old: unknown) => {
        if (!old || typeof old !== 'object') return old
        const data = old as { data: Array<{ id: string }>; meta: unknown }
        if (!data.data) return old
        return {
          ...data,
          data: data.data.filter((v) => v.id !== deletedId),
          meta: data.meta,
        }
      })

      return { previousVideos }
    },
    onError: (_err, _id, context) => {
      // Rollback on error
      if (context?.previousVideos) {
        context.previousVideos.forEach(([queryKey, data]) => {
          queryClient.setQueryData(queryKey, data)
        })
      }
    },
    onSettled: () => {
      // Refetch to ensure consistency
      queryClient.invalidateQueries({ queryKey: videoKeys.lists() })
    },
  })
}

// เพิ่มวิดีโอเข้าคิว transcoding
export function useQueueTranscoding() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (videoId: string) => videoService.queueTranscoding(videoId),
    onSuccess: (_, videoId) => {
      queryClient.invalidateQueries({ queryKey: videoKeys.detail(videoId) })
      queryClient.invalidateQueries({ queryKey: videoKeys.lists() })
      queryClient.invalidateQueries({ queryKey: videoKeys.transcodingStats() })
    },
  })
}

// ดึงสถานะ transcoding
export function useTranscodingStatus(videoId: string) {
  return useQuery({
    queryKey: videoKeys.transcodingStatus(videoId),
    queryFn: () => videoService.getTranscodingStatus(videoId),
    enabled: !!videoId,
    refetchInterval: 3000, // Refresh every 3 seconds
  })
}

// ดึงสถิติ transcoding
export function useTranscodingStats() {
  return useQuery({
    queryKey: videoKeys.transcodingStats(),
    queryFn: () => videoService.getTranscodingStats(),
    refetchInterval: 5000, // Refresh every 5 seconds
  })
}

// ดึงรายการ Workers
export function useWorkers() {
  return useQuery({
    queryKey: videoKeys.workers(),
    queryFn: () => videoService.getWorkers(),
    refetchInterval: 5000, // Refresh every 5 seconds
  })
}

// === Dead Letter Queue (DLQ) ===

// ดึงรายการ videos ใน DLQ
export function useDLQVideos(params?: { page?: number; limit?: number }) {
  return useQuery({
    queryKey: videoKeys.dlqList(params),
    queryFn: () => videoService.getDLQList(params),
    refetchInterval: 10000, // Refresh every 10 seconds
  })
}

// Retry video จาก DLQ
export function useRetryDLQ() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (videoId: string) => videoService.retryDLQ(videoId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: videoKeys.dlq() })
      queryClient.invalidateQueries({ queryKey: videoKeys.lists() })
    },
  })
}

// ลบ video จาก DLQ
export function useDeleteDLQ() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (videoId: string) => videoService.deleteDLQ(videoId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: videoKeys.dlq() })
      queryClient.invalidateQueries({ queryKey: videoKeys.lists() })
    },
  })
}

// === Config ===

export const configKeys = {
  all: ['config'] as const,
  uploadLimits: () => [...configKeys.all, 'upload-limits'] as const,
}

// ดึง upload limits จาก settings
export function useUploadLimits() {
  return useQuery({
    queryKey: configKeys.uploadLimits(),
    queryFn: () => videoService.getUploadLimits(),
    staleTime: 5 * 60 * 1000, // Cache 5 นาที
    gcTime: 10 * 60 * 1000,   // Keep in cache 10 นาที
  })
}

// === Gallery ===

// สร้าง gallery จาก HLS
export function useGenerateGallery() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (videoId: string) => videoService.generateGallery(videoId),
    onSuccess: (_, videoId) => {
      queryClient.invalidateQueries({ queryKey: videoKeys.detail(videoId) })
      queryClient.invalidateQueries({ queryKey: videoKeys.lists() })
    },
  })
}

// สร้าง gallery ใหม่ (ลบของเก่าแล้วสร้างใหม่)
export function useRegenerateGallery() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (videoId: string) => videoService.regenerateGallery(videoId),
    onSuccess: (_, videoId) => {
      queryClient.invalidateQueries({ queryKey: videoKeys.detail(videoId) })
      queryClient.invalidateQueries({ queryKey: videoKeys.lists() })
    },
  })
}

// ดึง presigned URLs สำหรับ gallery images
export function useGalleryUrls(videoCode: string, options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: [...videoKeys.byCode(videoCode), 'gallery-urls'],
    queryFn: () => videoService.getGalleryUrls(videoCode),
    enabled: options?.enabled ?? !!videoCode,
    staleTime: 30 * 60 * 1000, // Cache 30 นาที (URLs หมดอายุใน 1 ชม.)
    gcTime: 60 * 60 * 1000,    // Keep in cache 1 ชม.
  })
}

// ═══════════════════════════════════════════════════════════════════════════════
// Gallery Admin - Manual Selection Flow
// ═══════════════════════════════════════════════════════════════════════════════

export const galleryAdminKeys = {
  all: ['gallery-admin'] as const,
  images: (videoId: string) => [...galleryAdminKeys.all, 'images', videoId] as const,
}

// ดึงภาพทั้งหมดใน gallery (source, safe, nsfw)
export function useGalleryImages(videoId: string, options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: galleryAdminKeys.images(videoId),
    queryFn: () => videoService.getGalleryImages(videoId),
    enabled: options?.enabled ?? !!videoId,
    staleTime: 0, // Don't cache - ต้องการ fresh data หลังย้ายภาพ
  })
}

// ย้ายภาพเดี่ยว
export function useMoveImage() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ videoId, data }: { videoId: string; data: { filename: string; from: string; to: string } }) =>
      videoService.moveImage(videoId, data as import('./types').MoveImageRequest),
    onSuccess: (_, { videoId }) => {
      queryClient.invalidateQueries({ queryKey: galleryAdminKeys.images(videoId) })
    },
  })
}

// ย้ายหลายภาพ (batch)
export function useMoveBatch() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ videoId, data }: { videoId: string; data: { files: string[]; from: string; to: string } }) =>
      videoService.moveBatch(videoId, data as import('./types').MoveBatchRequest),
    onSuccess: (_, { videoId }) => {
      queryClient.invalidateQueries({ queryKey: galleryAdminKeys.images(videoId) })
    },
  })
}

// Publish gallery
export function usePublishGallery() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (videoId: string) => videoService.publishGallery(videoId),
    onSuccess: (_, videoId) => {
      queryClient.invalidateQueries({ queryKey: galleryAdminKeys.images(videoId) })
      queryClient.invalidateQueries({ queryKey: videoKeys.detail(videoId) })
      queryClient.invalidateQueries({ queryKey: videoKeys.lists() })
    },
  })
}
