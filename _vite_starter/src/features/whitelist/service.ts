import { apiClient, type PaginationMeta } from '@/lib/api-client'
import { WHITELIST_ROUTES } from '@/constants/api-routes'
import type {
  WhitelistProfile,
  ProfileDomain,
  PrerollAd,
  CreateWhitelistProfileRequest,
  UpdateWhitelistProfileRequest,
  AddDomainRequest,
  AddPrerollAdRequest,
  UpdatePrerollAdRequest,
  ReorderPrerollAdsRequest,
  AdImpressionStats,
  DeviceStats,
  ProfileRanking,
  AdStatsFilterParams,
  EmbedConfig,
  RecordAdImpressionRequest,
} from './types'

export const whitelistService = {
  // ==================== Profile Management ====================

  async getProfiles(params?: { page?: number; limit?: number }): Promise<{
    data: WhitelistProfile[]
    meta: PaginationMeta
  }> {
    return apiClient.getPaginated<WhitelistProfile>(WHITELIST_ROUTES.PROFILES, { params })
  },

  async getProfileById(id: string): Promise<WhitelistProfile> {
    return apiClient.get<WhitelistProfile>(WHITELIST_ROUTES.PROFILE_BY_ID(id))
  },

  async createProfile(data: CreateWhitelistProfileRequest): Promise<WhitelistProfile> {
    return apiClient.post<WhitelistProfile>(WHITELIST_ROUTES.PROFILES, data)
  },

  async updateProfile(id: string, data: UpdateWhitelistProfileRequest): Promise<WhitelistProfile> {
    return apiClient.put<WhitelistProfile>(WHITELIST_ROUTES.PROFILE_BY_ID(id), data)
  },

  async deleteProfile(id: string): Promise<void> {
    await apiClient.delete(WHITELIST_ROUTES.PROFILE_BY_ID(id))
  },

  // ==================== Domain Management ====================

  async addDomain(profileId: string, data: AddDomainRequest): Promise<ProfileDomain> {
    return apiClient.post<ProfileDomain>(WHITELIST_ROUTES.PROFILE_DOMAINS(profileId), data)
  },

  async removeDomain(domainId: string): Promise<void> {
    await apiClient.delete(WHITELIST_ROUTES.DOMAIN_BY_ID(domainId))
  },

  // ==================== Preroll Ads Management ====================

  async getPrerollAds(profileId: string): Promise<PrerollAd[]> {
    return apiClient.get<PrerollAd[]>(WHITELIST_ROUTES.PROFILE_PREROLLS(profileId))
  },

  async addPrerollAd(profileId: string, data: AddPrerollAdRequest): Promise<PrerollAd> {
    return apiClient.post<PrerollAd>(WHITELIST_ROUTES.PROFILE_PREROLLS(profileId), data)
  },

  async updatePrerollAd(prerollId: string, data: UpdatePrerollAdRequest): Promise<PrerollAd> {
    return apiClient.put<PrerollAd>(WHITELIST_ROUTES.PREROLL_BY_ID(prerollId), data)
  },

  async deletePrerollAd(prerollId: string): Promise<void> {
    await apiClient.delete(WHITELIST_ROUTES.PREROLL_BY_ID(prerollId))
  },

  async reorderPrerollAds(profileId: string, data: ReorderPrerollAdsRequest): Promise<PrerollAd[]> {
    return apiClient.put<PrerollAd[]>(WHITELIST_ROUTES.PROFILE_PREROLLS_REORDER(profileId), data)
  },

  // ==================== Ad Statistics ====================

  async getAdStats(params?: AdStatsFilterParams): Promise<AdImpressionStats> {
    return apiClient.get<AdImpressionStats>(WHITELIST_ROUTES.AD_STATS, { params })
  },

  async getAdStatsByProfile(profileId: string, params?: AdStatsFilterParams): Promise<AdImpressionStats> {
    return apiClient.get<AdImpressionStats>(WHITELIST_ROUTES.AD_STATS_BY_PROFILE(profileId), { params })
  },

  async getDeviceStats(params?: AdStatsFilterParams): Promise<DeviceStats> {
    return apiClient.get<DeviceStats>(WHITELIST_ROUTES.AD_STATS_DEVICES, { params })
  },

  async getProfileRanking(params?: AdStatsFilterParams): Promise<ProfileRanking[]> {
    return apiClient.get<ProfileRanking[]>(WHITELIST_ROUTES.AD_STATS_RANKING, { params })
  },

  async getSkipTimeDistribution(params?: AdStatsFilterParams): Promise<Record<number, number>> {
    return apiClient.get<Record<number, number>>(WHITELIST_ROUTES.AD_STATS_SKIP_DISTRIBUTION, { params })
  },

  // ==================== Cache Management ====================

  async clearAllCache(): Promise<{ message: string; deletedKeys: number }> {
    return apiClient.post<{ message: string; deletedKeys: number }>(WHITELIST_ROUTES.CACHE_CLEAR_ALL)
  },

  async clearDomainCache(domain: string): Promise<{ message: string; domain: string }> {
    return apiClient.deleteWithResponse<{ message: string; domain: string }>(WHITELIST_ROUTES.CACHE_CLEAR_DOMAIN(domain))
  },

  // ==================== Public Endpoints (for Embed Player) ====================

  async recordAdImpression(data: RecordAdImpressionRequest): Promise<void> {
    await apiClient.post(WHITELIST_ROUTES.AD_IMPRESSION, data)
  },

  async getEmbedConfig(): Promise<EmbedConfig> {
    return apiClient.get<EmbedConfig>(WHITELIST_ROUTES.EMBED_CONFIG)
  },
}
