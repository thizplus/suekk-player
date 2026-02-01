import { useState, useEffect, useMemo } from 'react'
import { useParams } from 'react-router-dom'
import { useVideoByCode, VideoPlayer } from '@/features/video'
import { useSubtitlesByCode } from '@/features/subtitle'
import { Loader2 } from 'lucide-react'
import { APP_CONFIG } from '@/constants/app-config'
import { LANGUAGE_LABELS } from '@/constants/enums'
import { useStreamAccess } from '../hooks/useStreamAccess'
import './embed.css'

/**
 * PreviewPage - หน้า preview วิดีโอแบบไม่มี ads/watermark
 * ใช้สำหรับ admin ดู preview จาก VideoDetailSheet
 */
export function PreviewPage() {
  const { code } = useParams<{ code: string }>()

  // ดึงข้อมูล video จาก API โดยใช้ code
  const { data: video, isLoading: videoLoading, error: videoError } = useVideoByCode(code || '')

  // ดึง HLS access token (JWT) สำหรับเล่นวิดีโอ
  const { data: streamAccess, isLoading: streamLoading } = useStreamAccess(code || '', {
    enabled: !!code && !!video && video.status === 'ready',
  })

  // ดึง subtitles สำหรับวิดีโอ
  const { data: subtitleData, isLoading: subtitleLoading } = useSubtitlesByCode(code || '', {
    enabled: !!code && !!video && video.status === 'ready',
  })

  // State สำหรับ subtitle blob URLs
  const [subtitleBlobUrls, setSubtitleBlobUrls] = useState<Record<string, string>>({})

  // Track when subtitle blobs are ready (to prevent player recreation)
  const [subtitlesReady, setSubtitlesReady] = useState(false)

  // State สำหรับ thumbnail blob URL
  const [thumbnailBlobUrl, setThumbnailBlobUrl] = useState<string | undefined>()

  // Fetch subtitles ด้วย token แล้วสร้าง Blob URLs
  // ต้องรอให้เสร็จก่อนแสดง player เพื่อไม่ให้ player ถูก recreate
  useEffect(() => {
    // รอจนกว่า streamAccess และ subtitle query จะพร้อม
    if (!streamAccess?.token || subtitleLoading) return

    // ถ้าไม่มี subtitle data หรือไม่มี ready subtitles → พร้อมแสดง player เลย
    const readySubtitles = subtitleData?.subtitles?.filter(
      sub => sub.status === 'ready' && sub.srtPath
    ) || []

    if (readySubtitles.length === 0) {
      setSubtitlesReady(true)
      return
    }

    // มี subtitles → ต้อง fetch blob ก่อน
    const blobUrls: Record<string, string> = {}
    const fetchPromises = readySubtitles.map(async (sub) => {
      try {
        const url = `${APP_CONFIG.cdnUrl}/${sub.srtPath}`
        const response = await fetch(url, {
          headers: {
            'X-Stream-Token': streamAccess.token,
          },
        })

        if (!response.ok) return

        const blob = await response.blob()
        blobUrls[sub.language] = URL.createObjectURL(blob)
      } catch {
        // Ignore errors
      }
    })

    Promise.all(fetchPromises).then(() => {
      setSubtitleBlobUrls(blobUrls)
      setSubtitlesReady(true) // Mark ready AFTER blobs are fetched
    })

    return () => {
      Object.values(blobUrls).forEach(url => URL.revokeObjectURL(url))
    }
  }, [subtitleData, subtitleLoading, streamAccess?.token])

  // Fetch thumbnail ด้วย token
  useEffect(() => {
    if (!video?.code || !streamAccess?.token) return

    let blobUrl: string | undefined

    const fetchThumbnail = async () => {
      try {
        const url = `${APP_CONFIG.streamUrl}/${video.code}/thumb.jpg`
        const response = await fetch(url, {
          headers: { 'X-Stream-Token': streamAccess.token },
        })

        if (!response.ok) return

        const blob = await response.blob()
        blobUrl = URL.createObjectURL(blob)
        setThumbnailBlobUrl(blobUrl)
      } catch {
        // Ignore errors
      }
    }

    fetchThumbnail()

    return () => {
      if (blobUrl) URL.revokeObjectURL(blobUrl)
    }
  }, [video?.code, streamAccess?.token])

  // Build subtitle options
  const subtitleOptions = useMemo(() => {
    if (!subtitleData?.subtitles) return []

    return subtitleData.subtitles
      .filter(sub => sub.status === 'ready' && sub.srtPath && subtitleBlobUrls[sub.language])
      .map(sub => ({
        url: subtitleBlobUrls[sub.language],
        name: LANGUAGE_LABELS[sub.language] || sub.language,
        language: sub.language,
        default: sub.language === 'th',
      }))
  }, [subtitleData, subtitleBlobUrls])

  // Loading state - รอให้ subtitle blobs พร้อมก่อนแสดง player
  if (videoLoading || streamLoading || !subtitlesReady) {
    return (
      <div className="embed-container embed-center">
        <Loader2 className="h-8 w-8 animate-spin text-white" />
      </div>
    )
  }

  // Error state
  if (videoError || !video) {
    return (
      <div className="embed-container embed-center">
        <p className="text-white text-lg">Video not available</p>
      </div>
    )
  }

  // Video not ready
  if (video.status !== 'ready') {
    return (
      <div className="embed-container embed-center">
        <div className="text-center">
          <Loader2 className="h-8 w-8 animate-spin text-white mx-auto mb-4" />
          <p className="text-white text-lg">Video is being processed...</p>
        </div>
      </div>
    )
  }

  const hlsUrl = `${APP_CONFIG.streamUrl}/${video.code}/master.m3u8`

  return (
    <div className="embed-container">
      <VideoPlayer
        src={hlsUrl}
        poster={thumbnailBlobUrl}
        streamToken={streamAccess.token}
        subtitles={subtitleOptions}
      />
    </div>
  )
}
