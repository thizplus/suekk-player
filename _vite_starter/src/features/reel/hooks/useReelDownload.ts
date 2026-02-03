import { useState, useCallback } from 'react'
import { useStreamAccess } from '@/features/embed/hooks'
import { toast } from 'sonner'

const CDN_BASE_URL = import.meta.env.VITE_CDN_BASE_URL || ''

interface UseReelDownloadOptions {
  videoCode: string
  reelId: string
  title?: string
}

/**
 * Hook สำหรับดาวน์โหลด reel output ผ่าน authenticated CDN
 */
export function useReelDownload({ videoCode, reelId, title }: UseReelDownloadOptions) {
  const [isDownloading, setIsDownloading] = useState(false)
  const { data: streamAccess } = useStreamAccess(videoCode)

  const getReelUrl = useCallback(() => {
    return `${CDN_BASE_URL}/reels/${videoCode}/${reelId}.mp4`
  }, [videoCode, reelId])

  const getThumbnailUrl = useCallback(() => {
    return `${CDN_BASE_URL}/reels/${videoCode}/${reelId}_thumb.jpg`
  }, [videoCode, reelId])

  const downloadReel = useCallback(async () => {
    if (!streamAccess?.token) {
      toast.error('ไม่สามารถดาวน์โหลดได้ กรุณาลองใหม่')
      return
    }

    setIsDownloading(true)
    try {
      const url = getReelUrl()
      const response = await fetch(url, {
        headers: {
          'X-Stream-Token': streamAccess.token,
        },
      })

      if (!response.ok) {
        throw new Error('Failed to fetch reel')
      }

      const blob = await response.blob()
      const blobUrl = URL.createObjectURL(blob)

      // Trigger download
      const a = document.createElement('a')
      a.href = blobUrl
      a.download = `${title || 'reel'}.mp4`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)

      // Cleanup
      setTimeout(() => URL.revokeObjectURL(blobUrl), 1000)

      toast.success('ดาวน์โหลดสำเร็จ')
    } catch (error) {
      console.error('Download error:', error)
      toast.error('ดาวน์โหลดไม่สำเร็จ')
    } finally {
      setIsDownloading(false)
    }
  }, [streamAccess?.token, getReelUrl, title])

  const openReel = useCallback(async () => {
    if (!streamAccess?.token) {
      toast.error('ไม่สามารถเปิดได้ กรุณาลองใหม่')
      return
    }

    setIsDownloading(true)
    try {
      const url = getReelUrl()
      const response = await fetch(url, {
        headers: {
          'X-Stream-Token': streamAccess.token,
        },
      })

      if (!response.ok) {
        throw new Error('Failed to fetch reel')
      }

      const blob = await response.blob()
      const blobUrl = URL.createObjectURL(blob)

      // Open in new tab
      window.open(blobUrl, '_blank')

      // Don't revoke immediately - user needs time to view
    } catch (error) {
      console.error('Open error:', error)
      toast.error('เปิดไม่สำเร็จ')
    } finally {
      setIsDownloading(false)
    }
  }, [streamAccess?.token, getReelUrl])

  return {
    isDownloading,
    downloadReel,
    openReel,
    getReelUrl,
    getThumbnailUrl,
    hasToken: !!streamAccess?.token,
  }
}
