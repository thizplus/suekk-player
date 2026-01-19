import { GalleryVerticalEnd } from 'lucide-react'
import { NavMain } from '@/components/layouts/NavMain'
import { NavUser } from '@/components/layouts/NavUser'
import { NAV_MAIN, APP_CONFIG } from '@/constants'
import { useAuthStore } from '@/features/auth'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
} from '@/components/ui/sidebar'

export function AppSidebar({ ...props }: React.ComponentProps<typeof Sidebar>) {
  const user = useAuthStore((s) => s.user)

  // Map User to display format with name field
  const displayUser = user
    ? {
        name: `${user.firstName} ${user.lastName}`.trim() || user.username,
        email: user.email,
        avatar: user.avatar,
      }
    : {
        name: 'Guest',
        email: 'guest@example.com',
        avatar: '/avatars/default.jpg',
      }

  return (
    <Sidebar collapsible="icon" {...props}>
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" asChild>
              <a href="#">
                <div className="bg-sidebar-primary text-sidebar-primary-foreground flex aspect-square size-8 items-center justify-center rounded-lg">
                  <GalleryVerticalEnd className="size-4" />
                </div>
                <div className="flex flex-col gap-0.5 leading-none">
                  <span className="font-medium">{APP_CONFIG.title}</span>
                  <span className="">V{APP_CONFIG.version}</span>
                </div>
              </a>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <NavMain items={NAV_MAIN} />
      </SidebarContent>
      <SidebarFooter>
        <NavUser user={displayUser} />
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  )
}
