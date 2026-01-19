import { apiClient } from '@/lib/api-client'
import { CATEGORY_ROUTES } from '@/constants/api-routes'
import type { Category, CategoryListResponse, CreateCategoryRequest, UpdateCategoryRequest, ReorderCategoriesRequest } from './types'

export const categoryService = {
  // ดึงรายการหมวดหมู่ทั้งหมด (flat)
  async getList(): Promise<Category[]> {
    const response = await apiClient.get<CategoryListResponse>(CATEGORY_ROUTES.LIST)
    return response.categories
  },

  // ดึงรายการหมวดหมู่แบบ tree
  async getTree(): Promise<Category[]> {
    const response = await apiClient.get<CategoryListResponse>(CATEGORY_ROUTES.TREE)
    return response.categories
  },

  // ดึงหมวดหมู่ตาม ID
  async getById(id: string): Promise<Category> {
    return apiClient.get<Category>(CATEGORY_ROUTES.BY_ID(id))
  },

  // สร้างหมวดหมู่ใหม่
  async create(data: CreateCategoryRequest): Promise<Category> {
    return apiClient.post<Category>(CATEGORY_ROUTES.LIST, data)
  },

  // อัปเดตหมวดหมู่
  async update(id: string, data: UpdateCategoryRequest): Promise<Category> {
    return apiClient.put<Category>(CATEGORY_ROUTES.BY_ID(id), data)
  },

  // ลบหมวดหมู่
  async delete(id: string): Promise<void> {
    return apiClient.delete(CATEGORY_ROUTES.BY_ID(id))
  },

  // จัดเรียงหมวดหมู่ใหม่
  async reorder(data: ReorderCategoriesRequest): Promise<void> {
    await apiClient.put(CATEGORY_ROUTES.REORDER, data)
  },
}
