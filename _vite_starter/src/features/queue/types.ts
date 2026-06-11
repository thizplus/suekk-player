// ==================== Worker Job Types ====================

// Job types (matches backend WorkerJobType)
export type WorkerJobType =
  | 'transcode'
  | 'gallery'
  | 'warmcache'
  | 'subtitle_detect'
  | 'subtitle_transcribe'
  | 'subtitle_translate'
  | 'reel'

// Job status (matches backend WorkerJobStatus)
export type WorkerJobStatus =
  | 'pending'
  | 'queued'
  | 'processing'
  | 'completed'
  | 'failed'
  | 'cancelled'
  | 'dead_letter'

// Entity types
export type WorkerEntityType = 'video' | 'subtitle' | 'reel'

// ==================== Worker Job Response ====================

export interface WorkerJob {
  id: string
  job_type: WorkerJobType
  entity_type: WorkerEntityType
  entity_id: string
  entity_code: string
  status: WorkerJobStatus
  progress: number
  current_stage: string
  message: string
  worker_id?: string
  priority: number
  created_at: string
  queued_at?: string
  started_at?: string
  completed_at?: string
  retry_count: number
  max_retries: number
  last_error?: string
  duration_sec?: number
}

export interface WorkerJobDetail extends WorkerJob {
  job_data?: Record<string, unknown>
  output_data?: Record<string, unknown>
  error_history?: WorkerJobErrorRecord[]
}

export interface WorkerJobErrorRecord {
  attempt: number
  error: string
  error_code?: string
  worker_id?: string
  stage?: string
  timestamp: string
}

// ==================== Stats ====================

export interface WorkerJobStats {
  total_jobs: number
  pending_jobs: number
  queued_jobs: number
  processing_jobs: number
  completed_jobs: number
  failed_jobs: number
  avg_duration_sec: number
  success_rate: number
}

// Stats grouped by job type (from /stats/all)
export type WorkerJobAllStats = {
  [K in WorkerJobType]?: WorkerJobStats
}

// ==================== Legacy Types (for warm cache) ====================

// Legacy stats (for WarmCache which uses Video.CacheStatus)
export interface LegacyQueueStats {
  transcode: {
    pending: number
    queued: number
    processing: number
    failed: number
    deadLetter: number
  }
  subtitle: {
    queued: number
    processing: number
    failed: number
  }
  warmCache: {
    notCached: number
    warming: number
    cached: number
    failed: number
  }
  gallery: {
    none: number
    processing: number
    pendingReview: number
    ready: number
    failed: number
  }
  reel: {
    draft: number
    exporting: number
    ready: number
    failed: number
  }
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

// ==================== Response Types ====================

export interface WorkerJobListResponse {
  jobs: WorkerJob[]
  meta: {
    total: number
    offset: number
    limit: number
  }
}

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

// ==================== Subtitle Batch Types ====================

export interface SubtitleStats {
  totalVideos: number
  detected: number
  notDetected: number
  transcribed: number
  notTranscribed: number
  translated: number
  notTranslated: number
}

export interface BatchActionResponse {
  queued: number
  skipped: number
  errors: string[]
  message: string
}
