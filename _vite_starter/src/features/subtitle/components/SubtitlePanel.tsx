import { useState, useEffect } from 'react'
import {
  Languages,
  Download,
  AlertCircle,
  CheckCircle2,
  Loader2,
  Sparkles,
  FileText,
  Plus,
  Clock,
  Trash2,
  RefreshCw,
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

export function SubtitlePanel({ videoId, videoStatus }: SubtitlePanelProps) {
  const [targetLanguage, setTargetLanguage] = useState<string>('')
  const [isJobPending, setIsJobPending] = useState(false)
  const [pendingAutoTranslate, setPendingAutoTranslate] = useState(false)
  const [currentStep, setCurrentStep] = useState<'idle' | 'transcribing' | 'translating'>('idle')

  // Queries
  const { data: subtitleData, isLoading } = useVideoSubtitles(videoId, {
    enabled: videoStatus === 'ready',
  })
  const { data: languages } = useSupportedLanguages()

  // Mutations
  const transcribe = useTranscribe()
  const translate = useTranslate()
  const deleteSubtitle = useDeleteSubtitle()

  // WebSocket progress
  const subtitleProgress = useSubtitleProgress(videoId)
  const activeProgress = subtitleProgress.length > 0 ? subtitleProgress[0] : null

  // Get subtitle data
  const subtitles = subtitleData?.subtitles ?? []
  const originalSubtitle = subtitles.find((s) => s.type === 'original')
  const translatedSubtitles = subtitles.filter((s) => s.type === 'translated')

  // Auto-translate: เมื่อ transcribe เสร็จ และยังไม่มี translation
  useEffect(() => {
    if (
      pendingAutoTranslate &&
      originalSubtitle?.status === 'ready' &&
      translatedSubtitles.length === 0
    ) {
      // หา target language อัตโนมัติ: th → en, อื่นๆ → th
      const targetLang = originalSubtitle.language === 'th' ? 'en' : 'th'

      setPendingAutoTranslate(false)
      setCurrentStep('translating')
      setIsJobPending(true)

      translate.mutate({ videoId, targetLanguages: [targetLang] }, {
        onError: () => {
          setIsJobPending(false)
          setCurrentStep('idle')
        },
      })
    }
  }, [pendingAutoTranslate, originalSubtitle, translatedSubtitles.length, videoId, translate])

  // Clear pending state when progress completes/fails
  useEffect(() => {
    if (activeProgress) {
      if (activeProgress.status === 'completed' || activeProgress.status === 'failed') {
        setIsJobPending(false)
        // ถ้า transcribe เสร็จแล้ว และ pendingAutoTranslate ยังเป็น true
        // useEffect ด้านบนจะ trigger translate
        if (activeProgress.status === 'completed' && currentStep === 'translating') {
          setCurrentStep('idle')
        }
      }
    }
  }, [activeProgress, currentStep])

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

  const handleCreateSubtitle = () => {
    setIsJobPending(true)
    setPendingAutoTranslate(true) // จะ auto translate หลัง transcribe เสร็จ
    setCurrentStep('transcribing')
    transcribe.mutate(videoId, {
      onError: () => {
        setIsJobPending(false)
        setPendingAutoTranslate(false)
        setCurrentStep('idle')
      },
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

  // ลองใหม่ original subtitle (ลบแล้วสร้างใหม่)
  const handleRetryOriginal = (subtitleId: string) => {
    deleteSubtitle.mutate({ subtitleId, videoId }, {
      onSuccess: () => {
        // หลังลบสำเร็จ ให้สร้างใหม่อัตโนมัติ
        handleCreateSubtitle()
      },
    })
  }

  // ลองใหม่ translated subtitle (ลบแล้วแปลใหม่)
  const handleRetryTranslation = (subtitleId: string, language: string) => {
    deleteSubtitle.mutate({ subtitleId, videoId }, {
      onSuccess: () => {
        // หลังลบสำเร็จ ให้แปลใหม่อัตโนมัติ
        setIsJobPending(true)
        translate.mutate({ videoId, targetLanguages: [language] }, {
          onError: () => setIsJobPending(false),
        })
      },
    })
  }

  // ลบ subtitle
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

  const getSrtUrl = (subtitle: Subtitle) => {
    if (!subtitle.srtPath) return null
    return `${APP_CONFIG.cdnUrl}/${subtitle.srtPath}`
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
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Languages className="size-4 text-muted-foreground" />
          <span className="text-sm font-medium">Subtitle</span>
        </div>
        {detectedLanguage && (
          <Badge variant="outline" className="text-xs">
            {LANGUAGE_FLAGS[detectedLanguage]} {LANGUAGE_LABELS[detectedLanguage]}
          </Badge>
        )}
      </div>

      {/* Progress */}
      {isProcessing && (
        <div className="rounded-lg border border-primary/30 bg-primary/5 p-3 space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium text-primary flex items-center gap-1.5">
              <Loader2 className="size-3.5 animate-spin" />
              {activeProgress?.currentStep || (
                currentStep === 'transcribing' ? 'กำลังถอดเสียง...' :
                currentStep === 'translating' ? 'กำลังแปลภาษา...' :
                'กำลังเริ่มต้น...'
              )}
            </span>
            <span className="text-xs text-muted-foreground tabular-nums">
              {activeProgress ? `${Math.round(activeProgress.progress)}%` : ''}
            </span>
          </div>
          <Progress value={activeProgress?.progress ?? 0} className="h-1.5" />
          {activeProgress?.message && (
            <p className="text-xs text-muted-foreground truncate">{activeProgress.message}</p>
          )}
          {/* Step indicator */}
          {pendingAutoTranslate && currentStep === 'transcribing' && (
            <p className="text-xs text-muted-foreground">
              ขั้นตอน 1/2: ถอดเสียง → จะแปลอัตโนมัติเมื่อเสร็จ
            </p>
          )}
          {currentStep === 'translating' && (
            <p className="text-xs text-muted-foreground">
              ขั้นตอน 2/2: กำลังแปลภาษา
            </p>
          )}
        </div>
      )}

      {/* Subtitle List */}
      <div className="space-y-1">
        {/* Original Subtitle */}
        {originalSubtitle ? (
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
            {/* Download button - only when ready */}
            {originalSubtitle.status === 'ready' && originalSubtitle.srtPath && (
              <Button size="icon" variant="ghost" className="size-8 shrink-0" asChild>
                <a href={getSrtUrl(originalSubtitle)!} download title="ดาวน์โหลด SRT">
                  <Download className="size-4" />
                </a>
              </Button>
            )}
            {/* Retry button - for queued, failed, ready */}
            {['queued', 'failed', 'ready'].includes(originalSubtitle.status) && (
              <Button
                size="icon"
                variant="ghost"
                className="size-8 shrink-0"
                onClick={() => handleRetryOriginal(originalSubtitle.id)}
                disabled={deleteSubtitle.isPending || isProcessing}
                title={originalSubtitle.status === 'queued' ? 'Queue ใหม่' : 'ลองใหม่'}
              >
                <RefreshCw className="size-4" />
              </Button>
            )}
            {/* Delete button - for queued, failed */}
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
        ) : (
          // ยังไม่มี Subtitle - แสดงปุ่มสร้าง
          <div className="rounded-lg border border-dashed p-4">
            <div className="flex flex-col items-center gap-3 text-center">
              <div className="size-10 rounded-full bg-primary/10 flex items-center justify-center">
                <Sparkles className="size-5 text-primary" />
              </div>
              <div>
                <p className="text-sm font-medium">สร้าง Subtitle อัตโนมัติ</p>
                <p className="text-xs text-muted-foreground mt-0.5">
                  ถอดเสียง → แปลเป็นไทย (หรืออังกฤษถ้าต้นฉบับเป็นไทย)
                </p>
              </div>
              <Button
                size="sm"
                onClick={handleCreateSubtitle}
                disabled={transcribe.isPending || isProcessing}
                className="gap-1.5"
              >
                {transcribe.isPending || isProcessing ? (
                  <Loader2 className="size-4 animate-spin" />
                ) : (
                  <Sparkles className="size-4" />
                )}
                สร้าง Subtitle
              </Button>
            </div>
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
            {/* Download button - only when ready */}
            {sub.status === 'ready' && sub.srtPath && (
              <Button size="icon" variant="ghost" className="size-8 shrink-0" asChild>
                <a href={getSrtUrl(sub)!} download title="ดาวน์โหลด SRT">
                  <Download className="size-4" />
                </a>
              </Button>
            )}
            {/* Retry button - for queued, failed, ready */}
            {['queued', 'failed', 'ready'].includes(sub.status) && (
              <Button
                size="icon"
                variant="ghost"
                className="size-8 shrink-0"
                onClick={() => handleRetryTranslation(sub.id, sub.language)}
                disabled={deleteSubtitle.isPending || isProcessing}
                title={sub.status === 'queued' ? 'Queue ใหม่' : 'ลองใหม่'}
              >
                <RefreshCw className="size-4" />
              </Button>
            )}
            {/* Delete button - for queued, failed */}
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

        {/* Add Translation */}
        {originalSubtitle?.status === 'ready' && untranslatedLanguages.length > 0 && (
          <div className="flex items-center gap-2 px-3 py-2 rounded-lg border border-dashed">
            <Plus className="size-4 text-muted-foreground" />
            <Select value={targetLanguage} onValueChange={setTargetLanguage}>
              <SelectTrigger className="h-8 flex-1 text-sm">
                <SelectValue placeholder="เพิ่มการแปล..." />
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
              className="shrink-0"
            >
              {translate.isPending ? (
                <Loader2 className="size-4 animate-spin" />
              ) : (
                'แปล'
              )}
            </Button>
          </div>
        )}
      </div>
    </div>
  )
}
