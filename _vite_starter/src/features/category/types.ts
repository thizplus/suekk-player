// Category Types
export interface Category {
  id: string
  name: string
  slug: string
  parentId: string | null
  sortOrder: number
  videoCount: number
  createdAt: string
  children?: Category[]
}

// Backend response wrapper
export interface CategoryListResponse {
  categories: Category[]
}

export interface CreateCategoryRequest {
  name: string
  slug: string
  parentId?: string
}

export interface UpdateCategoryRequest {
  name?: string
  slug?: string
  parentId?: string | null
  sortOrder?: number
}

export interface CategoryOrderItem {
  id: string
  parentId: string | null
  sortOrder: number
}

export interface ReorderCategoriesRequest {
  categories: CategoryOrderItem[]
}
