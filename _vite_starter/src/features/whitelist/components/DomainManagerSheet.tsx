import { useState, useEffect } from 'react'
import { Globe, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { TagInput } from '@/components/ui/tag-input'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { useWhitelistProfile, useAddDomain, useRemoveDomain } from '../hooks'
import type { WhitelistProfile } from '../types'

interface DomainManagerSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  profile?: WhitelistProfile
}

// Domain validation pattern
const domainPattern = /^(\*\.)?[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*$/

export function DomainManagerSheet({ open, onOpenChange, profile }: DomainManagerSheetProps) {
  const [localDomains, setLocalDomains] = useState<string[]>([])
  const [isSaving, setIsSaving] = useState(false)

  // Fetch latest profile data with domains
  const { data: profileData, isLoading } = useWhitelistProfile(profile?.id || '')
  const addDomain = useAddDomain()
  const removeDomain = useRemoveDomain()

  const currentProfile = profileData || profile
  const serverDomains = currentProfile?.domains || []

  // Sync local state with server data when sheet opens
  useEffect(() => {
    if (open) {
      const domains = currentProfile?.domains || []
      setLocalDomains(domains.map(d => d.domain))
    }
  }, [open, currentProfile?.id])

  // Validate domain format
  const validateDomain = (value: string): boolean | string => {
    if (!domainPattern.test(value)) {
      return 'รูปแบบ domain ไม่ถูกต้อง'
    }
    return true
  }

  // Transform to lowercase
  const transformDomain = (value: string): string => {
    return value.toLowerCase()
  }

  // Save changes to server
  const handleSave = async () => {
    if (!profile) return

    setIsSaving(true)
    try {
      const serverDomainSet = new Set(serverDomains.map(d => d.domain))
      const localDomainSet = new Set(localDomains)

      // Find domains to add
      const toAdd = localDomains.filter(d => !serverDomainSet.has(d))
      // Find domains to remove
      const toRemove = serverDomains.filter(d => !localDomainSet.has(d.domain))

      // Add new domains
      for (const domain of toAdd) {
        await addDomain.mutateAsync({
          profileId: profile.id,
          data: { domain },
        })
      }

      // Remove deleted domains
      for (const domain of toRemove) {
        await removeDomain.mutateAsync(domain.id)
      }

      toast.success('บันทึก domains สำเร็จ')
      onOpenChange(false)
    } catch (err) {
      toast.error('ไม่สามารถบันทึก domains ได้')
    } finally {
      setIsSaving(false)
    }
  }

  // Check if there are changes
  const hasChanges = () => {
    const serverSet = new Set(serverDomains.map(d => d.domain))
    const localSet = new Set(localDomains)
    if (serverSet.size !== localSet.size) return true
    for (const d of serverSet) {
      if (!localSet.has(d)) return true
    }
    return false
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-full sm:max-w-md flex flex-col">
        <SheetHeader>
          <SheetTitle className="flex items-center gap-2">
            <Globe className="h-5 w-5" />
            จัดการ Domains
          </SheetTitle>
          <SheetDescription>
            {currentProfile?.name} - กำหนด domains ที่อนุญาตให้ embed
          </SheetDescription>
        </SheetHeader>

        <div className="p-4 space-y-4 flex-1 overflow-y-auto relative">
          {/* Loading overlay */}
          {(isLoading || isSaving) && (
            <div className="absolute inset-0 bg-background/80 backdrop-blur-sm z-50 flex items-center justify-center">
              <div className="flex flex-col items-center gap-2">
                <Loader2 className="h-8 w-8 animate-spin text-primary" />
                <p className="text-sm text-muted-foreground">
                  {isSaving ? 'กำลังบันทึก...' : 'กำลังโหลด...'}
                </p>
              </div>
            </div>
          )}

          {/* Domain TagInput */}
          <div className="space-y-2">
            <Label>Domains ({localDomains.length})</Label>
            <TagInput
              value={localDomains}
              onChange={setLocalDomains}
              placeholder="พิมพ์ domain แล้วกด Enter..."
              validate={validateDomain}
              transform={transformDomain}
              disabled={isSaving}
            />
            <p className="text-xs text-muted-foreground">
              พิมพ์ domain แล้วกด Enter เพื่อเพิ่ม, คลิก × เพื่อลบ
            </p>
          </div>

          {/* Examples */}
          <div className="space-y-2 pt-4 border-t">
            <Label className="text-muted-foreground">ตัวอย่าง:</Label>
            <div className="space-y-1 text-xs text-muted-foreground">
              <p><code className="bg-muted px-1 rounded">example.com</code> - เฉพาะ domain นี้</p>
              <p><code className="bg-muted px-1 rounded">*.example.com</code> - ทุก subdomain</p>
              <p><code className="bg-muted px-1 rounded">blog.example.com</code> - เฉพาะ subdomain นี้</p>
            </div>
          </div>
        </div>

        <SheetFooter className="p-4 border-t">
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={isSaving}>
            ยกเลิก
          </Button>
          <Button onClick={handleSave} disabled={isSaving || !hasChanges()}>
            {isSaving && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            บันทึก
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
