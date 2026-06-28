// Status enums with labels and semantic class names

export const STATUS = {
  PENDING: 'pending',
  APPROVED: 'approved',
  REJECTED: 'rejected',
  ACTIVE: 'active',
  INACTIVE: 'inactive',
} as const

export type StatusType = (typeof STATUS)[keyof typeof STATUS]

export const STATUS_LABELS: Record<StatusType, string> = {
  pending: 'รออนุมัติ',
  approved: 'อนุมัติแล้ว',
  rejected: 'ปฏิเสธ',
  active: 'ใช้งาน',
  inactive: 'ไม่ใช้งาน',
}

export const STATUS_STYLES: Record<StatusType, string> = {
  pending: 'status-pending',
  approved: 'status-success',
  rejected: 'status-danger',
  active: 'status-success',
  inactive: 'status-muted',
}

// Video status
export const VIDEO_STATUS = {
  PENDING: 'pending',
  QUEUED: 'queued',
  PROCESSING: 'processing',
  READY: 'ready',
  FAILED: 'failed',
  DEAD_LETTER: 'dead_letter', // Poison pill - ต้องตรวจสอบ manual
} as const

export type VideoStatusType = (typeof VIDEO_STATUS)[keyof typeof VIDEO_STATUS]

export const VIDEO_STATUS_LABELS: Record<VideoStatusType, string> = {
  pending: 'รอดำเนินการ',
  queued: 'อยู่ในคิว',
  processing: 'กำลังประมวลผล',
  ready: 'พร้อมใช้งาน',
  failed: 'ล้มเหลว',
  dead_letter: 'ทีมงานกำลังตรวจสอบ', // User-friendly: ไม่ใช้คำว่า "ล้มเหลว"
}

export const VIDEO_STATUS_DESCRIPTIONS: Record<VideoStatusType, string> = {
  pending: 'ไฟล์กำลังอัปโหลดไปยังเซิร์ฟเวอร์',
  queued: 'อยู่ในคิวรอการประมวลผล',
  processing: 'ระบบกำลังแปลงไฟล์วิดีโอ',
  ready: 'วิดีโอพร้อมสำหรับการรับชม',
  failed: 'เกิดข้อผิดพลาด กรุณาลองอัปโหลดใหม่',
  dead_letter: 'ไฟล์ของคุณอยู่ระหว่างการตรวจสอบโดยทีมงาน เราจะแจ้งให้ทราบเมื่อดำเนินการเสร็จสิ้น',
}

export const VIDEO_STATUS_STYLES: Record<VideoStatusType, string> = {
  pending: 'status-pending',
  queued: 'status-queued',
  processing: 'status-processing',
  ready: 'status-success',
  failed: 'status-danger',
  dead_letter: 'status-info', // Blue/info color - ไม่ใช่ danger
}

// Worker status
export const WORKER_STATUS = {
  IDLE: 'idle',
  PROCESSING: 'processing',
  STOPPING: 'stopping',
  PAUSED: 'paused',
} as const

export type WorkerStatusEnum = (typeof WORKER_STATUS)[keyof typeof WORKER_STATUS]

export const WORKER_STATUS_LABELS: Record<WorkerStatusEnum, string> = {
  idle: 'ว่าง',
  processing: 'กำลังทำงาน',
  stopping: 'กำลังหยุด',
  paused: 'หยุดชั่วคราว',
}

export const WORKER_STATUS_STYLES: Record<WorkerStatusEnum, string> = {
  idle: 'status-muted',
  processing: 'status-processing',
  stopping: 'status-pending',
  paused: 'status-danger',
}

// Disk level (for worker disk monitoring)
export const DISK_LEVEL = {
  NORMAL: 'normal',
  WARNING: 'warning',
  CAUTION: 'caution',
  CRITICAL: 'critical',
} as const

export type DiskLevelEnum = (typeof DISK_LEVEL)[keyof typeof DISK_LEVEL]

export const DISK_LEVEL_LABELS: Record<DiskLevelEnum, string> = {
  normal: 'ปกติ',
  warning: 'เตือน',
  caution: 'ระวัง',
  critical: 'วิกฤต',
}

export const DISK_LEVEL_STYLES: Record<DiskLevelEnum, string> = {
  normal: 'status-success',
  warning: 'status-pending',
  caution: 'status-queued',
  critical: 'status-danger',
}

