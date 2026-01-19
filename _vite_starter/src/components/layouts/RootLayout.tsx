import { Outlet } from 'react-router-dom'
import { Toaster } from '@/components/ui/sonner'
import { UploadProgress } from '@/components/UploadProgress'

export function RootLayout() {
  return (
    <div className="bg-background text-foreground min-h-svh antialiased">
      <Outlet />
      <Toaster
        style={{
          fontFamily: 'Roboto, "Google Sans", "Google Sans Text", sans-serif',
        }}
        position="top-right"
        richColors
      />
      <UploadProgress />
    </div>
  )
}
