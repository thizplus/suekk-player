// ==================== Queue Stats ====================

export interface TranscodeStats {
  pending: number
  queued: number
  processing: number
  failed: number
  deadLetter: number
}

export interface SubtitleStats {
  queued: number
  processing: number
  failed: number
}

export interface WarmCacheStats {
  notCached: number
  warming: number
  cached: number
  failed: number
}

export interface GalleryStats {
  none: number
  processing: number
  pendingReview: number
  ready: number
  failed: number
}

export interface ReelStats {
  draft: number
  exporting: number
  ready: number
  failed: number
}

export interface QueueStatsResponse {
  transcode: TranscodeStats
  subtitle: SubtitleStats
  warmCache: WarmCacheStats
  gallery: GalleryStats
  reel: ReelStats
}

// ==================== Queue Items ====================

export interface TranscodeQueueItem {
  id: string
  code: string
  title: string
  status: string
  error: string
  retryCount: number
  createdAt: string
  updatedAt: string
}

export interface SubtitleQueueItem {
  id: string
  videoId: string
  videoCode: string
  videoTitle: string
  language: string
  type: string // transcribed | translated
  status: string
  error: string
  createdAt: string
  updatedAt: string
}

export interface WarmCacheQueueItem {
  id: string
  code: string
  title: string
  cacheStatus: string
  cachePercentage: number
  qualities: string[]
  error: string
  lastWarmedAt: string | null
}

export interface GalleryQueueItem {
  id: string
  code: string
  title: string
  galleryStatus: string
  sourceCount: number
  safeCount: number
  nsfwCount: number
  error?: string
  createdAt: string
  updatedAt: string
}

export interface ReelQueueItem {
  id: string
  videoId: string
  videoCode: string
  videoTitle: string
  reelTitle: string
  status: string
  error?: string
  duration: number
  createdAt: string
  updatedAt: string
}

// ==================== Response Types ====================

export interface RetryResponse {
  totalFound: number
  totalRetried: number
  skipped: number
  message: string
  errors?: string[]
}

export interface ClearResponse {
  totalFound: number
  totalDeleted: number
  skipped: number
  natsJobsPurged: number
  message: string
}

export interface QueueMissingResponse {
  totalVideos: number
  totalMissing: number
  totalQueued: number
  skipped: number
  message: string
}

export interface WarmCacheResponse {
  videoId: string
  code: string
  message: string
}

export interface WarmAllResponse {
  totalFound: number
  totalQueued: number
  message: string
}
