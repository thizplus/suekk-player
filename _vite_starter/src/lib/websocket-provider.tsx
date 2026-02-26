import { createContext, useContext, useEffect, useRef, useState, useCallback, type ReactNode } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { videoKeys } from '@/features/video/hooks'
import { queueKeys } from '@/features/queue/hooks'

// Progress Types (ตรงกับ backend ProgressData)
export type ProgressType = 'upload' | 'transcode' | 'subtitle' | 'gallery' | 'reel' | 'warmcache'
export type ProgressStatus = 'started' | 'processing' | 'completed' | 'failed'

export interface VideoProgress {
  videoId: string
  videoCode: string
  videoTitle: string
  type: ProgressType
  status: ProgressStatus
  progress: number      // 0-100
  currentStep: string   // เช่น "uploading", "transcoding", "generating_thumbnail"
  message: string
  errorMessage?: string
  timestamp?: number    // เวลาที่ได้รับ update ล่าสุด
  // Subtitle-specific fields
  subtitleId?: string
  language?: string
}

export interface ReelProgress {
  reelId: string
  videoCode: string
  type: 'reel'
  status: ProgressStatus
  progress: number
  currentStep: string
  message: string
  errorMessage?: string
  outputUrl?: string
  fileSize?: number
  timestamp?: number
}

interface WebSocketContextValue {
  isConnected: boolean
  sendMessage: (type: string, data: unknown) => void
  reconnect: () => void
  // Progress tracking
  activeProgress: Map<string, VideoProgress>
}

const WebSocketContext = createContext<WebSocketContextValue | null>(null)

interface WebSocketMessage {
  type: string
  data: unknown
}

const WS_URL = import.meta.env.VITE_WS_URL || 'ws://localhost:8080'

// Singleton WebSocket instance (ป้องกัน duplicate connections จาก StrictMode)
let globalWs: WebSocket | null = null
let globalReconnectTimeout: ReturnType<typeof setTimeout> | null = null
let cleanupTimeout: ReturnType<typeof setTimeout> | null = null
let reconnectAttempts = 0
const maxReconnectAttempts = 5

interface WebSocketProviderProps {
  children: ReactNode
}

