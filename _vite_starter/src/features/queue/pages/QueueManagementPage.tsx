import { useState } from 'react'
import { Link } from 'react-router-dom'
import {
  RefreshCw,
  RotateCcw,
  Flame,
  ExternalLink,
  Trash2,
  Plus,
  CheckCircle,
  XCircle,
  Monitor,
  Languages,
  Database,
  Images,
  Film,
  Wifi,
  WifiOff,
} from 'lucide-react'
import { Progress } from '@/components/ui/progress'
import { useWebSocketConnection, useVideoProgress, type VideoProgress } from '@/lib/websocket-provider'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  useLegacyStats,
  useFailedJobs,
  useProcessingJobs,
  useRetryJob,
  useRetrySubtitleAll,
  useClearSubtitleAll,
  useQueueMissingSubtitles,
  useWarmCachePending,
  useWarmCacheFailed,
  useWarmCacheOne,
  useWarmCacheAll,
  useDeleteOrphanedJobs,
  useDeleteCompletedJobs,
  useDeleteFailedJobs,
  usePurgeTranscodeStream,
} from '../hooks'
import type { WorkerJob, WarmCacheQueueItem } from '../types'

export function QueueManagementPage() {
  const { data: stats, isLoading: statsLoading, refetch: refetchStats } = useLegacyStats()
  const { isConnected, reconnect } = useWebSocketConnection()
  const activeProgress = useVideoProgress()

  // Cleanup mutations
  const deleteOrphaned = useDeleteOrphanedJobs()
  const deleteCompleted = useDeleteCompletedJobs()
  const deleteFailed = useDeleteFailedJobs()

  // Calculate totals for header
  const totalProcessing =
    (stats?.transcode?.processing || 0) +
    (stats?.subtitle?.processing || 0) +
    (stats?.warmCache?.warming || 0) +
    (stats?.gallery?.processing || 0) +
    (stats?.reel?.exporting || 0)

  return (
    <TooltipProvider>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <div className="flex items-center gap-3">
              <h1 className="text-2xl font-semibold">Queue Dashboard</h1>
              {totalProcessing > 0 && (
                <Badge className="bg-blue-500 text-white animate-pulse">
                  {totalProcessing} กำลังทำงาน
                </Badge>
              )}
              {/* WebSocket Status */}
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="sm"
                    className={`gap-1 ${isConnected ? 'text-green-500' : 'text-red-500'}`}
                    onClick={() => !isConnected && reconnect()}
                  >
                    {isConnected ? <Wifi className="h-4 w-4" /> : <WifiOff className="h-4 w-4" />}
                    <span className="text-xs">{isConnected ? 'Live' : 'Offline'}</span>
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  {isConnected ? 'WebSocket เชื่อมต่อแล้ว - รับ progress แบบ real-time' : 'คลิกเพื่อเชื่อมต่อใหม่'}
                </TooltipContent>
              </Tooltip>
            </div>
            <p className="text-sm text-muted-foreground mt-1">
              จัดการและดูสถานะคิวงานทั้งหมด
            </p>
          </div>
          <div className="flex gap-2">
            {/* Cleanup Buttons */}
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => deleteOrphaned.mutate()}
                  disabled={deleteOrphaned.isPending}
                >
                  <Trash2 className={`h-4 w-4 mr-2 ${deleteOrphaned.isPending ? 'animate-pulse' : ''}`} />
                  ลบ Orphaned
                </Button>
              </TooltipTrigger>
              <TooltipContent>ลบ jobs ที่ video/subtitle ถูกลบไปแล้ว</TooltipContent>
            </Tooltip>

            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => deleteCompleted.mutate(7)}
                  disabled={deleteCompleted.isPending}
                >
                  <Trash2 className={`h-4 w-4 mr-2 ${deleteCompleted.isPending ? 'animate-pulse' : ''}`} />
                  ลบ Completed
                </Button>
              </TooltipTrigger>
              <TooltipContent>ลบ jobs ที่สำเร็จแล้ว (เก่ากว่า 7 วัน)</TooltipContent>
            </Tooltip>

            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={() => deleteFailed.mutate()}
                  disabled={deleteFailed.isPending}
                >
                  <Trash2 className={`h-4 w-4 mr-2 ${deleteFailed.isPending ? 'animate-pulse' : ''}`} />
                  ลบ Failed
                </Button>
              </TooltipTrigger>
              <TooltipContent>ลบ jobs ที่ล้มเหลวทั้งหมด</TooltipContent>
            </Tooltip>

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
        </div>

        {/* Active Progress - Real-time via WebSocket */}
        {activeProgress.size > 0 && (
          <ActiveProgressSection activeProgress={activeProgress} />
        )}

        {/* Stats Overview */}
        {statsLoading ? (
          <div className="grid grid-cols-5 gap-4">
            {[...Array(5)].map((_, i) => (
              <Skeleton key={i} className="h-20" />
            ))}
          </div>
        ) : stats ? (
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-4">
            <StatBox
              icon={<Monitor className="h-5 w-5" />}
              label="แปลงวิดีโอ"
              processing={stats.transcode.processing || 0}
              failed={(stats.transcode.failed || 0) + (stats.transcode.deadLetter || 0)}
              pending={(stats.transcode.pending || 0) + (stats.transcode.queued || 0)}
            />
            <StatBox
              icon={<Languages className="h-5 w-5" />}
              label="ซับไตเติ้ล"
              processing={stats.subtitle.processing || 0}
              failed={stats.subtitle.failed || 0}
              pending={stats.subtitle.queued || 0}
            />
            <StatBox
              icon={<Database className="h-5 w-5" />}
              label="แคช CDN"
              processing={stats.warmCache.warming || 0}
              failed={stats.warmCache.failed || 0}
              pending={stats.warmCache.notCached || 0}
              success={stats.warmCache.cached || 0}
            />
            <StatBox
              icon={<Images className="h-5 w-5" />}
              label="Gallery"
              processing={stats.gallery?.processing || 0}
              failed={stats.gallery?.failed || 0}
              pending={stats.gallery?.pendingReview || 0}
            />
            <StatBox
              icon={<Film className="h-5 w-5" />}
              label="Reel"
              processing={stats.reel?.exporting || 0}
              failed={stats.reel?.failed || 0}
            />
          </div>
        ) : null}

        {/* Tabs */}
        <Tabs defaultValue="transcode" className="space-y-4">
          <TabsList>
            <TabsTrigger value="transcode" className="gap-2">
              <Monitor className="h-4 w-4" />
              แปลงวิดีโอ
              <CountBadge count={(stats?.transcode?.failed || 0) + (stats?.transcode?.deadLetter || 0)} />
            </TabsTrigger>
            <TabsTrigger value="subtitle" className="gap-2">
              <Languages className="h-4 w-4" />
              ซับไตเติ้ล
              <CountBadge count={(stats?.subtitle?.queued || 0) + (stats?.subtitle?.failed || 0)} />
            </TabsTrigger>
            <TabsTrigger value="warmcache" className="gap-2">
              <Database className="h-4 w-4" />
              แคช CDN
              <CountBadge count={stats?.warmCache?.notCached || 0} variant="warning" />
            </TabsTrigger>
            <TabsTrigger value="gallery" className="gap-2">
              <Images className="h-4 w-4" />
              Gallery
              <CountBadge count={stats?.gallery?.failed || 0} />
            </TabsTrigger>
            <TabsTrigger value="reel" className="gap-2">
              <Film className="h-4 w-4" />
              Reel
              <CountBadge count={stats?.reel?.failed || 0} />
            </TabsTrigger>
          </TabsList>

          <TabsContent value="transcode">
            <TranscodeTab />
          </TabsContent>
          <TabsContent value="subtitle">
            <SubtitleTab />
          </TabsContent>
          <TabsContent value="warmcache">
            <WarmCacheTab />
          </TabsContent>
          <TabsContent value="gallery">
            <GalleryTab />
          </TabsContent>
          <TabsContent value="reel">
            <ReelTab />
          </TabsContent>
        </Tabs>
      </div>
    </TooltipProvider>
  )
}

