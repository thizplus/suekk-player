/**
 * SRT Parser/Generator Utilities
 * แปลง SRT content ↔ SubtitleSegment[]
 */

import type { SubtitleSegment } from '../types'

/**
 * Parse SRT content เป็น SubtitleSegment[]
 * @param content - Raw SRT content
 * @returns Array ของ SubtitleSegment
 */
export function parseSRT(content: string): SubtitleSegment[] {
  const segments: SubtitleSegment[] = []

  // ทำความสะอาด content และแบ่งเป็น blocks
  const normalizedContent = content.replace(/\r\n/g, '\n').replace(/\r/g, '\n')
  const blocks = normalizedContent.split(/\n\n+/).filter((block) => block.trim())

  for (const block of blocks) {
    const lines = block.trim().split('\n')

    // ต้องมีอย่างน้อย 2 บรรทัด: index, timestamp (text อาจว่างก็ได้)
    if (lines.length < 2) continue

    // บรรทัดแรก: index
    const indexStr = lines[0].trim()
    const index = parseInt(indexStr, 10)
    if (isNaN(index)) continue

    // บรรทัดที่สอง: timestamp (00:01:23,456 --> 00:01:25,789)
    const timestampLine = lines[1].trim()
    const timestampMatch = timestampLine.match(
      /^(\d{2}:\d{2}:\d{2}[,\.]\d{3})\s*-->\s*(\d{2}:\d{2}:\d{2}[,\.]\d{3})$/
    )
    if (!timestampMatch) continue

    const startTime = timestampMatch[1].replace('.', ',')
    const endTime = timestampMatch[2].replace('.', ',')

    // บรรทัดที่เหลือ: text (อาจมีหลายบรรทัด)
    const text = lines.slice(2).join('\n').trim()

    segments.push({
      index,
      startTime,
      endTime,
      text,
    })
  }

  return segments
}

/**
 * Generate SRT content จาก SubtitleSegment[]
 * @param segments - Array ของ SubtitleSegment
 * @returns SRT content string
 */
export function generateSRT(segments: SubtitleSegment[]): string {
  return segments
    .map((segment, i) => {
      // Re-index เพื่อให้ต่อเนื่อง
      const index = i + 1
      return `${index}\n${segment.startTime} --> ${segment.endTime}\n${segment.text}`
    })
    .join('\n\n')
}

/**
 * แปลง timestamp string เป็น seconds
 * @param time - "00:01:23,456"
 * @returns seconds (83.456)
 */
export function timestampToSeconds(time: string): number {
  const [timePart, msPart] = time.split(/[,\.]/)
  const [hours, minutes, seconds] = timePart.split(':').map(Number)
  const ms = parseInt(msPart, 10) || 0
  return hours * 3600 + minutes * 60 + seconds + ms / 1000
}

/**
 * แปลง seconds เป็น timestamp string
 * @param seconds - 83.456
 * @returns "00:01:23,456"
 */
export function secondsToTimestamp(seconds: number): string {
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const secs = Math.floor(seconds % 60)
  const ms = Math.round((seconds % 1) * 1000)

  return `${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}:${String(secs).padStart(2, '0')},${String(ms).padStart(3, '0')}`
}

/**
 * หา segment ที่ตรงกับเวลาปัจจุบัน
 * @param segments - Array ของ SubtitleSegment
 * @param currentTime - เวลาปัจจุบัน (seconds)
 * @returns index ของ segment ที่ active หรือ -1 ถ้าไม่มี
 */
export function findActiveSegmentIndex(segments: SubtitleSegment[], currentTime: number): number {
  for (let i = 0; i < segments.length; i++) {
    const startSeconds = timestampToSeconds(segments[i].startTime)
    const endSeconds = timestampToSeconds(segments[i].endTime)

    if (currentTime >= startSeconds && currentTime <= endSeconds) {
      return i
    }
  }
  return -1
}

/**
 * หา segment ที่ใกล้เคียงที่สุด (สำหรับ auto-scroll)
 * @param segments - Array ของ SubtitleSegment
 * @param currentTime - เวลาปัจจุบัน (seconds)
 * @returns index ของ segment ที่ใกล้ที่สุด
 */
export function findNearestSegmentIndex(segments: SubtitleSegment[], currentTime: number): number {
  if (segments.length === 0) return -1

  let nearestIndex = 0
  let minDiff = Infinity

  for (let i = 0; i < segments.length; i++) {
    const startSeconds = timestampToSeconds(segments[i].startTime)
    const diff = Math.abs(currentTime - startSeconds)

    if (diff < minDiff) {
      minDiff = diff
      nearestIndex = i
    }
  }

  return nearestIndex
}

/**
 * Validate SRT content
 * @param content - SRT content string
 * @returns { valid: boolean, error?: string }
 */
export function validateSRT(content: string): { valid: boolean; error?: string } {
  if (!content.trim()) {
    return { valid: false, error: 'Content is empty' }
  }

  const segments = parseSRT(content)

  if (segments.length === 0) {
    return { valid: false, error: 'No valid subtitle segments found' }
  }

  // ตรวจสอบ timestamp ไม่ทับกัน
  for (let i = 1; i < segments.length; i++) {
    const prevEnd = timestampToSeconds(segments[i - 1].endTime)
    const currStart = timestampToSeconds(segments[i].startTime)

    if (currStart < prevEnd - 0.001) {
      // tolerance 1ms
      return {
        valid: false,
        error: `Segment ${i + 1} starts before segment ${i} ends`,
      }
    }
  }

  return { valid: true }
}
