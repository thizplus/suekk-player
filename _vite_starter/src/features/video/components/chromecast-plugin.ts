/**
 * Chromecast Plugin Wrapper for ArtPlayer
 * แก้ปัญหา Cast SDK โหลดซ้ำเมื่อ React re-render
 */

// Track if SDK is loaded globally
let castSdkLoaded = false
let castSdkLoading: Promise<void> | null = null

// Default Cast icon
const CAST_ICON = `<svg height="20" width="20" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 576 512"><path d="M512 96H64v99c-13-2-26.4-3-40-3H0V96C0 60.7 28.7 32 64 32H512c35.3 0 64 28.7 64 64V416c0 35.3-28.7 64-64 64H288V456c0-13.6-1-27-3-40H512V96zM24 224c128.1 0 232 103.9 232 232c0 13.3-10.7 24-24 24s-24-10.7-24-24c0-101.6-82.4-184-184-184c-13.3 0-24-10.7-24-24s10.7-24 24-24zm8 192a32 32 0 1 1 0 64 32 32 0 1 1 0-64zM0 344c0-13.3 10.7-24 24-24c75.1 0 136 60.9 136 136c0 13.3-10.7 24-24 24s-24-10.7-24-24c0-48.6-39.4-88-88-88c-13.3 0-24-10.7-24-24z"/></svg>`

interface SubtitleTrack {
  url: string      // SRT URL (จะแปลงเป็น VTT อัตโนมัติ)
  language: string // e.g., 'th', 'en', 'ja'
  name: string     // e.g., 'ไทย', 'English'
}

