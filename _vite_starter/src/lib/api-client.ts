import axios, { AxiosError, type AxiosRequestConfig } from 'axios'
import { APP_CONFIG } from '@/constants'

// API Response Types ตาม Backend Standard
export interface ApiResponse<T> {
  success: boolean
  data: T
}

export interface ApiErrorInfo {
  code: string
  message: string
  details?: Record<string, string[]> | null
}

export interface ApiErrorResponse {
  success: false
  error: ApiErrorInfo
}

export interface PaginationMeta {
  total: number
  page: number
  limit: number
  totalPages: number
  hasNext: boolean
  hasPrev: boolean
}

export interface PaginatedResponse<T> {
  success: boolean
  data: T[]
  meta: PaginationMeta
}

// Custom error class สำหรับ API errors
export class ApiError extends Error {
  code: string
  details?: Record<string, string[]> | null

  constructor(error: ApiErrorInfo) {
    super(error.message)
    this.name = 'ApiError'
    this.code = error.code
    this.details = error.details
  }
}

// Axios instance (internal use)
const axiosInstance = axios.create({
  baseURL: APP_CONFIG.apiUrl,
  headers: { 'Content-Type': 'application/json' },
})

// Request interceptor - attach token & handle FormData
axiosInstance.interceptors.request.use((config) => {
  // อ่าน token จาก Zustand persist storage
  const authStorage = localStorage.getItem('auth-storage')
  if (authStorage) {
    try {
      const { state } = JSON.parse(authStorage)
      if (state?.token) {
        config.headers.Authorization = `Bearer ${state.token}`
      }
    } catch {
      // Invalid JSON, ignore
    }
  }

  // ถ้าเป็น FormData ให้ลบ Content-Type เพื่อให้ browser ตั้งเองพร้อม boundary
  if (config.data instanceof FormData) {
    delete config.headers['Content-Type']
  }

  return config
})

// Response interceptor - handle standard response & errors
axiosInstance.interceptors.response.use(
  (response) => response,
  (error: AxiosError<ApiErrorResponse>) => {
    // Handle 401 Unauthorized
    if (error.response?.status === 401) {
      // ไม่ redirect ถ้าอยู่ในหน้า embed (เป็น public page)
      const isEmbedPage = window.location.pathname.startsWith('/embed')
      if (!isEmbedPage) {
        localStorage.removeItem('auth-storage')
        window.location.href = '/login'
      }
      return Promise.reject(new ApiError({
        code: 'UNAUTHORIZED',
        message: 'กรุณาเข้าสู่ระบบใหม่',
      }))
    }

    // Extract error from standard response
    if (error.response?.data?.error) {
      return Promise.reject(new ApiError(error.response.data.error))
    }

    // Fallback error
    return Promise.reject(new ApiError({
      code: 'UNKNOWN_ERROR',
      message: error.message || 'เกิดข้อผิดพลาด กรุณาลองใหม่อีกครั้ง',
    }))
  }
)

// ========== Typed API Client ==========
// Wrapper functions ที่ return type ถูกต้องตาม backend response pattern

export const apiClient = {
  /**
   * GET request - สำหรับ single item
   * Backend response: { success: true, data: T }
   */
  async get<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
    const response = await axiosInstance.get<ApiResponse<T>>(url, config)
    return response.data.data
  },

  /**
   * GET request - สำหรับ paginated list
   * Backend response: { success: true, data: T[], meta: {...} }
   */
  async getPaginated<T>(url: string, config?: AxiosRequestConfig): Promise<{ data: T[]; meta: PaginationMeta }> {
    const response = await axiosInstance.get<PaginatedResponse<T>>(url, config)
    return { data: response.data.data, meta: response.data.meta }
  },

  /**
   * POST request - return data
   * Backend response: { success: true, data: T }
   */
  async post<T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> {
    const response = await axiosInstance.post<ApiResponse<T>>(url, data, config)
    return response.data.data
  },

  /**
   * POST request - ไม่มี response data (เช่น logout)
   * Backend response: { success: true }
   */
  async postVoid(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<void> {
    await axiosInstance.post(url, data, config)
  },

  /**
   * PUT request
   * Backend response: { success: true, data: T }
   */
  async put<T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> {
    const response = await axiosInstance.put<ApiResponse<T>>(url, data, config)
    return response.data.data
  },

  /**
   * PATCH request
   * Backend response: { success: true, data: T }
   */
  async patch<T>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T> {
    const response = await axiosInstance.patch<ApiResponse<T>>(url, data, config)
    return response.data.data
  },

  /**
   * DELETE request - no response data
   * Backend response: { success: true } หรือ 204 No Content
   */
  async delete(url: string, config?: AxiosRequestConfig): Promise<void> {
    await axiosInstance.delete(url, config)
  },

  /**
   * DELETE request - with response data
   * Backend response: { success: true, data: T }
   */
  async deleteWithResponse<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
    const response = await axiosInstance.delete<ApiResponse<T>>(url, config)
    return response.data.data
  },

  /**
   * POST request with upload progress tracking
   * สำหรับ file uploads ที่ต้องการแสดง progress
   * @param onUploadProgress - callback รับ progress 0-100
   */
  async postWithProgress<T>(
    url: string,
    data: FormData,
    onUploadProgress?: (progress: number) => void,
    config?: AxiosRequestConfig
  ): Promise<T> {
    const response = await axiosInstance.post<ApiResponse<T>>(url, data, {
      ...config,
      onUploadProgress: (progressEvent) => {
        if (progressEvent.total && onUploadProgress) {
          const progress = Math.round((progressEvent.loaded * 100) / progressEvent.total)
          onUploadProgress(progress)
        }
      },
    })
    return response.data.data
  },
}
