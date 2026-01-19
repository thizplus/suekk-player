import { useState } from 'react'
import { RefreshCw, Trash2, AlertTriangle, Loader2, MoreVertical, FileWarning, Video, ChevronDown, ChevronUp, Clock, Server } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
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
  Empty,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
  EmptyDescription,
} from '@/components/ui/empty'
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
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { useDLQVideos, useRetryDLQ, useDeleteDLQ } from '../hooks'
import { toast } from 'sonner'
import type { DLQVideo } from '../types'

export function DLQPage() {
  const [page, setPage] = useState(1)
  const [deleteTarget, setDeleteTarget] = useState<DLQVideo | null>(null)
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const limit = 20

  const { data, isLoading, error, refetch } = useDLQVideos({ page, limit })
  const retryDLQ = useRetryDLQ()
  const deleteDLQ = useDeleteDLQ()

  const totalPages = data?.meta.totalPages ?? 1
  const videos = data?.data ?? []

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString('th-TH', {
      day: 'numeric',
      month: 'short',
      year: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  const handleRetry = (video: DLQVideo) => {
    retryDLQ.mutate(video.id, {
      onSuccess: (result) => {
        toast.success(`ส่ง "${video.title}" กลับเข้าคิวแล้ว`, {
          description: `รหัสวิดีโอ: ${result.code}`,
        })
      },
      onError: (err) => {
        toast.error('ไม่สามารถลองใหม่ได้', {
          description: err.message,
        })
      },
    })
  }

  const handleDelete = () => {
    if (!deleteTarget) return

    deleteDLQ.mutate(deleteTarget.id, {
      onSuccess: () => {
        toast.success(`ลบวิดีโอ "${deleteTarget.title}" เรียบร้อยแล้ว`)
        setDeleteTarget(null)
      },
      onError: (err) => {
        toast.error('ไม่สามารถลบได้', {
          description: err.message,
        })
      },
    })
  }

  const truncateError = (error: string, maxLength = 100) => {
    if (error.length <= maxLength) return error
    return error.slice(0, maxLength) + '...'
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="h-10 w-10 rounded-lg bg-destructive/10 flex items-center justify-center">
            <AlertTriangle className="h-5 w-5 text-destructive" />
          </div>
          <div>
            <h1 className="text-2xl font-semibold">งานล้มเหลว</h1>
            <p className="text-sm text-muted-foreground">
              {data ? `${data.meta.total} วิดีโอที่ต้องตรวจสอบ` : 'วิดีโอที่ล้มเหลวหลายครั้ง'}
            </p>
          </div>
        </div>
        <Button variant="outline" size="sm" onClick={() => refetch()}>
          <RefreshCw className="h-4 w-4 mr-2" />
          รีเฟรช
        </Button>
      </div>

      {/* Info Banner */}
      <div className="rounded-lg border border-destructive/30 bg-destructive/10 p-4">
        <div className="flex gap-3">
          <FileWarning className="h-5 w-5 text-destructive shrink-0 mt-0.5" />
          <div className="text-sm text-destructive">
            <p className="font-medium">
              วิดีโอเหล่านี้ล้มเหลวเกิน 3 ครั้ง
            </p>
            <p className="mt-1 opacity-80">
              ตรวจสอบ error message และตัดสินใจว่าจะ retry หรือลบออก
              การ retry จะรีเซ็ต retry count และส่งกลับเข้าคิวใหม่
            </p>
          </div>
        </div>
      </div>

      {/* DLQ List */}
      {isLoading ? (
        <div className="flex justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      ) : error ? (
        <p className="text-sm text-destructive py-8 text-center">เกิดข้อผิดพลาดในการโหลดข้อมูล</p>
      ) : videos.length === 0 ? (
        <Empty className="border">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <Video className="h-6 w-6" />
            </EmptyMedia>
            <EmptyTitle>ไม่มีงานล้มเหลว</EmptyTitle>
            <EmptyDescription>
              ระบบทำงานปกติ ไม่มีวิดีโอที่ต้องตรวจสอบ
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <div className="space-y-2">
          {videos.map((video) => (
            <Collapsible
              key={video.id}
              open={expandedId === video.id}
              onOpenChange={(open) => setExpandedId(open ? video.id : null)}
            >
              <div className="rounded-lg border border-destructive/20 bg-destructive/5 hover:bg-destructive/10 transition-colors">
                <div className="flex items-start gap-3 px-4 py-3">
                  <AlertTriangle className="h-4 w-4 text-destructive shrink-0 mt-1" />
                  <div className="flex-1 min-w-0 space-y-1">
                    <div className="flex items-center gap-2">
                      <p className="font-medium truncate">{video.title}</p>
                      <Badge variant="outline" className="shrink-0">
                        ลองแล้ว {video.retryCount} ครั้ง
                      </Badge>
                    </div>
                    <p className="text-xs text-muted-foreground font-mono">{video.code}</p>
                    <p className="text-sm text-destructive/80 font-mono bg-destructive/5 rounded px-2 py-1 mt-2">
                      {truncateError(video.lastError)}
                    </p>
                    <div className="flex items-center gap-4 mt-1">
                      <p className="text-xs text-muted-foreground">
                        อัปเดตล่าสุด: {formatDate(video.updatedAt)}
                      </p>
                      {video.errorHistory && video.errorHistory.length > 0 && (
                        <CollapsibleTrigger asChild>
                          <Button variant="ghost" size="sm" className="h-6 px-2 text-xs">
                            {expandedId === video.id ? (
                              <>
                                <ChevronUp className="h-3 w-3 mr-1" />
                                ซ่อนประวัติ
                              </>
                            ) : (
                              <>
                                <ChevronDown className="h-3 w-3 mr-1" />
                                ดูประวัติ ({video.errorHistory.length})
                              </>
                            )}
                          </Button>
                        </CollapsibleTrigger>
                      )}
                    </div>
                  </div>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0">
                        <MoreVertical className="h-4 w-4" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem
                        onClick={() => handleRetry(video)}
                        disabled={retryDLQ.isPending}
                      >
                        <RefreshCw className="h-4 w-4 mr-2" />
                        ลองใหม่
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem
                        className="text-destructive focus:text-destructive"
                        onClick={() => setDeleteTarget(video)}
                      >
                        <Trash2 className="h-4 w-4 mr-2" />
                        ลบวิดีโอ
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>

                {/* Error History */}
                <CollapsibleContent>
                  {video.errorHistory && video.errorHistory.length > 0 && (
                    <div className="px-4 pb-3 pt-0 border-t border-destructive/10">
                      <p className="text-xs font-medium text-muted-foreground mb-2 pt-3">
                        ประวัติความผิดพลาด
                      </p>
                      <div className="space-y-2">
                        {video.errorHistory.map((record, index) => (
                          <div
                            key={index}
                            className="text-xs bg-background/50 rounded p-2 space-y-1"
                          >
                            <div className="flex items-center gap-2 text-muted-foreground">
                              <Badge variant="secondary" className="text-[10px] h-4 px-1">
                                #{record.attempt}
                              </Badge>
                              <span className="inline-flex items-center gap-1">
                                <Clock className="h-3 w-3" />
                                {record.timestamp}
                              </span>
                              {record.workerId && (
                                <span className="inline-flex items-center gap-1">
                                  <Server className="h-3 w-3" />
                                  {record.workerId.slice(0, 8)}...
                                </span>
                              )}
                              {record.stage && (
                                <Badge variant="outline" className="text-[10px] h-4 px-1">
                                  {record.stage}
                                </Badge>
                              )}
                            </div>
                            <p className="font-mono text-destructive/70 break-all">
                              {record.error}
                            </p>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </CollapsibleContent>
              </div>
            </Collapsible>
          ))}
        </div>
      )}

      {/* Pagination */}
      {data && totalPages > 1 && (
        <div className="flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            หน้า {page} / {totalPages}
          </p>
          <Pagination>
            <PaginationContent>
              <PaginationItem>
                <PaginationPrevious
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  className={page <= 1 ? 'pointer-events-none opacity-50' : 'cursor-pointer'}
                />
              </PaginationItem>
              {Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
                let pageNum: number
                if (totalPages <= 5) {
                  pageNum = i + 1
                } else if (page <= 3) {
                  pageNum = i + 1
                } else if (page >= totalPages - 2) {
                  pageNum = totalPages - 4 + i
                } else {
                  pageNum = page - 2 + i
                }
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
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                  className={page >= totalPages ? 'pointer-events-none opacity-50' : 'cursor-pointer'}
                />
              </PaginationItem>
            </PaginationContent>
          </Pagination>
        </div>
      )}

      {/* Delete Confirmation Dialog */}
      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>ยืนยันการลบวิดีโอ</AlertDialogTitle>
            <AlertDialogDescription>
              คุณต้องการลบวิดีโอ <span className="font-medium text-foreground">"{deleteTarget?.title}"</span> หรือไม่?
              <br />
              <span className="text-destructive">การดำเนินการนี้จะลบไฟล์ต้นฉบับและข้อมูลทั้งหมด ไม่สามารถกู้คืนได้</span>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>ยกเลิก</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDelete}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {deleteDLQ.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin mr-2" />
              ) : (
                <Trash2 className="h-4 w-4 mr-2" />
              )}
              ลบวิดีโอ
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
