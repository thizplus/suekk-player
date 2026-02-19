/**
 * Chromecast Plugin Wrapper for ArtPlayer
 * แก้ปัญหา Cast SDK โหลดซ้ำเมื่อ React re-render
 */

// Track if SDK is loaded globally
let castSdkLoaded = false
let castSdkLoading: Promise<void> | null = null

// Default Cast icon
const CAST_ICON = `<svg height="20" width="20" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 576 512"><path d="M512 96H64v99c-13-2-26.4-3-40-3H0V96C0 60.7 28.7 32 64 32H512c35.3 0 64 28.7 64 64V416c0 35.3-28.7 64-64 64H288V456c0-13.6-1-27-3-40H512V96zM24 224c128.1 0 232 103.9 232 232c0 13.3-10.7 24-24 24s-24-10.7-24-24c0-101.6-82.4-184-184-184c-13.3 0-24-10.7-24-24s10.7-24 24-24zm8 192a32 32 0 1 1 0 64 32 32 0 1 1 0-64zM0 344c0-13.3 10.7-24 24-24c75.1 0 136 60.9 136 136c0 13.3-10.7 24-24 24s-24-10.7-24-24c0-48.6-39.4-88-88-88c-13.3 0-24-10.7-24-24z"/></svg>`

interface ChromecastOptions {
  sdk?: string
  url?: string
  mimeType?: string
  icon?: string
  token?: string // JWT token สำหรับ HLS authentication
}

// MIME type mapping
const MIME_TYPES: Record<string, string> = {
  mp4: 'video/mp4',
  webm: 'video/webm',
  ogg: 'video/ogg',
  ogv: 'video/ogg',
  mp3: 'audio/mp3',
  wav: 'audio/wav',
  flv: 'video/x-flv',
  mov: 'video/quicktime',
  avi: 'video/x-msvideo',
  wmv: 'video/x-ms-wmv',
  mpd: 'application/dash+xml',
  m3u8: 'application/x-mpegURL',
}

function getMimeType(url: string): string {
  const ext = url.split('?')[0].split('#')[0].split('.').pop()?.toLowerCase() || ''
  return MIME_TYPES[ext] || 'application/octet-stream'
}

