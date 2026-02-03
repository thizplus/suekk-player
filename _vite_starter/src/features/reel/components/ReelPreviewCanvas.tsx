import { useEffect, useRef, useCallback, useState } from 'react'
import Hls from 'hls.js'
import { Play, Pause, SkipBack, SkipForward, Scissors, Flag, Film, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useStreamAccess } from '@/features/embed/hooks/useStreamAccess'
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
  seekToTime?: number
  seekRequestId?: number
  onTimeUpdate: (time: number) => void
  onDurationChange: (duration: number) => void
  onVideoReady: (ready: boolean) => void
  onSegmentStartChange: (time: number) => void
  onSegmentEndChange: (time: number) => void
}

// Style-specific layout configurations
const STYLE_LAYOUTS: Record<ReelStyle, {
  videoStyle: React.CSSProperties
  containerClass: string
  titleY: string
  line1Y: string
  line2Y: string
  logoPos: { top?: string; left: string }
  hasGradient: boolean
  hasTextShadow: boolean
}> = {
  letterbox: {
    videoStyle: { width: '100%', height: 'auto', objectFit: 'contain' },
    containerClass: 'items-center justify-center',
    titleY: '20%',  // In top black bar
    line1Y: '71%',  // In bottom black bar
    line2Y: '76%',
    logoPos: { top: '35%', left: '5%' }, // Inside video frame
    hasGradient: false,
    hasTextShadow: false,
  },
  square: {
    videoStyle: { width: '100%', height: '56.25%', objectFit: 'cover' }, // 1:1 in center
    containerClass: 'items-center justify-center',
    titleY: '10%',  // In top black bar
    line1Y: '80%',  // In bottom black bar
    line2Y: '85%',
    logoPos: { top: '23%', left: '5%' }, // Inside video frame
    hasGradient: false,
    hasTextShadow: false,
  },
  fullcover: {
    videoStyle: { width: '100%', height: '100%', objectFit: 'cover' },
    containerClass: 'items-center justify-center',
    titleY: '82%',  // Over gradient
    line1Y: '88%',
    line2Y: '93%',
    logoPos: { top: '2%', left: '2%' }, // Top left
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

  // Stream access for video preview
  const { data: streamAccess, isLoading: isStreamLoading } = useStreamAccess(
    selectedVideo?.code || '',
    { enabled: !!selectedVideo?.code && selectedVideo?.status === 'ready' }
  )

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
      })
      hls.loadSource(hlsUrl)
      hls.attachMedia(video)
      hlsRef.current = hls

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

    video.currentTime = seekToTime
    setCurrentTime(seekToTime)
    onTimeUpdate(seekToTime)
  }, [seekRequestId])

  const togglePlayback = useCallback(() => {
    const video = videoRef.current
    if (!video) return

    if (video.paused) {
      if (video.currentTime < segmentStart || video.currentTime >= segmentEnd) {
        video.currentTime = segmentStart
      }
      video.play()
      setIsPlaying(true)
    } else {
      video.pause()
      setIsPlaying(false)
    }
  }, [segmentStart, segmentEnd])

  const seekTo = useCallback((time: number) => {
    const video = videoRef.current
    if (!video || !isVideoReady) return
    video.currentTime = time
    setCurrentTime(time)
    onTimeUpdate(time)
  }, [isVideoReady, onTimeUpdate])

  const textShadowStyle = layout.hasTextShadow
    ? { textShadow: '2px 2px 4px rgba(0,0,0,0.5)' }
    : {}

  return (
    <div className="w-full max-w-[320px] space-y-3">
      <div className="text-sm text-muted-foreground text-center">
        Preview ({style}) - 1080x1920
      </div>
      <div className="relative bg-black rounded-lg overflow-hidden aspect-[9/16]">
        {/* Video Container */}
        <div className={`absolute inset-0 flex overflow-hidden ${layout.containerClass}`}>
          <video
            ref={videoRef}
            className="max-w-full max-h-full"
            style={layout.videoStyle}
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

        {/* Thumbnail fallback */}
        {selectedVideo?.thumbnailUrl && !streamAccess?.token && !isStreamLoading && (
          <div className={`absolute inset-0 flex overflow-hidden ${layout.containerClass}`}>
            <img
              src={selectedVideo.thumbnailUrl}
              alt="Preview"
              className="max-w-full max-h-full"
              style={layout.videoStyle}
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

        {/* Gradient Overlay (for fullcover style) */}
        {layout.hasGradient && (
          <div
            className="absolute inset-0 pointer-events-none"
            style={{
              background: 'linear-gradient(to top, rgba(0,0,0,0.8) 0%, rgba(0,0,0,0.4) 30%, transparent 60%)',
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

        {/* Title Preview */}
        {title && (
          <div
            className="absolute left-0 right-0 text-center pointer-events-none px-4"
            style={{ top: layout.titleY, ...textShadowStyle }}
          >
            <span className="text-white font-bold drop-shadow-lg text-lg">
              {title}
            </span>
          </div>
        )}

        {/* Line1 Preview */}
        {line1 && (
          <div
            className="absolute left-0 right-0 text-center pointer-events-none px-4"
            style={{ top: layout.line1Y, ...textShadowStyle }}
          >
            <span className="text-white/90 drop-shadow-lg text-sm">
              {line1}
            </span>
          </div>
        )}

        {/* Line2 Preview */}
        {line2 && (
          <div
            className="absolute left-0 right-0 text-center pointer-events-none px-4"
            style={{ top: layout.line2Y, ...textShadowStyle }}
          >
            <span className="text-white/90 drop-shadow-lg text-sm">
              {line2}
            </span>
          </div>
        )}

        {/* Current Time Indicator */}
        {selectedVideo && (
          <div className="absolute bottom-2 left-2 right-2">
            <div className="bg-black/60 rounded px-2 py-1 text-xs text-white text-center">
              {formatTime(currentTime)} ({formatTime(segmentStart)} - {formatTime(segmentEnd)})
            </div>
          </div>
        )}
      </div>

      {/* Playback Controls */}
      {selectedVideo && streamAccess?.token && (
        <div className="space-y-2">
          <div className="flex items-center justify-center gap-2">
            <Button
              variant="outline"
              size="icon"
              className="h-8 w-8"
              onClick={() => seekTo(segmentStart)}
              disabled={!isVideoReady}
              title="ไปจุดเริ่มต้น"
            >
              <SkipBack className="h-4 w-4" />
            </Button>
            <Button
              variant="outline"
              size="icon"
              className="h-8 w-8"
              onClick={togglePlayback}
              disabled={!isVideoReady}
            >
              {isPlaying ? <Pause className="h-4 w-4" /> : <Play className="h-4 w-4" />}
            </Button>
            <Button
              variant="outline"
              size="icon"
              className="h-8 w-8"
              onClick={() => seekTo(Math.max(0, segmentEnd - 1))}
              disabled={!isVideoReady}
              title="ไปจุดสิ้นสุด"
            >
              <SkipForward className="h-4 w-4" />
            </Button>
          </div>

          <div className="flex items-center justify-center gap-2">
            <Button
              variant="secondary"
              size="sm"
              onClick={() => {
                onSegmentStartChange(currentTime)
                if (currentTime >= segmentEnd) {
                  onSegmentEndChange(currentTime + 30)
                }
              }}
              disabled={!isVideoReady}
            >
              <Flag className="h-3 w-3 mr-1" />
              จุดเริ่ม
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => {
                if (currentTime > segmentStart) {
                  onSegmentEndChange(currentTime)
                }
              }}
              disabled={!isVideoReady || currentTime <= segmentStart}
            >
              <Scissors className="h-3 w-3 mr-1" />
              จุดจบ
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
