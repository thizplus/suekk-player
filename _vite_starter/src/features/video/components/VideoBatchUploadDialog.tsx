import { useState, useCallback } from 'react'
import { Upload, X, Film, Files, FolderOpen } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { ScrollArea } from '@/components/ui/scroll-area'
import { useUploadLimits } from '../hooks'
import { useCategories } from '@/features/category/hooks'
import { useUploadStore } from '@/stores/upload-store'
import { toast } from 'sonner'

// Fallback values ถ้าดึงจาก API ไม่ได้
const DEFAULT_ALLOWED_TYPES = ['video/mp4', 'video/webm', 'video/quicktime', 'video/x-msvideo', 'video/x-matroska', 'video/MP2T', 'video/mp2t', 'video/vnd.dlna.mpeg-tts']
const DEFAULT_MAX_SIZE = 10 * 1024 * 1024 * 1024 // 10GB
const MAX_FILES = 10

interface FileItem {
  id: string
  file: File
  title: string // ชื่อที่ user กำหนด
}

interface VideoBatchUploadDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function VideoBatchUploadDialog({ open, onOpenChange }: VideoBatchUploadDialogProps) {
  const { addUpload } = useUploadStore()
  const { data: uploadLimits } = useUploadLimits()
  const { data: categories } = useCategories()
  const [files, setFiles] = useState<FileItem[]>([])
  const [categoryId, setCategoryId] = useState<string>('') // หมวดหมู่ที่ใช้กับทุกไฟล์
  const [dragActive, setDragActive] = useState(false)

  // ใช้ค่าจาก API หรือ fallback
  const maxSize = uploadLimits?.max_file_size ?? DEFAULT_MAX_SIZE
  const maxSizeGB = uploadLimits?.max_file_size_gb ?? 10
  const allowedTypes = uploadLimits?.allowed_types ?? DEFAULT_ALLOWED_TYPES

  const resetForm = () => {
    setFiles([])
    setCategoryId('')
  }

  const handleClose = () => {
    resetForm()
    onOpenChange(false)
  }

