import { useState } from 'react'
import {
  ListChecks,
  RefreshCw,
  Monitor,
  Languages,
  Database,
  Clock,
  CheckCircle,
  XCircle,
  RotateCcw,
  Flame,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  useQueueStats,
  useTranscodeFailed,
  useRetryTranscodeAll,
  useRetryTranscodeOne,
  useSubtitleStuck,
  useSubtitleFailed,
  useRetrySubtitleAll,
  useWarmCachePending,
  useWarmCacheFailed,
  useWarmCacheOne,
  useWarmCacheAll,
} from '../hooks'
import type { TranscodeQueueItem, SubtitleQueueItem, WarmCacheQueueItem } from '../types'

export function QueueManagementPage() {
  const { data: stats, isLoading: statsLoading, refetch: refetchStats } = useQueueStats()
  const [activeTab, setActiveTab] = useState('transcode')

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold flex items-center gap-2">
            <ListChecks className="h-6 w-6" />
            Queue Management
          </h1>
          <p className="text-muted-foreground">
            จัดการ queue ทั้งหมดในระบบ (Transcode, Subtitle, Warm Cache)
          </p>
        </div>

        <Button
          variant="outline"
          size="icon"
          onClick={() => refetchStats()}
          disabled={statsLoading}
        >
          <RefreshCw className={`h-4 w-4 ${statsLoading ? 'animate-spin' : ''}`} />
        </Button>
      </div>

      {/* Stats Overview */}
      {statsLoading ? (
        <div className="grid grid-cols-3 gap-4">
          <Skeleton className="h-24" />
          <Skeleton className="h-24" />
          <Skeleton className="h-24" />
        </div>
      ) : stats ? (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {/* Transcode Stats */}
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium flex items-center gap-2">
                <Monitor className="h-4 w-4" />
                Transcode
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex flex-wrap gap-2 text-xs">
                {stats.transcode.pending > 0 && (
                  <Badge variant="outline" className="gap-1">
                    <Clock className="h-3 w-3" />
                    Pending: {stats.transcode.pending}
                  </Badge>
                )}
                {stats.transcode.queued > 0 && (
                  <Badge variant="secondary" className="gap-1">
                    Queued: {stats.transcode.queued}
                  </Badge>
                )}
                {stats.transcode.processing > 0 && (
                  <Badge className="gap-1 status-processing">
                    Processing: {stats.transcode.processing}
                  </Badge>
                )}
                {stats.transcode.failed > 0 && (
                  <Badge variant="destructive" className="gap-1">
                    <XCircle className="h-3 w-3" />
                    Failed: {stats.transcode.failed}
                  </Badge>
                )}
                {stats.transcode.deadLetter > 0 && (
                  <Badge variant="destructive" className="gap-1">
                    DLQ: {stats.transcode.deadLetter}
                  </Badge>
                )}
              </div>
            </CardContent>
          </Card>

          {/* Subtitle Stats */}
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium flex items-center gap-2">
                <Languages className="h-4 w-4" />
                Subtitle
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex flex-wrap gap-2 text-xs">
                {stats.subtitle.queued > 0 && (
                  <Badge variant="secondary" className="gap-1">
                    <Clock className="h-3 w-3" />
                    Stuck: {stats.subtitle.queued}
                  </Badge>
                )}
                {stats.subtitle.processing > 0 && (
                  <Badge className="gap-1 status-processing">
                    Processing: {stats.subtitle.processing}
                  </Badge>
                )}
                {stats.subtitle.failed > 0 && (
                  <Badge variant="destructive" className="gap-1">
                    <XCircle className="h-3 w-3" />
                    Failed: {stats.subtitle.failed}
                  </Badge>
                )}
                {stats.subtitle.queued === 0 &&
                  stats.subtitle.processing === 0 &&
                  stats.subtitle.failed === 0 && (
                    <Badge variant="outline" className="gap-1 text-muted-foreground">
                      <CheckCircle className="h-3 w-3" />
                      No issues
                    </Badge>
                  )}
              </div>
            </CardContent>
          </Card>

          {/* Warm Cache Stats */}
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium flex items-center gap-2">
                <Database className="h-4 w-4" />
                Warm Cache
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex flex-wrap gap-2 text-xs">
                {stats.warmCache.notCached > 0 && (
                  <Badge variant="outline" className="gap-1">
                    <Clock className="h-3 w-3" />
                    Pending: {stats.warmCache.notCached}
                  </Badge>
                )}
                {stats.warmCache.warming > 0 && (
                  <Badge className="gap-1 status-processing">
                    <Flame className="h-3 w-3" />
                    Warming: {stats.warmCache.warming}
                  </Badge>
                )}
                {stats.warmCache.cached > 0 && (
                  <Badge variant="outline" className="gap-1 text-status-success border-status-success">
                    <CheckCircle className="h-3 w-3" />
                    Cached: {stats.warmCache.cached}
                  </Badge>
                )}
                {stats.warmCache.failed > 0 && (
                  <Badge variant="destructive" className="gap-1">
                    <XCircle className="h-3 w-3" />
                    Failed: {stats.warmCache.failed}
                  </Badge>
                )}
              </div>
            </CardContent>
          </Card>
        </div>
      ) : null}

      {/* Tabs */}
      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList className="grid w-full grid-cols-3">
          <TabsTrigger value="transcode" className="gap-2">
            <Monitor className="h-4 w-4" />
            Transcode
            {stats?.transcode?.failed ? (
              <Badge variant="destructive" className="ml-1 h-5 px-1.5">
                {stats.transcode.failed}
              </Badge>
            ) : null}
          </TabsTrigger>
          <TabsTrigger value="subtitle" className="gap-2">
            <Languages className="h-4 w-4" />
            Subtitle
            {(stats?.subtitle?.queued || 0) + (stats?.subtitle?.failed || 0) > 0 ? (
              <Badge variant="destructive" className="ml-1 h-5 px-1.5">
                {(stats?.subtitle?.queued || 0) + (stats?.subtitle?.failed || 0)}
              </Badge>
            ) : null}
          </TabsTrigger>
          <TabsTrigger value="warmcache" className="gap-2">
            <Database className="h-4 w-4" />
            Warm Cache
            {stats?.warmCache?.notCached ? (
              <Badge variant="secondary" className="ml-1 h-5 px-1.5">
                {stats.warmCache.notCached}
              </Badge>
            ) : null}
          </TabsTrigger>
        </TabsList>

        <TabsContent value="transcode" className="mt-4">
          <TranscodeTab />
        </TabsContent>

        <TabsContent value="subtitle" className="mt-4">
          <SubtitleTab />
        </TabsContent>

        <TabsContent value="warmcache" className="mt-4">
          <WarmCacheTab />
        </TabsContent>
      </Tabs>
    </div>
  )
}

