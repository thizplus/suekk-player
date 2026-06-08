import { apiClient } from '@/lib/api-client'
import { SERIES_ROUTES } from '@/constants/api-routes'
import type {
  Series,
  SeriesCategory,
  SeriesEpisode,
  SeriesFilterParams,
  CreateSeriesRequest,
  UpdateSeriesRequest,
  AddEpisodesRequest,
  UpdateEpisodeRequest,
  CreateCategoryRequest,
} from './types'

export const seriesService = {
  // ═══ Series ═══

  async getList(params?: SeriesFilterParams) {
    return apiClient.getPaginated<Series>(SERIES_ROUTES.LIST, { params })
  },

  async getByCode(code: string): Promise<Series> {
    return apiClient.get<Series>(SERIES_ROUTES.BY_CODE(code))
  },

  async getBySlug(slug: string): Promise<Series> {
    return apiClient.get<Series>(SERIES_ROUTES.BY_SLUG(slug))
  },

  async getById(id: string): Promise<Series> {
    return apiClient.get<Series>(SERIES_ROUTES.BY_ID(id))
  },

  async create(data: CreateSeriesRequest): Promise<Series> {
    return apiClient.post<Series>(SERIES_ROUTES.LIST, data)
  },

  async update(id: string, data: UpdateSeriesRequest): Promise<Series> {
    return apiClient.put<Series>(SERIES_ROUTES.BY_ID(id), data)
  },

  async delete(id: string): Promise<void> {
    await apiClient.delete(SERIES_ROUTES.BY_ID(id))
  },

  async upsert(data: CreateSeriesRequest): Promise<Series> {
    return apiClient.post<Series>(SERIES_ROUTES.UPSERT, data)
  },

  // ═══ Episodes ═══

  async getEpisodes(seriesId: string): Promise<SeriesEpisode[]> {
    return apiClient.get<SeriesEpisode[]>(SERIES_ROUTES.EPISODES(seriesId))
  },

  async addEpisodes(seriesId: string, data: AddEpisodesRequest): Promise<{ newEpisodes: number }> {
    return apiClient.post<{ newEpisodes: number }>(SERIES_ROUTES.EPISODES(seriesId), data)
  },

  async updateEpisode(seriesId: string, episodeNumber: number, data: UpdateEpisodeRequest): Promise<SeriesEpisode> {
    return apiClient.patch<SeriesEpisode>(SERIES_ROUTES.EPISODE(seriesId, episodeNumber), data)
  },

  // ═══ Categories ═══

  async getCategories(): Promise<SeriesCategory[]> {
    return apiClient.get<SeriesCategory[]>(SERIES_ROUTES.CATEGORIES)
  },

  async createCategory(data: CreateCategoryRequest): Promise<SeriesCategory> {
    return apiClient.post<SeriesCategory>(SERIES_ROUTES.CATEGORIES, data)
  },
}
