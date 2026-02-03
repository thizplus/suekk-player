// Reel Generator Types

export type ReelStatus = 'draft' | 'exporting' | 'ready' | 'failed'
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
  title: string
  description: string
  segmentStart: number
  segmentEnd: number
  duration: number
  status: ReelStatus
  outputUrl?: string
  thumbnailUrl?: string
  fileSize?: number
  exportError?: string
  exportedAt?: string
  layers: ReelLayer[]
  video?: VideoBasic
  template?: ReelTemplateBasic
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
  title?: string
  description?: string
  segmentStart: number
  segmentEnd: number
  templateId?: string
  layers?: ReelLayer[]
}

export interface UpdateReelRequest {
  title?: string
  description?: string
  segmentStart?: number
  segmentEnd?: number
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
