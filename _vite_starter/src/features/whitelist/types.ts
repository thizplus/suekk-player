// ==================== Whitelist Profile Types ====================

export interface WhitelistProfile {
  id: string
  name: string
  description: string
  isActive: boolean

  // Thumbnail Settings (แสดงก่อนกด play)
  thumbnailUrl: string

  // Watermark Settings
  watermarkEnabled: boolean
  watermarkUrl: string
  watermarkPosition: WatermarkPosition
  watermarkOpacity: number
  watermarkSize: number
  watermarkOffsetY: number

  // Pre-roll Ads Settings (legacy - single preroll)
  prerollEnabled: boolean
  prerollUrl: string
  prerollSkipAfter: number // 0 = ไม่ให้ skip

  // Relations
  domains?: ProfileDomain[]
  prerollAds?: PrerollAd[]

  // Timestamps
  createdAt: string
  updatedAt: string
}

// Ad Type
export type AdType = 'video' | 'image'

// Preroll Ad for multiple prerolls support
export interface PrerollAd {
  id: string
  profileId: string

  // Ad Type & Content
  type: AdType // video หรือ image
  url: string
  duration: number // ระยะเวลา (วินาที) - ใช้กับ image

  // Skip Settings
  skipAfter: number // 0 = บังคับดูจบ

  // Click/Link Settings
  clickUrl?: string // URL เมื่อคลิกโฆษณา
  clickText?: string // ข้อความปุ่ม เช่น "ดูรายละเอียด"

  // Display Settings
  title?: string // ชื่อโฆษณา/ผู้สนับสนุน

  // Order & Timestamps
  sortOrder: number
  createdAt: string
  updatedAt: string
}

export type WatermarkPosition = 'top-left' | 'top-right' | 'bottom-left' | 'bottom-right'

export interface ProfileDomain {
  id: string
  profileId: string
  domain: string
  createdAt: string
}

// ==================== Request DTOs ====================

export interface CreateWhitelistProfileRequest {
  name: string
  description?: string
  isActive?: boolean

  // Thumbnail Settings
  thumbnailUrl?: string

  // Watermark Settings
  watermarkEnabled?: boolean
  watermarkUrl?: string
  watermarkPosition?: WatermarkPosition
  watermarkOpacity?: number
  watermarkSize?: number
  watermarkOffsetY?: number

  // Pre-roll Ads Settings
  prerollEnabled?: boolean
  prerollUrl?: string
  prerollSkipAfter?: number

  // Initial domains
  domains?: string[]
}

export interface UpdateWhitelistProfileRequest {
  name?: string
  description?: string
  isActive?: boolean

  // Thumbnail Settings
  thumbnailUrl?: string

  // Watermark Settings
  watermarkEnabled?: boolean
  watermarkUrl?: string
  watermarkPosition?: WatermarkPosition
  watermarkOpacity?: number
  watermarkSize?: number
  watermarkOffsetY?: number

  // Pre-roll Ads Settings
  prerollEnabled?: boolean
  prerollUrl?: string
  prerollSkipAfter?: number
}

export interface AddDomainRequest {
  domain: string
}

// ==================== Preroll Ads Request DTOs ====================

export interface AddPrerollAdRequest {
  // Ad Type & Content (required)
  type: AdType // video หรือ image
  url: string
  duration?: number // ระยะเวลา (วินาที) - ใช้กับ image

  // Skip Settings
  skipAfter: number // 0 = บังคับดูจบ

  // Click/Link Settings (optional)
  clickUrl?: string // URL เมื่อคลิกโฆษณา
  clickText?: string // ข้อความปุ่ม เช่น "ดูรายละเอียด"

  // Display Settings (optional)
  title?: string // ชื่อโฆษณา/ผู้สนับสนุน
}

export interface UpdatePrerollAdRequest {
  // Ad Type & Content (required)
  type: AdType
  url: string
  duration?: number

  // Skip Settings
  skipAfter: number

  // Click/Link Settings
  clickUrl?: string
  clickText?: string

  // Display Settings
  title?: string
}

export interface ReorderPrerollAdsRequest {
  prerollIds: string[]
}

// ==================== Ad Statistics Types ====================

export interface AdImpressionStats {
  totalImpressions: number
  completed: number
  skipped: number
  errors: number
  completionRate: number
  skipRate: number
  errorRate: number
  avgWatchDuration: number
  avgSkipTime: number
}

export interface DeviceStats {
  mobile: number
  desktop: number
  tablet: number
}

export interface ProfileRanking {
  profileId: string
  profileName: string
  totalViews: number
  completionRate: number
}

export interface AdStatsFilterParams {
  start?: string // YYYY-MM-DD
  end?: string   // YYYY-MM-DD
  limit?: number
}

// ==================== Embed Config Types ====================

export interface EmbedConfig {
  profileId: string
  isAllowed: boolean
  streamToken?: string // Hybrid Shield - Token ส่งผ่าน X-Stream-Token header
  streamUrl?: string   // CDN URL สำหรับ HLS streaming
  thumbnailUrl?: string // Thumbnail จาก profile (ถ้าไม่มีใช้ของวิดีโอ)
  watermark?: WatermarkConfig
  preroll?: PrerollConfig // Legacy single preroll
  prerollAds?: PrerollConfig[] // Multiple prerolls
}

export interface WatermarkConfig {
  enabled: boolean
  url: string
  position: WatermarkPosition
  opacity: number
  size: number
  offsetY: number
}

export interface PrerollConfig {
  enabled: boolean

  // Ad Type & Content
  type: AdType // video หรือ image
  url: string
  duration?: number // ระยะเวลา (วินาที) - ใช้กับ image

  // Skip Settings
  skipAfter: number // 0 = ไม่ให้ skip

  // Click/Link Settings
  clickUrl?: string // URL เมื่อคลิกโฆษณา
  clickText?: string // ข้อความปุ่ม เช่น "ดูรายละเอียด"

  // Display Settings
  title?: string // ชื่อโฆษณา/ผู้สนับสนุน
}

// ==================== Ad Impression Recording ====================

export interface RecordAdImpressionRequest {
  profileId?: string
  videoCode: string
  domain?: string
  adUrl?: string
  adDuration?: number
  watchDuration?: number
  completed?: boolean
  skipped?: boolean
  skippedAt?: number
  errorOccurred?: boolean
}
