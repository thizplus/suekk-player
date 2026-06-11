# SUEKK Stream — Frontend Overview

> React 19 + TypeScript + Vite + shadcn/ui + Tailwind CSS v4
> Path: `_vite_starter/`

---

## Project Structure

```
_vite_starter/src/
├── main.tsx                         # Entry point
├── App.tsx                          # Root: BrowserRouter + ThemeProvider + QueryClient
├── index.css                        # Global CSS + Tailwind + ArtPlayer + semantic classes
├── routes/
│   ├── index.tsx                    # All route definitions
│   └── ProtectedRoute.tsx           # Auth guard (Zustand isAuthenticated)
├── components/
│   ├── layouts/
│   │   ├── RootLayout.tsx           # Toaster + UploadProgress
│   │   ├── PageLayout.tsx           # Sidebar + Header + WebSocketProvider
│   │   ├── AppSidebar.tsx           # Collapsible sidebar
│   │   ├── NavMain.tsx              # Navigation items
│   │   └── NavUser.tsx              # User avatar + logout
│   ├── ui/                          # shadcn/ui (30+ components)
│   └── UploadProgress.tsx           # Floating upload progress panel
├── constants/
│   ├── api-routes.ts                # ALL API endpoint constants
│   ├── enums.ts                     # Status enums + labels + semantic CSS classes
│   ├── sidebar-data.ts              # Navigation menu
│   └── app-config.ts                # APP_CONFIG (apiUrl, streamUrl, cdnUrl)
├── features/                        # Feature-based modules
│   ├── auth/                        # Login, Register, Google OAuth
│   ├── video/                       # Video CRUD, player, upload, gallery
│   ├── embed/                       # Public embed player + preroll ads
│   ├── subtitle/                    # Subtitle panel + editor
│   ├── dashboard/                   # Admin/Agent/Sales dashboards
│   ├── category/                    # Category CRUD
│   ├── whitelist/                   # Domain whitelist + ad profiles
│   ├── workers/                     # Worker monitoring
│   ├── queue/                       # Queue management (5 tabs)
│   ├── reel/                        # Reel generator (social clips)
│   ├── settings/                    # Admin settings
│   └── user/                        # User profile
├── hooks/
│   └── use-mobile.ts                # useIsMobile (768px)
├── lib/
│   ├── api-client.ts                # Axios wrapper + interceptors
│   ├── websocket-provider.tsx       # WebSocket singleton + React context
│   ├── direct-upload.ts             # S3 multipart direct upload
│   ├── lazy-with-reload.ts          # Lazy import with chunk error recovery
│   └── utils.ts                     # cn() helper
├── stores/
│   ├── index.ts
│   └── upload-store.ts              # Zustand upload queue
└── theme/
    ├── theme-provider.tsx           # Dark/Light mode (next-themes)
    └── mode-toggle.tsx              # Theme toggle button
```

---

## Provider Tree

```
BrowserRouter
  ThemeProvider (default: "dark")
    QueryClientProvider (React Query v5)
      AppRoutes
        RootLayout (Toaster + UploadProgress)
          PageLayout (WebSocketProvider + Sidebar + Header)
            Suspense -> <Outlet />   (lazy-loaded pages)
```

---

## Pages & Routes

### Public Routes

| Route | Page | Description |
|-------|------|-------------|
| `/login` | LoginPage | Email/password + Google OAuth |
| `/register` | RegisterPage | User registration |
| `/auth/google/callback` | GoogleCallbackPage | OAuth callback |
| `/embed/:code` | EmbedPage | Standalone embed player (for iframe) |
| `/preview/:code` | PreviewPage | Admin preview (no ads) |
| `/preview/:code/edit` | SubtitleEditorPage | Full-screen subtitle editor |

### Protected Routes (JWT Required)

| Route | Page | Description |
|-------|------|-------------|
| `/dashboard` | AdminDashboard | Stats, queue status, recent videos, live WS |
| `/dashboard/admin` | AdminDashboard | Admin dashboard |
| `/dashboard/agent` | AgentDashboard | Agent dashboard |
| `/dashboard/sales` | SalesDashboard | Sales dashboard |
| `/videos` | VideoListPage | Video list (paginated, filterable) |
| `/videos/page/:page` | VideoListPage | Paginated video list |
| `/videos/dlq` | DLQPage | Dead Letter Queue management |
| `/admin/videos/:id/gallery` | GalleryManagerPage | Manual gallery image management |
| `/categories` | CategoryListPage | Category CRUD + reorder |
| `/whitelist` | WhitelistPage | Domain whitelist + ad profiles |
| `/settings` | SettingsPage | Admin system settings |
| `/workers` | WorkersPage | Worker monitoring |
| `/queues` | QueueManagementPage | Queue management (5 tabs) |
| `/reels` | ReelListPage | Reel list |
| `/reels/create` | ReelGeneratorPage | Create social clip |
| `/reels/:id/edit` | ReelGeneratorPage | Edit reel |
| `/profile` | UserProfilePage | User profile |
| `*` | Redirect to `/dashboard` | 404 fallback |

