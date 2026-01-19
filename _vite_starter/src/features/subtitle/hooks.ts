import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { subtitleService } from './service'
import { toast } from 'sonner'
import type { SubtitlesResponse } from './types'
import { videoKeys } from '@/features/video/hooks'

// Query key factory
export const subtitleKeys = {
  all: ['subtitle'] as const,
  languages: () => [...subtitleKeys.all, 'languages'] as const,
  byVideo: (videoId: string) => [...subtitleKeys.all, 'video', videoId] as const,
  byCode: (code: string) => [...subtitleKeys.all, 'code', code] as const,
}

// ดึงรายการภาษาที่รองรับ
export function useSupportedLanguages() {
  return useQuery({
    queryKey: subtitleKeys.languages(),
    queryFn: () => subtitleService.getSupportedLanguages(),
    staleTime: 1000 * 60 * 60, // Cache 1 hour
  })
}

// ดึง subtitles ของ video (protected - ต้อง login)
export function useVideoSubtitles(videoId: string, options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: subtitleKeys.byVideo(videoId),
    queryFn: () => subtitleService.getByVideo(videoId),
    enabled: options?.enabled ?? !!videoId,
  })
}

// ดึง subtitles โดยใช้ video code (public - สำหรับ embed)
export function useSubtitlesByCode(code: string, options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: subtitleKeys.byCode(code),
    queryFn: () => subtitleService.getByCode(code),
    enabled: options?.enabled ?? !!code,
  })
}

// Helper: invalidate subtitle และ video queries
async function invalidateSubtitleAndVideo(
  queryClient: ReturnType<typeof useQueryClient>,
  videoId: string
) {
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: subtitleKeys.byVideo(videoId) }),
    queryClient.invalidateQueries({ queryKey: videoKeys.lists() }),
    queryClient.invalidateQueries({ queryKey: videoKeys.detail(videoId) }),
  ])
}

// Trigger detect language
export function useDetectLanguage() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (videoId: string) => subtitleService.detectLanguage(videoId),
    onSuccess: async (_data, videoId) => {
      toast.success('เริ่มตรวจจับภาษาแล้ว')
      await invalidateSubtitleAndVideo(queryClient, videoId)
    },
    onError: (error: Error) => {
      toast.error(error.message || 'ไม่สามารถเริ่มตรวจจับภาษาได้')
    },
  })
}

// Trigger transcribe
export function useTranscribe() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (videoId: string) => subtitleService.transcribe(videoId),
    onSuccess: async (_data, videoId) => {
      toast.success('เริ่มสร้าง Subtitle แล้ว')
      await invalidateSubtitleAndVideo(queryClient, videoId)
    },
    onError: (error: Error) => {
      toast.error(error.message || 'ไม่สามารถเริ่มสร้าง Subtitle ได้')
    },
  })
}

// Trigger translate
export function useTranslate() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ videoId, targetLanguages }: { videoId: string; targetLanguages: string[] }) =>
      subtitleService.translate(videoId, targetLanguages),
    onSuccess: async (data, { videoId }) => {
      toast.success(`เริ่มแปลเป็น ${data.targetLanguages.join(', ')} แล้ว`)
      await invalidateSubtitleAndVideo(queryClient, videoId)
    },
    onError: (error: Error) => {
      toast.error(error.message || 'ไม่สามารถเริ่มแปลได้')
    },
  })
}

// Delete subtitle
export function useDeleteSubtitle() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ subtitleId }: { subtitleId: string; videoId: string }) =>
      subtitleService.deleteSubtitle(subtitleId),
    onSuccess: async (_data, { subtitleId, videoId }) => {
      // อัพเดท subtitle cache โดยตรง (optimistic)
      const queryKey = subtitleKeys.byVideo(videoId)
      const currentData = queryClient.getQueryData<SubtitlesResponse>(queryKey)

      if (currentData?.subtitles) {
        queryClient.setQueryData<SubtitlesResponse>(queryKey, {
          ...currentData,
          subtitles: currentData.subtitles.filter((s) => s.id !== subtitleId),
        })
      }

      // Invalidate ทั้ง subtitle และ video list
      await invalidateSubtitleAndVideo(queryClient, videoId)
      toast.success('ลบ Subtitle แล้ว')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'ไม่สามารถลบ Subtitle ได้')
    },
  })
}
