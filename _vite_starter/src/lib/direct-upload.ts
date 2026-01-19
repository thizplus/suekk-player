/**
 * Direct Upload Library
 * อัปโหลดไฟล์ตรงไป S3 ผ่าน Presigned URL โดยไม่ผ่าน API Server
 *
 * Flow:
 * 1. Frontend ขอ presigned URLs จาก API
 * 2. Frontend อัปโหลดแต่ละ part ตรงไป S3
 * 3. Frontend แจ้ง API ให้รวม parts
 */

import { apiClient } from './api-client'
import { DIRECT_UPLOAD_ROUTES } from '@/constants/api-routes'

// ═══════════════════════════════════════════════════════════════════════════════
// Types
// ═══════════════════════════════════════════════════════════════════════════════

export interface PartURLInfo {
  partNumber: number
  url: string
}

export interface InitDirectUploadResponse {
  uploadId: string
  videoCode: string
  path: string
  partSize: number
  totalParts: number
  presignedUrls: PartURLInfo[]
  expiresIn: number
  // Note: videoId ไม่มีแล้ว เพราะ video จะถูกสร้างตอน complete เท่านั้น
}

export interface CompletedPart {
  partNumber: number
  etag: string
}

export interface CompleteDirectUploadResponse {
  videoId: string
  videoCode: string
  title: string
  status: string
  autoEnqueued: boolean
}

export interface DirectUploadProgress {
  phase: 'preparing' | 'uploading' | 'completing'
  percent: number
  uploadedParts: number
  totalParts: number
  uploadedBytes: number
  totalBytes: number
}

export interface DirectUploadOptions {
  file: File
  title?: string
  onProgress?: (progress: DirectUploadProgress) => void
  maxConcurrency?: number // จำนวน parts ที่อัปโหลดพร้อมกัน (default: 3)
  abortSignal?: AbortSignal
}

export interface DirectUploadResult {
  videoId: string
  videoCode: string
  title: string
  autoEnqueued: boolean
}

// ═══════════════════════════════════════════════════════════════════════════════
// Main Function
// ═══════════════════════════════════════════════════════════════════════════════

/**
 * อัปโหลดไฟล์ใหญ่ตรงไป S3 ผ่าน Presigned URL
 * รองรับ multipart upload, parallel upload, และ progress tracking
 */
