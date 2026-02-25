import { Routes, Route, Navigate } from 'react-router-dom'
import { Suspense } from 'react'
import { lazyWithReload } from '@/lib/lazy-with-reload'

// Layouts
import { RootLayout, PageLayout } from '@/components/layouts'
import { ProtectedRoute } from './ProtectedRoute'

// Auth feature (not lazy - first load)
import { LoginPage, RegisterPage, GoogleCallbackPage } from '@/features/auth'

// Lazy load embed page (public, lightweight)
const EmbedPage = lazyWithReload(() =>
  import('@/features/embed').then((m) => ({ default: m.EmbedPage }))
)

// Lazy load preview page (admin preview, no ads)
const PreviewPage = lazyWithReload(() =>
  import('@/features/embed').then((m) => ({ default: m.PreviewPage }))
)

// Lazy load subtitle editor page (admin edit subtitles)
const SubtitleEditorPage = lazyWithReload(() =>
  import('@/features/subtitle').then((m) => ({ default: m.SubtitleEditorPage }))
)

// Lazy load dashboard pages
const AdminDashboard = lazyWithReload(() =>
  import('@/features/dashboard').then((m) => ({ default: m.AdminDashboard }))
)
const AgentDashboard = lazyWithReload(() =>
  import('@/features/dashboard').then((m) => ({ default: m.AgentDashboard }))
)
const SalesDashboard = lazyWithReload(() =>
  import('@/features/dashboard').then((m) => ({ default: m.SalesDashboard }))
)

// Lazy load user pages
const UserProfilePage = lazyWithReload(() =>
  import('@/features/user').then((m) => ({ default: m.UserProfilePage }))
)

// Lazy load video pages
const VideoListPage = lazyWithReload(() =>
  import('@/features/video').then((m) => ({ default: m.VideoListPage }))
)
const DLQPage = lazyWithReload(() =>
  import('@/features/video').then((m) => ({ default: m.DLQPage }))
)
const GalleryPage = lazyWithReload(() =>
  import('@/features/video').then((m) => ({ default: m.GalleryPage }))
)
const GalleryManagerPage = lazyWithReload(() =>
  import('@/features/video').then((m) => ({ default: m.GalleryManagerPage }))
)

// Lazy load category pages
const CategoryListPage = lazyWithReload(() =>
  import('@/features/category').then((m) => ({ default: m.CategoryListPage }))
)

// Lazy load transcoding page
const TranscodingPage = lazyWithReload(() =>
  import('@/features/transcoding').then((m) => ({ default: m.TranscodingPage }))
)

// Lazy load whitelist page (Phase 6)
const WhitelistPage = lazyWithReload(() =>
  import('@/features/whitelist').then((m) => ({ default: m.WhitelistPage }))
)

// Lazy load settings page (Admin Settings)
const SettingsPage = lazyWithReload(() =>
  import('@/features/settings').then((m) => ({ default: m.SettingsPage }))
)

// Lazy load workers page (Phase 0: Worker Registry)
const WorkersPage = lazyWithReload(() =>
  import('@/features/workers').then((m) => ({ default: m.WorkersPage }))
)

// Lazy load queue management page
const QueueManagementPage = lazyWithReload(() =>
  import('@/features/queue').then((m) => ({ default: m.QueueManagementPage }))
)

// Lazy load reel pages (Reel Generator)
const ReelListPage = lazyWithReload(() =>
  import('@/features/reel').then((m) => ({ default: m.ReelListPage }))
)
const ReelGeneratorPage = lazyWithReload(() =>
  import('@/features/reel').then((m) => ({ default: m.ReelGeneratorPage }))
)

export default function AppRoutes() {
  return (
    <Routes>
      {/* Embed route - standalone, no layout (for iframe embedding) */}
      <Route
        path="/embed/:code"
        element={
          <Suspense fallback={<div className="fixed inset-0 bg-black flex items-center justify-center"><div className="animate-spin h-8 w-8 border-2 border-white border-t-transparent rounded-full" /></div>}>
            <EmbedPage />
          </Suspense>
        }
      />

      {/* Preview route - standalone, no ads (for admin preview) */}
      <Route
        path="/preview/:code"
        element={
          <Suspense fallback={<div className="fixed inset-0 bg-black flex items-center justify-center"><div className="animate-spin h-8 w-8 border-2 border-white border-t-transparent rounded-full" /></div>}>
            <PreviewPage />
          </Suspense>
        }
      />

      {/* Subtitle Editor route - edit Thai subtitles with real-time preview */}
      <Route
        path="/preview/:code/edit"
        element={
          <Suspense fallback={<div className="fixed inset-0 bg-background flex items-center justify-center"><div className="animate-spin h-8 w-8 border-2 border-primary border-t-transparent rounded-full" /></div>}>
            <SubtitleEditorPage />
          </Suspense>
        }
      />

      {/* Gallery route - standalone gallery viewer */}
      <Route
        path="/gallery/:code"
        element={
          <Suspense fallback={<div className="fixed inset-0 bg-background flex items-center justify-center"><div className="animate-spin h-8 w-8 border-2 border-primary border-t-transparent rounded-full" /></div>}>
            <GalleryPage />
          </Suspense>
        }
      />

      <Route element={<RootLayout />}>
        {/* Public routes */}
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route path="/auth/google/callback" element={<GoogleCallbackPage />} />

        {/* Root redirect */}
        <Route path="/" element={<Navigate to="/dashboard" replace />} />

        {/* Protected routes with layout */}
        <Route element={<ProtectedRoute />}>
          <Route element={<PageLayout />}>
            {/* Dashboard routes */}
            <Route path="/dashboard" element={<AdminDashboard />} />
            <Route path="/dashboard/admin" element={<AdminDashboard />} />
            <Route path="/dashboard/agent" element={<AgentDashboard />} />
            <Route path="/dashboard/sales" element={<SalesDashboard />} />

            {/* Video routes */}
            <Route path="/videos" element={<VideoListPage />} />
            <Route path="/videos/page/:page" element={<VideoListPage />} />
            <Route path="/videos/dlq" element={<DLQPage />} />

            {/* Gallery Admin - Manual Selection Flow */}
            <Route path="/admin/videos/:id/gallery" element={<GalleryManagerPage />} />

            {/* Category routes */}
            <Route path="/categories" element={<CategoryListPage />} />

            {/* Transcoding routes */}
            <Route path="/transcoding" element={<TranscodingPage />} />

            {/* Whitelist routes (Phase 6) */}
            <Route path="/whitelist" element={<WhitelistPage />} />

            {/* Settings routes (Admin Settings) */}
            <Route path="/settings" element={<SettingsPage />} />

            {/* Workers routes (Phase 0: Worker Registry) */}
            <Route path="/workers" element={<WorkersPage />} />

            {/* Queue Management routes */}
            <Route path="/queues" element={<QueueManagementPage />} />

            {/* Reel Generator routes */}
            <Route path="/reels" element={<ReelListPage />} />
            <Route path="/reels/create" element={<ReelGeneratorPage />} />
            <Route path="/reels/:id/edit" element={<ReelGeneratorPage />} />

            {/* User routes */}
            <Route path="/profile" element={<UserProfilePage />} />
          </Route>
        </Route>

        {/* 404 route */}
        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Route>
    </Routes>
  )
}
