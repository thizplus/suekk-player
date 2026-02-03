import { useState, useEffect } from 'react'
import { APP_CONFIG } from '@/constants/app-config'

interface UseThumbnailBlobOptions {
  videoCode: string | undefined
  streamToken: string | undefined
}

/**
 * Hook สำหรับ fetch thumbnail จาก CDN ด้วย stream token แล้วสร้าง Blob URL
 */
export function useThumbnailBlob({ videoCode, streamToken }: UseThumbnailBlobOptions) {
  const [thumbnailBlobUrl, setThumbnailBlobUrl] = useState<string | undefined>()
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | undefined>()

  useEffect(() => {
    if (!videoCode || !streamToken) {
      setThumbnailBlobUrl(undefined)
      setError(undefined)
      return
    }

    let isMounted = true
    let blobUrl: string | undefined

    const fetchThumbnail = async () => {
      setIsLoading(true)
      setError(undefined)

      try {
        const url = `${APP_CONFIG.streamUrl}/${videoCode}/thumb.jpg`
        const response = await fetch(url, {
          headers: {
            'X-Stream-Token': streamToken,
          },
        })

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`)
        }

        const blob = await response.blob()
        blobUrl = URL.createObjectURL(blob)

        if (isMounted) {
          setThumbnailBlobUrl(blobUrl)
        }
      } catch (err) {
        console.warn('[useThumbnailBlob] Error:', err)
        if (isMounted) {
          setError(err instanceof Error ? err.message : 'Failed to load thumbnail')
        }
      } finally {
        if (isMounted) {
          setIsLoading(false)
        }
      }
    }

    fetchThumbnail()

    return () => {
      isMounted = false
      if (blobUrl) {
        URL.revokeObjectURL(blobUrl)
      }
    }
  }, [videoCode, streamToken])

  return { thumbnailBlobUrl, isLoading, error }
}
