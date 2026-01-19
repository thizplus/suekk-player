import { useState } from 'react'
import { RefreshCw, Clock, XCircle, Loader2, Play, Video, CheckCircle, AlertCircle, Timer } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Progress } from '@/components/ui/progress'
import { Badge } from '@/components/ui/badge'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  Empty,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
  EmptyDescription,
} from '@/components/ui/empty'
import { useTranscodingStats, useVideos, useQueueTranscoding } from '@/features/video'
import { useVideoProgress, type VideoProgress } from '@/lib/websocket-provider'

type StatusType = 'pending' | 'queued' | 'processing' | 'failed'

const STATUS_CONFIG = {
  pending: {
    label: 'รอดำเนินการ',
    description: 'วิดีโอที่รอเข้าคิวประมวลผล',
    icon: Clock,
    colorClass: 'text-status-pending',
    emptyText: 'ไม่มีวิดีโอรอดำเนินการ',
  },
  queued: {
    label: 'อยู่ในคิว',
    description: 'วิดีโอที่รอ Worker ประมวลผล',
    icon: Timer,
    colorClass: 'text-status-queued',
    emptyText: 'ไม่มีวิดีโออยู่ในคิว',
  },
  processing: {
    label: 'กำลังประมวลผล',
    description: 'วิดีโอที่กำลังแปลงเป็น HLS',
    icon: Loader2,
    colorClass: 'text-status-processing',
    emptyText: 'ไม่มีวิดีโอกำลังประมวลผล',
  },
  failed: {
    label: 'ล้มเหลว',
    description: 'วิดีโอที่ประมวลผลไม่สำเร็จ',
    icon: XCircle,
    colorClass: 'text-destructive',
    emptyText: 'ไม่มีวิดีโอล้มเหลว',
  },
} as const

// Progress Item Component
function ProgressItem({ progress }: { progress: VideoProgress }) {
  const isFinished = progress.status === 'completed' || progress.status === 'failed'
  const getTypeLabel = () => progress.type === 'transcode' ? 'แปลงไฟล์' : 'อัพโหลด'

  // Minimal version สำหรับ completed/failed
  if (isFinished) {
    return (
      <div className={`flex items-center gap-2 px-3 py-2 rounded-md text-sm ${
        progress.status === 'completed' ? 'status-success' : 'status-danger'
      }`}>
        {progress.status === 'completed'
          ? <CheckCircle className="h-3.5 w-3.5 shrink-0" />
          : <AlertCircle className="h-3.5 w-3.5 shrink-0" />
        }
        <span className="truncate">{progress.videoTitle || progress.videoCode}</span>
        <span className="text-xs opacity-70 shrink-0">{getTypeLabel()}</span>
      </div>
    )
  }

  // Full version สำหรับ processing
  return (
    <div className="rounded-lg border p-3 space-y-2">
      <div className="flex items-center gap-3">
        <Loader2 className="h-4 w-4 animate-spin text-primary shrink-0" />
        <div className="flex-1 min-w-0">
          <p className="font-medium truncate text-sm">{progress.videoTitle || 'ไม่มีชื่อ'}</p>
          <p className="text-xs text-muted-foreground">
            {progress.videoCode} • {getTypeLabel()}
          </p>
        </div>
        <span className="text-sm font-semibold tabular-nums">{progress.progress}%</span>
      </div>
      <Progress value={progress.progress} className="h-1.5" />
      {progress.currentStep && (
        <p className="text-xs text-muted-foreground">{progress.currentStep}</p>
      )}
    </div>
  )
}

