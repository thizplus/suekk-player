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
  content: (subtitleId: string) => [...subtitleKeys.all, 'content', subtitleId] as const,
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

// Set language manually
export function useSetLanguage() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ videoId, language }: { videoId: string; language: string }) =>
      subtitleService.setLanguage(videoId, language),
    onSuccess: async (data, { videoId }) => {
      toast.success(`ตั้งค่าภาษาเป็น ${data.language} แล้ว`)
      await invalidateSubtitleAndVideo(queryClient, videoId)
    },
    onError: (error: Error) => {
      toast.error(error.message || 'ไม่สามารถตั้งค่าภาษาได้')
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

// Retry stuck subtitles ทั้งหมด (admin action)
export function useRetryStuckSubtitles() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: () => subtitleService.retryStuckSubtitles(),
    onSuccess: async (data) => {
      if (data.totalRetried > 0) {
        toast.success(data.message)
      } else if (data.totalFound === 0) {
        toast.info('ไม่พบ Subtitle ที่ค้างอยู่')
      } else {
        toast.warning(`ไม่สามารถ retry ได้ (${data.skipped} skipped)`)
      }
      // Invalidate all subtitle and video queries
      await queryClient.invalidateQueries({ queryKey: subtitleKeys.all })
      await queryClient.invalidateQueries({ queryKey: videoKeys.all })
    },
    onError: (error: Error) => {
      toast.error(error.message || 'ไม่สามารถ retry stuck subtitles ได้')
    },
  })
}

// === Content Editing Hooks ===

// ดึง content ของ subtitle (SRT file)
export function useSubtitleContent(subtitleId: string, options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: subtitleKeys.content(subtitleId),
    queryFn: () => subtitleService.getContent(subtitleId),
    enabled: options?.enabled ?? !!subtitleId,
  })
}

// อัปเดต content ของ subtitle (SRT file)
export function useUpdateSubtitleContent() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ subtitleId, content }: { subtitleId: string; content: string }) =>
      subtitleService.updateContent(subtitleId, content),
    onSuccess: async (_data, { subtitleId }) => {
      // Invalidate content cache
      await queryClient.invalidateQueries({ queryKey: subtitleKeys.content(subtitleId) })
      toast.success('บันทึก Subtitle สำเร็จ')
    },
    onError: (error: Error) => {
      toast.error(error.message || 'ไม่สามารถบันทึก Subtitle ได้')
    },
  })
}
