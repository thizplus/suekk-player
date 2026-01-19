import { useState } from 'react'
import {
  BarChart3,
  TrendingUp,
  SkipForward,
  AlertTriangle,
  Smartphone,
  Monitor,
  Tablet,
  Clock,
} from 'lucide-react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import { Skeleton } from '@/components/ui/skeleton'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { DEVICE_TYPE_LABELS } from '@/constants/enums'
import { useAdStats, useDeviceStats, useProfileRanking } from '../hooks'
import type { AdStatsFilterParams } from '../types'

export function AdStatsOverview() {
  const [dateRange, setDateRange] = useState<AdStatsFilterParams>({
    start: getDefaultStartDate(),
    end: getDefaultEndDate(),
  })

  const { data: stats, isLoading: statsLoading } = useAdStats(dateRange)
  const { data: deviceStats, isLoading: deviceLoading } = useDeviceStats(dateRange)
  const { data: rankings, isLoading: rankingLoading } = useProfileRanking({ ...dateRange, limit: 5 })

  return (
    <div className="space-y-6">
      {/* Date Range Filter */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">ช่วงเวลา</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex gap-4 items-end">
            <div className="space-y-2">
              <Label>จากวันที่</Label>
              <Input
                type="date"
                value={dateRange.start}
                onChange={(e) => setDateRange({ ...dateRange, start: e.target.value })}
              />
            </div>
            <div className="space-y-2">
              <Label>ถึงวันที่</Label>
              <Input
                type="date"
                value={dateRange.end}
                onChange={(e) => setDateRange({ ...dateRange, end: e.target.value })}
              />
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Stats Overview */}
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <StatsCard
          title="Total Impressions"
          value={stats?.totalImpressions || 0}
          icon={BarChart3}
          loading={statsLoading}
        />
        <StatsCard
          title="Completion Rate"
          value={`${(stats?.completionRate || 0).toFixed(1)}%`}
          icon={TrendingUp}
          description={`${stats?.completed || 0} ดูจบ`}
          loading={statsLoading}
          positive={stats?.completionRate ? stats.completionRate > 50 : undefined}
        />
        <StatsCard
          title="Skip Rate"
          value={`${(stats?.skipRate || 0).toFixed(1)}%`}
          icon={SkipForward}
          description={`${stats?.skipped || 0} กด Skip`}
          loading={statsLoading}
          warning={stats?.skipRate ? stats.skipRate > 50 : undefined}
        />
        <StatsCard
          title="Error Rate"
          value={`${(stats?.errorRate || 0).toFixed(1)}%`}
          icon={AlertTriangle}
          description={`${stats?.errors || 0} errors`}
          loading={statsLoading}
          danger={stats?.errorRate ? stats.errorRate > 5 : undefined}
        />
      </div>

      {/* Watch Duration & Skip Time */}
      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="text-base flex items-center gap-2">
              <Clock className="h-4 w-4" />
              เวลาเฉลี่ย
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {statsLoading ? (
              <div className="space-y-2">
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-4 w-3/4" />
              </div>
            ) : (
              <>
                <div className="flex justify-between items-center">
                  <span className="text-sm text-muted-foreground">เวลาดูเฉลี่ย</span>
                  <span className="font-mono font-medium">
                    {formatDuration(stats?.avgWatchDuration || 0)}
                  </span>
                </div>
                <div className="flex justify-between items-center">
                  <span className="text-sm text-muted-foreground">เวลา Skip เฉลี่ย</span>
                  <span className="font-mono font-medium">
                    {formatDuration(stats?.avgSkipTime || 0)}
                  </span>
                </div>
              </>
            )}
          </CardContent>
        </Card>

        {/* Device Stats */}
        <Card>
          <CardHeader>
            <CardTitle className="text-base">อุปกรณ์</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {deviceLoading ? (
              <div className="space-y-2">
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-4 w-full" />
              </div>
            ) : (
              <>
                <DeviceRow
                  icon={Smartphone}
                  label={DEVICE_TYPE_LABELS.mobile}
                  count={deviceStats?.mobile || 0}
                  total={getTotalDevices(deviceStats)}
                />
                <DeviceRow
                  icon={Monitor}
                  label={DEVICE_TYPE_LABELS.desktop}
                  count={deviceStats?.desktop || 0}
                  total={getTotalDevices(deviceStats)}
                />
                <DeviceRow
                  icon={Tablet}
                  label={DEVICE_TYPE_LABELS.tablet}
                  count={deviceStats?.tablet || 0}
                  total={getTotalDevices(deviceStats)}
                />
              </>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Profile Rankings */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Profile Rankings</CardTitle>
          <CardDescription>Top 5 profiles by ad views</CardDescription>
        </CardHeader>
        <CardContent>
          {rankingLoading ? (
            <div className="space-y-3">
              {[1, 2, 3].map((i) => (
                <Skeleton key={i} className="h-12 w-full" />
              ))}
            </div>
          ) : rankings && rankings.length > 0 ? (
            <div className="space-y-3">
              {rankings.map((rank, index) => (
                <div
                  key={rank.profileId}
                  className="flex items-center justify-between p-3 rounded-lg bg-muted/50"
                >
                  <div className="flex items-center gap-3">
                    <Badge variant="outline" className="w-6 h-6 flex items-center justify-center">
                      {index + 1}
                    </Badge>
                    <div>
                      <p className="font-medium">{rank.profileName}</p>
                      <p className="text-xs text-muted-foreground">
                        {rank.totalViews.toLocaleString()} views
                      </p>
                    </div>
                  </div>
                  <Badge variant={rank.completionRate > 50 ? 'default' : 'secondary'}>
                    {rank.completionRate.toFixed(1)}% completion
                  </Badge>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground text-center py-8">
              ยังไม่มีข้อมูล ad impressions
            </p>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

// ==================== Helper Components ====================

interface StatsCardProps {
  title: string
  value: string | number
  icon: React.ElementType
  description?: string
  loading?: boolean
  positive?: boolean
  warning?: boolean
  danger?: boolean
}

function StatsCard({
  title,
  value,
  icon: Icon,
  description,
  loading,
  positive,
  warning,
  danger,
}: StatsCardProps) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">{title}</CardTitle>
        <Icon className="h-4 w-4 text-muted-foreground" />
      </CardHeader>
      <CardContent>
        {loading ? (
          <Skeleton className="h-8 w-24" />
        ) : (
          <>
            <div
              className={`text-2xl font-bold ${
                positive ? 'text-status-success' : warning ? 'text-status-warning' : danger ? 'text-status-danger' : ''
              }`}
            >
              {typeof value === 'number' ? value.toLocaleString() : value}
            </div>
            {description && (
              <p className="text-xs text-muted-foreground">{description}</p>
            )}
          </>
        )}
      </CardContent>
    </Card>
  )
}

interface DeviceRowProps {
  icon: React.ElementType
  label: string
  count: number
  total: number
}

function DeviceRow({ icon: Icon, label, count, total }: DeviceRowProps) {
  const percentage = total > 0 ? (count / total) * 100 : 0

  return (
    <div className="space-y-1">
      <div className="flex items-center justify-between text-sm">
        <div className="flex items-center gap-2">
          <Icon className="h-4 w-4 text-muted-foreground" />
          <span>{label}</span>
        </div>
        <span className="font-mono">{count.toLocaleString()}</span>
      </div>
      <Progress value={percentage} className="h-2" />
    </div>
  )
}

// ==================== Helper Functions ====================

function getDefaultStartDate(): string {
  const date = new Date()
  date.setDate(date.getDate() - 7)
  return date.toISOString().split('T')[0]
}

function getDefaultEndDate(): string {
  return new Date().toISOString().split('T')[0]
}

function formatDuration(seconds: number): string {
  if (seconds < 60) {
    return `${seconds.toFixed(1)}s`
  }
  const mins = Math.floor(seconds / 60)
  const secs = Math.round(seconds % 60)
  return `${mins}m ${secs}s`
}

function getTotalDevices(stats?: { mobile: number; desktop: number; tablet: number }): number {
  if (!stats) return 0
  return stats.mobile + stats.desktop + stats.tablet
}