// ==================== Stats Box ====================

function StatBox({
  icon,
  label,
  processing = 0,
  failed = 0,
  pending = 0,
  success,
}: {
  icon: React.ReactNode
  label: string
  processing?: number
  failed?: number
  pending?: number
  success?: number
}) {
  const hasIssue = failed > 0 || pending > 0

  return (
    <div className={`rounded-lg border p-4 ${hasIssue ? 'border-yellow-500/50 bg-yellow-500/5' : ''}`}>
      <div className="flex items-center gap-2 text-muted-foreground mb-2">
        {icon}
        <span className="text-sm font-medium">{label}</span>
      </div>
      <div className="flex items-center gap-3 text-sm">
        {processing > 0 && (
          <span className="text-blue-500 flex items-center gap-1">
            <span className="relative flex h-2 w-2">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75"></span>
              <span className="relative inline-flex rounded-full h-2 w-2 bg-blue-500"></span>
            </span>
            {processing}
          </span>
        )}
        {pending > 0 && <span className="text-yellow-600">{pending} รอ</span>}
        {failed > 0 && <span className="text-red-500">{failed} ผิดพลาด</span>}
        {success !== undefined && success > 0 && (
          <span className="text-green-500">{success} สำเร็จ</span>
        )}
        {processing === 0 && pending === 0 && failed === 0 && (
          <span className="text-muted-foreground">-</span>
        )}
      </div>
    </div>
  )
}

