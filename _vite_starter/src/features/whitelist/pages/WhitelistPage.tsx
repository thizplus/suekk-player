import { useState } from 'react'
import { Plus, Trash2, Loader2, Search, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ProfileList } from '../components/ProfileList'
import { ProfileFormSheet } from '../components/ProfileFormSheet'
import { useWhitelistProfiles, useClearAllCache } from '../hooks'
import { toast } from 'sonner'

export function WhitelistPage() {
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [searchTerm, setSearchTerm] = useState('')
  const { data } = useWhitelistProfiles({ page: 1, limit: 100 })
  const clearCacheMutation = useClearAllCache()

  const handleClearCache = async () => {
    if (!confirm('ต้องการล้าง Cache ทั้งหมดหรือไม่?\nCache จะถูกสร้างใหม่เมื่อมีการเข้าถึงโดเมน')) {
      return
    }

    try {
      const result = await clearCacheMutation.mutateAsync()
      toast.success(`ล้าง Cache สำเร็จ (${result.deletedKeys} keys)`)
    } catch {
      toast.error('ล้าง Cache ไม่สำเร็จ')
    }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">จัดการโดเมน</h1>
          <p className="text-sm text-muted-foreground">
            {data ? `${data.meta.total} โปรไฟล์` : 'กำหนดโดเมนที่อนุญาตให้ embed วิดีโอ'}
          </p>
        </div>
        <div className="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={handleClearCache}
            disabled={clearCacheMutation.isPending}
          >
            {clearCacheMutation.isPending ? (
              <Loader2 className="h-4 w-4 mr-2 animate-spin" />
            ) : (
              <Trash2 className="h-4 w-4 mr-2" />
            )}
            ล้าง Cache
          </Button>
          <Button size="sm" onClick={() => setIsCreateOpen(true)}>
            <Plus className="h-4 w-4 mr-2" />
            สร้างโปรไฟล์
          </Button>
        </div>
      </div>

      {/* Search Filter */}
      <div className="flex items-center gap-3 pb-4 border-b">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="ค้นหาโดเมนหรือชื่อโปรไฟล์..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="pl-9"
          />
        </div>
        {searchTerm && (
          <Button variant="ghost" size="sm" onClick={() => setSearchTerm('')} className="gap-1.5">
            <X className="h-4 w-4" />
            ล้าง
          </Button>
        )}
      </div>

      {/* Profile List */}
      <ProfileList searchTerm={searchTerm} />

      {/* Create Profile Sheet */}
      <ProfileFormSheet
        open={isCreateOpen}
        onOpenChange={setIsCreateOpen}
        mode="create"
      />
    </div>
  )
}
