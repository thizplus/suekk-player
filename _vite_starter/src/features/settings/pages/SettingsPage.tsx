import { useState } from 'react'
import { RefreshCw, Save, RotateCcw, History, Lock, Database, FileCode2, AlertCircle, Check } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { Skeleton } from '@/components/ui/skeleton'
import { Badge } from '@/components/ui/badge'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { toast } from 'sonner'
import {
  useAllSettings,
  useUpdateSettings,
  useResetSettings,
  useReloadCache,
  useAuditLogs,
} from '../hooks'
import { SETTING_CATEGORIES, getCategoryLabel, type SettingResponse } from '../types'

// Thai labels for setting keys
const SETTING_LABELS: Record<string, string> = {
  // General
  'site_title': 'ชื่อเว็บไซต์',
  'site_description': 'คำอธิบายเว็บไซต์',
  'max_upload_size': 'ขนาดไฟล์สูงสุด (GB)',
  // Transcoding
  'default_qualities': 'ความละเอียดที่ต้องการแปลง',
  'auto_queue': 'เข้าคิวอัตโนมัติหลังอัปโหลด',
  'max_queue_size': 'จำนวน Jobs สูงสุดในคิว (0 = ไม่จำกัด)',
  // Alert
  'enabled': 'เปิดใช้งานการแจ้งเตือน',
  'telegram_bot_token': 'Telegram Bot Token',
  'telegram_chat_id': 'Telegram Chat ID',
  'on_transcode_complete': 'แจ้งเตือนเมื่อแปลงไฟล์สำเร็จ',
  'on_transcode_fail': 'แจ้งเตือนเมื่อแปลงไฟล์ล้มเหลว',
  'on_worker_offline': 'แจ้งเตือนเมื่อ Worker ออฟไลน์',
}

function getSettingLabel(key: string): string {
  return SETTING_LABELS[key] || key
}

// Available quality options
const QUALITY_OPTIONS = [
  { value: '1080p', label: '1080p (Full HD)' },
  { value: '720p', label: '720p (HD)' },
  { value: '480p', label: '480p (SD)' },
  { value: '360p', label: '360p' },
] as const

// Quality Multi-Select Component
function QualitySelector({
  value,
  onChange,
  disabled,
}: {
  value: string
  onChange: (value: string) => void
  disabled?: boolean
}) {
  // Parse comma-separated string to array
  const selectedQualities = value ? value.split(',').map((q) => q.trim()).filter(Boolean) : []

  const handleToggle = (quality: string) => {
    if (disabled) return

    let newQualities: string[]
    if (selectedQualities.includes(quality)) {
      // Remove if already selected (but keep at least one)
      if (selectedQualities.length > 1) {
        newQualities = selectedQualities.filter((q) => q !== quality)
      } else {
        return // Don't allow empty selection
      }
    } else {
      // Add if not selected
      newQualities = [...selectedQualities, quality]
    }

    // Sort by quality order (1080p first, then 720p, etc.)
    const sortedQualities = QUALITY_OPTIONS
      .map((opt) => opt.value)
      .filter((q) => newQualities.includes(q))

    onChange(sortedQualities.join(','))
  }

  return (
    <div className="flex flex-wrap gap-2">
      {QUALITY_OPTIONS.map((option) => {
        const isSelected = selectedQualities.includes(option.value)
        return (
          <button
            key={option.value}
            type="button"
            onClick={() => handleToggle(option.value)}
            disabled={disabled}
            className={`
              inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium
              transition-colors border
              ${isSelected
                ? 'bg-primary text-primary-foreground border-primary'
                : 'bg-background text-muted-foreground border-input hover:bg-muted'
              }
              ${disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
            `}
          >
            {isSelected && <Check className="h-3.5 w-3.5" />}
            {option.value}
          </button>
        )
      })}
    </div>
  )
}

// Helper to get source icon
function SourceBadge({ source, isLocked }: { source: string; isLocked: boolean }) {
  if (isLocked) {
    return (
      <Badge variant="outline" className="gap-1 status-warning text-xs">
        <Lock className="h-3 w-3" />
        ล็อค (ENV)
      </Badge>
    )
  }

  if (source === 'database') {
    return (
      <Badge variant="outline" className="gap-1 text-xs">
        <Database className="h-3 w-3" />
        บันทึกแล้ว
      </Badge>
    )
  }

  return (
    <Badge variant="secondary" className="gap-1 text-xs">
      <FileCode2 className="h-3 w-3" />
      ค่าเริ่มต้น
    </Badge>
  )
}

