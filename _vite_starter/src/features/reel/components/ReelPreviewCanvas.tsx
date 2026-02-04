import { useEffect, useRef, useCallback, useState } from 'react'
import Hls from 'hls.js'
import { Play, Pause, SkipBack, SkipForward, Scissors, Flag, Film, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useStreamAccess } from '@/features/embed/hooks'
import { useThumbnailBlob } from '../hooks/useThumbnailBlob'
import { APP_CONFIG } from '@/constants/app-config'
import type { Video } from '@/features/video/types'
import type { ReelStyle } from '../types'
import { formatTime } from './constants'

interface ReelPreviewCanvasProps {
  selectedVideo: Video | undefined
  style: ReelStyle
  segmentStart: number
  segmentEnd: number
  title: string
  line1: string
  line2: string
  showLogo: boolean
  cropX: number  // 0-100 crop position X
  cropY: number  // 0-100 crop position Y
  seekToTime?: number
  seekRequestId?: number
  onTimeUpdate: (time: number) => void
  onDurationChange: (duration: number) => void
  onVideoReady: (ready: boolean) => void
  onSegmentStartChange: (time: number) => void
  onSegmentEndChange: (time: number) => void
}

// Style-specific layout configurations matching FFmpeg output exactly
// Output: 1080x1920, Preview: 320px wide (scale factor ~0.296)
// Title: 120px → ~35px, Lines: 70px → ~21px
const STYLE_LAYOUTS: Record<ReelStyle, {
  videoStyle: React.CSSProperties
  containerClass: string
  titleY: string      // FFmpeg y position as percentage
  line1Y: string
  line2Y: string
  logoPos: { top: string; left: string }
  gradientStart: string  // Where gradient begins (percentage from top)
  hasGradient: boolean
  hasTextShadow: boolean
}> = {
  // Letterbox: 16:9 video (1080x608) centered in 1080x1920
  // Video spans 34.2% - 65.8% vertically
  letterbox: {
    videoStyle: { width: '100%', height: 'auto', objectFit: 'contain' },
    containerClass: 'items-center justify-center',
    titleY: '19.8%',   // FFmpeg y=380 → 380/1920
    line1Y: '71.4%',   // FFmpeg y=h-550 → 1370/1920
    line2Y: '76%',     // FFmpeg y=h-460 → 1460/1920
    logoPos: { top: '35.2%', left: '1.85%' }, // FFmpeg x=20, y=676
    gradientStart: '100%',
    hasGradient: false,
    hasTextShadow: false,
  },
  // Square: 1:1 video (1080x1080) centered in 1080x1920
  // Video spans 21.9% - 78.1% vertically
  square: {
    videoStyle: { width: '100%', height: '56.25%', objectFit: 'cover' },
    containerClass: 'items-center justify-center',
    titleY: '10.4%',   // FFmpeg y=200 → 200/1920
    line1Y: '80.2%',   // FFmpeg y=h-380 → 1540/1920
    line2Y: '84.9%',   // FFmpeg y=h-290 → 1630/1920
    logoPos: { top: '22.9%', left: '1.85%' }, // FFmpeg x=20, y=440
    gradientStart: '100%',
    hasGradient: false,
    hasTextShadow: false,
  },
  // Fullcover: Video fills entire 1080x1920 frame
  // Gradient PNG overlay at y=1320 (bottom 600px = 31.25%)
  fullcover: {
    videoStyle: { width: '100%', height: '100%', objectFit: 'cover' },
    containerClass: 'items-center justify-center',
    titleY: '81.8%',   // FFmpeg y=h-350 → 1570/1920
    line1Y: '88.5%',   // FFmpeg y=h-220 → 1700/1920
    line2Y: '93.2%',   // FFmpeg y=h-130 → 1790/1920
    logoPos: { top: '1%', left: '1.85%' }, // FFmpeg x=20, y=20
    gradientStart: '68.75%', // FFmpeg y=1320 → 1320/1920
    hasGradient: true,
    hasTextShadow: true,
  },
}

