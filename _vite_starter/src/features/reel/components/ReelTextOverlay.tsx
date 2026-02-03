import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
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
  // Callbacks
  onTitleChange: (title: string) => void
  onDescriptionChange: (desc: string) => void
  onShowTitleChange: (show: boolean) => void
  onShowDescriptionChange: (show: boolean) => void
  onShowGradientChange: (show: boolean) => void
  onTitlePositionChange: (pos: TitlePosition) => void
}

export function ReelTextOverlay({
  title,
  description,
  showTitle,
  showDescription,
  showGradient,
  titlePosition,
  onTitleChange,
  onDescriptionChange,
  onShowTitleChange,
  onShowDescriptionChange,
  onShowGradientChange,
  onTitlePositionChange,
}: ReelTextOverlayProps) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm">3. ข้อความบนวิดีโอ</CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Title */}
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <Label>หัวข้อ</Label>
            <Switch checked={showTitle} onCheckedChange={onShowTitleChange} />
          </div>
          {showTitle && (
            <>
              <Input
                value={title}
                onChange={(e) => onTitleChange(e.target.value)}
                placeholder="พิมพ์หัวข้อ..."
              />
              <Select
                value={titlePosition}
                onValueChange={(v) => onTitlePositionChange(v as TitlePosition)}
              >
                <SelectTrigger className="h-8">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="top">แสดงด้านบน</SelectItem>
                  <SelectItem value="center">แสดงตรงกลาง</SelectItem>
                  <SelectItem value="bottom">แสดงด้านล่าง</SelectItem>
                </SelectContent>
              </Select>
            </>
          )}
        </div>

        {/* Description */}
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <Label>คำอธิบาย</Label>
            <Switch checked={showDescription} onCheckedChange={onShowDescriptionChange} />
          </div>
          {showDescription && (
            <Textarea
              value={description}
              onChange={(e) => onDescriptionChange(e.target.value)}
              placeholder="พิมพ์คำอธิบาย..."
              rows={2}
            />
          )}
        </div>

        {/* Gradient Toggle */}
        <div className="flex items-center justify-between">
          <Label>Gradient พื้นหลัง</Label>
          <Switch checked={showGradient} onCheckedChange={onShowGradientChange} />
        </div>
      </CardContent>
    </Card>
  )
}