export async function directUpload(options: DirectUploadOptions): Promise<DirectUploadResult> {
  const { file, title, onProgress, maxConcurrency = 3, abortSignal } = options

  // Phase 1: Preparing - ขอ presigned URLs จาก API
  onProgress?.({
    phase: 'preparing',
    percent: 0,
    uploadedParts: 0,
    totalParts: 0,
    uploadedBytes: 0,
    totalBytes: file.size,
  })

  const initResponse = await apiClient.post<InitDirectUploadResponse>(DIRECT_UPLOAD_ROUTES.INIT, {
    filename: file.name,
    size: file.size,
    contentType: file.type || 'application/octet-stream',
    title: title || file.name.replace(/\.[^/.]+$/, ''), // ตัด extension
  })

  const { uploadId, videoCode, path, partSize, totalParts, presignedUrls } = initResponse
  const uploadTitle = title || file.name.replace(/\.[^/.]+$/, '')

  // Phase 2: Uploading - อัปโหลดแต่ละ part
  const completedParts: CompletedPart[] = []
  let uploadedBytes = 0
  let uploadedPartsCount = 0

  // ใช้ semaphore pattern สำหรับจำกัด concurrency
  const queue = [...presignedUrls]
  const inFlight: Promise<void>[] = []

  const uploadPart = async (partInfo: PartURLInfo): Promise<void> => {
    // Check abort signal
    if (abortSignal?.aborted) {
      throw new DOMException('Upload aborted', 'AbortError')
    }

    const partIndex = partInfo.partNumber - 1
    const start = partIndex * partSize
    const end = Math.min(start + partSize, file.size)
    const blob = file.slice(start, end)

    // Upload to S3 directly
    const response = await fetch(partInfo.url, {
      method: 'PUT',
      body: blob,
      signal: abortSignal,
    })

    if (!response.ok) {
      throw new Error(`Failed to upload part ${partInfo.partNumber}: ${response.statusText}`)
    }

    // Get ETag from response header
    const etag = response.headers.get('ETag') || ''

    completedParts.push({
      partNumber: partInfo.partNumber,
      etag: etag.replace(/"/g, ''), // Remove quotes
    })

    uploadedBytes += blob.size
    uploadedPartsCount++

    onProgress?.({
      phase: 'uploading',
      percent: Math.round((uploadedBytes / file.size) * 100),
      uploadedParts: uploadedPartsCount,
      totalParts,
      uploadedBytes,
      totalBytes: file.size,
    })
  }

  // Process queue with limited concurrency
  while (queue.length > 0 || inFlight.length > 0) {
    // Check abort signal
    if (abortSignal?.aborted) {
      throw new DOMException('Upload aborted', 'AbortError')
    }

    // Start new uploads up to maxConcurrency
    while (queue.length > 0 && inFlight.length < maxConcurrency) {
      const partInfo = queue.shift()!
      const promise = uploadPart(partInfo).then(() => {
        // Remove from inFlight when done
        const index = inFlight.indexOf(promise)
        if (index > -1) inFlight.splice(index, 1)
      })
      inFlight.push(promise)
    }

    // Wait for at least one to complete
    if (inFlight.length > 0) {
      await Promise.race(inFlight)
    }
  }

  // Wait for all remaining uploads
  await Promise.all(inFlight)

  // Sort completed parts by part number
  completedParts.sort((a, b) => a.partNumber - b.partNumber)

  // Phase 3: Completing - แจ้ง API ให้รวม parts
  onProgress?.({
    phase: 'completing',
    percent: 100,
    uploadedParts: totalParts,
    totalParts,
    uploadedBytes: file.size,
    totalBytes: file.size,
  })

  const completeResponse = await apiClient.post<CompleteDirectUploadResponse>(DIRECT_UPLOAD_ROUTES.COMPLETE, {
    uploadId,
    videoCode,
    path,
    filename: file.name,
    title: uploadTitle,
    parts: completedParts,
  })

  return {
    videoId: completeResponse.videoId,
    videoCode: completeResponse.videoCode,
    title: completeResponse.title,
    autoEnqueued: completeResponse.autoEnqueued,
  }
}

/**
 * ยกเลิก direct upload ที่ค้าง
 */
export async function abortDirectUpload(uploadId: string, path: string): Promise<void> {
  await apiClient.deleteWithResponse(DIRECT_UPLOAD_ROUTES.ABORT, {
    data: { uploadId, path },
  })
}

// ═══════════════════════════════════════════════════════════════════════════════
// Helper: Format bytes for display
// ═══════════════════════════════════════════════════════════════════════════════

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 Bytes'
  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

// ═══════════════════════════════════════════════════════════════════════════════
// Constants (Default values - ใช้เมื่อไม่ได้ส่ง limits มา)
// ═══════════════════════════════════════════════════════════════════════════════

export const DIRECT_UPLOAD_DEFAULTS = {
  MAX_FILE_SIZE: 10 * 1024 * 1024 * 1024, // 10GB
  PART_SIZE: 64 * 1024 * 1024, // 64MB
  DEFAULT_CONCURRENCY: 3,
  ALLOWED_TYPES: ['video/mp4', 'video/x-matroska', 'video/x-msvideo', 'video/quicktime', 'video/webm', 'video/MP2T', 'video/mp2t', 'video/vnd.dlna.mpeg-tts'],
}

// Alias for backward compatibility
export const DIRECT_UPLOAD_LIMITS = DIRECT_UPLOAD_DEFAULTS

export interface UploadLimitsConfig {
  maxFileSize?: number      // bytes
  allowedTypes?: string[]
}

/**
 * ตรวจสอบว่าไฟล์ใช้ได้กับ direct upload หรือไม่
 * @param file - ไฟล์ที่จะตรวจสอบ
 * @param limits - ค่า limits จาก settings (optional)
 */
export function validateFileForDirectUpload(
  file: File,
  limits?: UploadLimitsConfig
): { valid: boolean; error?: string } {
  const maxFileSize = limits?.maxFileSize ?? DIRECT_UPLOAD_DEFAULTS.MAX_FILE_SIZE
  const allowedTypes = limits?.allowedTypes ?? DIRECT_UPLOAD_DEFAULTS.ALLOWED_TYPES

  if (file.size > maxFileSize) {
    return {
      valid: false,
      error: `ไฟล์ใหญ่เกินไป (สูงสุด ${formatBytes(maxFileSize)})`,
    }
  }

  if (file.size === 0) {
    return { valid: false, error: 'ไฟล์ว่างเปล่า' }
  }

  // ตรวจสอบ type (อนุญาตถ้าไม่มี type หรือเป็น .ts/.mts เพราะบาง OS อาจไม่ส่ง MIME type ที่ถูกต้อง)
  const isTsFile = file.name.toLowerCase().endsWith('.ts') || file.name.toLowerCase().endsWith('.mts')
  if (file.type && !allowedTypes.includes(file.type) && !isTsFile) {
    return {
      valid: false,
      error: 'ประเภทไฟล์ไม่รองรับ (รองรับ mp4, mkv, avi, mov, webm, ts)',
    }
  }

  return { valid: true }
}