export function WebSocketProvider({ children }: WebSocketProviderProps) {
  const queryClient = useQueryClient()
  const [isConnected, setIsConnected] = useState(false)
  const [activeProgress, setActiveProgress] = useState<Map<string, VideoProgress>>(new Map())
  const messageHandlerRef = useRef<((event: MessageEvent) => void) | null>(null)

  // Message handler
  const handleMessage = useCallback(
    (event: MessageEvent) => {
      try {
        const message: WebSocketMessage = JSON.parse(event.data)
        console.log('[WebSocket] Message:', message.type)

        switch (message.type) {
          case 'video_progress': {
            const progressData = message.data as VideoProgress
            // เพิ่ม timestamp เพื่อ track ความ fresh ของ data
            progressData.timestamp = Date.now()
            // ใช้ composite key เพื่อแยก upload กับ transcode ของ video เดียวกัน
            const progressKey = `${progressData.videoId}-${progressData.type}`
            console.log('[WebSocket] Video progress:', progressData.videoCode, progressData.progress + '%', progressData.currentStep, `(${progressData.type})`)

            setActiveProgress(prev => {
              const newMap = new Map(prev)

              // ถ้าเพิ่งเริ่ม (started) ให้ invalidate เพื่อย้ายจาก pending -> processing
              if (progressData.status === 'started') {
                newMap.set(progressKey, progressData)
                // Invalidate ทันทีเมื่อเริ่ม
                queryClient.invalidateQueries({ queryKey: videoKeys.all })
              }
              // ถ้าเสร็จหรือ fail ให้ลบออกหลังจาก delay
              else if (progressData.status === 'completed' || progressData.status === 'failed') {
                // อัพเดทให้แสดง 100% ก่อน
                newMap.set(progressKey, progressData)

                // Invalidate video queries และ queue stats เพื่อ refresh
                setTimeout(() => {
                  queryClient.invalidateQueries({ queryKey: videoKeys.all })
                  queryClient.invalidateQueries({ queryKey: queueKeys.all })
                }, 500)

                // ลบออกหลังจาก 3 วินาที (ลดเวลาลง)
                setTimeout(() => {
                  setActiveProgress(current => {
                    const updated = new Map(current)
                    updated.delete(progressKey)
                    return updated
                  })
                }, 3000)
              } else {
                newMap.set(progressKey, progressData)
              }

              return newMap
            })
            break
          }

          case 'subtitle_progress': {
            const progressData = message.data as VideoProgress
            progressData.timestamp = Date.now()
            // ใช้ videoId เป็น key หลัก (ไม่แยกตาม language เพื่อป้องกัน key ไม่ตรงกัน)
            const progressKey = `${progressData.videoId}-subtitle`
            console.log('[WebSocket] Subtitle progress:', progressData.videoCode, progressData.progress + '%', progressData.currentStep, `(${progressData.language || 'detecting'})`)

            setActiveProgress(prev => {
              const newMap = new Map(prev)

              // ถ้าเสร็จหรือ fail ให้ลบออกหลังจาก delay
              if (progressData.status === 'completed' || progressData.status === 'failed') {
                newMap.set(progressKey, progressData)

                // Invalidate subtitle, video และ queue queries เพื่อ refresh
                setTimeout(() => {
                  queryClient.invalidateQueries({ queryKey: ['subtitle', 'video', progressData.videoId] })
                  queryClient.invalidateQueries({ queryKey: videoKeys.lists() })
                  queryClient.invalidateQueries({ queryKey: videoKeys.detail(progressData.videoId) })
                  queryClient.invalidateQueries({ queryKey: queueKeys.all })
                }, 500)

                // ลบออกหลังจาก 3 วินาที
                setTimeout(() => {
                  setActiveProgress(current => {
                    const updated = new Map(current)
                    updated.delete(progressKey)
                    return updated
                  })
                }, 3000)
              } else {
                newMap.set(progressKey, progressData)
              }

              return newMap
            })
            break
          }

          case 'reel_progress': {
            const progressData = message.data as ReelProgress
            progressData.timestamp = Date.now()
            console.log('[WebSocket] Reel progress:', progressData.reelId, progressData.videoCode, progressData.progress + '%', progressData.currentStep)

            // Invalidate queue stats เมื่อ reel เสร็จหรือ fail
            if (progressData.status === 'completed' || progressData.status === 'failed') {
              setTimeout(() => {
                queryClient.invalidateQueries({ queryKey: queueKeys.all })
                // Invalidate reel queries ด้วย
                queryClient.invalidateQueries({ queryKey: ['reel'] })
              }, 500)
            }
            break
          }

          case 'pong':
            // Heartbeat response
            break

          case 'room_joined':
            console.log('[WebSocket] Joined room:', message.data)
            break

          default:
            console.log('[WebSocket] Unknown message type:', message.type)
        }
      } catch (error) {
        console.error('[WebSocket] Error parsing message:', error)
      }
    },
    [queryClient]
  )

  // Keep message handler ref updated
  messageHandlerRef.current = handleMessage

  const connect = useCallback(() => {
    // ถ้า connection ยังอยู่ ใช้ต่อได้เลย
    if (globalWs && (globalWs.readyState === WebSocket.OPEN || globalWs.readyState === WebSocket.CONNECTING)) {
      setIsConnected(globalWs.readyState === WebSocket.OPEN)
      // Update message handler to use latest callback
      globalWs.onmessage = (e) => messageHandlerRef.current?.(e)
      return
    }

    // Clean up existing
    if (globalWs) {
      globalWs.close()
      globalWs = null
    }

    try {
      const wsUrl = `${WS_URL}/ws?room=analytics`
      console.log('[WebSocket] Connecting to', wsUrl)
      const ws = new WebSocket(wsUrl)

      ws.onopen = () => {
        console.log('[WebSocket] Connected (singleton)')
        reconnectAttempts = 0
        setIsConnected(true)
        // Clear stale progress data on reconnect
        setActiveProgress(new Map())
      }

      ws.onmessage = (e) => messageHandlerRef.current?.(e)

      ws.onclose = (event) => {
        console.log('[WebSocket] Disconnected:', event.code, event.reason)
        globalWs = null
        setIsConnected(false)

        // Auto reconnect with exponential backoff
        if (reconnectAttempts < maxReconnectAttempts) {
          const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), 30000)
          reconnectAttempts++
          console.log(`[WebSocket] Reconnecting in ${delay}ms (attempt ${reconnectAttempts})`)

          globalReconnectTimeout = setTimeout(() => {
            connect()
          }, delay)
        }
      }

      ws.onerror = (error) => {
        console.error('[WebSocket] Error:', error)
      }

      globalWs = ws
    } catch (error) {
      console.error('[WebSocket] Failed to create connection:', error)
    }
  }, [])

  // Send message
  const sendMessage = useCallback((type: string, data: unknown) => {
    if (globalWs?.readyState === WebSocket.OPEN) {
      globalWs.send(JSON.stringify({ type, data }))
    }
  }, [])

  // Heartbeat
  useEffect(() => {
    const heartbeatInterval = setInterval(() => {
      if (globalWs?.readyState === WebSocket.OPEN) {
        globalWs.send(JSON.stringify({ type: 'ping' }))
      }
    }, 30000)

    return () => clearInterval(heartbeatInterval)
  }, [])

  // Connect on mount with delayed cleanup for StrictMode
  useEffect(() => {
    // Cancel pending cleanup (StrictMode re-mount)
    if (cleanupTimeout) {
      clearTimeout(cleanupTimeout)
      cleanupTimeout = null
    }

    connect()

    return () => {
      // Delay cleanup เพื่อ handle StrictMode double-mount
      cleanupTimeout = setTimeout(() => {
        if (globalReconnectTimeout) {
          clearTimeout(globalReconnectTimeout)
          globalReconnectTimeout = null
        }
        if (globalWs) {
          globalWs.close()
          globalWs = null
        }
        cleanupTimeout = null
      }, 100)
    }
  }, [connect])

  return (
    <WebSocketContext.Provider value={{ isConnected, sendMessage, reconnect: connect, activeProgress }}>
      {children}
    </WebSocketContext.Provider>
  )
}

