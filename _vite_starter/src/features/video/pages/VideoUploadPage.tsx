import { useState, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { Upload, X, Film, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useUploadVideo } from '../hooks'
import { useCategories } from '@/features/category/hooks'
import { toast } from 'sonner'

const ALLOWED_TYPES = ['video/mp4', 'video/webm', 'video/quicktime', 'video/x-msvideo']
const MAX_SIZE = 2 * 1024 * 1024 * 1024 // 2GB

export function VideoUploadPage() {
  const navigate = useNavigate()
  const uploadVideo = useUploadVideo()
  const { data: categories } = useCategories()

  const [file, setFile] = useState<File | null>(null)
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [categoryId, setCategoryId] = useState('')
  const [dragActive, setDragActive] = useState(false)

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
    if (!ALLOWED_TYPES.includes(file.type)) {
      return 'รองรับเฉพาะไฟล์วิดีโอ (MP4, WebM, MOV, AVI)'
    }
    if (file.size > MAX_SIZE) {
      return 'ไฟล์ใหญ่เกินไป (สูงสุด 2GB)'
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

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    if (!file) {
      toast.error('กรุณาเลือกไฟล์วิดีโอ')
      return
    }

    if (!title.trim()) {
      toast.error('กรุณากรอกชื่อวิดีโอ')
      return
    }

    try {
      await uploadVideo.mutateAsync({
        file,
        title: title.trim(),
        description: description.trim() || undefined,
        categoryId: categoryId || undefined,
      })
      toast.success('อัปโหลดสำเร็จ')
      navigate('/videos')
    } catch {
      toast.error('อัปโหลดล้มเหลว กรุณาลองใหม่')
    }
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
    <div className="max-w-2xl mx-auto">
      <Card>
        <CardHeader>
          <CardTitle>อัปโหลดวิดีโอ</CardTitle>
          <CardDescription>
            อัปโหลดไฟล์วิดีโอเพื่อแปลงเป็น HLS สำหรับสตรีมมิ่ง
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-6">
            {/* File Drop Zone */}
            <div
              className={`relative border-2 border-dashed rounded-lg p-8 text-center transition-colors ${
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
                <div className="flex items-center justify-center gap-4">
                  <Film className="size-12 text-muted-foreground" />
                  <div className="text-left">
                    <p className="font-medium">{file.name}</p>
                    <p className="text-sm text-muted-foreground">
                      {formatFileSize(file.size)}
                    </p>
                  </div>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    onClick={() => setFile(null)}
                  >
                    <X className="size-4" />
                  </Button>
                </div>
              ) : (
                <>
                  <Upload className="size-12 mx-auto mb-4 text-muted-foreground" />
                  <p className="text-lg font-medium mb-1">
                    ลากไฟล์มาวางที่นี่
                  </p>
                  <p className="text-sm text-muted-foreground mb-4">
                    หรือคลิกเพื่อเลือกไฟล์
                  </p>
                  <p className="text-sm text-muted-foreground">
                    รองรับ: MP4, WebM, MOV, AVI (สูงสุด 2GB)
                  </p>
                </>
              )}
              <input
                type="file"
                accept="video/*"
                onChange={handleFileSelect}
                className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
              />
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
                className="flex min-h-[100px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
              />
            </div>

            {/* Category */}
            <div className="space-y-2">
              <Label htmlFor="category">หมวดหมู่</Label>
              <select
                id="category"
                value={categoryId}
                onChange={(e) => setCategoryId(e.target.value)}
                className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
              >
                <option value="">-- เลือกหมวดหมู่ --</option>
                {categories?.map((cat) => (
                  <option key={cat.id} value={cat.id}>
                    {cat.name}
                  </option>
                ))}
              </select>
            </div>

            {/* Submit */}
            <div className="flex gap-4">
              <Button
                type="button"
                variant="outline"
                onClick={() => navigate('/videos')}
              >
                ยกเลิก
              </Button>
              <Button
                type="submit"
                disabled={uploadVideo.isPending || !file}
                className="flex-1"
              >
                {uploadVideo.isPending ? (
                  <>
                    <Loader2 className="size-4 mr-2 animate-spin" />
                    กำลังอัปโหลด...
                  </>
                ) : (
                  <>
                    <Upload className="size-4 mr-2" />
                    อัปโหลด
                  </>
                )}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
