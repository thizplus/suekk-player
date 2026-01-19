import { useNavigate } from 'react-router-dom'
import {
  Video,
  FolderOpen,
  Clock,
  CheckCircle2,
  XCircle,
  Loader2,
  Eye,
  Activity,
  Timer,
  HardDrive,
} from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Progress } from '@/components/ui/progress'
import { useVideos, useTranscodingStats } from '@/features/video'
import { useStorageUsage } from '../hooks'
import { useWebSocketConnection, useVideoProgress } from '@/lib/websocket-provider'
import { VIDEO_STATUS_STYLES, VIDEO_STATUS_LABELS } from '@/constants/enums'

const formatDuration = (seconds: number) => {
  if (!seconds || seconds <= 0) return '-'
  const mins = Math.floor(seconds / 60)
  const secs = seconds % 60
  return `${mins}:${secs.toString().padStart(2, '0')}`
}

const formatDate = (dateString: string) => {
  return new Date(dateString).toLocaleString('th-TH', {
    day: 'numeric',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function AdminDashboard() {
  const navigate = useNavigate()
  const { data: videos, isLoading: videosLoading } = useVideos({ limit: 5 })
  const { data: stats, isLoading: statsLoading } = useTranscodingStats()
  const { data: storage, isLoading: storageLoading } = useStorageUsage()
  const activeProgress = useVideoProgress()

  // WebSocket connection status (Context with singleton inside)
  const { isConnected: wsConnected } = useWebSocketConnection()

  const totalVideos = videos?.meta.total ?? 0
  const totalViews = videos?.data.reduce((sum, v) => sum + v.views, 0) ?? 0
  const recentVideos = videos?.data ?? []

  const isLoading = videosLoading || statsLoading

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">แดชบอร์ด</h1>
          <p className="text-muted-foreground">ภาพรวมระบบ Video Streaming</p>
        </div>
        {wsConnected && (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <span className="relative flex h-2 w-2">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full indicator-online-ping opacity-75"></span>
              <span className="relative inline-flex rounded-full h-2 w-2 indicator-online"></span>
            </span>
            Live
          </div>
        )}
      </div>

      {/* Main Stats */}
      <div className="flex items-center gap-6 flex-wrap">
        <div>
          <p className="text-muted-foreground text-sm mb-1">วิดีโอทั้งหมด</p>
          {isLoading ? (
            <Skeleton className="h-8 w-16" />
          ) : (
            <p className="text-3xl font-semibold tabular-nums">{totalVideos}</p>
          )}
        </div>
        <Separator orientation="vertical" className="h-12" />
        <div>
          <p className="text-muted-foreground text-sm mb-1">พร้อมใช้งาน</p>
          {isLoading ? (
            <Skeleton className="h-8 w-16" />
          ) : (
            <p className="text-3xl font-semibold tabular-nums">{stats?.completed ?? 0}</p>
          )}
        </div>
        <Separator orientation="vertical" className="h-12" />
        <div>
          <p className="text-muted-foreground text-sm mb-1">ยอดวิวรวม</p>
          {isLoading ? (
            <Skeleton className="h-8 w-16" />
          ) : (
            <p className="text-3xl font-semibold tabular-nums">{totalViews.toLocaleString()}</p>
          )}
        </div>
      </div>

      {/* Quick Access */}
      <div className="space-y-3">
        <p className="text-muted-foreground text-sm">เข้าถึงด่วน</p>
        <div className="flex items-center gap-4 h-5">
          <button
            onClick={() => navigate('/videos')}
            className="flex items-center gap-2 text-sm hover:text-foreground text-muted-foreground transition-colors"
          >
            <Video className="h-4 w-4" />
            วิดีโอ
          </button>
          <Separator orientation="vertical" />
          <button
            onClick={() => navigate('/categories')}
            className="flex items-center gap-2 text-sm hover:text-foreground text-muted-foreground transition-colors"
          >
            <FolderOpen className="h-4 w-4" />
            หมวดหมู่
          </button>
          <Separator orientation="vertical" />
          <button
            onClick={() => navigate('/transcoding')}
            className="flex items-center gap-2 text-sm hover:text-foreground text-muted-foreground transition-colors"
          >
            <Activity className="h-4 w-4" />
            ประมวลผล
            {(stats?.pending ?? 0) > 0 && (
              <Badge variant="secondary" className="ml-1">{stats?.pending}</Badge>
            )}
          </button>
        </div>
      </div>

      {/* ประมวลผล Status */}
      <div className="space-y-3">
        <p className="text-muted-foreground text-sm">สถานะประมวลผล</p>
        <div className="flex items-center gap-4 h-5">
          <div className="flex items-center gap-2 text-sm">
            <Clock className="h-4 w-4 text-muted-foreground" />
            <span className="text-muted-foreground">รอ</span>
            {statsLoading ? (
              <Skeleton className="h-4 w-6" />
            ) : (
              <span className="font-semibold tabular-nums">{stats?.pending ?? 0}</span>
            )}
          </div>
          <Separator orientation="vertical" />
          <div className="flex items-center gap-2 text-sm">
            <Timer className="h-4 w-4 text-status-queued" />
            <span className="text-muted-foreground">คิว</span>
            {statsLoading ? (
              <Skeleton className="h-4 w-6" />
            ) : (
              <span className="font-semibold tabular-nums">{stats?.queued ?? 0}</span>
            )}
          </div>
          <Separator orientation="vertical" />
          <div className="flex items-center gap-2 text-sm">
            <Loader2 className={`h-4 w-4 text-muted-foreground ${stats?.processing ? 'animate-spin' : ''}`} />
            <span className="text-muted-foreground">กำลังทำ</span>
            {statsLoading ? (
              <Skeleton className="h-4 w-6" />
            ) : (
              <span className="font-semibold tabular-nums">{stats?.processing ?? 0}</span>
            )}
          </div>
          <Separator orientation="vertical" />
          <div className="flex items-center gap-2 text-sm">
            <CheckCircle2 className="h-4 w-4 text-muted-foreground" />
            <span className="text-muted-foreground">เสร็จ</span>
            {statsLoading ? (
              <Skeleton className="h-4 w-6" />
            ) : (
              <span className="font-semibold tabular-nums">{stats?.completed ?? 0}</span>
            )}
          </div>
          <Separator orientation="vertical" />
          <div className="flex items-center gap-2 text-sm">
            <XCircle className="h-4 w-4 text-muted-foreground" />
            <span className="text-muted-foreground">ล้มเหลว</span>
            {statsLoading ? (
              <Skeleton className="h-4 w-6" />
            ) : (
              <span className="font-semibold tabular-nums">{stats?.failed ?? 0}</span>
            )}
          </div>
        </div>
      </div>

      {/* Storage Usage */}
      <div className="space-y-3">
        <p className="text-muted-foreground text-sm">พื้นที่จัดเก็บ</p>
        {storageLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-2 w-full" />
            <Skeleton className="h-4 w-32" />
          </div>
        ) : storage?.unlimited ? (
          <div className="flex items-center gap-2 text-sm">
            <HardDrive className="h-4 w-4 text-muted-foreground" />
            <span className="text-muted-foreground">ใช้ไป</span>
            <span className="font-semibold">{storage.usedHuman}</span>
            <span className="text-muted-foreground">(ไม่จำกัด)</span>
          </div>
        ) : (
          <div className="space-y-2">
            <div className="flex items-center gap-3">
              <HardDrive className="h-4 w-4 text-muted-foreground shrink-0" />
              <Progress
                value={storage?.percent ?? 0}
                className="flex-1 h-2"
              />
              <span className="text-sm font-semibold tabular-nums whitespace-nowrap">
                {(storage?.percent ?? 0).toFixed(1)}%
              </span>
            </div>
            <p className="text-xs text-muted-foreground pl-7">
              {storage?.usedHuman ?? '0 B'} / {storage?.quotaHuman ?? '0 B'}
            </p>
          </div>
        )}
      </div>

      {/* Recent Videos */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <p className="text-muted-foreground text-sm">วิดีโอล่าสุด</p>
          <button
            onClick={() => navigate('/videos')}
            className="text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            ดูทั้งหมด →
          </button>
        </div>

        {videosLoading ? (
          <div className="space-y-3 p-4 border border-dashed rounded-lg">
            {[1, 2, 3].map((i) => (
              <div key={i} className="flex items-center gap-3">
                <Skeleton className="h-5 w-5" />
                <Skeleton className="h-4 flex-1" />
                <Skeleton className="h-4 w-16" />
              </div>
            ))}
          </div>
        ) : recentVideos.length === 0 ? (
          <p className="text-muted-foreground py-4 border border-dashed rounded-lg text-center">ยังไม่มีวิดีโอ</p>
        ) : (
          <div className="space-y-2">
            {recentVideos.map((video) => {
              const progress = activeProgress.get(video.id)
              return (
                <div
                  key={video.id}
                  className="flex items-center gap-3 px-3 py-2.5 rounded-lg border border-dashed hover:bg-accent/50 transition-colors cursor-pointer leading-none"
                  onClick={() => navigate('/videos')}
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
                      <span>{formatDate(video.createdAt)}</span>
                    </p>
                  </div>
                  {progress && progress.status !== 'completed' && progress.status !== 'failed' ? (
                    <Badge variant="outline" className="gap-1.5 tabular-nums">
                      <Loader2 className="h-3 w-3 animate-spin" />
                      {progress.progress}%
                    </Badge>
                  ) : (
                    <Badge className={VIDEO_STATUS_STYLES[video.status]}>
                      {VIDEO_STATUS_LABELS[video.status]}
                    </Badge>
                  )}
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}
