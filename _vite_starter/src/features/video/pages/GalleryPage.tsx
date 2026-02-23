import { useState, useEffect, useCallback, useMemo } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Loader2, X, ChevronLeft, ChevronRight, ImageOff, Shield, ShieldCheck, ShieldAlert } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Badge } from '@/components/ui/badge'
import { useVideoByCode, useGalleryUrls } from '../hooks'

type GalleryTab = 'super_safe' | 'safe' | 'nsfw' | 'all'

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
  const [activeTab, setActiveTab] = useState<GalleryTab>('super_safe')
  const [lightboxIndex, setLightboxIndex] = useState<number | null>(null)
  const [loadedImages, setLoadedImages] = useState<Set<number>>(new Set())
  const [failedImages, setFailedImages] = useState<Set<number>>(new Set())

  // Compute URLs based on active tab (Three-Tier)
  const superSafeUrls = galleryData?.superSafeUrls ?? galleryData?.urls ?? []
  const safeUrls = galleryData?.safeUrls ?? []
  const nsfwUrls = galleryData?.nsfwUrls ?? []
  const hasSafe = safeUrls.length > 0
  const hasNsfw = nsfwUrls.length > 0

  const imageUrls = useMemo(() => {
    switch (activeTab) {
      case 'super_safe':
        return superSafeUrls
      case 'safe':
        return safeUrls
      case 'nsfw':
        return nsfwUrls
      case 'all':
        return [...superSafeUrls, ...safeUrls, ...nsfwUrls]
      default:
        return superSafeUrls
    }
  }, [activeTab, superSafeUrls, safeUrls, nsfwUrls])

  const galleryCount = imageUrls.length

  // Reset loaded/failed state when tab changes
  useEffect(() => {
    setLoadedImages(new Set())
    setFailedImages(new Set())
    setLightboxIndex(null)
  }, [activeTab])

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

  // No gallery state (check total from API, not filtered count) - Three-Tier
  const totalGalleryCount = (galleryData?.superSafeCount ?? 0) + (galleryData?.safeCount ?? 0) + (galleryData?.nsfwCount ?? 0) || (galleryData?.count ?? 0)
  if (totalGalleryCount === 0) {
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
        <div className="max-w-7xl mx-auto px-4 py-3">
          <div className="flex items-center gap-3">
            <Button variant="ghost" size="icon" onClick={() => navigate(-1)}>
              <ArrowLeft className="size-5" />
            </Button>
            <div className="flex-1 min-w-0">
              <h1 className="font-semibold truncate">Gallery: {video.code}</h1>
              <p className="text-sm text-muted-foreground">{galleryCount} ภาพ</p>
            </div>
          </div>

          {/* Tabs สำหรับ Three-Tier (super_safe/safe/nsfw) */}
          {(hasSafe || hasNsfw) && (
            <div className="mt-3">
              <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as GalleryTab)}>
                <TabsList className="grid w-full grid-cols-4">
                  <TabsTrigger value="super_safe" className="gap-1 text-xs px-2">
                    <ShieldCheck className="size-3.5" />
                    <span className="hidden sm:inline">Super</span>Safe
                    <Badge variant="secondary" className="ml-1 text-[10px]">{superSafeUrls.length}</Badge>
                  </TabsTrigger>
                  <TabsTrigger value="safe" className="gap-1 text-xs px-2">
                    <Shield className="size-3.5" />
                    Safe
                    <Badge variant="secondary" className="ml-1 text-[10px]">{safeUrls.length}</Badge>
                  </TabsTrigger>
                  <TabsTrigger value="nsfw" className="gap-1 text-xs px-2">
                    <ShieldAlert className="size-3.5" />
                    NSFW
                    <Badge variant="secondary" className="ml-1 text-[10px]">{nsfwUrls.length}</Badge>
                  </TabsTrigger>
                  <TabsTrigger value="all" className="gap-1 text-xs px-2">
                    All
                    <Badge variant="secondary" className="ml-1 text-[10px]">{superSafeUrls.length + safeUrls.length + nsfwUrls.length}</Badge>
                  </TabsTrigger>
                </TabsList>
              </Tabs>
            </div>
          )}
        </div>
      </header>

      {/* Gallery Grid */}
      <main className="max-w-7xl mx-auto px-4 py-6">
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-2 sm:gap-3">
          {imageUrls.map((url, index) => (
            <GalleryImage
              key={`${activeTab}-${index}`}
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

// Extract filename from presigned URL (e.g., gallery/code/safe/001.jpg?X-Amz-... → 001.jpg)
function extractFilename(url: string): string {
  try {
    const urlPath = new URL(url).pathname
    const parts = urlPath.split('/')
    return parts[parts.length - 1] || `image`
  } catch {
    // Fallback: try to extract before query string
    const pathPart = url.split('?')[0]
    const parts = pathPart.split('/')
    return parts[parts.length - 1] || `image`
  }
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
  const filename = extractFilename(url)

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
      className="relative aspect-video bg-muted rounded-lg overflow-hidden cursor-pointer hover:ring-2 hover:ring-primary transition-all"
      onClick={onClick}
    >
      {isVisible && !failed ? (
        <img
          src={url}
          alt={filename}
          className={`w-full h-full object-cover ${loaded ? '' : 'opacity-0'}`}
          loading="lazy"
          onLoad={onLoad}
          onError={onError}
        />
      ) : null}

      {/* Filename badge */}
      {loaded && (
        <div className="absolute bottom-1 left-1 px-1.5 py-0.5 rounded bg-black/70 text-white text-xs font-mono">
          {filename}
        </div>
      )}

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

      {/* Image counter + filename */}
      <div className="absolute top-4 left-4 flex items-center gap-2">
        <div className="px-3 py-1.5 rounded bg-white/10 text-white text-sm">
          {index + 1} / {total}
        </div>
        <div className="px-3 py-1.5 rounded bg-white/10 text-white text-sm font-mono">
          {extractFilename(url)}
        </div>
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
