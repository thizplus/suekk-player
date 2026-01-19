// ==================== Online Worker Types (from NATS KV - Auto-Discovery) ====================

export type WorkerStatusType = 'idle' | 'processing' | 'stopping' | 'paused'
export type DiskLevelType = 'normal' | 'warning' | 'caution' | 'critical'
export type WorkerType = 'transcode' | 'subtitle'

export interface WorkerJob {
  video_id: string
  video_code: string
  title: string
  progress: number
  stage: string
  started_at: string
  eta: string
}

export interface WorkerStats {
  total_processed: number
  total_failed: number
  uptime_seconds: number
}

export interface WorkerConfig {
  gpu_enabled: boolean
  concurrency: number
  preset: string
}

export interface WorkerDisk {
  usage_percent: number
  total_gb: number
  free_gb: number
  used_gb: number
  level: DiskLevelType
  is_paused: boolean
}

export interface OnlineWorker {
  worker_id: string
  worker_type: WorkerType
  hostname: string
  internal_ip: string
  started_at: string
  last_seen: string
  status: WorkerStatusType
  current_jobs: WorkerJob[]
  stats: WorkerStats
  config: WorkerConfig
  disk: WorkerDisk
}

export interface WorkerTypeSummary {
  transcode: number
  subtitle: number
}

export interface OnlineWorkersResponse {
  workers: OnlineWorker[]
  total_online: number
  summary: {
    idle: number
    processing: number
    stopping: number
    paused: number
    total_jobs: number
    by_type: WorkerTypeSummary
  }
}
