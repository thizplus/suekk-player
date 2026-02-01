// Subtitle Status - ตรงกับ backend models.SubtitleStatus
export type SubtitleStatus =
  | 'pending'      // รอ process
  | 'queued'       // อยู่ในคิว รอ worker
  | 'detecting'    // กำลัง detect language
  | 'detected'     // detect เสร็จ รอสร้าง SRT
  | 'processing'   // กำลังสร้าง SRT
  | 'ready'        // พร้อมใช้งาน
  | 'translating'  // กำลังแปล
  | 'failed'       // ล้มเหลว

// Subtitle Type - ประเภทของ subtitle
export type SubtitleType = 'original' | 'translated'

// Subtitle record
export interface Subtitle {
  id: string
  videoId: string
  language: string
  type: SubtitleType
  sourceLanguage?: string
  confidence?: number
  srtPath?: string
  status: SubtitleStatus
  error?: string
  createdAt: string
  updatedAt: string
}

// Response จาก GET /api/v1/videos/:id/subtitles
export interface SubtitlesResponse {
  videoId: string
  detectedLanguage?: string
  hasAudio: boolean
  subtitles: Subtitle[]
  availableLanguages: string[]
}

// Language info
export interface LanguageInfo {
  code: string
  name: string
}

// Response จาก GET /api/v1/subtitles/languages
export interface SupportedLanguagesResponse {
  sourceLanguages: LanguageInfo[]
  translationPairs: Record<string, string[]>
}

// Request สำหรับ trigger translation
export interface TranslateRequest {
  targetLanguages: string[]
}

// Response หลัง trigger detect
export interface DetectLanguageResponse {
  videoId: string
  message: string
  audioPath?: string
}

// Response หลัง trigger transcribe
export interface TranscribeResponse {
  videoId: string
  subtitleId: string
  language: string
  message: string
}

// Response หลัง trigger translate
export interface TranslateJobResponse {
  videoId: string
  subtitleIds: string[]
  sourceLanguage: string
  targetLanguages: string[]
  message: string
}

// Response หลัง retry stuck subtitles
export interface RetryStuckResponse {
  totalFound: number
  totalRetried: number
  skipped: number
  message: string
  errors?: string[]
}
