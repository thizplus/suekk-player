import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Loader2, Plus, GripVertical, Trash2 } from 'lucide-react'
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Slider } from '@/components/ui/slider'
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Separator } from '@/components/ui/separator'
import { TagInput } from '@/components/ui/tag-input'
import { WATERMARK_POSITION, WATERMARK_POSITION_LABELS } from '@/constants/enums'
import { useCreateProfile, useUpdateProfile, useAddDomain, useRemoveDomain, whitelistKeys } from '../hooks'
import { whitelistService } from '../service'
import type { WhitelistProfile, CreateWhitelistProfileRequest, WatermarkPosition } from '../types'

interface ProfileFormSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  mode: 'create' | 'edit'
  profile?: WhitelistProfile
}

interface FormData {
  name: string
  description: string
  isActive: boolean
  // Thumbnail (แสดงก่อนกด play)
  thumbnailUrl: string
  // Watermark
  watermarkEnabled: boolean
  watermarkUrl: string
  watermarkPosition: WatermarkPosition
  watermarkOpacity: number
  watermarkSize: number
  watermarkOffsetY: number
}

// Local preroll item for managing state
interface LocalPrerollItem {
  id: string // temp id for new items, real id for existing
  type: 'video' | 'image'
  url: string
  duration: number // ระยะเวลา (สำหรับ image)
  skipAfter: number
  clickUrl: string
  clickText: string
  title: string
  isNew?: boolean // flag for new items that need to be created
}

// Domain validation pattern
const domainPattern = /^(\*\.)?[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*$/

// Sortable preroll item component
function SortablePrerollItem({
  item,
  onUpdate,
  onDelete,
  disabled,
}: {
  item: LocalPrerollItem
  onUpdate: (id: string, updates: Partial<LocalPrerollItem>) => void
  onDelete: (id: string) => void
  disabled: boolean
}) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: item.id })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
  }

  const [expanded, setExpanded] = useState(false)

  return (
    <div
      ref={setNodeRef}
      style={style}
      className="p-3 bg-muted/50 rounded-lg space-y-2"
    >
      {/* Main Row */}
      <div className="flex items-center gap-2">
        <button
          type="button"
          className="cursor-grab touch-none text-muted-foreground hover:text-foreground"
          {...attributes}
          {...listeners}
        >
          <GripVertical className="h-4 w-4" />
        </button>

        {/* Type Selector */}
        <Select
          value={item.type}
          onValueChange={(value: 'video' | 'image') => onUpdate(item.id, { type: value })}
          disabled={disabled}
        >
          <SelectTrigger className="w-24">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="video">วิดีโอ</SelectItem>
            <SelectItem value="image">รูปภาพ</SelectItem>
          </SelectContent>
        </Select>

        {/* URL */}
        <Input
          value={item.url}
          onChange={(e) => onUpdate(item.id, { url: e.target.value })}
          placeholder={item.type === 'video' ? 'URL วิดีโอ' : 'URL รูปภาพ'}
          className="flex-1"
          disabled={disabled}
        />

        {/* Duration (for image) or Skip After (for video) */}
        {item.type === 'image' ? (
          <div className="flex items-center gap-1 w-20">
            <Input
              type="number"
              min={1}
              max={60}
              value={item.duration}
              onChange={(e) => onUpdate(item.id, { duration: parseInt(e.target.value) || 10 })}
              className="w-14"
              disabled={disabled}
            />
            <span className="text-xs text-muted-foreground">วิ</span>
          </div>
        ) : (
          <div className="flex items-center gap-1 w-20">
            <Input
              type="number"
              min={0}
              max={120}
              value={item.skipAfter}
              onChange={(e) => onUpdate(item.id, { skipAfter: parseInt(e.target.value) || 0 })}
              className="w-14"
              disabled={disabled}
            />
            <span className="text-xs text-muted-foreground">วิ</span>
          </div>
        )}

        {/* Expand/Collapse */}
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={() => setExpanded(!expanded)}
          disabled={disabled}
        >
          {expanded ? 'ย่อ' : 'เพิ่มเติม'}
        </Button>

        {/* Delete */}
        <Button
          type="button"
          variant="ghost"
          size="icon"
          onClick={() => onDelete(item.id)}
          disabled={disabled}
          className="text-destructive hover:text-destructive"
        >
          <Trash2 className="h-4 w-4" />
        </Button>
      </div>

      {/* Expanded Options */}
      {expanded && (
        <div className="grid grid-cols-2 gap-2 pt-2 border-t border-border/50">
          {/* Skip After (for image) */}
          {item.type === 'image' && (
            <div className="space-y-1">
              <Label className="text-xs">ข้ามได้หลัง (วินาที)</Label>
              <Input
                type="number"
                min={0}
                max={60}
                value={item.skipAfter}
                onChange={(e) => onUpdate(item.id, { skipAfter: parseInt(e.target.value) || 0 })}
                placeholder="0 = ห้ามข้าม"
                disabled={disabled}
              />
            </div>
          )}

          {/* Title */}
          <div className="space-y-1">
            <Label className="text-xs">ชื่อ/ผู้สนับสนุน</Label>
            <Input
              value={item.title}
              onChange={(e) => onUpdate(item.id, { title: e.target.value })}
              placeholder="เช่น Sponsored by..."
              disabled={disabled}
            />
          </div>

          {/* Click URL */}
          <div className="space-y-1">
            <Label className="text-xs">ลิงก์เมื่อคลิก</Label>
            <Input
              value={item.clickUrl}
              onChange={(e) => onUpdate(item.id, { clickUrl: e.target.value })}
              placeholder="https://..."
              disabled={disabled}
            />
          </div>

          {/* Click Text */}
          <div className="space-y-1">
            <Label className="text-xs">ข้อความปุ่ม</Label>
            <Input
              value={item.clickText}
              onChange={(e) => onUpdate(item.id, { clickText: e.target.value })}
              placeholder="ดูรายละเอียด"
              disabled={disabled}
            />
          </div>
        </div>
      )}
    </div>
  )
}

