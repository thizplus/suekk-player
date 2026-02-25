import type { GalleryImage } from '../types'

interface GalleryHoverPreviewProps {
  image: GalleryImage | null
}

export function GalleryHoverPreview({ image }: GalleryHoverPreviewProps) {
  return (
    <div className="h-[280px] flex-shrink-0 rounded-lg border bg-muted/30 flex items-center justify-center overflow-hidden">
      {image ? (
        <div className="h-full flex items-center gap-4 px-4">
          <img
            src={image.url}
            alt={image.filename}
            className="h-full max-w-[500px] object-contain"
          />
          <div className="text-sm text-muted-foreground">
            {image.filename}
          </div>
        </div>
      ) : (
        <div className="text-muted-foreground text-sm">
          วางเมาส์บนภาพเพื่อดูขยาย
        </div>
      )}
    </div>
  )
}
