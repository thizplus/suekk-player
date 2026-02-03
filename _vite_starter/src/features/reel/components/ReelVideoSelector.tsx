import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Slider } from '@/components/ui/slider'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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
  // Callbacks
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

  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm">1. เลือกวิดีโอ</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <Select
          value={selectedVideoId}
          onValueChange={onVideoSelect}
          disabled={isEditing}
        >
          <SelectTrigger>
            <SelectValue placeholder="เลือกวิดีโอ..." />
          </SelectTrigger>
          <SelectContent>
            {videos.map((video) => (
              <SelectItem key={video.id} value={video.id}>
                {video.code} - {video.title} ({formatTime(video.duration)})
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        {/* Output Format & Video Fit Options */}
        {hasVideo && (
          <div className="space-y-3">
            {/* Output Format Selection */}
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">1. ขนาดกรอบ Output</Label>
              <div className="grid grid-cols-4 gap-1">
                {OUTPUT_FORMAT_OPTIONS.map((opt) => (
                  <Button
                    key={opt.value}
                    variant={outputFormat === opt.value ? 'default' : 'outline'}
                    size="sm"
                    className="h-auto py-2 text-xs px-2 flex flex-col"
                    onClick={() => onOutputFormatChange(opt.value)}
                  >
                    <span className="font-bold">{opt.label}</span>
                    <span className="text-[10px] opacity-70">{opt.description}</span>
                  </Button>
                ))}
              </div>
            </div>

            {/* Video Fit Selection */}
            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">2. Video ในกรอบ (Crop จาก 16:9)</Label>
              <div className="grid grid-cols-5 gap-1">
                {VIDEO_FIT_OPTIONS.map((opt) => (
                  <Button
                    key={opt.value}
                    variant={videoFit === opt.value ? 'default' : 'outline'}
                    size="sm"
                    className="h-auto py-1.5 text-xs px-1 flex flex-col"
                    onClick={() => onVideoFitChange(opt.value)}
                  >
                    <span className="font-bold text-[11px]">{opt.label}</span>
                    <span className="text-[9px] opacity-70">{opt.description}</span>
                  </Button>
                ))}
              </div>
            </div>

            {/* Crop Position Controls */}
            {needsCropPosition && (
              <div className="space-y-3 p-3 bg-muted/50 rounded-lg">
                <Label className="text-xs text-muted-foreground">3. ตำแหน่ง Crop</Label>

                {/* X Position */}
                <div className="space-y-1">
                  <div className="flex items-center justify-between text-xs">
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
                  <div className="flex items-center justify-between text-xs">
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
                      className="h-6 text-[10px]"
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
      </CardContent>
    </Card>
  )
}