export function ProfileFormSheet({ open, onOpenChange, mode, profile }: ProfileFormSheetProps) {
  const queryClient = useQueryClient()
  const createProfile = useCreateProfile()
  const updateProfile = useUpdateProfile()
  const addDomain = useAddDomain()
  const removeDomain = useRemoveDomain()

  const [isLoading, setIsLoading] = useState(false)
  const [domains, setDomains] = useState<string[]>([])
  const [prerollAds, setPrerollAds] = useState<LocalPrerollItem[]>([])

  // DnD sensors
  const sensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    })
  )

  const {
    register,
    handleSubmit,
    watch,
    setValue,
    reset,
    formState: { errors },
  } = useForm<FormData>({
    defaultValues: {
      name: '',
      description: '',
      isActive: true,
      thumbnailUrl: '',
      watermarkEnabled: false,
      watermarkUrl: '',
      watermarkPosition: 'bottom-right',
      watermarkOpacity: 0.7,
      watermarkSize: 80,
      watermarkOffsetY: 0,
    },
  })

  // Reset form when profile changes or mode changes
  useEffect(() => {
    if (mode === 'edit' && profile) {
      reset({
        name: profile.name,
        description: profile.description || '',
        isActive: profile.isActive,
        thumbnailUrl: profile.thumbnailUrl || '',
        watermarkEnabled: profile.watermarkEnabled,
        watermarkUrl: profile.watermarkUrl || '',
        watermarkPosition: profile.watermarkPosition || 'bottom-right',
        watermarkOpacity: profile.watermarkOpacity || 0.7,
        watermarkSize: profile.watermarkSize || 80,
        watermarkOffsetY: profile.watermarkOffsetY || 0,
      })
      // Load existing domains
      setDomains(profile.domains?.map(d => d.domain) || [])
      // Load existing preroll ads
      setPrerollAds(
        (profile.prerollAds || []).map(ad => ({
          id: ad.id,
          type: ad.type || 'video',
          url: ad.url,
          duration: ad.duration || 10,
          skipAfter: ad.skipAfter,
          clickUrl: ad.clickUrl || '',
          clickText: ad.clickText || '',
          title: ad.title || '',
        }))
      )
    } else if (mode === 'create') {
      reset({
        name: '',
        description: '',
        isActive: true,
        thumbnailUrl: '',
        watermarkEnabled: false,
        watermarkUrl: '',
        watermarkPosition: 'bottom-right',
        watermarkOpacity: 0.7,
        watermarkSize: 80,
        watermarkOffsetY: 0,
      })
      setDomains([])
      setPrerollAds([])
    }
  }, [mode, profile, reset])

  const watermarkEnabled = watch('watermarkEnabled')
  const watermarkOpacity = watch('watermarkOpacity')
  const watermarkSize = watch('watermarkSize')
  const watermarkOffsetY = watch('watermarkOffsetY')

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

  // Preroll handlers
  const handleAddPreroll = () => {
    const newId = `temp-${Date.now()}`
    setPrerollAds([...prerollAds, {
      id: newId,
      type: 'video',
      url: '',
      duration: 10,
      skipAfter: 5,
      clickUrl: '',
      clickText: '',
      title: '',
      isNew: true,
    }])
  }

  const handleUpdatePreroll = (id: string, updates: Partial<LocalPrerollItem>) => {
    setPrerollAds(prerollAds.map(p => (p.id === id ? { ...p, ...updates } : p)))
  }

  const handleDeletePreroll = (id: string) => {
    setPrerollAds(prerollAds.filter(p => p.id !== id))
  }

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event

    if (over && active.id !== over.id) {
      setPrerollAds((items) => {
        const oldIndex = items.findIndex((item) => item.id === active.id)
        const newIndex = items.findIndex((item) => item.id === over.id)
        return arrayMove(items, oldIndex, newIndex)
      })
    }
  }

  const onSubmit = async (data: FormData) => {
    setIsLoading(true)
    try {
      // แปลง empty string เป็น undefined เพื่อไม่ให้ส่งไป backend
      const cleanedData = {
        ...data,
        thumbnailUrl: data.thumbnailUrl?.trim() || undefined,
        watermarkUrl: data.watermarkUrl?.trim() || undefined,
        description: data.description?.trim() || undefined,
      }

      if (mode === 'create') {
        // 1. สร้าง profile ก่อน
        const newProfile = await createProfile.mutateAsync(cleanedData as CreateWhitelistProfileRequest)

        // 2. เพิ่ม domains ทีละตัว
        for (const domain of domains) {
          await addDomain.mutateAsync({
            profileId: newProfile.id,
            data: { domain },
          })
        }

        // 3. เพิ่ม preroll ads ทีละตัว (เรียงตาม order)
        for (const preroll of prerollAds) {
          if (preroll.url.trim()) {
            await whitelistService.addPrerollAd(newProfile.id, {
              type: preroll.type,
              url: preroll.url.trim(),
              duration: preroll.duration,
              skipAfter: preroll.skipAfter,
              clickUrl: preroll.clickUrl.trim() || undefined,
              clickText: preroll.clickText.trim() || undefined,
              title: preroll.title.trim() || undefined,
            })
          }
        }

        // Invalidate queries หลังจากเพิ่ม preroll ads เสร็จ
        await queryClient.invalidateQueries({ queryKey: whitelistKeys.profiles() })

        toast.success('สร้างโปรไฟล์สำเร็จ')
      } else if (profile) {
        // 1. อัพเดท profile
        await updateProfile.mutateAsync({ id: profile.id, data: cleanedData })

        // 2. จัดการ domains (เพิ่ม/ลบ)
        const existingDomains = profile.domains || []
        const existingSet = new Set(existingDomains.map(d => d.domain))
        const newSet = new Set(domains)

        const toAdd = domains.filter(d => !existingSet.has(d))
        const toRemove = existingDomains.filter(d => !newSet.has(d.domain))

        for (const domain of toAdd) {
          await addDomain.mutateAsync({
            profileId: profile.id,
            data: { domain },
          })
        }

        for (const domain of toRemove) {
          await removeDomain.mutateAsync(domain.id)
        }

        // 3. จัดการ preroll ads
        const existingPrerolls = profile.prerollAds || []
        const existingPrerollIds = new Set(existingPrerolls.map(p => p.id))
        const currentPrerollIds = new Set(prerollAds.filter(p => !p.isNew).map(p => p.id))

        // ลบ preroll ที่ถูกลบออก
        for (const preroll of existingPrerolls) {
          if (!currentPrerollIds.has(preroll.id)) {
            await whitelistService.deletePrerollAd(preroll.id)
          }
        }

        // เพิ่ม preroll ใหม่
        for (const preroll of prerollAds) {
          if (preroll.isNew && preroll.url.trim()) {
            await whitelistService.addPrerollAd(profile.id, {
              type: preroll.type,
              url: preroll.url.trim(),
              duration: preroll.duration,
              skipAfter: preroll.skipAfter,
              clickUrl: preroll.clickUrl.trim() || undefined,
              clickText: preroll.clickText.trim() || undefined,
              title: preroll.title.trim() || undefined,
            })
          }
        }

        // อัพเดท preroll ที่มีอยู่
        for (const preroll of prerollAds) {
          if (!preroll.isNew && existingPrerollIds.has(preroll.id)) {
            const existing = existingPrerolls.find(p => p.id === preroll.id)
            // ตรวจสอบว่ามีการเปลี่ยนแปลงหรือไม่
            const hasChanges = existing && (
              existing.type !== preroll.type ||
              existing.url !== preroll.url ||
              existing.duration !== preroll.duration ||
              existing.skipAfter !== preroll.skipAfter ||
              existing.clickUrl !== preroll.clickUrl ||
              existing.clickText !== preroll.clickText ||
              existing.title !== preroll.title
            )
            if (hasChanges) {
              await whitelistService.updatePrerollAd(preroll.id, {
                type: preroll.type,
                url: preroll.url.trim(),
                duration: preroll.duration,
                skipAfter: preroll.skipAfter,
                clickUrl: preroll.clickUrl.trim() || undefined,
                clickText: preroll.clickText.trim() || undefined,
                title: preroll.title.trim() || undefined,
              })
            }
          }
        }

        // Reorder if needed (หลังจากเพิ่ม/ลบเสร็จแล้ว)
        const currentIds = prerollAds.filter(p => !p.isNew).map(p => p.id)
        if (currentIds.length > 0) {
          await whitelistService.reorderPrerollAds(profile.id, { prerollIds: currentIds })
        }

        // Invalidate queries หลังจากจัดการ preroll ads เสร็จ
        await queryClient.invalidateQueries({ queryKey: whitelistKeys.profiles() })
        await queryClient.invalidateQueries({ queryKey: whitelistKeys.profileDetail(profile.id) })

        toast.success('บันทึกโปรไฟล์สำเร็จ')
      }
      onOpenChange(false)
    } catch (err) {
      toast.error(mode === 'create' ? 'ไม่สามารถสร้างโปรไฟล์ได้' : 'ไม่สามารถบันทึกโปรไฟล์ได้')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-full sm:max-w-lg overflow-y-auto">
        <SheetHeader>
          <SheetTitle>{mode === 'create' ? 'สร้างโปรไฟล์ใหม่' : 'แก้ไขโปรไฟล์'}</SheetTitle>
          <SheetDescription>
            กำหนดโดเมนที่อนุญาต, ลายน้ำ และโฆษณาก่อนเล่น
          </SheetDescription>
        </SheetHeader>

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-6 p-4 relative flex-1 overflow-y-auto">
          {/* Loading Overlay */}
          {isLoading && (
            <div className="absolute inset-0 bg-background/80 backdrop-blur-sm z-50 flex items-center justify-center">
              <div className="flex flex-col items-center gap-2">
                <Loader2 className="h-8 w-8 animate-spin text-primary" />
                <p className="text-sm text-muted-foreground">กำลังบันทึก...</p>
              </div>
            </div>
          )}

          {/* Basic Info */}
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="name">ชื่อโปรไฟล์ *</Label>
              <Input
                id="name"
                {...register('name', { required: 'กรุณาระบุชื่อ' })}
                placeholder="เช่น เว็บไซต์หลัก"
              />
              {errors.name && (
                <p className="text-xs text-destructive">{errors.name.message}</p>
              )}
            </div>

            <div className="space-y-2">
              <Label htmlFor="description">คำอธิบาย</Label>
              <Textarea
                id="description"
                {...register('description')}
                placeholder="คำอธิบายโปรไฟล์ (ไม่บังคับ)"
                rows={2}
              />
            </div>

            <div className="flex items-center justify-between">
              <Label htmlFor="isActive">เปิดใช้งาน</Label>
              <Switch
                id="isActive"
                checked={watch('isActive')}
                onCheckedChange={(checked) => setValue('isActive', checked)}
              />
            </div>

            <div className="space-y-2">
              <Label htmlFor="thumbnailUrl">Thumbnail (ภาพก่อนกด Play)</Label>
              <Input
                id="thumbnailUrl"
                {...register('thumbnailUrl')}
                placeholder="https://example.com/thumbnail.jpg"
              />
              <p className="text-xs text-muted-foreground">
                ถ้าไม่ใส่จะใช้ thumbnail ของวิดีโอแทน
              </p>
            </div>
          </div>

          <Separator />

          {/* Domains */}
          <div className="space-y-4">
            <div>
              <h4 className="font-medium">โดเมน ({domains.length})</h4>
              <p className="text-xs text-muted-foreground">กำหนดโดเมนที่อนุญาตให้ฝังวิดีโอ</p>
            </div>

            <TagInput
              value={domains}
              onChange={setDomains}
              placeholder="พิมพ์โดเมนแล้วกด Enter..."
              validate={validateDomain}
              transform={transformDomain}
              disabled={isLoading}
            />

            <div className="space-y-1 text-xs text-muted-foreground">
              <p><code className="bg-muted px-1 rounded">example.com</code> - เฉพาะโดเมนนี้</p>
              <p><code className="bg-muted px-1 rounded">*.example.com</code> - ทุกซับโดเมน</p>
            </div>
          </div>

          <Separator />

          {/* Watermark Settings */}
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <h4 className="font-medium">ลายน้ำ</h4>
                <p className="text-xs text-muted-foreground">แสดงโลโก้บนวิดีโอ</p>
              </div>
              <Switch
                checked={watermarkEnabled}
                onCheckedChange={(checked) => setValue('watermarkEnabled', checked)}
              />
            </div>

            {watermarkEnabled && (
              <div className="space-y-4 pl-4 border-l-2 border-muted">
                <div className="space-y-2">
                  <Label htmlFor="watermarkUrl">ลิงก์รูปภาพ</Label>
                  <Input
                    id="watermarkUrl"
                    {...register('watermarkUrl')}
                    placeholder="https://example.com/logo.png"
                  />
                </div>

                <div className="space-y-2">
                  <Label>ตำแหน่ง</Label>
                  <Select
                    value={watch('watermarkPosition')}
                    onValueChange={(value) => setValue('watermarkPosition', value as WatermarkPosition)}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {Object.entries(WATERMARK_POSITION).map(([_, value]) => (
                        <SelectItem key={value} value={value}>
                          {WATERMARK_POSITION_LABELS[value]}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <div className="space-y-2">
                  <div className="flex justify-between">
                    <Label>ความโปร่งใส</Label>
                    <span className="text-xs text-muted-foreground">{Math.round(watermarkOpacity * 100)}%</span>
                  </div>
                  <Slider
                    value={[watermarkOpacity]}
                    onValueChange={([value]) => setValue('watermarkOpacity', value)}
                    min={0.1}
                    max={1}
                    step={0.1}
                  />
                </div>

                <div className="space-y-2">
                  <div className="flex justify-between">
                    <Label>ขนาด</Label>
                    <span className="text-xs text-muted-foreground">{watermarkSize} พิกเซล</span>
                  </div>
                  <Slider
                    value={[watermarkSize]}
                    onValueChange={([value]) => setValue('watermarkSize', value)}
                    min={30}
                    max={200}
                    step={10}
                  />
                </div>

                <div className="space-y-2">
                  <div className="flex justify-between">
                    <Label>ระยะห่างจากขอบล่าง</Label>
                    <span className="text-xs text-muted-foreground">{watermarkOffsetY} พิกเซล</span>
                  </div>
                  <Slider
                    value={[watermarkOffsetY]}
                    onValueChange={([value]) => setValue('watermarkOffsetY', value)}
                    min={0}
                    max={100}
                    step={5}
                  />
                  <p className="text-xs text-muted-foreground">
                    สำหรับหลบปุ่มควบคุมบนมือถือ
                  </p>
                </div>
              </div>
            )}
          </div>

          <Separator />

          {/* Pre-roll Ads Settings - Multiple */}
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <h4 className="font-medium">โฆษณาก่อนเล่น ({prerollAds.length})</h4>
                <p className="text-xs text-muted-foreground">เล่นตามลำดับก่อนเริ่มวิดีโอ</p>
              </div>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={handleAddPreroll}
                disabled={isLoading}
              >
                <Plus className="h-4 w-4 mr-1" />
                เพิ่ม
              </Button>
            </div>

            {prerollAds.length > 0 && (
              <div className="space-y-2">

                <DndContext
                  sensors={sensors}
                  collisionDetection={closestCenter}
                  onDragEnd={handleDragEnd}
                >
                  <SortableContext
                    items={prerollAds.map(p => p.id)}
                    strategy={verticalListSortingStrategy}
                  >
                    <div className="space-y-2">
                      {prerollAds.map((item) => (
                        <SortablePrerollItem
                          key={item.id}
                          item={item}
                          onUpdate={handleUpdatePreroll}
                          onDelete={handleDeletePreroll}
                          disabled={isLoading}
                        />
                      ))}
                    </div>
                  </SortableContext>
                </DndContext>

                <p className="text-xs text-muted-foreground">
                  ลากเพื่อจัดลำดับ | 0 วินาที = บังคับดูจนจบ
                </p>
              </div>
            )}

            {prerollAds.length === 0 && (
              <p className="text-sm text-muted-foreground text-center py-4">
                ยังไม่มีโฆษณา คลิก "เพิ่ม" เพื่อเพิ่มโฆษณาก่อนเล่น
              </p>
            )}
          </div>

          <SheetFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={isLoading}>
              ยกเลิก
            </Button>
            <Button type="submit" disabled={isLoading}>
              {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              {mode === 'create' ? 'สร้างโปรไฟล์' : 'บันทึก'}
            </Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  )
}
