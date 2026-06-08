// ═══════════════════════════════════════════
// Series Types
// ═══════════════════════════════════════════

export interface Series {
  id: string
  code: string
  title: string
  thaiTitle: string
  slug: string
  description: string
  posterPath: string
  year: number
  rating: number
  quality: string
  audioType: string
  trailerYoutubeId: string
  totalEpisodes: number
  isCompleted: boolean
  platforms: string[]
  genres: string[]
  status: string
  category: SeriesCategory | null
  episodes: SeriesEpisode[]
  createdAt: string
  updatedAt: string
}

export interface SeriesEpisode {
  id: string
  episodeNumber: number
  videoCode: string
  videoStatus: string
  status: string
  createdAt: string
}

export interface SeriesCategory {
  id: string
  name: string
  slug: string
  parentId: string | null
  seriesCount?: number
  children?: SeriesCategory[]
}

// ═══════════════════════════════════════════
// Requests
// ═══════════════════════════════════════════

export interface CreateSeriesRequest {
  title: string
  thaiTitle?: string
  slug: string
  description?: string
  year?: number
  rating?: number
  quality?: string
  audioType?: string
  trailerYoutubeId?: string
  totalEpisodes?: number
  isCompleted?: boolean
  categoryId?: string
  sourceSite?: string
  sourceId?: number
  sourceUrl?: string
}

export interface UpdateSeriesRequest {
  title?: string
  thaiTitle?: string
  description?: string
  year?: number
  rating?: number
  quality?: string
  audioType?: string
  trailerYoutubeId?: string
  totalEpisodes?: number
  isCompleted?: boolean
  categoryId?: string
  status?: string
  posterPath?: string
}

export interface SeriesFilterParams {
  search?: string
  categoryId?: string
  audioType?: string
  year?: number
  status?: string
  sortBy?: string
  sortOrder?: string
  page?: number
  limit?: number
}

export interface AddEpisodesRequest {
  episodes: { episodeNumber: number; sourceUrl?: string }[]
}

export interface UpdateEpisodeRequest {
  videoId?: string
  status?: string
  sourceUrl?: string
}

// ═══════════════════════════════════════════
// Responses
// ═══════════════════════════════════════════

export interface SeriesListResponse {
  series: Series[]
}

export interface CreateCategoryRequest {
  name: string
  slug: string
  parentId?: string
}
