import { useState, useRef, useEffect, useMemo } from 'react'
import { Search, X, Loader2, Check } from 'lucide-react'
import { Label } from '@/components/ui/label'
import { useVideos } from '@/features/video/hooks'
import type { Video } from '@/features/video/types'
import type { ReelStyle } from '../types'
import { REEL_STYLE_OPTIONS, formatTime } from './constants'
import { cn } from '@/lib/utils'

// Custom hook สำหรับ debounce
function useDebounce<T>(value: T, delay: number): T {
  const [debouncedValue, setDebouncedValue] = useState<T>(value)

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedValue(value)
    }, delay)

    return () => {
      clearTimeout(timer)
    }
  }, [value, delay])

  return debouncedValue
}

interface ReelVideoSelectorProps {
  selectedVideoId: string
  selectedVideo: Video | undefined
  style: ReelStyle
  isEditing: boolean
  onVideoSelect: (videoId: string, video?: Video) => void
  onStyleChange: (style: ReelStyle) => void
}

export function ReelVideoSelector({
  selectedVideoId,
  selectedVideo,
  style,
  isEditing,
  onVideoSelect,
  onStyleChange,
}: ReelVideoSelectorProps) {
  const hasVideo = !!selectedVideoId

  const [search, setSearch] = useState('')
  const [isOpen, setIsOpen] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)
  const dropdownRef = useRef<HTMLDivElement>(null)

  // Debounce search term (300ms)
  const debouncedSearch = useDebounce(search, 300)

  // Fetch videos from API with search (only when dropdown is open)
  const { data: videosData, isLoading: isSearching } = useVideos(
    {
      status: 'ready',
      search: debouncedSearch || undefined,
      limit: 20,
    },
    { enabled: isOpen }
  )

  // Get filtered videos
  const filteredVideos = useMemo(() => {
    return videosData?.data || []
  }, [videosData?.data])

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(e.target as Node) &&
        inputRef.current &&
        !inputRef.current.contains(e.target as Node)
      ) {
        setIsOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  // Handle video selection
  const handleSelect = (video: Video) => {
    onVideoSelect(video.id, video)
    setSearch('')
    setIsOpen(false)
  }

  // Clear selection
  const handleClear = () => {
    onVideoSelect('', undefined)
    setSearch('')
  }

  return (
    <div className="space-y-6">
      {/* Video Search Input */}
      <div className="space-y-2">
        <Label className="text-sm font-medium">เลือกวิดีโอ</Label>
        <div className="relative">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <input
              ref={inputRef}
              type="text"
              value={selectedVideo && !isOpen ? `${selectedVideo.code} - ${selectedVideo.title}` : search}
              onChange={(e) => {
                setSearch(e.target.value)
                setIsOpen(true)
              }}
              onFocus={() => {
                if (!isEditing) {
                  setIsOpen(true)
                  if (selectedVideo) setSearch('')
                }
              }}
              placeholder="ค้นหาวิดีโอ (code หรือ title)..."
              disabled={isEditing}
              className="w-full pl-10 pr-10 py-2.5 bg-background border border-input rounded-lg focus:border-primary focus:ring-1 focus:ring-primary focus:outline-none transition-all disabled:opacity-50 disabled:cursor-not-allowed"
            />
            {selectedVideo && !isEditing && (
              <button
                onClick={handleClear}
                className="absolute right-3 top-1/2 -translate-y-1/2 p-1 hover:bg-muted rounded"
              >
                <X className="h-4 w-4 text-muted-foreground" />
              </button>
            )}
          </div>

          {/* Dropdown */}
          {isOpen && !isEditing && (
            <div
              ref={dropdownRef}
              className="absolute z-50 w-full mt-1 bg-popover border rounded-lg shadow-lg max-h-60 overflow-auto"
            >
              {isSearching ? (
                <div className="p-4 flex items-center justify-center text-muted-foreground">
                  <Loader2 className="h-4 w-4 animate-spin mr-2" />
                  <span className="text-sm">กำลังค้นหา...</span>
                </div>
              ) : filteredVideos.length === 0 ? (
                <div className="p-4 text-sm text-muted-foreground text-center">
                  {debouncedSearch ? 'ไม่พบวิดีโอ' : 'พิมพ์เพื่อค้นหาวิดีโอ'}
                </div>
              ) : (
                filteredVideos.map((video) => (
                  <button
                    key={video.id}
                    onClick={() => handleSelect(video)}
                    className={cn(
                      'w-full px-4 py-3 text-left hover:bg-muted transition-colors flex items-center justify-between',
                      video.id === selectedVideoId && 'bg-muted'
                    )}
                  >
                    <div className="flex-1 min-w-0">
                      <div className="font-medium truncate">{video.title}</div>
                      <div className="text-sm text-muted-foreground">{video.code}</div>
                    </div>
                    <div className="text-sm text-muted-foreground ml-2">
                      {formatTime(video.duration)}
                    </div>
                  </button>
                ))
              )}
            </div>
          )}
        </div>
      </div>

      {/* Style Selection - Only show after video is selected */}
      {hasVideo && (
        <div className="space-y-3">
          <Label className="text-sm font-medium">เลือกสไตล์</Label>
          <div className="grid grid-cols-3 gap-3">
            {REEL_STYLE_OPTIONS.map((opt) => (
              <button
                key={opt.value}
                onClick={() => onStyleChange(opt.value)}
                className={cn(
                  'relative flex flex-col items-center p-4 rounded-xl border-2 transition-all hover:border-primary/50',
                  style === opt.value
                    ? 'border-primary bg-primary/5'
                    : 'border-muted bg-muted/30 hover:bg-muted/50'
                )}
              >
                {/* Selected indicator */}
                {style === opt.value && (
                  <div className="absolute top-2 right-2">
                    <Check className="h-4 w-4 text-primary" />
                  </div>
                )}

                {/* Icon */}
                <span className="text-3xl mb-2">{opt.icon}</span>

                {/* Label */}
                <span className="font-semibold text-sm">{opt.label}</span>

                {/* Description */}
                <span className="text-xs text-muted-foreground mt-1 text-center">
                  {opt.description}
                </span>
              </button>
            ))}
          </div>

          {/* Style preview hint */}
          <p className="text-xs text-muted-foreground text-center">
            Output: 1080x1920 (9:16) พร้อม Logo และ Text Overlay
          </p>
        </div>
      )}
    </div>
  )
}
