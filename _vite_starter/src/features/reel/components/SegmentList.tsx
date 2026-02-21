import { Plus, Trash2, Clock, AlertCircle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { formatTime } from './constants'
import type { VideoSegment } from '../types'

// ค่าคงที่
const MAX_SEGMENTS = 10
const MAX_TOTAL_DURATION = 60 // วินาที
const MIN_SEGMENT_DURATION = 1 // วินาที

interface SegmentListProps {
  segments: VideoSegment[]
  videoDuration: number
  currentTime: number
  onChange: (segments: VideoSegment[]) => void
  onSeek: (time: number) => void
  onSelectSegment: (index: number) => void
  selectedIndex: number | null
}

export function SegmentList({
  segments,
  videoDuration,
  currentTime,
  onChange,
  onSeek,
  onSelectSegment,
  selectedIndex,
}: SegmentListProps) {
  // คำนวณ total duration
  const totalDuration = segments.reduce((sum, seg) => sum + (seg.end - seg.start), 0)
  const remainingDuration = MAX_TOTAL_DURATION - totalDuration
  const canAddMore = segments.length < MAX_SEGMENTS && remainingDuration >= MIN_SEGMENT_DURATION

  // เพิ่ม segment ใหม่
  const handleAddSegment = () => {
    if (!canAddMore) return

    // หา ID ใหม่
    const newId = `seg_${Date.now()}`

    // กำหนดช่วงเวลาใหม่ (เริ่มจากตำแหน่งปัจจุบัน)
    const startTime = Math.min(currentTime, videoDuration - MIN_SEGMENT_DURATION)
    const duration = Math.min(15, remainingDuration, videoDuration - startTime) // default 15s หรือเท่าที่เหลือ
    const endTime = startTime + duration

    const newSegment: VideoSegment = {
      id: newId,
      start: startTime,
      end: endTime,
    }

    onChange([...segments, newSegment])
    onSelectSegment(segments.length) // select ตัวใหม่
  }

  // ลบ segment
  const handleDeleteSegment = (index: number) => {
    const newSegments = segments.filter((_, i) => i !== index)
    onChange(newSegments)

    // ปรับ selected index
    if (selectedIndex === index) {
      onSelectSegment(newSegments.length > 0 ? 0 : null as unknown as number)
    } else if (selectedIndex !== null && selectedIndex > index) {
      onSelectSegment(selectedIndex - 1)
    }
  }

  // อัปเดต segment
  const handleUpdateSegment = (index: number, start: number, end: number) => {
    const newSegments = [...segments]
    newSegments[index] = { ...newSegments[index], start, end }
    onChange(newSegments)
  }

  return (
    <div className="space-y-3">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Clock className="h-4 w-4 text-muted-foreground" />
          <span className="text-sm font-medium">
            Segments ({segments.length}/{MAX_SEGMENTS})
          </span>
        </div>
        <div className="text-sm">
          <span className="text-muted-foreground">รวม: </span>
          <span className={`font-mono font-medium ${totalDuration > MAX_TOTAL_DURATION ? 'text-destructive' : 'text-primary'}`}>
            {formatTime(totalDuration)}
          </span>
          <span className="text-muted-foreground"> / {formatTime(MAX_TOTAL_DURATION)}</span>
        </div>
      </div>

      {/* Warning if over limit */}
      {totalDuration > MAX_TOTAL_DURATION && (
        <div className="flex items-center gap-2 p-2 bg-destructive/10 rounded text-destructive text-sm">
          <AlertCircle className="h-4 w-4" />
          <span>ความยาวรวมเกินกำหนด ({formatTime(MAX_TOTAL_DURATION)} วินาที)</span>
        </div>
      )}

      {/* Segment list */}
      <div className="space-y-2">
        {segments.length === 0 ? (
          <div className="text-center py-6 text-muted-foreground">
            <p>ยังไม่มี segment</p>
            <p className="text-sm">กดปุ่มด้านล่างเพื่อเพิ่ม</p>
          </div>
        ) : (
          segments.map((segment, index) => (
            <SegmentItem
              key={segment.id}
              segment={segment}
              index={index}
              isSelected={selectedIndex === index}
              videoDuration={videoDuration}
              maxDuration={MAX_TOTAL_DURATION - totalDuration + (segment.end - segment.start)}
              onSelect={() => {
                onSelectSegment(index)
                onSeek(segment.start)
              }}
              onDelete={() => handleDeleteSegment(index)}
              onUpdate={(start, end) => handleUpdateSegment(index, start, end)}
            />
          ))
        )}
      </div>

      {/* Add button */}
      <Button
        variant="outline"
        className="w-full"
        onClick={handleAddSegment}
        disabled={!canAddMore}
      >
        <Plus className="h-4 w-4 mr-2" />
        เพิ่ม Segment
        {remainingDuration > 0 && remainingDuration < MAX_TOTAL_DURATION && (
          <span className="ml-2 text-muted-foreground">
            (เหลือ {formatTime(remainingDuration)})
          </span>
        )}
      </Button>

      {/* Help text */}
      <p className="text-xs text-muted-foreground text-center">
        เลือกหลายช่วงเวลาแล้วนำมาต่อกันเป็นคลิปเดียว (สูงสุด {MAX_SEGMENTS} segments, รวม {MAX_TOTAL_DURATION} วินาที)
      </p>
    </div>
  )
}

// Segment item component
interface SegmentItemProps {
  segment: VideoSegment
  index: number
  isSelected: boolean
  videoDuration: number
  maxDuration: number
  onSelect: () => void
  onDelete: () => void
  onUpdate: (start: number, end: number) => void
}

function SegmentItem({
  segment,
  index,
  isSelected,
  videoDuration: _videoDuration,
  maxDuration: _maxDuration,
  onSelect,
  onDelete,
  onUpdate: _onUpdate,
}: SegmentItemProps) {
  // Reserved for future inline editing: _videoDuration, _maxDuration, _onUpdate
  const duration = segment.end - segment.start

  return (
    <Card
      className={`p-3 cursor-pointer transition-colors ${
        isSelected
          ? 'border-primary bg-primary/5'
          : 'hover:border-primary/50'
      }`}
      onClick={onSelect}
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          {/* Index badge */}
          <div className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-medium ${
            isSelected ? 'bg-primary text-primary-foreground' : 'bg-muted'
          }`}>
            {index + 1}
          </div>

          {/* Time info */}
          <div className="space-y-0.5">
            <div className="font-mono text-sm">
              {formatTime(segment.start)} - {formatTime(segment.end)}
            </div>
            <div className="text-xs text-muted-foreground">
              {formatTime(duration)}
            </div>
          </div>
        </div>

        {/* Delete button */}
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8 text-muted-foreground hover:text-destructive"
          onClick={(e) => {
            e.stopPropagation()
            onDelete()
          }}
        >
          <Trash2 className="h-4 w-4" />
        </Button>
      </div>
    </Card>
  )
}
