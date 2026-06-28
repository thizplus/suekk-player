/**
 * SubtitleEditorPage - หน้าแก้ไข subtitle พร้อม real-time preview
 * Route: /preview/:code/edit
 */

import { useState, useEffect, useMemo, useRef, useCallback } from 'react'
import type Artplayer from 'artplayer'
import { useParams, useNavigate, useBeforeUnload } from 'react-router-dom'
import { useVideoByCode, VideoPlayer } from '@/features/video'
import { useSubtitleContent, useUpdateSubtitleContent } from '@/features/subtitle'
import { SubtitleEditor } from '../components/SubtitleEditor'
import { parseSRT, generateSRT } from '../utils/srt-parser'
import type { SubtitleSegment, Subtitle } from '../types'
import { Loader2, ArrowLeft, ExternalLink } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { APP_CONFIG } from '@/constants/app-config'
import { LANGUAGE_LABELS, LANGUAGE_FLAGS } from '@/constants/enums'
import { useStreamAccess } from '@/features/embed/hooks/useStreamAccess'

// Simple debounced callback hook
function useDebouncedCallback<T extends (...args: Parameters<T>) => ReturnType<T>>(
  callback: T,
  delay: number
): T {
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const callbackRef = useRef(callback)
  callbackRef.current = callback

  return useCallback(
    ((...args: Parameters<T>) => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current)
      }
      timeoutRef.current = setTimeout(() => {
        callbackRef.current(...args)
      }, delay)
    }) as T,
    [delay]
  )
}