// ==================== Transcode Tab ====================

function TranscodeTab() {
  const [page, setPage] = useState(1)
  const { data, isLoading } = useTranscodeFailed(page)
  const retryAll = useRetryTranscodeAll()
  const retryOne = useRetryTranscodeOne()

  const items = data?.data ?? []
  const meta = data?.meta

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <div>
          <CardTitle>Transcode Failed</CardTitle>
          <CardDescription>
            รายการ video ที่ transcode ล้มเหลว
          </CardDescription>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => retryAll.mutate()}
          disabled={retryAll.isPending || items.length === 0}
        >
          <RotateCcw className={`h-4 w-4 mr-2 ${retryAll.isPending ? 'animate-spin' : ''}`} />
          Retry All
        </Button>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : items.length === 0 ? (
          <div className="text-center py-8 text-muted-foreground">
            <CheckCircle className="h-12 w-12 mx-auto mb-2 text-status-success" />
            <p>ไม่มี video ที่ failed</p>
          </div>
        ) : (
          <>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Code</TableHead>
                  <TableHead>Title</TableHead>
                  <TableHead>Error</TableHead>
                  <TableHead className="text-center">Retry</TableHead>
                  <TableHead className="w-24">Action</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((item: TranscodeQueueItem) => (
                  <TableRow key={item.id}>
                    <TableCell className="font-mono text-xs">{item.code}</TableCell>
                    <TableCell className="max-w-[200px] truncate">{item.title}</TableCell>
                    <TableCell className="max-w-[300px]">
                      <span className="text-destructive text-xs line-clamp-2">
                        {item.error || '-'}
                      </span>
                    </TableCell>
                    <TableCell className="text-center">
                      <Badge variant="outline">{item.retryCount}</Badge>
                    </TableCell>
                    <TableCell>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => retryOne.mutate(item.id)}
                        disabled={retryOne.isPending}
                      >
                        <RotateCcw className="h-4 w-4" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>

            {meta && meta.totalPages > 1 && (
              <div className="flex items-center justify-between mt-4">
                <p className="text-sm text-muted-foreground">
                  Page {meta.page} of {meta.totalPages} ({meta.total} items)
                </p>
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage((p) => Math.max(1, p - 1))}
                    disabled={!meta.hasPrev}
                  >
                    Previous
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage((p) => p + 1)}
                    disabled={!meta.hasNext}
                  >
                    Next
                  </Button>
                </div>
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}

// ==================== Subtitle Tab ====================

function SubtitleTab() {
  const [subtab, setSubtab] = useState<'stuck' | 'failed'>('stuck')
  const [page, setPage] = useState(1)

  const stuckQuery = useSubtitleStuck(page)
  const failedQuery = useSubtitleFailed(page)
  const retryAll = useRetrySubtitleAll()

  const { data, isLoading } = subtab === 'stuck' ? stuckQuery : failedQuery
  const items = data?.data ?? []
  const meta = data?.meta

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <div>
          <CardTitle>Subtitle Queue</CardTitle>
          <CardDescription>
            รายการ subtitle ที่ค้างหรือล้มเหลว
          </CardDescription>
        </div>
        <div className="flex gap-2">
          <Button
            variant={subtab === 'stuck' ? 'default' : 'outline'}
            size="sm"
            onClick={() => {
              setSubtab('stuck')
              setPage(1)
            }}
          >
            Stuck
          </Button>
          <Button
            variant={subtab === 'failed' ? 'default' : 'outline'}
            size="sm"
            onClick={() => {
              setSubtab('failed')
              setPage(1)
            }}
          >
            Failed
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => retryAll.mutate()}
            disabled={retryAll.isPending || items.length === 0}
          >
            <RotateCcw className={`h-4 w-4 mr-2 ${retryAll.isPending ? 'animate-spin' : ''}`} />
            Retry All
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : items.length === 0 ? (
          <div className="text-center py-8 text-muted-foreground">
            <CheckCircle className="h-12 w-12 mx-auto mb-2 text-status-success" />
            <p>ไม่มี subtitle ที่ {subtab === 'stuck' ? 'ค้าง' : 'failed'}</p>
          </div>
        ) : (
          <>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Code</TableHead>
                  <TableHead>Title</TableHead>
                  <TableHead>Language</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Error</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((item: SubtitleQueueItem) => (
                  <TableRow key={item.id}>
                    <TableCell className="font-mono text-xs">{item.videoCode}</TableCell>
                    <TableCell className="max-w-[200px] truncate">{item.videoTitle}</TableCell>
                    <TableCell>
                      <Badge variant="outline">{item.language}</Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant="secondary">{item.type}</Badge>
                    </TableCell>
                    <TableCell className="max-w-[300px]">
                      <span className="text-destructive text-xs line-clamp-2">
                        {item.error || '-'}
                      </span>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>

            {meta && meta.totalPages > 1 && (
              <div className="flex items-center justify-between mt-4">
                <p className="text-sm text-muted-foreground">
                  Page {meta.page} of {meta.totalPages} ({meta.total} items)
                </p>
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage((p) => Math.max(1, p - 1))}
                    disabled={!meta.hasPrev}
                  >
                    Previous
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage((p) => p + 1)}
                    disabled={!meta.hasNext}
                  >
                    Next
                  </Button>
                </div>
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}

// ==================== Warm Cache Tab ====================

function WarmCacheTab() {
  const [subtab, setSubtab] = useState<'pending' | 'failed'>('pending')
  const [page, setPage] = useState(1)

  const pendingQuery = useWarmCachePending(page)
  const failedQuery = useWarmCacheFailed(page)
  const warmOne = useWarmCacheOne()
  const warmAll = useWarmCacheAll()

  const { data, isLoading } = subtab === 'pending' ? pendingQuery : failedQuery
  const items = data?.data ?? []
  const meta = data?.meta

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <div>
          <CardTitle>Warm Cache Queue</CardTitle>
          <CardDescription>
            รายการ video ที่รอ warm CDN cache
          </CardDescription>
        </div>
        <div className="flex gap-2">
          <Button
            variant={subtab === 'pending' ? 'default' : 'outline'}
            size="sm"
            onClick={() => {
              setSubtab('pending')
              setPage(1)
            }}
          >
            Pending
          </Button>
          <Button
            variant={subtab === 'failed' ? 'default' : 'outline'}
            size="sm"
            onClick={() => {
              setSubtab('failed')
              setPage(1)
            }}
          >
            Failed
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => warmAll.mutate()}
            disabled={warmAll.isPending || items.length === 0}
          >
            <Flame className={`h-4 w-4 mr-2 ${warmAll.isPending ? 'animate-pulse' : ''}`} />
            Warm All
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : items.length === 0 ? (
          <div className="text-center py-8 text-muted-foreground">
            <CheckCircle className="h-12 w-12 mx-auto mb-2 text-status-success" />
            <p>
              {subtab === 'pending'
                ? 'ไม่มี video ที่รอ warm cache'
                : 'ไม่มี video ที่ warm cache failed'}
            </p>
          </div>
        ) : (
          <>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Code</TableHead>
                  <TableHead>Title</TableHead>
                  <TableHead>Qualities</TableHead>
                  <TableHead>Status</TableHead>
                  {subtab === 'failed' && <TableHead>Error</TableHead>}
                  <TableHead className="w-24">Action</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((item: WarmCacheQueueItem) => (
                  <TableRow key={item.id}>
                    <TableCell className="font-mono text-xs">{item.code}</TableCell>
                    <TableCell className="max-w-[200px] truncate">{item.title}</TableCell>
                    <TableCell>
                      <div className="flex flex-wrap gap-1">
                        {item.qualities?.map((q) => (
                          <Badge key={q} variant="outline" className="text-xs">
                            {q}
                          </Badge>
                        ))}
                      </div>
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={item.cacheStatus === 'failed' ? 'destructive' : 'secondary'}
                      >
                        {item.cacheStatus}
                      </Badge>
                      {item.cachePercentage > 0 && (
                        <span className="ml-2 text-xs text-muted-foreground">
                          {item.cachePercentage.toFixed(0)}%
                        </span>
                      )}
                    </TableCell>
                    {subtab === 'failed' && (
                      <TableCell className="max-w-[200px]">
                        <span className="text-destructive text-xs line-clamp-2">
                          {item.error || '-'}
                        </span>
                      </TableCell>
                    )}
                    <TableCell>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => warmOne.mutate(item.id)}
                        disabled={warmOne.isPending}
                      >
                        <Flame className="h-4 w-4" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>

            {meta && meta.totalPages > 1 && (
              <div className="flex items-center justify-between mt-4">
                <p className="text-sm text-muted-foreground">
                  Page {meta.page} of {meta.totalPages} ({meta.total} items)
                </p>
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage((p) => Math.max(1, p - 1))}
                    disabled={!meta.hasPrev}
                  >
                    Previous
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage((p) => p + 1)}
                    disabled={!meta.hasNext}
                  >
                    Next
                  </Button>
                </div>
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}
