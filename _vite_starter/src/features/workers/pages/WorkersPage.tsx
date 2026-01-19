import { Server, RefreshCw, Wifi, Activity, Loader2, Monitor, Languages } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import { useOnlineWorkers } from '../hooks'
import { WorkerTable } from '../components/WorkerTable'

export function WorkersPage() {
  const { data, isLoading, refetch } = useOnlineWorkers()

  const workers = data?.workers ?? []
  const summary = data?.summary
  const totalOnline = data?.total_online ?? 0

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold flex items-center gap-2">
            <Server className="h-6 w-6" />
            Workers
          </h1>
          <p className="text-muted-foreground">
            Workers ที่ออนไลน์อยู่ (Auto-Discovery)
          </p>
        </div>

        <Button
          variant="outline"
          size="icon"
          onClick={() => refetch()}
          disabled={isLoading}
        >
          <RefreshCw className={`h-4 w-4 ${isLoading ? 'animate-spin' : ''}`} />
        </Button>
      </div>

      {/* Summary Cards */}
      {!isLoading && (
        <div className="flex flex-wrap items-center gap-3">
          <Badge
            variant={totalOnline > 0 ? 'default' : 'secondary'}
            className="gap-1.5 py-1.5 px-3"
          >
            <Wifi className="h-3.5 w-3.5" />
            ออนไลน์: {totalOnline}
          </Badge>

          {summary && summary.idle > 0 && (
            <Badge variant="outline" className="gap-1.5 py-1.5 px-3 text-status-success border-status-success">
              ว่าง: {summary.idle}
            </Badge>
          )}

          {summary && summary.processing > 0 && (
            <Badge className="gap-1.5 py-1.5 px-3 status-processing">
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
              กำลังทำงาน: {summary.processing}
            </Badge>
          )}

          {summary && summary.paused > 0 && (
            <Badge variant="outline" className="gap-1.5 py-1.5 px-3 text-status-danger border-status-danger">
              หยุดชั่วคราว: {summary.paused}
            </Badge>
          )}

          {summary && summary.total_jobs > 0 && (
            <Badge variant="outline" className="gap-1.5 py-1.5 px-3">
              <Activity className="h-3.5 w-3.5" />
              งานทั้งหมด: {summary.total_jobs}
            </Badge>
          )}

          {/* Worker Type Summary */}
          {summary?.by_type && (
            <>
              <span className="text-muted-foreground">|</span>
              {summary.by_type.transcode > 0 && (
                <Badge variant="outline" className="gap-1.5 py-1.5 px-3 status-transcode">
                  <Monitor className="h-3.5 w-3.5" />
                  Transcode: {summary.by_type.transcode}
                </Badge>
              )}
              {summary.by_type.subtitle > 0 && (
                <Badge variant="outline" className="gap-1.5 py-1.5 px-3 status-subtitle">
                  <Languages className="h-3.5 w-3.5" />
                  Subtitle: {summary.by_type.subtitle}
                </Badge>
              )}
            </>
          )}
        </div>
      )}

      {/* Table */}
      <div className="border rounded-lg">
        {isLoading ? (
          <div className="p-4 space-y-4">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : (
          <WorkerTable workers={workers} />
        )}
      </div>
    </div>
  )
}
