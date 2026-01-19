import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { whitelistService } from './service'
import type {
  CreateWhitelistProfileRequest,
  UpdateWhitelistProfileRequest,
  AddDomainRequest,
  AdStatsFilterParams,
  RecordAdImpressionRequest,
} from './types'

// ==================== Query Keys ====================

export const whitelistKeys = {
  all: ['whitelist'] as const,
  profiles: () => [...whitelistKeys.all, 'profiles'] as const,
  profileList: (params?: { page?: number; limit?: number }) =>
    [...whitelistKeys.profiles(), 'list', params] as const,
  profileDetail: (id: string) => [...whitelistKeys.profiles(), 'detail', id] as const,

  // Ad Stats
  adStats: () => [...whitelistKeys.all, 'adStats'] as const,
  adStatsOverall: (params?: AdStatsFilterParams) =>
    [...whitelistKeys.adStats(), 'overall', params] as const,
  adStatsByProfile: (profileId: string, params?: AdStatsFilterParams) =>
    [...whitelistKeys.adStats(), 'profile', profileId, params] as const,
  deviceStats: (params?: AdStatsFilterParams) =>
    [...whitelistKeys.adStats(), 'devices', params] as const,
  profileRanking: (params?: AdStatsFilterParams) =>
    [...whitelistKeys.adStats(), 'ranking', params] as const,
  skipDistribution: (params?: AdStatsFilterParams) =>
    [...whitelistKeys.adStats(), 'skipDistribution', params] as const,

  // Embed Config
  embedConfig: () => [...whitelistKeys.all, 'embedConfig'] as const,
}

// ==================== Profile Queries ====================

export function useWhitelistProfiles(params?: { page?: number; limit?: number }) {
  return useQuery({
    queryKey: whitelistKeys.profileList(params),
    queryFn: () => whitelistService.getProfiles(params),
  })
}

export function useWhitelistProfile(id: string) {
  return useQuery({
    queryKey: whitelistKeys.profileDetail(id),
    queryFn: () => whitelistService.getProfileById(id),
    enabled: !!id,
  })
}

// ==================== Profile Mutations ====================

export function useCreateProfile() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: CreateWhitelistProfileRequest) => whitelistService.createProfile(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: whitelistKeys.profiles() })
    },
  })
}

export function useUpdateProfile() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateWhitelistProfileRequest }) =>
      whitelistService.updateProfile(id, data),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: whitelistKeys.profiles() })
      queryClient.invalidateQueries({ queryKey: whitelistKeys.profileDetail(id) })
    },
  })
}

export function useDeleteProfile() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => whitelistService.deleteProfile(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: whitelistKeys.profiles() })
    },
  })
}

// ==================== Domain Mutations ====================

export function useAddDomain() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ profileId, data }: { profileId: string; data: AddDomainRequest }) =>
      whitelistService.addDomain(profileId, data),
    onSuccess: (_, { profileId }) => {
      queryClient.invalidateQueries({ queryKey: whitelistKeys.profileDetail(profileId) })
      queryClient.invalidateQueries({ queryKey: whitelistKeys.profiles() })
    },
  })
}

export function useRemoveDomain() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (domainId: string) => whitelistService.removeDomain(domainId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: whitelistKeys.profiles() })
    },
  })
}

// ==================== Ad Stats Queries ====================

export function useAdStats(params?: AdStatsFilterParams) {
  return useQuery({
    queryKey: whitelistKeys.adStatsOverall(params),
    queryFn: () => whitelistService.getAdStats(params),
  })
}

export function useAdStatsByProfile(profileId: string, params?: AdStatsFilterParams) {
  return useQuery({
    queryKey: whitelistKeys.adStatsByProfile(profileId, params),
    queryFn: () => whitelistService.getAdStatsByProfile(profileId, params),
    enabled: !!profileId,
  })
}

export function useDeviceStats(params?: AdStatsFilterParams) {
  return useQuery({
    queryKey: whitelistKeys.deviceStats(params),
    queryFn: () => whitelistService.getDeviceStats(params),
  })
}

export function useProfileRanking(params?: AdStatsFilterParams) {
  return useQuery({
    queryKey: whitelistKeys.profileRanking(params),
    queryFn: () => whitelistService.getProfileRanking(params),
  })
}

export function useSkipTimeDistribution(params?: AdStatsFilterParams) {
  return useQuery({
    queryKey: whitelistKeys.skipDistribution(params),
    queryFn: () => whitelistService.getSkipTimeDistribution(params),
  })
}

// ==================== Embed Config ====================

export function useEmbedConfig() {
  return useQuery({
    queryKey: whitelistKeys.embedConfig(),
    queryFn: () => whitelistService.getEmbedConfig(),
    retry: false, // Don't retry on 403 (domain not allowed)
    staleTime: 5 * 60 * 1000, // Cache for 5 minutes
  })
}

// ==================== Ad Impression Recording ====================

export function useRecordAdImpression() {
  return useMutation({
    mutationFn: (data: RecordAdImpressionRequest) => whitelistService.recordAdImpression(data),
    // ไม่ต้อง invalidate queries เพราะเป็นการบันทึก stat
  })
}

// ==================== Cache Management ====================

export function useClearAllCache() {
  return useMutation({
    mutationFn: () => whitelistService.clearAllCache(),
  })
}

export function useClearDomainCache() {
  return useMutation({
    mutationFn: (domain: string) => whitelistService.clearDomainCache(domain),
  })
}
