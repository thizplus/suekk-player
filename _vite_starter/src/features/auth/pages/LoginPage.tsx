import { Link } from 'react-router-dom'
import { GalleryVerticalEnd } from 'lucide-react'
import { LoginForm } from '../components/LoginForm'
import { LoginAnimation } from '../components/LoginAnimation'
import { APP_CONFIG } from '@/constants'

export function LoginPage() {
  const { title: appTitle } = APP_CONFIG

  return (
    <div className="grid min-h-svh lg:grid-cols-2">
      {/* Left side - Login form */}
      <div className="flex flex-col gap-4 p-6 md:p-10">
        <div className="flex items-center justify-between">
          <Link to="/" className="flex items-center gap-2 font-medium">
            <div className="bg-primary text-primary-foreground flex size-6 items-center justify-center rounded-md">
              <GalleryVerticalEnd className="size-4" />
            </div>
            {appTitle}
          </Link>
        </div>
        <div className="flex flex-1 items-center justify-center">
          <div className="w-full max-w-xs">
            <LoginForm />
          </div>
        </div>
      </div>

      {/* Right side - Animation */}
      <div className="relative hidden overflow-hidden lg:block">
        <LoginAnimation />
      </div>
    </div>
  )
}
