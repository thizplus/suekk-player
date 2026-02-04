import { Play, Loader2, Image } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Slider } from '@/components/ui/slider'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { formatTime, QUICK_DURATIONS, type ChunkOption } from './constants'

interface ReelTimecodeSelectorProps {
  videoDuration: number
  rawDuration: number
  segmentStart: number
  segmentEnd: number
  currentTime: number
  isVideoReady: boolean
  selectedChunk: ChunkOption | null
  chunkOptions: ChunkOption[]
  coverTime: number // -1 = auto middle
  onChunkChange: (chunk: ChunkOption) => void
  onSegmentStartChange: (time: number) => void
  onSegmentEndChange: (time: number) => void
  onSeekTo: (time: number) => void
  onPreviewSegment: () => void
  onCoverTimeChange: (time: number) => void
}

export function ReelTimecodeSelector({
  videoDuration,
  rawDuration,
  segmentStart,
  segmentEnd,
  currentTime,
  isVideoReady,
  selectedChunk,
  chunkOptions,
  coverTime,
  onChunkChange,
  onSegmentStartChange,
  onSegmentEndChange,
  onSeekTo,
  onPreviewSegment,
  onCoverTimeChange,
}: ReelTimecodeSelectorProps) {
  const segmentDuration = segmentEnd - segmentStart
  const hasMultipleChunks = chunkOptions.length > 1

  // Chunk-relative values for timeline display
  const chunkStart = selectedChunk?.start ?? 0
  const chunkEnd = selectedChunk?.end ?? videoDuration
  const chunkDuration = chunkEnd - chunkStart

  return (
    <div className="space-y-4">
      {/* Chunk selector - แสดงเมื่อ video ยาวกว่า 1 chunk */}
      {hasMultipleChunks && (
        <div className="space-y-2">
          <Label className="text-muted-foreground">เลือกช่วงเวลา (วิดีโอยาว {formatTime(rawDuration)})</Label>
          <Select
            value={selectedChunk?.value.toString() ?? '0'}
            onValueChange={(value) => {
              const chunk = chunkOptions.find(c => c.value === parseInt(value))
              if (chunk) {
                onChunkChange(chunk)
                onSeekTo(chunk.start)
              }
            }}
          >
            <SelectTrigger>
              <SelectValue placeholder="เลือกช่วงเวลา" />
            </SelectTrigger>
            <SelectContent>
              {chunkOptions.map((chunk) => (
                <SelectItem key={chunk.value} value={chunk.value.toString()}>
                  {chunk.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
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
                    // จำกัดไม่ให้เกิน chunk end
                    const newEnd = Math.min(segmentStart + duration, chunkEnd)
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

          {/* Cover/Thumbnail Frame Selection */}
          <div className="p-3 bg-muted/50 rounded-lg space-y-2">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Image className="h-4 w-4 text-muted-foreground" />
                <span className="text-muted-foreground">ภาพปก</span>
              </div>
              <span className="text-sm font-mono">
                {coverTime < 0 ? (
                  <span className="text-muted-foreground">อัตโนมัติ (กลางคลิป)</span>
                ) : (
                  <span className="text-primary font-medium">{formatTime(coverTime)}</span>
                )}
              </span>
            </div>
            <div className="flex items-center gap-2">
              <Button
                variant={coverTime < 0 ? 'default' : 'outline'}
                size="sm"
                className="flex-1"
                onClick={() => onCoverTimeChange(-1)}
              >
                อัตโนมัติ
              </Button>
              <Button
                variant={coverTime >= 0 ? 'default' : 'outline'}
                size="sm"
                className="flex-1"
                onClick={() => {
                  // ใช้ตำแหน่งปัจจุบันเป็น cover time
                  if (currentTime >= segmentStart && currentTime <= segmentEnd) {
                    onCoverTimeChange(currentTime)
                  } else {
                    // ถ้าอยู่นอกช่วง segment ให้ใช้กลาง segment
                    onCoverTimeChange(segmentStart + segmentDuration / 2)
                  }
                }}
                disabled={!isVideoReady}
              >
                ใช้ frame ปัจจุบัน
              </Button>
            </div>
            {coverTime >= 0 && (
              <p className="text-xs text-muted-foreground">
                เลื่อน video ไปที่ตำแหน่งที่ต้องการแล้วกด "ใช้ frame ปัจจุบัน"
              </p>
            )}
          </div>

          {/* Timeline - แสดงเฉพาะช่วง chunk ที่เลือก */}
          <div className="space-y-1">
            <Label className="text-muted-foreground">Timeline (คลิกเพื่อ seek)</Label>
            <div
              className={`relative h-14 bg-muted rounded-lg overflow-hidden ${isVideoReady ? 'cursor-pointer' : 'cursor-not-allowed opacity-50'}`}
              onClick={(e) => {
                if (!isVideoReady) return
                const rect = e.currentTarget.getBoundingClientRect()
                const x = e.clientX - rect.left
                // คำนวณเวลาภายใน chunk
                const time = chunkStart + (x / rect.width) * chunkDuration
                onSeekTo(time)
              }}
            >
              {/* ช่วงที่เลือก (relative to chunk) */}
              {segmentStart >= chunkStart && segmentStart < chunkEnd && (
                <div
                  className="absolute top-0 bottom-0 bg-primary/30 border-x-2 border-primary"
                  style={{
                    left: `${((segmentStart - chunkStart) / chunkDuration) * 100}%`,
                    width: `${((Math.min(segmentEnd, chunkEnd) - segmentStart) / chunkDuration) * 100}%`,
                  }}
                />
              )}

              {/* Cover time marker (relative to chunk) */}
              {coverTime >= chunkStart && coverTime <= chunkEnd && (
                <div
                  className="absolute top-0 bottom-0 w-1 bg-amber-500 z-5"
                  style={{ left: `${((coverTime - chunkStart) / chunkDuration) * 100}%` }}
                  title={`ภาพปก: ${formatTime(coverTime)}`}
                >
                  <div className="absolute -top-1 left-1/2 -translate-x-1/2 w-3 h-3 bg-amber-500 rotate-45" />
                </div>
              )}

              {/* ตำแหน่งปัจจุบัน (relative to chunk) */}
              {currentTime >= chunkStart && currentTime <= chunkEnd && (
                <div
                  className="absolute top-0 bottom-0 w-0.5 bg-red-500 z-10"
                  style={{ left: `${((currentTime - chunkStart) / chunkDuration) * 100}%` }}
                >
                  <div className="absolute top-1 left-1/2 -translate-x-1/2 text-sm font-mono bg-red-500 text-white px-1.5 py-0.5 rounded whitespace-nowrap">
                    {formatTime(currentTime)}
                  </div>
                </div>
              )}

              {/* เวลาเริ่มต้น-สิ้นสุดของ chunk */}
              <div className="absolute bottom-1 left-2 text-sm text-muted-foreground">
                {formatTime(chunkStart)}
              </div>
              <div className="absolute bottom-1 right-2 text-sm text-muted-foreground">
                {formatTime(chunkEnd)}
              </div>

              {/* กำลังโหลด */}
              {!isVideoReady && (
                <div className="absolute inset-0 flex items-center justify-center">
                  <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
                </div>
              )}
            </div>
          </div>

          {/* Slider จุดเริ่มต้น - ทำงานภายใน chunk */}
          <div className="space-y-1">
            <div className="flex items-center justify-between">
              <Label>จุดเริ่มต้น</Label>
              <span className="text-sm font-mono text-muted-foreground">
                {formatTime(segmentStart)}
              </span>
            </div>
            <Slider
              value={[segmentStart]}
              min={chunkStart}
              max={Math.max(chunkStart, chunkEnd - 1)}
              step={0.5}
              onValueChange={([value]) => {
                onSegmentStartChange(value)
                if (value >= segmentEnd) {
                  onSegmentEndChange(Math.min(value + 15, chunkEnd))
                }
                onSeekTo(value)
              }}
            />
          </div>

          {/* Slider จุดสิ้นสุด - ทำงานภายใน chunk */}
          <div className="space-y-1">
            <div className="flex items-center justify-between">
              <Label>จุดสิ้นสุด</Label>
              <span className="text-sm font-mono text-muted-foreground">
                {formatTime(segmentEnd)}
              </span>
            </div>
            <Slider
              value={[segmentEnd]}
              min={Math.max(chunkStart, segmentStart + 1)}
              max={chunkEnd}
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
