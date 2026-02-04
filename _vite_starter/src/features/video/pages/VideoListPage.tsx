import { useState, useEffect } from 'react'
import { useSearchParams, useNavigate } from 'react-router-dom'
import { Plus, Eye, Trash2, RefreshCw, Copy, MoreVertical, Loader2, Video, Clock, Files, HardDrive, Languages, CheckCircle2, AlertCircle, Sparkles, Folder, Film } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  DropdownMenuSub,
  DropdownMenuSubTrigger,
  DropdownMenuSubContent,
  DropdownMenuPortal,
} from '@/components/ui/dropdown-menu'
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from '@/components/ui/pagination'
import {
  Empty,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
  EmptyDescription,
  EmptyContent,
} from '@/components/ui/empty'
import { useVideos, useDeleteVideo, useQueueTranscoding, useVideoByCode } from '../hooks'
import { VideoUploadDialog } from '../components/VideoUploadDialog'
import { VideoBatchUploadDialog } from '../components/VideoBatchUploadDialog'
import { VideoDetailSheet } from '../components/VideoDetailSheet'
import { VideoFilters } from '../components/VideoFilters'
import { VIDEO_STATUS_LABELS, VIDEO_STATUS_STYLES, LANGUAGE_LABELS, LANGUAGE_FLAGS } from '@/constants/enums'
import type { VideoFilterParams, SubtitleSummary } from '../types'
import { useVideoProgress } from '@/lib/websocket-provider'
import { toast } from 'sonner'
import { useTranscribe, useTranslate, useSupportedLanguages } from '@/features/subtitle/hooks'

interface DeleteTarget {
  id: string
  title: string
}

