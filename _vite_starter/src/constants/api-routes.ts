// Centralized API endpoints

export const AUTH_ROUTES = {
  LOGIN: '/api/v1/auth/login',
  REGISTER: '/api/v1/auth/register',
  LOGOUT: '/api/v1/auth/logout',
  REFRESH: '/api/v1/auth/refresh',
  ME: '/api/v1/auth/me',
  GOOGLE_URL: '/api/v1/auth/google',
  GOOGLE_CALLBACK: '/api/v1/auth/google/callback',
}

export const USER_ROUTES = {
  LIST: '/api/v1/users',
  BY_ID: (id: string) => `/api/v1/users/${id}`,
  PROFILE: '/api/v1/users/profile',
  SET_PASSWORD: '/api/v1/users/set-password',
}

export const DASHBOARD_ROUTES = {
  STATS: '/api/v1/dashboard/stats',
  ADMIN: '/api/v1/dashboard/admin',
  AGENT: '/api/v1/dashboard/agent',
  SALES: '/api/v1/dashboard/sales',
}

export const VIDEO_ROUTES = {
  LIST: '/api/v1/videos',
  BY_ID: (id: string) => `/api/v1/videos/${id}`,
  BY_CODE: (code: string) => `/api/v1/videos/code/${code}`,
  UPLOAD: '/api/v1/videos/upload',
  BATCH_UPLOAD: '/api/v1/videos/batch',
  UPDATE_STATUS: (id: string) => `/api/v1/videos/${id}/status`,
  // Gallery Generation
  GENERATE_GALLERY: (id: string) => `/api/v1/videos/${id}/generate-gallery`,
  REGENERATE_GALLERY: (id: string) => `/api/v1/videos/${id}/regenerate-gallery`,
  // Dead Letter Queue (DLQ) Management
  DLQ_LIST: '/api/v1/videos/dlq',
  DLQ_RETRY: (id: string) => `/api/v1/videos/dlq/${id}/retry`,
  DLQ_DELETE: (id: string) => `/api/v1/videos/dlq/${id}`,
}

export const CATEGORY_ROUTES = {
  LIST: '/api/v1/categories',
  TREE: '/api/v1/categories/tree',
  BY_ID: (id: string) => `/api/v1/categories/${id}`,
  REORDER: '/api/v1/categories/reorder',
}

export const TRANSCODING_ROUTES = {
  QUEUE: '/api/v1/transcoding/queue',
  QUEUE_VIDEO: (videoId: string) => `/api/v1/transcoding/queue/${videoId}`,
  STATUS: (videoId: string) => `/api/v1/transcoding/status/${videoId}`,
  STATS: '/api/v1/transcoding/stats',
  WORKERS: '/api/v1/transcoding/workers',
}

// Phase 6: Domain Whitelist & Ad Management
export const WHITELIST_ROUTES = {
  // Profile Management (Protected)
  PROFILES: '/api/v1/whitelist/profiles',
  PROFILE_BY_ID: (id: string) => `/api/v1/whitelist/profiles/${id}`,
  PROFILE_DOMAINS: (profileId: string) => `/api/v1/whitelist/profiles/${profileId}/domains`,
  DOMAIN_BY_ID: (domainId: string) => `/api/v1/whitelist/domains/${domainId}`,

  // Preroll Ads Management (Protected)
  PROFILE_PREROLLS: (profileId: string) => `/api/v1/whitelist/profiles/${profileId}/prerolls`,
  PROFILE_PREROLLS_REORDER: (profileId: string) => `/api/v1/whitelist/profiles/${profileId}/prerolls/reorder`,
  PREROLL_BY_ID: (prerollId: string) => `/api/v1/whitelist/prerolls/${prerollId}`,

  // Ad Statistics (Protected)
  AD_STATS: '/api/v1/ads/stats',
  AD_STATS_BY_PROFILE: (profileId: string) => `/api/v1/ads/stats/profile/${profileId}`,
  AD_STATS_DEVICES: '/api/v1/ads/stats/devices',
  AD_STATS_RANKING: '/api/v1/ads/stats/ranking',
  AD_STATS_SKIP_DISTRIBUTION: '/api/v1/ads/stats/skip-distribution',

  // Cache Management (Protected)
  CACHE_CLEAR_ALL: '/api/v1/whitelist/cache/clear',
  CACHE_CLEAR_DOMAIN: (domain: string) => `/api/v1/whitelist/cache/domain/${domain}`,

  // Public endpoints (for embed player)
  AD_IMPRESSION: '/api/v1/ads/impression',
  EMBED_CONFIG: '/api/v1/embed/config',
}

// Phase 0: Worker Registry Management
export const WORKER_ROUTES = {
  LIST: '/api/v1/admin/workers',
  BY_ID: (id: string) => `/api/v1/admin/workers/${id}`,
  REGENERATE_TOKEN: (id: string) => `/api/v1/admin/workers/${id}/regenerate-token`,
  ENABLE: (id: string) => `/api/v1/admin/workers/${id}/enable`,
  DISABLE: (id: string) => `/api/v1/admin/workers/${id}/disable`,
}

// Admin Settings Management
export const SETTINGS_ROUTES = {
  // Get all settings (grouped by category)
  ALL: '/api/v1/settings',
  // Get categories list
  CATEGORIES: '/api/v1/settings/categories',
  // Get/Update settings by category
  BY_CATEGORY: (category: string) => `/api/v1/settings/${category}`,
  // Reset settings to defaults
  RESET_CATEGORY: (category: string) => `/api/v1/settings/${category}/reset`,
  // Audit logs
  AUDIT_LOGS: '/api/v1/settings/audit-logs',
  // Reload cache
  RELOAD_CACHE: '/api/v1/settings/reload-cache',
}