---

## Feature Modules

### Auth

- **Components**: LoginForm (email/password + Google button), RegisterForm, LoginAnimation
- **Store**: Zustand + persist (`auth-storage` key) -> `user`, `token`, `isAuthenticated`
- **Flow**: Login -> API -> setAuth(user, token) -> localStorage -> interceptor reads token
- **Google OAuth**: Button -> `VITE_API_URL/api/v1/auth/google` -> Google -> callback -> same flow
- **Auto-logout**: 401 response -> clear storage -> redirect `/login`

### Video

- **VideoListPage**: Paginated (100/page), URL-based pagination, filters (search, status, category, date, sort), real-time status badges via WebSocket
- **VideoDetailSheet**: Thumbnail preview, edit title/description/category, copy embed code, SubtitlePanel embedded, gallery section, action buttons (transcode, delete, preview)
- **VideoUploadDialog**: Drag-and-drop + file picker, auto-selects direct upload for files >= 20MB
- **VideoPlayer**: ArtPlayer v5 + HLS.js (see Player section below)
- **DLQPage**: Dead letter queue videos, retry/delete actions
- **GalleryManagerPage**: Three-folder DnD (source/safe/nsfw), drag between folders, batch move, publish

### Embed (Public Player)

- **EmbedPage** flow:
  1. Fetch video by code
  2. Fetch embed config (domain whitelist check)
  3. If domain not allowed -> "Embedding not allowed"
  4. If preroll configured -> show PrerollPlayer -> then main video
  5. Fetch HLS stream token (JWT)
  6. Fetch subtitle blobs -> create Blob URLs
  7. Render VideoPlayer + optional Watermark overlay
- **PreviewPage**: Same as embed but skips domain check, no ads (admin use)
- **Components**: Watermark (overlay), PrerollPlayer (pre-roll ad)

### Subtitle

- **SubtitlePanel** (in VideoDetailSheet): Handles detect -> transcribe -> translate pipeline with WebSocket progress bars, download/edit links per subtitle
- **SubtitleEditorPage** (`/preview/:code/edit`): Full-screen split (60% player / 40% editor), edit Thai SRT in-place, real-time preview via Blob URL regeneration (debounced 300ms), unsaved changes warning
- **SRT Parser**: `parseSRT()` and `generateSRT()` utilities

### Dashboard

- **AdminDashboard**: Stats cards (total videos, ready, views), queue status (pending/queued/processing/done/failed), storage progress bar, recent 5 videos with live WebSocket badges, connection indicator
- **AgentDashboard**, **SalesDashboard**: Role-specific views

### Queue Management

- **5 Tabs**: Transcode, Subtitle, Warm Cache (CDN), Gallery, Reel
- Per-tab: active WebSocket progress inline, stats, failed list
- Batch actions: retry all, clear completed/failed/orphaned, purge NATS stream

### Reel Generator

- **ReelGeneratorPage**: 4 tabs (Video, Timecode, Text, Audio/TTS)
- **ReelPreviewCanvas**: 9:16 live preview
- Features: multi-segment selection, text overlays, logo toggle, crop position, TTS voice selection, export to MP4

### Whitelist & Ads

- **WhitelistPage**: Profile list with search, create/edit via Sheet
- **DomainManagerSheet**: Add/remove domains per profile
- **AdStatsOverview**: Impression stats, device breakdown, skip distribution
- **PrerollAd management**: Add/edit/reorder preroll ads per profile

### Workers

- **WorkersPage**: Online workers from NATS heartbeat/KV
- Summary badges: idle/processing/paused, by type (transcode/subtitle)
- Per-worker: hostname, GPU, disk usage (with level indicator), current job progress, uptime

### Settings

- **SettingsPage**: Admin settings grouped by category (general, transcoding, alert, etc.)

### Category

- **CategoryListPage**: CRUD + drag reorder

### User

- **UserProfilePage**: View/edit profile, set password for Google-only accounts

---

## Video Player

**Library**: ArtPlayer v5 + HLS.js

