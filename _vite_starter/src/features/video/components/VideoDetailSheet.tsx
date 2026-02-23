import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { Eye, Clock, RefreshCw, Copy, Check, ExternalLink, Play, Folder, X, Timer, Code2, Pencil, Save, Loader2, Images } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { useVideo, useQueueTranscoding, useUpdateVideo, useGenerateGallery, useRegenerateGallery } from '../hooks'
import { useCategories } from '@/features/category/hooks'
import { EmbedCodeDialog } from './EmbedCodeDialog'
import { SubtitlePanel } from '@/features/subtitle'
import { useStreamAccess } from '@/features/embed'
import { APP_CONFIG } from '@/constants/app-config'
import { VIDEO_STATUS_LABELS, VIDEO_STATUS_STYLES, VIDEO_STATUS_DESCRIPTIONS } from '@/constants/enums'
import { toast } from 'sonner'

interface VideoDetailSheetProps {
  videoId: string | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function VideoDetailSheet({ videoId, open, onOpenChange }: VideoDetailSheetProps) {
  const { data: video, isLoading } = useVideo(videoId ?? '')
  const { data: categories } = useCategories()
  const queueTranscoding = useQueueTranscoding()
  const updateVideo = useUpdateVideo()
  const generateGallery = useGenerateGallery()
  const regenerateGallery = useRegenerateGallery()
  const [copiedField, setCopiedField] = useState<string | null>(null)
  const [embedDialogOpen, setEmbedDialogOpen] = useState(false)

  // Edit mode state
  const [isEditing, setIsEditing] = useState(false)
  const [editTitle, setEditTitle] = useState('')
  const [editDescription, setEditDescription] = useState('')
  const [editCategoryId, setEditCategoryId] = useState<string>('')

  // Reset edit state when video changes or sheet opens
  useEffect(() => {
    if (video && open) {
      setEditTitle(video.title)
      setEditDescription(video.description ?? '')
      // API returns category as object, use category.id
      setEditCategoryId(video.category?.id ?? '')
    }
    setIsEditing(false)
  }, [video?.id, video?.category?.id, open])

  const handleSave = async () => {
    if (!video || !editTitle.trim()) return

    try {
      await updateVideo.mutateAsync({
        id: video.id,
        data: {
          title: editTitle.trim(),
          description: editDescription.trim() || undefined,
          categoryId: editCategoryId || undefined,
        },
      })
      setIsEditing(false)
      toast.success('บันทึกการเปลี่ยนแปลงแล้ว')
    } catch {
      // Error handled by mutation
    }
  }

  const handleCancelEdit = () => {
    setEditTitle(video?.title ?? '')
    setEditDescription(video?.description ?? '')
    setEditCategoryId(video?.category?.id ?? '')
    setIsEditing(false)
  }

  // ดึง stream token สำหรับ video ที่ ready (ต้องมี token เพื่อเข้าถึง CDN)
  const { data: streamAccess } = useStreamAccess(video?.code ?? '', {
    enabled: !!video?.code && video?.status === 'ready',
  })

  // State สำหรับ thumbnail blob URL
  const [thumbnailBlobUrl, setThumbnailBlobUrl] = useState<string | undefined>()

  // Reset thumbnail เมื่อ video เปลี่ยน หรือ sheet ปิด
  useEffect(() => {
    if (!open || !video?.code) {
      setThumbnailBlobUrl(undefined)
    }
  }, [open, video?.code])

  // Fetch thumbnail ด้วย token แล้วสร้าง Blob URL
  useEffect(() => {
    if (!video?.code || !streamAccess?.token || !open) return

    let cancelled = false

    const fetchThumbnail = async () => {
      try {
        const url = `${APP_CONFIG.streamUrl}/${video.code}/thumb.jpg`
        const response = await fetch(url, {
          headers: {
            'X-Stream-Token': streamAccess.token,
          },
        })

        if (!response.ok || cancelled) return

        const blob = await response.blob()
        if (cancelled) return

        const newBlobUrl = URL.createObjectURL(blob)
        setThumbnailBlobUrl(prev => {
          // Revoke URL เก่าถ้ามี
          if (prev) URL.revokeObjectURL(prev)
          return newBlobUrl
        })
      } catch {
        // Ignore errors
      }
    }

    fetchThumbnail()

    return () => {
      cancelled = true
    }
  }, [video?.code, streamAccess?.token, open])

  // Cleanup blob URL เมื่อ component unmount
  useEffect(() => {
    return () => {
      setThumbnailBlobUrl(prev => {
        if (prev) URL.revokeObjectURL(prev)
        return undefined
      })
    }
  }, [])

  const embedUrl = video?.code ? `${window.location.origin}/embed/${video.code}` : ''
  const previewUrl = video?.code ? `/preview/${video.code}` : ''
  const embedCode = video?.code
    ? `<iframe src="${embedUrl}" width="640" height="360" frameborder="0" allowfullscreen></iframe>`
    : ''

  const copyToClipboard = (text: string, field: string) => {
    navigator.clipboard.writeText(text)
    setCopiedField(field)
    toast.success('คัดลอกแล้ว')
    setTimeout(() => setCopiedField(null), 2000)
  }

  const formatDuration = (seconds: number) => {
    if (!seconds || seconds <= 0) return '-'
    const mins = Math.floor(seconds / 60)
    const secs = seconds % 60
    return `${mins}:${secs.toString().padStart(2, '0')}`
  }

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString('th-TH', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    })
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-full sm:max-w-2xl overflow-y-auto">
        <SheetHeader className="pb-4">
          <SheetTitle className="text-left pr-6">รายละเอียดวิดีโอ</SheetTitle>
        </SheetHeader>