function CountBadge({ count, variant = 'error' }: { count: number; variant?: 'error' | 'warning' }) {
  if (count === 0) return null
  return (
    <Badge
      variant={variant === 'error' ? 'destructive' : 'secondary'}
      className="h-5 px-1.5 text-xs"
    >
      {count}
    </Badge>
  )
}

// ==================== Active Progress Section ====================

function ActiveProgressSection({ activeProgress }: { activeProgress: Map<string, VideoProgress> }) {
  const progressItems = Array.from(activeProgress.values())

  // Group by type
  const transcodeItems = progressItems.filter(p => p.type === 'transcode')
  const subtitleItems = progressItems.filter(p => p.type === 'subtitle')
  const galleryItems = progressItems.filter(p => p.type === 'gallery')
  const reelItems = progressItems.filter(p => p.type === 'reel')

  return (
    <div className="rounded-lg border border-blue-500/30 bg-blue-500/5 p-4 space-y-4">
      <div className="flex items-center gap-2">
        <span className="relative flex h-3 w-3">
          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75"></span>
          <span className="relative inline-flex rounded-full h-3 w-3 bg-blue-500"></span>
        </span>
        <h3 className="font-medium">กำลังประมวลผล (Real-time)</h3>
        <Badge variant="secondary" className="text-xs">{progressItems.length} งาน</Badge>
      </div>

      <div className="grid gap-3">
        {transcodeItems.map(item => (
          <ProgressItem key={`${item.videoId}-${item.type}`} item={item} icon={<Monitor className="h-4 w-4" />} />
        ))}
        {subtitleItems.map(item => (
          <ProgressItem key={`${item.videoId}-${item.type}`} item={item} icon={<Languages className="h-4 w-4" />} />
        ))}
        {galleryItems.map(item => (
          <ProgressItem key={`${item.videoId}-${item.type}`} item={item} icon={<Images className="h-4 w-4" />} />
        ))}
        {reelItems.map(item => (
          <ProgressItem key={`${item.videoId}-${item.type}`} item={item} icon={<Film className="h-4 w-4" />} />
        ))}
      </div>
    </div>
  )
}