export function VideoListPage() {
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const [filters, setFilters] = useState<VideoFilterParams>({
    page: 1,
    limit: 15,
    sortBy: 'created_at',
    sortOrder: 'desc',
  })
  const [uploadDialogOpen, setUploadDialogOpen] = useState(false)
  const [batchUploadDialogOpen, setBatchUploadDialogOpen] = useState(false)
  const [selectedVideoId, setSelectedVideoId] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null)

  // รองรับเปิด detail sheet จาก URL param ?code=xxx
  const codeParam = searchParams.get('code')
  const { data: videoByCode } = useVideoByCode(codeParam || '')

  // เมื่อ fetch video by code สำเร็จ → เปิด sheet
  useEffect(() => {
    if (codeParam && videoByCode?.id) {
      setSelectedVideoId(videoByCode.id)
      // ลบ param ออกจาก URL เพื่อไม่ให้ refresh แล้วเปิดซ้ำ
      setSearchParams({}, { replace: true })
    }
  }, [codeParam, videoByCode?.id, setSearchParams])

  const { data, isLoading, error } = useVideos(filters)
  const deleteVideo = useDeleteVideo()
  const queueTranscoding = useQueueTranscoding()
  const activeProgress = useVideoProgress()

  // Subtitle hooks
  const transcribe = useTranscribe()
  const translate = useTranslate()
  const { data: supportedLanguages } = useSupportedLanguages()

  const page = filters.page ?? 1
  const totalPages = data?.meta.totalPages ?? 1
  const videos = data?.data ?? []

  const setPage = (newPage: number | ((p: number) => number)) => {
    const nextPage = typeof newPage === 'function' ? newPage(page) : newPage
    setFilters((prev) => ({ ...prev, page: nextPage }))
  }

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString('th-TH', {
      day: 'numeric',
      month: 'short',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  const formatDuration = (seconds: number) => {
    if (!seconds || seconds <= 0) return '-'
    const mins = Math.floor(seconds / 60)
    const secs = seconds % 60
    return `${mins}:${secs.toString().padStart(2, '0')}`
  }

  const formatBytes = (bytes: number | undefined) => {
    if (!bytes || bytes === 0) return '-'
    const units = ['B', 'KB', 'MB', 'GB', 'TB']
    let i = 0
    let size = bytes
    while (size >= 1024 && i < units.length - 1) {
      size /= 1024
      i++
    }
    return `${size.toFixed(i > 1 ? 2 : 0)} ${units[i]}`
  }

  const getQualityCount = (qualitySizes: Record<string, number> | undefined) => {
    if (!qualitySizes) return 0
    return Object.keys(qualitySizes).length
  }

  const getSubtitleLabel = (summary: SubtitleSummary) => {
    const parts: string[] = []
    if (summary.original) {
      const lang = LANGUAGE_LABELS[summary.original.language] || summary.original.language
      parts.push(lang)
    }
    if (summary.translations && summary.translations.length > 0) {
      // แสดงภาษาที่แปล
      const translationLangs = summary.translations.map(t => {
        const lang = LANGUAGE_LABELS[t.language] || t.language
        const status = t.status === 'ready' ? '' : t.status === 'failed' ? '✗' : '...'
        return status ? `${lang}${status}` : lang
      })
      parts.push('→', translationLangs.join(', '))
    }
    return parts.length > 0 ? parts.join(' ') : 'ไม่มี'
  }

  const getSubtitleTooltip = (summary: SubtitleSummary) => {
    const lines: string[] = []
    if (summary.original) {
      const lang = LANGUAGE_LABELS[summary.original.language] || summary.original.language
      const status = summary.original.status === 'ready' ? '✓' : summary.original.status === 'failed' ? '✗' : '...'
      lines.push(`ต้นฉบับ: ${lang} ${status}`)
    }
    if (summary.translations && summary.translations.length > 0) {
      summary.translations.forEach(t => {
        const lang = LANGUAGE_LABELS[t.language] || t.language
        const status = t.status === 'ready' ? '✓' : t.status === 'failed' ? '✗' : '...'
        lines.push(`แปล: ${lang} ${status}`)
      })
    }
    return lines.join('\n')
  }

  const copyEmbedCode = (code: string) => {
    const embedCode = `<iframe src="${window.location.origin}/embed/${code}" width="640" height="360" frameborder="0" allowfullscreen></iframe>`
    navigator.clipboard.writeText(embedCode)
    toast.success('คัดลอก Embed Code แล้ว')
  }

  // หาภาษาที่สามารถแปลได้สำหรับ video
  const getAvailableTranslations = (summary: SubtitleSummary | undefined) => {
    if (!summary?.original || summary.original.status !== 'ready') return []
    if (!supportedLanguages?.translationPairs) return []

    const sourceLanguage = summary.original.language
    const possibleTargets = supportedLanguages.translationPairs[sourceLanguage] ?? []

    // กรองภาษาที่แปลแล้ว
    const alreadyTranslated = (summary.translations ?? []).map(t => t.language)
    return possibleTargets.filter(lang => !alreadyTranslated.includes(lang))
  }

  const handleTranslate = (videoId: string, targetLang: string) => {
    translate.mutate({ videoId, targetLanguages: [targetLang] })
  }

  const openDeleteDialog = (e: React.MouseEvent, id: string, title: string) => {
    e.stopPropagation()
    setDeleteTarget({ id, title })
  }

  const handleConfirmDelete = async () => {
    if (!deleteTarget) return

    const { id, title } = deleteTarget

    // ปิด dialog ทันที
    setDeleteTarget(null)

    // แสดง toast กำลังลบ
    const toastId = toast.loading(`กำลังลบ "${title}"...`, {
      description: 'กำลังลบไฟล์ต้นฉบับและ HLS',
    })

    // ลบใน background
    try {
      await deleteVideo.mutateAsync(id)
      toast.success(`ลบ "${title}" สำเร็จ`, {
        id: toastId,
        description: 'ลบไฟล์ทั้งหมดเรียบร้อยแล้ว',
      })
    } catch (error) {
      toast.error(`ลบ "${title}" ไม่สำเร็จ`, {
        id: toastId,
        description: error instanceof Error ? error.message : 'เกิดข้อผิดพลาด',
      })
    }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">วิดีโอ</h1>
          <p className="text-sm text-muted-foreground">
            {data ? `${data.meta.total} รายการ` : 'จัดการวิดีโอทั้งหมด'}
          </p>
        </div>
        <div className="flex gap-2">
          <Button size="sm" variant="outline" onClick={() => setBatchUploadDialogOpen(true)}>
            <Files className="h-4 w-4 mr-2" />
            อัปโหลดหลายไฟล์
          </Button>
          <Button size="sm" onClick={() => setUploadDialogOpen(true)}>
            <Plus className="h-4 w-4 mr-2" />
            อัปโหลดวิดีโอ
          </Button>
        </div>
      </div>

      {/* Filters */}
      <VideoFilters filters={filters} onFiltersChange={setFilters} />

      {/* Video List */}
      {isLoading ? (
        <div className="flex justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      ) : error ? (
        <p className="text-sm text-destructive py-8 text-center">เกิดข้อผิดพลาดในการโหลดข้อมูล</p>
      ) : videos.length === 0 ? (
        <Empty className="border">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <Video className="h-6 w-6" />
            </EmptyMedia>
            <EmptyTitle>ยังไม่มีวิดีโอ</EmptyTitle>
            <EmptyDescription>
              เริ่มอัปโหลดวิดีโอแรกของคุณ
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <Button size="sm" onClick={() => setUploadDialogOpen(true)}>
              <Plus className="h-4 w-4 mr-2" />
              อัปโหลดวิดีโอ
            </Button>
          </EmptyContent>
        </Empty>
      ) : (
        <div className="space-y-2">
          {videos.map((video) => (
            <div
              key={video.id}
              className="flex items-center gap-3 px-3 py-2.5 rounded-lg border border-dashed hover:bg-accent/50 transition-colors cursor-pointer leading-none"
              onClick={() => setSelectedVideoId(video.id)}
            >
              <Video className="h-4 w-4 text-muted-foreground shrink-0" />
              <div className="flex-1 min-w-0">
                <p className="font-medium truncate">{video.title}</p>
                <p className="flex items-center gap-3 mt-1 text-xs text-muted-foreground">
                  <span className="font-mono">{video.code}</span>
                  <span className="inline-flex items-center gap-1">
                    <Clock className="h-3 w-3" />
                    {formatDuration(video.duration)}
                  </span>
                  <span className="inline-flex items-center gap-1">
                    <Eye className="h-3 w-3" />
                    {video.views.toLocaleString()}
                  </span>
                  {video.status === 'ready' && video.diskUsage ? (
                    <span className="inline-flex items-center gap-1" title={getQualityCount(video.qualitySizes) > 0 ? Object.entries(video.qualitySizes || {}).map(([q, b]) => `${q}: ${formatBytes(b)}`).join(', ') : undefined}>
                      <HardDrive className="h-3 w-3" />
                      {formatBytes(video.diskUsage)}
                      {getQualityCount(video.qualitySizes) > 0 && (
                        <span className="text-muted-foreground/70">({getQualityCount(video.qualitySizes)} res)</span>
                      )}
                    </span>
                  ) : null}
                  {/* Reel Count */}
                  {video.status === 'ready' && video.reelCount !== undefined && video.reelCount > 0 && (
                    <button
                      className="inline-flex items-center gap-1 hover:text-primary transition-colors"
                      title={`ดู ${video.reelCount} Reels`}
                      onClick={(e) => {
                        e.stopPropagation()
                        navigate(`/reels?videoId=${video.id}`)
                      }}
                    >
                      <Film className="h-3 w-3" />
                      {video.reelCount}
                    </button>
                  )}
                  {/* Subtitle Status */}
                  {video.status === 'ready' && video.subtitleSummary && (
                    <span className="inline-flex items-center gap-1" title={getSubtitleTooltip(video.subtitleSummary)}>
                      <Languages className="h-3 w-3" />
                      {video.subtitleSummary.original ? (
                        video.subtitleSummary.original.status === 'ready' ? (
                          <CheckCircle2 className="h-3 w-3 text-status-success" />
                        ) : video.subtitleSummary.original.status === 'failed' ? (
                          <AlertCircle className="h-3 w-3 text-destructive" />
                        ) : (
                          <Loader2 className="h-3 w-3 animate-spin" />
                        )
                      ) : null}
                      <span className="text-muted-foreground/70">
                        {getSubtitleLabel(video.subtitleSummary)}
                      </span>
                    </span>
                  )}
                  {/* Category */}
                  {video.category && (
                    <span className="inline-flex items-center gap-1">
                      <Folder className="h-3 w-3" />
                      {video.category.name}
                    </span>
                  )}
                  <span>{formatDate(video.createdAt)}</span>
                </p>
              </div>
              {(() => {
                const progress = activeProgress.get(video.id)
                if (progress && progress.status !== 'completed' && progress.status !== 'failed') {
                  return (
                    <Badge variant="outline" className="gap-1.5 tabular-nums">
                      <Loader2 className="h-3 w-3 animate-spin" />
                      {progress.progress}%
                    </Badge>
                  )
                }
                return (
                  <Badge className={VIDEO_STATUS_STYLES[video.status]}>
                    {VIDEO_STATUS_LABELS[video.status]}
                  </Badge>
                )
              })()}
              <DropdownMenu>
                <DropdownMenuTrigger asChild onClick={(e) => e.stopPropagation()}>
                  <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0">
                    <MoreVertical className="h-4 w-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem onClick={() => setSelectedVideoId(video.id)}>
                    <Eye className="h-4 w-4 mr-2" />
                    ดูรายละเอียด
                  </DropdownMenuItem>
                  {video.status === 'ready' && (
                    <DropdownMenuItem onClick={() => copyEmbedCode(video.code)}>
                      <Copy className="h-4 w-4 mr-2" />
                      คัดลอก Embed
                    </DropdownMenuItem>
                  )}
                  {(video.status === 'pending' || video.status === 'failed') && (
                    <DropdownMenuItem
                      onClick={() => queueTranscoding.mutate(video.id)}
                      disabled={queueTranscoding.isPending}
                    >
                      <RefreshCw className="h-4 w-4 mr-2" />
                      {video.status === 'failed' ? 'ลองใหม่' : 'เริ่มประมวลผล'}
                    </DropdownMenuItem>
                  )}

                  {/* Subtitle Actions */}
                  {video.status === 'ready' && (
                    <>
                      <DropdownMenuSeparator />
                      {/* สร้าง Subtitle (ถ้ายังไม่มี original) */}
                      {!video.subtitleSummary?.original && (
                        <DropdownMenuItem
                          onClick={() => transcribe.mutate(video.id)}
                          disabled={transcribe.isPending}
                        >
                          <Sparkles className="h-4 w-4 mr-2" />
                          สร้าง Subtitle
                        </DropdownMenuItem>
                      )}

                      {/* แปลภาษา (ถ้ามี original ready) */}
                      {(() => {
                        const availableLangs = getAvailableTranslations(video.subtitleSummary)
                        if (availableLangs.length === 0) return null

                        return (
                          <DropdownMenuSub>
                            <DropdownMenuSubTrigger>
                              <Languages className="h-4 w-4 mr-2" />
                              แปลเป็น...
                            </DropdownMenuSubTrigger>
                            <DropdownMenuPortal>
                              <DropdownMenuSubContent>
                                {availableLangs.map((lang) => (
                                  <DropdownMenuItem
                                    key={lang}
                                    onClick={() => handleTranslate(video.id, lang)}
                                    disabled={translate.isPending}
                                  >
                                    {LANGUAGE_FLAGS[lang]} {LANGUAGE_LABELS[lang] || lang}
                                  </DropdownMenuItem>
                                ))}
                              </DropdownMenuSubContent>
                            </DropdownMenuPortal>
                          </DropdownMenuSub>
                        )
                      })()}
                    </>
                  )}

                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    className="text-destructive focus:text-destructive"
                    onClick={(e) => openDeleteDialog(e, video.id, video.title)}
                  >
                    <Trash2 className="h-4 w-4 mr-2" />
                    ลบ
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          ))}
        </div>
      )}

      {/* Pagination */}
      {data && totalPages > 1 && (
        <div className="flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            หน้า {page} / {totalPages}
          </p>
          <Pagination>
            <PaginationContent>
              <PaginationItem>
                <PaginationPrevious
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  className={page <= 1 ? 'pointer-events-none opacity-50' : 'cursor-pointer'}
                />
              </PaginationItem>
              {Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
                let pageNum: number
                if (totalPages <= 5) {
                  pageNum = i + 1
                } else if (page <= 3) {
                  pageNum = i + 1
                } else if (page >= totalPages - 2) {
                  pageNum = totalPages - 4 + i
                } else {
                  pageNum = page - 2 + i
                }
                return (
                  <PaginationItem key={pageNum}>
                    <PaginationLink
                      onClick={() => setPage(pageNum)}
                      isActive={page === pageNum}
                      className="cursor-pointer"
                    >
                      {pageNum}
                    </PaginationLink>
                  </PaginationItem>
                )
              })}
              <PaginationItem>
                <PaginationNext
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                  className={page >= totalPages ? 'pointer-events-none opacity-50' : 'cursor-pointer'}
                />
              </PaginationItem>
            </PaginationContent>
          </Pagination>
        </div>
      )}

      {/* Dialogs */}
      <VideoUploadDialog open={uploadDialogOpen} onOpenChange={setUploadDialogOpen} />
      <VideoBatchUploadDialog open={batchUploadDialogOpen} onOpenChange={setBatchUploadDialogOpen} />
      <VideoDetailSheet
        videoId={selectedVideoId}
        open={!!selectedVideoId}
        onOpenChange={(open) => !open && setSelectedVideoId(null)}
      />

      {/* Delete Confirmation Dialog */}
      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>ยืนยันการลบวิดีโอ</AlertDialogTitle>
            <AlertDialogDescription>
              คุณต้องการลบวิดีโอ <span className="font-medium text-foreground">"{deleteTarget?.title}"</span> หรือไม่?
              <br />
              <span className="text-destructive">การดำเนินการนี้จะลบไฟล์ต้นฉบับและไฟล์ HLS ทั้งหมด ไม่สามารถกู้คืนได้</span>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(null)}>
              ยกเลิก
            </Button>
            <Button variant="destructive" onClick={handleConfirmDelete}>
              <Trash2 className="h-4 w-4 mr-2" />
              ลบวิดีโอ
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
