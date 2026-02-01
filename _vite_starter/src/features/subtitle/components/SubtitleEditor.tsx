/**
 * SubtitleEditor - Component สำหรับแก้ไข subtitle
 * แสดง list ของ segments พร้อม highlight ตาม video time
 */

import { useRef, useEffect, useCallback } from 'react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
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
import { Save, RotateCcw, Clock, Play } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { SubtitleSegment } from '../types'
import { timestampToSeconds } from '../utils/srt-parser'

interface SubtitleEditorProps {
  segments: SubtitleSegment[]
  currentTime: number // video time in seconds
  onSegmentChange: (index: number, text: string) => void
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
      <ScrollArea ref={scrollAreaRef} className="flex-1">
        <div className="space-y-2 p-4">
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
                    'rounded-lg border p-3 transition-all',
                    isActive
                      ? 'border-primary bg-primary/5 ring-1 ring-primary/20'
                      : 'border-border hover:border-muted-foreground/30'
                  )}
                >
                  {/* Timestamp row */}
                  <div className="mb-2 flex items-center justify-between">
                    <button
                      onClick={() => onSeek(startSeconds)}
                      className="flex items-center gap-1.5 rounded px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                      title="คลิกเพื่อข้ามไปยังเวลานี้"
                    >
                      <Play className="h-3 w-3" />
                      {segment.startTime}
                    </button>
                    <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                      <Clock className="h-3 w-3" />
                      {segment.endTime}
                    </div>
                  </div>

                  {/* Text input */}
                  <Textarea
                    value={segment.text}
                    onChange={(e) => onSegmentChange(index, e.target.value)}
                    className={cn(
                      'min-h-[60px] resize-none border-0 bg-transparent p-0 text-sm focus-visible:ring-0',
                      isActive && 'font-medium'
                    )}
                    placeholder="(ว่าง)"
                    rows={Math.max(2, segment.text.split('\n').length)}
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
