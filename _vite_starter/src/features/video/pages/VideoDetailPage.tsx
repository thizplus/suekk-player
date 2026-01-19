import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Play, Eye, Clock, RefreshCw, Copy, Check, ExternalLink, Timer } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { useVideo, useQueueTranscoding } from '../hooks'
import { VideoPlayer } from '../components/VideoPlayer'
import { APP_CONFIG } from '@/constants/app-config'
import { VIDEO_STATUS_LABELS, VIDEO_STATUS_STYLES } from '@/constants/enums'
import { useState } from 'react'
import { toast } from 'sonner'

export function VideoDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { data: video, isLoading, error } = useVideo(id!)
  const queueTranscoding = useQueueTranscoding()
  const [copied, setCopied] = useState(false)

  const hlsUrl = video?.code ? `${APP_CONFIG.streamUrl}/${video.code}/master.m3u8` : ''
  const embedCode = video?.code
    ? `<iframe src="${window.location.origin}/embed/${video.code}" width="640" height="360" frameborder="0" allowfullscreen></iframe>`
    : ''

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text)
    setCopied(true)
    toast.success('คัดลอกแล้ว')
    setTimeout(() => setCopied(false), 2000)
  }

  const formatDuration = (seconds: number) => {
    const mins = Math.floor(seconds / 60)
    const secs = seconds % 60
    return `${mins}:${secs.toString().padStart(2, '0')}`
  }

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-48" />
        <Skeleton className="aspect-video w-full max-w-4xl" />
        <div className="grid gap-4 md:grid-cols-2">
          <Skeleton className="h-40" />
          <Skeleton className="h-40" />
        </div>
      </div>
    )
  }

  if (error || !video) {
    return (
      <div className="text-center py-12">
        <p className="text-destructive mb-4">ไม่พบวิดีโอ</p>
        <Button onClick={() => navigate('/videos')}>กลับไปรายการวิดีโอ</Button>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" onClick={() => navigate('/videos')}>
          <ArrowLeft className="size-5" />
        </Button>
        <div className="flex-1">
          <h1 className="text-2xl font-bold">{video.title}</h1>
          <div className="flex items-center gap-4 mt-1 text-sm text-muted-foreground">
            <span className="flex items-center gap-1">
              <Eye className="size-4" />
              {video.views.toLocaleString()} views
            </span>
            <span className="flex items-center gap-1">
              <Clock className="size-4" />
              {new Date(video.createdAt).toLocaleDateString('th-TH', {
                year: 'numeric',
                month: 'long',
                day: 'numeric',
              })}
            </span>
            <span className={`px-2 py-0.5 rounded text-xs font-medium ${VIDEO_STATUS_STYLES[video.status]}`}>
              {VIDEO_STATUS_LABELS[video.status]}
            </span>
          </div>
        </div>
      </div>

      {/* Video Player or Status */}
      {video.status === 'ready' ? (
        <VideoPlayer
          src={hlsUrl}
          poster={`${APP_CONFIG.streamUrl}/${video.code}/thumb.jpg`}
        />
      ) : (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            {video.status === 'pending' && (
              <>
                <Clock className="size-16 text-muted-foreground mb-4" />
                <p className="text-lg font-medium mb-2">วิดีโอรอการประมวลผล</p>
                <p className="text-sm text-muted-foreground mb-4">
                  คลิกปุ่มด้านล่างเพื่อเริ่มแปลงวิดีโอเป็น HLS
                </p>
                <Button
                  onClick={() => queueTranscoding.mutate(video.id)}
                  disabled={queueTranscoding.isPending}
                >
                  <RefreshCw className={`size-4 mr-2 ${queueTranscoding.isPending ? 'animate-spin' : ''}`} />
                  เริ่มประมวลผล
                </Button>
              </>
            )}
            {video.status === 'queued' && (
              <>
                <Timer className="size-16 text-status-queued mb-4" />
                <p className="text-lg font-medium text-status-queued mb-2">วิดีโออยู่ในคิว</p>
                <p className="text-sm text-muted-foreground">
                  รอ Worker ประมวลผล กรุณารอสักครู่
                </p>
              </>
            )}
            {video.status === 'processing' && (
              <>
                <RefreshCw className="size-16 text-status-processing animate-spin mb-4" />
                <p className="text-lg font-medium mb-2">กำลังประมวลผลวิดีโอ</p>
                <p className="text-sm text-muted-foreground">
                  กรุณารอสักครู่ ระบบกำลังแปลงวิดีโอเป็น HLS
                </p>
              </>
            )}
            {video.status === 'failed' && (
              <>
                <div className="size-16 rounded-full bg-destructive/10 flex items-center justify-center mb-4">
                  <span className="text-3xl">!</span>
                </div>
                <p className="text-lg font-medium mb-2 text-destructive">การประมวลผลล้มเหลว</p>
                <p className="text-sm text-muted-foreground mb-4">
                  เกิดข้อผิดพลาดในการแปลงวิดีโอ กรุณาลองใหม่
                </p>
                <Button
                  onClick={() => queueTranscoding.mutate(video.id)}
                  disabled={queueTranscoding.isPending}
                >
                  <RefreshCw className={`size-4 mr-2 ${queueTranscoding.isPending ? 'animate-spin' : ''}`} />
                  ลองใหม่
                </Button>
              </>
            )}
          </CardContent>
        </Card>
      )}

      {/* Info Cards */}
      <div className="grid gap-4 md:grid-cols-2">
        {/* Video Info */}
        <Card>
          <CardHeader>
            <CardTitle>ข้อมูลวิดีโอ</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="flex justify-between">
              <span className="text-muted-foreground">รหัส</span>
              <span className="font-mono">{video.code}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">ความยาว</span>
              <span>{video.duration > 0 ? formatDuration(video.duration) : '-'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">คุณภาพ</span>
              <span>{video.quality || '-'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">หมวดหมู่</span>
              <span>{video.category?.name || '-'}</span>
            </div>
            {video.description && (
              <div className="pt-2 border-t">
                <p className="text-sm text-muted-foreground">{video.description}</p>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Embed & Links */}
        {video.status === 'ready' && (
          <Card>
            <CardHeader>
              <CardTitle>ลิงก์และ Embed</CardTitle>
              <CardDescription>ใช้สำหรับแชร์หรือฝังวิดีโอในเว็บไซต์</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              {/* HLS URL */}
              <div>
                <label className="text-sm font-medium">HLS URL</label>
                <div className="flex gap-2 mt-1">
                  <input
                    type="text"
                    readOnly
                    value={hlsUrl}
                    className="flex-1 px-3 py-2 text-sm bg-muted rounded-md font-mono truncate"
                  />
                  <Button
                    size="icon"
                    variant="outline"
                    onClick={() => copyToClipboard(hlsUrl)}
                  >
                    {copied ? <Check className="size-4" /> : <Copy className="size-4" />}
                  </Button>
                  <Button
                    size="icon"
                    variant="outline"
                    onClick={() => window.open(hlsUrl, '_blank')}
                  >
                    <ExternalLink className="size-4" />
                  </Button>
                </div>
              </div>

              {/* Embed Code */}
              <div>
                <label className="text-sm font-medium">Embed Code</label>
                <div className="flex gap-2 mt-1">
                  <input
                    type="text"
                    readOnly
                    value={embedCode}
                    className="flex-1 px-3 py-2 text-sm bg-muted rounded-md font-mono truncate"
                  />
                  <Button
                    size="icon"
                    variant="outline"
                    onClick={() => copyToClipboard(embedCode)}
                  >
                    <Copy className="size-4" />
                  </Button>
                </div>
              </div>

              {/* Direct Play Button */}
              <Button className="w-full" asChild>
                <a href={hlsUrl} target="_blank" rel="noopener noreferrer">
                  <Play className="size-4 mr-2" />
                  เปิดในแท็บใหม่
                </a>
              </Button>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  )
}
