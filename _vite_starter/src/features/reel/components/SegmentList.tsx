import { useMemo } from 'react'
import { Plus, Trash2, Clock, AlertCircle, GripVertical, Play } from 'lucide-react'
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
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
  onSelectSegment: (index: number | null) => void  // null = deselect
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
  // DnD sensors
  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: {
        distance: 8,
      },
    }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    })
  )

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
    const duration = Math.min(10, remainingDuration, videoDuration - startTime) // default 10s
    const endTime = startTime + duration

    const newSegment: VideoSegment = {
      id: newId,
      start: startTime,
      end: endTime,
    }

    onChange([...segments, newSegment])
    // ไม่ select segment ใหม่ - ให้ user browse หาตำแหน่งถัดไปได้อิสระ
    onSelectSegment(null)
    // ไม่ seek - ให้ user อยู่ตำแหน่งเดิมเพื่อดู preview แล้วไปต่อ
  }

  // ลบ segment
  const handleDeleteSegment = (index: number, e: React.MouseEvent) => {
    e.stopPropagation()
    const newSegments = segments.filter((_, i) => i !== index)
    onChange(newSegments)

    // ปรับ selected index
    if (selectedIndex === index) {
      if (newSegments.length > 0) {
        const newIndex = Math.min(index, newSegments.length - 1)
        onSelectSegment(newIndex)
        onSeek(newSegments[newIndex].start)
      } else {
        onSelectSegment(null)
      }
    } else if (selectedIndex !== null && selectedIndex > index) {
      onSelectSegment(selectedIndex - 1)
    }
  }

  // Preview segment
  const handlePreviewSegment = (index: number) => {
    onSelectSegment(index)
    onSeek(segments[index].start)
  }

  // Drag end handler
  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event

    if (over && active.id !== over.id) {
      const oldIndex = segments.findIndex((s) => s.id === active.id)
      const newIndex = segments.findIndex((s) => s.id === over.id)

      const newSegments = arrayMove(segments, oldIndex, newIndex)
      onChange(newSegments)

      // Update selected index after reorder
      if (selectedIndex === oldIndex) {
        onSelectSegment(newIndex)
      } else if (selectedIndex !== null) {
        if (oldIndex < selectedIndex && newIndex >= selectedIndex) {
          onSelectSegment(selectedIndex - 1)
        } else if (oldIndex > selectedIndex && newIndex <= selectedIndex) {
          onSelectSegment(selectedIndex + 1)
        }
      }
    }
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

      {/* Timeline Visualization */}
      {segments.length > 0 && videoDuration > 0 && (
        <SegmentTimeline
          segments={segments}
          videoDuration={videoDuration}
          selectedIndex={selectedIndex}
          onSelectSegment={handlePreviewSegment}
        />
      )}

      {/* Warning if over limit */}
      {totalDuration > MAX_TOTAL_DURATION && (
        <div className="flex items-center gap-2 p-2 bg-destructive/10 rounded text-destructive text-sm">
          <AlertCircle className="h-4 w-4" />
          <span>ความยาวรวมเกินกำหนด ({formatTime(MAX_TOTAL_DURATION)} วินาที)</span>
        </div>
      )}

      {/* Segment list with drag & drop */}
      <div className="space-y-2">
        {segments.length === 0 ? (
          <div className="text-center py-6 text-muted-foreground">
            <p>ยังไม่มี segment</p>
            <p className="text-sm">กดปุ่มด้านล่างเพื่อเพิ่ม</p>
          </div>
        ) : (
          <DndContext
            sensors={sensors}
            collisionDetection={closestCenter}
            onDragEnd={handleDragEnd}
          >
            <SortableContext
              items={segments.map((s) => s.id)}
              strategy={verticalListSortingStrategy}
            >
              {segments.map((segment, index) => (
                <SortableSegmentItem
                  key={segment.id}
                  segment={segment}
                  index={index}
                  isSelected={selectedIndex === index}
                  onSelect={() => handlePreviewSegment(index)}
                  onDelete={(e) => handleDeleteSegment(index, e)}
                />
              ))}
            </SortableContext>
          </DndContext>
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
        เพิ่ม Segment จากตำแหน่งปัจจุบัน
        {remainingDuration > 0 && remainingDuration < MAX_TOTAL_DURATION && (
          <span className="ml-2 text-muted-foreground">
            (เหลือ {formatTime(remainingDuration)})
          </span>
        )}
      </Button>

      {/* Help text */}
      <p className="text-xs text-muted-foreground text-center">
        ลากเพื่อเรียงลำดับ • กดเพื่อแก้ไข • สูงสุด {MAX_SEGMENTS} segments รวม {MAX_TOTAL_DURATION} วินาที
      </p>
    </div>
  )
}

