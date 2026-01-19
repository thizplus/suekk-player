import { APP_CONFIG } from '@/constants'

export function LoginAnimation() {
  const { title, description } = APP_CONFIG

  return (
    <div className="bg-primary flex h-full w-full flex-col items-center justify-center overflow-hidden p-8">
      {/* Animated blobs */}
      <div className="absolute inset-0">
        <div className="bg-primary-foreground/20 absolute -left-20 -top-20 h-[500px] w-[500px] animate-pulse rounded-full blur-3xl filter" />
        <div
          className="bg-primary-foreground/15 absolute -right-32 top-20 h-[600px] w-[600px] animate-pulse rounded-full blur-3xl filter"
          style={{ animationDelay: '1s' }}
        />
        <div
          className="bg-primary-foreground/15 absolute -bottom-32 left-1/4 h-[550px] w-[550px] animate-pulse rounded-full blur-3xl filter"
          style={{ animationDelay: '2s' }}
        />
        <div
          className="bg-primary-foreground/20 absolute -right-20 bottom-20 h-[450px] w-[450px] animate-pulse rounded-full blur-3xl filter"
          style={{ animationDelay: '3s' }}
        />
      </div>

      {/* Content */}
      <div className="relative z-10 text-center">
        <h2 className="text-primary-foreground text-4xl font-bold">{title}</h2>
        <p className="text-primary-foreground/80 mt-4 text-xl">{description}</p>
      </div>
    </div>
  )
}
