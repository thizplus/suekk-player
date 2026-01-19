import { useCallback, useRef } from 'react'
import { apiClient } from '@/lib/api-client'
import { WHITELIST_ROUTES } from '@/constants/api-routes'
import type { RecordAdImpressionRequest } from '@/features/whitelist'

interface TrackAdParams {
  profileId: string
  videoCode: string
  domain: string
  adUrl: string
  adDuration: number
}

/**
 * Hook สำหรับ track ad impressions
 * Track ทั้ง complete และ skip events
 */
export function useAdTracking({ profileId, videoCode, domain, adUrl, adDuration }: TrackAdParams) {
  const trackedRef = useRef(false)
  const startTimeRef = useRef<number>(0)

  // Record start time when ad begins
  const trackStart = useCallback(() => {
    startTimeRef.current = Date.now()
  }, [])

  // Record impression when ad completes
  const trackComplete = useCallback(async () => {
    if (trackedRef.current) return
    trackedRef.current = true

    const watchDuration = (Date.now() - startTimeRef.current) / 1000

    const data: RecordAdImpressionRequest = {
      profileId,
      videoCode,
      domain,
      adUrl,
      adDuration,
      watchDuration,
      completed: true,
      skipped: false,
    }

    try {
      await apiClient.post(WHITELIST_ROUTES.AD_IMPRESSION, data)
    } catch (err) {
      console.warn('Failed to track ad completion:', err)
    }
  }, [profileId, videoCode, domain, adUrl, adDuration])

  // Record impression when ad is skipped
  const trackSkip = useCallback(async (skippedAt: number) => {
    if (trackedRef.current) return
    trackedRef.current = true

    const watchDuration = (Date.now() - startTimeRef.current) / 1000

    const data: RecordAdImpressionRequest = {
      profileId,
      videoCode,
      domain,
      adUrl,
      adDuration,
      watchDuration,
      completed: false,
      skipped: true,
      skippedAt,
    }

    try {
      await apiClient.post(WHITELIST_ROUTES.AD_IMPRESSION, data)
    } catch (err) {
      console.warn('Failed to track ad skip:', err)
    }
  }, [profileId, videoCode, domain, adUrl, adDuration])

  // Record error during ad playback
  const trackError = useCallback(async () => {
    if (trackedRef.current) return
    trackedRef.current = true

    const watchDuration = (Date.now() - startTimeRef.current) / 1000

    const data: RecordAdImpressionRequest = {
      profileId,
      videoCode,
      domain,
      adUrl,
      adDuration,
      watchDuration,
      completed: false,
      skipped: false,
      errorOccurred: true,
    }

    try {
      await apiClient.post(WHITELIST_ROUTES.AD_IMPRESSION, data)
    } catch (err) {
      console.warn('Failed to track ad error:', err)
    }
  }, [profileId, videoCode, domain, adUrl, adDuration])

  const resetTracking = useCallback(() => {
    trackedRef.current = false
    startTimeRef.current = 0
  }, [])

  return {
    trackStart,
    trackComplete,
    trackSkip,
    trackError,
    resetTracking,
  }
}

/**
 * Detect device type based on screen width
 */
export function getDeviceType(): 'desktop' | 'mobile' {
  return window.innerWidth < 768 ? 'mobile' : 'desktop'
}

/**
 * Get current domain from referrer (for iframe embed)
 */
export function getCurrentDomain(): string {
  // ถ้าอยู่ใน iframe ใช้ referrer
  if (window !== window.parent && document.referrer) {
    try {
      const url = new URL(document.referrer)
      return url.hostname
    } catch {
      return window.location.hostname
    }
  }
  return window.location.hostname
}
