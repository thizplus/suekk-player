import { useState } from 'react'
import { User, Mail, Shield, Calendar, AtSign, Key, Loader2, Check } from 'lucide-react'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Separator } from '@/components/ui/separator'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useAuthStore } from '@/features/auth'
import { ROLE_LABELS, type RoleType } from '@/constants/enums'
import { userService } from '../service'
import { toast } from 'sonner'

export function UserProfilePage() {
  const user = useAuthStore((s) => s.user)
  const setUser = useAuthStore((s) => s.setUser)

  // State สำหรับ Set Password form
  const [showPasswordForm, setShowPasswordForm] = useState(false)
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString('th-TH', {
      year: 'numeric',
      month: 'long',
      day: 'numeric',
    })
  }

  // Display name like NavUser
  const displayName = user
    ? `${user.firstName} ${user.lastName}`.trim() || user.username
    : ''

  const initials = displayName
    .split(' ')
    .map((n) => n[0])
    .join('')
    .toUpperCase()
    .slice(0, 2)

  // Handle set password submit
  const handleSetPassword = async (e: React.FormEvent) => {
    e.preventDefault()

    if (newPassword.length < 8) {
      toast.error('รหัสผ่านต้องมีอย่างน้อย 8 ตัวอักษร')
      return
    }

    if (newPassword !== confirmPassword) {
      toast.error('รหัสผ่านไม่ตรงกัน')
      return
    }

    setIsSubmitting(true)
    try {
      await userService.setPassword({ newPassword, confirmPassword })
      toast.success('ตั้งรหัสผ่านสำเร็จ')

      // Update user state to reflect hasPassword = true
      if (user) {
        setUser({ ...user, hasPassword: true })
      }

      setShowPasswordForm(false)
      setNewPassword('')
      setConfirmPassword('')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'ไม่สามารถตั้งรหัสผ่านได้')
    } finally {
      setIsSubmitting(false)
    }
  }

  if (!user) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-2xl font-semibold">โปรไฟล์</h1>
          <p className="text-sm text-muted-foreground">ข้อมูลบัญชีของคุณ</p>
        </div>
        <p className="text-sm text-destructive py-8 text-center">ไม่สามารถโหลดข้อมูลโปรไฟล์ได้</p>
      </div>
    )
  }

  // ตรวจสอบว่าเป็น Google user และยังไม่มี password
  const canSetPassword = user.isGoogleUser && !user.hasPassword

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-semibold">โปรไฟล์</h1>
        <p className="text-sm text-muted-foreground">ข้อมูลบัญชีของคุณ</p>
      </div>

      {/* User Info - NavUser style */}
      <div className="flex items-center gap-3">
        <Avatar className="h-8 w-8 rounded-lg">
          <AvatarImage src={user.avatar} alt={displayName} />
          <AvatarFallback className="rounded-lg">{initials}</AvatarFallback>
        </Avatar>
        <div className="grid flex-1 text-left text-sm leading-tight">
          <span className="truncate font-medium">{displayName}</span>
          <span className="truncate text-xs text-muted-foreground">{user.email}</span>
        </div>
        <span className="text-xs text-muted-foreground">{ROLE_LABELS[user.role as RoleType]}</span>
      </div>

      <Separator />

      {/* User Details - Inline style */}
      <div className="space-y-3">
        <p className="text-sm text-muted-foreground">ข้อมูลส่วนตัว</p>
        <div className="space-y-2">
          <div className="flex items-center gap-3 px-3 py-2.5 rounded-lg border border-dashed leading-none">
            <User className="h-4 w-4 text-muted-foreground shrink-0" />
            <span className="text-sm text-muted-foreground w-24">ชื่อ-นามสกุล</span>
            <span className="text-sm font-medium">{displayName}</span>
          </div>
          <div className="flex items-center gap-3 px-3 py-2.5 rounded-lg border border-dashed leading-none">
            <AtSign className="h-4 w-4 text-muted-foreground shrink-0" />
            <span className="text-sm text-muted-foreground w-24">ชื่อผู้ใช้</span>
            <span className="text-sm font-medium">{user.username}</span>
          </div>
          <div className="flex items-center gap-3 px-3 py-2.5 rounded-lg border border-dashed leading-none">
            <Mail className="h-4 w-4 text-muted-foreground shrink-0" />
            <span className="text-sm text-muted-foreground w-24">อีเมล</span>
            <span className="text-sm font-medium">{user.email}</span>
          </div>
          <div className="flex items-center gap-3 px-3 py-2.5 rounded-lg border border-dashed leading-none">
            <Shield className="h-4 w-4 text-muted-foreground shrink-0" />
            <span className="text-sm text-muted-foreground w-24">บทบาท</span>
            <span className="text-sm font-medium">{ROLE_LABELS[user.role as RoleType]}</span>
          </div>
          <div className="flex items-center gap-3 px-3 py-2.5 rounded-lg border border-dashed leading-none">
            <Calendar className="h-4 w-4 text-muted-foreground shrink-0" />
            <span className="text-sm text-muted-foreground w-24">สมาชิกตั้งแต่</span>
            <span className="text-sm font-medium">{formatDate(user.createdAt)}</span>
          </div>
        </div>
      </div>

      {/* Set Password Section - สำหรับ Google users ที่ยังไม่มี password */}
      {canSetPassword && (
        <>
          <Separator />
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium">ตั้งรหัสผ่าน</p>
                <p className="text-xs text-muted-foreground">
                  ตั้งรหัสผ่านเพื่อใช้ login ด้วย email/password
                </p>
              </div>
              {!showPasswordForm && (
                <Button variant="outline" size="sm" onClick={() => setShowPasswordForm(true)}>
                  <Key className="h-4 w-4 mr-1.5" />
                  ตั้งรหัสผ่าน
                </Button>
              )}
            </div>

            {showPasswordForm && (
              <form onSubmit={handleSetPassword} className="space-y-4 rounded-lg border p-4">
                <div className="space-y-2">
                  <Label htmlFor="newPassword">รหัสผ่านใหม่</Label>
                  <Input
                    id="newPassword"
                    type="password"
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                    placeholder="อย่างน้อย 8 ตัวอักษร"
                    minLength={8}
                    required
                    disabled={isSubmitting}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="confirmPassword">ยืนยันรหัสผ่าน</Label>
                  <Input
                    id="confirmPassword"
                    type="password"
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    placeholder="กรอกรหัสผ่านอีกครั้ง"
                    minLength={8}
                    required
                    disabled={isSubmitting}
                  />
                </div>
                <div className="flex gap-2">
                  <Button type="submit" disabled={isSubmitting}>
                    {isSubmitting ? (
                      <>
                        <Loader2 className="h-4 w-4 mr-1.5 animate-spin" />
                        กำลังบันทึก...
                      </>
                    ) : (
                      <>
                        <Check className="h-4 w-4 mr-1.5" />
                        บันทึก
                      </>
                    )}
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => {
                      setShowPasswordForm(false)
                      setNewPassword('')
                      setConfirmPassword('')
                    }}
                    disabled={isSubmitting}
                  >
                    ยกเลิก
                  </Button>
                </div>
              </form>
            )}
          </div>
        </>
      )}

      {/* แสดงสถานะ password สำหรับ Google users ที่มี password แล้ว */}
      {user.isGoogleUser && user.hasPassword && (
        <>
          <Separator />
          <div className="flex items-center gap-3 px-3 py-2.5 rounded-lg border border-dashed leading-none">
            <Key className="h-4 w-4 text-muted-foreground shrink-0" />
            <span className="text-sm text-muted-foreground w-24">รหัสผ่าน</span>
            <span className="text-sm font-medium text-green-600">ตั้งค่าแล้ว</span>
          </div>
        </>
      )}
    </div>
  )
}