function loadCastSdk(sdkUrl: string): Promise<void> {
  // Already loaded
  if (castSdkLoaded && window.cast?.framework) {
    return Promise.resolve()
  }

  // Currently loading - return existing promise
  if (castSdkLoading) {
    return castSdkLoading
  }

  // Start loading
  castSdkLoading = new Promise((resolve, reject) => {
    // Setup callback BEFORE loading script
    window.__onGCastApiAvailable = (isAvailable: boolean) => {
      if (isAvailable && window.cast?.framework) {
        try {
          window.cast.framework.CastContext.getInstance().setOptions({
            receiverApplicationId: window.chrome.cast.media.DEFAULT_MEDIA_RECEIVER_APP_ID,
            autoJoinPolicy: window.chrome.cast.AutoJoinPolicy.ORIGIN_SCOPED,
          })
          castSdkLoaded = true
          console.log('[Chromecast] SDK initialized successfully')
          resolve()
        } catch (err) {
          console.error('[Chromecast] Failed to initialize:', err)
          reject(err)
        }
      } else {
        reject(new Error('Cast SDK not available'))
      }
    }

    // Check if script already exists
    const existingScript = document.querySelector(`script[src*="cast_sender.js"]`)
    if (existingScript) {
      // Script exists, wait for callback
      console.log('[Chromecast] SDK script already exists, waiting for init...')
      return
    }

    // Load script
    const script = document.createElement('script')
    script.src = sdkUrl
    script.async = true
    script.onerror = () => {
      castSdkLoading = null
      reject(new Error('Failed to load Cast SDK'))
    }
    document.body.appendChild(script)
    console.log('[Chromecast] Loading SDK...')
  })

  return castSdkLoading
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function loadMedia(session: any, url: string, mimeType: string, art: any) {
  const mediaInfo = new window.chrome.cast.media.MediaInfo(url, mimeType)

  // Set stream type for HLS
  mediaInfo.streamType = window.chrome.cast.media.StreamType.BUFFERED

  // Set HLS specific content type
  if (url.includes('.m3u8')) {
    mediaInfo.contentType = 'application/x-mpegurl'
    mediaInfo.hlsSegmentFormat = window.chrome.cast.media.HlsSegmentFormat.TS
    mediaInfo.hlsVideoSegmentFormat = window.chrome.cast.media.HlsVideoSegmentFormat.MPEG2_TS
  }

  const request = new window.chrome.cast.media.LoadRequest(mediaInfo)

  console.log('[Chromecast] Loading media:', { url, mimeType, streamType: mediaInfo.streamType })

  session.loadMedia(request).then(
    () => {
      console.log('[Chromecast] Media loaded successfully')
      art.notice.show = 'กำลังแคสต์...'
    },
    (error: Error) => {
      console.error('[Chromecast] Error loading media:', error)
      art.notice.show = 'เกิดข้อผิดพลาดในการแคสต์'
    }
  )
}

export default function artplayerPluginChromecast(options: ChromecastOptions = {}) {
  const sdkUrl = options.sdk || 'https://www.gstatic.com/cv/js/sender/v1/cast_sender.js?loadCastFramework=1'

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  return (art: any) => {
    // Pre-load SDK when plugin initializes
    loadCastSdk(sdkUrl).catch(() => {
      // Silently fail - will show error when user clicks
    })

    art.controls.add({
      name: 'chromecast',
      position: 'right',
      index: 5,
      tooltip: 'Chromecast',
      html: `<i class="art-icon art-icon-cast">${options.icon || CAST_ICON}</i>`,
      async click() {
        try {
          // Ensure SDK is loaded
          await loadCastSdk(sdkUrl)

          const castContext = window.cast.framework.CastContext.getInstance()
          const currentSession = castContext.getCurrentSession()
          const sessionState = castContext.getSessionState()

          console.log('[Chromecast] Session state:', sessionState, 'Session:', currentSession ? 'exists' : 'null')

          // If already casting - stop and disconnect
          if (currentSession && sessionState === 'SESSION_STARTED') {
            console.log('[Chromecast] Stopping current session...')
            art.notice.show = 'หยุดการแคสต์...'
            await castContext.endCurrentSession(true)
            console.log('[Chromecast] Session ended')
            art.notice.show = 'หยุดแคสต์แล้ว'
            return
          }

          // Build URL with token
          let url = options.url || art.option.url
          const mimeType = options.mimeType || getMimeType(url)

          if (options.token) {
            const separator = url.includes('?') ? '&' : '?'
            url = `${url}${separator}token=${options.token}`
          }
          console.log('[Chromecast] Casting URL:', url)

          // Request new session
          console.log('[Chromecast] Requesting new session...')
          art.notice.show = 'กำลังเชื่อมต่อ Chromecast...'

          castContext.requestSession().then(
            () => {
              const session = castContext.getCurrentSession()
              if (session) {
                console.log('[Chromecast] New session created')
                loadMedia(session, url, mimeType, art)
              }
            },
            (error: Error) => {
              console.error('[Chromecast] Session error:', error)
              art.notice.show = 'ไม่สามารถเชื่อมต่อ Chromecast ได้'
            }
          )
        } catch (error) {
          console.error('[Chromecast] Error:', error)
          art.notice.show = 'Chromecast ไม่พร้อมใช้งาน'
        }
      },
    })

    return {
      name: 'artplayerPluginChromecast',
    }
  }
}

// Type declarations for Cast SDK
declare global {
  interface Window {
    __onGCastApiAvailable: (isAvailable: boolean) => void
    cast: {
      framework: {
        CastContext: {
          getInstance: () => {
            setOptions: (options: {
              receiverApplicationId: string
              autoJoinPolicy: unknown
            }) => void
            getCurrentSession: () => unknown
            getSessionState: () => string
            requestSession: () => Promise<void>
            endCurrentSession: (stopCasting: boolean) => Promise<void>
          }
        }
      }
    }
    chrome: {
      cast: {
        media: {
          DEFAULT_MEDIA_RECEIVER_APP_ID: string
          MediaInfo: new (url: string, mimeType: string) => {
            streamType: unknown
            contentType: string
            hlsSegmentFormat: unknown
            hlsVideoSegmentFormat: unknown
          }
          LoadRequest: new (mediaInfo: unknown) => unknown
          StreamType: {
            BUFFERED: unknown
            LIVE: unknown
            OTHER: unknown
          }
          HlsSegmentFormat: {
            TS: unknown
            AAC: unknown
            FMP4: unknown
          }
          HlsVideoSegmentFormat: {
            MPEG2_TS: unknown
            FMP4: unknown
          }
        }
        AutoJoinPolicy: {
          ORIGIN_SCOPED: unknown
        }
      }
    }
  }
}