function ProgressItem({ item, icon }: { item: VideoProgress; icon: React.ReactNode }) {
  const getStatusColor = () => {
    switch (item.status) {
      case 'completed': return 'text-green-500'
      case 'failed': return 'text-red-500'
      default: return 'text-blue-500'
    }
  }

  const getTypeLabel = () => {
    switch (item.type) {
      case 'transcode': return 'แปลงวิดีโอ'
      case 'subtitle': return item.language ? `ซับ ${item.language}` : 'ซับไตเติ้ล'
      case 'gallery': return 'Gallery'
      case 'reel': return 'Reel'
      default: return item.type
    }
  }

  return (
    <div className="bg-background rounded-lg border p-3 space-y-2">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="text-muted-foreground">{icon}</span>
          <span className="font-mono text-sm">{item.videoCode}</span>
          <Badge variant="outline" className="text-xs">{getTypeLabel()}</Badge>
        </div>
        <div className="flex items-center gap-2">
          <span className={`text-sm font-medium ${getStatusColor()}`}>
            {item.progress}%
          </span>
          {item.status === 'completed' && <CheckCircle className="h-4 w-4 text-green-500" />}
          {item.status === 'failed' && <XCircle className="h-4 w-4 text-red-500" />}
        </div>
      </div>

      <Progress value={item.progress} className="h-2" />

      <div className="flex items-center justify-between text-xs text-muted-foreground">
        <span>{item.currentStep || item.message}</span>
        {item.errorMessage && (
          <span className="text-red-500 truncate max-w-[200px]">{item.errorMessage}</span>
        )}
      </div>
    </div>
  )
}

// ==================== Tabs ====================

function TranscodeTab() {
  const [page, _setPage] = useState(1)
  const { data, isLoading } = useFailedJobs('transcode', page)
  const retryJob = useRetryJob()
  const purgeStream = usePurgeTranscodeStream()
  const jobs = data?.jobs ?? []

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="font-medium">วิดีโอที่แปลงล้มเหลว</h3>
        <Button
          variant="outline"
          size="sm"
          onClick={() => purgeStream.mutate()}
          disabled={purgeStream.isPending}
        >
          <Trash2 className={`h-4 w-4 mr-2 ${purgeStream.isPending ? 'animate-pulse' : ''}`} />
          Purge NATS Stream
        </Button>
      </div>

      {isLoading ? (
        <TableSkeleton />
      ) : jobs.length === 0 ? (
        <EmptyState message="ไม่มีวิดีโอที่ล้มเหลว" />
      ) : (
        <JobTable jobs={jobs} onRetry={(id) => retryJob.mutate(id)} isRetrying={retryJob.isPending} />
      )}
    </div>
  )
}

function SubtitleTab() {
  const [view, setView] = useState<'processing' | 'failed'>('failed')
  const [page, _setPage] = useState(1)

  const processingQuery = useProcessingJobs('subtitle_transcribe', page)
  const failedQuery = useFailedJobs('subtitle_transcribe', page)

  const retryAll = useRetrySubtitleAll()
  const clearAll = useClearSubtitleAll()
  const queueMissing = useQueueMissingSubtitles()
  const retryJob = useRetryJob()

  const { data, isLoading } = view === 'processing' ? processingQuery : failedQuery
  const jobs = data?.jobs ?? []

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <h3 className="font-medium">ซับไตเติ้ล</h3>
          <div className="flex gap-1 ml-4">
            <Button variant={view === 'failed' ? 'secondary' : 'ghost'} size="sm" onClick={() => setView('failed')}>
              ล้มเหลว
            </Button>
            <Button variant={view === 'processing' ? 'secondary' : 'ghost'} size="sm" onClick={() => setView('processing')}>
              กำลังทำ
            </Button>
          </div>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={() => retryAll.mutate()} disabled={retryAll.isPending}>
            <RotateCcw className={`h-4 w-4 mr-1 ${retryAll.isPending ? 'animate-spin' : ''}`} />
            Retry All
          </Button>
          {view === 'processing' && (
            <>
              <Button variant="outline" size="sm" onClick={() => clearAll.mutate()} disabled={clearAll.isPending}>
                <Trash2 className="h-4 w-4 mr-1" />
                Clear
              </Button>
              <Button variant="outline" size="sm" onClick={() => queueMissing.mutate()} disabled={queueMissing.isPending}>
                <Plus className="h-4 w-4 mr-1" />
                Queue Missing
              </Button>
            </>
          )}
        </div>
      </div>

      {isLoading ? (
        <TableSkeleton />
      ) : jobs.length === 0 ? (
        <EmptyState message={`ไม่มีซับไตเติ้ลที่${view === 'processing' ? 'กำลังทำ' : 'ล้มเหลว'}`} />
      ) : (
        <JobTable jobs={jobs} onRetry={(id) => retryJob.mutate(id)} isRetrying={retryJob.isPending} showJobType />
      )}
    </div>
  )
}

