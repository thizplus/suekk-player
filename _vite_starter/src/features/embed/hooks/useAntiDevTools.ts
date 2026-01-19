import { useEffect, useRef } from 'react'

interface AntiDevToolsOptions {
  onDetected?: () => void
  redirectUrl?: string
  checkInterval?: number
}

/**
 * Anti-DevTools Hook
 * ตรวจจับและป้องกันการเปิด DevTools (F12)
 */
export function useAntiDevTools(options: AntiDevToolsOptions = {}) {
  const {
    onDetected,
    redirectUrl = 'about:blank',
    checkInterval = 1000,
  } = options

  const isDetectedRef = useRef(false)

  useEffect(() => {
    // ถ้าเป็น development mode ไม่ต้องทำงาน
    if (import.meta.env.DEV) {
      return
    }

    const handleDetected = () => {
      if (isDetectedRef.current) return
      isDetectedRef.current = true

      console.clear()

      if (onDetected) {
        onDetected()
      } else {
        // Default: redirect ออก
        window.location.href = redirectUrl
      }
    }

    // === Method 1: ตรวจจับขนาดหน้าจอ ===
    // Skip เมื่ออยู่ใน iframe เพราะ outerWidth/innerWidth จะต่างกันมากโดยธรรมชาติ
    const isInIframe = window.self !== window.top
    const checkWindowSize = () => {
      // ข้าม check นี้ถ้าอยู่ใน iframe
      if (isInIframe) return

      const widthThreshold = 160
      const heightThreshold = 160

      const widthDiff = window.outerWidth - window.innerWidth
      const heightDiff = window.outerHeight - window.innerHeight

      if (widthDiff > widthThreshold || heightDiff > heightThreshold) {
        handleDetected()
      }
    }

    // === Method 2: Debugger Trap + Detection ===
    // ตรวจจับว่า debugger ถูกหยุด (DevTools เปิด) แล้วแสดง 404
    const debuggerTrap = () => {
      const start = performance.now()

      // สร้าง function แบบ anonymous เพื่อหลบ detection
      const check = new Function('debugger')
      check()

      const end = performance.now()

      // ถ้าใช้เวลานานกว่า 100ms แสดงว่า debugger หยุดอยู่ (DevTools เปิด)
      if (end - start > 100) {
        handleDetected()
      }
    }

    // รัน debugger trap ถี่มาก (100ms)
    const debuggerIntervalId = setInterval(debuggerTrap, 100)

    // === Method 4: ตรวจจับ Element inspection ===
    const setupElementTrap = () => {
      const element = new Image()
      Object.defineProperty(element, 'id', {
        get: function () {
          handleDetected()
          return ''
        },
      })
      console.log('%c', element)
      console.clear()
    }

    // === Method 5: ตรวจจับ Firebug ===
    const checkFirebug = () => {
      if (
        // @ts-expect-error - Firebug detection
        window.Firebug &&
        // @ts-expect-error - Firebug detection
        window.Firebug.chrome &&
        // @ts-expect-error - Firebug detection
        window.Firebug.chrome.isInitialized
      ) {
        handleDetected()
      }
    }

    // === Block keyboard shortcuts ===
    // ไม่ block F12/DevTools แล้ว - ให้เปิดได้ แต่จะโดน debugger trap แสดง 404 แทน
    const handleKeyDown = (e: KeyboardEvent) => {
      // Ctrl+U (View Source) - ยังคง block
      if (e.ctrlKey && e.key === 'u') {
        e.preventDefault()
        return false
      }
    }

    // === Block right-click ===
    const handleContextMenu = (e: MouseEvent) => {
      e.preventDefault()
      return false
    }

    // === Run checks ===
    const intervalId = setInterval(() => {
      checkWindowSize()
      checkFirebug()
      // checkConsoleTiming() // อาจ false positive เยอะ
    }, checkInterval)

    // Setup traps
    setupElementTrap()

    // Add event listeners
    document.addEventListener('keydown', handleKeyDown)
    document.addEventListener('contextmenu', handleContextMenu)

    // Cleanup
    return () => {
      clearInterval(intervalId)
      clearInterval(debuggerIntervalId)
      document.removeEventListener('keydown', handleKeyDown)
      document.removeEventListener('contextmenu', handleContextMenu)
    }
  }, [onDetected, redirectUrl, checkInterval])

  return {
    isDetected: isDetectedRef.current,
  }
}

/**
 * ป้องกันการ copy text
 */
export function useDisableCopy() {
  useEffect(() => {
    if (import.meta.env.DEV) return

    const handleCopy = (e: ClipboardEvent) => {
      e.preventDefault()
      return false
    }

    const handleSelectStart = (e: Event) => {
      e.preventDefault()
      return false
    }

    document.addEventListener('copy', handleCopy)
    document.addEventListener('selectstart', handleSelectStart)

    return () => {
      document.removeEventListener('copy', handleCopy)
      document.removeEventListener('selectstart', handleSelectStart)
    }
  }, [])
}
