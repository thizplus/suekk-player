import { useState } from 'react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useLogin } from '../hooks'
import { toast } from 'sonner'

export function LoginForm({ className, ...props }: React.ComponentProps<'div'>) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const { mutate: login, isPending } = useLogin()

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    login({ email, password }, {
      onError: (err) => {
        toast.error(err.message || 'เข้าสู่ระบบไม่สำเร็จ')
      }
    })
  }

  const isDisabled = isPending

  return (
    <div className={cn('flex flex-col gap-6', className)} {...props}>
      <div className="flex flex-col items-center gap-2 text-center">
        <h1 className="text-2xl font-bold">เข้าสู่ระบบ</h1>
        <p className="text-muted-foreground text-balance text-sm">
          กรอกอีเมลและรหัสผ่านเพื่อเข้าสู่ระบบ
        </p>
      </div>

      <form className="grid gap-6" onSubmit={handleSubmit}>
        <div className="grid gap-3">
          <Label htmlFor="email">อีเมล</Label>
          <Input
            id="email"
            type="email"
            placeholder="m@example.com"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            disabled={isDisabled}
          />
        </div>
        <div className="grid gap-3">
          <div className="flex items-center">
            <Label htmlFor="password">รหัสผ่าน</Label>
            <a href="#" className="ml-auto text-sm underline-offset-2 hover:underline">
              ลืมรหัสผ่าน?
            </a>
          </div>
          <Input
            id="password"
            type="password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            disabled={isDisabled}
          />
        </div>
        <Button type="submit" className="w-full" disabled={isDisabled}>
          {isPending ? 'กำลังเข้าสู่ระบบ...' : 'เข้าสู่ระบบ'}
        </Button>
      </form>

      <div className="text-center text-sm">
        ยังไม่มีบัญชี?{' '}
        <a href="/register" className="underline underline-offset-4">
          สมัครสมาชิก
        </a>
      </div>
    </div>
  )
}