**Key Features**:
- HLS.js custom loader with `X-Stream-Token` header via `xhrSetup`
- Quality selector from `MANIFEST_PARSED` event (ABR + manual)
- Multi-language subtitle selector (`artplayer-plugin-multiple-subtitles`)
- Chromecast support (custom plugin, token query param, VTT conversion)
- Skip +/- 10 seconds custom toolbar controls
- Thai i18n translations
- Netflix-red theme (`#e50914`)
- `dynamicSubtitle` mode: update subtitle blob URL without recreating player
- HLS optimizations: `maxBufferLength: 30`, `backBufferLength: 10`, fast abort on seek
- Responsive subtitle font sizes via `clamp()`

**ArtPlayer CSS** (index.css): Heavily customized settings panel, Netflix-style subtitle with text-shadow, responsive font sizes at 768px/1440px/2560px breakpoints

---

## Upload Flow

### Traditional Upload (files < 20MB)
```
VideoUploadDialog -> videoService.upload() -> POST /api/v1/videos/upload (FormData)
Progress via Axios onUploadProgress callback
```

### Direct Upload (files >= 20MB, default)
```
1. POST /api/v1/direct-upload/init -> get uploadId, videoCode, presigned part URLs
2. Upload parts directly to S3 via fetch PUT (5 concurrent parts)
3. POST /api/v1/direct-upload/complete -> API merges, creates video, auto-enqueues transcode
Supports AbortSignal, reports phases: preparing | uploading | completing
```

**UploadProgress.tsx**: Floating bottom-right panel, per-file progress bars, minimize/expand, clear completed. Invalidates React Query on success.

---

## API Layer

**`lib/api-client.ts`** — Axios wrapper:

- **Base URL**: `APP_CONFIG.apiUrl` (from `VITE_API_URL`)
- **Request interceptor**: Reads token from `auth-storage` localStorage -> `Authorization: Bearer {token}`
- **Response interceptor**: 401 -> clear storage + redirect `/login` (skips for `/embed/*`)
- **Methods**: `get<T>`, `getPaginated<T>`, `post<T>`, `postVoid`, `put<T>`, `patch<T>`, `delete`, `deleteWithResponse<T>`, `postWithProgress<T>`
- All responses follow `{ success: bool, data: T }` or `{ success: bool, data: T[], meta: PaginationMeta }`

### Constants (`constants/api-routes.ts`)

All endpoints centralized:
`AUTH_ROUTES`, `USER_ROUTES`, `DASHBOARD_ROUTES`, `VIDEO_ROUTES`, `CATEGORY_ROUTES`, `WHITELIST_ROUTES`, `WORKER_ROUTES`, `SETTINGS_ROUTES`, `DIRECT_UPLOAD_ROUTES`, `CONFIG_ROUTES`, `STORAGE_ROUTES`, `WORKER_JOB_ROUTES`, `QUEUE_ROUTES`, `REEL_ROUTES`, `HLS_ROUTES`, `GALLERY_ADMIN_ROUTES`, `SUBTITLE_ROUTES`

---

## State Management

| Data Type | Solution | Location |
|-----------|----------|----------|
| Auth (user/token) | Zustand + persist | `features/auth/store/auth-store.ts` |
| Upload queue | Zustand (no persist) | `stores/upload-store.ts` |
| Server data | React Query v5 | Per-feature `hooks.ts` |
| UI state | useState | Components |
| Theme | localStorage via next-themes | `theme/theme-provider.tsx` |

### React Query Patterns

- Query key factories per feature: `videoKeys`, `queueKeys`, `subtitleKeys`, etc.
- Optimistic updates on delete
- Automatic invalidation on mutations
- Custom stale times: gallery URLs 30 min, stream tokens 3 hours
- Polling: transcoding stats every 5s, DLQ every 10s

---

## WebSocket (Real-time)

**`lib/websocket-provider.tsx`** — Singleton WebSocket:

- URL: `${VITE_WS_URL}/ws?room=analytics`
- Singleton pattern (prevents React StrictMode double-connect)
- Auto-reconnect: exponential backoff, max 5 attempts, max 30s delay
- 30s heartbeat ping/pong

### Events Handled

| Event | Key | Behavior |
|-------|-----|----------|
| `video_progress` | `${videoId}-${type}` | Update activeProgress Map (transcode/gallery/upload/warmcache) |
| `subtitle_progress` | `${videoId}-subtitle` | Update Map |
| `reel_progress` | - | Invalidate queue stats |
| `pong` | - | Heartbeat response |

### Status Change Behavior

