import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Loader2, X, ChevronLeft, ChevronRight, ImageOff } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useVideoByCode, useGalleryUrls } from '../hooks'

export function GalleryPage() {
  const { code } = useParams<{ code: string }>()
  const navigate = useNavigate()

  // Fetch video data
  const { data: video, isLoading: videoLoading, error: videoError } = useVideoByCode(code ?? '')

  // Fetch presigned URLs for all gallery images (single API call)
  const { data: galleryData, isLoading: galleryLoading } = useGalleryUrls(code ?? '', {
    enabled: !!code && !!video && video.status === 'ready' && (video.galleryCount ?? 0) > 0,
  })

  // State
  const [lightboxIndex, setLightboxIndex] = useState<number | null>(null)
  const [loadedImages, setLoadedImages] = useState<Set<number>>(new Set())
  const [failedImages, setFailedImages] = useState<Set<number>>(new Set())

  const galleryCount = galleryData?.count ?? video?.galleryCount ?? 0
  const imageUrls = galleryData?.urls ?? []

  // Lightbox navigation
  const openLightbox = (index: number) => setLightboxIndex(index)
  const closeLightbox = () => setLightboxIndex(null)

  const goToPrev = useCallback(() => {
    if (lightboxIndex === null) return
    setLightboxIndex(lightboxIndex > 0 ? lightboxIndex - 1 : galleryCount - 1)
  }, [lightboxIndex, galleryCount])

  const goToNext = useCallback(() => {
    if (lightboxIndex === null) return
    setLightboxIndex(lightboxIndex < galleryCount - 1 ? lightboxIndex + 1 : 0)
  }, [lightboxIndex, galleryCount])

  // Keyboard navigation for lightbox
  useEffect(() => {
    if (lightboxIndex === null) return

    const handleKeyDown = (e: KeyboardEvent) => {
      switch (e.key) {
        case 'Escape':
          closeLightbox()
          break
        case 'ArrowLeft':
          goToPrev()
          break
        case 'ArrowRight':
          goToNext()
          break
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [lightboxIndex, goToPrev, goToNext])

  // Loading state
  if (videoLoading || galleryLoading) {
    return (
      <div className="fixed inset-0 bg-background flex items-center justify-center">
        <Loader2 className="size-8 animate-spin text-muted-foreground" />
      </div>
    )
  }

  // Error state
  if (videoError || !video) {
    return (
      <div className="fixed inset-0 bg-background flex flex-col items-center justify-center gap-4">
        <ImageOff className="size-12 text-muted-foreground" />
        <p className="text-muted-foreground">ไม่พบวิดีโอ</p>
        <Button variant="outline" onClick={() => navigate(-1)}>
          <ArrowLeft className="size-4 mr-1.5" />
          กลับ
        </Button>
      </div>
    )
  }

  // No gallery state
  if (!galleryCount || galleryCount === 0 || imageUrls.length === 0) {
    return (
      <div className="fixed inset-0 bg-background flex flex-col items-center justify-center gap-4">
        <ImageOff className="size-12 text-muted-foreground" />
        <p className="text-muted-foreground">วิดีโอนี้ไม่มี Gallery</p>
        <Button variant="outline" onClick={() => navigate(-1)}>
          <ArrowLeft className="size-4 mr-1.5" />
          กลับ
        </Button>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <header className="sticky top-0 z-10 bg-background/95 backdrop-blur border-b">
        <div className="max-w-7xl mx-auto px-4 py-3 flex items-center gap-3">
          <Button variant="ghost" size="icon" onClick={() => navigate(-1)}>
            <ArrowLeft className="size-5" />
          </Button>
          <div className="flex-1 min-w-0">
            <h1 className="font-semibold truncate">Gallery: {video.code}</h1>
            <p className="text-sm text-muted-foreground">{galleryCount} ภาพ</p>
          </div>
        </div>
      </header>

      {/* Gallery Grid */}
      <main className="max-w-7xl mx-auto px-4 py-6">
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-2 sm:gap-3">
          {imageUrls.map((url, index) => (
            <GalleryImage
              key={index}
              index={index}
              url={url}
              loaded={loadedImages.has(index)}
              failed={failedImages.has(index)}
              onLoad={() => setLoadedImages(prev => new Set(prev).add(index))}
              onError={() => setFailedImages(prev => new Set(prev).add(index))}
              onClick={() => openLightbox(index)}
            />
          ))}
        </div>
      </main>

      {/* Lightbox */}
      {lightboxIndex !== null && imageUrls[lightboxIndex] && (
        <Lightbox
          url={imageUrls[lightboxIndex]}
          index={lightboxIndex}
          total={galleryCount}
          onClose={closeLightbox}
          onPrev={goToPrev}
          onNext={goToNext}
        />
      )}
    </div>
  )
}

// Gallery Image Component with lazy loading (presigned URL - no headers needed)
interface GalleryImageProps {
  index: number
  url: string
  loaded: boolean
  failed: boolean
  onLoad: () => void
  onError: () => void
  onClick: () => void
}

function GalleryImage({ index, url, loaded, failed, onLoad, onError, onClick }: GalleryImageProps) {
  const [isVisible, setIsVisible] = useState(false)

  // Intersection Observer for lazy loading
  useEffect(() => {
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) {
          setIsVisible(true)
          observer.disconnect()
        }
      },
      { rootMargin: '100px' }
    )

    const element = document.getElementById(`gallery-img-${index}`)
    if (element) observer.observe(element)

    return () => observer.disconnect()
  }, [index])

  return (
    <div
      id={`gallery-img-${index}`}
      className="aspect-video bg-muted rounded-lg overflow-hidden cursor-pointer hover:ring-2 hover:ring-primary transition-all"
      onClick={onClick}
    >
      {isVisible && !failed ? (
        <img
          src={url}
          alt={`Gallery ${index + 1}`}
          className={`w-full h-full object-cover ${loaded ? '' : 'opacity-0'}`}
          loading="lazy"
          onLoad={onLoad}
          onError={onError}
        />
      ) : null}

      {/* Loading state */}
      {isVisible && !loaded && !failed && (
        <div className="absolute inset-0 flex items-center justify-center">
          <Loader2 className="size-5 animate-spin text-muted-foreground/50" />
        </div>
      )}

      {/* Failed state */}
      {failed && (
        <div className="w-full h-full flex items-center justify-center">
          <ImageOff className="size-6 text-muted-foreground/50" />
        </div>
      )}

      {/* Placeholder before visible */}
      {!isVisible && (
        <div className="w-full h-full flex items-center justify-center">
          <div className="size-4 rounded-full bg-muted-foreground/20" />
        </div>
      )}
    </div>
  )
}

// Lightbox Component (presigned URL - no headers needed)
interface LightboxProps {
  url: string
  index: number
  total: number
  onClose: () => void
  onPrev: () => void
  onNext: () => void
}

function Lightbox({ url, index, total, onClose, onPrev, onNext }: LightboxProps) {
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)

  // Reset state when URL changes
  useEffect(() => {
    setLoading(true)
    setError(false)
  }, [url])

  return (
    <div
      className="fixed inset-0 z-50 bg-black/95 flex items-center justify-center"
      onClick={onClose}
    >
      {/* Close button */}
      <button
        className="absolute top-4 right-4 z-10 p-2 rounded-full bg-white/10 hover:bg-white/20 transition-colors"
        onClick={onClose}
      >
        <X className="size-6 text-white" />
      </button>

      {/* Image counter */}
      <div className="absolute top-4 left-4 px-3 py-1.5 rounded bg-white/10 text-white text-sm">
        {index + 1} / {total}
      </div>

      {/* Navigation buttons */}
      <button
        className="absolute left-4 top-1/2 -translate-y-1/2 p-3 rounded-full bg-white/10 hover:bg-white/20 transition-colors"
        onClick={(e) => {
          e.stopPropagation()
          onPrev()
        }}
      >
        <ChevronLeft className="size-8 text-white" />
      </button>

      <button
        className="absolute right-4 top-1/2 -translate-y-1/2 p-3 rounded-full bg-white/10 hover:bg-white/20 transition-colors"
        onClick={(e) => {
          e.stopPropagation()
          onNext()
        }}
      >
        <ChevronRight className="size-8 text-white" />
      </button>

      {/* Image */}
      <div
        className="max-w-[90vw] max-h-[90vh] flex items-center justify-center"
        onClick={(e) => e.stopPropagation()}
      >
        {loading && !error && (
          <Loader2 className="size-10 animate-spin text-white/50" />
        )}

        {!error ? (
          <img
            src={url}
            alt={`Gallery ${index + 1}`}
            className={`max-w-full max-h-[90vh] object-contain ${loading ? 'hidden' : ''}`}
            onLoad={() => setLoading(false)}
            onError={() => {
              setLoading(false)
              setError(true)
            }}
          />
        ) : (
          <ImageOff className="size-12 text-white/50" />
        )}
      </div>
    </div>
  )
}
