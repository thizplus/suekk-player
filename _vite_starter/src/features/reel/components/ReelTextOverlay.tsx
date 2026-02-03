import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import type { ReelStyle } from '../types'

interface ReelTextOverlayProps {
  style: ReelStyle
  title: string
  line1: string
  line2: string
  showLogo: boolean
  onTitleChange: (title: string) => void
  onLine1Change: (line1: string) => void
  onLine2Change: (line2: string) => void
  onShowLogoChange: (show: boolean) => void
}

// Text position info per style (for display only)
const STYLE_TEXT_INFO: Record<ReelStyle, { titlePos: string; linesPos: string }> = {
  letterbox: { titlePos: 'ด้านบน (ในแถบดำ)', linesPos: 'ด้านล่าง (ในแถบดำ)' },
  square: { titlePos: 'ด้านบน (ในแถบดำ)', linesPos: 'ด้านล่าง (ในแถบดำ)' },
  fullcover: { titlePos: 'ด้านล่าง (บน gradient)', linesPos: 'ด้านล่าง (บน gradient)' },
}

export function ReelTextOverlay({
  style,
  title,
  line1,
  line2,
  showLogo,
  onTitleChange,
  onLine1Change,
  onLine2Change,
  onShowLogoChange,
}: ReelTextOverlayProps) {
  const textInfo = STYLE_TEXT_INFO[style]

  return (
    <div className="space-y-5">
      {/* Title - Main heading */}
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <Label className="text-sm font-medium">หัวข้อหลัก (ตัวใหญ่)</Label>
          <span className="text-xs text-muted-foreground">{textInfo.titlePos}</span>
        </div>
        <Input
          value={title}
          onChange={(e) => onTitleChange(e.target.value)}
          placeholder="พิมพ์หัวข้อ..."
          maxLength={50}
          className="text-base"
        />
        <p className="text-xs text-muted-foreground">ขนาด 120px, ตัวหนา, กลางจอ</p>
      </div>

      {/* Line 1 - Secondary text */}
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <Label className="text-sm font-medium">บรรทัดที่ 1</Label>
          <span className="text-xs text-muted-foreground">{textInfo.linesPos}</span>
        </div>
        <Input
          value={line1}
          onChange={(e) => onLine1Change(e.target.value)}
          placeholder="พิมพ์ข้อความบรรทัดที่ 1..."
          maxLength={50}
        />
        <p className="text-xs text-muted-foreground">ขนาด 70px, กลางจอ</p>
      </div>

      {/* Line 2 - Third text */}
      <div className="space-y-2">
        <Label className="text-sm font-medium">บรรทัดที่ 2</Label>
        <Input
          value={line2}
          onChange={(e) => onLine2Change(e.target.value)}
          placeholder="พิมพ์ข้อความบรรทัดที่ 2..."
          maxLength={50}
        />
        <p className="text-xs text-muted-foreground">ขนาด 70px, กลางจอ</p>
      </div>

      {/* Logo Toggle */}
      <div className="flex items-center justify-between p-3 bg-muted/50 rounded-lg">
        <div>
          <Label className="text-sm font-medium">แสดง Logo</Label>
          <p className="text-xs text-muted-foreground mt-0.5">
            {style === 'fullcover' ? 'มุมซ้ายบน' : 'มุมซ้ายบนในวิดีโอ'}, โปร่งใส 30%
          </p>
        </div>
        <Switch checked={showLogo} onCheckedChange={onShowLogoChange} />
      </div>

      {/* Style-specific note */}
      {style === 'fullcover' && (
        <div className="p-3 bg-primary/5 border border-primary/20 rounded-lg">
          <p className="text-xs text-primary">
            สไตล์ Full Cover จะมี gradient ด้านล่างและ text shadow อัตโนมัติ
          </p>
        </div>
      )}
    </div>
  )
}
