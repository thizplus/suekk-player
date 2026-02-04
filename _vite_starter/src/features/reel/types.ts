// Reel Generator Types

export type ReelStatus = 'draft' | 'exporting' | 'ready' | 'failed'

// NEW: Reel Style - simplified 3-style system
export type ReelStyle = 'letterbox' | 'square' | 'fullcover'

// Style option for UI
export interface ReelStyleOption {
  value: ReelStyle
  label: string
  description: string
  icon: string
}

// LEGACY: Output format - the final canvas/frame size (deprecated)
export type OutputFormat = '9:16' | '1:1' | '4:5' | '16:9'

// LEGACY: Video fit - how the source video fills the output frame (deprecated)
export type VideoFit = 'fill' | 'fit' | 'crop-1:1' | 'crop-4:3' | 'crop-4:5'

export type TitlePosition = 'top' | 'center' | 'bottom'

// Output format option for UI
export interface OutputFormatOption {
  value: OutputFormat
  label: string
  aspectClass: string
  description: string
}

// Video fit option for UI
export interface VideoFitOption {
  value: VideoFit
  label: string
  description: string
  aspectRatio?: string
}
export type ReelLayerType = 'text' | 'image' | 'shape' | 'background'

// Layer definition for composition
export interface ReelLayer {
  type: ReelLayerType
  content?: string           // text content or image URL
  fontFamily?: string        // for text
  fontSize?: number          // for text
  fontColor?: string         // for text
  fontWeight?: 'normal' | 'bold'
  x: number                  // position X (0-100%)
  y: number                  // position Y (0-100%)
  width?: number             // width (0-100%)
  height?: number            // height (0-100%)
  opacity?: number           // 0-1
  zIndex?: number            // layer order
  style?: string             // gradient style, shape type, etc.
}

// Video basic info for reel
export interface VideoBasic {
  id: string
  code: string
  title: string
  duration: number
  status: 'pending' | 'queued' | 'processing' | 'ready' | 'failed'
  thumbnailUrl?: string
}

// Template basic info
export interface ReelTemplateBasic {
  id: string
  name: string
}

// Full reel object
export interface Reel {
  id: string
  segmentStart: number
  segmentEnd: number
  coverTime: number // -1 = auto middle
  duration: number
  status: ReelStatus

  // NEW: Style-based fields
  style?: ReelStyle
  title: string
  line1?: string
  line2?: string
  showLogo: boolean

  // TTS (Text-to-Speech)
  ttsText?: string // ข้อความพากย์เสียง

  // Output
  outputUrl?: string
  thumbnailUrl?: string
  fileSize?: number
  exportError?: string
  exportedAt?: string

  // LEGACY: Layer-based fields (for backward compatibility)
  description?: string
  outputFormat?: OutputFormat
  videoFit?: VideoFit
  cropX?: number
  cropY?: number
  layers?: ReelLayer[]
  template?: ReelTemplateBasic

  // Relations
  video?: VideoBasic
  createdAt: string
  updatedAt: string
}

// Full template object
export interface ReelTemplate {
  id: string
  name: string
  description: string
  thumbnail?: string
  backgroundStyle?: string
  fontFamily?: string
  primaryColor?: string
  secondaryColor?: string
  defaultLayers: ReelLayer[]
  isActive: boolean
  sortOrder: number
  createdAt: string
}

// === Request DTOs ===

export interface CreateReelRequest {
  videoId: string
  segmentStart: number
  segmentEnd: number
  coverTime?: number // -1 = auto middle, or absolute time from video

  // NEW: Style-based fields (preferred)
  style?: ReelStyle
  title?: string
  line1?: string
  line2?: string
  showLogo?: boolean

  // TTS (Text-to-Speech)
  ttsText?: string // ข้อความพากย์เสียง (max 5000 chars)

  // LEGACY: Layer-based fields (deprecated but still supported)
  description?: string
  outputFormat?: OutputFormat
  videoFit?: VideoFit
  cropX?: number
  cropY?: number
  templateId?: string
  layers?: ReelLayer[]
}

export interface UpdateReelRequest {
  segmentStart?: number
  segmentEnd?: number
  coverTime?: number // -1 = auto middle, or absolute time from video

  // NEW: Style-based fields
  style?: ReelStyle
  title?: string
  line1?: string
  line2?: string
  showLogo?: boolean

  // TTS (Text-to-Speech)
  ttsText?: string // ข้อความพากย์เสียง (max 5000 chars)

  // LEGACY: Layer-based fields
  description?: string
  outputFormat?: OutputFormat
  videoFit?: VideoFit
  cropX?: number
  cropY?: number
  templateId?: string
  layers?: ReelLayer[]
}

export interface ReelFilterParams {
  videoId?: string
  status?: ReelStatus
  search?: string
  sortBy?: 'created_at' | 'updated_at' | 'title'
  sortOrder?: 'asc' | 'desc'
  page?: number
  limit?: number
}

// === Response DTOs ===

export interface ReelExportResponse {
  id: string
  status: ReelStatus
  message: string
}

// === UI State ===

export interface ReelEditorState {
  // Video segment
  segmentStart: number
  segmentEnd: number
  currentTime: number
  isPlaying: boolean

  // Layers
  layers: ReelLayer[]
  selectedLayerIndex: number | null

  // Template
  selectedTemplateId: string | null

  // Actions
  setSegment: (start: number, end: number) => void
  setCurrentTime: (time: number) => void
  setIsPlaying: (isPlaying: boolean) => void
  addLayer: (layer: ReelLayer) => void
  updateLayer: (index: number, layer: Partial<ReelLayer>) => void
  removeLayer: (index: number) => void
  selectLayer: (index: number | null) => void
  setTemplate: (templateId: string | null) => void
  setLayers: (layers: ReelLayer[]) => void
  reset: () => void
}
