import { lazy, type ComponentType } from 'react'

/**
 * Wrapper รอบ React.lazy() ที่จัดการ chunk load error
 * เมื่อ deploy version ใหม่ แล้ว user ยังใช้ version เก่า
 * การโหลด chunk เก่าจะ 404 → auto reload หน้าใหม่
 */
export function lazyWithReload<T extends ComponentType<unknown>>(
  factory: () => Promise<{ default: T }>
): React.LazyExoticComponent<T> {
  return lazy(async () => {
    try {
      return await factory()
    } catch (error) {
      // ตรวจสอบว่าเป็น chunk load error หรือไม่
      const isChunkError =
        error instanceof Error &&
        (error.message.includes('Failed to fetch dynamically imported module') ||
          error.message.includes('Loading chunk') ||
          error.message.includes('Loading CSS chunk'))

      if (isChunkError) {
        // เช็คว่าเคย reload แล้วหรือยัง (ป้องกัน infinite loop)
        const lastReload = sessionStorage.getItem('chunk-reload-timestamp')
        const now = Date.now()

        // ถ้าเคย reload ภายใน 10 วินาที ไม่ reload อีก
        if (lastReload && now - parseInt(lastReload) < 10000) {
          console.error('Chunk load failed after reload, giving up')
          throw error
        }

        // บันทึกเวลา reload แล้ว reload หน้า
        sessionStorage.setItem('chunk-reload-timestamp', now.toString())
        console.warn('Chunk load failed, reloading page...')
        window.location.reload()

        // Return empty component ระหว่างรอ reload
        return { default: (() => null) as unknown as T }
      }

      // ถ้าไม่ใช่ chunk error ให้ throw ต่อ
      throw error
    }
  })
}