export function SubtitleEditorPage() {
  const { code } = useParams<{ code: string }>()
  const navigate = useNavigate()

  // === Data Fetching ===
  const { data: video, isLoading: videoLoading, error: videoError } = useVideoByCode(code || '')
  const { data: streamAccess, isLoading: streamLoading } = useStreamAccess(code || '', {
    enabled: !!code && !!video && video.status === 'ready',
  })

  // หา subtitle ที่ ready ทั้งหมด
  const readySubtitles = useMemo(() => {
    return (video?.subtitles || []).filter(
      (sub) => sub.status === 'ready' && sub.srtPath
    )
  }, [video?.subtitles])

  // เลือกภาษา — default เป็นตัวแรกที่ ready
  const [selectedLanguage, setSelectedLanguage] = useState<string>('')

  // Set default language เมื่อ subtitles โหลดเสร็จ
  useEffect(() => {
    if (readySubtitles.length > 0 && !selectedLanguage) {
      // เลือก th ก่อน ถ้าไม่มีเลือกตัวแรก
      const defaultLang = readySubtitles.find(s => s.language === 'th')?.language
        || readySubtitles[0].language
      setSelectedLanguage(defaultLang)
    }
  }, [readySubtitles, selectedLanguage])

  const activeSubtitle = useMemo((): Subtitle | undefined => {
    return readySubtitles.find(s => s.language === selectedLanguage)
  }, [readySubtitles, selectedLanguage])

  // ดึง content ของ subtitle ที่เลือก
  const {
    data: subtitleContent,
    isLoading: contentLoading,
  } = useSubtitleContent(activeSubtitle?.id || '', {
    enabled: !!activeSubtitle?.id,
  })

  // Mutation สำหรับ save
  const updateSubtitleMutation = useUpdateSubtitleContent()

  // === State ===
  const [segments, setSegments] = useState<SubtitleSegment[]>([])
  const [originalContent, setOriginalContent] = useState<string>('')
  const [currentTime, setCurrentTime] = useState(0)
  const [subtitleBlobUrl, setSubtitleBlobUrl] = useState<string>('')
  const [thumbnailBlobUrl, setThumbnailBlobUrl] = useState<string | undefined>()
  // Track when subtitle blob is ready (to prevent player mounting before subtitle is loaded)
  const [subtitleReady, setSubtitleReady] = useState(false)
  // Store Artplayer instance for seek control
  const artRef = useRef<Artplayer | null>(null)

  // Track if content has changed
  const isDirty = useMemo(() => {
    const currentContent = generateSRT(segments)
    return currentContent !== originalContent && segments.length > 0
  }, [segments, originalContent])

  // Warn before leaving with unsaved changes
  useBeforeUnload(
    useCallback(
      (e) => {
        if (isDirty) {
          e.preventDefault()
          e.returnValue = ''
        }
      },
      [isDirty]
    )
  )

  // === Reset state when language changes ===
  const prevLanguageRef = useRef(selectedLanguage)
  useEffect(() => {
    if (prevLanguageRef.current === selectedLanguage) return
    prevLanguageRef.current = selectedLanguage
    setSegments([])
    setOriginalContent('')
    setSubtitleBlobUrl('')
    // ไม่ set subtitleReady=false เพื่อไม่ให้ player unmount
    // blob URL จะถูก update ผ่าน fetch effect อัตโนมัติ
  }, [selectedLanguage])

  // === Initialize segments from fetched content ===
  useEffect(() => {
    if (subtitleContent?.content) {
      const parsed = parseSRT(subtitleContent.content)
      setSegments(parsed)
      setOriginalContent(subtitleContent.content)
    }
  }, [subtitleContent?.content])

  // === Create initial subtitle Blob URL ===
  useEffect(() => {
    // รอให้ video พร้อม (subtitles มาพร้อมกับ video แล้ว)
    if (!video || video.status !== 'ready') {
      console.log('[SubtitleEditor] Waiting for video to be ready...')
      return
    }

    console.log('[SubtitleEditor] Creating blob URL...', {
      hasToken: !!streamAccess?.token,
      activeSubtitle,
      srtPath: activeSubtitle?.srtPath,
    })

    // ถ้าไม่มี activeSubtitle → mark as ready (จะไปแสดง error page)
    if (!activeSubtitle?.srtPath) {
      console.log('[SubtitleEditor] No Thai subtitle found')
      setSubtitleReady(true)
      return
    }

    // ถ้ายังไม่มี token → รอ (streamAccess ยังโหลดอยู่)
    if (!streamAccess?.token) {
      console.log('[SubtitleEditor] Waiting for stream token...')
      return
    }

    let blobUrl: string | undefined

    const fetchSubtitle = async () => {
      try {
        const url = `${APP_CONFIG.cdnUrl}/${activeSubtitle.srtPath}`
        console.log('[SubtitleEditor] Fetching subtitle from:', url)
        const response = await fetch(url, {
          headers: { 'X-Stream-Token': streamAccess.token },
        })

        if (!response.ok) {
          console.error('[SubtitleEditor] Fetch failed:', response.status)
          setSubtitleReady(true) // Mark ready even on error so page doesn't hang
          return
        }

        const blob = await response.blob()
        blobUrl = URL.createObjectURL(blob)
        setSubtitleBlobUrl(blobUrl)
        setSubtitleReady(true) // Mark ready AFTER blob URL is set
        console.log('[SubtitleEditor] Subtitle blob URL created:', blobUrl)
      } catch (error) {
        console.error('[SubtitleEditor] Error:', error)
        setSubtitleReady(true) // Mark ready even on error
      }
    }

    fetchSubtitle()

    return () => {
      if (blobUrl) URL.revokeObjectURL(blobUrl)
    }
  }, [video, streamAccess?.token, activeSubtitle?.srtPath])

  // === Fetch thumbnail ===
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

  // === Real-time Preview: Regenerate Blob URL when segments change ===
  const previousBlobUrlRef = useRef<string>('')

  const debouncedUpdatePreview = useDebouncedCallback((newSegments: SubtitleSegment[]) => {
    // Generate new SRT content
    const srtContent = generateSRT(newSegments)

    // Create new Blob URL
    const blob = new Blob([srtContent], { type: 'text/plain; charset=utf-8' })
    const newUrl = URL.createObjectURL(blob)

    // Cleanup previous Blob URL
    if (previousBlobUrlRef.current) {
      URL.revokeObjectURL(previousBlobUrlRef.current)
    }
    previousBlobUrlRef.current = newUrl

    // Update state to trigger player refresh
    setSubtitleBlobUrl(newUrl)
  }, 300)

  // === Handlers ===
  const handleSegmentChange = useCallback(
    (index: number, text: string) => {
      setSegments((prev) => {
        const newSegments = [...prev]
        newSegments[index] = { ...newSegments[index], text }

        // Trigger real-time preview
        debouncedUpdatePreview(newSegments)

        return newSegments
      })
    },
    [debouncedUpdatePreview]
  )

  const handleTimecodeChange = useCallback(
    (index: number, startTime: string, endTime: string) => {
      setSegments((prev) => {
        const newSegments = [...prev]
        newSegments[index] = { ...newSegments[index], startTime, endTime }

        // Trigger real-time preview
        debouncedUpdatePreview(newSegments)

        return newSegments
      })
    },
    [debouncedUpdatePreview]
  )

  const handleSeek = useCallback((seconds: number) => {
    // Use stored Artplayer instance for reliable seeking
    if (artRef.current) {
      artRef.current.seek = seconds
      console.log('[SubtitleEditor] Seek via artRef to:', seconds)
      return
    }

    // Fallback: use video element directly
    const videoEl = document.querySelector('.artplayer-container video') as HTMLVideoElement
    if (videoEl) {
      videoEl.currentTime = seconds
      // Force frame update if paused
      if (videoEl.paused) {
        videoEl.play().then(() => videoEl.pause()).catch(() => {})
      }
    }
  }, [])

  // Handle player ready - store art instance
  const handlePlayerReady = useCallback((art: Artplayer) => {
    artRef.current = art
    console.log('[SubtitleEditor] Player ready, art instance stored')
  }, [])

  const handleSave = useCallback(async () => {
    if (!activeSubtitle?.id) return

    const content = generateSRT(segments)
    await updateSubtitleMutation.mutateAsync({
      subtitleId: activeSubtitle.id,
      content,
    })

    // Update original content to mark as saved
    setOriginalContent(content)
  }, [activeSubtitle?.id, segments, updateSubtitleMutation])

  const handleReset = useCallback(() => {
    if (subtitleContent?.content) {
      const parsed = parseSRT(subtitleContent.content)
      setSegments(parsed)

      // Regenerate Blob URL with original content
      const blob = new Blob([subtitleContent.content], { type: 'text/plain; charset=utf-8' })
      const newUrl = URL.createObjectURL(blob)

      if (previousBlobUrlRef.current) {
        URL.revokeObjectURL(previousBlobUrlRef.current)
      }
      previousBlobUrlRef.current = newUrl
      setSubtitleBlobUrl(newUrl)
    }
  }, [subtitleContent?.content])

  const handleTimeUpdate = useCallback((time: number) => {
    setCurrentTime(time)
  }, [])

  // === Build subtitle options for player ===
  const subtitleOptions = useMemo(() => {
    if (!subtitleBlobUrl || !selectedLanguage) return []
    return [
      {
        url: subtitleBlobUrl,
        name: LANGUAGE_LABELS[selectedLanguage] || selectedLanguage,
        language: selectedLanguage,
        default: true,
      },
    ]
  }, [subtitleBlobUrl, selectedLanguage])

  // === Loading states ===
  // รอให้ subtitle blob พร้อมก่อน mount player เพื่อให้ subtitle แสดงตั้งแต่แรก
  const isLoading = videoLoading || streamLoading || !subtitleReady

  if (isLoading) {
    return (
      <div className="flex h-screen items-center justify-center bg-background">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
      </div>
    )
  }

  if (videoError || !video) {
    return (
      <div className="flex h-screen flex-col items-center justify-center gap-4 bg-background">
        <p className="text-lg text-muted-foreground">Video not found</p>
        <Button variant="outline" onClick={() => navigate(-1)}>
          <ArrowLeft className="mr-2 h-4 w-4" />
          กลับ
        </Button>
      </div>
    )
  }

  if (video.status !== 'ready') {
    return (
      <div className="flex h-screen flex-col items-center justify-center gap-4 bg-background">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
        <p className="text-lg text-muted-foreground">Video is being processed...</p>
      </div>
    )
  }

  if (!activeSubtitle) {
    return (
      <div className="flex h-screen flex-col items-center justify-center gap-4 bg-background">
        <p className="text-lg text-muted-foreground">ไม่พบ subtitle สำหรับวิดีโอนี้</p>
        <Button variant="outline" onClick={() => navigate(-1)}>
          <ArrowLeft className="mr-2 h-4 w-4" />
          กลับ
        </Button>
      </div>
    )
  }

  if (!streamAccess?.token) {
    return (
      <div className="flex h-screen items-center justify-center bg-background">
        <Loader2 className="h-8 w-8 animate-spin text-primary" />
      </div>
    )
  }

  const hlsUrl = `${APP_CONFIG.streamUrl}/${video.code}/master.m3u8`

  return (
    <div className="flex h-screen flex-col bg-background">
      {/* Header */}
      <header className="flex items-center justify-between border-b px-4 py-2">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="sm" onClick={() => navigate(-1)}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div>
            <h1 className="text-sm font-semibold">{video.title}</h1>
            <p className="text-xs text-muted-foreground">{video.code}</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {readySubtitles.length > 1 && (
            <Select value={selectedLanguage} onValueChange={setSelectedLanguage}>
              <SelectTrigger className="h-8 w-[160px] text-sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {readySubtitles.map((sub) => (
                  <SelectItem key={sub.language} value={sub.language}>
                    {LANGUAGE_FLAGS[sub.language]} {LANGUAGE_LABELS[sub.language] || sub.language}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
          <Button variant="outline" size="sm" asChild>
            <a href={`/preview/${code}`} target="_blank" rel="noopener noreferrer">
              <ExternalLink className="mr-1.5 h-4 w-4" />
              Preview
            </a>
          </Button>
        </div>
      </header>

      {/* Main content */}
      <div className="flex flex-1 overflow-hidden">
        {/* Video Player (60%) */}
        <div className="flex w-[60%] flex-col border-r">
          <div className="relative flex-1 bg-black">
            <VideoPlayer
              src={hlsUrl}
              poster={thumbnailBlobUrl}
              streamToken={streamAccess.token}
              subtitles={subtitleOptions}
              dynamicSubtitle={true}
              onTimeUpdate={handleTimeUpdate}
              onReady={handlePlayerReady}
            />
          </div>
        </div>

        {/* Subtitle Editor (40%) */}
        <div className="flex w-[40%] flex-col">
          <SubtitleEditor
            segments={segments}
            currentTime={currentTime}
            onSegmentChange={handleSegmentChange}
            onTimecodeChange={handleTimecodeChange}
            onSeek={handleSeek}
            onSave={handleSave}
            onReset={handleReset}
            isDirty={isDirty}
            isSaving={updateSubtitleMutation.isPending}
            isLoading={contentLoading}
            language={selectedLanguage}
          />
        </div>
      </div>
    </div>
  )
}
