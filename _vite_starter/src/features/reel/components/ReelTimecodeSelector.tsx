import { Play, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Slider } from '@/components/ui/slider'
import { formatTime, QUICK_DURATIONS, MAX_REEL_DURATION } from './constants'

interface ReelTimecodeSelectorProps {
  videoDuration: number
  rawDuration: number
  segmentStart: number
  segmentEnd: number
  currentTime: number
  isVideoReady: boolean
  onSegmentStartChange: (time: number) => void
  onSegmentEndChange: (time: number) => void
  onSeekTo: (time: number) => void
  onPreviewSegment: () => void
}

export function ReelTimecodeSelector({
  videoDuration,
  rawDuration,
  segmentStart,
  segmentEnd,
  currentTime,
  isVideoReady,
  onSegmentStartChange,
  onSegmentEndChange,
  onSeekTo,
  onPreviewSegment,
}: ReelTimecodeSelectorProps) {
  const isVideoCapped = rawDuration > MAX_REEL_DURATION

  return (
    <div className="space-y-4">
      {/* Notice if video is capped */}
      {isVideoCapped && (
        <div className="p-2 bg-yellow-500/10 border border-yellow-500/30 rounded text-sm text-yellow-600 dark:text-yellow-400">
          วิดีโอยาว {formatTime(rawDuration)} - ใช้ได้แค่ 10 นาทีแรก
        </div>
      )}

      {/* Show loading if duration not yet available */}
      {videoDuration === 0 && (
        <div className="flex items-center justify-center py-8 text-muted-foreground">
          <Loader2 className="h-5 w-5 animate-spin mr-2" />
          <span>กำลังโหลดข้อมูลวิดีโอ...</span>
        </div>
      )}

      {videoDuration > 0 && (
        <>
          {/* Selected Segment Info */}
          <div className="p-3 bg-primary/10 border border-primary/30 rounded-lg">
            <div className="flex items-center justify-between mb-2">
              <span className="font-medium">Segment ที่เลือก</span>
              <span className="text-lg font-bold text-primary">
                {formatTime(segmentEnd - segmentStart)}
              </span>
            </div>
            <div className="flex items-center justify-between text-sm text-muted-foreground">
              <span>เริ่ม: <span className="font-mono text-foreground">{formatTime(segmentStart)}</span></span>
              <span>→</span>
              <span>จบ: <span className="font-mono text-foreground">{formatTime(segmentEnd)}</span></span>
            </div>
          </div>

          {/* Timeline Visual */}
          <div
            className={`relative h-16 bg-muted rounded-lg overflow-hidden ${isVideoReady ? 'cursor-pointer' : 'cursor-not-allowed opacity-50'}`}
            onClick={(e) => {
              if (!isVideoReady) return
              const rect = e.currentTarget.getBoundingClientRect()
              const x = e.clientX - rect.left
              const time = (x / rect.width) * videoDuration
              onSeekTo(time)
            }}
          >
            {/* Selected range highlight */}
            <div
              className="absolute top-0 bottom-0 bg-primary/40 border-x-2 border-primary"
              style={{
                left: `${(segmentStart / videoDuration) * 100}%`,
                width: `${((segmentEnd - segmentStart) / videoDuration) * 100}%`,
              }}
            >
              <div className="absolute -left-1 top-1 text-sm font-bold text-primary bg-background px-1 rounded">
                IN
              </div>
              <div className="absolute -right-1 top-1 text-sm font-bold text-primary bg-background px-1 rounded">
                OUT
              </div>
            </div>

            {/* Current playhead */}
            <div
              className="absolute top-0 bottom-0 w-1 bg-red-500 z-10"
              style={{ left: `${(currentTime / videoDuration) * 100}%` }}
            >
              <div className="absolute -top-1 left-1/2 -translate-x-1/2 w-3 h-3 bg-red-500 rounded-full" />
              <div className="absolute top-4 left-1/2 -translate-x-1/2 text-sm font-mono bg-red-500 text-white px-1 rounded whitespace-nowrap">
                {formatTime(currentTime)}
              </div>
            </div>

            {/* Time markers */}
            <div className="absolute bottom-1 left-2 text-sm text-muted-foreground">
              0:00
            </div>
            <div className="absolute bottom-1 right-2 text-sm text-muted-foreground">
              {formatTime(videoDuration)}
            </div>

            {/* Loading indicator */}
            {!isVideoReady && (
              <div className="absolute inset-0 flex items-center justify-center">
                <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
              </div>
            )}
          </div>

          {/* Instructions */}
          <p className="text-sm text-muted-foreground text-center">
            คลิก timeline เพื่อ seek → กดปุ่ม "จุดเริ่ม" / "จุดจบ" ใต้ preview
          </p>

          {/* Start/End Sliders */}
          <div className="space-y-3">
            <div className="space-y-1">
              <div className="flex items-center justify-between">
                <Label>จุดเริ่มต้น</Label>
                <span className="text-sm text-muted-foreground font-mono">
                  {formatTime(segmentStart)}
                </span>
              </div>
              <Slider
                value={[segmentStart]}
                min={0}
                max={Math.max(0, videoDuration - 1)}
                step={0.5}
                onValueChange={([value]) => {
                  onSegmentStartChange(value)
                  if (value >= segmentEnd) {
                    onSegmentEndChange(Math.min(value + 15, videoDuration))
                  }
                  onSeekTo(value)
                }}
              />
            </div>

            <div className="space-y-1">
              <div className="flex items-center justify-between">
                <Label>จุดสิ้นสุด</Label>
                <span className="text-sm text-muted-foreground font-mono">
                  {formatTime(segmentEnd)}
                </span>
              </div>
              <Slider
                value={[segmentEnd]}
                min={segmentStart + 1}
                max={videoDuration}
                step={0.5}
                onValueChange={([value]) => {
                  onSegmentEndChange(value)
                  onSeekTo(value - 0.5)
                }}
              />
            </div>
          </div>

          {/* Preview Segment Button */}
          <Button
            variant="secondary"
            className="w-full"
            onClick={onPreviewSegment}
            disabled={!isVideoReady}
          >
            <Play className="h-4 w-4 mr-2" />
            Preview Segment ({formatTime(segmentEnd - segmentStart)})
          </Button>

          {/* Quick Duration Buttons */}
          <div className="flex gap-2">
            {QUICK_DURATIONS.map((duration) => (
              <Button
                key={duration}
                variant={segmentEnd - segmentStart === duration ? 'default' : 'outline'}
                size="sm"
                className="flex-1"
                onClick={() => {
                  const newEnd = Math.min(segmentStart + duration, videoDuration)
                  onSegmentEndChange(newEnd)
                }}
              >
                {duration}s
              </Button>
            ))}
          </div>
        </>
      )}
    </div>
  )
}