  const handleDrag = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    if (e.type === 'dragenter' || e.type === 'dragover') {
      setDragActive(true)
    } else if (e.type === 'dragleave') {
      setDragActive(false)
    }
  }, [])

  const validateFile = (file: File): string | null => {
    // รองรับทั้ง MIME type และ extension สำหรับ .ts files
    const isValidType = allowedTypes.includes(file.type) ||
      file.name.toLowerCase().endsWith('.ts') ||
      file.name.toLowerCase().endsWith('.mts')
    if (!isValidType) {
      return 'ไม่รองรับประเภทไฟล์นี้ (รองรับ MP4, WebM, MOV, AVI, TS)'
    }
    if (file.size > maxSize) {
      return `ไฟล์ใหญ่เกินไป (สูงสุด ${maxSizeGB}GB)`
    }
    return null
  }

  const addFiles = (newFiles: FileList | File[]) => {
    const fileArray = Array.from(newFiles)
    const currentCount = files.length
    const availableSlots = MAX_FILES - currentCount

    if (availableSlots <= 0) {
      toast.error(`อัปโหลดได้สูงสุด ${MAX_FILES} ไฟล์`)
      return
    }

    const filesToAdd = fileArray.slice(0, availableSlots)
    const skipped = fileArray.length - filesToAdd.length

    const newFileItems: FileItem[] = filesToAdd
      .filter(file => {
        const error = validateFile(file)
        if (error) {
          toast.error(`${file.name}: ${error}`)
          return false
        }
        return true
      })
      .map(file => ({
        id: `${file.name}-${Date.now()}-${Math.random()}`,
        file,
        title: file.name.replace(/\.[^/.]+$/, ''), // ใช้ชื่อไฟล์เป็น default
      }))

    if (skipped > 0) {
      toast.warning(`ข้าม ${skipped} ไฟล์ (เกินจำนวนที่กำหนด)`)
    }

    setFiles(prev => [...prev, ...newFileItems])
  }

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setDragActive(false)

    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      addFiles(e.dataTransfer.files)
    }
  }, [files.length])

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      addFiles(e.target.files)
    }
    e.target.value = ''
  }

  const removeFile = (id: string) => {
    setFiles(prev => prev.filter(f => f.id !== id))
  }

  const updateFileTitle = (id: string, newTitle: string) => {
    setFiles(prev => prev.map(f =>
      f.id === id ? { ...f, title: newTitle } : f
    ))
  }

  // Batch Upload: เพิ่มทุกไฟล์เข้า upload queue แล้วปิด dialog ทันที
  const handleUpload = () => {
    if (files.length === 0) return

    // ตรวจสอบว่าทุกไฟล์มี title
    const emptyTitles = files.filter(f => !f.title.trim())
    if (emptyTitles.length > 0) {
      toast.error('กรุณาใส่ชื่อวิดีโอให้ครบทุกไฟล์')
      return
    }

    // เพิ่มแต่ละไฟล์เข้า upload store (จะ upload ใน background)
    files.forEach(fileItem => {
      addUpload({
        file: fileItem.file,
        title: fileItem.title.trim(),
        categoryId: categoryId || undefined, // ใช้หมวดหมู่เดียวกันทุกไฟล์
      })
    })

    toast.success(`เริ่มอัปโหลด ${files.length} ไฟล์ในพื้นหลัง`, {
      description: 'คุณสามารถดูความคืบหน้าได้ที่มุมขวาล่าง',
    })

    resetForm()
    onOpenChange(false)
  }

  const formatFileSize = (bytes: number) => {
    if (bytes < 1024 * 1024) {
      return `${(bytes / 1024).toFixed(1)} KB`
    }
    if (bytes < 1024 * 1024 * 1024) {
      return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
    }
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
  }

  const totalCount = files.length

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Files className="h-5 w-5" />
            อัปโหลดหลายไฟล์
          </DialogTitle>
          <DialogDescription>
            เลือกหลายไฟล์พร้อมกัน (สูงสุด {MAX_FILES} ไฟล์)
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {/* Category Selector - ใช้กับทุกไฟล์ */}
          <div className="space-y-2">
            <Label className="flex items-center gap-1.5">
              <FolderOpen className="size-4" />
              หมวดหมู่ (ทุกไฟล์)
            </Label>
            <Select value={categoryId} onValueChange={setCategoryId}>
              <SelectTrigger>
                <SelectValue placeholder="เลือกหมวดหมู่ (ไม่บังคับ)" />
              </SelectTrigger>
              <SelectContent>
                {categories?.map((cat) => (
                  <SelectItem key={cat.id} value={cat.id}>
                    {cat.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* File Drop Zone */}
          {files.length < MAX_FILES && (
            <div
              className={`relative border-2 border-dashed rounded-lg p-6 text-center transition-colors ${
                dragActive
                  ? 'border-primary bg-primary/5'
                  : 'border-muted-foreground/25 hover:border-muted-foreground/50'
              }`}
              onDragEnter={handleDrag}
              onDragLeave={handleDrag}
              onDragOver={handleDrag}
              onDrop={handleDrop}
            >
              <Upload className="size-8 mx-auto mb-2 text-muted-foreground" />
              <p className="font-medium mb-1">ลากไฟล์มาวางที่นี่</p>
              <p className="text-sm text-muted-foreground">
                หรือคลิกเพื่อเลือก (เลือกได้หลายไฟล์)
              </p>
              <input
                type="file"
                accept="video/*,.ts,.mts"
                multiple
                onChange={handleFileSelect}
                className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
              />
            </div>
          )}

          {/* File List with Title Inputs */}
          {files.length > 0 && (
            <div className="space-y-2">
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">
                  {files.length} ไฟล์
                </span>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={resetForm}
                  className="h-7 text-xs"
                >
                  ล้างทั้งหมด
                </Button>
              </div>

              <ScrollArea className="h-[280px] rounded-md border p-2">
                <div className="space-y-3">
                  {files.map((fileItem, index) => (
                    <div
                      key={fileItem.id}
                      className="p-3 rounded-md bg-muted/50 space-y-2"
                    >
                      {/* File Info Header */}
                      <div className="flex items-center gap-2">
                        <Film className="size-4 text-muted-foreground shrink-0" />
                        <div className="flex-1 min-w-0">
                          <p className="text-xs text-muted-foreground truncate" title={fileItem.file.name}>
                            {fileItem.file.name} ({formatFileSize(fileItem.file.size)})
                          </p>
                        </div>
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          onClick={() => removeFile(fileItem.id)}
                          className="size-6 shrink-0"
                        >
                          <X className="size-3" />
                        </Button>
                      </div>

                      {/* Title Input */}
                      <div className="flex items-center gap-2">
                        <span className="text-xs text-muted-foreground w-8 shrink-0">#{index + 1}</span>
                        <Input
                          value={fileItem.title}
                          onChange={(e) => updateFileTitle(fileItem.id, e.target.value)}
                          placeholder="ชื่อวิดีโอ"
                          className="h-8 text-sm"
                        />
                      </div>
                    </div>
                  ))}
                </div>
              </ScrollArea>
            </div>
          )}

          {/* Actions */}
          <div className="flex gap-3 pt-2">
            <Button
              type="button"
              variant="outline"
              onClick={handleClose}
              className="flex-1"
            >
              ยกเลิก
            </Button>
            {totalCount > 0 && (
              <Button
                type="button"
                onClick={handleUpload}
                className="flex-1"
              >
                <Upload className="size-4 mr-2" />
                อัปโหลด ({totalCount})
              </Button>
            )}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
