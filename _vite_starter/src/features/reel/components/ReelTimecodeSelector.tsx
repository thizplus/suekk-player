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
  const segmentDuration = segmentEnd - segmentStart

  return (
    <div className="space-y-4">
      {/* แจ้งเตือนถ้าวิดีโอยาวเกิน */}
      {isVideoCapped && (
        <div className="p-2 bg-yellow-500/10 border border-yellow-500/30 rounded text-sm text-yellow-600 dark:text-yellow-400">
          วิดีโอยาว {formatTime(rawDuration)} - ใช้ได้แค่ 10 นาทีแรก
        </div>
      )}

      {/* กำลังโหลด */}
      {videoDuration === 0 && (
        <div className="flex items-center justify-center py-8 text-muted-foreground">
          <Loader2 className="h-5 w-5 animate-spin mr-2" />
          <span>กำลังโหลดข้อมูลวิดีโอ...</span>
        </div>
      )}

      {videoDuration > 0 && (
        <>
          {/* ความยาวคลิปที่ต้องการ */}
          <div className="space-y-2">
            <Label className="text-muted-foreground">ความยาวคลิป</Label>
            <div className="flex flex-wrap gap-2">
              {QUICK_DURATIONS.map((duration) => (
                <Button
                  key={duration}
                  variant={Math.round(segmentDuration) === duration ? 'default' : 'outline'}
                  size="sm"
                  onClick={() => {
                    const newEnd = Math.min(segmentStart + duration, videoDuration)
                    onSegmentEndChange(newEnd)
                  }}
                >
                  {duration} วินาที
                </Button>
              ))}
            </div>
          </div>

          {/* สรุปช่วงที่เลือก */}
          <div className="p-3 bg-muted/50 rounded-lg space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-muted-foreground">ช่วงที่เลือก</span>
              <span className="text-lg font-bold text-primary">
                {formatTime(segmentDuration)}
              </span>
            </div>
            <div className="flex items-center justify-between text-sm">
              <span>
                <span className="text-muted-foreground">เริ่ม </span>
                <span className="font-mono font-medium">{formatTime(segmentStart)}</span>
              </span>
              <span className="text-muted-foreground">ถึง</span>
              <span>
                <span className="text-muted-foreground">จบ </span>
                <span className="font-mono font-medium">{formatTime(segmentEnd)}</span>
              </span>
            </div>
          </div>

          {/* Timeline */}
          <div className="space-y-1">
            <Label className="text-muted-foreground">เลือกช่วงเวลา (คลิกเพื่อ seek)</Label>
            <div
              className={`relative h-14 bg-muted rounded-lg overflow-hidden ${isVideoReady ? 'cursor-pointer' : 'cursor-not-allowed opacity-50'}`}
              onClick={(e) => {
                if (!isVideoReady) return
                const rect = e.currentTarget.getBoundingClientRect()
                const x = e.clientX - rect.left
                const time = (x / rect.width) * videoDuration
                onSeekTo(time)
              }}
            >
              {/* ช่วงที่เลือก */}
              <div
                className="absolute top-0 bottom-0 bg-primary/30 border-x-2 border-primary"
                style={{
                  left: `${(segmentStart / videoDuration) * 100}%`,
                  width: `${((segmentEnd - segmentStart) / videoDuration) * 100}%`,
                }}
              />

              {/* ตำแหน่งปัจจุบัน */}
              <div
                className="absolute top-0 bottom-0 w-0.5 bg-red-500 z-10"
                style={{ left: `${(currentTime / videoDuration) * 100}%` }}
              >
                <div className="absolute top-1 left-1/2 -translate-x-1/2 text-sm font-mono bg-red-500 text-white px-1.5 py-0.5 rounded whitespace-nowrap">
                  {formatTime(currentTime)}
                </div>
              </div>

              {/* เวลาเริ่มต้น-สิ้นสุดของวิดีโอ */}
              <div className="absolute bottom-1 left-2 text-sm text-muted-foreground">
                0:00
              </div>
              <div className="absolute bottom-1 right-2 text-sm text-muted-foreground">
                {formatTime(videoDuration)}
              </div>

              {/* กำลังโหลด */}
              {!isVideoReady && (
                <div className="absolute inset-0 flex items-center justify-center">
                  <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
                </div>
              )}
            </div>
          </div>

          {/* Slider จุดเริ่มต้น */}
          <div className="space-y-1">
            <div className="flex items-center justify-between">
              <Label>จุดเริ่มต้น</Label>
              <span className="text-sm font-mono text-muted-foreground">
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

          {/* Slider จุดสิ้นสุด */}
          <div className="space-y-1">
            <div className="flex items-center justify-between">
              <Label>จุดสิ้นสุด</Label>
              <span className="text-sm font-mono text-muted-foreground">
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

          {/* ปุ่ม Preview */}
          <Button
            variant="secondary"
            className="w-full"
            onClick={onPreviewSegment}
            disabled={!isVideoReady}
          >
            <Play className="h-4 w-4 mr-2" />
            ดูตัวอย่างคลิป ({formatTime(segmentDuration)})
          </Button>
        </>
      )}
    </div>
  )
}