// Setting Item Component
function SettingItem({
  setting,
  value,
  onChange,
  disabled,
}: {
  setting: SettingResponse
  value: string
  onChange: (value: string) => void
  disabled?: boolean
}) {
  const isBoolean = setting.value_type === 'boolean'
  const isNumber = setting.value_type === 'number'
  const isSecret = setting.is_secret
  const isQualitySelector = setting.key === 'default_qualities'

  // Quality selector gets full width layout
  if (isQualitySelector) {
    return (
      <div className="py-3 border-b last:border-b-0 space-y-2">
        <div className="flex items-center gap-2">
          <span className="font-medium text-sm">{getSettingLabel(setting.key)}</span>
          <SourceBadge source={setting.source} isLocked={setting.is_locked} />
        </div>
        <QualitySelector
          value={value}
          onChange={onChange}
          disabled={disabled || setting.is_locked}
        />
      </div>
    )
  }

  return (
    <div className="grid grid-cols-[1fr,auto] gap-4 items-center py-3 border-b last:border-b-0">
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <span className="font-medium text-sm">{getSettingLabel(setting.key)}</span>
          <SourceBadge source={setting.source} isLocked={setting.is_locked} />
        </div>
      </div>

      <div className="w-48">
        {isBoolean ? (
          <Select
            value={value}
            onValueChange={onChange}
            disabled={disabled || setting.is_locked}
          >
            <SelectTrigger>
              <SelectValue>
                {value === 'true' ? 'เปิด' : 'ปิด'}
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="true">เปิด</SelectItem>
              <SelectItem value="false">ปิด</SelectItem>
            </SelectContent>
          </Select>
        ) : (
          <Input
            type={isNumber ? 'number' : isSecret ? 'password' : 'text'}
            value={value}
            onChange={(e) => onChange(e.target.value)}
            disabled={disabled || setting.is_locked}
          />
        )}
      </div>
    </div>
  )
}

