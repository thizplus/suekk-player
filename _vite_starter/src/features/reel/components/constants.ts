import type { OutputFormat, VideoFit, OutputFormatOption, VideoFitOption, ReelStyle, ReelStyleOption } from '../types'

// NEW: Style-based options (preferred)
export const REEL_STYLE_OPTIONS: ReelStyleOption[] = [
  {
    value: 'letterbox',
    label: 'Letterbox',
    description: 'วิดีโอ 16:9 มีขอบดำบน-ล่าง',
    icon: '🎬',
  },
  {
    value: 'square',
    label: 'Square',
    description: 'วิดีโอ 1:1 มีขอบดำบน-ล่าง',
    icon: '⬜',
  },
  {
    value: 'fullcover',
    label: 'Full Cover',
    description: 'เต็มจอ + gradient ล่าง',
    icon: '📱',
  },
]

// Helper to get style option
export const getStyleOption = (style: ReelStyle): ReelStyleOption | undefined => {
  return REEL_STYLE_OPTIONS.find(o => o.value === style)
}

// LEGACY: Output format options (deprecated)
export const OUTPUT_FORMAT_OPTIONS: OutputFormatOption[] = [
  { value: '9:16', label: '9:16', aspectClass: 'aspect-[9/16]', description: 'Reels/TikTok' },
  { value: '1:1', label: '1:1', aspectClass: 'aspect-square', description: 'Square' },
  { value: '4:5', label: '4:5', aspectClass: 'aspect-[4/5]', description: 'Instagram' },
  { value: '16:9', label: '16:9', aspectClass: 'aspect-video', description: 'YouTube' },
]

// LEGACY: Video fit options (deprecated)
export const VIDEO_FIT_OPTIONS: VideoFitOption[] = [
  { value: 'fill', label: 'เต็มกรอบ', description: 'Crop ให้เต็ม' },
  { value: 'fit', label: 'พอดี', description: 'มีขอบดำ' },
  { value: 'crop-1:1', label: '1:1', description: 'สี่เหลี่ยม', aspectRatio: '1/1' },
  { value: 'crop-4:3', label: '4:3', description: 'จอเก่า', aspectRatio: '4/3' },
  { value: 'crop-4:5', label: '4:5', description: 'IG', aspectRatio: '4/5' },
]

// ขนาด chunk สำหรับแบ่ง timeline (10 นาที = 600 วินาที)
export const CHUNK_SIZE = 600

// จำกัด duration สูงสุด 10 นาที (600 วินาที) สำหรับทำ reel
export const MAX_REEL_DURATION = 600

// Quick duration presets (seconds)
export const QUICK_DURATIONS = [15, 30, 45, 60, 90]

// สร้าง chunk options จาก video duration
export interface ChunkOption {
  value: number // chunk index (0, 1, 2, ...)
  label: string // "0:00 - 10:00"
  start: number // seconds
  end: number   // seconds
}

export const generateChunkOptions = (totalDuration: number): ChunkOption[] => {
  const chunks: ChunkOption[] = []
  let chunkIndex = 0
  let start = 0

  while (start < totalDuration) {
    const end = Math.min(start + CHUNK_SIZE, totalDuration)
    chunks.push({
      value: chunkIndex,
      label: `${formatTime(start)} - ${formatTime(end)}`,
      start,
      end,
    })
    start = end
    chunkIndex++
  }

  return chunks
}

// Helper functions
export const getAspectClass = (format: OutputFormat): string => {
  return OUTPUT_FORMAT_OPTIONS.find(o => o.value === format)?.aspectClass || 'aspect-[9/16]'
}

export const getCropAspectRatio = (fit: VideoFit): string | undefined => {
  return VIDEO_FIT_OPTIONS.find(o => o.value === fit)?.aspectRatio
}

export const formatTime = (seconds: number): string => {
  const mins = Math.floor(seconds / 60)
  const secs = Math.floor(seconds % 60)
  return `${mins}:${secs.toString().padStart(2, '0')}`
}
