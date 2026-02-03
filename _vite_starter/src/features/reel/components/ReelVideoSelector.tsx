import { useState, useRef, useEffect } from 'react'
import { Search, X } from 'lucide-react'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Slider } from '@/components/ui/slider'
import type { Video } from '@/features/video/types'
import type { OutputFormat, VideoFit } from '../types'
import { OUTPUT_FORMAT_OPTIONS, VIDEO_FIT_OPTIONS, formatTime } from './constants'

interface ReelVideoSelectorProps {
  videos: Video[]
  selectedVideoId: string
  outputFormat: OutputFormat
  videoFit: VideoFit
  cropX: number
  cropY: number
  isEditing: boolean
  onVideoSelect: (videoId: string) => void
  onOutputFormatChange: (format: OutputFormat) => void
  onVideoFitChange: (fit: VideoFit) => void
  onCropXChange: (x: number) => void
  onCropYChange: (y: number) => void
}

export function ReelVideoSelector({
  videos,
  selectedVideoId,
  outputFormat,
  videoFit,
  cropX,
  cropY,
  isEditing,
  onVideoSelect,
  onOutputFormatChange,
  onVideoFitChange,
  onCropXChange,
  onCropYChange,
}: ReelVideoSelectorProps) {
  const needsCropPosition = videoFit !== 'fit'
  const hasVideo = !!selectedVideoId

  const [search, setSearch] = useState('')
  const [isOpen, setIsOpen] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)
  const dropdownRef = useRef<HTMLDivElement>(null)

  // Get selected video info
  const selectedVideo = videos.find(v => v.id === selectedVideoId)

  // Filter videos by search (title or code)
  const filteredVideos = videos.filter(video => {
    if (!search) return true
    const searchLower = search.toLowerCase()
    return (
      video.title.toLowerCase().includes(searchLower) ||
      video.code.toLowerCase().includes(searchLower)
    )
  })

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
  const handleSelect = (videoId: string) => {
    onVideoSelect(videoId)
    setSearch('')
    setIsOpen(false)
  }

  // Clear selection
  const handleClear = () => {
    onVideoSelect('')
    setSearch('')
  }

  return (
    <div className="space-y-4">
      {/* Video Search Input */}
      <div className="relative">
        <div className="relative">
          <Search className="absolute left-0 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
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
            className="w-full pl-6 pr-8 py-2 bg-transparent border-0 border-b border-input focus:border-primary focus:outline-none transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          />
          {selectedVideo && !isEditing && (
            <button
              onClick={handleClear}
              className="absolute right-0 top-1/2 -translate-y-1/2 p-1 hover:bg-muted rounded"
            >
              <X className="h-4 w-4 text-muted-foreground" />
            </button>
          )}
        </div>

        {/* Dropdown */}
        {isOpen && !isEditing && (
          <div
            ref={dropdownRef}
            className="absolute z-50 w-full mt-1 bg-popover border rounded-md shadow-lg max-h-60 overflow-auto"
          >
            {filteredVideos.length === 0 ? (
              <div className="p-3 text-sm text-muted-foreground text-center">
                ไม่พบวิดีโอ
              </div>
            ) : (
              filteredVideos.map((video) => (
                <button
                  key={video.id}
                  onClick={() => handleSelect(video.id)}
                  className={`w-full px-3 py-2 text-left hover:bg-muted transition-colors flex items-center justify-between ${
                    video.id === selectedVideoId ? 'bg-muted' : ''
                  }`}
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

      {hasVideo && (
        <div className="space-y-4">
          {/* Output Format Selection */}
          <div className="space-y-2">
            <Label className="text-sm text-muted-foreground">ขนาดกรอบ Output</Label>
            <div className="grid grid-cols-4 gap-2">
              {OUTPUT_FORMAT_OPTIONS.map((opt) => (
                <Button
                  key={opt.value}
                  variant={outputFormat === opt.value ? 'default' : 'outline'}
                  size="sm"
                  className="h-auto py-2 flex flex-col"
                  onClick={() => onOutputFormatChange(opt.value)}
                >
                  <span className="font-bold">{opt.label}</span>
                  <span className="text-sm opacity-70">{opt.description}</span>
                </Button>
              ))}
            </div>
          </div>

          {/* Video Fit Selection */}
          <div className="space-y-2">
            <Label className="text-sm text-muted-foreground">Video ในกรอบ (Crop จาก 16:9)</Label>
            <div className="grid grid-cols-5 gap-1">
              {VIDEO_FIT_OPTIONS.map((opt) => (
                <Button
                  key={opt.value}
                  variant={videoFit === opt.value ? 'default' : 'outline'}
                  size="sm"
                  className="h-auto py-2 flex flex-col"
                  onClick={() => onVideoFitChange(opt.value)}
                >
                  <span className="font-bold">{opt.label}</span>
                  <span className="text-sm opacity-70">{opt.description}</span>
                </Button>
              ))}
            </div>
          </div>

          {/* Crop Position Controls */}
          {needsCropPosition && (
            <div className="space-y-3 p-3 bg-muted/50 rounded-lg">
              <Label className="text-sm text-muted-foreground">ตำแหน่ง Crop</Label>

              {/* X Position */}
              <div className="space-y-1">
                <div className="flex items-center justify-between text-sm">
                  <span>ซ้าย</span>
                  <span className="font-mono">{cropX}%</span>
                  <span>ขวา</span>
                </div>
                <Slider
                  value={[cropX]}
                  min={0}
                  max={100}
                  step={1}
                  onValueChange={([v]) => onCropXChange(v)}
                />
              </div>

              {/* Y Position */}
              <div className="space-y-1">
                <div className="flex items-center justify-between text-sm">
                  <span>บน</span>
                  <span className="font-mono">{cropY}%</span>
                  <span>ล่าง</span>
                </div>
                <Slider
                  value={[cropY]}
                  min={0}
                  max={100}
                  step={1}
                  onValueChange={([v]) => onCropYChange(v)}
                />
              </div>

              {/* Quick Position Buttons */}
              <div className="grid grid-cols-3 gap-1">
                {[
                  { x: 0, y: 0, label: '↖' },
                  { x: 50, y: 0, label: '↑' },
                  { x: 100, y: 0, label: '↗' },
                  { x: 0, y: 50, label: '←' },
                  { x: 50, y: 50, label: '●' },
                  { x: 100, y: 50, label: '→' },
                  { x: 0, y: 100, label: '↙' },
                  { x: 50, y: 100, label: '↓' },
                  { x: 100, y: 100, label: '↘' },
                ].map((pos) => (
                  <Button
                    key={`${pos.x}-${pos.y}`}
                    variant={cropX === pos.x && cropY === pos.y ? 'default' : 'outline'}
                    size="sm"
                    className="h-8"
                    onClick={() => {
                      onCropXChange(pos.x)
                      onCropYChange(pos.y)
                    }}
                  >
                    {pos.label}
                  </Button>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
