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
  Clock,
  Loader2,
  Search,
  FileText,
  Globe,
} from 'lucide-react'
import { Progress } from '@/components/ui/progress'
import { useWebSocketConnection, useVideoProgress, type VideoProgress } from '@/lib/websocket-provider'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useCategories } from '@/features/category/hooks'
import {
  useLegacyStats,
  useFailedJobs,
  useProcessingJobs,
  useQueuedJobs,
  useRetryJob,
  useRetryAllTranscode,
  useRetrySubtitleAll,
  useClearSubtitleAll,
  useQueueMissingSubtitles,
  useSubtitleStats,
  useBatchDetect,
  useBatchTranscribe,
  useBatchTranslate,
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
  const { isConnected } = useWebSocketConnection()
  const activeProgress = useVideoProgress()

  const deleteOrphaned = useDeleteOrphanedJobs()
  const deleteCompleted = useDeleteCompletedJobs()
  const deleteFailed = useDeleteFailedJobs()

  return (
    <TooltipProvider>
      <div className="space-y-8">
        {/* Header - match AdminDashboard style */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-semibold">จัดการคิว</h1>
            <p className="text-muted-foreground">จัดการและดูสถานะคิวงานทั้งหมด</p>
          </div>
          <div className="flex items-center gap-3">
            {isConnected && (
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <span className="relative flex h-2 w-2">
                  <span className="animate-ping absolute inline-flex h-full w-full rounded-full indicator-online-ping opacity-75"></span>
                  <span className="relative inline-flex rounded-full h-2 w-2 indicator-online"></span>
                </span>
                Live
              </div>
            )}
            <Button
              variant="ghost"
              size="sm"
              onClick={() => refetchStats()}
              disabled={statsLoading}
            >
              <RefreshCw className={`h-4 w-4 ${statsLoading ? 'animate-spin' : ''}`} />
            </Button>
          </div>
        </div>

        {/* Stats - inline style like AdminDashboard */}
        <div className="space-y-3">
          <p className="text-muted-foreground text-sm">สถานะคิว</p>
          {statsLoading ? (
            <div className="flex items-center gap-6">
              {[1, 2, 3, 4, 5].map(i => <Skeleton key={i} className="h-8 w-20" />)}
            </div>
          ) : (
            <div className="flex items-center gap-6 flex-wrap">
              <StatInline
                icon={<Monitor className="h-4 w-4" />}
                label="แปลงวิดีโอ"
                processing={stats?.transcode?.processing || 0}
                pending={(stats?.transcode?.pending || 0) + (stats?.transcode?.queued || 0)}
                failed={(stats?.transcode?.failed || 0) + (stats?.transcode?.deadLetter || 0)}
              />
              <Separator orientation="vertical" className="h-10" />
              <StatInline
                icon={<Languages className="h-4 w-4" />}
                label="ซับไตเติ้ล"
                processing={stats?.subtitle?.processing || 0}
                pending={stats?.subtitle?.queued || 0}
                failed={stats?.subtitle?.failed || 0}
              />
              <Separator orientation="vertical" className="h-10" />
              <StatInline
                icon={<Database className="h-4 w-4" />}
                label="แคช CDN"
                processing={stats?.warmCache?.warming || 0}
                pending={stats?.warmCache?.notCached || 0}
                failed={stats?.warmCache?.failed || 0}
              />
              <Separator orientation="vertical" className="h-10" />
              <StatInline
                icon={<Images className="h-4 w-4" />}
                label="Gallery"
                processing={stats?.gallery?.processing || 0}
                failed={stats?.gallery?.failed || 0}
              />
              <Separator orientation="vertical" className="h-10" />
              <StatInline
                icon={<Film className="h-4 w-4" />}
                label="Reel"
                processing={stats?.reel?.exporting || 0}
                failed={stats?.reel?.failed || 0}
              />
            </div>
          )}
        </div>

        {/* Active Progress */}
        {activeProgress.size > 0 && (
          <ActiveProgressSection activeProgress={activeProgress} />
        )}

        {/* Actions */}
        <div className="space-y-3">
          <p className="text-muted-foreground text-sm">จัดการ</p>
          <div className="flex items-center gap-2 flex-wrap">
            <Button
              variant="outline"
              size="sm"
              onClick={() => deleteOrphaned.mutate()}
              disabled={deleteOrphaned.isPending}
            >
              <Trash2 className={`h-4 w-4 mr-2 ${deleteOrphaned.isPending ? 'animate-pulse' : ''}`} />
              ลบ Orphaned
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => deleteCompleted.mutate(7)}
              disabled={deleteCompleted.isPending}
            >
              <Trash2 className={`h-4 w-4 mr-2 ${deleteCompleted.isPending ? 'animate-pulse' : ''}`} />
              ลบ Completed
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => deleteFailed.mutate()}
              disabled={deleteFailed.isPending}
            >
              <Trash2 className={`h-4 w-4 mr-2 ${deleteFailed.isPending ? 'animate-pulse' : ''}`} />
              ลบ Failed
            </Button>
          </div>
        </div>

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
              <CountBadge count={stats?.warmCache?.notCached || 0} />
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

          <TabsContent value="transcode"><TranscodeTab /></TabsContent>
          <TabsContent value="subtitle"><SubtitleTab /></TabsContent>
          <TabsContent value="warmcache"><WarmCacheTab /></TabsContent>
          <TabsContent value="gallery"><GalleryTab /></TabsContent>
          <TabsContent value="reel"><ReelTab /></TabsContent>
        </Tabs>
      </div>
    </TooltipProvider>
  )
}

