import { useMutation, useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { authService } from './service'
import { useAuthStore } from './store/auth-store'
import type { LoginCredentials, RegisterCredentials, GoogleCallbackRequest } from './types'

export const authKeys = {
  all: ['auth'] as const,
  me: () => [...authKeys.all, 'me'] as const,
}

export function useLogin() {
  const setAuth = useAuthStore((s) => s.setAuth)
  const navigate = useNavigate()

  return useMutation({
    mutationFn: (credentials: LoginCredentials) => authService.login(credentials),
    onSuccess: (data) => {
      setAuth(data.user, data.token)
      navigate('/dashboard')
    },
  })
}

export function useRegister() {
  const setAuth = useAuthStore((s) => s.setAuth)
  const navigate = useNavigate()

  return useMutation({
    mutationFn: (credentials: RegisterCredentials) => authService.register(credentials),
    onSuccess: (data) => {
      setAuth(data.user, data.token)
      navigate('/dashboard')
    },
  })
}

export function useLogout() {
  const logout = useAuthStore((s) => s.logout)
  const navigate = useNavigate()

  return useMutation({
    mutationFn: () => authService.logout(),
    onSuccess: () => {
      logout()
      navigate('/login')
    },
    onError: () => {
      // ถ้า logout ไม่สำเร็จ ก็ clear local state อยู่ดี
      logout()
      navigate('/login')
    },
  })
}

export function useCurrentUser() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)

  return useQuery({
    queryKey: authKeys.me(),
    queryFn: () => authService.getMe(),
    enabled: isAuthenticated,
  })
}

// Google OAuth - Redirect to Google
export function useGoogleLogin() {
  const handleGoogleLogin = () => {
    const url = authService.getGoogleOAuthURL()
    window.location.href = url
  }

  return {
    mutate: handleGoogleLogin,
    isPending: false,
  }
}

// Google OAuth - Handle callback
export function useGoogleCallback() {
  const setAuth = useAuthStore((s) => s.setAuth)
  const navigate = useNavigate()

  return useMutation({
    mutationFn: (data: GoogleCallbackRequest) => authService.googleCallback(data),
    onSuccess: (data) => {
      setAuth(data.user, data.token)
      navigate('/dashboard')
    },
  })
}
