import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Loader2, X, ChevronLeft, ChevronRight, ImageOff } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useVideoByCode } from '../hooks'
import { useStreamAccess } from '@/features/embed'
import { APP_CONFIG } from '@/constants/app-config'

export function GalleryPage() {
  const { code } = useParams<{ code: string }>()
  const navigate = useNavigate()

  // Fetch video data
  const { data: video, isLoading: videoLoading, error: videoError } = useVideoByCode(code ?? '')

  // Get stream token for CDN access
  const { data: streamAccess, isLoading: tokenLoading } = useStreamAccess(code ?? '', {
    enabled: !!code && !!video && video.status === 'ready',
  })

  // State
  const [lightboxIndex, setLightboxIndex] = useState<number | null>(null)
  const [loadedImages, setLoadedImages] = useState<Set<number>>(new Set())
  const [failedImages, setFailedImages] = useState<Set<number>>(new Set())

  const galleryCount = video?.galleryCount ?? 0
  const galleryPath = video?.galleryPath ?? ''

  // Build image URL with token
  const getImageUrl = useCallback((index: number) => {
    if (!galleryPath || !streamAccess?.token) return ''
    const imageNum = String(index + 1).padStart(3, '0')
    // CDN URL: cdnUrl/{galleryPath}/{imageNum}.jpg
    // Remove trailing slash from galleryPath to avoid double slash
    const cleanPath = galleryPath.replace(/\/+$/, '')
    return `${APP_CONFIG.cdnUrl}/${cleanPath}/${imageNum}.jpg`
  }, [galleryPath, streamAccess?.token])

  // Image fetch headers (need token)
  const fetchHeaders: Record<string, string> = streamAccess?.token
    ? { 'X-Stream-Token': streamAccess.token }
    : {}

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
  if (videoLoading || tokenLoading) {
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
  if (!galleryCount || galleryCount === 0) {
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
          {Array.from({ length: galleryCount }, (_, index) => (
            <GalleryImage
              key={index}
              index={index}
              url={getImageUrl(index)}
              headers={fetchHeaders}
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
      {lightboxIndex !== null && (
        <Lightbox
          url={getImageUrl(lightboxIndex)}
          headers={fetchHeaders}
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

// Gallery Image Component with lazy loading
interface GalleryImageProps {
  index: number
  url: string
  headers: Record<string, string>
  loaded: boolean
  failed: boolean
  onLoad: () => void
  onError: () => void
  onClick: () => void
}

function GalleryImage({ index, url, headers, loaded, failed, onLoad, onError, onClick }: GalleryImageProps) {
  const [blobUrl, setBlobUrl] = useState<string | undefined>()
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

  // Fetch image when visible
  useEffect(() => {
    if (!isVisible || !url || loaded || failed) return

    let cancelled = false

    const fetchImage = async () => {
      try {
        const response = await fetch(url, { headers })
        if (!response.ok || cancelled) {
          if (!cancelled) onError()
          return
        }

        const blob = await response.blob()
        if (cancelled) return

        const newBlobUrl = URL.createObjectURL(blob)
        setBlobUrl(newBlobUrl)
        onLoad()
      } catch {
        if (!cancelled) onError()
      }
    }

    fetchImage()

    return () => {
      cancelled = true
    }
  }, [isVisible, url, headers, loaded, failed, onLoad, onError])

  // Cleanup blob URL
  useEffect(() => {
    return () => {
      if (blobUrl) URL.revokeObjectURL(blobUrl)
    }
  }, [blobUrl])

  return (
    <div
      id={`gallery-img-${index}`}
      className="aspect-video bg-muted rounded-lg overflow-hidden cursor-pointer hover:ring-2 hover:ring-primary transition-all"
      onClick={onClick}
    >
      {blobUrl ? (
        <img
          src={blobUrl}
          alt={`Gallery ${index + 1}`}
          className="w-full h-full object-cover"
          loading="lazy"
        />
      ) : failed ? (
        <div className="w-full h-full flex items-center justify-center">
          <ImageOff className="size-6 text-muted-foreground/50" />
        </div>
      ) : (
        <div className="w-full h-full flex items-center justify-center">
          <Loader2 className="size-5 animate-spin text-muted-foreground/50" />
        </div>
      )}
    </div>
  )
}

// Lightbox Component
interface LightboxProps {
  url: string
  headers: Record<string, string>
  index: number
  total: number
  onClose: () => void
  onPrev: () => void
  onNext: () => void
}

function Lightbox({ url, headers, index, total, onClose, onPrev, onNext }: LightboxProps) {
  const [blobUrl, setBlobUrl] = useState<string | undefined>()
  const [loading, setLoading] = useState(true)

  // Fetch image for lightbox
  useEffect(() => {
    if (!url) return

    let cancelled = false
    setLoading(true)

    const fetchImage = async () => {
      try {
        const response = await fetch(url, { headers })
        if (!response.ok || cancelled) return

        const blob = await response.blob()
        if (cancelled) return

        const newBlobUrl = URL.createObjectURL(blob)
        setBlobUrl(prev => {
          if (prev) URL.revokeObjectURL(prev)
          return newBlobUrl
        })
        setLoading(false)
      } catch {
        setLoading(false)
      }
    }

    fetchImage()

    return () => {
      cancelled = true
    }
  }, [url, headers])

  // Cleanup
  useEffect(() => {
    return () => {
      if (blobUrl) URL.revokeObjectURL(blobUrl)
    }
  }, [blobUrl])

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
        {loading ? (
          <Loader2 className="size-10 animate-spin text-white/50" />
        ) : blobUrl ? (
          <img
            src={blobUrl}
            alt={`Gallery ${index + 1}`}
            className="max-w-full max-h-[90vh] object-contain"
          />
        ) : (
          <ImageOff className="size-12 text-white/50" />
        )}
      </div>
    </div>
  )
}
