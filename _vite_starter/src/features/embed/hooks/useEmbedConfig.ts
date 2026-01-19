import { useQuery } from '@tanstack/react-query'
import { apiClient } from '@/lib/api-client'
import { WHITELIST_ROUTES } from '@/constants/api-routes'
import type { EmbedConfig } from '@/features/whitelist'

// Query key factory
export const embedKeys = {
  all: ['embed'] as const,
  config: () => [...embedKeys.all, 'config'] as const,
}

/**
 * Hook สำหรับดึง embed config จาก server
 * Config จะถูก determine จาก Origin/Referer header โดย server
 */
export function useEmbedConfig() {
  return useQuery({
    queryKey: embedKeys.config(),
    queryFn: async () => {
      return apiClient.get<EmbedConfig>(WHITELIST_ROUTES.EMBED_CONFIG)
    },
    // Cache config ไว้ไม่ต้อง refetch บ่อย
    staleTime: 5 * 60 * 1000, // 5 minutes
    gcTime: 10 * 60 * 1000, // 10 minutes
    retry: 1,
  })
}
