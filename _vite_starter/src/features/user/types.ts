import type { RoleType, StatusType } from '@/constants/enums'

export interface UserProfile {
  id: string
  email: string
  name: string
  role: RoleType
  avatar?: string
  phone?: string
  department?: string
  createdAt: string
  updatedAt: string
}

export interface UserListItem {
  id: string
  email: string
  name: string
  role: RoleType
  status: StatusType
  avatar?: string
  createdAt: string
}

export interface UpdateProfilePayload {
  name?: string
  phone?: string
  avatar?: string
}

export interface UserListParams {
  page?: number
  limit?: number
  search?: string
  role?: RoleType
  status?: StatusType
}
