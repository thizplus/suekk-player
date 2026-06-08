import { useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  Tv,
  Search,
  Loader2,
  Star,
  Play,
  ChevronLeft,
  ChevronRight,
  FolderOpen,
  Plus,
  Trash2,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Empty,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
  EmptyDescription,
} from '@/components/ui/empty'
import { useSeriesList, useSeriesCategories, useCreateSeriesCategory } from '../hooks'
import type { Series, SeriesFilterParams, SeriesCategory } from '../types'
import { toast } from 'sonner'

// ═══════════════════════════════════════════
// Series Card
// ═══════════════════════════════════════════

function SeriesCard({
  series,
  onClick,
}: {
  series: Series
  onClick: () => void
}) {
  return (
    <div
      className="group cursor-pointer rounded-lg border bg-card overflow-hidden hover:border-primary/50 transition-all"
      onClick={onClick}
    >
      {/* Poster */}
      <div className="relative aspect-[2/3] bg-muted overflow-hidden">
        {series.posterPath ? (
          <img
            src={series.posterPath}
            alt={series.title}
            className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-300"
            loading="lazy"
          />
        ) : (
          <div className="w-full h-full flex items-center justify-center">
            <Tv className="h-10 w-10 text-muted-foreground/30" />
          </div>
        )}

        {/* Overlay badges */}
        <div className="absolute top-2 left-2 flex flex-col gap-1">
          {series.quality && (
            <Badge variant="secondary" className="text-[10px] px-1.5 py-0 bg-black/70 text-white border-0">
              {series.quality}
            </Badge>
          )}
          {series.audioType && (
            <Badge
              variant="secondary"
              className={`text-[10px] px-1.5 py-0 border-0 ${
                series.audioType === 'Thai'
                  ? 'bg-sky-600 text-white'
                  : 'bg-black/70 text-white'
              }`}
            >
              {series.audioType === 'Thai' ? 'พากย์ไทย' : 'ซับไทย'}
            </Badge>
          )}
        </div>

        {/* Rating */}
        {series.rating > 0 && (
          <div className="absolute top-2 right-2 flex items-center gap-0.5 bg-black/70 text-yellow-400 rounded px-1.5 py-0.5 text-xs font-medium">
            <Star className="h-3 w-3 fill-current" />
            {series.rating}
          </div>
        )}

        {/* Episode count */}
        <div className="absolute bottom-2 right-2 bg-black/70 text-white rounded px-1.5 py-0.5 text-[10px]">
          {series.totalEpisodes} ตอน{series.isCompleted ? ' (จบ)' : ''}
        </div>

        {/* Trailer play icon */}
        {series.trailerYoutubeId && (
          <div className="absolute inset-0 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity bg-black/30">
            <Play className="h-10 w-10 text-white fill-white/80" />
          </div>
        )}
      </div>

      {/* Info */}
      <div className="p-2.5">
        <h3 className="text-sm font-medium truncate" title={series.title}>
          {series.title}
        </h3>
        {series.thaiTitle && (
          <p className="text-xs text-muted-foreground truncate mt-0.5" title={series.thaiTitle}>
            {series.thaiTitle}
          </p>
        )}
        <div className="flex items-center gap-2 mt-1.5">
          {series.year > 0 && (
            <span className="text-[11px] text-muted-foreground">{series.year}</span>
          )}
          {series.category && (
            <span className="text-[11px] text-muted-foreground truncate">
              {series.category.name}
            </span>
          )}
        </div>
      </div>
    </div>
  )
}

// ═══════════════════════════════════════════
// Series Detail Sheet
// ═══════════════════════════════════════════