function WarmCacheTab() {
  const [view, setView] = useState<'pending' | 'failed'>('pending')
  const [page, _setPage] = useState(1)

  const pendingQuery = useWarmCachePending(page)
  const failedQuery = useWarmCacheFailed(page)
  const warmOne = useWarmCacheOne()
  const warmAll = useWarmCacheAll()

  const { data, isLoading } = view === 'pending' ? pendingQuery : failedQuery
  const items = data?.data ?? []

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <h3 className="font-medium">แคช CDN</h3>
          <div className="flex gap-1 ml-4">
            <Button variant={view === 'pending' ? 'secondary' : 'ghost'} size="sm" onClick={() => setView('pending')}>
              รอแคช
            </Button>
            <Button variant={view === 'failed' ? 'secondary' : 'ghost'} size="sm" onClick={() => setView('failed')}>
              ล้มเหลว
            </Button>
          </div>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => warmAll.mutate()}
          disabled={warmAll.isPending || items.length === 0}
        >
          <Flame className={`h-4 w-4 mr-1 ${warmAll.isPending ? 'animate-pulse' : ''}`} />
          Warm All
        </Button>
      </div>

      {isLoading ? (
        <TableSkeleton />
      ) : items.length === 0 ? (
        <EmptyState message={view === 'pending' ? 'ไม่มีวิดีโอที่รอแคช' : 'ไม่มีวิดีโอที่แคชล้มเหลว'} />
      ) : (
        <div className="border rounded-lg overflow-hidden">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>รหัส</TableHead>
                <TableHead>ชื่อ</TableHead>
                <TableHead>คุณภาพ</TableHead>
                <TableHead>สถานะ</TableHead>
                <TableHead className="w-20"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((item: WarmCacheQueueItem) => (
                <TableRow key={item.id}>
                  <TableCell className="font-mono text-xs">{item.code}</TableCell>
                  <TableCell className="max-w-[200px] truncate text-sm">{item.title}</TableCell>
                  <TableCell>
                    <div className="flex gap-1">
                      {item.qualities?.map((q) => (
                        <Badge key={q} variant="outline" className="text-xs px-1">
                          {q}
                        </Badge>
                      ))}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant={item.cacheStatus === 'failed' ? 'destructive' : 'secondary'} className="text-xs">
                      {item.cacheStatus}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <div className="flex gap-1">
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button variant="ghost" size="sm" onClick={() => warmOne.mutate(item.id)} disabled={warmOne.isPending}>
                            <Flame className="h-4 w-4" />
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>Warm</TooltipContent>
                      </Tooltip>
                      <LinkButton code={item.code} />
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  )
}

function GalleryTab() {
  const [view, setView] = useState<'processing' | 'failed'>('failed')
  const [page, _setPage] = useState(1)

  const processingQuery = useProcessingJobs('gallery', page)
  const failedQuery = useFailedJobs('gallery', page)
  const retryJob = useRetryJob()

  const { data, isLoading } = view === 'processing' ? processingQuery : failedQuery
  const jobs = data?.jobs ?? []

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <h3 className="font-medium">Gallery</h3>
          <div className="flex gap-1 ml-4">
            <Button variant={view === 'failed' ? 'secondary' : 'ghost'} size="sm" onClick={() => setView('failed')}>
              ล้มเหลว
            </Button>
            <Button variant={view === 'processing' ? 'secondary' : 'ghost'} size="sm" onClick={() => setView('processing')}>
              กำลังสร้าง
            </Button>
          </div>
        </div>
      </div>

      {isLoading ? (
        <TableSkeleton />
      ) : jobs.length === 0 ? (
        <EmptyState message={`ไม่มี gallery ที่${view === 'processing' ? 'กำลังสร้าง' : 'ล้มเหลว'}`} />
      ) : (
        <JobTable jobs={jobs} onRetry={(id) => retryJob.mutate(id)} isRetrying={retryJob.isPending} />
      )}
    </div>
  )
}

function ReelTab() {
  const [view, setView] = useState<'processing' | 'failed'>('failed')
  const [page, _setPage] = useState(1)

  const processingQuery = useProcessingJobs('reel', page)
  const failedQuery = useFailedJobs('reel', page)
  const retryJob = useRetryJob()

  const { data, isLoading } = view === 'processing' ? processingQuery : failedQuery
  const jobs = data?.jobs ?? []

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <h3 className="font-medium">Reel</h3>
          <div className="flex gap-1 ml-4">
            <Button variant={view === 'failed' ? 'secondary' : 'ghost'} size="sm" onClick={() => setView('failed')}>
              ล้มเหลว
            </Button>
            <Button variant={view === 'processing' ? 'secondary' : 'ghost'} size="sm" onClick={() => setView('processing')}>
              กำลัง export
            </Button>
          </div>
        </div>
      </div>

      {isLoading ? (
        <TableSkeleton />
      ) : jobs.length === 0 ? (
        <EmptyState message={`ไม่มี reel ที่${view === 'processing' ? 'กำลัง export' : 'ล้มเหลว'}`} />
      ) : (
        <JobTable jobs={jobs} onRetry={(id) => retryJob.mutate(id)} isRetrying={retryJob.isPending} linkTo="/reels" />
      )}
    </div>
  )
}

