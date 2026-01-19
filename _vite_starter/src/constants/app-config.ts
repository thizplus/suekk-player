// App configuration - ดึงจาก environment variables
export const APP_CONFIG = {
  title: import.meta.env.VITE_APP_TITLE || 'Suekk Stream',
  description: import.meta.env.VITE_APP_DESCRIPTION || 'ระบบจัดการวิดีโอสตรีมมิ่ง',
  version: import.meta.env.VITE_APP_VERSION || '1.0.0',
  apiUrl: import.meta.env.VITE_API_URL || 'http://localhost:8080',
  streamUrl: import.meta.env.VITE_STREAM_URL || 'http://localhost:8080/hls',
  // CDN URL สำหรับ subtitle และ assets อื่นๆ (ไม่มี /hls)
  cdnUrl: (import.meta.env.VITE_STREAM_URL || 'http://localhost:8080/hls').replace(/\/hls$/, ''),
} as const
