// ==================== Queue Stats ====================

export interface TranscodeStats {
  pending: number
  queued: number
  processing: number
  failed: number
  dead_letter: number
}

export interface SubtitleStats {
  queued: number
  processing: number
  failed: number
}

export interface WarmCacheStats {
  not_cached: number
  warming: number
  cached: number
  failed: number
}

export interface QueueStatsResponse {
  transcode: TranscodeStats
  subtitle: SubtitleStats
  warm_cache: WarmCacheStats
}

// ==================== Queue Items ====================

export interface TranscodeQueueItem {
  id: string
  code: string
  title: string
  status: string
  error: string
  retry_count: number
  created_at: string
  updated_at: string
}

export interface SubtitleQueueItem {
  id: string
  video_id: string
  video_code: string
  video_title: string
  language: string
  type: string // transcribed | translated
  status: string
  error: string
  created_at: string
  updated_at: string
}

export interface WarmCacheQueueItem {
  id: string
  code: string
  title: string
  cache_status: string
  cache_percentage: number
  qualities: string[]
  error: string
  last_warmed_at: string | null
}

// ==================== Response Types ====================

export interface RetryResponse {
  total_found: number
  total_retried: number
  skipped: number
  message: string
  errors?: string[]
}

export interface WarmCacheResponse {
  video_id: string
  code: string
  message: string
}

export interface WarmAllResponse {
  total_found: number
  total_queued: number
  message: string
}