        {isLoading && (
          <div className="space-y-4">
            <Skeleton className="aspect-video w-full rounded-lg" />
            <Skeleton className="h-6 w-3/4" />
            <Skeleton className="h-4 w-1/2" />
            <div className="space-y-2">
              {Array.from({ length: 4 }).map((_, i) => (
                <Skeleton key={i} className="h-10 w-full" />
              ))}
            </div>
          </div>
        )}

        {!isLoading && !video && (
          <div className="text-center py-8">
            <p className="text-sm text-destructive">ไม่พบวิดีโอ</p>
          </div>
        )}

        {video && (
          <div className="space-y-4 p-4">
            {/* Thumbnail + Preview Button or Status */}
            {video.status === 'ready' ? (
              <div className="relative rounded-lg overflow-hidden bg-black aspect-video group">
                {thumbnailBlobUrl ? (
                  <img
                    src={thumbnailBlobUrl}
                    alt={video.title}
                    className="w-full h-full object-cover"
                  />
                ) : (
                  <div className="w-full h-full flex items-center justify-center">
                    <RefreshCw className="size-6 text-white/50 animate-spin" />
                  </div>
                )}
                {/* Preview Button Overlay */}
                <div className="absolute inset-0 flex items-center justify-center bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity">
                  <Button
                    size="lg"
                    className="gap-2"
                    onClick={() => window.open(`/preview/${video.code}`, '_blank')}
                  >
                    <Play className="size-5" />
                    ดูตัวอย่าง
                  </Button>
                </div>
              </div>
            ) : (
              <div className="aspect-video bg-muted rounded-lg flex flex-col items-center justify-center p-4">
                {video.status === 'pending' && (
                  <>
                    <Clock className="size-10 text-muted-foreground mb-2" />
                    <p className="text-sm font-medium mb-1">รอการประมวลผล</p>
                    <Button
                      size="sm"
                      onClick={() => queueTranscoding.mutate(video.id)}
                      disabled={queueTranscoding.isPending}
                      className="mt-2"
                    >
                      <RefreshCw className={`size-4 mr-1.5 ${queueTranscoding.isPending ? 'animate-spin' : ''}`} />
                      เริ่มประมวลผล
                    </Button>
                  </>
                )}
                {video.status === 'queued' && (
                  <>
                    <Timer className="size-10 text-status-queued mb-2" />
                    <p className="text-sm font-medium text-status-queued mb-1">อยู่ในคิว</p>
                    <p className="text-xs text-muted-foreground">รอ Worker ประมวลผล</p>
                  </>
                )}
                {video.status === 'processing' && (
                  <>
                    <RefreshCw className="size-10 text-status-processing animate-spin mb-2" />
                    <p className="text-sm font-medium">กำลังประมวลผล...</p>
                  </>
                )}
                {video.status === 'failed' && (
                  <>
                    <X className="size-10 text-destructive mb-2" />
                    <p className="text-sm font-medium text-destructive mb-1">ประมวลผลล้มเหลว</p>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => queueTranscoding.mutate(video.id)}
                      disabled={queueTranscoding.isPending}
                      className="mt-2"
                    >
                      <RefreshCw className={`size-4 mr-1.5 ${queueTranscoding.isPending ? 'animate-spin' : ''}`} />
                      ลองใหม่
                    </Button>
                  </>
                )}
                {video.status === 'dead_letter' && (
                  <>
                    <Eye className="size-10 text-status-info mb-2" />
                    <p className="text-sm font-medium text-status-info mb-1">ทีมงานกำลังตรวจสอบ</p>
                    <p className="text-xs text-muted-foreground text-center max-w-[200px]">
                      {VIDEO_STATUS_DESCRIPTIONS.dead_letter}
                    </p>
                  </>
                )}
              </div>
            )}

            {/* Title & Status */}
            <div className="space-y-3">
              <div className="flex items-start justify-between gap-2">
                {isEditing ? (
                  <Input
                    value={editTitle}
                    onChange={(e) => setEditTitle(e.target.value)}
                    placeholder="ชื่อวิดีโอ"
                    className="flex-1 font-semibold"
                  />
                ) : (
                  <h3 className="font-semibold">{video.title}</h3>
                )}
                <span className={`shrink-0 px-2 py-0.5 text-[10px] font-medium rounded ${VIDEO_STATUS_STYLES[video.status]}`}>
                  {VIDEO_STATUS_LABELS[video.status]}
                </span>
              </div>

              {/* Category Edit */}
              {isEditing && (
                <div className="space-y-1.5">
                  <Label className="text-sm text-muted-foreground">หมวดหมู่</Label>
                  <Select value={editCategoryId || '_none'} onValueChange={(val) => setEditCategoryId(val === '_none' ? '' : val)}>
                    <SelectTrigger>
                      <SelectValue placeholder="เลือกหมวดหมู่" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="_none">ไม่มีหมวดหมู่</SelectItem>
                      {categories?.map((cat) => (
                        <SelectItem key={cat.id} value={cat.id}>
                          {cat.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              )}

              {/* Edit/Save Buttons */}
              <div className="flex items-center gap-2">
                {isEditing ? (
                  <>
                    <Button
                      size="sm"
                      onClick={handleSave}
                      disabled={updateVideo.isPending || !editTitle.trim()}
                    >
                      {updateVideo.isPending ? (
                        <Loader2 className="size-4 mr-1.5 animate-spin" />
                      ) : (
                        <Save className="size-4 mr-1.5" />
                      )}
                      บันทึก
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={handleCancelEdit}
                      disabled={updateVideo.isPending}
                    >
                      ยกเลิก
                    </Button>
                  </>
                ) : (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setIsEditing(true)}
                  >
                    <Pencil className="size-4 mr-1.5" />
                    แก้ไข
                  </Button>
                )}
              </div>

              {/* Description */}
              {isEditing ? (
                <div className="space-y-1.5">
                  <Label className="text-sm text-muted-foreground">รายละเอียด</Label>
                  <Textarea
                    value={editDescription}
                    onChange={(e) => setEditDescription(e.target.value)}
                    placeholder="รายละเอียดวิดีโอ (ไม่บังคับ)"
                    rows={3}
                  />
                </div>
              ) : video.description ? (
                <p className="text-sm text-muted-foreground">{video.description}</p>
              ) : null}
            </div>

            {/* Info - Inline */}
            <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-muted-foreground leading-none">
              <span className="font-mono">{video.code}</span>
              <span className="flex items-center gap-1">
                <Eye className="size-3" />
                {video.views.toLocaleString()}
              </span>
              <span className="flex items-center gap-1">
                <Clock className="size-3" />
                {formatDuration(video.duration)}
              </span>
              {video.quality && <span>{video.quality}</span>}
              {video.category && (
                <span className="flex items-center gap-1">
                  <Folder className="size-3" />
                  {video.category.name}
                </span>
              )}
              <span>{formatDate(video.createdAt)}</span>
            </div>

            {/* Links & Embed (only for ready videos) */}
            {video.status === 'ready' && (
              <div className="space-y-3">
                <p className="text-sm font-medium">ลิงก์และ Embed</p>

                {/* Embed URL */}
                <div className="space-y-1">
                  <label className="text-sm text-muted-foreground">Embed URL</label>
                  <div className="flex gap-1.5">
                    <input
                      type="text"
                      readOnly
                      value={embedUrl}
                      className="flex-1 px-2 py-1.5 text-sm bg-muted rounded font-mono truncate"
                    />
                    <Button
                      size="icon"
                      variant="outline"
                      className="size-8 shrink-0"
                      onClick={() => copyToClipboard(embedUrl, 'embed-url')}
                    >
                      {copiedField === 'embed-url' ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
                    </Button>
                    <Button
                      size="icon"
                      variant="outline"
                      className="size-8 shrink-0"
                      onClick={() => window.open(embedUrl, '_blank')}
                    >
                      <ExternalLink className="size-3.5" />
                    </Button>
                  </div>
                </div>

                {/* Embed Code */}
                <div className="space-y-1">
                  <label className="text-sm text-muted-foreground">Embed Code</label>
                  <div className="flex gap-1.5">
                    <input
                      type="text"
                      readOnly
                      value={embedCode}
                      className="flex-1 px-2 py-1.5 text-sm bg-muted rounded font-mono truncate"
                    />
                    <Button
                      size="icon"
                      variant="outline"
                      className="size-8 shrink-0"
                      onClick={() => copyToClipboard(embedCode, 'embed')}
                    >
                      {copiedField === 'embed' ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
                    </Button>
                    <Button
                      size="icon"
                      variant="outline"
                      className="size-8 shrink-0"
                      onClick={() => setEmbedDialogOpen(true)}
                      title="More embed options"
                    >
                      <Code2 className="size-3.5" />
                    </Button>
                  </div>
                </div>

                {/* Preview Button */}
                <Button className="w-full" size="sm" asChild>
                  <a href={previewUrl} target="_blank" rel="noopener noreferrer">
                    <Play className="size-4 mr-1.5" />
                    ดูตัวอย่าง
                  </a>
                </Button>
              </div>
            )}

            {/* Subtitle Panel */}
            {video.status === 'ready' && (
              <div className="pt-2 border-t">
                <SubtitlePanel
                  videoId={video.id}
                  videoCode={video.code}
                  videoStatus={video.status}
                />
              </div>
            )}

            {/* Gallery Section */}
            {video.status === 'ready' && (
              <div className="pt-2 border-t">
                {video.galleryCount && video.galleryCount > 0 ? (
                  <div className="flex gap-2">
                    <Button variant="outline" className="flex-1" asChild>
                      <Link to={`/gallery/${video.code}`}>
                        <Images className="size-4 mr-1.5" />
                        ดู Gallery ({video.galleryCount} ภาพ)
                      </Link>
                    </Button>
                    <Button
                      variant="outline"
                      onClick={() => {
                        regenerateGallery.mutate(video.id, {
                          onSuccess: () => {
                            toast.success('เริ่มสร้าง Gallery ใหม่แล้ว')
                          },
                          onError: () => {
                            toast.error('ไม่สามารถสร้าง Gallery ใหม่ได้')
                          },
                        })
                      }}
                      disabled={regenerateGallery.isPending}
                      title="สร้าง Gallery ใหม่"
                    >
                      {regenerateGallery.isPending ? (
                        <Loader2 className="size-4 animate-spin" />
                      ) : (
                        <RefreshCw className="size-4" />
                      )}
                    </Button>
                  </div>
                ) : (
                  <Button
                    variant="outline"
                    className="w-full"
                    onClick={() => {
                      generateGallery.mutate(video.id, {
                        onSuccess: () => {
                          toast.success('เริ่มสร้าง Gallery แล้ว')
                        },
                        onError: () => {
                          toast.error('ไม่สามารถสร้าง Gallery ได้')
                        },
                      })
                    }}
                    disabled={generateGallery.isPending}
                  >
                    {generateGallery.isPending ? (
                      <Loader2 className="size-4 mr-1.5 animate-spin" />
                    ) : (
                      <Images className="size-4 mr-1.5" />
                    )}
                    สร้าง Gallery
                  </Button>
                )}
              </div>
            )}

            {/* Embed Code Dialog */}
            {video && video.status === 'ready' && (
              <EmbedCodeDialog
                videoCode={video.code}
                videoTitle={video.title}
                open={embedDialogOpen}
                onOpenChange={setEmbedDialogOpen}
              />
            )}
          </div>
        )}
      </SheetContent>
    </Sheet>
  )
}