// ==================== Inline Stat (like AdminDashboard) ====================

function StatInline({
  icon,
  label,
  processing = 0,
  pending = 0,
  failed = 0,
}: {
  icon: React.ReactNode
  label: string
  processing?: number
  pending?: number
  failed?: number
}) {
  return (
    <div>
      <div className="flex items-center gap-1.5 text-muted-foreground text-sm mb-1">
        {icon}
        <span>{label}</span>
      </div>
      <div className="flex items-center gap-3 text-sm">
        {processing > 0 && (
          <span className="flex items-center gap-1.5 font-semibold tabular-nums">
            <Loader2 className="h-3.5 w-3.5 animate-spin text-muted-foreground" />
            {processing}
          </span>
        )}
        {pending > 0 && (
          <span className="flex items-center gap-1 tabular-nums text-muted-foreground">
            <Clock className="h-3.5 w-3.5" />
            {pending}
          </span>
        )}
        {failed > 0 && (
          <span className="flex items-center gap-1 tabular-nums text-muted-foreground">
            <XCircle className="h-3.5 w-3.5" />
            {failed}
          </span>
        )}
        {processing === 0 && pending === 0 && failed === 0 && (
          <span className="text-muted-foreground tabular-nums">-</span>
        )}
      </div>
    </div>
  )
}

function CountBadge({ count }: { count: number }) {
  if (count === 0) return null
  return (
    <Badge variant="secondary" className="h-5 px-1.5 text-xs">
      {count}
    </Badge>
  )
}

// ==================== Active Progress ====================

function ActiveProgressSection({ activeProgress }: { activeProgress: Map<string, VideoProgress> }) {
  const items = Array.from(activeProgress.values())

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <p className="text-muted-foreground text-sm">กำลังประมวลผล</p>
        <Badge variant="secondary" className="text-xs">{items.length} งาน</Badge>
      </div>
      <div className="space-y-2">
        {items.map(item => (
          <ProgressItem key={`${item.videoId}-${item.type}`} item={item} />
        ))}
      </div>
    </div>
  )
}

function ProgressItem({ item }: { item: VideoProgress }) {
  const typeLabel = (() => {
    switch (item.type) {
      case 'transcode': return 'แปลงวิดีโอ'
      case 'subtitle': return item.language ? `ซับ ${item.language}` : 'ซับไตเติ้ล'
      case 'gallery': return 'Gallery'
      case 'reel': return 'Reel'
      default: return item.type
    }
  })()

  return (
    <div className="flex items-center gap-3 px-3 py-2.5 rounded-lg border border-dashed hover:bg-accent/50 transition-colors">
      <span className="font-mono text-sm">{item.videoCode}</span>
      <Badge variant="outline" className="text-xs">{typeLabel}</Badge>
      <div className="flex-1">
        <Progress value={item.progress} className="h-1.5" />
      </div>
      <span className="text-sm font-semibold tabular-nums w-12 text-right">
        {item.progress}%
      </span>
      {item.status === 'completed' && <CheckCircle className="h-4 w-4 text-muted-foreground" />}
      {item.status === 'failed' && <XCircle className="h-4 w-4 text-muted-foreground" />}
      <span className="text-xs text-muted-foreground max-w-[180px] truncate">
        {item.currentStep || item.message}
      </span>
    </div>
  )
}

