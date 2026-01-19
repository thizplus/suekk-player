import { create } from 'zustand'
import { videoService } from '@/features/video/service'
import {
  directUpload,
  validateFileForDirectUpload,
  type DirectUploadProgress,
} from '@/lib/direct-upload'

export type UploadStatus = 'uploading' | 'success' | 'error'
export type UploadMethod = 'traditional' | 'direct' // traditional = ผ่าน API, direct = Presigned URL

export interface UploadItem {
  id: string
  file: File
  title: string
  description?: string
  categoryId?: string
  status: UploadStatus
  progress: number
  error?: string
  videoId?: string // ID จาก server หลัง upload สำเร็จ
  method: UploadMethod // วิธีที่ใช้อัปโหลด
  phase?: DirectUploadProgress['phase'] // สถานะปัจจุบัน (สำหรับ direct upload)
}

interface UploadStore {
  uploads: UploadItem[]
  isMinimized: boolean
  useDirectUpload: boolean // ใช้ Direct Upload หรือไม่ (default: true สำหรับไฟล์ใหญ่)

  // Actions
  addUpload: (params: {
    file: File
    title: string
    description?: string
    categoryId?: string
    forceMethod?: UploadMethod // บังคับใช้วิธีนี้
  }) => void
  removeUpload: (id: string) => void
  clearCompleted: () => void
  setMinimized: (minimized: boolean) => void
  setUseDirectUpload: (use: boolean) => void
}

// Threshold สำหรับใช้ Direct Upload (ไฟล์ใหญ่กว่า 100MB จะใช้ Direct Upload อัตโนมัติ)
const DIRECT_UPLOAD_THRESHOLD = 100 * 1024 * 1024 // 100MB

export const useUploadStore = create<UploadStore>((set, get) => ({
  uploads: [],
  isMinimized: false,
  useDirectUpload: true, // เปิด Direct Upload เป็นค่าเริ่มต้น

  addUpload: async (params) => {
    const id = crypto.randomUUID()
    const { useDirectUpload: preferDirectUpload } = get()

    // ตัดสินใจว่าจะใช้วิธีไหน
    let method: UploadMethod = 'traditional'
    if (params.forceMethod) {
      method = params.forceMethod
    } else if (preferDirectUpload && params.file.size >= DIRECT_UPLOAD_THRESHOLD) {
      // ไฟล์ใหญ่กว่า threshold และเปิดใช้ direct upload
      const validation = validateFileForDirectUpload(params.file)
      if (validation.valid) {
        method = 'direct'
      }
    }

    // เพิ่ม upload item ใน store ทันที
    const newItem: UploadItem = {
      id,
      file: params.file,
      title: params.title,
      description: params.description,
      categoryId: params.categoryId,
      status: 'uploading',
      progress: 0,
      method,
      phase: method === 'direct' ? 'preparing' : undefined,
    }

    set((state) => ({
      uploads: [...state.uploads, newItem],
    }))

    // เริ่ม upload ใน background
    try {
      if (method === 'direct') {
        // Direct Upload - ผ่าน Presigned URL
        const result = await directUpload({
          file: params.file,
          title: params.title,
          onProgress: (progress) => {
            set((state) => ({
              uploads: state.uploads.map((item) =>
                item.id === id
                  ? { ...item, progress: progress.percent, phase: progress.phase }
                  : item
              ),
            }))
          },
        })

        // อัพเดทสถานะเป็น success
        set((state) => ({
          uploads: state.uploads.map((item) =>
            item.id === id
              ? { ...item, status: 'success' as const, progress: 100, videoId: result.videoId, phase: undefined }
              : item
          ),
        }))
      } else {
        // Traditional Upload - ผ่าน API
        const response = await videoService.upload(
          params.file,
          params.title,
          params.description,
          params.categoryId,
          // onProgress callback - อัพเดท progress ใน store
          (progress) => {
            set((state) => ({
              uploads: state.uploads.map((item) =>
                item.id === id ? { ...item, progress } : item
              ),
            }))
          }
        )

        // อัพเดทสถานะเป็น success
        set((state) => ({
          uploads: state.uploads.map((item) =>
            item.id === id
              ? { ...item, status: 'success' as const, progress: 100, videoId: response.id }
              : item
          ),
        }))
      }
    } catch (err) {
      // อัพเดทสถานะเป็น error
      const errorMessage = err instanceof Error ? err.message : 'อัปโหลดล้มเหลว'
      set((state) => ({
        uploads: state.uploads.map((item) =>
          item.id === id
            ? { ...item, status: 'error' as const, error: errorMessage, phase: undefined }
            : item
        ),
      }))
    }
  },

  removeUpload: (id) => {
    set((state) => ({
      uploads: state.uploads.filter((item) => item.id !== id),
    }))
  },

  clearCompleted: () => {
    set((state) => ({
      uploads: state.uploads.filter(
        (item) => item.status === 'uploading'
      ),
    }))
  },

  setMinimized: (minimized) => {
    set({ isMinimized: minimized })
  },

  setUseDirectUpload: (use) => {
    set({ useDirectUpload: use })
  },
}))