// Timeline visualization component
interface SegmentTimelineProps {
  segments: VideoSegment[]
  videoDuration: number
  selectedIndex: number | null
  onSelectSegment: (index: number) => void
}

function SegmentTimeline({
  segments,
  videoDuration,
  selectedIndex,
  onSelectSegment,
}: SegmentTimelineProps) {
  // Sort segments by start time for display
  const sortedForDisplay = useMemo(() => {
    return segments
      .map((seg, index) => ({ ...seg, originalIndex: index }))
      .sort((a, b) => a.start - b.start)
  }, [segments])

  return (
    <div className="space-y-1">
      <div className="text-xs text-muted-foreground">Timeline (ตำแหน่งจริงใน video)</div>
      <div className="relative h-8 bg-muted rounded overflow-hidden">
        {/* Background grid lines */}
        <div className="absolute inset-0 flex">
          {[...Array(10)].map((_, i) => (
            <div
              key={i}
              className="flex-1 border-r border-background/20 last:border-r-0"
            />
          ))}
        </div>

        {/* Segment blocks */}
        {sortedForDisplay.map((segment) => {
          const left = (segment.start / videoDuration) * 100
          const width = ((segment.end - segment.start) / videoDuration) * 100
          const isSelected = selectedIndex === segment.originalIndex

          return (
            <button
              key={segment.id}
              className={`absolute top-1 bottom-1 rounded cursor-pointer transition-all ${
                isSelected
                  ? 'bg-primary ring-2 ring-primary ring-offset-1 ring-offset-background'
                  : 'bg-primary/60 hover:bg-primary/80'
              }`}
              style={{
                left: `${left}%`,
                width: `${Math.max(width, 1)}%`,
              }}
              onClick={() => onSelectSegment(segment.originalIndex)}
              title={`Segment ${segment.originalIndex + 1}: ${formatTime(segment.start)} - ${formatTime(segment.end)}`}
            >
              <span className="absolute inset-0 flex items-center justify-center text-[10px] font-medium text-primary-foreground">
                {segment.originalIndex + 1}
              </span>
            </button>
          )
        })}

        {/* Time markers */}
        <div className="absolute bottom-0 left-0 right-0 flex justify-between text-[8px] text-muted-foreground px-1">
          <span>0:00</span>
          <span>{formatTime(videoDuration / 2)}</span>
          <span>{formatTime(videoDuration)}</span>
        </div>
      </div>
    </div>
  )
}

// Sortable segment item component
interface SortableSegmentItemProps {
  segment: VideoSegment
  index: number
  isSelected: boolean
  onSelect: () => void
  onDelete: (e: React.MouseEvent) => void
}

function SortableSegmentItem({
  segment,
  index,
  isSelected,
  onSelect,
  onDelete,
}: SortableSegmentItemProps) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: segment.id })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  }

  const duration = segment.end - segment.start

  return (
    <Card
      ref={setNodeRef}
      style={style}
      className={`p-3 transition-colors ${
        isDragging
          ? 'opacity-50 shadow-lg'
          : isSelected
          ? 'border-primary bg-primary/5'
          : 'hover:border-primary/50'
      }`}
    >
      <div className="flex items-center gap-2">
        {/* Drag handle */}
        <button
          className="cursor-grab active:cursor-grabbing p-1 -m-1 text-muted-foreground hover:text-foreground touch-none"
          {...attributes}
          {...listeners}
        >
          <GripVertical className="h-4 w-4" />
        </button>

        {/* Index badge */}
        <div
          className={`w-6 h-6 rounded-full flex items-center justify-center text-xs font-medium shrink-0 ${
            isSelected ? 'bg-primary text-primary-foreground' : 'bg-muted'
          }`}
        >
          {index + 1}
        </div>

        {/* Time info - clickable */}
        <button
          className="flex-1 text-left"
          onClick={onSelect}
        >
          <div className="font-mono text-sm">
            {formatTime(segment.start)} - {formatTime(segment.end)}
          </div>
          <div className="text-xs text-muted-foreground">
            ความยาว {formatTime(duration)}
          </div>
        </button>

        {/* Preview button */}
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8 text-muted-foreground hover:text-primary"
          onClick={(e) => {
            e.stopPropagation()
            onSelect()
          }}
          title="Preview"
        >
          <Play className="h-4 w-4" />
        </Button>

        {/* Delete button */}
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8 text-muted-foreground hover:text-destructive"
          onClick={onDelete}
        >
          <Trash2 className="h-4 w-4" />
        </Button>
      </div>
    </Card>
  )
}
