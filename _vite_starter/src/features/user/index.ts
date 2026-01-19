// Barrel exports for user feature
export { UserProfile } from './components/UserProfile'
export { UserProfilePage } from './pages/UserProfilePage'
export { useUserList, useUserById, useUserProfile, useUpdateProfile, userKeys } from './hooks'
export { userService } from './service'
export type {
  UserProfile as UserProfileType,
  UserListItem,
  UpdateProfilePayload,
  UserListParams,
} from './types'