// ==================== Tabs ====================

function TranscodeTab() {
  const [page, _setPage] = useState(1)
  const { data, isLoading } = useFailedJobs('transcode', page)
  const retryJob = useRetryJob()
  const retryAll = useRetryAllTranscode()
  const purgeStream = usePurgeTranscodeStream()
  const jobs = data?.jobs ?? []

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="font-medium">วิดีโอที่แปลงล้มเหลว</h3>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => retryAll.mutate()}
            disabled={retryAll.isPending}
          >
            <RotateCcw className={`h-4 w-4 mr-2 ${retryAll.isPending ? 'animate-spin' : ''}`} />
            Queue ใหม่ทั้งหมด
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => purgeStream.mutate()}
            disabled={purgeStream.isPending}
          >
            <Trash2 className={`h-4 w-4 mr-2 ${purgeStream.isPending ? 'animate-pulse' : ''}`} />
            Purge Stream
          </Button>
        </div>
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
  const [category, setCategory] = useState<string>('all')
  const [view, setView] = useState<'failed' | 'processing' | 'queued'>('failed')
  const [page, _setPage] = useState(1)

  const { data: categories = [] } = useCategories()
  const categoryId = category === 'all' ? undefined : category
  const { data: stats, isLoading: statsLoading } = useSubtitleStats(categoryId)

  const failedQuery = useFailedJobs('subtitle_transcribe', page)
  const processingQuery = useProcessingJobs('subtitle_transcribe', page)
  const queuedQuery = useQueuedJobs('subtitle_transcribe', page)

  const retryAll = useRetrySubtitleAll()
  const clearAll = useClearSubtitleAll()
  const queueMissing = useQueueMissingSubtitles()
  const retryJob = useRetryJob()

  const batchDetect = useBatchDetect()
  const batchTranscribe = useBatchTranscribe()
  const batchTranslate = useBatchTranslate()

  const currentQuery = view === 'failed' ? failedQuery : view === 'processing' ? processingQuery : queuedQuery
  const jobs = currentQuery.data?.jobs ?? []


  return (
    <div className="space-y-6">
      {/* Category Filter */}
      <div className="flex items-center gap-3">
        <span className="text-sm text-muted-foreground">หมวดหมู่:</span>
        <Select value={category} onValueChange={setCategory}>
          <SelectTrigger className="w-[200px]">
            <SelectValue placeholder="ทั้งหมด" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">ทั้งหมด</SelectItem>
            {categories.map((cat) => (
              <SelectItem key={cat.id} value={cat.id}>{cat.name}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {/* Subtitle Stats Panel */}
      <div className="border rounded-lg p-4 space-y-3">
        <h3 className="font-medium text-sm text-muted-foreground">สถานะ Subtitle</h3>
        {statsLoading ? (
          <div className="space-y-2">
            {[1, 2, 3].map(i => <Skeleton key={i} className="h-9 w-full" />)}
          </div>
        ) : stats ? (
          <div className="space-y-2">
            <div className="flex items-center justify-between px-3 py-2 rounded-lg border border-dashed">
              <div className="flex items-center gap-2 text-sm">
                <Search className="h-4 w-4 text-muted-foreground" />
                <span>ยังไม่ detect</span>
                <Badge variant="secondary" className="text-xs tabular-nums">{stats.notDetected}</Badge>
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={() => batchDetect.mutate({ category: categoryId })}
                disabled={batchDetect.isPending || stats.notDetected === 0}
              >
                {batchDetect.isPending ? <Loader2 className="h-4 w-4 mr-1 animate-spin" /> : <Search className="h-4 w-4 mr-1" />}
                Detect ทั้งหมด
              </Button>
            </div>

            <div className="flex items-center justify-between px-3 py-2 rounded-lg border border-dashed">
              <div className="flex items-center gap-2 text-sm">
                <FileText className="h-4 w-4 text-muted-foreground" />
                <span>ยังไม่ transcribe</span>
                <Badge variant="secondary" className="text-xs tabular-nums">{stats.notTranscribed}</Badge>
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={() => batchTranscribe.mutate({ category: categoryId })}
                disabled={batchTranscribe.isPending || stats.notTranscribed === 0}
              >
                {batchTranscribe.isPending ? <Loader2 className="h-4 w-4 mr-1 animate-spin" /> : <FileText className="h-4 w-4 mr-1" />}
                Transcribe ทั้งหมด
              </Button>
            </div>

            <div className="flex items-center justify-between px-3 py-2 rounded-lg border border-dashed">
              <div className="flex items-center gap-2 text-sm">
                <Globe className="h-4 w-4 text-muted-foreground" />
                <span>ยังไม่ translate</span>
                <Badge variant="secondary" className="text-xs tabular-nums">{stats.notTranslated}</Badge>
              </div>
              <Button
                variant="outline"
                size="sm"
                onClick={() => batchTranslate.mutate({ category: categoryId })}
                disabled={batchTranslate.isPending || stats.notTranslated === 0}
              >
                {batchTranslate.isPending ? <Loader2 className="h-4 w-4 mr-1 animate-spin" /> : <Globe className="h-4 w-4 mr-1" />}
                Translate ทั้งหมด
              </Button>
            </div>

            <div className="flex items-center gap-2 px-3 py-2 text-sm text-muted-foreground">
              <CheckCircle className="h-4 w-4" />
              <span>ครบแล้ว: {stats.translated}</span>
              <span className="ml-auto">รวม {stats.totalVideos} วิดีโอ</span>
            </div>
          </div>
        ) : null}
      </div>

      {/* Sub-tabs: Failed / Processing / Queued */}
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="flex gap-1">
              <Button variant={view === 'failed' ? 'secondary' : 'ghost'} size="sm" onClick={() => setView('failed')}>ล้มเหลว</Button>
              <Button variant={view === 'processing' ? 'secondary' : 'ghost'} size="sm" onClick={() => setView('processing')}>กำลังทำ</Button>
              <Button variant={view === 'queued' ? 'secondary' : 'ghost'} size="sm" onClick={() => setView('queued')}>รอคิว</Button>
            </div>
          </div>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={() => retryAll.mutate()} disabled={retryAll.isPending}>
              <RotateCcw className={`h-4 w-4 mr-1 ${retryAll.isPending ? 'animate-spin' : ''}`} />
              Retry All
            </Button>
            <Button variant="outline" size="sm" onClick={() => clearAll.mutate()} disabled={clearAll.isPending}>
              <Trash2 className="h-4 w-4 mr-1" /> Clear Stuck
            </Button>
            <Button variant="outline" size="sm" onClick={() => queueMissing.mutate()} disabled={queueMissing.isPending}>
              <Plus className="h-4 w-4 mr-1" /> Queue Missing
            </Button>
          </div>
        </div>

        {currentQuery.isLoading ? (
          <TableSkeleton />
        ) : jobs.length === 0 ? (
          <EmptyState message={
            view === 'failed' ? 'ไม่มีซับไตเติ้ลที่ล้มเหลว' :
            view === 'processing' ? 'ไม่มีซับไตเติ้ลที่กำลังทำ' :
            'ไม่มีซับไตเติ้ลที่รอคิว'
          } />
        ) : (
          <JobTable jobs={jobs} onRetry={(id) => retryJob.mutate(id)} isRetrying={retryJob.isPending} showJobType />
        )}
      </div>
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
            <Button variant={view === 'pending' ? 'secondary' : 'ghost'} size="sm" onClick={() => setView('pending')}>รอแคช</Button>
            <Button variant={view === 'failed' ? 'secondary' : 'ghost'} size="sm" onClick={() => setView('failed')}>ล้มเหลว</Button>
          </div>
        </div>
        <Button variant="outline" size="sm" onClick={() => warmAll.mutate()} disabled={warmAll.isPending || items.length === 0}>
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
                        <Badge key={q} variant="outline" className="text-xs px-1">{q}</Badge>
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
            <Button variant={view === 'failed' ? 'secondary' : 'ghost'} size="sm" onClick={() => setView('failed')}>ล้มเหลว</Button>
            <Button variant={view === 'processing' ? 'secondary' : 'ghost'} size="sm" onClick={() => setView('processing')}>กำลังสร้าง</Button>
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
            <Button variant={view === 'failed' ? 'secondary' : 'ghost'} size="sm" onClick={() => setView('failed')}>ล้มเหลว</Button>
            <Button variant={view === 'processing' ? 'secondary' : 'ghost'} size="sm" onClick={() => setView('processing')}>กำลัง export</Button>
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
    <div className="text-center py-12 text-muted-foreground border border-dashed rounded-lg">
      <CheckCircle className="h-8 w-8 mx-auto mb-2 text-muted-foreground" />
      <p className="text-sm">{message}</p>
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
                <span className="text-xs text-muted-foreground tabular-nums">{job.retry_count}x</span>
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