// ==================== Shared Components ====================

function TableSkeleton() {
  return (
    <div className="space-y-2">
      <Skeleton className="h-10 w-full" />
      <Skeleton className="h-10 w-full" />
      <Skeleton className="h-10 w-full" />
    </div>
  )
}

function EmptyState({ message }: { message: string }) {
  return (
    <div className="text-center py-12 text-muted-foreground border rounded-lg bg-muted/20">
      <CheckCircle className="h-10 w-10 mx-auto mb-3 text-green-500" />
      <p>{message}</p>
    </div>
  )
}

function JobTable({
  jobs,
  onRetry,
  isRetrying,
  showJobType,
  linkTo,
}: {
  jobs: WorkerJob[]
  onRetry: (id: string) => void
  isRetrying: boolean
  showJobType?: boolean
  linkTo?: string
}) {
  return (
    <div className="border rounded-lg overflow-hidden">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>รหัส</TableHead>
            {showJobType && <TableHead>ประเภท</TableHead>}
            <TableHead>สถานะ</TableHead>
            <TableHead>ข้อผิดพลาด</TableHead>
            <TableHead className="text-center w-20">Retry</TableHead>
            <TableHead className="w-20"></TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {jobs.map((job) => (
            <TableRow key={job.id}>
              <TableCell className="font-mono text-xs">{job.entity_code}</TableCell>
              {showJobType && (
                <TableCell>
                  <Badge variant="outline" className="text-xs">
                    {job.job_type === 'subtitle_detect' && 'detect'}
                    {job.job_type === 'subtitle_transcribe' && 'transcribe'}
                    {job.job_type === 'subtitle_translate' && 'translate'}
                    {!job.job_type.startsWith('subtitle') && job.job_type}
                  </Badge>
                </TableCell>
              )}
              <TableCell>
                <Badge variant={job.status === 'failed' ? 'destructive' : 'secondary'} className="text-xs">
                  {job.status}
                </Badge>
              </TableCell>
              <TableCell className="max-w-[300px]">
                <span className="text-xs text-muted-foreground line-clamp-1">{job.last_error || '-'}</span>
              </TableCell>
              <TableCell className="text-center">
                <span className="text-xs text-muted-foreground">{job.retry_count}x</span>
              </TableCell>
              <TableCell>
                <div className="flex gap-1">
                  {job.status === 'failed' && (
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button variant="ghost" size="sm" onClick={() => onRetry(job.id)} disabled={isRetrying}>
                          <RotateCcw className="h-4 w-4" />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>Retry</TooltipContent>
                    </Tooltip>
                  )}
                  <LinkButton code={job.entity_code} to={linkTo} />
                </div>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

function LinkButton({ code, to }: { code: string; to?: string }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button variant="ghost" size="sm" asChild>
          <Link to={to || `/videos?code=${code}`}>
            <ExternalLink className="h-4 w-4" />
          </Link>
        </Button>
      </TooltipTrigger>
      <TooltipContent>ดูรายละเอียด</TooltipContent>
    </Tooltip>
  )
}
