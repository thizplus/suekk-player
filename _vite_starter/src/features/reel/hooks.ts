import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { reelService } from './service'
import type { CreateReelRequest, UpdateReelRequest, ReelFilterParams } from './types'

// Query key factory
export const reelKeys = {
  all: ['reels'] as const,
  lists: () => [...reelKeys.all, 'list'] as const,
  list: (params?: ReelFilterParams) => [...reelKeys.lists(), params] as const,
  details: () => [...reelKeys.all, 'detail'] as const,
  detail: (id: string) => [...reelKeys.details(), id] as const,
  byVideo: (videoId: string) => [...reelKeys.all, 'video', videoId] as const,
  templates: () => [...reelKeys.all, 'templates'] as const,
  template: (id: string) => [...reelKeys.templates(), id] as const,
}

// === Templates ===

export function useReelTemplates() {
  return useQuery({
    queryKey: reelKeys.templates(),
    queryFn: () => reelService.getTemplates(),
    staleTime: 5 * 60 * 1000, // 5 minutes
  })
}

export function useReelTemplate(id: string) {
  return useQuery({
    queryKey: reelKeys.template(id),
    queryFn: () => reelService.getTemplateById(id),
    enabled: !!id,
  })
}

// === Reels ===

export function useReels(params?: ReelFilterParams) {
  return useQuery({
    queryKey: reelKeys.list(params),
    queryFn: () => reelService.list(params),
  })
}

export function useReel(id: string) {
  return useQuery({
    queryKey: reelKeys.detail(id),
    queryFn: () => reelService.getById(id),
    enabled: !!id,
  })
}

export function useReelsByVideo(videoId: string, page = 1, limit = 20) {
  return useQuery({
    queryKey: [...reelKeys.byVideo(videoId), page, limit],
    queryFn: () => reelService.listByVideo(videoId, page, limit),
    enabled: !!videoId,
  })
}

// === Mutations ===

export function useCreateReel() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: CreateReelRequest) => reelService.create(data),
    onSuccess: (newReel) => {
      // Invalidate list queries
      queryClient.invalidateQueries({ queryKey: reelKeys.lists() })
      // Invalidate video reels
      if (newReel.video?.id) {
        queryClient.invalidateQueries({ queryKey: reelKeys.byVideo(newReel.video.id) })
      }
    },
  })
}

export function useUpdateReel() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateReelRequest }) =>
      reelService.update(id, data),
    onSuccess: (updatedReel) => {
      // Update cache
      queryClient.setQueryData(reelKeys.detail(updatedReel.id), updatedReel)
      // Invalidate lists
      queryClient.invalidateQueries({ queryKey: reelKeys.lists() })
    },
  })
}

export function useDeleteReel() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => reelService.delete(id),
    onSuccess: (_, id) => {
      // Remove from cache
      queryClient.removeQueries({ queryKey: reelKeys.detail(id) })
      // Invalidate lists
      queryClient.invalidateQueries({ queryKey: reelKeys.lists() })
    },
  })
}

export function useExportReel() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => reelService.export(id),
    onSuccess: (_, id) => {
      // Invalidate reel detail to get new status
      queryClient.invalidateQueries({ queryKey: reelKeys.detail(id) })
      // Invalidate lists
      queryClient.invalidateQueries({ queryKey: reelKeys.lists() })
    },
  })
}