interface ChromecastOptions {
  sdk?: string
  url?: string
  mimeType?: string
  icon?: string
  token?: string // JWT token สำหรับ HLS authentication
  subtitles?: SubtitleTrack[] // Subtitles สำหรับ Chromecast (จะใช้ VTT)
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

/**
 * Convert SRT URL to VTT URL (replace extension)
 */
function toVttUrl(srtUrl: string): string {
  // Replace .srt with .vtt
  if (srtUrl.endsWith('.srt')) {
    return srtUrl.slice(0, -4) + '.vtt'
  }
  // If already .vtt or other format, return as-is
  return srtUrl
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
function loadMedia(session: any, url: string, mimeType: string, art: any, subtitles?: SubtitleTrack[], token?: string) {
  const mediaInfo = new window.chrome.cast.media.MediaInfo(url, mimeType)

  // Set stream type for HLS
  mediaInfo.streamType = window.chrome.cast.media.StreamType.BUFFERED

  // Set HLS specific content type
  if (url.includes('.m3u8')) {
    mediaInfo.contentType = 'application/x-mpegurl'
    mediaInfo.hlsSegmentFormat = window.chrome.cast.media.HlsSegmentFormat.TS
    mediaInfo.hlsVideoSegmentFormat = window.chrome.cast.media.HlsVideoSegmentFormat.MPEG2_TS
  }

  // Add subtitle tracks (VTT format for Chromecast)
  // Prioritize Thai (th) subtitle
  let thaiTrackId = 1 // Default to first track

  if (subtitles && subtitles.length > 0) {
    const tracks: chrome.cast.media.Track[] = []

    subtitles.forEach((sub, index) => {
      // Convert SRT URL to VTT URL
      let vttUrl = toVttUrl(sub.url)

      // Append token to VTT URL if needed
      if (token) {
        const separator = vttUrl.includes('?') ? '&' : '?'
        vttUrl = `${vttUrl}${separator}token=${token}`
      }

      const trackId = index + 1 // trackId (1-based)

      // Remember Thai track ID
      if (sub.language === 'th') {
        thaiTrackId = trackId
      }

      const track = new window.chrome.cast.media.Track(
        trackId,
        window.chrome.cast.media.TrackType.TEXT
      )
      track.trackContentId = vttUrl
      track.trackContentType = 'text/vtt'
      track.subtype = window.chrome.cast.media.TextTrackType.SUBTITLES
      track.name = sub.name
      track.language = sub.language

      tracks.push(track)
      console.log('[Chromecast] Added subtitle track:', { language: sub.language, trackId, url: vttUrl })
    })

    mediaInfo.tracks = tracks

    // Auto-enable Thai subtitle track (or first if no Thai)
    mediaInfo.activeTrackIds = [thaiTrackId]
    console.log('[Chromecast] Default subtitle trackId:', thaiTrackId)
  }

  const request = new window.chrome.cast.media.LoadRequest(mediaInfo)

  console.log('[Chromecast] Loading media:', {
    url,
    mimeType,
    streamType: mediaInfo.streamType,
    subtitleTracks: subtitles?.length || 0,
  })

  session.loadMedia(request).then(
    () => {
      console.log('[Chromecast] Media loaded successfully')
      art.notice.show = 'กำลังแคสต์...'

      // Force enable subtitle after media loaded
      // Retry multiple times as media session might take time to be ready
      if (subtitles && subtitles.length > 0) {
        let retries = 0
        const maxRetries = 5

        const tryEnableSubtitle = () => {
          retries++
          try {
            const media = session.getMediaSession()
            console.log(`[Chromecast] Media session (attempt ${retries}):`, media ? 'available' : 'null')

            if (media) {
              // Create track info request to enable Thai subtitle (or first if no Thai)
              const trackInfoRequest = new window.chrome.cast.media.EditTracksInfoRequest([thaiTrackId])

              // Optional: Set text track style for better visibility
              const textTrackStyle = new window.chrome.cast.media.TextTrackStyle()
              textTrackStyle.backgroundColor = '#00000080' // Semi-transparent black
              textTrackStyle.foregroundColor = '#FFFFFFFF' // White text
              textTrackStyle.fontScale = 1.2
              textTrackStyle.fontFamily = 'sans-serif'
              trackInfoRequest.textTrackStyle = textTrackStyle

              media.editTracksInfo(trackInfoRequest,
                () => console.log('[Chromecast] Subtitle enabled successfully'),
                (err: Error) => console.error('[Chromecast] Failed to enable subtitle:', err)
              )
            } else if (retries < maxRetries) {
              console.log(`[Chromecast] Retrying in 2 seconds... (${retries}/${maxRetries})`)
              setTimeout(tryEnableSubtitle, 2000)
            } else {
              console.warn('[Chromecast] Media session not available after max retries. Try enabling subtitle from TV remote.')
            }
          } catch (err) {
            console.error('[Chromecast] Error enabling subtitle:', err)
          }
        }

        // Start first attempt after 2 seconds
        setTimeout(tryEnableSubtitle, 2000)
      }
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
                loadMedia(session, url, mimeType, art, options.subtitles, options.token)
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
  // eslint-disable-next-line @typescript-eslint/no-namespace
  namespace chrome.cast.media {
    interface Track {
      trackId: number
      trackContentId: string
      trackContentType: string
      subtype: unknown
      name: string
      language: string
    }
  }

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
            tracks: chrome.cast.media.Track[]
            activeTrackIds: number[]
          }
          LoadRequest: new (mediaInfo: unknown) => unknown
          Track: new (trackId: number, trackType: unknown) => chrome.cast.media.Track
          TrackType: {
            TEXT: unknown
            AUDIO: unknown
            VIDEO: unknown
          }
          TextTrackType: {
            SUBTITLES: unknown
            CAPTIONS: unknown
            DESCRIPTIONS: unknown
            CHAPTERS: unknown
            METADATA: unknown
          }
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
          EditTracksInfoRequest: new (activeTrackIds: number[]) => {
            textTrackStyle: unknown
          }
          TextTrackStyle: new () => {
            backgroundColor: string
            foregroundColor: string
            fontScale: number
            fontFamily: string
          }
        }
        AutoJoinPolicy: {
          ORIGIN_SCOPED: unknown
        }
      }
    }
  }
}
