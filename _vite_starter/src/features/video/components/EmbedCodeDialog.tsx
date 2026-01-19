import { useState } from 'react'
import { Copy, Check, Code2 } from 'lucide-react'
import { toast } from 'sonner'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Label } from '@/components/ui/label'

interface EmbedCodeDialogProps {
  videoCode: string
  videoTitle: string
  open: boolean
  onOpenChange: (open: boolean) => void
}

const EMBED_SIZES = [
  { label: '854x480 (480p)', width: '854', height: '480' },
  { label: '1280x720 (720p)', width: '1280', height: '720' },
  { label: '1920x1080 (1080p)', width: '1920', height: '1080' },
  { label: 'Responsive (100%)', width: '100%', height: '100%' },
]

export function EmbedCodeDialog({
  videoCode,
  videoTitle,
  open,
  onOpenChange,
}: EmbedCodeDialogProps) {
  const [selectedSizeLabel, setSelectedSizeLabel] = useState('1280x720 (720p)')
  const [copied, setCopied] = useState(false)

  const selectedSize = EMBED_SIZES.find((s) => s.label === selectedSizeLabel) || EMBED_SIZES[1]
  const embedUrl = `${window.location.origin}/embed/${videoCode}`

  // สร้าง embed code พร้อม attributes ที่แนะนำ
  const embedCode = `<iframe
  src="${embedUrl}"
  width="${selectedSize.width}"
  height="${selectedSize.height}"
  frameborder="0"
  allow="autoplay; encrypted-media; picture-in-picture"
  allowfullscreen
  title="${videoTitle}">
</iframe>`

  const copyToClipboard = async () => {
    try {
      await navigator.clipboard.writeText(embedCode)
      setCopied(true)
      toast.success('Copied to clipboard!')
      setTimeout(() => setCopied(false), 2000)
    } catch {
      toast.error('Failed to copy')
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Code2 className="h-5 w-5" />
            Embed Code
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          {/* Size selector */}
          <div className="space-y-2">
            <Label>Size</Label>
            <Select value={selectedSizeLabel} onValueChange={setSelectedSizeLabel}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {EMBED_SIZES.map((size) => (
                  <SelectItem key={size.label} value={size.label}>
                    {size.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* Preview info */}
          <div className="space-y-2">
            <Label>Embed URL</Label>
            <div className="bg-muted px-3 py-2 rounded-md text-sm text-muted-foreground break-all">
              {embedUrl}
            </div>
          </div>

          {/* Code preview */}
          <div className="space-y-2">
            <Label>HTML Code</Label>
            <pre className="bg-muted p-4 rounded-md text-sm overflow-x-auto whitespace-pre-wrap font-mono">
              {embedCode}
            </pre>
          </div>

          {/* Copy button */}
          <Button onClick={copyToClipboard} className="w-full" size="lg">
            {copied ? (
              <>
                <Check className="h-4 w-4 mr-2" />
                Copied!
              </>
            ) : (
              <>
                <Copy className="h-4 w-4 mr-2" />
                Copy Embed Code
              </>
            )}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
