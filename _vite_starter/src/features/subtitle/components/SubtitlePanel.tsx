import { useState, useEffect } from 'react'
import {
  Languages,
  Download,
  AlertCircle,
  CheckCircle2,
  Loader2,
  FileText,
  Clock,
  Trash2,
  RefreshCw,
  Pencil,
  Search,
  Mic,
  Globe,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { useSubtitleProgress } from '@/lib/websocket-provider'
import { Progress } from '@/components/ui/progress'
import {
  useVideoSubtitles,
  useSupportedLanguages,
  useTranscribe,
  useTranslate,
  useDeleteSubtitle,
  useDetectLanguage,
  useSetLanguage,
} from '../hooks'
import {
  SUBTITLE_STATUS_LABELS,
  SUBTITLE_STATUS_STYLES,
  LANGUAGE_LABELS,
  LANGUAGE_FLAGS,
} from '@/constants/enums'
import type { Subtitle, SubtitleStatus } from '../types'
import { APP_CONFIG } from '@/constants/app-config'

interface SubtitlePanelProps {
  videoId: string
  videoCode?: string
  videoStatus: string
}

export function SubtitlePanel({ videoId, videoCode, videoStatus }: SubtitlePanelProps) {
  const [targetLanguage, setTargetLanguage] = useState<string>('')
  const [isJobPending, setIsJobPending] = useState(false)

  // Queries
  const { data: subtitleData, isLoading } = useVideoSubtitles(videoId, {
    enabled: videoStatus === 'ready',
  })
  const { data: languages } = useSupportedLanguages()

  // Mutations
  const transcribe = useTranscribe()
  const translate = useTranslate()
  const deleteSubtitle = useDeleteSubtitle()
  const detectLanguage = useDetectLanguage()
  const setLanguage = useSetLanguage()

  // WebSocket progress
  const subtitleProgress = useSubtitleProgress(videoId)
  const activeProgress = subtitleProgress.length > 0 ? subtitleProgress[0] : null

  // Get subtitle data
  const subtitles = subtitleData?.subtitles ?? []
  const originalSubtitle = subtitles.find((s) => s.type === 'original')
  const translatedSubtitles = subtitles.filter((s) => s.type === 'translated')

  // Clear pending state when progress completes/fails
  useEffect(() => {
    if (activeProgress) {
      if (activeProgress.status === 'completed' || activeProgress.status === 'failed') {
        setIsJobPending(false)
      }
    }
  }, [activeProgress])

  const isProcessing = isJobPending || !!activeProgress

  if (videoStatus !== 'ready') return null

  if (isLoading) {
    return (
      <div className="space-y-2">
        <Skeleton className="h-4 w-32" />
        <Skeleton className="h-12 w-full" />
        <Skeleton className="h-12 w-full" />
      </div>
    )
  }

  const hasAudio = subtitleData?.hasAudio ?? false
  const detectedLanguage = subtitleData?.detectedLanguage

  // หาภาษาที่สามารถแปลได้
  const sourceLanguage = originalSubtitle?.language || detectedLanguage
  const availableTargetLanguages = sourceLanguage
    ? languages?.translationPairs[sourceLanguage] ?? []
    : []
  const translatedLanguages = translatedSubtitles.map((s) => s.language)
  const untranslatedLanguages = availableTargetLanguages.filter(
    (lang) => !translatedLanguages.includes(lang)
  )

  // === Handlers (แต่ละ step เป็นอิสระ) ===

  const handleDetect = () => {
    setIsJobPending(true)
    detectLanguage.mutate(videoId, {
      onError: () => setIsJobPending(false),
    })
  }

  const handleTranscribe = () => {
    setIsJobPending(true)
    transcribe.mutate(videoId, {
      onError: () => setIsJobPending(false),
    })
  }

  const handleTranslate = () => {
    if (targetLanguage) {
      setIsJobPending(true)
      translate.mutate({ videoId, targetLanguages: [targetLanguage] }, {
        onError: () => setIsJobPending(false),
      })
      setTargetLanguage('')
    }
  }

  const handleRetryOriginal = (subtitleId: string) => {
    deleteSubtitle.mutate({ subtitleId, videoId }, {
      onSuccess: () => handleTranscribe(),
    })
  }

  const handleRetryTranslation = (subtitleId: string, language: string) => {
    deleteSubtitle.mutate({ subtitleId, videoId }, {
      onSuccess: () => {
        setIsJobPending(true)
        translate.mutate({ videoId, targetLanguages: [language] }, {
          onError: () => setIsJobPending(false),
        })
      },
    })
  }

  const handleDelete = (subtitleId: string) => {
    deleteSubtitle.mutate({ subtitleId, videoId })
  }

  const getStatusIcon = (status: SubtitleStatus) => {
    switch (status) {
      case 'ready':
        return <CheckCircle2 className="size-4 text-status-success" />
      case 'failed':
        return <AlertCircle className="size-4 text-destructive" />
      case 'queued':
        return <Clock className="size-4 text-status-pending" />
      case 'detecting':
      case 'processing':
      case 'translating':
        return <Loader2 className="size-4 animate-spin text-primary" />
      default:
        return <FileText className="size-4 text-muted-foreground" />
    }
  }

  const handleDownloadSrt = async (subtitle: Subtitle) => {
    if (!subtitle.id) return
    try {
      const { apiClient } = await import('@/lib/api-client')
      const res = await apiClient.get<{ data: { content: string } }>(
        `/api/v1/subtitles/${subtitle.id}/content`
      )
      const content = res.data?.content || res.data
      const blob = new Blob([typeof content === 'string' ? content : JSON.stringify(content)], {
        type: 'text/plain; charset=utf-8',
      })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `${videoCode || 'subtitle'}_${subtitle.language}.srt`
      a.click()
      URL.revokeObjectURL(url)
    } catch {
      // fallback: เปิด CDN URL ตรง (อาจ fail ถ้า private)
      if (subtitle.srtPath) {
        window.open(`${APP_CONFIG.cdnUrl}/${subtitle.srtPath}`, '_blank')
      }
    }
  }

  // ไม่มี Audio
  if (!hasAudio) {
    return (
      <div className="space-y-3">
        <div className="flex items-center gap-2">
          <Languages className="size-4 text-muted-foreground" />
          <span className="text-sm font-medium">Subtitle</span>
        </div>
        <div className="rounded-lg border border-dashed p-4 text-center">
          <p className="text-sm text-muted-foreground">ไม่พบ Audio Track</p>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-3">
      {/* Header */}
      <div className="flex items-center gap-2">
        <Languages className="size-4 text-muted-foreground" />
        <span className="text-sm font-medium">Subtitle</span>
      </div>

      {/* Step 1: Detect Language */}
      <div className="rounded-lg border p-3 space-y-2">
        <div className="flex items-center justify-between">
          <span className="text-xs font-medium text-muted-foreground">1. ตรวจจับภาษา</span>
          {detectedLanguage ? (
            <Badge variant="outline" className="text-xs">
              {LANGUAGE_FLAGS[detectedLanguage]} {LANGUAGE_LABELS[detectedLanguage]}
            </Badge>
          ) : (
            <Badge variant="secondary" className="text-xs">ยังไม่ตรวจจับ</Badge>
          )}
        </div>
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            variant={detectedLanguage ? 'outline' : 'default'}
            onClick={handleDetect}
            disabled={detectLanguage.isPending || isProcessing}
            className="gap-1.5 flex-1"
          >
            {detectLanguage.isPending ? (
              <Loader2 className="size-3.5 animate-spin" />
            ) : (
              <Search className="size-3.5" />
            )}
            {detectedLanguage ? 'ตรวจจับใหม่' : 'ตรวจจับภาษา'}
          </Button>
          <Select
            value=""
            onValueChange={(lang) => setLanguage.mutate({ videoId, language: lang })}
            disabled={setLanguage.isPending || isProcessing}
          >
            <SelectTrigger className="h-8 w-[130px] text-xs">
              <SelectValue placeholder="ตั้งค่าเอง..." />
            </SelectTrigger>
            <SelectContent>
              {languages?.sourceLanguages.map((lang) => (
                <SelectItem key={lang.code} value={lang.code}>
                  {LANGUAGE_FLAGS[lang.code]} {LANGUAGE_LABELS[lang.code] || lang.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      {/* Step 2: Transcribe */}
      {detectedLanguage && !originalSubtitle && (
        <div className="rounded-lg border p-3 space-y-2">
          <span className="text-xs font-medium text-muted-foreground">2. ถอดเสียงเป็นข้อความ</span>
          <Button
            size="sm"
            onClick={handleTranscribe}
            disabled={transcribe.isPending || isProcessing}
            className="gap-1.5 w-full"
          >
            {transcribe.isPending ? (
              <Loader2 className="size-3.5 animate-spin" />
            ) : (
              <Mic className="size-3.5" />
            )}
            ถอดเสียง ({LANGUAGE_FLAGS[detectedLanguage]} {LANGUAGE_LABELS[detectedLanguage]})
          </Button>
        </div>
      )}

      {/* Progress */}
      {isProcessing && (
        <div className="rounded-lg border border-primary/30 bg-primary/5 p-3 space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium text-primary flex items-center gap-1.5">
              <Loader2 className="size-3.5 animate-spin" />
              {activeProgress?.currentStep || 'กำลังเริ่มต้น...'}
            </span>
            <span className="text-xs text-muted-foreground tabular-nums">
              {activeProgress ? `${Math.round(activeProgress.progress)}%` : ''}
            </span>
          </div>
          <Progress value={activeProgress?.progress ?? 0} className="h-1.5" />
          {activeProgress?.message && (
            <p className="text-xs text-muted-foreground truncate">{activeProgress.message}</p>
          )}
        </div>
      )}

      {/* Subtitle List */}
      <div className="space-y-1">
        {/* Original Subtitle */}
        {originalSubtitle && (
          <div className="flex items-center gap-3 px-3 py-2 rounded-lg border hover:bg-accent/50 transition-colors">
            {getStatusIcon(originalSubtitle.status)}
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium">
                  {LANGUAGE_FLAGS[originalSubtitle.language]} {LANGUAGE_LABELS[originalSubtitle.language]}
                </span>
                <Badge variant="secondary" className="text-[10px] px-1.5 py-0">
                  ต้นฉบับ
                </Badge>
              </div>
              {originalSubtitle.status === 'failed' && originalSubtitle.error && (
                <p className="text-xs text-destructive truncate mt-0.5">{originalSubtitle.error}</p>
              )}
            </div>
            {originalSubtitle.status === 'ready' && originalSubtitle.srtPath && (
              <>
                <Button size="icon" variant="ghost" className="size-8 shrink-0" onClick={() => handleDownloadSrt(originalSubtitle)} title="ดาวน์โหลด SRT">
                  <Download className="size-4" />
                </Button>
                {videoCode && (
                  <Button size="icon" variant="ghost" className="size-8 shrink-0" asChild>
                    <a href={`/preview/${videoCode}/edit`} target="_blank" rel="noopener noreferrer" title="แก้ไข Subtitle">
                      <Pencil className="size-4" />
                    </a>
                  </Button>
                )}
              </>
            )}
            {['queued', 'failed', 'ready'].includes(originalSubtitle.status) && (
              <Button
                size="icon"
                variant="ghost"
                className="size-8 shrink-0"
                onClick={() => handleRetryOriginal(originalSubtitle.id)}
                disabled={deleteSubtitle.isPending || isProcessing}
                title="ลองใหม่"
              >
                <RefreshCw className="size-4" />
              </Button>
            )}
            {['queued', 'failed'].includes(originalSubtitle.status) && (
              <Button
                size="icon"
                variant="ghost"
                className="size-8 shrink-0 text-destructive hover:text-destructive"
                onClick={() => handleDelete(originalSubtitle.id)}
                disabled={deleteSubtitle.isPending}
                title="ลบ"
              >
                <Trash2 className="size-4" />
              </Button>
            )}
            <Badge className={SUBTITLE_STATUS_STYLES[originalSubtitle.status]}>
              {SUBTITLE_STATUS_LABELS[originalSubtitle.status]}
            </Badge>
          </div>
        )}

        {/* Translated Subtitles */}
        {translatedSubtitles.map((sub) => (
          <div
            key={sub.id}
            className="flex items-center gap-3 px-3 py-2 rounded-lg border hover:bg-accent/50 transition-colors"
          >
            {getStatusIcon(sub.status)}
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium">
                  {LANGUAGE_FLAGS[sub.language]} {LANGUAGE_LABELS[sub.language]}
                </span>
                <Badge variant="outline" className="text-[10px] px-1.5 py-0">
                  แปล
                </Badge>
              </div>
              {sub.status === 'failed' && sub.error && (
                <p className="text-xs text-destructive truncate mt-0.5">{sub.error}</p>
              )}
            </div>
            {sub.status === 'ready' && sub.srtPath && (
              <>
                <Button size="icon" variant="ghost" className="size-8 shrink-0" onClick={() => handleDownloadSrt(sub)} title="ดาวน์โหลด SRT">
                  <Download className="size-4" />
                </Button>
                {videoCode && (
                  <Button size="icon" variant="ghost" className="size-8 shrink-0" asChild>
                    <a href={`/preview/${videoCode}/edit`} target="_blank" rel="noopener noreferrer" title="แก้ไข Subtitle">
                      <Pencil className="size-4" />
                    </a>
                  </Button>
                )}
              </>
            )}
            {['queued', 'failed', 'ready'].includes(sub.status) && (
              <Button
                size="icon"
                variant="ghost"
                className="size-8 shrink-0"
                onClick={() => handleRetryTranslation(sub.id, sub.language)}
                disabled={deleteSubtitle.isPending || isProcessing}
                title="ลองใหม่"
              >
                <RefreshCw className="size-4" />
              </Button>
            )}
            {['queued', 'failed'].includes(sub.status) && (
              <Button
                size="icon"
                variant="ghost"
                className="size-8 shrink-0 text-destructive hover:text-destructive"
                onClick={() => handleDelete(sub.id)}
                disabled={deleteSubtitle.isPending}
                title="ลบ"
              >
                <Trash2 className="size-4" />
              </Button>
            )}
            <Badge className={SUBTITLE_STATUS_STYLES[sub.status]}>
              {SUBTITLE_STATUS_LABELS[sub.status]}
            </Badge>
          </div>
        ))}

        {/* Step 3: Add Translation */}
        {originalSubtitle?.status === 'ready' && untranslatedLanguages.length > 0 && (
          <div className="rounded-lg border border-dashed p-3 space-y-2">
            <span className="text-xs font-medium text-muted-foreground">3. แปลภาษา</span>
            <div className="flex items-center gap-2">
              <Globe className="size-4 text-muted-foreground shrink-0" />
              <Select value={targetLanguage} onValueChange={setTargetLanguage}>
                <SelectTrigger className="h-8 flex-1 text-sm">
                  <SelectValue placeholder="เลือกภาษา..." />
                </SelectTrigger>
                <SelectContent>
                  {untranslatedLanguages.map((lang) => (
                    <SelectItem key={lang} value={lang}>
                      {LANGUAGE_FLAGS[lang]} {LANGUAGE_LABELS[lang]}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button
                size="sm"
                onClick={handleTranslate}
                disabled={!targetLanguage || translate.isPending || isProcessing}
                className="shrink-0 gap-1.5"
              >
                {translate.isPending ? (
                  <Loader2 className="size-3.5 animate-spin" />
                ) : (
                  <Globe className="size-3.5" />
                )}
                แปล
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
