import { apiClient } from '@/lib/api-client'
import { SUBTITLE_ROUTES } from '@/constants/api-routes'
import type {
  SubtitlesResponse,
  SupportedLanguagesResponse,
  DetectLanguageResponse,
  SetLanguageRequest,
  SetLanguageResponse,
  TranscribeResponse,
  TranslateJobResponse,
  TranslateRequest,
  RetryStuckResponse,
  SubtitleContentResponse,
  UpdateSubtitleContentRequest,
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

  // Set language manually (override auto-detect)
  async setLanguage(videoId: string, language: string): Promise<SetLanguageResponse> {
    const payload: SetLanguageRequest = { language }
    return apiClient.post<SetLanguageResponse>(SUBTITLE_ROUTES.SET_LANGUAGE(videoId), payload)
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

  // === Content Editing ===

  // ดึง content ของ subtitle (SRT file)
  async getContent(subtitleId: string): Promise<SubtitleContentResponse> {
    return apiClient.get<SubtitleContentResponse>(SUBTITLE_ROUTES.CONTENT(subtitleId))
  },

  // อัปเดต content ของ subtitle (SRT file)
  async updateContent(subtitleId: string, content: string): Promise<void> {
    const payload: UpdateSubtitleContentRequest = { content }
    return apiClient.put(SUBTITLE_ROUTES.CONTENT(subtitleId), payload)
  },
}
