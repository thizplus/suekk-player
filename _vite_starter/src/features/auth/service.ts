import { apiClient } from '@/lib/api-client'
import { AUTH_ROUTES } from '@/constants/api-routes'
import type {
  LoginCredentials,
  RegisterCredentials,
  AuthResponse,
  User,
  GoogleCallbackRequest,
} from './types'

export const authService = {
  // Email/Password Login
  async login(credentials: LoginCredentials): Promise<AuthResponse> {
    return apiClient.post<AuthResponse>(AUTH_ROUTES.LOGIN, credentials)
  },

  // Register
  async register(credentials: RegisterCredentials): Promise<AuthResponse> {
    return apiClient.post<AuthResponse>(AUTH_ROUTES.REGISTER, credentials)
  },

  // Logout
  async logout(): Promise<void> {
    await apiClient.postVoid(AUTH_ROUTES.LOGOUT)
  },

  // Get current user
  async getMe(): Promise<User> {
    return apiClient.get<User>(AUTH_ROUTES.ME)
  },

  // Google OAuth - Get redirect URL (backend will redirect to Google)
  getGoogleOAuthURL(): string {
    // Backend redirects directly to Google, so we just return the backend URL
    return `${import.meta.env.VITE_API_URL}${AUTH_ROUTES.GOOGLE_URL}`
  },

  // Google OAuth - Handle callback
  async googleCallback(data: GoogleCallbackRequest): Promise<AuthResponse> {
    return apiClient.post<AuthResponse>(AUTH_ROUTES.GOOGLE_CALLBACK, data)
  },
}