// Category Settings Component
function CategorySettings({
  category,
  settings,
  isLoading,
}: {
  category: string
  settings?: SettingResponse[]
  isLoading: boolean
}) {
  const [localValues, setLocalValues] = useState<Record<string, string>>({})
  const updateSettings = useUpdateSettings()
  const resetSettings = useResetSettings()

  // Initialize local values from settings
  const getSettingValue = (setting: SettingResponse) => {
    return localValues[setting.key] !== undefined ? localValues[setting.key] : setting.value
  }

  const handleValueChange = (key: string, value: string) => {
    setLocalValues((prev) => ({ ...prev, [key]: value }))
  }

  // Get changed settings
  const changedSettings = settings?.filter((s) => {
    const currentValue = localValues[s.key]
    return currentValue !== undefined && currentValue !== s.value && !s.is_locked
  }) ?? []

  const hasChanges = changedSettings.length > 0

  const handleSave = async () => {
    if (!hasChanges) return

    const settingsToUpdate: Record<string, string> = {}
    changedSettings.forEach((s) => {
      settingsToUpdate[s.key] = localValues[s.key]
    })

    try {
      await updateSettings.mutateAsync({
        category,
        data: { settings: settingsToUpdate },
      })
      toast.success('บันทึกสำเร็จ')
      setLocalValues({})
    } catch {
      toast.error('บันทึกไม่สำเร็จ')
    }
  }

  const handleReset = async () => {
    try {
      await resetSettings.mutateAsync({ category, data: {} })
      toast.success('รีเซ็ตสำเร็จ')
      setLocalValues({})
    } catch {
      toast.error('รีเซ็ตไม่สำเร็จ')
    }
  }

  if (isLoading) {
    return (
      <div className="space-y-4">
        {[1, 2, 3, 4].map((i) => (
          <Skeleton key={i} className="h-24 w-full" />
        ))}
      </div>
    )
  }

  if (!settings || settings.length === 0) {
    return (
      <div className="rounded-lg border border-dashed p-6 text-center">
        <AlertCircle className="h-8 w-8 mx-auto text-muted-foreground mb-2" />
        <p className="text-muted-foreground">ไม่พบการตั้งค่าสำหรับหมวดนี้</p>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {/* Settings List */}
      <div className="rounded-lg border p-4">
        {settings.map((setting) => (
          <SettingItem
            key={`${setting.category}-${setting.key}`}
            setting={setting}
            value={getSettingValue(setting)}
            onChange={(value) => handleValueChange(setting.key, value)}
            disabled={updateSettings.isPending || resetSettings.isPending}
          />
        ))}
      </div>

      {/* Actions */}
      {hasChanges && (
        <div className="flex items-center gap-3 pt-4 border-t">
          <Button onClick={handleSave} disabled={updateSettings.isPending}>
            {updateSettings.isPending ? (
              <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
            ) : (
              <Save className="h-4 w-4 mr-2" />
            )}
            บันทึก
          </Button>
          <Button
            variant="outline"
            onClick={() => setLocalValues({})}
            disabled={updateSettings.isPending}
          >
            ยกเลิก
          </Button>
          <span className="text-sm text-muted-foreground">
            ({changedSettings.length} รายการ)
          </span>
        </div>
      )}

      {/* Reset Button */}
      {!hasChanges && (
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button variant="outline" size="sm" className="gap-2">
              <RotateCcw className="h-4 w-4" />
              รีเซ็ตเป็นค่าเริ่มต้น
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>รีเซ็ตการตั้งค่า {getCategoryLabel(category)}</AlertDialogTitle>
              <AlertDialogDescription>
                การตั้งค่าทั้งหมดในหมวด "{getCategoryLabel(category)}" จะถูกรีเซ็ตเป็นค่าเริ่มต้น
                ค่าที่ถูก override โดย ENV จะไม่ถูกเปลี่ยน
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>ยกเลิก</AlertDialogCancel>
              <AlertDialogAction onClick={handleReset}>
                รีเซ็ต
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      )}
    </div>
  )
}

// Audit Logs Sheet
function AuditLogsSheet({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const { data, isLoading } = useAuditLogs({ page: 1, limit: 50 })

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString('th-TH', {
      day: 'numeric',
      month: 'short',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-[calc(100%-2rem)] max-w-lg overflow-y-auto">
        <SheetHeader className="pb-4">
          <SheetTitle className="text-left flex items-center gap-2">
            <History className="h-5 w-5" />
            ประวัติการแก้ไข
          </SheetTitle>
        </SheetHeader>

        <div className="space-y-3 pb-6">
          {isLoading ? (
            <>
              {[1, 2, 3].map((i) => (
                <Skeleton key={i} className="h-20 w-full" />
              ))}
            </>
          ) : data?.data && data.data.length > 0 ? (
            data.data.map((log) => (
              <div key={log.id} className="rounded-lg border border-dashed p-3 space-y-2">
                <div className="flex items-start justify-between">
                  <div>
                    <p className="text-sm font-medium">
                      {getCategoryLabel(log.category)}.{log.key}
                    </p>
                    <p className="text-xs text-muted-foreground">
                      {formatDate(log.changed_at)}
                    </p>
                  </div>
                </div>
                <div className="flex items-center gap-2 text-xs">
                  <span className="px-2 py-0.5 rounded bg-destructive/10 text-destructive font-mono truncate max-w-[120px]">
                    {log.old_value || '(ว่าง)'}
                  </span>
                  <span>→</span>
                  <span className="px-2 py-0.5 rounded status-success font-mono truncate max-w-[120px]">
                    {log.new_value || '(ว่าง)'}
                  </span>
                </div>
                {log.reason && (
                  <p className="text-xs text-muted-foreground italic">
                    "{log.reason}"
                  </p>
                )}
              </div>
            ))
          ) : (
            <div className="rounded-lg border border-dashed p-6 text-center">
              <History className="h-8 w-8 mx-auto text-muted-foreground mb-2" />
              <p className="text-muted-foreground">ยังไม่มีประวัติการแก้ไข</p>
            </div>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}

// Main Settings Page
export function SettingsPage() {
  const [activeTab, setActiveTab] = useState(SETTING_CATEGORIES[0].name)
  const [auditLogsOpen, setAuditLogsOpen] = useState(false)
  const { data: allSettings, isLoading, refetch } = useAllSettings()
  const reloadCache = useReloadCache()

  const handleReloadCache = async () => {
    try {
      await reloadCache.mutateAsync()
      await refetch()
      toast.success('รีโหลด Cache สำเร็จ')
    } catch {
      toast.error('ไม่สามารถรีโหลด Cache ได้')
    }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold">ตั้งค่าระบบ</h1>
          <p className="text-sm text-muted-foreground">
            กำหนดค่าการทำงานของระบบ
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setAuditLogsOpen(true)}
          >
            <History className="h-4 w-4 mr-2" />
            ประวัติ
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={handleReloadCache}
            disabled={reloadCache.isPending || isLoading}
          >
            <RefreshCw className={`h-4 w-4 mr-2 ${reloadCache.isPending || isLoading ? 'animate-spin' : ''}`} />
            รีเฟรช
          </Button>
        </div>
      </div>

      {/* Tabs */}
      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList className="flex-wrap h-auto p-1 gap-1">
          {SETTING_CATEGORIES.map((category) => (
            <TabsTrigger key={category.name} value={category.name} className="text-xs sm:text-sm">
              {category.label}
            </TabsTrigger>
          ))}
        </TabsList>

        {SETTING_CATEGORIES.map((category) => (
          <TabsContent key={category.name} value={category.name} className="mt-6">
            <div className="mb-4">
              <p className="text-sm text-muted-foreground">{category.description}</p>
            </div>
            <CategorySettings
              category={category.name}
              settings={allSettings?.[category.name]}
              isLoading={isLoading}
            />
          </TabsContent>
        ))}
      </Tabs>

      {/* Audit Logs Sheet */}
      <AuditLogsSheet open={auditLogsOpen} onOpenChange={setAuditLogsOpen} />
    </div>
  )
}
