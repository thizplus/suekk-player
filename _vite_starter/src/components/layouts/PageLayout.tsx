import { Outlet, useLocation } from 'react-router-dom'
import { Suspense } from 'react'
import { AppSidebar } from '@/components/layouts/AppSidebar'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import { Separator } from '@/components/ui/separator'
import { SidebarInset, SidebarProvider, SidebarTrigger } from '@/components/ui/sidebar'
import { ModeToggle } from '@/theme/mode-toggle'
import { APP_CONFIG } from '@/constants'
import { LoadingOverlay } from '@/components/ui/loading'
import { WebSocketProvider } from '@/lib/websocket-provider'

// แปลงชื่อ path เป็นภาษาไทย
const PAGE_TITLES: Record<string, string> = {
  dashboard: 'แดชบอร์ด',
  users: 'ผู้ใช้งาน',
  create: 'เพิ่มใหม่',
  profile: 'โปรไฟล์',
  settings: 'ตั้งค่า',
  account: 'บัญชี',
  admin: 'ผู้ดูแลระบบ',
  agent: 'ตัวแทน',
  sales: 'ฝ่ายขาย',
  videos: 'วิดีโอ',
  transcoding: 'ประมวลผล',
  categories: 'หมวดหมู่',
  analytics: 'สถิติ',
  storage: 'พื้นที่จัดเก็บ',
  domains: 'โดเมน',
}

export function PageLayout() {
  const location = useLocation()
  const pathSegments = location.pathname.split('/').filter(Boolean)
  const currentPage = pathSegments[pathSegments.length - 1] || 'dashboard'
  const pageTitle = PAGE_TITLES[currentPage] || currentPage

  return (
    <WebSocketProvider>
      <SidebarProvider>
        <AppSidebar />
        <SidebarInset>
          <header className="flex h-16 shrink-0 items-center gap-2 transition-[width,height] ease-linear group-has-data-[collapsible=icon]/sidebar-wrapper:h-12">
            <div className="flex w-full items-center gap-2 px-4">
              <SidebarTrigger className="-ml-1" />
              <Separator orientation="vertical" className="mr-2 data-[orientation=vertical]:h-4" />
              <Breadcrumb>
                <BreadcrumbList>
                  <BreadcrumbItem className="hidden md:block">
                    <BreadcrumbLink href="/dashboard">{APP_CONFIG.title}</BreadcrumbLink>
                  </BreadcrumbItem>
                  <BreadcrumbSeparator className="hidden md:block" />
                  <BreadcrumbItem>
                    <BreadcrumbPage>{pageTitle}</BreadcrumbPage>
                  </BreadcrumbItem>
                </BreadcrumbList>
              </Breadcrumb>
              <ModeToggle />
            </div>
          </header>
          <div className="flex flex-1 flex-col gap-4 p-4 pt-0">
            <Suspense fallback={<LoadingOverlay className="flex-1" />}>
              <Outlet />
            </Suspense>
          </div>
        </SidebarInset>
      </SidebarProvider>
    </WebSocketProvider>
  )
}
