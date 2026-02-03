import { useState, useEffect, useRef, useCallback } from 'react'
import Hls from 'hls.js'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import {
  ArrowLeft,
  Save,
  Download,
  Loader2,
  Film,
  Play,
  Pause,
  SkipBack,
  SkipForward,
  Scissors,
  Flag,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Slider } from '@/components/ui/slider'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { useReel, useCreateReel, useUpdateReel, useExportReel } from '../hooks'
import { useVideoByCode, useVideos } from '@/features/video/hooks'
import { useStreamAccess } from '@/features/embed/hooks/useStreamAccess'
import { APP_CONFIG } from '@/constants/app-config'
import type { ReelLayer, CreateReelRequest, UpdateReelRequest } from '../types'
import { toast } from 'sonner'

// Output format options - how to display the 16:9 source video
type OutputFormat = '9:16' | '1:1' | '4:5' | 'full'

const OUTPUT_FORMAT_OPTIONS: { value: OutputFormat; label: string; aspectClass: string; description: string }[] = [
  { value: '9:16', label: '9:16', aspectClass: 'aspect-[9/16]', description: 'Reels/TikTok' },
  { value: '1:1', label: '1:1', aspectClass: 'aspect-square', description: 'Square' },
  { value: '4:5', label: '4:5', aspectClass: 'aspect-[4/5]', description: 'Instagram' },
  { value: 'full', label: 'Full', aspectClass: 'aspect-video', description: '16:9 เต็ม' },
]

