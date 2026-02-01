import { apiClient } from '@/lib/api-client'
import { SUBTITLE_ROUTES } from '@/constants/api-routes'
import type {
  SubtitlesResponse,
  SupportedLanguagesResponse,
  DetectLanguageResponse,
  TranscribeResponse,
  TranslateJobResponse,
  TranslateRequest,
  RetryStuckResponse,
} from './types'

export const subtitleService = {
  // ดึงรายการภาษาที่รองรับ
  async getSupportedLanguages(): Promise<SupportedLanguagesResponse> {
    return apiClient.get<SupportedLanguagesResponse>(SUBTITLE_ROUTES.LANGUAGES)
  },

  // ดึง subtitles ของ video (protected - ต้อง login)
  async getByVideo(videoId: string): Promise<SubtitlesResponse> {
    return apiClient.get<SubtitlesResponse>(SUBTITLE_ROUTES.BY_VIDEO(videoId))
  },

  // ดึง subtitles โดยใช้ video code (public - สำหรับ embed)
  async getByCode(code: string): Promise<SubtitlesResponse> {
    return apiClient.get<SubtitlesResponse>(SUBTITLE_ROUTES.BY_CODE(code))
  },

  // Trigger detect language
  async detectLanguage(videoId: string): Promise<DetectLanguageResponse> {
    return apiClient.post<DetectLanguageResponse>(SUBTITLE_ROUTES.DETECT(videoId))
  },

  // Trigger transcribe (สร้าง SRT)
  async transcribe(videoId: string): Promise<TranscribeResponse> {
    return apiClient.post<TranscribeResponse>(SUBTITLE_ROUTES.TRANSCRIBE(videoId))
  },

  // Trigger translate
  async translate(videoId: string, targetLanguages: string[]): Promise<TranslateJobResponse> {
    const payload: TranslateRequest = { targetLanguages }
    return apiClient.post<TranslateJobResponse>(SUBTITLE_ROUTES.TRANSLATE(videoId), payload)
  },

  // Delete subtitle
  async deleteSubtitle(subtitleId: string): Promise<void> {
    return apiClient.delete(SUBTITLE_ROUTES.DELETE(subtitleId))
  },

  // Retry stuck subtitles ทั้งหมด (status = queued ที่ค้างอยู่)
  async retryStuckSubtitles(): Promise<RetryStuckResponse> {
    return apiClient.post<RetryStuckResponse>(SUBTITLE_ROUTES.RETRY_STUCK)
  },
}
