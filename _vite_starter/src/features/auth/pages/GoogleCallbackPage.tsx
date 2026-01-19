import { useEffect, useRef } from 'react'
import { useSearchParams, useNavigate } from 'react-router-dom'
import { Loader2 } from 'lucide-react'
import { useAuthStore } from '../store/auth-store'
import { authService } from '../service'
import { toast } from 'sonner'

export function GoogleCallbackPage() {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const setAuth = useAuthStore((s) => s.setAuth)
  const hasRun = useRef(false)

  useEffect(() => {
    // ป้องกัน StrictMode รัน 2 ครั้ง
    if (hasRun.current) return
    hasRun.current = true

    const token = searchParams.get('token')
    const errorParam = searchParams.get('error')

    if (errorParam) {
      toast.error('การเข้าสู่ระบบถูกยกเลิก: ' + errorParam)
      navigate('/login')
      return
    }

    if (token) {
      // บันทึก token ก่อน แล้วดึงข้อมูล user
      localStorage.setItem('auth-storage', JSON.stringify({ state: { token, isAuthenticated: true } }))

      // ดึงข้อมูล user จาก /auth/me
      authService.getMe()
        .then((user) => {
          setAuth(user, token)
          toast.success('เข้าสู่ระบบสำเร็จ')
          navigate('/dashboard')
        })
        .catch(() => {
          toast.error('ไม่สามารถดึงข้อมูลผู้ใช้ได้')
          navigate('/login')
        })
    } else {
      toast.error('ไม่พบ token')
      navigate('/login')
    }
  }, [searchParams, navigate, setAuth])

  return (
    <div className="min-h-screen flex flex-col items-center justify-center">
      <Loader2 className="size-12 animate-spin text-primary mb-4" />
      <p className="text-lg font-medium">กำลังเข้าสู่ระบบ...</p>
      <p className="text-sm text-muted-foreground mt-2">กรุณารอสักครู่</p>
    </div>
  )
}