export function TranscodingPage() {
  const [selectedStatus, setSelectedStatus] = useState<StatusType | null>(null)
  const { data: stats, isLoading: statsLoading, refetch: refetchStats } = useTranscodingStats()
  const { data: pendingVideos, isLoading: pendingLoading, refetch: refetchPending } = useVideos({ status: 'pending', limit: 50 })
  const { data: queuedVideos, isLoading: queuedLoading, refetch: refetchQueued } = useVideos({ status: 'queued', limit: 50 })
  const { data: processingVideos, isLoading: processingLoading, refetch: refetchProcessing } = useVideos({ status: 'processing', limit: 50 })
  const { data: failedVideos, isLoading: failedLoading, refetch: refetchFailed } = useVideos({ status: 'failed', limit: 50 })
  const queueTranscoding = useQueueTranscoding()
  const activeProgress = useVideoProgress()

  // Refetch เมื่อเปิด sheet
  const handleSelectStatus = (status: StatusType) => {
    setSelectedStatus(status)
    // Refetch ข้อมูลล่าสุด
    refetchStats()
    switch (status) {
      case 'pending': refetchPending(); break
      case 'queued': refetchQueued(); break
      case 'processing': refetchProcessing(); break
      case 'failed': refetchFailed(); break
    }
  }

  const getVideosByStatus = (status: StatusType) => {
    switch (status) {
      case 'pending': return pendingVideos?.data ?? []
      case 'queued': return queuedVideos?.data ?? []
      case 'processing': return processingVideos?.data ?? []
      case 'failed': return failedVideos?.data ?? []
    }
  }

  const isSheetLoading = (status: StatusType) => {
    switch (status) {
      case 'pending': return pendingLoading
      case 'queued': return queuedLoading
      case 'processing': return processingLoading
      case 'failed': return failedLoading
    }
  }

  const handleQueueAll = async () => {
    if (!pendingVideos?.data) return
    for (const video of pendingVideos.data) {
      await queueTranscoding.mutateAsync(video.id)
    }
    refetchStats()
  }

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString('th-TH', {
      day: 'numeric',
      month: 'short',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  const selectedVideos = selectedStatus ? getVideosByStatus(selectedStatus) : []
  const selectedConfig = selectedStatus ? STATUS_CONFIG[selectedStatus] : null

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">ประมวลผล</h1>
          <p className="text-sm text-muted-foreground">จัดการคิวแปลงวิดีโอเป็น HLS</p>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => refetchStats()}
          disabled={statsLoading}
        >
          <RefreshCw className={`h-4 w-4 mr-2 ${statsLoading ? 'animate-spin' : ''}`} />
          รีเฟรช
        </Button>
      </div>

      {/* Stats - Big numbers */}
      <div className="flex items-center gap-6 flex-wrap">
        <div>
          <p className="text-muted-foreground text-sm mb-1">รอดำเนินการ</p>
          {statsLoading ? (
            <Skeleton className="h-8 w-16" />
          ) : (
            <p className="text-3xl font-semibold tabular-nums">{stats?.pending ?? 0}</p>
          )}
        </div>
        <Separator orientation="vertical" className="h-12" />
        <div>
          <p className="text-muted-foreground text-sm mb-1 flex items-center gap-1">
            <Timer className="h-3 w-3" />
            อยู่ในคิว
          </p>
          {statsLoading ? (
            <Skeleton className="h-8 w-16" />
          ) : (
            <p className="text-3xl font-semibold tabular-nums text-status-queued">{stats?.queued ?? 0}</p>
          )}
        </div>
        <Separator orientation="vertical" className="h-12" />
        <div>
          <p className="text-muted-foreground text-sm mb-1">กำลังประมวลผล</p>
          {statsLoading ? (
            <Skeleton className="h-8 w-16" />
          ) : (
            <p className="text-3xl font-semibold tabular-nums">{stats?.processing ?? 0}</p>
          )}
        </div>
        <Separator orientation="vertical" className="h-12" />
        <div>
          <p className="text-muted-foreground text-sm mb-1">เสร็จสิ้น</p>
          {statsLoading ? (
            <Skeleton className="h-8 w-16" />
          ) : (
            <p className="text-3xl font-semibold tabular-nums">{stats?.completed ?? 0}</p>
          )}
        </div>
        <Separator orientation="vertical" className="h-12" />
        <div>
          <p className="text-muted-foreground text-sm mb-1">ล้มเหลว</p>
          {statsLoading ? (
            <Skeleton className="h-8 w-16" />
          ) : (
            <p className="text-3xl font-semibold tabular-nums">{stats?.failed ?? 0}</p>
          )}
        </div>
      </div>

      {/* Active Progress - Real-time */}
      {activeProgress.size > 0 && (
        <div className="space-y-4">
          {/* Upload Progress */}
          {Array.from(activeProgress.values()).some(p => p.type === 'upload') && (
            <div className="space-y-3">
              <div className="flex items-center gap-2">
                <Loader2 className="h-4 w-4 animate-spin text-status-info" />
                <p className="text-sm font-medium">กำลังอัพโหลด</p>
              </div>
              <div className="grid gap-3">
                {Array.from(activeProgress.values())
                  .filter(p => p.type === 'upload')
                  .map((progress) => (
                    <ProgressItem key={progress.videoId} progress={progress} />
                  ))}
              </div>
            </div>
          )}

          {/* Transcode Progress */}
          {Array.from(activeProgress.values()).some(p => p.type === 'transcode') && (
            <div className="space-y-3">
              <div className="flex items-center gap-2">
                <Loader2 className="h-4 w-4 animate-spin text-primary" />
                <p className="text-sm font-medium">
                  กำลังประมวลผล ({Array.from(activeProgress.values()).filter(p => p.type === 'transcode').length})
                </p>
              </div>
              <div className="grid gap-3">
                {Array.from(activeProgress.values())
                  .filter(p => p.type === 'transcode')
                  .map((progress) => (
                    <ProgressItem key={progress.videoId} progress={progress} />
                  ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Status Items */}
      <div className="space-y-3">
        <p className="text-muted-foreground text-sm">จัดการคิว</p>
        <div className="flex items-center gap-4 h-5">
          <button
            onClick={() => handleSelectStatus('pending')}
            className="flex items-center gap-2 text-sm hover:text-foreground text-muted-foreground transition-colors"
          >
            <Clock className="h-4 w-4" />
            <span>รอ</span>
            <span className="font-semibold tabular-nums text-foreground">{stats?.pending ?? 0}</span>
          </button>
          <Separator orientation="vertical" />
          <button
            onClick={() => handleSelectStatus('queued')}
            className="flex items-center gap-2 text-sm hover:text-foreground text-muted-foreground transition-colors"
          >
            <Timer className="h-4 w-4 text-status-queued" />
            <span>คิว</span>
            <span className="font-semibold tabular-nums text-foreground">{stats?.queued ?? 0}</span>
          </button>
          <Separator orientation="vertical" />
          <button
            onClick={() => handleSelectStatus('processing')}
            className="flex items-center gap-2 text-sm hover:text-foreground text-muted-foreground transition-colors"
          >
            <Loader2 className={`h-4 w-4 ${stats?.processing ? 'animate-spin' : ''}`} />
            <span>กำลังทำ</span>
            <span className="font-semibold tabular-nums text-foreground">{stats?.processing ?? 0}</span>
          </button>
          <Separator orientation="vertical" />
          <button
            onClick={() => handleSelectStatus('failed')}
            className="flex items-center gap-2 text-sm hover:text-foreground text-muted-foreground transition-colors"
          >
            <XCircle className="h-4 w-4" />
            <span>ล้มเหลว</span>
            <span className="font-semibold tabular-nums text-foreground">{stats?.failed ?? 0}</span>
          </button>
        </div>
      </div>

      {/* Status Sheet */}
      <Sheet open={!!selectedStatus} onOpenChange={(open) => !open && setSelectedStatus(null)}>
        <SheetContent className="w-[calc(100%-2rem)] max-w-lg overflow-y-auto">
          <SheetHeader className="pb-4">
            <SheetTitle className="text-left pr-6 flex items-center gap-2">
              {selectedConfig && (
                <>
                  <selectedConfig.icon className={`h-5 w-5 ${selectedConfig.colorClass} ${selectedStatus === 'processing' ? 'animate-spin' : ''}`} />
                  {selectedConfig.label}
                </>
              )}
            </SheetTitle>
          </SheetHeader>

          <div className="p-4 space-y-4">
            {/* Queue All Button for Pending */}
            {selectedStatus === 'pending' && selectedVideos.length > 0 && (
              <Button
                size="sm"
                className="w-full"
                onClick={handleQueueAll}
                disabled={queueTranscoding.isPending}
              >
                <Play className="h-4 w-4 mr-2" />
                เริ่มประมวลผลทั้งหมด ({selectedVideos.length})
              </Button>
            )}

            {selectedStatus && isSheetLoading(selectedStatus) ? (
              <div className="flex justify-center py-12">
                <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
              </div>
            ) : selectedVideos.length === 0 ? (
              <Empty>
                <EmptyHeader>
                  <EmptyMedia variant="icon">
                    {selectedConfig && <selectedConfig.icon className="h-6 w-6" />}
                  </EmptyMedia>
                  <EmptyTitle>{selectedConfig?.emptyText}</EmptyTitle>
                  <EmptyDescription>
                    ไม่มีวิดีโอในสถานะนี้
                  </EmptyDescription>
                </EmptyHeader>
              </Empty>
            ) : (
              <div className="space-y-2">
                {selectedVideos.map((video) => {
                  // ใช้ composite key: videoId-type (ตรงกับ websocket-provider)
                  const videoProgress = activeProgress.get(`${video.id}-transcode`)

                  return (
                    <div
                      key={video.id}
                      className="rounded-lg border border-dashed hover:bg-accent/50 transition-colors"
                    >
                      <div className="flex items-center gap-3 px-3 py-2.5 leading-none">
                        <Video className="h-4 w-4 text-muted-foreground shrink-0" />
                        <div className="flex-1 min-w-0">
                          <p className="font-medium truncate">{video.title}</p>
                          <p className="text-xs text-muted-foreground mt-1">
                            {formatDate(video.createdAt)}
                          </p>
                        </div>
                        {selectedStatus === 'pending' && (
                          <Button
                            size="icon"
                            variant="outline"
                            className="h-7 w-7 shrink-0"
                            onClick={() => queueTranscoding.mutate(video.id)}
                            disabled={queueTranscoding.isPending}
                          >
                            <Play className="h-3 w-3" />
                          </Button>
                        )}
                        {selectedStatus === 'queued' && (
                          <Badge variant="outline" className="gap-1 text-status-queued border-status-queued shrink-0">
                            <Timer className="h-3 w-3" />
                            รอ Worker
                          </Badge>
                        )}
                        {selectedStatus === 'processing' && (
                          <span className="text-xs font-medium tabular-nums text-primary">
                            {videoProgress ? `${videoProgress.progress}%` : '...'}
                          </span>
                        )}
                        {selectedStatus === 'failed' && (
                          <Button
                            size="icon"
                            variant="outline"
                            className="h-7 w-7 shrink-0"
                            onClick={() => queueTranscoding.mutate(video.id)}
                            disabled={queueTranscoding.isPending}
                          >
                            <RefreshCw className="h-3 w-3" />
                          </Button>
                        )}
                      </div>

                      {/* Progress bar for processing videos */}
                      {selectedStatus === 'processing' && videoProgress && (
                        <div className="px-3 pb-2.5 space-y-1">
                          <Progress value={videoProgress.progress} className="h-1.5" />
                          <p className="text-xs text-muted-foreground">
                            {videoProgress.message}
                          </p>
                        </div>
                      )}
                    </div>
                  )
                })}
              </div>
            )}
          </div>
        </SheetContent>
      </Sheet>
    </div>
  )
}
