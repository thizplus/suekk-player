import { useEffect, useRef, useCallback, useState } from 'react'
import Hls from 'hls.js'
import { Play, Pause, SkipBack, SkipForward, Scissors, Flag, Film, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useStreamAccess } from '@/features/embed/hooks/useStreamAccess'
import { APP_CONFIG } from '@/constants/app-config'
import type { Video } from '@/features/video/types'
import type { OutputFormat, VideoFit, TitlePosition } from '../types'
import { getAspectClass, getCropAspectRatio, formatTime } from './constants'

interface ReelPreviewCanvasProps {
  selectedVideo: Video | undefined
  outputFormat: OutputFormat
  videoFit: VideoFit
  cropX: number
  cropY: number
  segmentStart: number
  segmentEnd: number
  title: string
  description: string
  showTitle: boolean
  showDescription: boolean
  showGradient: boolean
  titlePosition: TitlePosition
  // Callbacks
  onTimeUpdate: (time: number) => void
  onDurationChange: (duration: number) => void
  onVideoReady: (ready: boolean) => void
  onSegmentStartChange: (time: number) => void
  onSegmentEndChange: (time: number) => void
}

export function ReelPreviewCanvas({
  selectedVideo,
  outputFormat,
  videoFit,
  cropX,
  cropY,
  segmentStart,
  segmentEnd,
  title,
  description,
  showTitle,
  showDescription,
  showGradient,
  titlePosition,
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

  // Stream access for video preview
  const { data: streamAccess, isLoading: isStreamLoading } = useStreamAccess(
    selectedVideo?.code || '',
    { enabled: !!selectedVideo?.code && selectedVideo?.status === 'ready' }
  )

  // HLS URL
  const hlsUrl = selectedVideo?.code ? `${APP_CONFIG.streamUrl}/${selectedVideo.code}/master.m3u8` : ''

  // Get crop aspect ratio
  const cropAspectRatio = getCropAspectRatio(videoFit)

  // Initialize HLS.js
  useEffect(() => {
    const video = videoRef.current
    if (!video || !hlsUrl || !streamAccess?.token) return

    setIsVideoReady(false)
    onVideoReady(false)

    // Destroy previous instance
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

  // Toggle play/pause
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

  // Seek to time
  const seekTo = useCallback((time: number) => {
    const video = videoRef.current
    if (!video || !isVideoReady) return
    video.currentTime = time
    setCurrentTime(time)
    onTimeUpdate(time)
  }, [isVideoReady, onTimeUpdate])

  // Get title Y position
  const getTitleY = () => {
    switch (titlePosition) {
      case 'top': return '12%'
      case 'center': return '50%'
      case 'bottom': return '88%'
    }
  }

  // Get video style
  const getVideoStyle = (): React.CSSProperties => {
    if (cropAspectRatio) {
      return {
        width: '100%',
        height: '100%',
        aspectRatio: cropAspectRatio,
        objectFit: 'cover',
        objectPosition: `${cropX}% ${cropY}%`,
      }
    }
    if (videoFit === 'fit') {
      return {
        width: '100%',
        height: '100%',
        objectFit: 'contain',
      }
    }
    return {
      width: '100%',
      height: '100%',
      objectFit: 'cover',
      objectPosition: `${cropX}% ${cropY}%`,
    }
  }

  return (
    <div className="w-full max-w-[320px] space-y-4">
      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-sm">Preview ({outputFormat})</CardTitle>
        </CardHeader>
        <CardContent className="p-3">
          <div className={`relative bg-black rounded-lg overflow-hidden ${getAspectClass(outputFormat)}`}>
            {/* Video Container */}
            <div className="absolute inset-0 flex items-center justify-center overflow-hidden">
              <video
                ref={videoRef}
                className="max-w-full max-h-full"
                style={getVideoStyle()}
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
              <div className="absolute inset-0 flex items-center justify-center overflow-hidden">
                <img
                  src={selectedVideo.thumbnailUrl}
                  alt="Preview"
                  className="max-w-full max-h-full"
                  style={getVideoStyle()}
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

            {/* Gradient Overlay */}
            {showGradient && (
              <div
                className="absolute inset-0 pointer-events-none"
                style={{
                  background: 'linear-gradient(to bottom, rgba(0,0,0,0.3) 0%, transparent 30%, transparent 60%, rgba(0,0,0,0.7) 100%)',
                }}
              />
            )}

            {/* Title Preview */}
            {showTitle && title && (
              <div
                className="absolute left-0 right-0 text-center pointer-events-none px-4"
                style={{ top: getTitleY(), transform: 'translateY(-50%)' }}
              >
                <span className="text-white font-bold drop-shadow-lg" style={{ fontSize: '14px' }}>
                  {title}
                </span>
              </div>
            )}

            {/* Description Preview */}
            {showDescription && description && (
              <div
                className="absolute left-0 right-0 text-center pointer-events-none px-4"
                style={{
                  top: titlePosition === 'bottom' ? '78%' : '88%',
                  transform: 'translateY(-50%)'
                }}
              >
                <span className="text-white/90 drop-shadow-lg" style={{ fontSize: '10px' }}>
                  {description}
                </span>
              </div>
            )}

            {/* Current Time Indicator */}
            {selectedVideo && (
              <div className="absolute bottom-2 left-2 right-2">
                <div className="bg-black/60 rounded px-2 py-1 text-xs text-white text-center">
                  {formatTime(currentTime)} / {formatTime(segmentEnd - segmentStart)}
                </div>
              </div>
            )}
          </div>

          {/* Playback Controls */}
          {selectedVideo && streamAccess?.token && (
            <div className="mt-3 space-y-2">
              {/* Play/Seek Controls */}
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

              {/* Mark In/Out Controls */}
              <div className="flex items-center justify-center gap-2">
                <Button
                  variant="secondary"
                  size="sm"
                  className="h-7 text-xs"
                  onClick={() => {
                    onSegmentStartChange(currentTime)
                    if (currentTime >= segmentEnd) {
                      onSegmentEndChange(currentTime + 30)
                    }
                  }}
                  disabled={!isVideoReady}
                >
                  <Flag className="h-3 w-3 mr-1" />
                  จุดเริ่ม [{formatTime(currentTime)}]
                </Button>
                <Button
                  variant="secondary"
                  size="sm"
                  className="h-7 text-xs"
                  onClick={() => {
                    if (currentTime > segmentStart) {
                      onSegmentEndChange(currentTime)
                    }
                  }}
                  disabled={!isVideoReady || currentTime <= segmentStart}
                >
                  <Scissors className="h-3 w-3 mr-1" />
                  จุดจบ [{formatTime(currentTime)}]
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
