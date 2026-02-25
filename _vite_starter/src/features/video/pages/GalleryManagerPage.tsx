import { useState, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Loader2, CheckCircle, AlertCircle, Send, FolderInput, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { useVideo, useGalleryImages, useMoveBatch, usePublishGallery } from '../hooks'
import type { GalleryImage, GalleryFolder } from '../types'

// Drop zone component
function DropZone({
  folder,
  images,
  selectedImages,
  onSelect,
  onDrop,
  isDragOver,
  onDragOver,
  onDragLeave,
  label,
  badgeVariant,
}: {
  folder: GalleryFolder
  images: GalleryImage[]
  selectedImages: Set<string>
  onSelect: (filename: string) => void
  onDrop: (folder: GalleryFolder) => void
  isDragOver: boolean
  onDragOver: () => void
  onDragLeave: () => void
  label: string
  badgeVariant: 'default' | 'secondary' | 'destructive' | 'outline'
}) {
  return (
    <div
      className={cn(
        'flex-1 min-w-0 rounded-lg border-2 border-dashed transition-all p-4',
        isDragOver ? 'border-primary bg-primary/5 scale-[1.02]' : 'border-muted',
      )}
      onDragOver={(e) => {
        e.preventDefault()
        onDragOver()
      }}
      onDragLeave={(e) => {
        e.preventDefault()
        onDragLeave()
      }}
      onDrop={(e) => {
        e.preventDefault()
        onDrop(folder)
      }}
    >
      {/* Header */}
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <Badge variant={badgeVariant}>{label}</Badge>
          <span className="text-sm text-muted-foreground">({images.length})</span>
        </div>
        {isDragOver && (
          <div className="flex items-center gap-1 text-primary text-sm animate-pulse">
            <FolderInput className="size-4" />
            วางที่นี่
          </div>
        )}
      </div>

      {/* Images Grid */}
      <div className="grid grid-cols-4 sm:grid-cols-5 md:grid-cols-6 lg:grid-cols-8 gap-2 min-h-[200px]">
        {images.map((img) => {
          const isSelected = selectedImages.has(img.filename)
          return (
            <div
              key={img.filename}
              draggable
              onDragStart={(e) => {
                e.dataTransfer.setData('text/plain', img.filename)
                e.dataTransfer.setData('from-folder', folder)
                // ถ้ายัง select ไว้หลายรูป ส่งทั้งหมด
                if (isSelected && selectedImages.size > 1) {
                  e.dataTransfer.setData('selected-images', Array.from(selectedImages).join(','))
                }
              }}
              onClick={() => onSelect(img.filename)}
              className={cn(
                'aspect-square rounded-md overflow-hidden cursor-pointer relative group border-2 transition-all',
                isSelected ? 'border-primary ring-2 ring-primary/30' : 'border-transparent hover:border-muted-foreground/30',
              )}
            >
              <img
                src={img.url}
                alt={img.filename}
                className="w-full h-full object-cover"
                loading="lazy"
              />
              {/* Selection overlay */}
              {isSelected && (
                <div className="absolute inset-0 bg-primary/20 flex items-center justify-center">
                  <CheckCircle className="size-6 text-primary drop-shadow" />
                </div>
              )}
              {/* Filename tooltip */}
              <div className="absolute bottom-0 left-0 right-0 bg-black/60 text-white text-xs px-1 py-0.5 opacity-0 group-hover:opacity-100 transition truncate">
                {img.filename}
              </div>
            </div>
          )
        })}

        {/* Empty state */}
        {images.length === 0 && (
          <div className="col-span-full flex items-center justify-center h-[200px] text-muted-foreground text-sm">
            {isDragOver ? 'ปล่อยเพื่อย้ายภาพมาที่นี่' : 'ไม่มีภาพ'}
          </div>
        )}
      </div>
    </div>
  )
}

export function GalleryManagerPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()

  // Fetch data
  const { data: video, isLoading: videoLoading } = useVideo(id ?? '')
  const { data: gallery, isLoading: galleryLoading, refetch } = useGalleryImages(id ?? '', {
    enabled: !!id,
  })

  // Mutations
  const moveBatch = useMoveBatch()
  const publishGallery = usePublishGallery()

  // State
  const [selectedImages, setSelectedImages] = useState<Set<string>>(new Set())
  const [selectedFolder, setSelectedFolder] = useState<GalleryFolder | null>(null)
  const [dragOverFolder, setDragOverFolder] = useState<GalleryFolder | null>(null)

  // Toggle image selection
  const toggleSelect = useCallback((filename: string, folder: GalleryFolder) => {
    setSelectedImages((prev) => {
      const next = new Set(prev)
      if (next.has(filename)) {
        next.delete(filename)
        if (next.size === 0) setSelectedFolder(null)
      } else {
        // ถ้าเลือก folder อื่น → clear selection แล้วเลือกใหม่
        if (selectedFolder && selectedFolder !== folder) {
          next.clear()
        }
        next.add(filename)
        setSelectedFolder(folder)
      }
      return next
    })
  }, [selectedFolder])

  // Handle drop
  const handleDrop = useCallback(async (targetFolder: GalleryFolder) => {
    setDragOverFolder(null)

    if (!id || selectedImages.size === 0 || !selectedFolder) return
    if (selectedFolder === targetFolder) return // Same folder

    const files = Array.from(selectedImages)

    try {
      await moveBatch.mutateAsync({
        videoId: id,
        data: { files, from: selectedFolder, to: targetFolder },
      })

      toast.success(`ย้าย ${files.length} ภาพไป ${targetFolder}`)
      setSelectedImages(new Set())
      setSelectedFolder(null)
      refetch()
    } catch {
      toast.error('ย้ายภาพไม่สำเร็จ')
    }
  }, [id, selectedImages, selectedFolder, moveBatch, refetch])

  // Quick move buttons
  const handleQuickMove = async (targetFolder: GalleryFolder) => {
    if (!id || selectedImages.size === 0 || !selectedFolder) return
    if (selectedFolder === targetFolder) return

    const files = Array.from(selectedImages)

    try {
      await moveBatch.mutateAsync({
        videoId: id,
        data: { files, from: selectedFolder, to: targetFolder },
      })

      toast.success(`ย้าย ${files.length} ภาพไป ${targetFolder}`)
      setSelectedImages(new Set())
      setSelectedFolder(null)
      refetch()
    } catch {
      toast.error('ย้ายภาพไม่สำเร็จ')
    }
  }

  // Publish gallery
  const handlePublish = async () => {
    if (!id) return

    try {
      const result = await publishGallery.mutateAsync(id)
      toast.success(`Publish สำเร็จ! Safe: ${result.safeCount}, NSFW: ${result.nsfwCount}`)
      refetch()
    } catch {
      toast.error('Publish ไม่สำเร็จ')
    }
  }

  // Clear selection
  const clearSelection = () => {
    setSelectedImages(new Set())
    setSelectedFolder(null)
  }

  // Loading
  if (videoLoading || galleryLoading) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <Loader2 className="size-8 animate-spin text-muted-foreground" />
      </div>
    )
  }

  // Not found
  if (!video || !gallery) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[60vh] gap-4">
        <AlertCircle className="size-12 text-muted-foreground" />
        <p className="text-muted-foreground">ไม่พบข้อมูล Gallery</p>
        <Button variant="outline" onClick={() => navigate(-1)}>
          กลับ
        </Button>
      </div>
    )
  }

  const sourceImages = gallery.source ?? []
  const safeImages = gallery.safe ?? []
  const nsfwImages = gallery.nsfw ?? []
  const canPublish = safeImages.length > 0 || nsfwImages.length > 0

  return (
    <div className="container py-6 max-w-7xl mx-auto">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-4">
          <Button variant="ghost" size="icon" onClick={() => navigate(-1)}>
            <ArrowLeft className="size-5" />
          </Button>
          <div>
            <h1 className="text-xl font-semibold">Gallery Manager</h1>
            <p className="text-sm text-muted-foreground">
              {video.code} - {video.title}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          {/* Status Badge */}
          <Badge
            variant={
              gallery.status === 'ready' ? 'default' :
              gallery.status === 'pending_review' ? 'secondary' :
              'outline'
            }
          >
            {gallery.status === 'ready' ? 'Published' :
             gallery.status === 'pending_review' ? 'รอตรวจสอบ' :
             gallery.status === 'processing' ? 'กำลังสร้าง' :
             'ยังไม่มี'}
          </Badge>

          {/* Publish Button */}
          <Button
            onClick={handlePublish}
            disabled={!canPublish || publishGallery.isPending}
          >
            {publishGallery.isPending ? (
              <Loader2 className="size-4 animate-spin mr-1.5" />
            ) : (
              <Send className="size-4 mr-1.5" />
            )}
            Publish
          </Button>
        </div>
      </div>

      {/* Selection Toolbar */}
      {selectedImages.size > 0 && (
        <div className="bg-muted rounded-lg p-3 mb-4 flex items-center justify-between">
          <span className="text-sm">
            เลือก <strong>{selectedImages.size}</strong> ภาพ จาก <Badge variant="outline">{selectedFolder}</Badge>
          </span>
          <div className="flex items-center gap-2">
            {/* Quick Move Buttons */}
            {selectedFolder !== 'safe' && (
              <Button
                size="sm"
                variant="outline"
                onClick={() => handleQuickMove('safe')}
                disabled={moveBatch.isPending}
              >
                <FolderInput className="size-4 mr-1" />
                Safe
              </Button>
            )}
            {selectedFolder !== 'nsfw' && (
              <Button
                size="sm"
                variant="outline"
                onClick={() => handleQuickMove('nsfw')}
                disabled={moveBatch.isPending}
              >
                <FolderInput className="size-4 mr-1" />
                NSFW
              </Button>
            )}
            {selectedFolder !== 'source' && (
              <Button
                size="sm"
                variant="ghost"
                onClick={() => handleQuickMove('source')}
                disabled={moveBatch.isPending}
              >
                <Trash2 className="size-4 mr-1" />
                คืน Source
              </Button>
            )}
            <Button size="sm" variant="ghost" onClick={clearSelection}>
              ยกเลิก
            </Button>
          </div>
        </div>
      )}

      {/* Instructions */}
      <p className="text-sm text-muted-foreground mb-4">
        คลิกเพื่อเลือก แล้ว drag ไปวางใน folder ที่ต้องการ หรือใช้ปุ่มย้ายด้านบน
      </p>

      {/* Drop Zones */}
      <div className="flex flex-col lg:flex-row gap-4">
        {/* Source */}
        <DropZone
          folder="source"
          images={sourceImages}
          selectedImages={selectedFolder === 'source' ? selectedImages : new Set()}
          onSelect={(f) => toggleSelect(f, 'source')}
          onDrop={handleDrop}
          isDragOver={dragOverFolder === 'source'}
          onDragOver={() => setDragOverFolder('source')}
          onDragLeave={() => setDragOverFolder(null)}
          label="Source"
          badgeVariant="outline"
        />

        {/* Safe */}
        <DropZone
          folder="safe"
          images={safeImages}
          selectedImages={selectedFolder === 'safe' ? selectedImages : new Set()}
          onSelect={(f) => toggleSelect(f, 'safe')}
          onDrop={handleDrop}
          isDragOver={dragOverFolder === 'safe'}
          onDragOver={() => setDragOverFolder('safe')}
          onDragLeave={() => setDragOverFolder(null)}
          label="Safe (Public)"
          badgeVariant="default"
        />

        {/* NSFW */}
        <DropZone
          folder="nsfw"
          images={nsfwImages}
          selectedImages={selectedFolder === 'nsfw' ? selectedImages : new Set()}
          onSelect={(f) => toggleSelect(f, 'nsfw')}
          onDrop={handleDrop}
          isDragOver={dragOverFolder === 'nsfw'}
          onDragOver={() => setDragOverFolder('nsfw')}
          onDragLeave={() => setDragOverFolder(null)}
          label="NSFW (Members)"
          badgeVariant="destructive"
        />
      </div>
    </div>
  )
}
