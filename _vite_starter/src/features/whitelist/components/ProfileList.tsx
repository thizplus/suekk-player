import { useState } from 'react'
import { MoreVertical, Pencil, Trash2, Globe, Image, Video, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Empty, EmptyMedia, EmptyHeader, EmptyTitle, EmptyDescription, EmptyContent } from '@/components/ui/empty'
import { PROFILE_STATUS_LABELS, PROFILE_STATUS_STYLES } from '@/constants/enums'
import { useWhitelistProfiles, useDeleteProfile } from '../hooks'
import type { WhitelistProfile } from '../types'
import { ProfileFormSheet } from './ProfileFormSheet'

interface ProfileListProps {
  searchTerm?: string
}

export function ProfileList({ searchTerm = '' }: ProfileListProps) {
  const [page] = useState(1)
  const { data, isLoading, error } = useWhitelistProfiles({ page, limit: 100 }) // เพิ่ม limit เพื่อ client-side filter
  const deleteProfile = useDeleteProfile()

  const [editingProfile, setEditingProfile] = useState<WhitelistProfile | null>(null)
  const [deletingProfile, setDeletingProfile] = useState<WhitelistProfile | null>(null)

  const handleDelete = async () => {
    if (!deletingProfile) return

    try {
      await deleteProfile.mutateAsync(deletingProfile.id)
      toast.success('ลบโปรไฟล์สำเร็จ')
      setDeletingProfile(null)
    } catch (err) {
      toast.error('ไม่สามารถลบโปรไฟล์ได้')
    }
  }

  const allProfiles = data?.data || []

  // Client-side filter โดย domain name หรือ profile name
  const profiles = searchTerm
    ? allProfiles.filter((profile) => {
        const term = searchTerm.toLowerCase()
        // ค้นหาใน profile name
        if (profile.name.toLowerCase().includes(term)) return true
        // ค้นหาใน domains
        if (profile.domains?.some(d => d.domain.toLowerCase().includes(term))) return true
        return false
      })
    : allProfiles

  if (isLoading) {
    return (
      <div className="flex justify-center py-12">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  if (error) {
    return (
      <p className="text-sm text-destructive py-8 text-center">เกิดข้อผิดพลาดในการโหลดข้อมูล</p>
    )
  }

  if (profiles.length === 0) {
    // กรณีค้นหาไม่พบ
    if (searchTerm && allProfiles.length > 0) {
      return (
        <Empty className="border">
          <EmptyHeader>
            <EmptyMedia variant="icon">
              <Globe className="h-6 w-6" />
            </EmptyMedia>
            <EmptyTitle>ไม่พบผลลัพธ์</EmptyTitle>
            <EmptyDescription>
              ไม่พบโปรไฟล์ที่มีโดเมนหรือชื่อตรงกับ "{searchTerm}"
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      )
    }

    // กรณียังไม่มีโปรไฟล์เลย
    return (
      <Empty className="border">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <Globe className="h-6 w-6" />
          </EmptyMedia>
          <EmptyTitle>ยังไม่มีโปรไฟล์</EmptyTitle>
          <EmptyDescription>
            สร้างโปรไฟล์แรกเพื่อกำหนดโดเมนที่อนุญาต
          </EmptyDescription>
        </EmptyHeader>
        <EmptyContent>
          <p className="text-sm text-muted-foreground">คลิกปุ่ม "สร้างโปรไฟล์" ด้านบน</p>
        </EmptyContent>
      </Empty>
    )
  }

  return (
    <>
      <div className="space-y-2">
        {profiles.map((profile) => {
          const domainCount = profile.domains?.length || 0

          return (
            <div
              key={profile.id}
              className="flex items-center gap-3 px-3 py-2.5 rounded-lg border border-dashed hover:bg-accent/50 transition-colors cursor-pointer leading-none"
              onClick={() => setEditingProfile(profile)}
            >
              <Globe className="h-4 w-4 text-muted-foreground shrink-0" />

              <div className="flex-1 min-w-0">
                <p className="font-medium truncate">{profile.name}</p>
                {/* แสดงรายชื่อโดเมน */}
                {profile.domains && profile.domains.length > 0 && (
                  <p className="mt-1 text-xs text-muted-foreground truncate">
                    {profile.domains.slice(0, 3).map(d => d.domain).join(', ')}
                    {profile.domains.length > 3 && ` +${profile.domains.length - 3}`}
                  </p>
                )}
                <p className="flex items-center gap-3 mt-1 text-xs text-muted-foreground">
                  <span>{domainCount} โดเมน</span>
                  {profile.watermarkEnabled && (
                    <span className="inline-flex items-center gap-1">
                      <Image className="h-3 w-3" />
                      ลายน้ำ
                    </span>
                  )}
                  {profile.prerollEnabled && (
                    <span className="inline-flex items-center gap-1">
                      <Video className="h-3 w-3" />
                      โฆษณา
                    </span>
                  )}
                  {profile.description && (
                    <span className="truncate max-w-[200px]">{profile.description}</span>
                  )}
                </p>
              </div>

              <Badge className={PROFILE_STATUS_STYLES[String(profile.isActive)]}>
                {PROFILE_STATUS_LABELS[String(profile.isActive)]}
              </Badge>

              <DropdownMenu>
                <DropdownMenuTrigger asChild onClick={(e) => e.stopPropagation()}>
                  <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0">
                    <MoreVertical className="h-4 w-4" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem onClick={() => setEditingProfile(profile)}>
                    <Pencil className="h-4 w-4 mr-2" />
                    แก้ไข
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    className="text-destructive focus:text-destructive"
                    onClick={() => setDeletingProfile(profile)}
                  >
                    <Trash2 className="h-4 w-4 mr-2" />
                    ลบ
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          )
        })}
      </div>

      {/* Edit Profile Sheet */}
      <ProfileFormSheet
        open={!!editingProfile}
        onOpenChange={(open) => !open && setEditingProfile(null)}
        mode="edit"
        profile={editingProfile || undefined}
      />

      {/* Delete Confirmation */}
      <AlertDialog open={!!deletingProfile} onOpenChange={(open) => !open && setDeletingProfile(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>ยืนยันการลบโปรไฟล์</AlertDialogTitle>
            <AlertDialogDescription>
              คุณต้องการลบโปรไฟล์ "{deletingProfile?.name}" หรือไม่?
              การดำเนินการนี้ไม่สามารถย้อนกลับได้ และจะลบโดเมนทั้งหมดในโปรไฟล์นี้ด้วย
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>ยกเลิก</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleDelete}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              ลบโปรไฟล์
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
