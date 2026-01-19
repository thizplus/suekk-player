import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { useAuthStore } from '@/features/auth'

export function ProtectedRoute() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const location = useLocation()

  if (!isAuthenticated) {
    // Redirect ไปหน้า login โดยเก็บ path เดิมไว้ใน state
    return <Navigate to="/login" state={{ from: location }} replace />
  }

  return <Outlet />
}
