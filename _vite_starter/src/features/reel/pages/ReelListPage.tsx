import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Plus,
  Trash2,
  MoreVertical,
  Loader2,
  Film,
  Clock,
  Download,
  Eye,
  Pencil,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from '@/components/ui/pagination'
import {
  Card,
  CardContent,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { useReels, useDeleteReel, useExportReel } from '../hooks'
import type { Reel, ReelFilterParams, ReelStatus } from '../types'
import { toast } from 'sonner'

// Status styles
const REEL_STATUS_LABELS: Record<ReelStatus, string> = {
  draft: 'แบบร่าง',
  exporting: 'กำลัง Export',
  ready: 'พร้อมใช้งาน',
  failed: 'ล้มเหลว',
}

const REEL_STATUS_STYLES: Record<ReelStatus, string> = {
  draft: 'status-info',
  exporting: 'status-pending',
  ready: 'status-success',
  failed: 'status-danger',
}

interface DeleteTarget {
  id: string
  title: string
}

export function ReelListPage() {
  const navigate = useNavigate()
  const [filters, setFilters] = useState<ReelFilterParams>({
    page: 1,
    limit: 12,
    sortBy: 'created_at',
    sortOrder: 'desc',
  })
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null)

  const { data, isLoading, error } = useReels(filters)
  const deleteReel = useDeleteReel()
  const exportReel = useExportReel()

  const page = filters.page ?? 1
  const totalPages = data?.meta.totalPages ?? 1
  const reels = data?.data ?? []

  const setPage = (newPage: number) => {
    setFilters((prev) => ({ ...prev, page: newPage }))
  }

  const handleDelete = () => {
    if (!deleteTarget) return

    deleteReel.mutate(deleteTarget.id, {
      onSuccess: () => {
        toast.success('ลบ Reel สำเร็จ')
        setDeleteTarget(null)
      },
      onError: (err) => {
        toast.error(`ลบไม่สำเร็จ: ${err.message}`)
      },
    })
  }

  const handleExport = (reel: Reel) => {
    exportReel.mutate(reel.id, {
      onSuccess: () => {
        toast.success('เริ่ม Export แล้ว')
      },
      onError: (err) => {
        toast.error(`Export ไม่สำเร็จ: ${err.message}`)
      },
    })
  }

  const formatDuration = (seconds: number) => {
    const mins = Math.floor(seconds / 60)
    const secs = Math.floor(seconds % 60)
    return `${mins}:${secs.toString().padStart(2, '0')}`
  }

  const formatFileSize = (bytes?: number) => {
    if (!bytes) return '-'
    const mb = bytes / (1024 * 1024)
    return `${mb.toFixed(1)} MB`
  }

  if (error) {
    return (
      <div className="flex items-center justify-center h-64">
        <p className="text-destructive">เกิดข้อผิดพลาด: {error.message}</p>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">Reel Generator</h1>
          <p className="text-muted-foreground">
            สร้าง Reels สำหรับโปรโมทบน Social Media
          </p>
        </div>
        <Button onClick={() => navigate('/reels/create')}>
          <Plus className="h-4 w-4 mr-2" />
          สร้าง Reel ใหม่
        </Button>
      </div>

      {/* Content */}
      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <Card key={i}>
              <Skeleton className="aspect-[9/16] w-full" />
              <CardContent className="p-4 space-y-2">
                <Skeleton className="h-4 w-3/4" />
                <Skeleton className="h-3 w-1/2" />
              </CardContent>
            </Card>
          ))}
        </div>
      ) : reels.length === 0 ? (
        <Card className="p-12">
          <div className="flex flex-col items-center justify-center text-center space-y-4">
            <Film className="h-16 w-16 text-muted-foreground/50" />
            <div>
              <h3 className="text-lg font-medium">ยังไม่มี Reels</h3>
              <p className="text-muted-foreground">
                เริ่มสร้าง Reel แรกของคุณเพื่อโปรโมทวิดีโอบน Social Media
              </p>
            </div>
            <Button onClick={() => navigate('/reels/create')}>
              <Plus className="h-4 w-4 mr-2" />
              สร้าง Reel ใหม่
            </Button>
          </div>
        </Card>
      ) : (
        <>
          {/* Reel Grid */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
            {reels.map((reel) => (
              <Card
                key={reel.id}
                className="group overflow-hidden hover:ring-2 hover:ring-primary/20 transition-all cursor-pointer"
                onClick={() => navigate(`/reels/${reel.id}/edit`)}
              >
                {/* Thumbnail */}
                <div className="relative aspect-[9/16] bg-muted">
                  {reel.thumbnailUrl ? (
                    <img
                      src={reel.thumbnailUrl}
                      alt={reel.title}
                      className="w-full h-full object-cover"
                    />
                  ) : reel.video?.thumbnailUrl ? (
                    <img
                      src={reel.video.thumbnailUrl}
                      alt={reel.title}
                      className="w-full h-full object-cover opacity-50"
                    />
                  ) : (
                    <div className="w-full h-full flex items-center justify-center">
                      <Film className="h-12 w-12 text-muted-foreground/30" />
                    </div>
                  )}

                  {/* Status Badge */}
                  <div className="absolute top-2 left-2">
                    <Badge className={REEL_STATUS_STYLES[reel.status]}>
                      {REEL_STATUS_LABELS[reel.status]}
                    </Badge>
                  </div>

                  {/* Duration */}
                  <div className="absolute bottom-2 right-2 bg-black/70 text-white text-xs px-2 py-1 rounded">
                    <Clock className="h-3 w-3 inline mr-1" />
                    {formatDuration(reel.segmentEnd - reel.segmentStart)}
                  </div>

                  {/* Hover Overlay */}
                  <div className="absolute inset-0 bg-black/50 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center gap-2">
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={(e) => {
                        e.stopPropagation()
                        navigate(`/reels/${reel.id}/edit`)
                      }}
                    >
                      <Pencil className="h-4 w-4" />
                    </Button>
                    {reel.status === 'ready' && reel.outputUrl && (
                      <Button
                        size="sm"
                        variant="secondary"
                        onClick={(e) => {
                          e.stopPropagation()
                          window.open(reel.outputUrl, '_blank')
                        }}
                      >
                        <Download className="h-4 w-4" />
                      </Button>
                    )}
                  </div>
                </div>

                {/* Info */}
                <CardContent className="p-3">
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0 flex-1">
                      <h3 className="font-medium truncate">
                        {reel.title || reel.video?.code || 'Untitled'}
                      </h3>
                      <p className="text-xs text-muted-foreground truncate">
                        {reel.video?.title}
                      </p>
                      {reel.status === 'ready' && reel.fileSize && (
                        <p className="text-xs text-muted-foreground mt-1">
                          {formatFileSize(reel.fileSize)}
                        </p>
                      )}
                    </div>

                    {/* Actions */}
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild onClick={(e) => e.stopPropagation()}>
                        <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0">
                          <MoreVertical className="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem
                          onClick={(e) => {
                            e.stopPropagation()
                            navigate(`/reels/${reel.id}/edit`)
                          }}
                        >
                          <Pencil className="h-4 w-4 mr-2" />
                          แก้ไข
                        </DropdownMenuItem>

                        {reel.status === 'draft' && (
                          <DropdownMenuItem
                            onClick={(e) => {
                              e.stopPropagation()
                              handleExport(reel)
                            }}
                          >
                            <Download className="h-4 w-4 mr-2" />
                            Export
                          </DropdownMenuItem>
                        )}

                        {reel.status === 'ready' && reel.outputUrl && (
                          <DropdownMenuItem
                            onClick={(e) => {
                              e.stopPropagation()
                              window.open(reel.outputUrl, '_blank')
                            }}
                          >
                            <Eye className="h-4 w-4 mr-2" />
                            ดูผลลัพธ์
                          </DropdownMenuItem>
                        )}

                        <DropdownMenuSeparator />

                        <DropdownMenuItem
                          className="text-destructive"
                          onClick={(e) => {
                            e.stopPropagation()
                            setDeleteTarget({
                              id: reel.id,
                              title: reel.title || reel.video?.code || 'Untitled',
                            })
                          }}
                        >
                          <Trash2 className="h-4 w-4 mr-2" />
                          ลบ
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>

          {/* Pagination */}
          {totalPages > 1 && (
            <Pagination>
              <PaginationContent>
                <PaginationItem>
                  <PaginationPrevious
                    onClick={() => setPage(Math.max(1, page - 1))}
                    className={page <= 1 ? 'pointer-events-none opacity-50' : 'cursor-pointer'}
                  />
                </PaginationItem>

                {Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
                  const pageNum = i + 1
                  return (
                    <PaginationItem key={pageNum}>
                      <PaginationLink
                        onClick={() => setPage(pageNum)}
                        isActive={page === pageNum}
                        className="cursor-pointer"
                      >
                        {pageNum}
                      </PaginationLink>
                    </PaginationItem>
                  )
                })}

                <PaginationItem>
                  <PaginationNext
                    onClick={() => setPage(Math.min(totalPages, page + 1))}
                    className={page >= totalPages ? 'pointer-events-none opacity-50' : 'cursor-pointer'}
                  />
                </PaginationItem>
              </PaginationContent>
            </Pagination>
          )}
        </>
      )}

      {/* Delete Confirmation */}
      <AlertDialog open={!!deleteTarget} onOpenChange={() => setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>ยืนยันการลบ</AlertDialogTitle>
            <AlertDialogDescription>
              คุณต้องการลบ Reel "{deleteTarget?.title}" หรือไม่?
              การดำเนินการนี้ไม่สามารถย้อนกลับได้
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>ยกเลิก</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDelete}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              disabled={deleteReel.isPending}
            >
              {deleteReel.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin mr-2" />
              ) : (
                <Trash2 className="h-4 w-4 mr-2" />
              )}
              ลบ
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