// User roles
export const ROLE = {
  ADMIN: 'admin',
  AGENT: 'agent',
  SALES: 'sales',
} as const

export type RoleType = (typeof ROLE)[keyof typeof ROLE]

export const ROLE_LABELS: Record<RoleType, string> = {
  admin: 'ผู้ดูแลระบบ',
  agent: 'ตัวแทน',
  sales: 'พนักงานขาย',
}

// ==================== Phase 6: Whitelist & Ad Management ====================

// Watermark positions
export const WATERMARK_POSITION = {
  TOP_LEFT: 'top-left',
  TOP_RIGHT: 'top-right',
  BOTTOM_LEFT: 'bottom-left',
  BOTTOM_RIGHT: 'bottom-right',
} as const

export type WatermarkPositionType = (typeof WATERMARK_POSITION)[keyof typeof WATERMARK_POSITION]

export const WATERMARK_POSITION_LABELS: Record<WatermarkPositionType, string> = {
  'top-left': 'บนซ้าย',
  'top-right': 'บนขวา',
  'bottom-left': 'ล่างซ้าย',
  'bottom-right': 'ล่างขวา',
}

// Device types (for ad stats)
export const DEVICE_TYPE = {
  MOBILE: 'mobile',
  DESKTOP: 'desktop',
  TABLET: 'tablet',
} as const

export type DeviceTypeEnum = (typeof DEVICE_TYPE)[keyof typeof DEVICE_TYPE]

export const DEVICE_TYPE_LABELS: Record<DeviceTypeEnum, string> = {
  mobile: 'มือถือ',
  desktop: 'คอมพิวเตอร์',
  tablet: 'แท็บเล็ต',
}

export const DEVICE_TYPE_ICONS: Record<DeviceTypeEnum, string> = {
  mobile: 'Smartphone',
  desktop: 'Monitor',
  tablet: 'Tablet',
}

// Profile active status
export const PROFILE_STATUS = {
  ACTIVE: true,
  INACTIVE: false,
} as const

export const PROFILE_STATUS_LABELS: Record<string, string> = {
  true: 'ใช้งาน',
  false: 'ไม่ใช้งาน',
}

export const PROFILE_STATUS_STYLES: Record<string, string> = {
  true: 'status-success',
  false: 'status-muted',
}

// ==================== Subtitle Status ====================

export const SUBTITLE_STATUS = {
  PENDING: 'pending',
  QUEUED: 'queued',
  DETECTING: 'detecting',
  DETECTED: 'detected',
  PROCESSING: 'processing',
  READY: 'ready',
  TRANSLATING: 'translating',
  FAILED: 'failed',
} as const

export type SubtitleStatusType = (typeof SUBTITLE_STATUS)[keyof typeof SUBTITLE_STATUS]

export const SUBTITLE_STATUS_LABELS: Record<SubtitleStatusType, string> = {
  pending: 'รอดำเนินการ',
  queued: 'อยู่ในคิว',
  detecting: 'กำลังตรวจจับภาษา',
  detected: 'ตรวจจับภาษาแล้ว',
  processing: 'กำลังสร้าง Subtitle',
  ready: 'พร้อมใช้งาน',
  translating: 'กำลังแปล',
  failed: 'ล้มเหลว',
}

export const SUBTITLE_STATUS_STYLES: Record<SubtitleStatusType, string> = {
  pending: 'status-muted',
  queued: 'status-queued',
  detecting: 'status-processing',
  detected: 'status-info',
  processing: 'status-processing',
  ready: 'status-success',
  translating: 'status-processing',
  failed: 'status-danger',
}

// Language labels (ISO 639-1)
export const LANGUAGE_LABELS: Record<string, string> = {
  ja: 'ญี่ปุ่น',
  en: 'อังกฤษ',
  th: 'ไทย',
  zh: 'จีน',
  ko: 'เกาหลี',
  ru: 'รัสเซีย',
}

// Native language names (สำหรับ player — ทุกชาติอ่านได้)
export const LANGUAGE_NATIVE_LABELS: Record<string, string> = {
  ja: '日本語',
  en: 'English',
  th: 'ไทย',
  zh: '中文',
  ko: '한국어',
  ru: 'Русский',
}

export const LANGUAGE_FLAGS: Record<string, string> = {
  ja: '🇯🇵',
  en: '🇬🇧',
  th: '🇹🇭',
  zh: '🇨🇳',
  ko: '🇰🇷',
  ru: '🇷🇺',
}
