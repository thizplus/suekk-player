import { LayoutDashboard, Video, FolderOpen, Activity, Settings, UserCircle, Globe, Server, ListChecks, type LucideIcon } from 'lucide-react'

// Sidebar navigation types
export interface NavSubItem {
  title: string
  url: string
}

export interface NavItem {
  title: string
  url: string
  icon?: LucideIcon
  isActive?: boolean
  items?: NavSubItem[]
}

// Sidebar navigation data
export const NAV_MAIN: NavItem[] = [
  {
    title: 'แดชบอร์ด',
    url: '/dashboard',
    icon: LayoutDashboard,
  },
  {
    title: 'วิดีโอ',
    url: '/videos',
    icon: Video,
  },
  {
    title: 'หมวดหมู่',
    url: '/categories',
    icon: FolderOpen,
  },
  {
    title: 'ประมวลผล',
    url: '/transcoding',
    icon: Activity,
  },
  {
    title: 'เครื่องประมวลผล',
    url: '/workers',
    icon: Server,
  },
  {
    title: 'จัดการคิว',
    url: '/queues',
    icon: ListChecks,
  },
  {
    title: 'โดเมน',
    url: '/whitelist',
    icon: Globe,
  },
  {
    title: 'โปรไฟล์',
    url: '/profile',
    icon: UserCircle,
  },
  {
    title: 'ตั้งค่า',
    url: '/settings',
    icon: Settings,
  },
]