- `started` -> immediately invalidate `videoKeys.all`
- `completed` / `failed` -> invalidate after 500ms, remove from map after 3s

### Hooks Exported

- `useWebSocketConnection()` — raw context
- `useVideoProgress()` — filtered Map (excludes entries > 2 min old)
- `useVideoProgressById(videoId, type)` — single entry
- `useSubtitleProgress(videoId)` — subtitle progress array

---

## Theming

### Color System (OKLCH)

- `:root` = light mode, `.dark` = dark mode (default is dark)
- Standard shadcn/ui tokens: `--background`, `--foreground`, `--primary`, `--secondary`, `--muted`, `--accent`, `--destructive`, etc.
- Custom status tokens: `--status-{pending|queued|processing|success|danger|info|muted}-{bg|text}`
- Worker-type tokens: `--status-{transcode|subtitle|warning}-{bg|text}`
- ArtPlayer tokens: `--art-bg`, `--art-accent` (Netflix red), etc.

### Semantic CSS Classes (`@layer components` in index.css)

```css
.status-pending    /* yellow tones */
.status-queued     /* blue tones */
.status-processing /* indigo tones */
.status-success    /* green tones */
.status-danger     /* red tones */
.status-info       /* blue tones */
.status-muted      /* gray tones */
.status-transcode  /* purple tones */
.status-subtitle   /* teal tones */
```

Usage in enums.ts: `STATUS_STYLES = { pending: 'status-pending', ... }`
Usage in components: `<Badge className={STATUS_STYLES[item.status]}>`

### Custom Font

`DB` font (Thai, woff2 at `/fonts/db.woff2`, weight 200-700), base font size 18px

### Tailwind v4

Via `@tailwindcss/vite` plugin, CSS variables in `index.css` using `@theme inline`

---

## Key Dependencies

| Library | Version | Purpose |
|---------|---------|---------|
| React | 19 | Core framework |
| react-router-dom | v7 | Routing |
| TanStack React Query | v5 | Server state + caching |
| Zustand | v5 | Client state (auth + uploads) |
| Axios | - | HTTP client |
| artplayer | v5 | Video player |
| hls.js | - | HLS streaming |
| artplayer-plugin-multiple-subtitles | - | Multi-language subs |
| @dnd-kit | - | Drag-and-drop (gallery, reel segments) |
| react-hook-form | - | Form handling |
| sonner | - | Toast notifications |
| recharts | - | Charts (dashboard) |
| lucide-react | - | Icons |
| date-fns | - | Date formatting |
| next-themes | - | Dark/light mode |
| tailwindcss | v4 | CSS framework |
| shadcn/ui (@radix-ui/*) | - | UI primitives (30+ components) |

---

## Feature Data Flow Pattern

```
Page -> Hook (React Query) -> Service (apiClient) -> API
                                                      |
                                              WebSocket progress
                                                      |
                                              ProgressMap -> UI update
```

### Per-Feature File Convention

```
feature/
├── components/          # Feature-specific UI
├── pages/              # Routable pages
├── hooks.ts            # React Query hooks (query key factories)
├── service.ts          # API calls (via apiClient)
├── types.ts            # TypeScript interfaces
├── constants.ts        # Feature-specific constants
└── index.ts            # Barrel exports
```

---

## Responsive Design

- `useIsMobile()` hook: 768px breakpoint
- Sidebar: collapsible icon mode
- ArtPlayer CSS: extensive mobile overrides at 768px/480px
- Subtitle font: `clamp()` for responsive scaling
- Embed page: device detection for watermark config

---

## Enums & Labels (Thai)

```typescript
VIDEO_STATUS = { PENDING: 'pending', QUEUED: 'queued', PROCESSING: 'processing', READY: 'ready', FAILED: 'failed' }
VIDEO_STATUS_LABELS = { pending: 'รอดำเนินการ', queued: 'อยู่ในคิว', processing: 'กำลังประมวลผล', ready: 'พร้อมใช้งาน', failed: 'ล้มเหลว' }
VIDEO_STATUS_STYLES = { pending: 'status-pending', queued: 'status-queued', processing: 'status-processing', ready: 'status-success', failed: 'status-danger' }

LANGUAGE_LABELS = { ja: 'ญี่ปุ่น', en: 'อังกฤษ', th: 'ไทย', zh: 'จีน', ko: 'เกาหลี', ru: 'รัสเซีย' }
LANGUAGE_FLAGS = { ja: '🇯🇵', en: '🇬🇧', th: '🇹🇭', zh: '🇨🇳', ko: '🇰🇷', ru: '🇷🇺' }
```
