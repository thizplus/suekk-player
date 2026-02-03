import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { Slider } from '@/components/ui/slider'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import type { TitlePosition } from '../types'

interface ReelTextOverlayProps {
  title: string
  description: string
  showTitle: boolean
  showDescription: boolean
  showGradient: boolean
  titlePosition: TitlePosition
  titleFontSize: number
  descriptionFontSize: number
  onTitleChange: (title: string) => void
  onDescriptionChange: (desc: string) => void
  onShowTitleChange: (show: boolean) => void
  onShowDescriptionChange: (show: boolean) => void
  onShowGradientChange: (show: boolean) => void
  onTitlePositionChange: (pos: TitlePosition) => void
  onTitleFontSizeChange: (size: number) => void
  onDescriptionFontSizeChange: (size: number) => void
}

export function ReelTextOverlay({
  title,
  description,
  showTitle,
  showDescription,
  showGradient,
  titlePosition,
  titleFontSize,
  descriptionFontSize,
  onTitleChange,
  onDescriptionChange,
  onShowTitleChange,
  onShowDescriptionChange,
  onShowGradientChange,
  onTitlePositionChange,
  onTitleFontSizeChange,
  onDescriptionFontSizeChange,
}: ReelTextOverlayProps) {
  return (
    <div className="space-y-4">
      {/* Title */}
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <Label>หัวข้อ</Label>
          <Switch checked={showTitle} onCheckedChange={onShowTitleChange} />
        </div>
        {showTitle && (
          <div className="space-y-3">
            <Input
              value={title}
              onChange={(e) => onTitleChange(e.target.value)}
              placeholder="พิมพ์หัวข้อ..."
            />
            <Select
              value={titlePosition}
              onValueChange={(v) => onTitlePositionChange(v as TitlePosition)}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="top">แสดงด้านบน</SelectItem>
                <SelectItem value="center">แสดงตรงกลาง</SelectItem>
                <SelectItem value="bottom">แสดงด้านล่าง</SelectItem>
              </SelectContent>
            </Select>
            <div className="space-y-1">
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">ขนาดตัวอักษร</span>
                <span className="font-mono">{titleFontSize}px</span>
              </div>
              <Slider
                value={[titleFontSize]}
                min={24}
                max={100}
                step={2}
                onValueChange={([v]) => onTitleFontSizeChange(v)}
              />
            </div>
          </div>
        )}
      </div>

      {/* Description */}
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <Label>คำอธิบาย</Label>
          <Switch checked={showDescription} onCheckedChange={onShowDescriptionChange} />
        </div>
        {showDescription && (
          <div className="space-y-3">
            <Textarea
              value={description}
              onChange={(e) => onDescriptionChange(e.target.value)}
              placeholder="พิมพ์คำอธิบาย..."
              rows={2}
            />
            <div className="space-y-1">
              <div className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">ขนาดตัวอักษร</span>
                <span className="font-mono">{descriptionFontSize}px</span>
              </div>
              <Slider
                value={[descriptionFontSize]}
                min={16}
                max={52}
                step={2}
                onValueChange={([v]) => onDescriptionFontSizeChange(v)}
              />
            </div>
          </div>
        )}
      </div>

      {/* Gradient Toggle */}
      <div className="flex items-center justify-between">
        <Label>Gradient พื้นหลัง</Label>
        <Switch checked={showGradient} onCheckedChange={onShowGradientChange} />
      </div>
    </div>
  )
}