export function ReelGeneratorPage() {
  const navigate = useNavigate()
  const { id } = useParams<{ id: string }>()
  const [searchParams] = useSearchParams()
  const videoCode = searchParams.get('video')

  const isEditing = !!id

  // Fetch existing reel if editing
  const { data: existingReel, isLoading: isLoadingReel } = useReel(id || '')

  // Fetch video if creating from video code
  const { data: videoByCode, isLoading: isLoadingVideo } = useVideoByCode(videoCode || '')

  // Fetch videos for selection
  const { data: videosData } = useVideos({ status: 'ready', limit: 50 })

  // Mutations
  const createReel = useCreateReel()
  const updateReel = useUpdateReel()
  const exportReel = useExportReel()

  // Form state
  const [selectedVideoId, setSelectedVideoId] = useState<string>('')
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [segmentStart, setSegmentStart] = useState(0)
  const [segmentEnd, setSegmentEnd] = useState(60)

  // Display options
  const [outputFormat, setOutputFormat] = useState<OutputFormat>('9:16')
  const [cropX, setCropX] = useState(50) // 0-100, 50 = center
  const [cropY, setCropY] = useState(50) // 0-100, 50 = center

  // Check if current format needs cropping (not full 16:9)
  const needsCrop = outputFormat !== 'full'
  const [showTitle, setShowTitle] = useState(true)
  const [showDescription, setShowDescription] = useState(true)
  const [showGradient, setShowGradient] = useState(true)
  const [titlePosition, setTitlePosition] = useState<'top' | 'center' | 'bottom'>('top')

  // Video info
  const selectedVideo = videosData?.data.find((v) => v.id === selectedVideoId) || videoByCode
  const [actualDuration, setActualDuration] = useState(0)
  // จำกัด duration สูงสุด 10 นาที (600 วินาที) สำหรับทำ reel
  const MAX_REEL_DURATION = 600
  const rawDuration = actualDuration || selectedVideo?.duration || 0
  const videoDuration = Math.min(rawDuration, MAX_REEL_DURATION)
  const isVideoCapped = rawDuration > MAX_REEL_DURATION

  // Stream access for video preview
  const { data: streamAccess, isLoading: isStreamLoading } = useStreamAccess(selectedVideo?.code || '', {
    enabled: !!selectedVideo?.code && selectedVideo?.status === 'ready',
  })

  // HLS URL for video player
  const hlsUrl = selectedVideo?.code ? `${APP_CONFIG.streamUrl}/${selectedVideo.code}/master.m3u8` : ''

  // Video player ref and state
  const videoRef = useRef<HTMLVideoElement>(null)
  const hlsRef = useRef<Hls | null>(null)
  const [isPlaying, setIsPlaying] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [isVideoReady, setIsVideoReady] = useState(false)

  // Initialize HLS.js for video preview
  useEffect(() => {
    const video = videoRef.current
    if (!video || !hlsUrl || !streamAccess?.token) return

    setIsVideoReady(false)
    setActualDuration(0)

    // Destroy previous instance
    if (hlsRef.current) {
      hlsRef.current.destroy()
      hlsRef.current = null
    }

    // Handler เมื่อ video โหลด metadata เสร็จ
    const handleLoadedMetadata = () => {
      console.log('Video metadata loaded, duration:', video.duration)
      if (video.duration && isFinite(video.duration)) {
        setActualDuration(video.duration)
        // ตั้งค่า segment end ถ้ายังไม่ได้ตั้ง
        if (segmentEnd === 60 || segmentEnd > video.duration) {
          setSegmentEnd(Math.min(60, video.duration))
        }
      }
      setIsVideoReady(true)
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

      // ใช้ loadedmetadata event แทน MANIFEST_PARSED เพื่อให้ได้ duration ที่ถูกต้อง
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
      setCurrentTime(video.currentTime)
      if (video.currentTime >= segmentEnd) {
        video.currentTime = segmentStart
        if (isPlaying) {
          video.play()
        }
      }
    }

    video.addEventListener('timeupdate', handleTimeUpdate)
    return () => video.removeEventListener('timeupdate', handleTimeUpdate)
  }, [segmentStart, segmentEnd, isPlaying])

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

    // Clamp time to valid range
    const clampedTime = Math.max(0, Math.min(time, videoDuration))
    video.currentTime = clampedTime
    setCurrentTime(clampedTime)
  }, [isVideoReady, videoDuration])

  // Initialize form when data loads
  useEffect(() => {
    if (existingReel) {
      setSelectedVideoId(existingReel.video?.id || '')
      setTitle(existingReel.title || '')
      setDescription(existingReel.description || '')
      setSegmentStart(existingReel.segmentStart)
      setSegmentEnd(existingReel.segmentEnd)
      // Parse layers to restore display options
      const titleLayer = existingReel.layers?.find((l: ReelLayer) => l.type === 'text' && l.y < 30)
      const descLayer = existingReel.layers?.find((l: ReelLayer) => l.type === 'text' && l.y > 70)
      const bgLayer = existingReel.layers?.find((l: ReelLayer) => l.type === 'background')
      setShowTitle(!!titleLayer)
      setShowDescription(!!descLayer)
      setShowGradient(!!bgLayer)
    } else if (videoByCode) {
      setSelectedVideoId(videoByCode.id)
      setSegmentEnd(Math.min(60, videoByCode.duration))
    }
  }, [existingReel, videoByCode])

  // Build layers from display options
  const buildLayers = (): ReelLayer[] => {
    const layers: ReelLayer[] = []

    // Background gradient
    if (showGradient) {
      layers.push({
        type: 'background',
        style: 'gradient-dark',
        x: 0,
        y: 0,
        width: 100,
        height: 100,
        opacity: 0.5,
        zIndex: 1,
      })
    }

    // Title
    if (showTitle && title) {
      const yPos = titlePosition === 'top' ? 12 : titlePosition === 'center' ? 50 : 88
      layers.push({
        type: 'text',
        content: title,
        fontFamily: 'Google Sans',
        fontSize: 48,
        fontColor: '#ffffff',
        fontWeight: 'bold',
        x: 50,
        y: yPos,
        opacity: 1,
        zIndex: 10,
      })
    }

    // Description (always at bottom if shown)
    if (showDescription && description) {
      const yPos = titlePosition === 'bottom' ? 78 : 88
      layers.push({
        type: 'text',
        content: description,
        fontFamily: 'Google Sans',
        fontSize: 24,
        fontColor: '#ffffff',
        fontWeight: 'normal',
        x: 50,
        y: yPos,
        opacity: 0.9,
        zIndex: 10,
      })
    }

    return layers
  }

  // Save/Update
  const handleSave = async () => {
    if (!selectedVideoId) {
      toast.error('กรุณาเลือกวิดีโอ')
      return
    }

    if (segmentEnd <= segmentStart) {
      toast.error('ช่วงเวลาไม่ถูกต้อง')
      return
    }

    const layers = buildLayers()

    try {
      if (isEditing && id) {
        const data: UpdateReelRequest = {
          title,
          description,
          segmentStart,
          segmentEnd,
          layers,
        }
        await updateReel.mutateAsync({ id, data })
        toast.success('บันทึกสำเร็จ')
      } else {
        const data: CreateReelRequest = {
          videoId: selectedVideoId,
          title,
          description,
          segmentStart,
          segmentEnd,
          layers,
        }
        const newReel = await createReel.mutateAsync(data)
        toast.success('สร้าง Reel สำเร็จ')
        navigate(`/reels/${newReel.id}/edit`, { replace: true })
      }
    } catch (err: any) {
      toast.error(err.message || 'เกิดข้อผิดพลาด')
    }
  }

  // Export
  const handleExport = async () => {
    if (!id) {
      toast.error('กรุณาบันทึกก่อน Export')
      return
    }

    try {
      await exportReel.mutateAsync(id)
      toast.success('เริ่ม Export แล้ว')
    } catch (err: any) {
      toast.error(err.message || 'Export ไม่สำเร็จ')
    }
  }

  const formatTime = (seconds: number) => {
    const mins = Math.floor(seconds / 60)
    const secs = Math.floor(seconds % 60)
    return `${mins}:${secs.toString().padStart(2, '0')}`
  }

  // Get video style based on output format
  const getVideoStyle = (): React.CSSProperties => {
    if (outputFormat === 'full') {
      // Full 16:9 - no crop needed
      return { objectFit: 'contain' }
    }
    // Cropped formats - use cover with custom position
    return {
      objectFit: 'cover',
      objectPosition: `${cropX}% ${cropY}%`,
    }
  }

  // Get title Y position for preview
  const getTitleY = () => {
    switch (titlePosition) {
      case 'top': return '12%'
      case 'center': return '50%'
      case 'bottom': return '88%'
    }
  }

  const isLoading = isLoadingReel || isLoadingVideo

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-48" />
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <Skeleton className="aspect-[9/16] max-w-[300px]" />
          <div className="space-y-4">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-32 w-full" />
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button variant="ghost" size="icon" onClick={() => navigate('/reels')}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <h1 className="text-2xl font-semibold">
            {isEditing ? 'แก้ไข Reel' : 'สร้าง Reel ใหม่'}
          </h1>
        </div>

        <div className="flex items-center gap-2">
          {isEditing && existingReel?.status === 'draft' && (
            <Button
              variant="outline"
              onClick={handleExport}
              disabled={exportReel.isPending}
            >
              {exportReel.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin mr-2" />
              ) : (
                <Download className="h-4 w-4 mr-2" />
              )}
              Export
            </Button>
          )}
          <Button
            onClick={handleSave}
            disabled={createReel.isPending || updateReel.isPending || !selectedVideoId}
          >
            {(createReel.isPending || updateReel.isPending) ? (
              <Loader2 className="h-4 w-4 animate-spin mr-2" />
            ) : (
              <Save className="h-4 w-4 mr-2" />
            )}
            บันทึก
          </Button>
        </div>
      </div>

      {/* Main Content */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Preview Canvas */}
        <div className="flex justify-center">
          <div className="w-full max-w-[320px] space-y-4">
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm">Preview ({outputFormat === 'full' ? '16:9' : outputFormat})</CardTitle>
              </CardHeader>
              <CardContent className="p-3">
                <div className={`relative bg-black rounded-lg overflow-hidden ${OUTPUT_FORMAT_OPTIONS.find(o => o.value === outputFormat)?.aspectClass || 'aspect-[9/16]'}`}>
                  {/* Video Preview */}
                  {selectedVideo && streamAccess?.token && hlsUrl ? (
                    <>
                      <video
                        ref={videoRef}
                        className="absolute inset-0 w-full h-full"
                        style={getVideoStyle()}
                        muted
                        playsInline
                        onClick={togglePlayback}
                      />
                      {/* Play/Pause Overlay */}
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
                    </>
                  ) : selectedVideo && isStreamLoading ? (
                    <div className="absolute inset-0 flex items-center justify-center">
                      <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
                    </div>
                  ) : selectedVideo?.thumbnailUrl ? (
                    <img
                      src={selectedVideo.thumbnailUrl}
                      alt="Preview"
                      className="absolute inset-0 w-full h-full"
                      style={getVideoStyle()}
                    />
                  ) : (
                    <div className="absolute inset-0 flex items-center justify-center">
                      <Film className="h-16 w-16 text-muted-foreground/30" />
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
                      <span
                        className="text-white font-bold drop-shadow-lg"
                        style={{ fontSize: '14px' }}
                      >
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
                      <span
                        className="text-white/90 drop-shadow-lg"
                        style={{ fontSize: '10px' }}
                      >
                        {description}
                      </span>
                    </div>
                  )}

                  {/* Current Time Indicator */}
                  {selectedVideo && (
                    <div className="absolute bottom-2 left-2 right-2">
                      <div className="bg-black/60 rounded px-2 py-1 text-xs text-white text-center">
                        {formatTime(currentTime)} / {formatTime(videoDuration)}
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
                        {isPlaying ? (
                          <Pause className="h-4 w-4" />
                        ) : (
                          <Play className="h-4 w-4" />
                        )}
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
                          setSegmentStart(currentTime)
                          if (currentTime >= segmentEnd) {
                            setSegmentEnd(Math.min(currentTime + 30, videoDuration))
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
                            setSegmentEnd(currentTime)
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
        </div>

        {/* Settings Panel */}
        <div className="space-y-4">
          {/* Step 1: Video Selection */}
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm">1. เลือกวิดีโอ</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <Select
                value={selectedVideoId}
                onValueChange={(v) => {
                  setSelectedVideoId(v)
                  const video = videosData?.data.find((vid) => vid.id === v)
                  if (video) {
                    setSegmentEnd(Math.min(60, video.duration))
                    setSegmentStart(0)
                  }
                }}
                disabled={isEditing}
              >
                <SelectTrigger>
                  <SelectValue placeholder="เลือกวิดีโอ..." />
                </SelectTrigger>
                <SelectContent>
                  {videosData?.data.map((video) => (
                    <SelectItem key={video.id} value={video.id}>
                      {video.code} - {video.title} ({formatTime(video.duration)})
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>

              {/* Output Format Options */}
              {selectedVideo && (
                <div className="space-y-3">
                  {/* Format Selection */}
                  <div className="space-y-2">
                    <Label className="text-xs text-muted-foreground">รูปแบบ Output (Crop จาก 16:9)</Label>
                    <div className="grid grid-cols-4 gap-1">
                      {OUTPUT_FORMAT_OPTIONS.map((opt) => (
                        <Button
                          key={opt.value}
                          variant={outputFormat === opt.value ? 'default' : 'outline'}
                          size="sm"
                          className="h-auto py-2 text-xs px-2 flex flex-col"
                          onClick={() => setOutputFormat(opt.value)}
                        >
                          <span className="font-bold">{opt.label}</span>
                          <span className="text-[10px] opacity-70">{opt.description}</span>
                        </Button>
                      ))}
                    </div>
                  </div>

                  {/* Crop Position Controls (only when cropping) */}
                  {needsCrop && (
                    <div className="space-y-3 p-3 bg-muted/50 rounded-lg">
                      <Label className="text-xs text-muted-foreground">ตำแหน่ง Crop (เลื่อนส่วนที่จะแสดง)</Label>

                      {/* X Position (Left-Right) */}
                      <div className="space-y-1">
                        <div className="flex items-center justify-between text-xs">
                          <span>ซ้าย</span>
                          <span className="font-mono">{cropX}%</span>
                          <span>ขวา</span>
                        </div>
                        <Slider
                          value={[cropX]}
                          min={0}
                          max={100}
                          step={1}
                          onValueChange={([v]) => setCropX(v)}
                        />
                      </div>

                      {/* Y Position - only needed for extreme crops like 9:16 */}
                      {outputFormat === '9:16' && (
                        <div className="space-y-1">
                          <div className="flex items-center justify-between text-xs">
                            <span>บน</span>
                            <span className="font-mono">{cropY}%</span>
                            <span>ล่าง</span>
                          </div>
                          <Slider
                            value={[cropY]}
                            min={0}
                            max={100}
                            step={1}
                            onValueChange={([v]) => setCropY(v)}
                          />
                        </div>
                      )}

                      {/* Quick Position Buttons */}
                      <div className="flex gap-1 justify-center">
                        <Button
                          variant={cropX === 0 ? 'default' : 'outline'}
                          size="sm"
                          className="h-7 text-xs"
                          onClick={() => setCropX(0)}
                        >
                          ← ซ้าย
                        </Button>
                        <Button
                          variant={cropX === 50 ? 'default' : 'outline'}
                          size="sm"
                          className="h-7 text-xs"
                          onClick={() => setCropX(50)}
                        >
                          ● กลาง
                        </Button>
                        <Button
                          variant={cropX === 100 ? 'default' : 'outline'}
                          size="sm"
                          className="h-7 text-xs"
                          onClick={() => setCropX(100)}
                        >
                          ขวา →
                        </Button>
                      </div>
                    </div>
                  )}
                </div>
              )}
            </CardContent>
          </Card>

          {/* Step 2: Timecode Selection */}
          {selectedVideo && (
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm">2. เลือกช่วงเวลา</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                {/* Notice if video is capped */}
                {isVideoCapped && (
                  <div className="p-2 bg-yellow-500/10 border border-yellow-500/30 rounded text-xs text-yellow-600 dark:text-yellow-400">
                    วิดีโอยาว {formatTime(rawDuration)} - ใช้ได้แค่ 10 นาทีแรก
                  </div>
                )}

                {/* Show loading if duration not yet available */}
                {videoDuration === 0 && (
                  <div className="flex items-center justify-center py-8 text-muted-foreground">
                    <Loader2 className="h-5 w-5 animate-spin mr-2" />
                    <span className="text-sm">กำลังโหลดข้อมูลวิดีโอ...</span>
                  </div>
                )}

                {videoDuration > 0 && (
                  <>
                {/* Selected Segment Info */}
                <div className="p-3 bg-primary/10 border border-primary/30 rounded-lg">
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-sm font-medium">Segment ที่เลือก</span>
                    <span className="text-lg font-bold text-primary">
                      {formatTime(segmentEnd - segmentStart)}
                    </span>
                  </div>
                  <div className="flex items-center justify-between text-sm text-muted-foreground">
                    <span>เริ่ม: <span className="font-mono text-foreground">{formatTime(segmentStart)}</span></span>
                    <span>→</span>
                    <span>จบ: <span className="font-mono text-foreground">{formatTime(segmentEnd)}</span></span>
                  </div>
                </div>

                {/* Timeline Visual */}
                <div
                  className={`relative h-16 bg-muted rounded-lg overflow-hidden ${isVideoReady ? 'cursor-pointer' : 'cursor-not-allowed opacity-50'}`}
                  onClick={(e) => {
                    if (!isVideoReady) return
                    const rect = e.currentTarget.getBoundingClientRect()
                    const x = e.clientX - rect.left
                    const time = (x / rect.width) * videoDuration
                    seekTo(time)
                  }}
                >
                  {/* Selected range highlight */}
                  <div
                    className="absolute top-0 bottom-0 bg-primary/40 border-x-2 border-primary"
                    style={{
                      left: `${(segmentStart / videoDuration) * 100}%`,
                      width: `${((segmentEnd - segmentStart) / videoDuration) * 100}%`,
                    }}
                  >
                    {/* Segment start/end labels */}
                    <div className="absolute -left-1 top-1 text-[9px] font-bold text-primary bg-background px-1 rounded">
                      IN
                    </div>
                    <div className="absolute -right-1 top-1 text-[9px] font-bold text-primary bg-background px-1 rounded">
                      OUT
                    </div>
                  </div>

                  {/* Current playhead */}
                  <div
                    className="absolute top-0 bottom-0 w-1 bg-red-500 z-10"
                    style={{ left: `${(currentTime / videoDuration) * 100}%` }}
                  >
                    <div className="absolute -top-1 left-1/2 -translate-x-1/2 w-3 h-3 bg-red-500 rounded-full" />
                    <div className="absolute top-4 left-1/2 -translate-x-1/2 text-[9px] font-mono bg-red-500 text-white px-1 rounded whitespace-nowrap">
                      {formatTime(currentTime)}
                    </div>
                  </div>

                  {/* Time markers */}
                  <div className="absolute bottom-1 left-2 text-[10px] text-muted-foreground">
                    0:00
                  </div>
                  <div className="absolute bottom-1 right-2 text-[10px] text-muted-foreground">
                    {formatTime(videoDuration)}
                  </div>

                  {/* Loading indicator */}
                  {!isVideoReady && (
                    <div className="absolute inset-0 flex items-center justify-center">
                      <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
                    </div>
                  )}
                </div>

                {/* Instructions */}
                <p className="text-xs text-muted-foreground text-center">
                  คลิก timeline เพื่อ seek → กดปุ่ม "จุดเริ่ม" / "จุดจบ" ใต้ preview
                </p>

                {/* Start/End Sliders */}
                <div className="space-y-3">
                  <div className="space-y-1">
                    <div className="flex items-center justify-between">
                      <Label className="text-xs">จุดเริ่มต้น</Label>
                      <span className="text-xs text-muted-foreground font-mono">
                        {formatTime(segmentStart)}
                      </span>
                    </div>
                    <Slider
                      value={[segmentStart]}
                      min={0}
                      max={Math.max(0, videoDuration - 1)}
                      step={0.5}
                      onValueChange={([value]) => {
                        setSegmentStart(value)
                        if (value >= segmentEnd) {
                          setSegmentEnd(Math.min(value + 15, videoDuration))
                        }
                        seekTo(value)
                      }}
                    />
                  </div>

                  <div className="space-y-1">
                    <div className="flex items-center justify-between">
                      <Label className="text-xs">จุดสิ้นสุด</Label>
                      <span className="text-xs text-muted-foreground font-mono">
                        {formatTime(segmentEnd)}
                      </span>
                    </div>
                    <Slider
                      value={[segmentEnd]}
                      min={segmentStart + 1}
                      max={videoDuration}
                      step={0.5}
                      onValueChange={([value]) => {
                        setSegmentEnd(value)
                        // Seek to end point to preview where it ends
                        seekTo(value - 0.5)
                      }}
                    />
                  </div>
                </div>

                {/* Preview Segment Button */}
                <Button
                  variant="secondary"
                  className="w-full"
                  onClick={() => {
                    seekTo(segmentStart)
                    togglePlayback()
                  }}
                  disabled={!isVideoReady}
                >
                  <Play className="h-4 w-4 mr-2" />
                  Preview Segment ({formatTime(segmentEnd - segmentStart)})
                </Button>

                {/* Quick Duration Buttons */}
                <div className="flex gap-2">
                  {[15, 30, 60, 90].map((duration) => (
                    <Button
                      key={duration}
                      variant={segmentEnd - segmentStart === duration ? 'default' : 'outline'}
                      size="sm"
                      className="flex-1"
                      onClick={() => {
                        const newEnd = Math.min(segmentStart + duration, videoDuration)
                        setSegmentEnd(newEnd)
                      }}
                    >
                      {duration}s
                    </Button>
                  ))}
                </div>
                  </>
                )}
              </CardContent>
            </Card>
          )}

          {/* Step 3: Text Overlay */}
          {selectedVideo && (
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm">3. ข้อความบนวิดีโอ</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                {/* Title */}
                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <Label>หัวข้อ</Label>
                    <Switch checked={showTitle} onCheckedChange={setShowTitle} />
                  </div>
                  {showTitle && (
                    <>
                      <Input
                        value={title}
                        onChange={(e) => setTitle(e.target.value)}
                        placeholder="พิมพ์หัวข้อ..."
                      />
                      <Select value={titlePosition} onValueChange={(v) => setTitlePosition(v as 'top' | 'center' | 'bottom')}>
                        <SelectTrigger className="h-8">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="top">แสดงด้านบน</SelectItem>
                          <SelectItem value="center">แสดงตรงกลาง</SelectItem>
                          <SelectItem value="bottom">แสดงด้านล่าง</SelectItem>
                        </SelectContent>
                      </Select>
                    </>
                  )}
                </div>

                {/* Description */}
                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <Label>คำอธิบาย</Label>
                    <Switch checked={showDescription} onCheckedChange={setShowDescription} />
                  </div>
                  {showDescription && (
                    <Textarea
                      value={description}
                      onChange={(e) => setDescription(e.target.value)}
                      placeholder="พิมพ์คำอธิบาย..."
                      rows={2}
                    />
                  )}
                </div>

                {/* Gradient Toggle */}
                <div className="flex items-center justify-between">
                  <Label>Gradient พื้นหลัง</Label>
                  <Switch checked={showGradient} onCheckedChange={setShowGradient} />
                </div>
              </CardContent>
            </Card>
          )}
        </div>
      </div>
    </div>
  )
}
