import type { WatermarkConfig } from '@/features/whitelist'

interface WatermarkProps {
  config: WatermarkConfig
  isMobile?: boolean
}

/**
 * Watermark overlay component
 * แสดง watermark บนวิดีโอ รองรับ responsive position
 */
export function Watermark({ config, isMobile = false }: WatermarkProps) {
  if (!config.enabled || !config.url) {
    return null
  }

  // คำนวณ position styles
  const positionStyles = getPositionStyles(config.position, config.offsetY, isMobile)

  return (
    <div
      className="absolute pointer-events-none z-10"
      style={{
        ...positionStyles,
        opacity: config.opacity,
      }}
    >
      <img
        src={config.url}
        alt="Watermark"
        style={{
          width: config.size,
          height: 'auto',
          maxWidth: isMobile ? '60px' : `${config.size}px`,
        }}
        draggable={false}
      />
    </div>
  )
}

function getPositionStyles(
  position: string,
  offsetY: number,
  isMobile: boolean
): React.CSSProperties {
  const baseOffset = 12
  const mobileOffsetY = isMobile ? offsetY + 40 : offsetY // เพิ่ม offset สำหรับ mobile controls

  switch (position) {
    case 'top-left':
      return {
        top: baseOffset + offsetY,
        left: baseOffset,
      }
    case 'top-right':
      return {
        top: baseOffset + offsetY,
        right: baseOffset,
      }
    case 'bottom-left':
      return {
        bottom: baseOffset + mobileOffsetY,
        left: baseOffset,
      }
    case 'bottom-right':
    default:
      return {
        bottom: baseOffset + mobileOffsetY,
        right: baseOffset,
      }
  }
}
