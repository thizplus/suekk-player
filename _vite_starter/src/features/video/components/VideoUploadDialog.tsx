import { useState, useCallback } from 'react'
import { Upload, X, Film } from 'lucide-react'
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
import { useCategories } from '@/features/category/hooks'
import { useUploadLimits } from '../hooks'
import { useUploadStore } from '@/stores/upload-store'
import { toast } from 'sonner'

// Fallback values ถ้าดึงจาก API ไม่ได้
const DEFAULT_ALLOWED_TYPES = ['video/mp4', 'video/webm', 'video/quicktime', 'video/x-msvideo', 'video/x-matroska', 'video/MP2T', 'video/mp2t', 'video/vnd.dlna.mpeg-tts']
const DEFAULT_MAX_SIZE = 10 * 1024 * 1024 * 1024 // 10GB

interface VideoUploadDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function VideoUploadDialog({ open, onOpenChange }: VideoUploadDialogProps) {
  const { addUpload } = useUploadStore()
  const { data: categories } = useCategories()
  const { data: uploadLimits } = useUploadLimits()

  // ใช้ค่าจาก API หรือ fallback
  const maxSize = uploadLimits?.max_file_size ?? DEFAULT_MAX_SIZE
  const maxSizeGB = uploadLimits?.max_file_size_gb ?? 10
  const allowedTypes = uploadLimits?.allowed_types ?? DEFAULT_ALLOWED_TYPES

  const [file, setFile] = useState<File | null>(null)
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [categoryId, setCategoryId] = useState('')
  const [dragActive, setDragActive] = useState(false)

  const resetForm = () => {
    setFile(null)
    setTitle('')
    setDescription('')
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
      return 'รองรับเฉพาะไฟล์วิดีโอ (MP4, WebM, MOV, AVI, TS)'
    }
    if (file.size > maxSize) {
      return `ไฟล์ใหญ่เกินไป (สูงสุด ${maxSizeGB}GB)`
    }
    return null
  }

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setDragActive(false)

    if (e.dataTransfer.files && e.dataTransfer.files[0]) {
      const droppedFile = e.dataTransfer.files[0]
      const error = validateFile(droppedFile)
      if (error) {
        toast.error(error)
        return
      }
      setFile(droppedFile)
      if (!title) {
        setTitle(droppedFile.name.replace(/\.[^/.]+$/, ''))
      }
    }
  }, [title])

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files[0]) {
      const selectedFile = e.target.files[0]
      const error = validateFile(selectedFile)
      if (error) {
        toast.error(error)
        return
      }
      setFile(selectedFile)
      if (!title) {
        setTitle(selectedFile.name.replace(/\.[^/.]+$/, ''))
      }
    }
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()

    if (!file) {
      toast.error('กรุณาเลือกไฟล์วิดีโอ')
      return
    }

    if (!title.trim()) {
      toast.error('กรุณากรอกชื่อวิดีโอ')
      return
    }

    // เพิ่มเข้า upload queue และปิด dialog ทันที
    addUpload({
      file,
      title: title.trim(),
      description: description.trim() || undefined,
      categoryId: categoryId || undefined,
    })

    toast.success('เริ่มอัปโหลดในพื้นหลัง', {
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

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>อัปโหลดวิดีโอ</DialogTitle>
          <DialogDescription>
            อัปโหลดไฟล์วิดีโอเพื่อแปลงเป็น HLS สำหรับสตรีมมิ่ง
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4 overflow-hidden">
          {/* File Drop Zone */}
          <div
            className={`relative border-2 border-dashed rounded-lg p-6 text-center transition-colors overflow-hidden ${
              dragActive
                ? 'border-primary bg-primary/5'
                : 'border-muted-foreground/25 hover:border-muted-foreground/50'
            }`}
            onDragEnter={handleDrag}
            onDragLeave={handleDrag}
            onDragOver={handleDrag}
            onDrop={handleDrop}
          >
            {file ? (
              <div className="flex items-center gap-3">
                <Film className="size-10 text-muted-foreground shrink-0" />
                <div className="text-left min-w-0 flex-1">
                  <p className="font-medium truncate max-w-[280px] sm:max-w-[320px]" title={file.name}>
                    {file.name}
                  </p>
                  <p className="text-sm text-muted-foreground">
                    {formatFileSize(file.size)}
                  </p>
                </div>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  onClick={() => setFile(null)}
                  className="shrink-0"
                >
                  <X className="size-4" />
                </Button>
              </div>
            ) : (
              <>
                <Upload className="size-10 mx-auto mb-3 text-muted-foreground" />
                <p className="font-medium mb-1">ลากไฟล์มาวางที่นี่</p>
                <p className="text-sm text-muted-foreground mb-2">
                  หรือคลิกเพื่อเลือกไฟล์
                </p>
                <p className="text-sm text-muted-foreground">
                  รองรับ: MP4, WebM, MOV, AVI, TS (สูงสุด {maxSizeGB}GB)
                </p>
                <input
                  type="file"
                  accept="video/*,.ts,.mts"
                  onChange={handleFileSelect}
                  className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
                />
              </>
            )}
          </div>

          {/* Title */}
          <div className="space-y-2">
            <Label htmlFor="title">ชื่อวิดีโอ *</Label>
            <Input
              id="title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="กรอกชื่อวิดีโอ"
              required
            />
          </div>

          {/* Description */}
          <div className="space-y-2">
            <Label htmlFor="description">คำอธิบาย</Label>
            <textarea
              id="description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="กรอกคำอธิบายวิดีโอ (ไม่บังคับ)"
              rows={3}
              className="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 resize-none"
            />
          </div>

          {/* Category */}
          <div className="space-y-2">
            <Label htmlFor="category">หมวดหมู่</Label>
            <Select value={categoryId} onValueChange={setCategoryId}>
              <SelectTrigger>
                <SelectValue placeholder="เลือกหมวดหมู่" />
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
            <Button
              type="submit"
              disabled={!file}
              className="flex-1"
            >
              <Upload className="size-4 mr-2" />
              อัปโหลด
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