// Direct Upload - อัปโหลดตรงไป S3 ผ่าน Presigned URL
export const DIRECT_UPLOAD_ROUTES = {
  // เริ่ม multipart upload, รับ presigned URLs
  INIT: '/api/v1/direct-upload/init',
  // รวม parts และ auto-queue transcode
  COMPLETE: '/api/v1/direct-upload/complete',
  // ยกเลิก upload ที่ค้าง
  ABORT: '/api/v1/direct-upload/abort',
}

// Config - ค่าตั้งค่าสำหรับ Frontend
export const CONFIG_ROUTES = {
  // ดึง upload limits (max file size, allowed types)
  UPLOAD_LIMITS: '/api/v1/config/upload-limits',
}

// Storage - ข้อมูล Storage usage และ quota
export const STORAGE_ROUTES = {
  // ดึง storage usage (used, quota, percent)
  USAGE: '/api/v1/storage/usage',
  // ดึง storage stats (admin)
  STATS: '/api/v1/storage/stats',
}

// Queue Management - จัดการ queue ทั้งหมด (transcode/subtitle/warmcache)
export const QUEUE_ROUTES = {
  // Stats รวมทุก queue
  STATS: '/api/v1/admin/queues/stats',
  // Transcode Queue
  TRANSCODE_FAILED: '/api/v1/admin/queues/transcode/failed',
  TRANSCODE_RETRY_ALL: '/api/v1/admin/queues/transcode/retry-all',
  TRANSCODE_RETRY_ONE: (id: string) => `/api/v1/admin/queues/transcode/${id}/retry`,
  // Subtitle Queue
  SUBTITLE_STUCK: '/api/v1/admin/queues/subtitle/stuck',
  SUBTITLE_FAILED: '/api/v1/admin/queues/subtitle/failed',
  SUBTITLE_RETRY_ALL: '/api/v1/admin/queues/subtitle/retry-all',
  SUBTITLE_CLEAR_ALL: '/api/v1/admin/queues/subtitle/clear-all',
  SUBTITLE_QUEUE_MISSING: '/api/v1/admin/queues/subtitle/queue-missing',
  // Warm Cache Queue
  WARM_CACHE_PENDING: '/api/v1/admin/queues/warm-cache/pending',
  WARM_CACHE_FAILED: '/api/v1/admin/queues/warm-cache/failed',
  WARM_CACHE_ONE: (id: string) => `/api/v1/admin/queues/warm-cache/${id}/warm`,
  WARM_CACHE_ALL: '/api/v1/admin/queues/warm-cache/warm-all',
}

// Reel Generator - สร้าง reels สำหรับ social media
export const REEL_ROUTES = {
  // Templates
  TEMPLATES: '/api/v1/reels/templates',
  TEMPLATE_BY_ID: (id: string) => `/api/v1/reels/templates/${id}`,
  // Reels CRUD
  LIST: '/api/v1/reels',
  BY_ID: (id: string) => `/api/v1/reels/${id}`,
  CREATE: '/api/v1/reels',
  UPDATE: (id: string) => `/api/v1/reels/${id}`,
  DELETE: (id: string) => `/api/v1/reels/${id}`,
  EXPORT: (id: string) => `/api/v1/reels/${id}/export`,
  // Video reels
  BY_VIDEO: (videoId: string) => `/api/v1/videos/${videoId}/reels`,
}

// HLS - Streaming access and gallery
export const HLS_ROUTES = {
  // Get HLS access token and playlist URL
  ACCESS: (code: string) => `/api/v1/hls/${code}/access`,
  // Get presigned URLs for gallery images (single API call)
  GALLERY_URLS: (code: string) => `/api/v1/hls/${code}/gallery`,
}

// Subtitle - จัดการ subtitle และ translation
export const SUBTITLE_ROUTES = {
  // ภาษาที่รองรับ
  LANGUAGES: '/api/v1/subtitles/languages',
  // ดึง subtitles ของ video (protected - ต้อง login)
  BY_VIDEO: (videoId: string) => `/api/v1/videos/${videoId}/subtitles`,
  // ดึง subtitles โดยใช้ video code (public - สำหรับ embed)
  BY_CODE: (code: string) => `/api/v1/embed/videos/${code}/subtitles`,
  // Trigger actions
  DETECT: (videoId: string) => `/api/v1/videos/${videoId}/subtitle/detect`,
  SET_LANGUAGE: (videoId: string) => `/api/v1/videos/${videoId}/subtitle/language`,
  TRANSCRIBE: (videoId: string) => `/api/v1/videos/${videoId}/subtitle/transcribe`,
  TRANSLATE: (videoId: string) => `/api/v1/videos/${videoId}/subtitle/translate`,
  // Delete subtitle
  DELETE: (subtitleId: string) => `/api/v1/subtitles/${subtitleId}`,
  // Admin: Retry stuck subtitles ทั้งหมด
  RETRY_STUCK: '/api/v1/admin/subtitles/retry-stuck',
  // Content editing - GET/PUT subtitle content (SRT file)
  CONTENT: (subtitleId: string) => `/api/v1/subtitles/${subtitleId}/content`,
}
