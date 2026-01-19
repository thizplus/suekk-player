import { cn } from '@/lib/utils'

interface LoadingProps {
  className?: string
  size?: 'sm' | 'md' | 'lg'
  fullScreen?: boolean
}

export function Loading({ className, size = 'md', fullScreen = false }: LoadingProps) {
  const sizeClasses = {
    sm: 'h-4 w-4',
    md: 'h-8 w-8',
    lg: 'h-12 w-12',
  }

  const spinner = (
    <div className={cn('relative', sizeClasses[size], className)}>
      {/* Outer ring */}
      <div className="absolute inset-0 rounded-full border-2 border-muted" />
      {/* Spinning arc */}
      <div className="absolute inset-0 animate-spin rounded-full border-2 border-transparent border-t-primary" />
    </div>
  )

  if (fullScreen) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        {spinner}
      </div>
    )
  }

  return spinner
}

interface LoadingOverlayProps {
  className?: string
}

export function LoadingOverlay({ className }: LoadingOverlayProps) {
  return (
    <div className={cn('flex items-center justify-center p-8', className)}>
      <Loading size="md" />
    </div>
  )
}
