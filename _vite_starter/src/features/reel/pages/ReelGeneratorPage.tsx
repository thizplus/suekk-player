import { useState, useEffect, useCallback } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { ArrowLeft, Save, Download, Loader2, Film, Clock, Type } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { toast } from 'sonner'

import { useReel, useCreateReel, useUpdateReel, useExportReel } from '../hooks'
import { useVideoByCode, useVideos } from '@/features/video/hooks'
import type { ReelLayer, CreateReelRequest, UpdateReelRequest, OutputFormat, VideoFit, TitlePosition } from '../types'
import {
  ReelPreviewCanvas,
  ReelVideoSelector,
  ReelTimecodeSelector,
  ReelTextOverlay,
  MAX_REEL_DURATION,
} from '../components'

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

  // === Form State ===
  const [selectedVideoId, setSelectedVideoId] = useState<string>('')
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [segmentStart, setSegmentStart] = useState(0)
  const [segmentEnd, setSegmentEnd] = useState(60)

  // Display options
  const [outputFormat, setOutputFormat] = useState<OutputFormat>('9:16')
  const [videoFit, setVideoFit] = useState<VideoFit>('fill')
  const [cropX, setCropX] = useState(50)
  const [cropY, setCropY] = useState(50)

  // Text overlay
  const [showTitle, setShowTitle] = useState(true)
  const [showDescription, setShowDescription] = useState(true)
  const [showGradient, setShowGradient] = useState(true)
  const [titlePosition, setTitlePosition] = useState<TitlePosition>('top')

  // Video state (from preview canvas)
  const [actualDuration, setActualDuration] = useState(0)
  const [currentTime, setCurrentTime] = useState(0)
  const [isVideoReady, setIsVideoReady] = useState(false)
  // Seek request - use counter to ensure seek triggers even for same time
  const [seekRequest, setSeekRequest] = useState<{ time: number; id: number } | null>(null)

  // Tab state
  const [activeTab, setActiveTab] = useState('video')

  // === Derived State ===
  const selectedVideo = videosData?.data.find((v) => v.id === selectedVideoId) || videoByCode
  const rawDuration = actualDuration || selectedVideo?.duration || 0
  const videoDuration = Math.min(rawDuration, MAX_REEL_DURATION)

  // === Initialize form when data loads ===
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

  // Auto switch to timecode tab when video is selected
  useEffect(() => {
    if (selectedVideoId && activeTab === 'video') {
      // Keep on video tab to let user configure output format first
    }
  }, [selectedVideoId, activeTab])

  // === Callbacks for child components ===
  const handleVideoSelect = useCallback((videoId: string) => {
    setSelectedVideoId(videoId)
    const video = videosData?.data.find((v) => v.id === videoId)
    if (video) {
      setSegmentEnd(Math.min(60, video.duration))
      setSegmentStart(0)
    }
  }, [videosData?.data])

  const handleDurationChange = useCallback((duration: number) => {
    setActualDuration(duration)
    if (segmentEnd === 60 || segmentEnd > duration) {
      setSegmentEnd(Math.min(60, duration))
    }
  }, [segmentEnd])

  const handleSegmentStartChange = useCallback((time: number) => {
    setSegmentStart(time)
    if (time >= segmentEnd) {
      setSegmentEnd(Math.min(time + 30, videoDuration))
    }
  }, [segmentEnd, videoDuration])

  const handleSegmentEndChange = useCallback((time: number) => {
    setSegmentEnd(Math.min(time, videoDuration))
  }, [videoDuration])

  // Trigger seek in video player
  const triggerSeek = useCallback((time: number) => {
    setSeekRequest(prev => ({ time, id: (prev?.id || 0) + 1 }))
  }, [])

  // Handle time update from video player (just update display)
  const handleTimeUpdate = useCallback((time: number) => {
    setCurrentTime(time)
  }, [])

  // Preview segment from start
  const handlePreviewSegment = useCallback(() => {
    triggerSeek(segmentStart)
  }, [segmentStart, triggerSeek])

  // === Build layers ===
  const buildLayers = (): ReelLayer[] => {
    const layers: ReelLayer[] = []

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

  // === Actions ===
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

  // === Loading State ===
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
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Button variant="ghost" size="icon" onClick={() => navigate('/reels')}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <h1 className="text-xl font-semibold">
            {isEditing ? 'แก้ไข Reel' : 'สร้าง Reel'}
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
          <ReelPreviewCanvas
            selectedVideo={selectedVideo}
            outputFormat={outputFormat}
            videoFit={videoFit}
            cropX={cropX}
            cropY={cropY}
            segmentStart={segmentStart}
            segmentEnd={segmentEnd}
            title={title}
            description={description}
            showTitle={showTitle}
            showDescription={showDescription}
            showGradient={showGradient}
            titlePosition={titlePosition}
            seekToTime={seekRequest?.time}
            seekRequestId={seekRequest?.id}
            onTimeUpdate={handleTimeUpdate}
            onDurationChange={handleDurationChange}
            onVideoReady={setIsVideoReady}
            onSegmentStartChange={handleSegmentStartChange}
            onSegmentEndChange={handleSegmentEndChange}
          />
        </div>

        {/* Settings Panel with Tabs */}
        <div>
          <Tabs value={activeTab} onValueChange={setActiveTab}>
            <TabsList className="w-full grid grid-cols-3">
              <TabsTrigger value="video" className="gap-2">
                <Film className="h-4 w-4" />
                วิดีโอ
              </TabsTrigger>
              <TabsTrigger value="timecode" className="gap-2" disabled={!selectedVideo}>
                <Clock className="h-4 w-4" />
                ช่วงเวลา
              </TabsTrigger>
              <TabsTrigger value="text" className="gap-2" disabled={!selectedVideo}>
                <Type className="h-4 w-4" />
                ข้อความ
              </TabsTrigger>
            </TabsList>

            <TabsContent value="video" className="mt-4">
              <ReelVideoSelector
                videos={videosData?.data || []}
                selectedVideoId={selectedVideoId}
                outputFormat={outputFormat}
                videoFit={videoFit}
                cropX={cropX}
                cropY={cropY}
                isEditing={isEditing}
                onVideoSelect={handleVideoSelect}
                onOutputFormatChange={setOutputFormat}
                onVideoFitChange={setVideoFit}
                onCropXChange={setCropX}
                onCropYChange={setCropY}
              />
            </TabsContent>

            <TabsContent value="timecode" className="mt-4">
              {selectedVideo && (
                <ReelTimecodeSelector
                  videoDuration={videoDuration}
                  rawDuration={rawDuration}
                  segmentStart={segmentStart}
                  segmentEnd={segmentEnd}
                  currentTime={currentTime}
                  isVideoReady={isVideoReady}
                  onSegmentStartChange={handleSegmentStartChange}
                  onSegmentEndChange={handleSegmentEndChange}
                  onSeekTo={triggerSeek}
                  onPreviewSegment={handlePreviewSegment}
                />
              )}
            </TabsContent>

            <TabsContent value="text" className="mt-4">
              {selectedVideo && (
                <ReelTextOverlay
                  title={title}
                  description={description}
                  showTitle={showTitle}
                  showDescription={showDescription}
                  showGradient={showGradient}
                  titlePosition={titlePosition}
                  onTitleChange={setTitle}
                  onDescriptionChange={setDescription}
                  onShowTitleChange={setShowTitle}
                  onShowDescriptionChange={setShowDescription}
                  onShowGradientChange={setShowGradient}
                  onTitlePositionChange={setTitlePosition}
                />
              )}
            </TabsContent>
          </Tabs>
        </div>
      </div>
    </div>
  )
}
