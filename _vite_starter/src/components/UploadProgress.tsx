import { X, CheckCircle, AlertCircle, Loader2, ChevronDown, ChevronUp, Film, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { useUploadStore, type UploadItem } from '@/stores/upload-store'
import { cn } from '@/lib/utils'
import { useQueryClient } from '@tanstack/react-query'
import { videoKeys } from '@/features/video/hooks'
import { useEffect, useRef } from 'react'

function UploadItemRow({ item, onRemove }: { item: UploadItem; onRemove: () => void }) {
  const isUploading = item.status === 'uploading'
  const isSuccess = item.status === 'success'
  const isError = item.status === 'error'

  return (
    <div className="flex items-center gap-3 p-3 border-b border-border last:border-b-0">
      {/* Icon */}
      <div className="shrink-0">
        {isUploading && <Loader2 className="size-5 text-primary animate-spin" />}
        {isSuccess && <CheckCircle className="size-5 text-status-success" />}
        {isError && <AlertCircle className="size-5 text-destructive" />}
      </div>

      {/* Info */}
      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium truncate" title={item.title}>
          {item.title}
        </p>
        <div className="flex items-center gap-2">
          {isUploading && (
            <span className="text-xs text-muted-foreground">กำลังอัปโหลด...</span>
          )}
          {isSuccess && (
            <span className="text-xs text-status-success">สำเร็จ</span>
          )}
          {isError && (
            <span className="text-xs text-destructive truncate" title={item.error}>
              {item.error || 'ล้มเหลว'}
            </span>
          )}
        </div>
        {isUploading && (
          <Progress value={item.progress} className="h-1 mt-1" />
        )}
      </div>

      {/* Remove button (only for completed/error) */}
      {!isUploading && (
        <Button
          variant="ghost"
          size="icon"
          className="size-7 shrink-0"
          onClick={onRemove}
        >
          <X className="size-4" />
        </Button>
      )}
    </div>
  )
}

export function UploadProgress() {
  const { uploads, isMinimized, removeUpload, clearCompleted, setMinimized } = useUploadStore()
  const queryClient = useQueryClient()
  const prevSuccessCountRef = useRef(0)

  // นับจำนวนที่ success เพื่อ invalidate query เมื่อมีการเปลี่ยนแปลง
  const successCount = uploads.filter((u) => u.status === 'success').length
  const uploadingCount = uploads.filter((u) => u.status === 'uploading').length
  const errorCount = uploads.filter((u) => u.status === 'error').length
  const hasCompleted = successCount > 0 || errorCount > 0

  // Invalidate video list เมื่อมี upload สำเร็จใหม่
  useEffect(() => {
    if (successCount > prevSuccessCountRef.current) {
      queryClient.invalidateQueries({ queryKey: videoKeys.all })
    }
    prevSuccessCountRef.current = successCount
  }, [successCount, queryClient])

  // ไม่แสดงถ้าไม่มี uploads
  if (uploads.length === 0) {
    return null
  }

  return (
    <div
      className={cn(
        'fixed bottom-4 right-4 z-50 w-80 bg-card border border-border rounded-lg shadow-lg overflow-hidden',
        'animate-in slide-in-from-bottom-5 duration-300'
      )}
    >
      {/* Header */}
      <div
        className="flex items-center justify-between px-3 py-2 bg-muted/50 cursor-pointer"
        onClick={() => setMinimized(!isMinimized)}
      >
        <div className="flex items-center gap-2">
          <Film className="size-4 text-muted-foreground" />
          <span className="text-sm font-medium">
            อัปโหลดวิดีโอ
            {uploadingCount > 0 && (
              <span className="text-muted-foreground ml-1">({uploadingCount} กำลังอัปโหลด)</span>
            )}
          </span>
        </div>
        <div className="flex items-center gap-1">
          {hasCompleted && (
            <Button
              variant="ghost"
              size="icon"
              className="size-6"
              onClick={(e) => {
                e.stopPropagation()
                clearCompleted()
              }}
              title="ล้างรายการที่เสร็จแล้ว"
            >
              <Trash2 className="size-3.5" />
            </Button>
          )}
          <Button variant="ghost" size="icon" className="size-6">
            {isMinimized ? (
              <ChevronUp className="size-4" />
            ) : (
              <ChevronDown className="size-4" />
            )}
          </Button>
        </div>
      </div>

      {/* Upload List */}
      {!isMinimized && (
        <div className="max-h-64 overflow-y-auto">
          {uploads.map((item) => (
            <UploadItemRow
              key={item.id}
              item={item}
              onRemove={() => removeUpload(item.id)}
            />
          ))}
        </div>
      )}

      {/* Minimized summary - แสดง progress เฉลี่ยของไฟล์ที่กำลังอัพโหลด */}
      {isMinimized && uploadingCount > 0 && (
        <div className="px-3 py-2">
          <Progress
            value={
              uploads
                .filter((u) => u.status === 'uploading')
                .reduce((sum, u) => sum + u.progress, 0) / uploadingCount
            }
            className="h-1"
          />
        </div>
      )}
    </div>
  )
}