export function ReelPreviewCanvas({
  selectedVideo,
  style,
  segmentStart,
  segmentEnd,
  title,
  line1,
  line2,
  showLogo,
  cropX,
  cropY,
  seekToTime,
  seekRequestId,
  onTimeUpdate,
  onDurationChange,
  onVideoReady,
  onSegmentStartChange,
  onSegmentEndChange,
}: ReelPreviewCanvasProps) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const hlsRef = useRef<Hls | null>(null)
  const [isPlaying, setIsPlaying] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [isVideoReady, setIsVideoReady] = useState(false)

  const layout = STYLE_LAYOUTS[style]

  // Compute video style with crop position for square/fullcover
  const computedVideoStyle: React.CSSProperties = {
    ...layout.videoStyle,
    // Add object-position for crop preview (only affects square/fullcover which use object-fit: cover)
    ...(style === 'square' || style === 'fullcover'
      ? { objectPosition: `${cropX}% ${style === 'square' ? cropY : 50}%` }
      : {}),
  }

  // Stream access for video preview
  // Note: ถ้า status ไม่มี (API เก่า) ให้ assume ว่า ready
  const { data: streamAccess, isLoading: isStreamLoading } = useStreamAccess(
    selectedVideo?.code || '',
    { enabled: !!selectedVideo?.code && (selectedVideo?.status === 'ready' || !selectedVideo?.status) }
  )

  // Thumbnail blob URL (fetch ด้วย stream token)
  const { thumbnailBlobUrl } = useThumbnailBlob({
    videoCode: selectedVideo?.code,
    streamToken: streamAccess?.token,
  })

  // HLS URL
  const hlsUrl = selectedVideo?.code ? `${APP_CONFIG.streamUrl}/${selectedVideo.code}/master.m3u8` : ''

  // Initialize HLS.js
  useEffect(() => {
    const video = videoRef.current
    if (!video || !hlsUrl || !streamAccess?.token) return

    setIsVideoReady(false)
    onVideoReady(false)

    if (hlsRef.current) {
      hlsRef.current.destroy()
      hlsRef.current = null
    }

    const handleLoadedMetadata = () => {
      if (video.duration && isFinite(video.duration)) {
        onDurationChange(video.duration)
      }
      setIsVideoReady(true)
      onVideoReady(true)
      video.currentTime = segmentStart
      setCurrentTime(segmentStart)
    }

    if (Hls.isSupported()) {
      const hls = new Hls({
        xhrSetup: (xhr) => {
          xhr.setRequestHeader('X-Stream-Token', streamAccess.token)
        },
        // จำกัด buffer เพื่อลด segment ที่โหลด
        maxBufferLength: 10,        // buffer แค่ 10 วินาที (default 30)
        maxMaxBufferLength: 30,     // max buffer 30 วินาที (default 600!)
        maxBufferSize: 10 * 1000 * 1000, // 10MB max buffer
        startLevel: -1,             // auto select quality
        autoStartLoad: false,       // ไม่โหลดทันที รอ user กด play
      })
      hls.loadSource(hlsUrl)
      hls.attachMedia(video)
      hlsRef.current = hls

      // โหลด manifest เพื่อดึง duration แต่ยังไม่โหลด segments
      hls.on(Hls.Events.MANIFEST_PARSED, () => {
        if (video.duration && isFinite(video.duration)) {
          onDurationChange(video.duration)
        }
        setIsVideoReady(true)
        onVideoReady(true)
      })

      video.addEventListener('loadedmetadata', handleLoadedMetadata, { once: true })

      hls.on(Hls.Events.ERROR, (_, data) => {
        console.error('HLS Error:', data)
      })
    } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
      video.src = hlsUrl
      video.addEventListener('loadedmetadata', handleLoadedMetadata, { once: true })
    }

    return () => {
      video.removeEventListener('loadedmetadata', handleLoadedMetadata)
      if (hlsRef.current) {
        hlsRef.current.destroy()
        hlsRef.current = null
      }
    }
  }, [hlsUrl, streamAccess?.token])

  // Sync video time with segment
  useEffect(() => {
    const video = videoRef.current
    if (!video) return

    const handleTimeUpdate = () => {
      const time = video.currentTime
      setCurrentTime(time)
      onTimeUpdate(time)
      if (time >= segmentEnd) {
        video.currentTime = segmentStart
        if (isPlaying) {
          video.play()
        }
      }
    }

    video.addEventListener('timeupdate', handleTimeUpdate)
    return () => video.removeEventListener('timeupdate', handleTimeUpdate)
  }, [segmentStart, segmentEnd, isPlaying, onTimeUpdate])

  // Handle seek from parent
  useEffect(() => {
    const video = videoRef.current
    if (!video || !isVideoReady || seekToTime === undefined || !seekRequestId) return

    // เริ่มโหลด segments ที่ตำแหน่งที่ต้องการ
    if (hlsRef.current) {
      hlsRef.current.startLoad(seekToTime)
    }
    video.currentTime = seekToTime
    setCurrentTime(seekToTime)
    onTimeUpdate(seekToTime)
  }, [seekRequestId])

  const togglePlayback = useCallback(() => {
    const video = videoRef.current
    if (!video) return

    if (video.paused) {
      // เริ่มโหลด segments เมื่อกด play (ถ้ายังไม่โหลด)
      if (hlsRef.current) {
        hlsRef.current.startLoad(segmentStart)
      }
      if (video.currentTime < segmentStart || video.currentTime >= segmentEnd) {
        video.currentTime = segmentStart
      }
      // รอให้ buffer พร้อมก่อน play
      const playWhenReady = () => {
        video.play().catch(() => {
          // Retry after short delay if play fails
          setTimeout(() => video.play().catch(() => {}), 100)
        })
        setIsPlaying(true)
      }
      // ถ้า buffer พร้อมแล้วเล่นเลย ถ้าไม่รอ canplay
      if (video.readyState >= 3) {
        playWhenReady()
      } else {
        video.addEventListener('canplay', playWhenReady, { once: true })
      }
    } else {
      video.pause()
      setIsPlaying(false)
    }
  }, [segmentStart, segmentEnd])

  const seekTo = useCallback((time: number) => {
    const video = videoRef.current
    if (!video || !isVideoReady) return
    // เริ่มโหลด segments ที่ตำแหน่งที่ต้องการ (ถ้ายังไม่โหลด)
    if (hlsRef.current) {
      hlsRef.current.startLoad(time)
    }
    video.currentTime = time
    setCurrentTime(time)
    onTimeUpdate(time)
  }, [isVideoReady, onTimeUpdate])

  // FFmpeg: shadowcolor=black@0.5:shadowx=2:shadowy=2
  const textShadowStyle = layout.hasTextShadow
    ? { textShadow: '2px 2px 4px rgba(0,0,0,0.7)' }
    : {}

  // Style label in Thai
  const styleLabels: Record<ReelStyle, string> = {
    letterbox: 'แบบมีขอบดำ (16:9)',
    square: 'แบบสี่เหลี่ยม (1:1)',
    fullcover: 'แบบเต็มจอ',
  }

  return (
    <div className="w-full max-w-[320px] space-y-3">
      <div className="relative bg-black overflow-hidden aspect-[9/16]">
        {/* Video Container */}
        <div className={`absolute inset-0 flex overflow-hidden ${layout.containerClass}`}>
          <video
            ref={videoRef}
            className="max-w-full max-h-full"
            style={computedVideoStyle}
            muted
            playsInline
            onClick={togglePlayback}
          />
        </div>

        {/* Loading state */}
        {selectedVideo && isStreamLoading && (
          <div className="absolute inset-0 flex items-center justify-center bg-black">
            <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
          </div>
        )}

        {/* Thumbnail fallback - แสดงขณะรอ HLS โหลด */}
        {thumbnailBlobUrl && !isVideoReady && (
          <div className={`absolute inset-0 flex overflow-hidden ${layout.containerClass}`}>
            <img
              src={thumbnailBlobUrl}
              alt="Preview"
              className="max-w-full max-h-full"
              style={computedVideoStyle}
            />
          </div>
        )}

        {/* Empty state */}
        {!selectedVideo && (
          <div className="absolute inset-0 flex items-center justify-center">
            <Film className="h-16 w-16 text-muted-foreground/30" />
          </div>
        )}

        {/* Play/Pause Overlay */}
        {selectedVideo && streamAccess?.token && (
          <div
            className="absolute inset-0 flex items-center justify-center cursor-pointer bg-black/20 opacity-0 hover:opacity-100 transition-opacity"
            onClick={togglePlayback}
          >
            {isPlaying ? (
              <Pause className="h-12 w-12 text-white drop-shadow-lg" />
            ) : (
              <Play className="h-12 w-12 text-white drop-shadow-lg" />
            )}
          </div>
        )}

        {/* Gradient Overlay (for fullcover style) - matches FFmpeg gradient at y=1320 */}
        {layout.hasGradient && (
          <div
            className="absolute left-0 right-0 bottom-0 pointer-events-none"
            style={{
              top: layout.gradientStart,
              background: 'linear-gradient(to top, rgba(0,0,0,0.85) 0%, rgba(0,0,0,0.6) 40%, rgba(0,0,0,0.3) 70%, transparent 100%)',
            }}
          />
        )}

        {/* Logo Preview */}
        {showLogo && (
          <div
            className="absolute pointer-events-none"
            style={{ top: layout.logoPos.top, left: layout.logoPos.left }}
          >
            <div className="bg-white/30 rounded px-2 py-1 text-[8px] text-white font-bold">
              LOGO
            </div>
          </div>
        )}

        {/* Title Preview - FFmpeg: fontsize=120 → preview ~35px (320/1080 scale) */}
        {title && (
          <div
            className="absolute left-0 right-0 text-center pointer-events-none px-2"
            style={{ top: layout.titleY, ...textShadowStyle }}
          >
            <span
              className="text-white font-bold"
              style={{ fontSize: '35px', lineHeight: 1.1 }}
            >
              {title}
            </span>
          </div>
        )}

        {/* Line1 Preview - FFmpeg: fontsize=70 → preview ~21px */}
        {line1 && (
          <div
            className="absolute left-0 right-0 text-center pointer-events-none px-2"
            style={{ top: layout.line1Y, ...textShadowStyle }}
          >
            <span
              className="text-white"
              style={{ fontSize: '21px', lineHeight: 1.2 }}
            >
              {line1}
            </span>
          </div>
        )}

        {/* Line2 Preview - FFmpeg: fontsize=70 → preview ~21px */}
        {line2 && (
          <div
            className="absolute left-0 right-0 text-center pointer-events-none px-2"
            style={{ top: layout.line2Y, ...textShadowStyle }}
          >
            <span
              className="text-white"
              style={{ fontSize: '21px', lineHeight: 1.2 }}
            >
              {line2}
            </span>
          </div>
        )}

      </div>

      {/* Style label below preview */}
      <div className="text-xs text-muted-foreground text-center">
        {styleLabels[style]} • 1080×1920
      </div>

      {/* Controls below preview */}
      {selectedVideo && streamAccess?.token && (
        <div className="space-y-3">
          {/* Time indicator */}
          <div className="text-center text-sm font-mono text-muted-foreground">
            <span className="text-foreground font-semibold">{formatTime(currentTime)}</span>
            <span className="mx-2">|</span>
            <span>{formatTime(segmentStart)} - {formatTime(segmentEnd)}</span>
          </div>

          {/* Playback controls */}
          <div className="flex items-center justify-center gap-1">
            <Button
              variant="outline"
              size="icon"
              className="h-9 w-9"
              onClick={() => seekTo(segmentStart)}
              disabled={!isVideoReady}
              title="ไปจุดเริ่มต้น"
            >
              <SkipBack className="h-4 w-4" />
            </Button>
            <Button
              variant="outline"
              size="icon"
              className="h-9 w-9"
              onClick={togglePlayback}
              disabled={!isVideoReady}
            >
              {isPlaying ? <Pause className="h-4 w-4" /> : <Play className="h-4 w-4" />}
            </Button>
            <Button
              variant="outline"
              size="icon"
              className="h-9 w-9"
              onClick={() => seekTo(Math.max(0, segmentEnd - 1))}
              disabled={!isVideoReady}
              title="ไปจุดสิ้นสุด"
            >
              <SkipForward className="h-4 w-4" />
            </Button>
            <div className="w-px h-6 bg-border mx-1" />
            <Button
              variant="secondary"
              size="sm"
              className="h-9 px-3"
              onClick={() => {
                onSegmentStartChange(currentTime)
                if (currentTime >= segmentEnd) {
                  onSegmentEndChange(currentTime + 30)
                }
              }}
              disabled={!isVideoReady}
            >
              <Flag className="h-3.5 w-3.5 mr-1.5" />
              เริ่ม
            </Button>
            <Button
              variant="secondary"
              size="sm"
              className="h-9 px-3"
              onClick={() => {
                if (currentTime > segmentStart) {
                  onSegmentEndChange(currentTime)
                }
              }}
              disabled={!isVideoReady || currentTime <= segmentStart}
            >
              <Scissors className="h-3.5 w-3.5 mr-1.5" />
              จบ
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
