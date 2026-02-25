import { useState, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Loader2, AlertCircle, Send, FolderInput, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { toast } from 'sonner'
import { useVideo, useGalleryImages, useMoveBatch, usePublishGallery } from '../hooks'
import { GalleryDropZone } from '../components/GalleryDropZone'
import { GalleryHoverPreview } from '../components/GalleryHoverPreview'
import type { GalleryImage, GalleryFolder } from '../types'

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
  const [hoveredImage, setHoveredImage] = useState<GalleryImage | null>(null)

  // Toggle image selection
  const toggleSelect = useCallback((filename: string, folder: GalleryFolder) => {
    setSelectedImages((prev) => {
      const next = new Set(prev)
      if (next.has(filename)) {
        next.delete(filename)
        if (next.size === 0) setSelectedFolder(null)
      } else {
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
    <div className="h-screen flex flex-col p-4 max-w-[1800px] mx-auto">
      {/* Header */}
      <div className="flex items-center justify-between mb-4 flex-shrink-0">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" onClick={() => navigate(-1)}>
            <ArrowLeft className="size-5" />
          </Button>
          <div>
            <h1 className="text-lg font-semibold">Gallery: {video.code}</h1>
            <p className="text-xs text-muted-foreground truncate max-w-[200px]">{video.title}</p>
          </div>
        </div>

        <div className="flex items-center gap-2">
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

          <Button
            onClick={handlePublish}
            disabled={!canPublish || publishGallery.isPending}
            size="sm"
          >
            {publishGallery.isPending ? (
              <Loader2 className="size-4 animate-spin mr-1" />
            ) : (
              <Send className="size-4 mr-1" />
            )}
            Publish
          </Button>
        </div>
      </div>

      {/* Selection Toolbar */}
      {selectedImages.size > 0 && (
        <div className="bg-muted rounded-lg p-2 mb-3 flex items-center justify-between flex-shrink-0">
          <span className="text-sm">
            เลือก <strong>{selectedImages.size}</strong> ภาพ จาก <Badge variant="outline" className="ml-1">{selectedFolder}</Badge>
          </span>
          <div className="flex items-center gap-1">
            {selectedFolder !== 'safe' && (
              <Button size="sm" variant="outline" onClick={() => handleQuickMove('safe')} disabled={moveBatch.isPending}>
                <FolderInput className="size-3 mr-1" /> Safe
              </Button>
            )}
            {selectedFolder !== 'nsfw' && (
              <Button size="sm" variant="outline" onClick={() => handleQuickMove('nsfw')} disabled={moveBatch.isPending}>
                <FolderInput className="size-3 mr-1" /> NSFW
              </Button>
            )}
            {selectedFolder !== 'source' && (
              <Button size="sm" variant="ghost" onClick={() => handleQuickMove('source')} disabled={moveBatch.isPending}>
                <Trash2 className="size-3 mr-1" /> คืน
              </Button>
            )}
            <Button size="sm" variant="ghost" onClick={clearSelection}>ยกเลิก</Button>
          </div>
        </div>
      )}

      {/* Main content area */}
      <div className="flex-1 flex gap-3 min-h-0">
        {/* Left: Preview + Source */}
        <div className="flex-1 flex flex-col gap-3 min-w-0">
          {/* Top: Hover Preview */}
          <GalleryHoverPreview image={hoveredImage} />

          {/* Bottom: Source */}
          <GalleryDropZone
            folder="source"
            images={sourceImages}
            selectedImages={selectedFolder === 'source' ? selectedImages : new Set()}
            onSelect={(f) => toggleSelect(f, 'source')}
            onHover={setHoveredImage}
            onDrop={handleDrop}
            isDragOver={dragOverFolder === 'source'}
            onDragOver={() => setDragOverFolder('source')}
            onDragLeave={() => setDragOverFolder(null)}
            label="Source"
            badgeVariant="outline"
            columns={3}
          />
        </div>

        {/* Right: Safe & NSFW */}
        <div className="flex-1 flex flex-col gap-3 min-w-0">
          <GalleryDropZone
            folder="safe"
            images={safeImages}
            selectedImages={selectedFolder === 'safe' ? selectedImages : new Set()}
            onSelect={(f) => toggleSelect(f, 'safe')}
            onHover={setHoveredImage}
            onDrop={handleDrop}
            isDragOver={dragOverFolder === 'safe'}
            onDragOver={() => setDragOverFolder('safe')}
            onDragLeave={() => setDragOverFolder(null)}
            label="Safe"
            badgeVariant="default"
            columns={5}
          />

          <GalleryDropZone
            folder="nsfw"
            images={nsfwImages}
            selectedImages={selectedFolder === 'nsfw' ? selectedImages : new Set()}
            onSelect={(f) => toggleSelect(f, 'nsfw')}
            onHover={setHoveredImage}
            onDrop={handleDrop}
            isDragOver={dragOverFolder === 'nsfw'}
            onDragOver={() => setDragOverFolder('nsfw')}
            onDragLeave={() => setDragOverFolder(null)}
            label="NSFW"
            badgeVariant="destructive"
            columns={5}
          />
        </div>
      </div>
    </div>
  )
}
