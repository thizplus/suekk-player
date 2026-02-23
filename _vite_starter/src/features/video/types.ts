import type { Subtitle } from '@/features/subtitle/types'

// Video Status enum ตรงกับ backend
export type VideoStatus = 'pending' | 'queued' | 'processing' | 'ready' | 'failed' | 'dead_letter'

// Subtitle Brief (สำหรับแสดงใน video list)
export interface SubtitleBrief {
  language: string
  status: string
}

// Subtitle Summary
export interface SubtitleSummary {
  original?: SubtitleBrief
  translations?: SubtitleBrief[]
}

// Video Response จาก API
export interface Video {
  id: string
  code: string
  title: string
  description: string
  duration: number
  quality: string
  thumbnailUrl: string
  hlsPath?: string
  status: VideoStatus
  views: number
  diskUsage?: number            // ขนาดไฟล์รวม (bytes)
  qualitySizes?: QualitySizes   // ขนาดแยกตาม quality {"1080p": bytes}
  category?: Category | null
  user?: UserBasic | null
  hasAudio?: boolean
  detectedLanguage?: string
  subtitleSummary?: SubtitleSummary  // สรุป subtitle
  subtitles?: Subtitle[]             // Full subtitle list (สำหรับ embed/preview)
  reelCount?: number                 // จำนวน reels ที่สร้างจาก video นี้
  galleryPath?: string               // S3 path prefix e.g., "gallery/ABC123"
  galleryCount?: number              // จำนวนภาพทั้งหมด (0 = ไม่มี)
  gallerySafeCount?: number          // จำนวนภาพ safe (SFW)
  galleryNsfwCount?: number          // จำนวนภาพ nsfw
  createdAt: string
  updatedAt: string
}

// Quality sizes map (quality -> bytes)
export type QualitySizes = Record<string, number>

export interface Category {
  id: string
  name: string
  slug: string
  createdAt: string
}

export interface UserBasic {
  id: string
  username: string
  avatar?: string
}

// Upload Response
export interface VideoUploadResponse {
  id: string
  code: string
  title: string
  status: string
}

// Batch Upload Types
export interface BatchUploadResult {
  filename: string
  success: boolean
  video?: VideoUploadResponse
  error?: string
}

export interface BatchUploadResponse {
  total: number
  success: number
  errors: number
  results: BatchUploadResult[]
}

// Request Types
export interface CreateVideoRequest {
  title: string
  description?: string
  categoryId?: string
}

export interface UpdateVideoRequest {
  title?: string
  description?: string
  categoryId?: string
}

export interface VideoFilterParams {
  search?: string
  status?: VideoStatus
  categoryId?: string
  dateFrom?: string   // YYYY-MM-DD
  dateTo?: string     // YYYY-MM-DD
  sortBy?: 'created_at' | 'title' | 'views'
  sortOrder?: 'asc' | 'desc'
  page?: number
  limit?: number
}

// Transcoding Types
export interface TranscodingStatus {
  videoId: string
  status: VideoStatus
  progress: number
  error?: string
}

export interface TranscodingStats {
  pending: number
  queued: number
  processing: number
  completed: number
  failed: number
}

// Worker Types (from heartbeat)
export type WorkerStatusType = 'idle' | 'processing' | 'stopping' | 'paused'
export type DiskLevelType = 'normal' | 'warning' | 'caution' | 'critical'

export interface WorkerJob {
  video_id: string
  video_code: string
  title: string
  progress: number
  stage: string // downloading, transcoding, uploading
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

export interface Worker {
  worker_id: string
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

export interface WorkersResponse {
  workers: Worker[]
  total_online: number
  summary: {
    idle: number
    processing: number
    stopping: number
    paused: number
    total_jobs: number
  }
}

// Dead Letter Queue (DLQ) Types
export interface ErrorRecord {
  attempt: number
  error: string
  workerId: string
  stage: string
  timestamp: string
}

export interface DLQVideo {
  id: string
  code: string
  title: string
  retryCount: number
  lastError: string
  errorHistory?: ErrorRecord[]
  createdAt: string
  updatedAt: string
  userId: string
}

// Upload Limits (from /api/v1/config/upload-limits)
export interface UploadLimits {
  max_file_size: number      // bytes
  max_file_size_gb: number   // GB
  part_size: number          // bytes
  allowed_types: string[]
}

// Gallery URLs Response (presigned URLs for all gallery images)
export interface GalleryUrlsResponse {
  code: string
  count: number
  urls: string[]
  expires_at: number // Unix timestamp
}
