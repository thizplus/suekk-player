import { useState, useEffect, useCallback, useMemo } from 'react'
import { useParams } from 'react-router-dom'
import { useVideoByCode, VideoPlayer } from '@/features/video'
import { useSubtitlesByCode } from '@/features/subtitle'
import { Loader2 } from 'lucide-react'
import { APP_CONFIG } from '@/constants/app-config'
import { LANGUAGE_LABELS } from '@/constants/enums'
import { Watermark } from '../components/Watermark'
import { PrerollPlayer } from '../components/PrerollPlayer'
import { useEmbedConfig } from '../hooks/useEmbedConfig'
import { useStreamAccess } from '../hooks/useStreamAccess'
import { getDeviceType } from '../hooks/useAdTracking'
// TODO: เปิดคืนหลัง debug เสร็จ
// import { useAntiDevTools } from '../hooks/useAntiDevTools'
import './embed.css'

type EmbedPhase = 'loading' | 'preroll' | 'main' | 'blocked' | 'not-found'

export function EmbedPage() {
  const { code } = useParams<{ code: string }>()
  const [phase, setPhase] = useState<EmbedPhase>('loading')

  // Anti-DevTools protection (ป้องกันเปิด F12)
  // TODO: เปิดคืนหลัง debug เสร็จ
  // useAntiDevTools({
  //   onDetected: () => {
  //     // แสดงหน้า 404 แทนหน้าว่าง
  //     setPhase('not-found')
  //   },
  // })

  // ดึงข้อมูล video จาก API โดยใช้ code
  const { data: video, isLoading: videoLoading, error: videoError } = useVideoByCode(code || '')

  // ดึง embed config จาก server (based on domain)
  // Note: Cookie จะถูก set โดย API เมื่อ domain ผ่าน whitelist
  const { data: embedConfig, isLoading: configLoading, error: configError } = useEmbedConfig()

  // ดึง HLS access token (JWT) สำหรับเล่นวิดีโอ
  const { data: streamAccess, isLoading: streamLoading } = useStreamAccess(code || '', {
    enabled: !!code && !!video && video.status === 'ready' && !!embedConfig?.isAllowed,
  })

  // ดึง subtitles สำหรับวิดีโอ (ใช้ code แทน id เพราะเป็น public endpoint)
  const { data: subtitleData } = useSubtitlesByCode(code || '', {
    enabled: !!code && !!video && video.status === 'ready',
  })

  // State สำหรับ subtitle blob URLs (เพราะต้อง fetch ด้วย token)
  const [subtitleBlobUrls, setSubtitleBlobUrls] = useState<Record<string, string>>({})

  // State สำหรับ thumbnail blob URL
  const [thumbnailBlobUrl, setThumbnailBlobUrl] = useState<string | undefined>()

  // Fetch subtitles ด้วย token แล้วสร้าง Blob URLs
  useEffect(() => {
    if (!subtitleData?.subtitles || !streamAccess?.token) return

    const readySubtitles = subtitleData.subtitles.filter(
      sub => sub.status === 'ready' && sub.srtPath
    )

    if (readySubtitles.length === 0) return

    const blobUrls: Record<string, string> = {}
    const fetchPromises = readySubtitles.map(async (sub) => {
      try {
        const url = `${APP_CONFIG.cdnUrl}/${sub.srtPath}`
        const response = await fetch(url, {
          headers: {
            'X-Stream-Token': streamAccess.token,
          },
        })

        if (!response.ok) {
          console.error(`[Subtitle] Failed to fetch ${sub.language}:`, response.status)
          return
        }

        const blob = await response.blob()
        const blobUrl = URL.createObjectURL(blob)
        blobUrls[sub.language] = blobUrl
        console.log(`[Subtitle] Loaded ${sub.language}:`, blobUrl)
      } catch (error) {
        console.error(`[Subtitle] Error fetching ${sub.language}:`, error)
      }
    })

    Promise.all(fetchPromises).then(() => {
      setSubtitleBlobUrls(blobUrls)
    })

    // Cleanup blob URLs on unmount
    return () => {
      Object.values(blobUrls).forEach(url => {
        URL.revokeObjectURL(url)
      })
    }
  }, [subtitleData, streamAccess?.token])

  // Fetch thumbnail ด้วย token แล้วสร้าง Blob URL
  useEffect(() => {
    if (!video?.code || !streamAccess?.token) return

    const fetchThumbnail = async () => {
      try {
        const url = `${APP_CONFIG.streamUrl}/${video.code}/thumb.jpg`
        const response = await fetch(url, {
          headers: {
            'X-Stream-Token': streamAccess.token,
          },
        })

        if (!response.ok) {
          console.warn(`[Thumbnail] Failed to fetch: ${response.status}`)
          return
        }

        const blob = await response.blob()
        const blobUrl = URL.createObjectURL(blob)
        setThumbnailBlobUrl(blobUrl)
        console.log('[Thumbnail] Loaded:', blobUrl)
      } catch (error) {
        console.warn('[Thumbnail] Error fetching:', error)
      }
    }

    fetchThumbnail()

    // Cleanup
    return () => {
      if (thumbnailBlobUrl) {
        URL.revokeObjectURL(thumbnailBlobUrl)
      }
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [video?.code, streamAccess?.token])

  // Build subtitle options สำหรับ player (ใช้ Blob URLs)
  const subtitleOptions = useMemo(() => {
    if (!subtitleData?.subtitles) return []

    return subtitleData.subtitles
      .filter(sub => sub.status === 'ready' && sub.srtPath && subtitleBlobUrls[sub.language])
      .map(sub => ({
        url: subtitleBlobUrls[sub.language], // ใช้ Blob URL แทน
        name: LANGUAGE_LABELS[sub.language] || sub.language,
        language: sub.language,
        default: sub.language === 'th', // ใช้ภาษาไทยเป็น default ถ้ามี
      }))
  }, [subtitleData, subtitleBlobUrls])

  const deviceType = getDeviceType()
  const isMobile = deviceType === 'mobile'

  // Get preroll configs (prefer array, fallback to legacy single)
  const prerollConfigs = embedConfig?.prerollAds?.length
    ? embedConfig.prerollAds
    : embedConfig?.preroll?.enabled && embedConfig.preroll.url
      ? [embedConfig.preroll]
      : []

  const hasPrerolls = prerollConfigs.length > 0

  // Determine initial phase after config loads
  useEffect(() => {
    if (configLoading || videoLoading) {
      setPhase('loading')
      return
    }

    // ถ้า API error หรือไม่ได้รับอนุญาต → block
    if (configError || !embedConfig || !embedConfig.isAllowed) {
      setPhase('blocked')
      return
    }

    // ถ้ามี preroll ad
    if (hasPrerolls) {
      setPhase('preroll')
      return
    }

    // ไม่มี preroll ไปวิดีโอหลักเลย
    setPhase('main')
  }, [configLoading, videoLoading, embedConfig, configError, hasPrerolls])

  // Handle preroll complete
  const handlePrerollComplete = useCallback(() => {
    setPhase('main')
  }, [])

  // Handle preroll skip
  const handlePrerollSkip = useCallback((_skipTime: number, _adIndex?: number) => {
    // Skip tracking removed
  }, [])

  // Handle preroll error
  const handlePrerollError = useCallback(() => {
    setPhase('main')
  }, [])

  // Loading state
  if (phase === 'loading' || videoLoading || configLoading || streamLoading) {
    return (
      <div className="embed-container embed-center">
        <Loader2 className="h-8 w-8 animate-spin text-white" />
      </div>
    )
  }

  // Blocked state (domain not whitelisted)
  if (phase === 'blocked') {
    return (
      <div className="embed-container embed-center">
        <div className="text-center">
          <p className="text-white text-lg">Embedding not allowed</p>
          <p className="text-gray-400 text-sm mt-2">This domain is not authorized</p>
        </div>
      </div>
    )
  }

  // 404 state (DevTools detected)
  if (phase === 'not-found') {
    return (
      <div className="embed-container embed-center bg-gray-900">
        <div className="text-center">
          <p className="text-7xl font-bold text-gray-600 mb-4">404</p>
          <p className="text-white text-xl">Page Not Found</p>
          <p className="text-gray-500 text-sm mt-2">The requested resource could not be found</p>
        </div>
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
          <p className="text-gray-400 text-sm mt-2">Please check back later</p>
        </div>
      </div>
    )
  }

  // Pre-roll ad phase
  // Thumbnail: ใช้ custom จาก profile ถ้ามี, ไม่งั้นใช้ thumbnail ของวิดีโอ
  const prerollThumbnail = embedConfig?.thumbnailUrl || thumbnailBlobUrl

  if (phase === 'preroll' && hasPrerolls) {
    return (
      <div className="embed-container">
        <PrerollPlayer
          configs={prerollConfigs}
          thumbnailUrl={prerollThumbnail}
          onComplete={handlePrerollComplete}
          onSkip={handlePrerollSkip}
          onError={handlePrerollError}
        />
      </div>
    )
  }

  // Main video phase - Use R2 Public URL directly (cookie handles auth)
  // Cookie จะถูก set โดย embed config API call ก่อนหน้า
  const hlsUrl = `${APP_CONFIG.streamUrl}/${video.code}/master.m3u8`

  // Build watermark config
  const watermarkConfig = embedConfig?.watermark
    ? {
        enabled: embedConfig.watermark.enabled,
        url: embedConfig.watermark.url,
        position: embedConfig.watermark.position,
        opacity: embedConfig.watermark.opacity,
        size: embedConfig.watermark.size,
        offsetY: embedConfig.watermark.offsetY,
      }
    : null

  return (
    <div className="embed-container">
      {/* Main Video Player */}
      <VideoPlayer
        src={hlsUrl}
        poster={thumbnailBlobUrl}
        streamToken={streamAccess?.token}
        subtitles={subtitleOptions}
      />

      {/* Watermark Overlay */}
      {watermarkConfig && (
        <Watermark config={watermarkConfig} isMobile={isMobile} />
      )}
    </div>
  )
}
