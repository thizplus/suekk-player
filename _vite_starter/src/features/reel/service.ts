import { apiClient, type PaginationMeta } from '@/lib/api-client'
import { REEL_ROUTES } from '@/constants/api-routes'
import type {
  Reel,
  ReelTemplate,
  CreateReelRequest,
  UpdateReelRequest,
  ReelFilterParams,
  ReelExportResponse,
} from './types'

export const reelService = {
  // === Templates ===

  async getTemplates(): Promise<ReelTemplate[]> {
    return apiClient.get<ReelTemplate[]>(REEL_ROUTES.TEMPLATES)
  },

  async getTemplateById(id: string): Promise<ReelTemplate> {
    return apiClient.get<ReelTemplate>(REEL_ROUTES.TEMPLATE_BY_ID(id))
  },

  // === Reels CRUD ===

  async create(data: CreateReelRequest): Promise<Reel> {
    return apiClient.post<Reel>(REEL_ROUTES.CREATE, data)
  },

  async getById(id: string): Promise<Reel> {
    return apiClient.get<Reel>(REEL_ROUTES.BY_ID(id))
  },

  async update(id: string, data: UpdateReelRequest): Promise<Reel> {
    return apiClient.put<Reel>(REEL_ROUTES.UPDATE(id), data)
  },

  async delete(id: string): Promise<void> {
    return apiClient.delete(REEL_ROUTES.DELETE(id))
  },

  // === List ===

  async list(params?: ReelFilterParams): Promise<{ data: Reel[]; meta: PaginationMeta }> {
    return apiClient.getPaginated<Reel>(REEL_ROUTES.LIST, { params })
  },

  async listByVideo(videoId: string, page = 1, limit = 20): Promise<{ data: Reel[]; meta: PaginationMeta }> {
    return apiClient.getPaginated<Reel>(REEL_ROUTES.BY_VIDEO(videoId), {
      params: { page, limit },
    })
  },

  // === Export ===

  async export(id: string): Promise<ReelExportResponse> {
    return apiClient.post<ReelExportResponse>(REEL_ROUTES.EXPORT(id))
  },
}
