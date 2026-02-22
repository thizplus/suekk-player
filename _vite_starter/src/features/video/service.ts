import { apiClient, type PaginationMeta } from '@/lib/api-client'
import { VIDEO_ROUTES, TRANSCODING_ROUTES, CONFIG_ROUTES, HLS_ROUTES } from '@/constants/api-routes'
import type {
  Video,
  VideoUploadResponse,
  BatchUploadResponse,
  VideoFilterParams,
  UpdateVideoRequest,
  TranscodingStatus,
  TranscodingStats,
  WorkersResponse,
  DLQVideo,
  UploadLimits,
  GalleryUrlsResponse,
} from './types'

export const videoService = {
  // ดึงรายการวิดีโอ (paginated)
  async getList(params?: VideoFilterParams): Promise<{ data: Video[]; meta: PaginationMeta }> {
    return apiClient.getPaginated<Video>(VIDEO_ROUTES.LIST, { params })
  },

  // ดึงวิดีโอตาม ID
  async getById(id: string): Promise<Video> {
    return apiClient.get<Video>(VIDEO_ROUTES.BY_ID(id))
  },

  // ดึงวิดีโอตาม Code
  async getByCode(code: string): Promise<Video> {
    return apiClient.get<Video>(VIDEO_ROUTES.BY_CODE(code))
  },

  // อัปโหลดวิดีโอใหม่ พร้อม progress tracking
  async upload(
    file: File,
    title: string,
    description?: string,
    categoryId?: string,
    onProgress?: (progress: number) => void
  ): Promise<VideoUploadResponse> {
    const formData = new FormData()
    formData.append('video', file)
    formData.append('title', title)
    if (description) formData.append('description', description)
    if (categoryId) formData.append('category_id', categoryId)

    // ใช้ postWithProgress เพื่อรายงาน % การอัพโหลด
    return apiClient.postWithProgress<VideoUploadResponse>(VIDEO_ROUTES.UPLOAD, formData, onProgress)
  },

  // อัปโหลดหลายไฟล์พร้อมกัน (batch upload)
  async batchUpload(files: File[]): Promise<BatchUploadResponse> {
    const formData = new FormData()
    files.forEach(file => {
      formData.append('videos', file)
    })

    return apiClient.post<BatchUploadResponse>(VIDEO_ROUTES.BATCH_UPLOAD, formData)
  },

  // อัปเดตข้อมูลวิดีโอ
  async update(id: string, data: UpdateVideoRequest): Promise<Video> {
    return apiClient.put<Video>(VIDEO_ROUTES.BY_ID(id), data)
  },

  // ลบวิดีโอ
  async delete(id: string): Promise<void> {
    return apiClient.delete(VIDEO_ROUTES.BY_ID(id))
  },

  // === Transcoding ===

  // เพิ่มวิดีโอเข้าคิว transcoding
  async queueTranscoding(videoId: string): Promise<void> {
    await apiClient.post(TRANSCODING_ROUTES.QUEUE_VIDEO(videoId))
  },

  // ดึงสถานะ transcoding
  async getTranscodingStatus(videoId: string): Promise<TranscodingStatus> {
    return apiClient.get<TranscodingStatus>(TRANSCODING_ROUTES.STATUS(videoId))
  },

  // ดึงสถิติ transcoding
  async getTranscodingStats(): Promise<TranscodingStats> {
    return apiClient.get<TranscodingStats>(TRANSCODING_ROUTES.STATS)
  },

  // === Workers ===

  // ดึงรายการ Workers ทั้งหมด
  async getWorkers(): Promise<WorkersResponse> {
    return apiClient.get<WorkersResponse>(TRANSCODING_ROUTES.WORKERS)
  },

  // === Dead Letter Queue (DLQ) ===

  // ดึงรายการ videos ที่อยู่ใน DLQ
  async getDLQList(params?: { page?: number; limit?: number }): Promise<{ data: DLQVideo[]; meta: PaginationMeta }> {
    return apiClient.getPaginated<DLQVideo>(VIDEO_ROUTES.DLQ_LIST, { params })
  },

  // Retry video จาก DLQ
  async retryDLQ(videoId: string): Promise<{ message: string; video_id: string; code: string }> {
    return apiClient.post(VIDEO_ROUTES.DLQ_RETRY(videoId))
  },

  // ลบ video จาก DLQ
  async deleteDLQ(videoId: string): Promise<void> {
    return apiClient.delete(VIDEO_ROUTES.DLQ_DELETE(videoId))
  },

  // === Config ===

  // ดึง upload limits จาก settings
  async getUploadLimits(): Promise<UploadLimits> {
    return apiClient.get<UploadLimits>(CONFIG_ROUTES.UPLOAD_LIMITS)
  },

  // === Gallery ===

  // สร้าง gallery จาก HLS (สำหรับ video ที่ยังไม่มี gallery)
  async generateGallery(videoId: string): Promise<{ message: string }> {
    return apiClient.post<{ message: string }>(VIDEO_ROUTES.GENERATE_GALLERY(videoId))
  },

  // ดึง presigned URLs สำหรับ gallery images ทั้งหมด (single API call)
  async getGalleryUrls(videoCode: string): Promise<GalleryUrlsResponse> {
    return apiClient.get<GalleryUrlsResponse>(HLS_ROUTES.GALLERY_URLS(videoCode))
  },
}
