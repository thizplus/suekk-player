import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { categoryService } from './service'
import type { CreateCategoryRequest, UpdateCategoryRequest, ReorderCategoriesRequest } from './types'

// Query Key Factory
export const categoryKeys = {
  all: ['categories'] as const,
  list: () => [...categoryKeys.all, 'list'] as const,
  tree: () => [...categoryKeys.all, 'tree'] as const,
  detail: (id: string) => [...categoryKeys.all, 'detail', id] as const,
}

// ดึงรายการหมวดหมู่ (flat)
export function useCategories() {
  return useQuery({
    queryKey: categoryKeys.list(),
    queryFn: () => categoryService.getList(),
  })
}

// ดึงรายการหมวดหมู่แบบ tree
export function useCategoriesTree() {
  return useQuery({
    queryKey: categoryKeys.tree(),
    queryFn: () => categoryService.getTree(),
  })
}

// ดึงหมวดหมู่ตาม ID
export function useCategory(id: string) {
  return useQuery({
    queryKey: categoryKeys.detail(id),
    queryFn: () => categoryService.getById(id),
    enabled: !!id,
  })
}

// สร้างหมวดหมู่
export function useCreateCategory() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: CreateCategoryRequest) => categoryService.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: categoryKeys.all })
    },
  })
}

// อัปเดตหมวดหมู่
export function useUpdateCategory() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateCategoryRequest }) =>
      categoryService.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: categoryKeys.all })
    },
  })
}

// ลบหมวดหมู่
export function useDeleteCategory() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => categoryService.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: categoryKeys.all })
    },
  })
}

// จัดเรียงหมวดหมู่ใหม่
export function useReorderCategories() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: ReorderCategoriesRequest) => categoryService.reorder(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: categoryKeys.all })
    },
  })
}
