import { useState } from 'react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

export function RegisterForm({ className, ...props }: React.ComponentProps<'div'>) {
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [isLoading, setIsLoading] = useState<'google' | 'line' | 'form' | null>(null)

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setIsLoading('form')
    // TODO: Implement register
    console.log('Register submitted', { name, email, password, confirmPassword })
  }

  const handleLineRegister = () => {
    setIsLoading('line')
    // TODO: Implement LINE OAuth
    console.log('LINE register clicked')
  }

  const handleGoogleRegister = () => {
    setIsLoading('google')
    // TODO: Implement Google OAuth
    console.log('Google register clicked')
  }

  const isDisabled = isLoading !== null

  return (
    <div className={cn('flex flex-col gap-6', className)} {...props}>
      <div className="flex flex-col items-center gap-2 text-center">
        <h1 className="text-2xl font-bold">สมัครสมาชิก</h1>
        <p className="text-muted-foreground text-balance text-sm">
          กรอกข้อมูลเพื่อสร้างบัญชีใหม่
        </p>
      </div>

      <form className="grid gap-4" onSubmit={handleSubmit}>
        <div className="grid gap-2">
          <Label htmlFor="name">ชื่อ-นามสกุล</Label>
          <Input
            id="name"
            type="text"
            placeholder="สมชาย ใจดี"
            required
            value={name}
            onChange={(e) => setName(e.target.value)}
            disabled={isDisabled}
          />
        </div>
        <div className="grid gap-2">
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
        <div className="grid gap-2">
          <Label htmlFor="password">รหัสผ่าน</Label>
          <Input
            id="password"
            type="password"
            placeholder="••••••••"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            disabled={isDisabled}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="confirmPassword">ยืนยันรหัสผ่าน</Label>
          <Input
            id="confirmPassword"
            type="password"
            placeholder="••••••••"
            required
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            disabled={isDisabled}
          />
        </div>
        <Button type="submit" className="w-full" disabled={isDisabled}>
          {isLoading === 'form' ? 'กำลังสมัครสมาชิก...' : 'สมัครสมาชิก'}
        </Button>
      </form>

      <div className="after:border-border relative text-center text-sm after:absolute after:inset-0 after:top-1/2 after:z-0 after:flex after:items-center after:border-t">
        <span className="bg-background text-muted-foreground relative z-10 px-2">
          หรือสมัครด้วย
        </span>
      </div>

      <div className="flex flex-col gap-4">
        {/* LINE Register Button */}
        <Button
          variant="outline"
          type="button"
          className="w-full border-[#00B900] bg-[#00B900] text-white hover:border-[#00A000] hover:bg-[#00A000]"
          onClick={handleLineRegister}
          disabled={isDisabled}
        >
          {isLoading === 'line' ? (
            <span className="animate-pulse">กำลังดำเนินการ...</span>
          ) : (
            <>
              <svg
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 24 24"
                className="mr-2 h-4 w-4"
                fill="currentColor"
              >
                <path d="M19.365 9.863c.349 0 .63.285.63.631 0 .345-.281.63-.63.63H17.61v1.125h1.755c.349 0 .63.283.63.63 0 .344-.281.629-.63.629h-2.386c-.345 0-.627-.285-.627-.629V8.108c0-.345.282-.63.63-.63h2.386c.349 0 .63.285.63.63 0 .349-.281.63-.63.63H17.61v1.125h1.755zm-3.855 3.016c0 .27-.174.51-.432.596-.064.021-.133.031-.199.031-.211 0-.391-.09-.51-.25l-2.443-3.317v2.94c0 .344-.279.629-.631.629-.346 0-.626-.285-.626-.629V8.108c0-.27.173-.51.43-.595.06-.023.136-.033.194-.033.195 0 .375.104.495.254l2.462 3.33V8.108c0-.345.282-.63.63-.63.345 0 .63.285.63.63v4.771zm-5.741 0c0 .344-.282.629-.631.629-.345 0-.627-.285-.627-.629V8.108c0-.345.282-.63.63-.63.346 0 .628.285.628.63v4.771zm-2.466.629H4.917c-.345 0-.63-.285-.63-.629V8.108c0-.345.285-.63.63-.63.348 0 .63.285.63.63v4.141h1.756c.348 0 .629.283.629.63 0 .344-.282.629-.629.629M24 10.314C24 4.943 18.615.572 12 .572S0 4.943 0 10.314c0 4.811 4.27 8.842 10.035 9.608.391.082.923.258 1.058.59.12.301.079.766.038 1.08l-.164 1.02c-.045.301-.24 1.186 1.049.645 1.291-.539 6.916-4.078 9.436-6.975C23.176 14.393 24 12.458 24 10.314" />
              </svg>
              สมัครด้วย LINE
            </>
          )}
        </Button>

        {/* Google Register Button */}
        <Button
          variant="outline"
          type="button"
          className="w-full"
          onClick={handleGoogleRegister}
          disabled={isDisabled}
        >
          {isLoading === 'google' ? (
            <span className="animate-pulse">กำลังดำเนินการ...</span>
          ) : (
            <>
              <svg
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 24 24"
                className="mr-2 h-4 w-4"
              >
                <path
                  d="M12.48 10.92v3.28h7.84c-.24 1.84-.853 3.187-1.787 4.133-1.147 1.147-2.933 2.4-6.053 2.4-4.827 0-8.6-3.893-8.6-8.72s3.773-8.72 8.6-8.72c2.6 0 4.507 1.027 5.907 2.347l2.307-2.307C18.747 1.44 16.133 0 12.48 0 5.867 0 .307 5.387.307 12s5.56 12 12.173 12c3.573 0 6.267-1.173 8.373-3.36 2.16-2.16 2.84-5.213 2.84-7.667 0-.76-.053-1.467-.173-2.053H12.48z"
                  fill="currentColor"
                />
              </svg>
              สมัครด้วย Google
            </>
          )}
        </Button>
      </div>

      <div className="text-center text-sm">
        มีบัญชีอยู่แล้ว?{' '}
        <a href="/login" className="underline underline-offset-4">
          เข้าสู่ระบบ
        </a>
      </div>
    </div>
  )
}
