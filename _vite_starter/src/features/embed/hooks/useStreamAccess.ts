import { useQuery } from '@tanstack/react-query'
import { apiClient } from '@/lib/api-client'

// Types
interface StreamAccessResponse {
  video_id: string
  video_code: string
  title: string
  playlist_url: string
  token: string
  expires_at: number // Unix timestamp
  cdn_base_url: string
}

// Query key factory
export const streamAccessKeys = {
  all: ['stream-access'] as const,
  byCode: (code: string) => [...streamAccessKeys.all, code] as const,
}

/**
 * Hook สำหรับขอ HLS access token จาก server
 * Token จะใช้สำหรับเข้าถึง HLS files ผ่าน Cloudflare CDN
 */
export function useStreamAccess(videoCode: string, options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: streamAccessKeys.byCode(videoCode),
    queryFn: async () => {
      return apiClient.get<StreamAccessResponse>(`/api/v1/hls/${videoCode}/access`)
    },
    enabled: !!videoCode && (options?.enabled ?? true),
    // Token หมดอายุ 4 ชม. ดังนั้น cache ไว้ 3 ชม.
    staleTime: 3 * 60 * 60 * 1000, // 3 hours
    gcTime: 3.5 * 60 * 60 * 1000, // 3.5 hours
    retry: 2,
    refetchOnWindowFocus: false,
  })
}

/**
 * Calculate time remaining until token expires
 * @returns milliseconds until expiration, or 0 if already expired
 */
export function getTokenTimeRemaining(expiresAt: number): number {
  const now = Math.floor(Date.now() / 1000)
  const remaining = expiresAt - now
  return remaining > 0 ? remaining * 1000 : 0
}

/**
 * Check if token needs refresh (less than 1 hour remaining)
 */
export function shouldRefreshToken(expiresAt: number): boolean {
  const remaining = getTokenTimeRemaining(expiresAt)
  const oneHour = 60 * 60 * 1000 // 1 hour in ms
  return remaining > 0 && remaining < oneHour
}
