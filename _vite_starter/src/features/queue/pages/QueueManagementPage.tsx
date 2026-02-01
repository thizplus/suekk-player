import { useState } from 'react'
import { Link } from 'react-router-dom'
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
  ExternalLink,
  HelpCircle,
  AlertTriangle,
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
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  Alert,
  AlertDescription,
} from '@/components/ui/alert'
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
    <TooltipProvider>
      <div className="space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-semibold flex items-center gap-2">
              <ListChecks className="h-6 w-6" />
              จัดการคิว
            </h1>
            <p className="text-muted-foreground">
              ดูสถานะและแก้ไขปัญหางานที่ค้างหรือล้มเหลว
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
                  แปลงวิดีโอ
                  <Tooltip>
                    <TooltipTrigger>
                      <HelpCircle className="h-3 w-3 text-muted-foreground" />
                    </TooltipTrigger>
                    <TooltipContent>
                      <p>แปลงไฟล์วิดีโอเป็น HLS สำหรับสตรีม</p>
                    </TooltipContent>
                  </Tooltip>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="flex flex-wrap gap-2 text-xs">
                  {stats.transcode.pending > 0 && (
                    <Badge variant="outline" className="gap-1">
                      <Clock className="h-3 w-3" />
                      รอคิว: {stats.transcode.pending}
                    </Badge>
                  )}
                  {stats.transcode.queued > 0 && (
                    <Badge variant="secondary" className="gap-1">
                      อยู่ในคิว: {stats.transcode.queued}
                    </Badge>
                  )}
                  {stats.transcode.processing > 0 && (
                    <Badge className="gap-1 status-processing">
                      กำลังทำ: {stats.transcode.processing}
                    </Badge>
                  )}
                  {stats.transcode.failed > 0 && (
                    <Badge variant="destructive" className="gap-1">
                      <XCircle className="h-3 w-3" />
                      ล้มเหลว: {stats.transcode.failed}
                    </Badge>
                  )}
                  {stats.transcode.deadLetter > 0 && (
                    <Badge variant="destructive" className="gap-1">
                      ล้มเหลวถาวร: {stats.transcode.deadLetter}
                    </Badge>
                  )}
                  {stats.transcode.pending === 0 &&
                    stats.transcode.queued === 0 &&
                    stats.transcode.processing === 0 &&
                    stats.transcode.failed === 0 && (
                      <Badge variant="outline" className="gap-1 text-muted-foreground">
                        <CheckCircle className="h-3 w-3" />
                        ไม่มีปัญหา
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
                  ซับไตเติ้ล
                  <Tooltip>
                    <TooltipTrigger>
                      <HelpCircle className="h-3 w-3 text-muted-foreground" />
                    </TooltipTrigger>
                    <TooltipContent className="max-w-xs">
                      <p>สร้างและแปลซับไตเติ้ลอัตโนมัติด้วย AI</p>
                    </TooltipContent>
                  </Tooltip>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="flex flex-wrap gap-2 text-xs">
                  {stats.subtitle.queued > 0 && (
                    <Badge variant="secondary" className="gap-1">
                      <AlertTriangle className="h-3 w-3" />
                      ค้าง: {stats.subtitle.queued}
                    </Badge>
                  )}
                  {stats.subtitle.processing > 0 && (
                    <Badge className="gap-1 status-processing">
                      กำลังทำ: {stats.subtitle.processing}
                    </Badge>
                  )}
                  {stats.subtitle.failed > 0 && (
                    <Badge variant="destructive" className="gap-1">
                      <XCircle className="h-3 w-3" />
                      ล้มเหลว: {stats.subtitle.failed}
                    </Badge>
                  )}
                  {stats.subtitle.queued === 0 &&
                    stats.subtitle.processing === 0 &&
                    stats.subtitle.failed === 0 && (
                      <Badge variant="outline" className="gap-1 text-muted-foreground">
                        <CheckCircle className="h-3 w-3" />
                        ไม่มีปัญหา
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
                  แคช CDN
                  <Tooltip>
                    <TooltipTrigger>
                      <HelpCircle className="h-3 w-3 text-muted-foreground" />
                    </TooltipTrigger>
                    <TooltipContent className="max-w-xs">
                      <p>โหลดวิดีโอไปเก็บที่ CDN ล่วงหน้า เพื่อให้ผู้ชมโหลดเร็วขึ้น</p>
                    </TooltipContent>
                  </Tooltip>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="flex flex-wrap gap-2 text-xs">
                  {stats.warmCache.notCached > 0 && (
                    <Badge variant="outline" className="gap-1">
                      <Clock className="h-3 w-3" />
                      รอแคช: {stats.warmCache.notCached}
                    </Badge>
                  )}
                  {stats.warmCache.warming > 0 && (
                    <Badge className="gap-1 status-processing">
                      <Flame className="h-3 w-3" />
                      กำลังแคช: {stats.warmCache.warming}
                    </Badge>
                  )}
                  {stats.warmCache.cached > 0 && (
                    <Badge variant="outline" className="gap-1 text-status-success border-status-success">
                      <CheckCircle className="h-3 w-3" />
                      แคชแล้ว: {stats.warmCache.cached}
                    </Badge>
                  )}
                  {stats.warmCache.failed > 0 && (
                    <Badge variant="destructive" className="gap-1">
                      <XCircle className="h-3 w-3" />
                      ล้มเหลว: {stats.warmCache.failed}
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
              แปลงวิดีโอ
              {stats?.transcode?.failed ? (
                <Badge variant="destructive" className="ml-1 h-5 px-1.5">
                  {stats.transcode.failed}
                </Badge>
              ) : null}
            </TabsTrigger>
            <TabsTrigger value="subtitle" className="gap-2">
              <Languages className="h-4 w-4" />
              ซับไตเติ้ล
              {(stats?.subtitle?.queued || 0) + (stats?.subtitle?.failed || 0) > 0 ? (
                <Badge variant="destructive" className="ml-1 h-5 px-1.5">
                  {(stats?.subtitle?.queued || 0) + (stats?.subtitle?.failed || 0)}
                </Badge>
              ) : null}
            </TabsTrigger>
            <TabsTrigger value="warmcache" className="gap-2">
              <Database className="h-4 w-4" />
              แคช CDN
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
    </TooltipProvider>
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
      <CardHeader>
        <div className="flex flex-row items-start justify-between">
          <div>
            <CardTitle>วิดีโอที่แปลงไม่สำเร็จ</CardTitle>
            <CardDescription>
              รายการวิดีโอที่แปลงล้มเหลว สามารถกด "ลองใหม่" เพื่อส่งงานเข้าคิวอีกครั้ง
            </CardDescription>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={() => retryAll.mutate()}
            disabled={retryAll.isPending || items.length === 0}
          >
            <RotateCcw className={`h-4 w-4 mr-2 ${retryAll.isPending ? 'animate-spin' : ''}`} />
            ลองใหม่ทั้งหมด
          </Button>
        </div>

        <Alert className="mt-4">
          <HelpCircle className="h-4 w-4" />
          <AlertDescription>
            <strong>วิธีแก้ไข:</strong> กด "ลองใหม่" เพื่อส่งงานเข้าคิวอีกครั้ง หากยังไม่สำเร็จ
            ให้ตรวจสอบไฟล์ต้นฉบับว่าเสียหายหรือไม่ หรือติดต่อผู้ดูแลระบบ
          </AlertDescription>
        </Alert>
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
            <p>ไม่มีวิดีโอที่ล้มเหลว</p>
          </div>
        ) : (
          <>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>รหัส</TableHead>
                  <TableHead>ชื่อ</TableHead>
                  <TableHead>ข้อผิดพลาด</TableHead>
                  <TableHead className="text-center">ลองแล้ว</TableHead>
                  <TableHead className="w-32">จัดการ</TableHead>
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
                      <Badge variant="outline">{item.retryCount} ครั้ง</Badge>
                    </TableCell>
                    <TableCell>
                      <div className="flex gap-1">
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => retryOne.mutate(item.id)}
                              disabled={retryOne.isPending}
                            >
                              <RotateCcw className="h-4 w-4" />
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent>ลองใหม่</TooltipContent>
                        </Tooltip>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button variant="ghost" size="sm" asChild>
                              <Link to={`/videos?search=${item.code}`}>
                                <ExternalLink className="h-4 w-4" />
                              </Link>
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent>ดูรายละเอียด</TooltipContent>
                        </Tooltip>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>

            {meta && meta.totalPages > 1 && (
              <div className="flex items-center justify-between mt-4">
                <p className="text-sm text-muted-foreground">
                  หน้า {meta.page} จาก {meta.totalPages} (ทั้งหมด {meta.total} รายการ)
                </p>
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage((p) => Math.max(1, p - 1))}
                    disabled={!meta.hasPrev}
                  >
                    ก่อนหน้า
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage((p) => p + 1)}
                    disabled={!meta.hasNext}
                  >
                    ถัดไป
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
      <CardHeader>
        <div className="flex flex-row items-start justify-between">
          <div>
            <CardTitle>ซับไตเติ้ลที่มีปัญหา</CardTitle>
            <CardDescription>
              รายการซับไตเติ้ลที่ค้างหรือล้มเหลว
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
              ค้าง
            </Button>
            <Button
              variant={subtab === 'failed' ? 'default' : 'outline'}
              size="sm"
              onClick={() => {
                setSubtab('failed')
                setPage(1)
              }}
            >
              ล้มเหลว
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => retryAll.mutate()}
              disabled={retryAll.isPending || items.length === 0}
            >
              <RotateCcw className={`h-4 w-4 mr-2 ${retryAll.isPending ? 'animate-spin' : ''}`} />
              ลองใหม่ทั้งหมด
            </Button>
          </div>
        </div>

        <Alert className="mt-4">
          <HelpCircle className="h-4 w-4" />
          <AlertDescription>
            {subtab === 'stuck' ? (
              <>
                <strong>"ค้าง" คืออะไร?</strong> งานที่รออยู่ในคิวนานเกินไป อาจเกิดจาก Worker หยุดทำงานกลางคัน
                กด "ลองใหม่ทั้งหมด" เพื่อส่งงานเข้าคิวอีกครั้ง
              </>
            ) : (
              <>
                <strong>วิธีแก้ไข:</strong> กด "ลองใหม่ทั้งหมด" หากยังไม่สำเร็จ
                ให้ตรวจสอบว่าไฟล์เสียงของวิดีโอมีปัญหาหรือไม่ หรือติดต่อผู้ดูแลระบบ
              </>
            )}
          </AlertDescription>
        </Alert>
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
            <p>ไม่มีซับไตเติ้ลที่{subtab === 'stuck' ? 'ค้าง' : 'ล้มเหลว'}</p>
          </div>
        ) : (
          <>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>รหัส</TableHead>
                  <TableHead>ชื่อวิดีโอ</TableHead>
                  <TableHead>ภาษา</TableHead>
                  <TableHead>ประเภท</TableHead>
                  <TableHead>ข้อผิดพลาด</TableHead>
                  <TableHead className="w-20">จัดการ</TableHead>
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
                      <Badge variant="secondary">
                        {item.type === 'transcribed' ? 'ถอดเสียง' : 'แปล'}
                      </Badge>
                    </TableCell>
                    <TableCell className="max-w-[200px]">
                      <span className="text-destructive text-xs line-clamp-2">
                        {item.error || '-'}
                      </span>
                    </TableCell>
                    <TableCell>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <Button variant="ghost" size="sm" asChild>
                            <Link to={`/videos?search=${item.videoCode}`}>
                              <ExternalLink className="h-4 w-4" />
                            </Link>
                          </Button>
                        </TooltipTrigger>
                        <TooltipContent>ดูรายละเอียดวิดีโอ</TooltipContent>
                      </Tooltip>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>

            {meta && meta.totalPages > 1 && (
              <div className="flex items-center justify-between mt-4">
                <p className="text-sm text-muted-foreground">
                  หน้า {meta.page} จาก {meta.totalPages} (ทั้งหมด {meta.total} รายการ)
                </p>
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage((p) => Math.max(1, p - 1))}
                    disabled={!meta.hasPrev}
                  >
                    ก่อนหน้า
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage((p) => p + 1)}
                    disabled={!meta.hasNext}
                  >
                    ถัดไป
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
      <CardHeader>
        <div className="flex flex-row items-start justify-between">
          <div>
            <CardTitle>แคช CDN</CardTitle>
            <CardDescription>
              โหลดวิดีโอไปเก็บที่ CDN ล่วงหน้า เพื่อให้ผู้ชมโหลดได้เร็วขึ้น
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
              รอแคช
            </Button>
            <Button
              variant={subtab === 'failed' ? 'default' : 'outline'}
              size="sm"
              onClick={() => {
                setSubtab('failed')
                setPage(1)
              }}
            >
              ล้มเหลว
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => warmAll.mutate()}
              disabled={warmAll.isPending || items.length === 0}
            >
              <Flame className={`h-4 w-4 mr-2 ${warmAll.isPending ? 'animate-pulse' : ''}`} />
              แคชทั้งหมด
            </Button>
          </div>
        </div>

        <Alert className="mt-4">
          <HelpCircle className="h-4 w-4" />
          <AlertDescription>
            {subtab === 'pending' ? (
              <>
                <strong>รอแคช:</strong> วิดีโอเหล่านี้ยังไม่ได้โหลดไปเก็บที่ CDN
                กด "แคชทั้งหมด" หรือกดไอคอนไฟเพื่อแคชทีละตัว
                (ไม่จำเป็นต้องทำทันที - ระบบจะแคชอัตโนมัติเมื่อมีคนดู)
              </>
            ) : (
              <>
                <strong>แคชล้มเหลว:</strong> อาจเกิดจากปัญหาเครือข่าย กด "แคชทั้งหมด" เพื่อลองใหม่
              </>
            )}
          </AlertDescription>
        </Alert>
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
                ? 'ไม่มีวิดีโอที่รอแคช'
                : 'ไม่มีวิดีโอที่แคชล้มเหลว'}
            </p>
          </div>
        ) : (
          <>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>รหัส</TableHead>
                  <TableHead>ชื่อ</TableHead>
                  <TableHead>คุณภาพ</TableHead>
                  <TableHead>สถานะ</TableHead>
                  {subtab === 'failed' && <TableHead>ข้อผิดพลาด</TableHead>}
                  <TableHead className="w-32">จัดการ</TableHead>
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
                        {item.cacheStatus === 'pending' && 'รอแคช'}
                        {item.cacheStatus === 'warming' && 'กำลังแคช'}
                        {item.cacheStatus === 'cached' && 'แคชแล้ว'}
                        {item.cacheStatus === 'failed' && 'ล้มเหลว'}
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
                      <div className="flex gap-1">
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              variant="ghost"
                              size="sm"
                              onClick={() => warmOne.mutate(item.id)}
                              disabled={warmOne.isPending}
                            >
                              <Flame className="h-4 w-4" />
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent>แคชวิดีโอนี้</TooltipContent>
                        </Tooltip>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button variant="ghost" size="sm" asChild>
                              <Link to={`/videos?search=${item.code}`}>
                                <ExternalLink className="h-4 w-4" />
                              </Link>
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent>ดูรายละเอียด</TooltipContent>
                        </Tooltip>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>

            {meta && meta.totalPages > 1 && (
              <div className="flex items-center justify-between mt-4">
                <p className="text-sm text-muted-foreground">
                  หน้า {meta.page} จาก {meta.totalPages} (ทั้งหมด {meta.total} รายการ)
                </p>
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage((p) => Math.max(1, p - 1))}
                    disabled={!meta.hasPrev}
                  >
                    ก่อนหน้า
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setPage((p) => p + 1)}
                    disabled={!meta.hasNext}
                  >
                    ถัดไป
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
