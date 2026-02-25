import type { GalleryImage } from '../types'

interface GalleryHoverPreviewProps {
  image: GalleryImage | null
}

export function GalleryHoverPreview({ image }: GalleryHoverPreviewProps) {
  return (
    <div className="w-[400px] flex-shrink-0 rounded-lg border bg-muted/30 flex items-center justify-center overflow-hidden">
      {image ? (
        <div className="w-full h-full flex flex-col">
          <img
            src={image.url}
            alt={image.filename}
            className="flex-1 min-h-0 object-contain"
          />
          <div className="text-center text-sm text-muted-foreground py-2 bg-background/80">
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
