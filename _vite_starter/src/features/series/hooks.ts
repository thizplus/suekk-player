import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { seriesService } from './service'
import type {
  SeriesFilterParams,
  CreateSeriesRequest,
  UpdateSeriesRequest,
  AddEpisodesRequest,
  UpdateEpisodeRequest,
  CreateCategoryRequest,
} from './types'

// ═══════════════════════════════════════════
// Query Key Factory
// ═══════════════════════════════════════════

export const seriesKeys = {
  all: ['series'] as const,
  list: (params?: SeriesFilterParams) => [...seriesKeys.all, 'list', params] as const,
  detail: (id: string) => [...seriesKeys.all, 'detail', id] as const,
  detailByCode: (code: string) => [...seriesKeys.all, 'code', code] as const,
  episodes: (seriesId: string) => [...seriesKeys.all, 'episodes', seriesId] as const,
  categories: () => [...seriesKeys.all, 'categories'] as const,
}

// ═══════════════════════════════════════════
// Series Queries
// ═══════════════════════════════════════════

export function useSeriesList(params?: SeriesFilterParams) {
  return useQuery({
    queryKey: seriesKeys.list(params),
    queryFn: () => seriesService.getList(params),
  })
}

export function useSeriesDetail(id: string) {
  return useQuery({
    queryKey: seriesKeys.detail(id),
    queryFn: () => seriesService.getById(id),
    enabled: !!id,
  })
}

export function useSeriesByCode(code: string) {
  return useQuery({
    queryKey: seriesKeys.detailByCode(code),
    queryFn: () => seriesService.getByCode(code),
    enabled: !!code,
  })
}

// ═══════════════════════════════════════════
// Series Mutations
// ═══════════════════════════════════════════

export function useCreateSeries() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: CreateSeriesRequest) => seriesService.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: seriesKeys.all })
    },
  })
}

export function useUpdateSeries() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateSeriesRequest }) =>
      seriesService.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: seriesKeys.all })
    },
  })
}

export function useDeleteSeries() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => seriesService.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: seriesKeys.all })
    },
  })
}

// ═══════════════════════════════════════════
// Episode Mutations
// ═══════════════════════════════════════════

export function useAddEpisodes() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ seriesId, data }: { seriesId: string; data: AddEpisodesRequest }) =>
      seriesService.addEpisodes(seriesId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: seriesKeys.all })
    },
  })
}

export function useUpdateEpisode() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ seriesId, episodeNumber, data }: { seriesId: string; episodeNumber: number; data: UpdateEpisodeRequest }) =>
      seriesService.updateEpisode(seriesId, episodeNumber, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: seriesKeys.all })
    },
  })
}

// ═══════════════════════════════════════════
// Category Queries & Mutations
// ═══════════════════════════════════════════

export function useSeriesCategories() {
  return useQuery({
    queryKey: seriesKeys.categories(),
    queryFn: () => seriesService.getCategories(),
  })
}

export function useCreateSeriesCategory() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: CreateCategoryRequest) => seriesService.createCategory(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: seriesKeys.categories() })
    },
  })
}