function SeriesDetailSheet({
  series,
  open,
  onClose,
}: {
  series: Series | null
  open: boolean
  onClose: () => void
}) {
  if (!series) return null

  return (
    <Sheet open={open} onOpenChange={onClose}>
      <SheetContent className="w-full sm:max-w-xl overflow-y-auto">
        <SheetHeader>
          <SheetTitle className="text-left">{series.title}</SheetTitle>
        </SheetHeader>

        <div className="mt-4 space-y-4">
          {/* Poster + meta */}
          <div className="flex gap-4">
            {series.posterPath && (
              <img
                src={series.posterPath}
                alt={series.title}
                className="w-28 rounded-lg object-cover"
              />
            )}
            <div className="flex-1 space-y-2">
              {series.thaiTitle && (
                <p className="text-sm text-muted-foreground">{series.thaiTitle}</p>
              )}
              <div className="flex flex-wrap gap-1.5">
                {series.year > 0 && <Badge variant="outline">{series.year}</Badge>}
                {series.rating > 0 && (
                  <Badge variant="outline" className="gap-1">
                    <Star className="h-3 w-3 text-yellow-500 fill-yellow-500" />
                    {series.rating}
                  </Badge>
                )}
                {series.audioType && (
                  <Badge variant={series.audioType === 'Thai' ? 'default' : 'secondary'}>
                    {series.audioType === 'Thai' ? 'พากย์ไทย' : 'ซับไทย'}
                  </Badge>
                )}
                {series.isCompleted ? (
                  <Badge className="status-success">จบแล้ว</Badge>
                ) : (
                  <Badge className="status-processing">กำลังฉาย</Badge>
                )}
                <Badge variant="outline">{series.totalEpisodes} ตอน</Badge>
              </div>
              {series.quality && (
                <p className="text-xs text-muted-foreground">คุณภาพ: {series.quality}</p>
              )}
            </div>
          </div>

          {/* Trailer */}
          {series.trailerYoutubeId && (
            <div>
              <h4 className="text-sm font-medium mb-2">Trailer</h4>
              <div className="aspect-video rounded-lg overflow-hidden bg-muted">
                <iframe
                  src={`https://www.youtube.com/embed/${series.trailerYoutubeId}`}
                  className="w-full h-full"
                  allowFullScreen
                  title="Trailer"
                />
              </div>
            </div>
          )}

          {/* Description */}
          {series.description && (
            <div>
              <h4 className="text-sm font-medium mb-1">เรื่องย่อ</h4>
              <p className="text-sm text-muted-foreground leading-relaxed">
                {series.description}
              </p>
            </div>
          )}

          {/* Episodes */}
          {series.episodes && series.episodes.length > 0 && (
            <div>
              <h4 className="text-sm font-medium mb-2">
                ตอนทั้งหมด ({series.episodes.length})
              </h4>
              <div className="grid grid-cols-6 sm:grid-cols-8 gap-1.5">
                {series.episodes.map((ep) => (
                  <div
                    key={ep.id}
                    className={`text-center py-1.5 rounded text-xs font-medium border ${
                      ep.videoCode
                        ? 'bg-primary/10 border-primary/30 text-primary'
                        : 'bg-muted border-transparent text-muted-foreground'
                    }`}
                    title={ep.videoCode ? `Video: ${ep.videoCode}` : 'ยังไม่มี video'}
                  >
                    {ep.episodeNumber}
                  </div>
                ))}
              </div>
              <p className="text-[11px] text-muted-foreground mt-1.5">
                <span className="inline-block w-3 h-3 rounded bg-primary/10 border border-primary/30 align-middle mr-1" />
                มี video แล้ว
                <span className="inline-block w-3 h-3 rounded bg-muted align-middle ml-3 mr-1" />
                ยังไม่มี
              </p>
            </div>
          )}

          {/* Source link */}
          {series.slug && (
            <div className="pt-2 border-t">
              <p className="text-xs text-muted-foreground">
                Code: <span className="font-mono">{series.code}</span>
              </p>
            </div>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}

// ═══════════════════════════════════════════
// Category Manager Dialog
// ═══════════════════════════════════════════

function CategoryManagerDialog({
  open,
  onClose,
}: {
  open: boolean
  onClose: () => void
}) {
  const { data: categories = [], isLoading } = useSeriesCategories()
  const createCategory = useCreateSeriesCategory()
  const [newName, setNewName] = useState('')

  const handleCreate = async () => {
    const name = newName.trim()
    if (!name) return
    const slug = name.toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-]/g, '')
    try {
      await createCategory.mutateAsync({ name, slug })
      toast.success(`เพิ่มหมวดหมู่ "${name}" สำเร็จ`)
      setNewName('')
    } catch {
      toast.error('เกิดข้อผิดพลาด')
    }
  }

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>หมวดหมู่ซีรีส์</DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          {/* Add new */}
          <div className="flex gap-2">
            <Input
              placeholder="ชื่อหมวดหมู่ใหม่..."
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
              className="h-9"
            />
            <Button
              size="sm"
              onClick={handleCreate}
              disabled={!newName.trim() || createCategory.isPending}
              className="h-9"
            >
              <Plus className="h-4 w-4 mr-1" />
              เพิ่ม
            </Button>
          </div>

          {/* List */}
          {isLoading ? (
            <div className="flex justify-center py-4">
              <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
            </div>
          ) : (categories as SeriesCategory[]).length === 0 ? (
            <p className="text-sm text-muted-foreground text-center py-4">ยังไม่มีหมวดหมู่</p>
          ) : (
            <div className="space-y-1.5">
              {(categories as SeriesCategory[]).map((cat) => (
                <div
                  key={cat.id}
                  className="flex items-center justify-between px-3 py-2 rounded-lg border"
                >
                  <div>
                    <p className="text-sm font-medium">{cat.name}</p>
                    <p className="text-xs text-muted-foreground font-mono">/{cat.slug}</p>
                  </div>
                  <Badge variant="outline" className="text-xs">
                    {cat.seriesCount || 0} เรื่อง
                  </Badge>
                </div>
              ))}
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}

// ═══════════════════════════════════════════
// Main Page
// ═══════════════════════════════════════════

export function SeriesListPage() {
  const [searchParams, setSearchParams] = useSearchParams()

  // Filters from URL
  const filters: SeriesFilterParams = {
    search: searchParams.get('search') || '',
    categoryId: searchParams.get('categoryId') || '',
    audioType: searchParams.get('audioType') || '',
    year: Number(searchParams.get('year')) || 0,
    sortBy: searchParams.get('sortBy') || 'created_at',
    sortOrder: searchParams.get('sortOrder') || 'desc',
    page: Number(searchParams.get('page')) || 1,
    limit: 24,
  }

  const { data, isLoading } = useSeriesList(filters)
  const { data: categories } = useSeriesCategories()
  const [selectedSeries, setSelectedSeries] = useState<Series | null>(null)
  const [searchInput, setSearchInput] = useState(filters.search || '')
  const [showCategories, setShowCategories] = useState(false)

  const seriesList = data?.data || []
  const meta = data?.meta

  const updateFilter = (key: string, value: string) => {
    const params = new URLSearchParams(searchParams)
    if (value) {
      params.set(key, value)
    } else {
      params.delete(key)
    }
    params.set('page', '1')
    setSearchParams(params)
  }

  const handleSearch = () => {
    updateFilter('search', searchInput)
  }

  const goToPage = (page: number) => {
    const params = new URLSearchParams(searchParams)
    params.set('page', String(page))
    setSearchParams(params)
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">ซีรีส์</h1>
          <p className="text-sm text-muted-foreground">
            {meta ? `${meta.total} เรื่อง` : 'จัดการซีรีส์ทั้งหมด'}
          </p>
        </div>
        <Button size="sm" variant="outline" onClick={() => setShowCategories(true)}>
          <FolderOpen className="h-4 w-4 mr-2" />
          หมวดหมู่
        </Button>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap gap-2">
        <div className="flex gap-1 flex-1 min-w-[200px] max-w-sm">
          <Input
            placeholder="ค้นหาซีรีส์..."
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
            className="h-9"
          />
          <Button size="sm" variant="secondary" onClick={handleSearch} className="h-9 px-3">
            <Search className="h-4 w-4" />
          </Button>
        </div>

        {categories && (categories as SeriesCategory[]).length > 0 && (
          <Select value={filters.categoryId || 'all'} onValueChange={(v) => updateFilter('categoryId', v === 'all' ? '' : v)}>
            <SelectTrigger className="w-[160px] h-9">
              <SelectValue placeholder="หมวดหมู่" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">ทุกหมวดหมู่</SelectItem>
              {(categories as SeriesCategory[]).map((cat) => (
                <SelectItem key={cat.id} value={cat.id}>{cat.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}

        <Select value={filters.audioType || 'all'} onValueChange={(v) => updateFilter('audioType', v === 'all' ? '' : v)}>
          <SelectTrigger className="w-[130px] h-9">
            <SelectValue placeholder="เสียง" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">ทุกเสียง</SelectItem>
            <SelectItem value="Thai">พากย์ไทย</SelectItem>
            <SelectItem value="Sound Track">ซับไทย</SelectItem>
          </SelectContent>
        </Select>

        <Select value={filters.sortBy || 'created_at'} onValueChange={(v) => updateFilter('sortBy', v)}>
          <SelectTrigger className="w-[140px] h-9">
            <SelectValue placeholder="เรียงตาม" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="created_at">ล่าสุด</SelectItem>
            <SelectItem value="year">ปี</SelectItem>
            <SelectItem value="rating">คะแนน</SelectItem>
            <SelectItem value="title">ชื่อ</SelectItem>
            <SelectItem value="total_episodes">จำนวนตอน</SelectItem>
          </SelectContent>
        </Select>

        {(filters.search || filters.audioType || filters.categoryId) && (
          <Button
            size="sm"
            variant="ghost"
            className="h-9"
            onClick={() => {
              setSearchInput('')
              setSearchParams({})
            }}
          >
            ล้าง
          </Button>
        )}
      </div>

      {/* Grid */}
      {isLoading ? (
        <div className="flex justify-center py-20">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      ) : seriesList.length === 0 ? (
        <Empty className="border py-16">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <Tv className="h-6 w-6" />
            </EmptyMedia>
            <EmptyTitle>ไม่พบซีรีส์</EmptyTitle>
            <EmptyDescription>
              {filters.search ? `ไม่พบ "${filters.search}"` : 'ยังไม่มีซีรีส์ในระบบ'}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <>
          <div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 xl:grid-cols-8 gap-3">
            {seriesList.map((s) => (
              <SeriesCard
                key={s.id}
                series={s}
                onClick={() => setSelectedSeries(s)}
              />
            ))}
          </div>

          {/* Pagination */}
          {meta && meta.totalPages > 1 && (
            <div className="flex items-center justify-center gap-2 pt-4">
              <Button
                size="sm"
                variant="outline"
                disabled={!meta || meta.page <= 1}
                onClick={() => goToPage((meta?.page || 1) - 1)}
              >
                <ChevronLeft className="h-4 w-4" />
              </Button>
              <span className="text-sm text-muted-foreground">
                หน้า {meta.page} / {meta.totalPages}
              </span>
              <Button
                size="sm"
                variant="outline"
                disabled={!meta || meta.page >= meta.totalPages}
                onClick={() => goToPage((meta?.page || 1) + 1)}
              >
                <ChevronRight className="h-4 w-4" />
              </Button>
            </div>
          )}
        </>
      )}

      {/* Detail Sheet */}
      <SeriesDetailSheet
        series={selectedSeries}
        open={!!selectedSeries}
        onClose={() => setSelectedSeries(null)}
      />

      {/* Category Manager */}
      <CategoryManagerDialog
        open={showCategories}
        onClose={() => setShowCategories(false)}
      />
    </div>
  )
}
