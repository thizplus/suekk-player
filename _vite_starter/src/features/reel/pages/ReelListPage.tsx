import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus, Trash2, MoreVertical, Loader2, Film, Download, Eye, Pencil } from 'lucide-react'
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
import { Dialog, DialogContent } from '@/components/ui/dialog'
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { useReels, useDeleteReel, useExportReel } from '../hooks'
import type { Reel, ReelFilterParams, ReelStatus } from '../types'
import { downloadReel, getReelBlobUrl } from '../utils/reelDownload'
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

// Style labels
const REEL_STYLE_LABELS: Record<string, string> = {
  letterbox: 'Letterbox',
  square: 'Square',
  fullcover: 'Full Cover',
}

interface DeleteTarget {
  id: string
  title: string
}

interface PreviewTarget {
  reel: Reel
  blobUrl: string | null
  isLoading: boolean
}

export function ReelListPage() {
  const navigate = useNavigate()
  const [filters, setFilters] = useState<ReelFilterParams>({
    page: 1,
    limit: 20,
    sortBy: 'created_at',
    sortOrder: 'desc',
  })
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null)
  const [preview, setPreview] = useState<PreviewTarget | null>(null)

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

  const formatDate = (dateString?: string) => {
    if (!dateString) return '-'
    return new Date(dateString).toLocaleDateString('th-TH', {
      day: '2-digit',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  const handlePreview = async (reel: Reel) => {
    if (!reel.video?.code) return

    // Set loading state
    setPreview({ reel, blobUrl: null, isLoading: true })

    // Fetch blob URL
    const blobUrl = await getReelBlobUrl(reel.video.code, reel.id)
    if (blobUrl) {
      setPreview({ reel, blobUrl, isLoading: false })
    } else {
      setPreview(null)
    }
  }

  const closePreview = () => {
    if (preview?.blobUrl) {
      URL.revokeObjectURL(preview.blobUrl)
    }
    setPreview(null)
  }

  // Cleanup blob URL on unmount
  useEffect(() => {
    return () => {
      if (preview?.blobUrl) {
        URL.revokeObjectURL(preview.blobUrl)
      }
    }
  }, [preview?.blobUrl])

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
        <Card>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>วิดีโอ</TableHead>
                <TableHead>รูปแบบ</TableHead>
                <TableHead>ช่วงเวลา</TableHead>
                <TableHead>สถานะ</TableHead>
                <TableHead>ขนาดไฟล์</TableHead>
                <TableHead>สร้างเมื่อ</TableHead>
                <TableHead className="w-[100px]"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {Array.from({ length: 5 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell><Skeleton className="h-4 w-48" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-20" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                  <TableCell><Skeleton className="h-5 w-16" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                  <TableCell><Skeleton className="h-4 w-32" /></TableCell>
                  <TableCell><Skeleton className="h-8 w-8" /></TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
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
          {/* Reel Table */}
          <Card>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>วิดีโอ</TableHead>
                  <TableHead>รูปแบบ</TableHead>
                  <TableHead>ช่วงเวลา</TableHead>
                  <TableHead>สถานะ</TableHead>
                  <TableHead>ขนาดไฟล์</TableHead>
                  <TableHead>สร้างเมื่อ</TableHead>
                  <TableHead className="w-[100px]"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {reels.map((reel) => (
                  <TableRow
                    key={reel.id}
                    className="cursor-pointer hover:bg-muted/50"
                    onClick={() => navigate(`/reels/${reel.id}/edit`)}
                  >
                    {/* Video Info */}
                    <TableCell>
                      <div className="flex items-center gap-3">
                        <div className="w-10 h-10 rounded bg-muted flex items-center justify-center shrink-0">
                          <Film className="h-5 w-5 text-muted-foreground/50" />
                        </div>
                        <div className="min-w-0">
                          <p className="font-medium truncate max-w-[200px]">
                            {reel.title || 'Untitled'}
                          </p>
                          <p className="text-xs text-muted-foreground truncate max-w-[200px]">
                            {reel.video?.title || reel.video?.code}
                          </p>
                        </div>
                      </div>
                    </TableCell>

                    {/* Style */}
                    <TableCell>
                      <span className="text-sm">
                        {REEL_STYLE_LABELS[reel.style || ''] || '-'}
                      </span>
                    </TableCell>

                    {/* Duration */}
                    <TableCell>
                      <span className="text-sm font-mono">
                        {formatDuration(reel.segmentEnd - reel.segmentStart)}
                      </span>
                    </TableCell>

                    {/* Status */}
                    <TableCell>
                      <Badge className={REEL_STATUS_STYLES[reel.status]}>
                        {REEL_STATUS_LABELS[reel.status]}
                      </Badge>
                    </TableCell>

                    {/* File Size */}
                    <TableCell>
                      <span className="text-sm text-muted-foreground">
                        {formatFileSize(reel.fileSize)}
                      </span>
                    </TableCell>

                    {/* Created At */}
                    <TableCell>
                      <span className="text-sm text-muted-foreground">
                        {formatDate(reel.createdAt)}
                      </span>
                    </TableCell>

                    {/* Actions */}
                    <TableCell>
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild onClick={(e) => e.stopPropagation()}>
                          <Button variant="ghost" size="icon" className="h-8 w-8">
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

                          {(reel.status === 'draft' || reel.status === 'ready' || reel.status === 'failed') && (
                            <DropdownMenuItem
                              onClick={(e) => {
                                e.stopPropagation()
                                handleExport(reel)
                              }}
                            >
                              <Download className="h-4 w-4 mr-2" />
                              {reel.status === 'ready' ? 'Re-Export' : 'Export'}
                            </DropdownMenuItem>
                          )}

                          {reel.status === 'ready' && reel.video?.code && (
                            <>
                              <DropdownMenuItem
                                onClick={(e) => {
                                  e.stopPropagation()
                                  handlePreview(reel)
                                }}
                              >
                                <Eye className="h-4 w-4 mr-2" />
                                ดูผลลัพธ์
                              </DropdownMenuItem>
                              <DropdownMenuItem
                                onClick={(e) => {
                                  e.stopPropagation()
                                  downloadReel(reel.video!.code, reel.id, reel.title)
                                }}
                              >
                                <Download className="h-4 w-4 mr-2" />
                                ดาวน์โหลด
                              </DropdownMenuItem>
                            </>
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
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </Card>

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

      {/* Preview Dialog */}
      <Dialog open={!!preview} onOpenChange={(open) => !open && closePreview()}>
        <DialogContent className="w-auto max-w-[90vw] p-0 overflow-hidden max-h-[85vh] [&>button]:hidden">
          <div className="aspect-[9/16] h-[75vh] bg-black flex items-center justify-center">
            {preview?.isLoading ? (
              <div className="flex flex-col items-center gap-2 text-white">
                <Loader2 className="h-8 w-8 animate-spin" />
                <span className="text-sm">กำลังโหลด...</span>
              </div>
            ) : preview?.blobUrl ? (
              <video
                src={preview.blobUrl}
                controls
                autoPlay
                className="w-full h-full object-contain"
              />
            ) : null}
          </div>
          {preview?.reel && !preview.isLoading && (
            <div className="p-3 flex justify-end border-t">
              <Button
                size="sm"
                onClick={() => {
                  if (preview.reel.video?.code) {
                    downloadReel(preview.reel.video.code, preview.reel.id, preview.reel.title)
                  }
                }}
              >
                <Download className="h-4 w-4 mr-2" />
                ดาวน์โหลด
              </Button>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
