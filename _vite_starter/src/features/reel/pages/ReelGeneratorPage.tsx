import { useState, useEffect, useCallback } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { ArrowLeft, Save, Download, Loader2, Film, Clock, Type, Volume2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { toast } from 'sonner'

import { useReel, useCreateReel, useUpdateReel, useExportReel } from '../hooks'
import { useVideoByCode } from '@/features/video/hooks'
import type { Video } from '@/features/video/types'
import type { ReelStyle, CreateReelRequest, UpdateReelRequest, VideoSegment } from '../types'
import {
  ReelPreviewCanvas,
  ReelVideoSelector,
  ReelTimecodeSelector,
  ReelTextOverlay,
  SegmentList,
  generateChunkOptions,
  type ChunkOption,
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

  // Mutations
  const createReel = useCreateReel()
  const updateReel = useUpdateReel()
  const exportReel = useExportReel()

  // === Form State ===
  const [selectedVideoId, setSelectedVideoId] = useState<string>('')
  const [selectedVideo, setSelectedVideo] = useState<Video | undefined>(undefined)
  const [segmentStart, setSegmentStart] = useState(0)
  const [segmentEnd, setSegmentEnd] = useState(60)

  // Multi-segment state
  const [segments, setSegments] = useState<VideoSegment[]>([])
  const [selectedSegmentIndex, setSelectedSegmentIndex] = useState<number | null>(null)

  // Style-based fields
  const [style, setStyle] = useState<ReelStyle>('letterbox')
  const [title, setTitle] = useState('')
  const [line1, setLine1] = useState('')
  const [line2, setLine2] = useState('')
  const [showLogo, setShowLogo] = useState(true)
  const [cropX, setCropX] = useState(50) // 0-100, center default
  const [cropY, setCropY] = useState(50) // 0-100, center default
  const [coverTime, setCoverTime] = useState(-1) // -1 = auto middle
  const [ttsText, setTtsText] = useState('') // TTS text

  // Video state (from preview canvas)
  const [actualDuration, setActualDuration] = useState(0)
  const [currentTime, setCurrentTime] = useState(0)
  const [isVideoReady, setIsVideoReady] = useState(false)
  const [seekRequest, setSeekRequest] = useState<{ time: number; id: number } | null>(null)

  // Chunk state สำหรับ video ยาว
  const [selectedChunk, setSelectedChunk] = useState<ChunkOption | null>(null)

  // Tab state
  const [activeTab, setActiveTab] = useState('video')

  // === Derived State ===
  const activeVideo = selectedVideo || videoByCode
  const rawDuration = actualDuration || activeVideo?.duration || 0

  // สร้าง chunk options จาก duration จริง
  const chunkOptions = generateChunkOptions(rawDuration)

  // videoDuration สำหรับ chunk ที่เลือก (หรือ chunk แรกถ้ายังไม่เลือก)
  const currentChunk = selectedChunk || chunkOptions[0] || { start: 0, end: rawDuration }
  const videoDuration = currentChunk.end

  // === Initialize form when data loads ===
  useEffect(() => {
    if (existingReel) {
      setSelectedVideoId(existingReel.video?.id || '')
      setSelectedVideo(existingReel.video as Video | undefined)
      setSegmentStart(existingReel.segmentStart)
      setSegmentEnd(existingReel.segmentEnd)

      // Multi-segment: load from API if exists
      if (existingReel.segments && existingReel.segments.length > 0) {
        const loadedSegments: VideoSegment[] = existingReel.segments.map((seg, i) => ({
          id: `seg_${i}_${Date.now()}`,
          start: seg.start,
          end: seg.end,
        }))
        setSegments(loadedSegments)
        setSelectedSegmentIndex(0)
      }

      // Style-based fields
      if (existingReel.style) {
        setStyle(existingReel.style)
      }
      setTitle(existingReel.title || '')
      setLine1(existingReel.line1 || '')
      setLine2(existingReel.line2 || '')
      setShowLogo(existingReel.showLogo ?? true)
      setCropX(existingReel.cropX ?? 50)
      setCropY(existingReel.cropY ?? 50)
      setCoverTime(existingReel.coverTime ?? -1)
      setTtsText(existingReel.ttsText || '')
    } else if (videoByCode) {
      setSelectedVideoId(videoByCode.id)
      setSelectedVideo(videoByCode)
      setSegmentEnd(Math.min(60, videoByCode.duration))
    }
  }, [existingReel, videoByCode])

  // === Callbacks for child components ===
  const handleVideoSelect = useCallback((videoId: string, video?: Video) => {
    setSelectedVideoId(videoId)
    setSelectedVideo(video)
    if (video) {
      setSegmentEnd(Math.min(60, video.duration))
      setSegmentStart(0)
    }
  }, [])

  const handleDurationChange = useCallback((duration: number) => {
    setActualDuration(duration)

    // สร้าง chunks และ set chunk ที่ตรงกับ segmentStart (สำหรับ edit mode)
    const chunks = generateChunkOptions(duration)
    if (!selectedChunk && chunks.length > 0) {
      // หา chunk ที่ segmentStart อยู่ในช่วง (สำหรับ edit reel ที่มี segment อยู่แล้ว)
      const matchingChunk = chunks.find(c => segmentStart >= c.start && segmentStart < c.end)
      if (matchingChunk) {
        setSelectedChunk(matchingChunk)
      } else {
        // ถ้าไม่เจอ ใช้ chunk แรก
        setSelectedChunk(chunks[0])
        // ตั้งค่า segment ภายใน chunk แรก (สำหรับ create mode)
        if (segmentEnd === 60 || segmentEnd > chunks[0].end) {
          setSegmentEnd(Math.min(60, chunks[0].end))
        }
      }
    }
  }, [segmentStart, segmentEnd, selectedChunk])

  const handleChunkChange = useCallback((chunk: ChunkOption) => {
    setSelectedChunk(chunk)
    // Reset segment to beginning of new chunk
    setSegmentStart(chunk.start)
    setSegmentEnd(Math.min(chunk.start + 60, chunk.end))
  }, [])

  const handleSegmentStartChange = useCallback((time: number) => {
    setSegmentStart(time)
    if (time >= segmentEnd) {
      setSegmentEnd(Math.min(time + 30, videoDuration))
    }
  }, [segmentEnd, videoDuration])

  const handleSegmentEndChange = useCallback((time: number) => {
    setSegmentEnd(Math.min(time, videoDuration))
  }, [videoDuration])

  const triggerSeek = useCallback((time: number) => {
    setSeekRequest(prev => ({ time, id: (prev?.id || 0) + 1 }))
  }, [])

  const handleTimeUpdate = useCallback((time: number) => {
    setCurrentTime(time)
  }, [])

  const handlePreviewSegment = useCallback(() => {
    triggerSeek(segmentStart)
  }, [segmentStart, triggerSeek])

  // === Actions ===
  const handleSave = async () => {
    if (!selectedVideoId) {
      toast.error('กรุณาเลือกวิดีโอ')
      return
    }

    // ถ้ามี segments ให้ใช้ segments, ไม่งั้นใช้ segmentStart/End
    const hasSegments = segments.length > 0
    const segmentsForApi = hasSegments
      ? segments.map(seg => ({ start: seg.start, end: seg.end }))
      : undefined

    // Validate
    if (hasSegments) {
      const totalDuration = segments.reduce((sum, seg) => sum + (seg.end - seg.start), 0)
      if (totalDuration > 60) {
        toast.error('ความยาวรวมเกิน 60 วินาที')
        return
      }
    } else {
      if (segmentEnd <= segmentStart) {
        toast.error('ช่วงเวลาไม่ถูกต้อง')
        return
      }
    }

    try {
      if (isEditing && id) {
        const data: UpdateReelRequest = {
          segments: segmentsForApi,
          segmentStart: hasSegments ? undefined : segmentStart,
          segmentEnd: hasSegments ? undefined : segmentEnd,
          coverTime,
          style,
          title,
          line1,
          line2,
          showLogo,
          cropX,
          cropY,
          ttsText: ttsText || undefined,
        }
        await updateReel.mutateAsync({ id, data })
        toast.success('บันทึกสำเร็จ')
      } else {
        const data: CreateReelRequest = {
          videoId: selectedVideoId,
          segments: segmentsForApi,
          segmentStart: hasSegments ? undefined : segmentStart,
          segmentEnd: hasSegments ? undefined : segmentEnd,
          coverTime,
          style,
          title,
          line1,
          line2,
          showLogo,
          cropX,
          cropY,
          ttsText: ttsText || undefined,
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
          {/* Export button - แสดงเมื่อไม่ได้กำลัง export อยู่ (อนุญาตให้ re-export ได้) */}
          {isEditing && existingReel?.status !== 'exporting' && (
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
              {existingReel?.status === 'ready' ? 'Re-Export' : 'Export'}
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
            selectedVideo={activeVideo}
            style={style}
            segmentStart={segmentStart}
            segmentEnd={segmentEnd}
            title={title}
            line1={line1}
            line2={line2}
            showLogo={showLogo}
            cropX={cropX}
            cropY={cropY}
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
            <TabsList className="w-full grid grid-cols-4">
              <TabsTrigger value="video" className="gap-2">
                <Film className="h-4 w-4" />
                วิดีโอ
              </TabsTrigger>
              <TabsTrigger value="timecode" className="gap-2" disabled={!activeVideo}>
                <Clock className="h-4 w-4" />
                ช่วงเวลา
              </TabsTrigger>
              <TabsTrigger value="text" className="gap-2" disabled={!activeVideo}>
                <Type className="h-4 w-4" />
                ข้อความ
              </TabsTrigger>
              <TabsTrigger value="tts" className="gap-2" disabled={!activeVideo}>
                <Volume2 className="h-4 w-4" />
                เสียง
              </TabsTrigger>
            </TabsList>

            <TabsContent value="video" className="mt-4">
              <ReelVideoSelector
                selectedVideoId={selectedVideoId}
                selectedVideo={activeVideo}
                style={style}
                isEditing={isEditing}
                cropX={cropX}
                cropY={cropY}
                onVideoSelect={handleVideoSelect}
                onStyleChange={setStyle}
                onCropXChange={setCropX}
                onCropYChange={setCropY}
              />
            </TabsContent>

            <TabsContent value="timecode" className="mt-4 space-y-4">
              {activeVideo && (
                <>
                  {/* Multi-segment list */}
                  <SegmentList
                    segments={segments}
                    videoDuration={rawDuration}
                    currentTime={currentTime}
                    onChange={setSegments}
                    onSeek={triggerSeek}
                    onSelectSegment={(index) => {
                      setSelectedSegmentIndex(index)
                      if (index !== null && segments[index]) {
                        // Update single-segment state เพื่อแสดงใน ReelTimecodeSelector
                        setSegmentStart(segments[index].start)
                        setSegmentEnd(segments[index].end)
                      }
                    }}
                    selectedIndex={selectedSegmentIndex}
                  />

                  {/* Divider */}
                  {segments.length > 0 && selectedSegmentIndex !== null && (
                    <div className="border-t pt-4">
                      <p className="text-sm text-muted-foreground mb-2">
                        แก้ไข Segment {(selectedSegmentIndex ?? 0) + 1}
                      </p>
                    </div>
                  )}

                  {/* Single segment editor (ใช้ edit segment ที่เลือก หรือ single mode) */}
                  <ReelTimecodeSelector
                    videoDuration={videoDuration}
                    rawDuration={rawDuration}
                    segmentStart={segmentStart}
                    segmentEnd={segmentEnd}
                    currentTime={currentTime}
                    isVideoReady={isVideoReady}
                    selectedChunk={selectedChunk}
                    chunkOptions={chunkOptions}
                    coverTime={coverTime}
                    onChunkChange={handleChunkChange}
                    onSegmentStartChange={(time) => {
                      handleSegmentStartChange(time)
                      // Update segment in list if editing
                      if (selectedSegmentIndex !== null && segments[selectedSegmentIndex]) {
                        const newSegments = [...segments]
                        newSegments[selectedSegmentIndex] = {
                          ...newSegments[selectedSegmentIndex],
                          start: time,
                        }
                        setSegments(newSegments)
                      }
                    }}
                    onSegmentEndChange={(time) => {
                      handleSegmentEndChange(time)
                      // Update segment in list if editing
                      if (selectedSegmentIndex !== null && segments[selectedSegmentIndex]) {
                        const newSegments = [...segments]
                        newSegments[selectedSegmentIndex] = {
                          ...newSegments[selectedSegmentIndex],
                          end: time,
                        }
                        setSegments(newSegments)
                      }
                    }}
                    onSeekTo={triggerSeek}
                    onPreviewSegment={handlePreviewSegment}
                    onCoverTimeChange={setCoverTime}
                  />
                </>
              )}
            </TabsContent>

            <TabsContent value="text" className="mt-4">
              {activeVideo && (
                <ReelTextOverlay
                  style={style}
                  title={title}
                  line1={line1}
                  line2={line2}
                  showLogo={showLogo}
                  onTitleChange={setTitle}
                  onLine1Change={setLine1}
                  onLine2Change={setLine2}
                  onShowLogoChange={setShowLogo}
                />
              )}
            </TabsContent>

            <TabsContent value="tts" className="mt-4">
              {activeVideo && (
                <div className="space-y-4">
                  <div className="space-y-2">
                    <label className="text-sm font-medium">เสียงพากย์ (TTS)</label>
                    <textarea
                      value={ttsText}
                      onChange={(e) => setTtsText(e.target.value)}
                      placeholder="ใส่ข้อความที่ต้องการให้ AI พากย์... (เว้นว่างถ้าไม่ต้องการเสียง)"
                      rows={6}
                      maxLength={5000}
                      className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 resize-none"
                    />
                    <p className="text-xs text-muted-foreground">
                      {ttsText.length.toLocaleString()}/5,000 ตัวอักษร
                    </p>
                  </div>
                  <div className="rounded-lg bg-muted p-3 text-sm text-muted-foreground">
                    <p className="font-medium mb-1">💡 คำแนะนำ:</p>
                    <ul className="list-disc list-inside space-y-1">
                      <li>ใส่ข้อความภาษาไทยหรืออังกฤษได้</li>
                      <li>เว้นว่างถ้าไม่ต้องการเสียงพากย์</li>
                      <li>เสียงจะถูกสร้างอัตโนมัติเมื่อ Export</li>
                    </ul>
                  </div>
                </div>
              )}
            </TabsContent>
          </Tabs>
        </div>
      </div>
    </div>
  )
}
