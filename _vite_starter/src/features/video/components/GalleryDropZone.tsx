import { CheckCircle, FolderInput } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import type { GalleryImage, GalleryFolder } from '../types'

interface GalleryDropZoneProps {
  folder: GalleryFolder
  images: GalleryImage[]
  selectedImages: Set<string>
  onSelect: (filename: string) => void
  onHover: (image: GalleryImage | null) => void
  onDrop: (targetFolder: GalleryFolder, fromFolder: GalleryFolder, files: string[]) => void
  isDragOver: boolean
  onDragOver: () => void
  onDragLeave: () => void
  label: string
  badgeVariant: 'default' | 'secondary' | 'destructive' | 'outline'
  columns?: 2 | 3 | 4 | 5
}

const GRID_COLS = {
  2: 'grid-cols-2',
  3: 'grid-cols-3',
  4: 'grid-cols-4',
  5: 'grid-cols-5',
}

export function GalleryDropZone({
  folder,
  images,
  selectedImages,
  onSelect,
  onHover,
  onDrop,
  isDragOver,
  onDragOver,
  onDragLeave,
  label,
  badgeVariant,
  columns = 2,
}: GalleryDropZoneProps) {
  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault()

    const fromFolder = e.dataTransfer.getData('from-folder') as GalleryFolder
    const singleFile = e.dataTransfer.getData('text/plain')
    const selectedFiles = e.dataTransfer.getData('selected-images')

    // ถ้ามี selected files ใช้ทั้งหมด ไม่งั้นใช้ไฟล์เดียว
    const files = selectedFiles ? selectedFiles.split(',') : [singleFile]

    if (fromFolder && files.length > 0 && fromFolder !== folder) {
      onDrop(folder, fromFolder, files)
    }
  }

  return (
    <div
      className={cn(
        'flex-1 min-w-0 min-h-0 rounded-lg border-2 border-dashed transition-all p-3 flex flex-col overflow-hidden',
        isDragOver ? 'border-primary bg-primary/5' : 'border-muted',
      )}
      onDragOver={(e) => {
        e.preventDefault()
        onDragOver()
      }}
      onDragLeave={(e) => {
        e.preventDefault()
        onDragLeave()
      }}
      onDrop={handleDrop}
    >
      {/* Header */}
      <div className="flex items-center justify-between mb-2 flex-shrink-0">
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

      {/* Images Grid with inner scroll */}
      <div className="flex-1 overflow-y-auto min-h-0">
        <div className={cn('grid gap-1.5', GRID_COLS[columns])}>
          {images.map((img) => {
            const isSelected = selectedImages.has(img.filename)
            return (
              <div
                key={img.filename}
                draggable
                onDragStart={(e) => {
                  e.dataTransfer.setData('text/plain', img.filename)
                  e.dataTransfer.setData('from-folder', folder)
                  // ถ้า select ไว้หลายรูป และรูปนี้อยู่ใน selection ส่งทั้งหมด
                  if (isSelected && selectedImages.size > 1) {
                    e.dataTransfer.setData('selected-images', Array.from(selectedImages).join(','))
                  }
                }}
                onClick={() => onSelect(img.filename)}
                onMouseEnter={() => onHover(img)}
                onMouseLeave={() => onHover(null)}
                className={cn(
                  'aspect-video rounded overflow-hidden cursor-grab relative border-2 transition-all active:cursor-grabbing',
                  isSelected ? 'border-primary ring-2 ring-primary/30' : 'border-transparent hover:border-muted-foreground/50',
                )}
              >
                <img
                  src={img.url}
                  alt={img.filename}
                  className="w-full h-full object-cover pointer-events-none"
                  loading="lazy"
                  draggable={false}
                />
                {/* Selection overlay */}
                {isSelected && (
                  <div className="absolute inset-0 bg-primary/20 flex items-center justify-center pointer-events-none">
                    <CheckCircle className="size-5 text-primary drop-shadow" />
                  </div>
                )}
              </div>
            )
          })}
        </div>

        {/* Empty state */}
        {images.length === 0 && (
          <div className="flex items-center justify-center h-32 text-muted-foreground text-sm">
            {isDragOver ? 'ปล่อยเพื่อย้ายภาพมาที่นี่' : 'ไม่มีภาพ'}
          </div>
        )}
      </div>
    </div>
  )
}