// Hook to use WebSocket context
export function useWebSocketConnection() {
  const context = useContext(WebSocketContext)
  if (!context) {
    throw new Error('useWebSocketConnection must be used within WebSocketProvider')
  }
  return context
}

// Hook สำหรับดึง active progress ของ video ที่กำลังประมวลผล
// กรอง stale entries ที่เก่ากว่า 2 นาทีออก
export function useVideoProgress() {
  const { activeProgress } = useWebSocketConnection()

  // กรอง entries ที่ยังไม่หมดอายุ (2 นาที)
  const STALE_THRESHOLD = 2 * 60 * 1000 // 2 minutes
  const now = Date.now()

  const freshProgress = new Map<string, VideoProgress>()
  activeProgress.forEach((value, key) => {
    // ถ้าไม่มี timestamp หรือยังไม่หมดอายุ ให้แสดง
    if (!value.timestamp || (now - value.timestamp) < STALE_THRESHOLD) {
      freshProgress.set(key, value)
    }
  })

  return freshProgress
}

// Hook สำหรับดึง progress ของ video เฉพาะตัว
// ใช้ composite key: videoId-type (default: transcode)
export function useVideoProgressById(videoId: string, type: ProgressType = 'transcode') {
  const { activeProgress } = useWebSocketConnection()
  return activeProgress.get(`${videoId}-${type}`) ?? null
}

// Hook สำหรับดึง subtitle progress ของ video
// ใช้ key: videoId-subtitle
export function useSubtitleProgress(videoId: string) {
  const { activeProgress } = useWebSocketConnection()
  const progress = activeProgress.get(`${videoId}-subtitle`)
  return progress ? [progress] : []
}
