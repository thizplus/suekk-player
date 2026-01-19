import { useState, Fragment } from 'react'
import {
  Server,
  Loader2,
  HardDrive,
  Pause,
  ChevronDown,
  ChevronRight,
} from 'lucide-react'
import { formatDistanceToNow } from 'date-fns'
import { th } from 'date-fns/locale'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import type { OnlineWorker } from '../types'

interface WorkerTableProps {
  workers: OnlineWorker[]
}

export function WorkerTable({ workers }: WorkerTableProps) {
  const [expandedWorkers, setExpandedWorkers] = useState<Set<string>>(new Set())

  const toggleExpanded = (id: string) => {
    setExpandedWorkers((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  const formatUptime = (seconds: number) => {
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)
    return `${hours}h ${minutes}m`
  }

  const formatStartedAt = (startedAt: string) => {
    return formatDistanceToNow(new Date(startedAt), { addSuffix: true, locale: th })
  }

  const statusConfig = {
    idle: { label: 'ว่าง', style: 'status-success' },
    processing: { label: 'ทำงาน', style: 'status-processing' },
    stopping: { label: 'กำลังหยุด', style: 'status-pending' },
    paused: { label: 'หยุดชั่วคราว', style: 'status-danger' },
  } as const

  const workerTypeConfig = {
    transcode: { label: 'Transcode', style: 'status-transcode' },
    subtitle: { label: 'Subtitle', style: 'status-subtitle' },
  } as const

  const diskLevelConfig = {
    normal: { style: 'text-status-success', progressStyle: '' },
    warning: { style: 'text-status-pending', progressStyle: 'progress-pending' },
    caution: { style: 'text-status-pending', progressStyle: 'progress-pending' },
    critical: { style: 'text-status-danger', progressStyle: 'progress-danger' },
  } as const

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead className="w-[40px]"></TableHead>
          <TableHead>Worker</TableHead>
          <TableHead>ประเภท</TableHead>
          <TableHead>สถานะ</TableHead>
          <TableHead>Config</TableHead>
          <TableHead>Stats</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {workers.length === 0 ? (
          <TableRow>
            <TableCell colSpan={6} className="text-center text-muted-foreground py-12">
              <Server className="h-8 w-8 mx-auto mb-2 opacity-50" />
              <p>ไม่มี Worker ออนไลน์</p>
              <p className="text-xs mt-1">Worker จะแสดงที่นี่เมื่อเชื่อมต่อกับ NATS</p>
            </TableCell>
          </TableRow>
        ) : (
          workers.map((worker) => {
            const isExpanded = expandedWorkers.has(worker.worker_id)
            const status = statusConfig[worker.status]
            const workerType = workerTypeConfig[worker.worker_type] ?? workerTypeConfig.transcode
            const diskConfig = worker.disk?.level
              ? diskLevelConfig[worker.disk.level]
              : diskLevelConfig.normal

            return (
              <Fragment key={worker.worker_id}>
                <TableRow
                  className="cursor-pointer hover:bg-muted/50"
                  onClick={() => toggleExpanded(worker.worker_id)}
                >
                  <TableCell>
                    <button className="p-1 hover:bg-muted rounded">
                      {isExpanded ? (
                        <ChevronDown className="h-4 w-4" />
                      ) : (
                        <ChevronRight className="h-4 w-4" />
                      )}
                    </button>
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-3">
                      <Server className="h-4 w-4 text-muted-foreground shrink-0" />
                      <div>
                        <p className="font-medium">{worker.worker_id}</p>
                        <p className="text-xs text-muted-foreground">
                          {worker.hostname} • {worker.internal_ip}
                        </p>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline" className={workerType.style}>
                      {workerType.label}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <Badge className={status.style}>{status.label}</Badge>
                      {worker.current_jobs.length > 0 && (
                        <Badge variant="outline" className="gap-1 tabular-nums">
                          <Loader2 className="h-3 w-3 animate-spin" />
                          {Math.round(worker.current_jobs[0].progress)}%
                        </Badge>
                      )}
                      {worker.status === 'paused' && (
                        <Badge variant="outline" className="gap-1 text-status-danger border-status-danger">
                          <Pause className="h-3 w-3" />
                          Disk Full
                        </Badge>
                      )}
                    </div>
                  </TableCell>
                  <TableCell>
                    <span className="text-sm text-muted-foreground">
                      {worker.config.gpu_enabled ? 'GPU' : 'CPU'} ×{worker.config.concurrency}
                    </span>
                  </TableCell>
                  <TableCell>
                    <div className="text-sm text-muted-foreground">
                      <span className="text-status-success">✓{worker.stats.total_processed}</span>
                      {' '}
                      <span className="text-status-danger">✗{worker.stats.total_failed}</span>
                    </div>
                  </TableCell>
                </TableRow>

                {/* Expanded Details */}
                {isExpanded && (
                  <TableRow className="bg-muted/30 hover:bg-muted/30">
                    <TableCell colSpan={6} className="p-4">
                      <div className="space-y-3">
                        {/* Online Stats */}
                        <div className="flex items-center gap-6 text-sm">
                          <span className="text-muted-foreground">
                            เริ่มทำงาน: <span className="text-foreground">{formatStartedAt(worker.started_at)}</span>
                          </span>
                          <span className="text-muted-foreground">
                            Uptime: <span className="text-foreground">{formatUptime(worker.stats.uptime_seconds)}</span>
                          </span>
                          {/* Preset only for transcode workers */}
                          {worker.worker_type !== 'subtitle' && worker.config.preset && (
                            <span className="text-muted-foreground">
                              Preset: <span className="font-mono text-foreground">{worker.config.preset}</span>
                            </span>
                          )}
                        </div>

                        {/* Disk Usage */}
                        {worker.disk && worker.disk.usage_percent > 0 && (
                          <div className="flex items-center gap-3">
                            <HardDrive className={`h-4 w-4 shrink-0 ${diskConfig.style}`} />
                            <div className="flex-1 max-w-xs">
                              <Progress
                                value={worker.disk.usage_percent}
                                className={`h-2 ${diskConfig.progressStyle}`}
                              />
                            </div>
                            <span className={`text-sm tabular-nums ${diskConfig.style}`}>
                              {worker.disk.usage_percent.toFixed(0)}%
                            </span>
                            <span className="text-xs text-muted-foreground">
                              ({worker.disk.free_gb.toFixed(1)} GB free)
                            </span>
                          </div>
                        )}

                        {/* Current Job */}
                        {worker.current_jobs.length > 0 && (
                          <div className="border rounded-lg p-3 space-y-2">
                            <div className="flex items-center justify-between">
                              <span className="text-sm font-medium">
                                กำลังประมวลผล: {worker.current_jobs[0].title}
                              </span>
                              <span className="text-sm font-semibold tabular-nums">
                                {Math.round(worker.current_jobs[0].progress)}%
                              </span>
                            </div>
                            <Progress value={worker.current_jobs[0].progress} className="h-1.5" />
                            <p className="text-xs text-muted-foreground">
                              {worker.current_jobs[0].stage} • ETA: {worker.current_jobs[0].eta}
                            </p>
                          </div>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                )}
              </Fragment>
            )
          })
        )}
      </TableBody>
    </Table>
  )
}
