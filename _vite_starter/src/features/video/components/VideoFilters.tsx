import { Search, X } from 'lucide-react'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useCategories } from '@/features/category/hooks'
import type { VideoFilterParams, VideoStatus } from '../types'

interface VideoFiltersProps {
  filters: VideoFilterParams
  onFiltersChange: (filters: VideoFilterParams) => void
}

export function VideoFilters({ filters, onFiltersChange }: VideoFiltersProps) {
  const { data: categories = [] } = useCategories()

  const hasActiveFilters = !!(
    filters.search ||
    filters.status ||
    filters.categoryId ||
    filters.dateFrom ||
    filters.dateTo ||
    (filters.sortBy && filters.sortBy !== 'created_at') ||
    filters.sortOrder === 'asc'
  )

  const clearFilters = () => {
    onFiltersChange({
      page: 1,
      limit: filters.limit,
      sortBy: 'created_at',
      sortOrder: 'desc',
    })
  }

  const updateFilter = (key: keyof VideoFilterParams, value: string | undefined) => {
    onFiltersChange({
      ...filters,
      [key]: value,
      page: 1, // reset page เมื่อ filter เปลี่ยน
    })
  }

  return (
    <div className="flex flex-wrap items-center gap-3 pb-4 border-b">
      {/* Search Input */}
      <div className="relative flex-1 min-w-[200px] max-w-sm">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
        <Input
          placeholder="ค้นหาชื่อหรือรหัสวิดีโอ..."
          value={filters.search || ''}
          onChange={(e) => updateFilter('search', e.target.value || undefined)}
          className="pl-9"
        />
      </div>

      {/* Category Select */}
      <Select
        value={filters.categoryId || 'all'}
        onValueChange={(v) => updateFilter('categoryId', v === 'all' ? undefined : v)}
      >
        <SelectTrigger className="w-[180px]">
          <SelectValue placeholder="หมวดหมู่" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">ทุกหมวดหมู่</SelectItem>
          {categories.map((cat) => (
            <SelectItem key={cat.id} value={cat.id}>
              {cat.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      {/* Status Select */}
      <Select
        value={filters.status || 'all'}
        onValueChange={(v) => updateFilter('status', v === 'all' ? undefined : v as VideoStatus)}
      >
        <SelectTrigger className="w-[150px]">
          <SelectValue placeholder="สถานะ" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">ทุกสถานะ</SelectItem>
          <SelectItem value="pending">รอดำเนินการ</SelectItem>
          <SelectItem value="queued">อยู่ในคิว</SelectItem>
          <SelectItem value="processing">กำลังประมวลผล</SelectItem>
          <SelectItem value="ready">พร้อมใช้งาน</SelectItem>
          <SelectItem value="failed">ล้มเหลว</SelectItem>
        </SelectContent>
      </Select>

      {/* Sort Select */}
      <Select
        value={`${filters.sortBy || 'created_at'}-${filters.sortOrder || 'desc'}`}
        onValueChange={(v) => {
          const [sortBy, sortOrder] = v.split('-') as [VideoFilterParams['sortBy'], VideoFilterParams['sortOrder']]
          onFiltersChange({
            ...filters,
            sortBy,
            sortOrder,
            page: 1,
          })
        }}
      >
        <SelectTrigger className="w-[150px]">
          <SelectValue placeholder="เรียงตาม" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="created_at-desc">ใหม่สุด</SelectItem>
          <SelectItem value="created_at-asc">เก่าสุด</SelectItem>
          <SelectItem value="title-asc">ชื่อ A-Z</SelectItem>
          <SelectItem value="title-desc">ชื่อ Z-A</SelectItem>
          <SelectItem value="views-desc">ยอดวิวสูงสุด</SelectItem>
        </SelectContent>
      </Select>

      {/* Clear Filters */}
      {hasActiveFilters && (
        <Button variant="ghost" size="sm" onClick={clearFilters} className="gap-1.5">
          <X className="h-4 w-4" />
          ล้างตัวกรอง
        </Button>
      )}
    </div>
  )
}
