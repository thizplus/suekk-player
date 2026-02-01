/**
 * SubtitleEditor - Component สำหรับแก้ไข subtitle
 * แสดง list ของ segments พร้อม highlight ตาม video time
 */

import { useRef, useEffect, useCallback } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { Save, RotateCcw } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { SubtitleSegment } from '../types'
import { timestampToSeconds } from '../utils/srt-parser'

interface SubtitleEditorProps {
  segments: SubtitleSegment[]
  currentTime: number // video time in seconds
  onSegmentChange: (index: number, text: string) => void
  onTimecodeChange?: (index: number, startTime: string, endTime: string) => void
  onSeek: (seconds: number) => void
  onSave: () => void
  onReset: () => void
  isDirty: boolean
  isSaving: boolean
  isLoading?: boolean
  language?: string
}

export function SubtitleEditor({
  segments,
  currentTime,
  onSegmentChange,
  onTimecodeChange,
  onSeek,
  onSave,
  onReset,
  isDirty,
  isSaving,
  isLoading = false,
  language = 'th',
}: SubtitleEditorProps) {
  const scrollAreaRef = useRef<HTMLDivElement>(null)
  const activeRowRef = useRef<HTMLDivElement>(null)

  // หา active segment index
  const activeIndex = segments.findIndex((segment) => {
    const start = timestampToSeconds(segment.startTime)
    const end = timestampToSeconds(segment.endTime)
    return currentTime >= start && currentTime <= end
  })

  // Auto-scroll ไปยัง active segment
  useEffect(() => {
    if (activeRowRef.current && scrollAreaRef.current) {
      const container = scrollAreaRef.current.querySelector('[data-radix-scroll-area-viewport]')
      if (container) {
        const rowTop = activeRowRef.current.offsetTop
        const rowHeight = activeRowRef.current.offsetHeight
        const containerHeight = container.clientHeight
        const scrollTop = container.scrollTop

        // Scroll ถ้า row อยู่นอก viewport
        if (rowTop < scrollTop || rowTop + rowHeight > scrollTop + containerHeight) {
          container.scrollTo({
            top: rowTop - containerHeight / 3,
            behavior: 'smooth',
          })
        }
      }
    }
  }, [activeIndex])

  // Handle Ctrl+S
  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === 's') {
        e.preventDefault()
        if (isDirty && !isSaving) {
          onSave()
        }
      }
    },
    [isDirty, isSaving, onSave]
  )

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [handleKeyDown])

  if (isLoading) {
    return (
      <div className="flex h-full flex-col">
        <div className="flex items-center justify-between border-b p-3">
          <Skeleton className="h-6 w-24" />
          <div className="flex gap-2">
            <Skeleton className="h-9 w-20" />
            <Skeleton className="h-9 w-20" />
          </div>
        </div>
        <div className="flex-1 space-y-3 p-4">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-20 w-full" />
          ))}
        </div>
      </div>
    )
  }

  const languageLabel = language === 'th' ? 'ไทย' : language === 'en' ? 'English' : language

  return (
    <div className="flex h-full flex-col">
      {/* Header */}
      <div className="flex items-center justify-between border-b bg-muted/30 px-4 py-3">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium">Subtitle Editor</span>
          <span className="rounded bg-primary/10 px-2 py-0.5 text-xs text-primary">
            {languageLabel}
          </span>
          {isDirty && (
            <span className="rounded bg-yellow-500/10 px-2 py-0.5 text-xs text-yellow-600 dark:text-yellow-400">
              ยังไม่บันทึก
            </span>
          )}
        </div>
        <div className="flex gap-2">
          {/* Reset button */}
          <TooltipProvider>
            <Tooltip>
              <AlertDialog>
                <TooltipTrigger asChild>
                  <AlertDialogTrigger asChild>
                    <Button variant="outline" size="sm" disabled={!isDirty || isSaving}>
                      <RotateCcw className="mr-1.5 h-4 w-4" />
                      รีเซ็ต
                    </Button>
                  </AlertDialogTrigger>
                </TooltipTrigger>
                <TooltipContent>ยกเลิกการเปลี่ยนแปลงทั้งหมด</TooltipContent>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>ยกเลิกการแก้ไข?</AlertDialogTitle>
                    <AlertDialogDescription>
                      การเปลี่ยนแปลงทั้งหมดจะถูกยกเลิก และกลับไปเป็นค่าเดิม
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel>ยกเลิก</AlertDialogCancel>
                    <AlertDialogAction onClick={onReset}>รีเซ็ต</AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            </Tooltip>
          </TooltipProvider>

          {/* Save button */}
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  size="sm"
                  onClick={onSave}
                  disabled={!isDirty || isSaving}
                  className="min-w-[80px]"
                >
                  {isSaving ? (
                    <>
                      <span className="mr-1.5 h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
                      กำลังบันทึก...
                    </>
                  ) : (
                    <>
                      <Save className="mr-1.5 h-4 w-4" />
                      บันทึก
                    </>
                  )}
                </Button>
              </TooltipTrigger>
              <TooltipContent>Ctrl+S</TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </div>
      </div>

      {/* Segments list */}
      <ScrollArea ref={scrollAreaRef} className="flex-1 overflow-auto">
        <div className="divide-y">
          {segments.length === 0 ? (
            <div className="py-8 text-center text-muted-foreground">ไม่มี subtitle</div>
          ) : (
            segments.map((segment, index) => {
              const isActive = index === activeIndex
              const startSeconds = timestampToSeconds(segment.startTime)

              return (
                <div
                  key={segment.index}
                  ref={isActive ? activeRowRef : null}
                  className={cn(
                    'flex items-center gap-1.5 px-2 py-1.5 transition-colors',
                    isActive ? 'bg-primary/10' : 'hover:bg-muted/50'
                  )}
                >
                  {/* Start Time - click to seek, editable */}
                  {onTimecodeChange ? (
                    <input
                      type="text"
                      value={segment.startTime.slice(0, 8)}
                      onChange={(e) => {
                        const newStart = e.target.value + segment.startTime.slice(8)
                        onTimecodeChange(index, newStart, segment.endTime)
                      }}
                      onFocus={() => onSeek(startSeconds)}
                      className="w-[62px] shrink-0 rounded border border-transparent bg-transparent px-1 font-mono text-[10px] text-muted-foreground hover:border-border focus:border-primary focus:outline-none"
                      title="คลิกเพื่อข้ามไป"
                    />
                  ) : (
                    <button
                      onClick={() => onSeek(startSeconds)}
                      className="shrink-0 font-mono text-[10px] text-muted-foreground hover:text-primary"
                      title="ข้ามไป"
                    >
                      {segment.startTime.slice(0, 8)}
                    </button>
                  )}

                  {/* End Time - click to seek, editable */}
                  {onTimecodeChange && (
                    <>
                      <span className="text-[10px] text-muted-foreground/50">→</span>
                      <input
                        type="text"
                        value={segment.endTime.slice(0, 8)}
                        onChange={(e) => {
                          const newEnd = e.target.value + segment.endTime.slice(8)
                          onTimecodeChange(index, segment.startTime, newEnd)
                        }}
                        onFocus={() => onSeek(timestampToSeconds(segment.endTime))}
                        className="w-[62px] shrink-0 rounded border border-transparent bg-transparent px-1 font-mono text-[10px] text-muted-foreground hover:border-border focus:border-primary focus:outline-none"
                        title="คลิกเพื่อข้ามไป"
                      />
                    </>
                  )}

                  {/* Text input - border bottom only, click to seek */}
                  <Input
                    value={segment.text.replace(/\n/g, ' ')}
                    onChange={(e) => onSegmentChange(index, e.target.value)}
                    onFocus={() => onSeek(startSeconds)}
                    className={cn(
                      'h-7 flex-1 rounded-none border-0 border-b bg-transparent px-1 text-sm shadow-none focus-visible:ring-0 focus-visible:border-primary',
                      isActive && 'border-primary font-medium'
                    )}
                    placeholder="(ว่าง)"
                  />
                </div>
              )
            })
          )}
        </div>
      </ScrollArea>

      {/* Footer with segment count */}
      <div className="border-t bg-muted/30 px-4 py-2">
        <div className="flex items-center justify-between text-xs text-muted-foreground">
          <span>{segments.length} segments</span>
          {activeIndex >= 0 && <span>#{activeIndex + 1}</span>}
        </div>
      </div>
    </div>
  )
}
