import { User, Mail, Shield, Calendar, AtSign } from 'lucide-react'
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Separator } from '@/components/ui/separator'
import { useAuthStore } from '@/features/auth'
import { ROLE_LABELS, type RoleType } from '@/constants/enums'

export function UserProfilePage() {
  const user = useAuthStore((s) => s.user)

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
    </div>
  )
}
