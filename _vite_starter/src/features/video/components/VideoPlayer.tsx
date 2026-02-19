import { useEffect, useRef, useMemo } from 'react'
import Artplayer from 'artplayer'
import Hls from 'hls.js'
import artplayerPluginMultipleSubtitles from 'artplayer-plugin-multiple-subtitles'
import artplayerPluginChromecast from './chromecast-plugin'

// HLS instance type
type HlsInstance = InstanceType<typeof Hls>

// Subtitle option interface
export interface SubtitleOption {
  url: string
  name: string
  language: string
  default?: boolean
}

// Thai language translations
const thaiI18n = {
  'Video Info': 'ข้อมูลวิดีโอ',
  Close: 'ปิด',
  'Video Load Failed': 'โหลดวิดีโอล้มเหลว',
  Volume: 'ระดับเสียง',
  Play: 'เล่น',
  Pause: 'หยุด',
  Rate: 'ความเร็ว',
  Mute: 'ปิดเสียง',
  'Video Flip': 'พลิกวิดีโอ',
  Horizontal: 'แนวนอน',
  Vertical: 'แนวตั้ง',
  Reconnect: 'เชื่อมต่อใหม่',
  'Show Setting': 'แสดงการตั้งค่า',
  'Hide Setting': 'ซ่อนการตั้งค่า',
  Screenshot: 'ถ่ายภาพหน้าจอ',
  'Play Speed': 'ความเร็วเล่น',
  'Aspect Ratio': 'อัตราส่วนภาพ',
  Default: 'ค่าเริ่มต้น',
  Normal: 'ปกติ',
  Open: 'เปิด',
  'Switch Video': 'เปลี่ยนวิดีโอ',
  'Switch Subtitle': 'เปลี่ยนคำบรรยาย',
  Fullscreen: 'เต็มจอ',
  'Exit Fullscreen': 'ออกจากเต็มจอ',
  'Web Fullscreen': 'เต็มจอเว็บ',
  'Exit Web Fullscreen': 'ออกจากเต็มจอเว็บ',
  'Mini Player': 'หน้าต่างเล็ก',
  'PIP Mode': 'โหมด PIP',
  'Exit PIP Mode': 'ออกจากโหมด PIP',
  'PIP Not Supported': 'ไม่รองรับ PIP',
  'Fullscreen Not Supported': 'ไม่รองรับเต็มจอ',
  'Subtitle Offset': 'ปรับเวลาคำบรรยาย',
  'Last Seen': 'ดูล่าสุด',
  'Jump Play': 'ข้ามไปเล่น',
  AirPlay: 'AirPlay',
  'AirPlay Not Available': 'AirPlay ไม่พร้อมใช้งาน',
}

interface VideoPlayerProps {
  src: string
  poster?: string
  streamToken?: string
  autoPlay?: boolean
  subtitles?: SubtitleOption[]
  /** ใช้ native subtitle แทน plugin (สำหรับ editor ที่ต้องการ dynamic update) */
  dynamicSubtitle?: boolean
  onPlay?: () => void
  onPause?: () => void
  onEnded?: () => void
  onTimeUpdate?: (currentTime: number, duration: number) => void
  /** Callback เมื่อ player พร้อมใช้งาน - ส่ง art instance มาให้ */
  onReady?: (art: Artplayer) => void
}

export function VideoPlayer({
  src,
  poster,
  streamToken,
  autoPlay = false,
  subtitles = [],
  dynamicSubtitle = false,
  onPlay,
  onPause,
  onEnded,
  onTimeUpdate,
  onReady,
}: VideoPlayerProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const artRef = useRef<Artplayer | null>(null)

  // Find default subtitle
  const defaultSubtitle = subtitles.find(s => s.default) || subtitles[0]

  // Build subtitle config for plugin
  const subtitleConfig = useMemo(() => {
    if (subtitles.length === 0) return []
    return subtitles.map(sub => ({
      name: sub.language, // ใช้ language code เป็น name (สำหรับ plugin)
      url: sub.url,
      type: 'srt' as const, // ต้องระบุ type เพราะ blob URL ไม่มี extension
    }))
  }, [subtitles])

  // Store art reference for callbacks (will be set after player creation)
  const artInstanceRef = useRef<Artplayer | null>(null)

  // Track if initial HLS loading started (ป้องกัน re-create player เมื่อ subtitles โหลดเสร็จ)
  const initializedRef = useRef(false)
  const prevSrcRef = useRef<string | null>(null)

  useEffect(() => {
    if (!containerRef.current || !src) return

    // ถ้า src ไม่เปลี่ยน และ player สร้างแล้ว → ไม่ต้อง recreate
    // เพราะ subtitles สามารถโหลดแยกได้ทีหลัง
    if (initializedRef.current && prevSrcRef.current === src && artRef.current) {
      console.log('[VideoPlayer] Skipping recreation - src unchanged, player exists')
      return
    }

    prevSrcRef.current = src

    // Store HLS instance
    let hlsInstance: HlsInstance | null = null

    // HLS.js custom loader with token support
    const playM3u8 = (video: HTMLVideoElement, url: string, art: Artplayer) => {
      if (Hls.isSupported()) {
        if (hlsInstance) hlsInstance.destroy()
        const hls = new Hls({
          // ═══════════════════════════════════════════════════════
          // SEEK FAST OPTIMIZATION - Phase 2 (Fix Cancel Delay)
          // ═══════════════════════════════════════════════════════

          // Web Worker (offload parsing from UI thread)
          enableWorker: true,

          // Buffer Management (ลดให้เหมาะกับ seek)
          maxBufferLength: 30,              // Prefetch 30 seconds ahead
          maxMaxBufferLength: 60,           // Max buffer cap
          backBufferLength: 10,             // ลดจาก 30 → 10 (ไม่ต้องเก็บ history มาก)
          maxBufferSize: 60 * 1000 * 1000,  // 60 MB max buffer
          maxBufferHole: 0.5,               // Allow 0.5s gaps

          // ═══════════════════════════════════════════════════════
          // CRITICAL: Fast Abort Settings (แก้ปัญหา 700ms cancel)
          // ═══════════════════════════════════════════════════════
          maxLoadingDelay: 2,               // ลดจาก 4 → 2 (abort เร็วขึ้น)
          highBufferWatchdogPeriod: 1,      // Check buffer ทุก 1 วินาที (default: 3)

          // Performance & Fast Start
          startLevel: -1,                   // Auto-select quality
          capLevelToPlayerSize: true,       // Don't load 1080p on small player
          startFragPrefetch: true,          // Prefetch first fragment

          // ABR (Adaptive Bitrate)
          abrEwmaDefaultEstimate: 5_000_000,  // 5 Mbps default estimate
          abrBandWidthFactor: 0.95,           // Use 95% of measured bandwidth
          abrBandWidthUpFactor: 0.7,          // Conservative when upgrading quality
          testBandwidth: true,

          // Timeout settings (ลดลงเพื่อ fail fast)
          manifestLoadingTimeOut: 15000,      // 15 วินาที (ลดจาก 20)
          fragLoadingTimeOut: 15000,          // 15 วินาที (ลดจาก 20)
          levelLoadingTimeOut: 15000,         // 15 วินาที (ลดจาก 20)

          // Reliability & Retry
          manifestLoadingMaxRetry: 3,         // ลดจาก 4
          levelLoadingMaxRetry: 3,            // ลดจาก 4
          fragLoadingMaxRetry: 4,             // ลดจาก 6
          fragLoadingMaxRetryTimeout: 32000,  // ลดจาก 64000

          // Seek Optimization
          nudgeOffset: 0.1,
          nudgeMaxRetry: 5,
          maxFragLookUpTolerance: 0.25,

          // Token Header
          xhrSetup: (xhr: XMLHttpRequest) => {
            if (streamToken) {
              xhr.setRequestHeader('X-Stream-Token', streamToken)
            }
          },
        })

        // Track if auto mode is enabled
        let isAutoMode = true

        // Setup quality selector when HLS levels are available
        hls.on(Hls.Events.MANIFEST_PARSED, (_event, data) => {
          console.log('[HLS] MANIFEST_PARSED - levels:', data.levels.length, data.levels.map(l => `${l.height}p`))

          if (data.levels.length <= 1) {
            console.log('[HLS] Only 1 level, skipping quality selector')
            return
          }

          // Build quality options - sort by height descending
          const sortedLevels = [...data.levels].sort((a, b) => b.height - a.height)

          const qualitySelector = [
            { html: 'อัตโนมัติ', value: -1, default: true },
            ...sortedLevels.map((level) => ({
              html: `${level.height}p`,
              value: data.levels.findIndex(l => l.height === level.height),
            })),
          ]

          const onQualitySelect = function (item: { html: string; value: number }) {
            console.log('[Quality] Selected:', item.html, 'level:', item.value)
            hls.currentLevel = item.value
            isAutoMode = item.value === -1
            return item.html
          }

          // Add quality to settings panel
          art.setting.add({
            name: 'quality',
            html: 'คุณภาพ',
            icon: `<svg viewBox="0 0 24 24" width="22" height="22" fill="currentColor">
              <path d="M19 3H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2zm-8 12H9.5v-2h-2v2H6V9h1.5v2.5h2V9H11v6zm7-1c0 .55-.45 1-1 1h-.75v1.5h-1.5V15H14c-.55 0-1-.45-1-1v-4c0-.55.45-1 1-1h3c.55 0 1 .45 1 1v4zm-3.5-.5h2v-3h-2v3z"/>
            </svg>`,
            tooltip: 'เลือกคุณภาพวิดีโอ',
            selector: qualitySelector,
            onSelect: function (item) {
              return onQualitySelect(item as { html: string; value: number })
            },
          })
          console.log('[ArtPlayer] Quality setting added')

          console.log('[ArtPlayer] Quality options:', data.levels.map(l => `${l.height}p`))

        })

        // Log when HLS switches level (for auto mode)
        hls.on(Hls.Events.LEVEL_SWITCHED, (_event, data) => {
          if (isAutoMode) {
            const level = hls.levels[data.level]
            if (level) {
              console.log('[Quality] Auto switched to:', `${level.height}p`)
            }
          }
        })

        // Debug: log HLS errors
        hls.on(Hls.Events.ERROR, (_event, data) => {
          console.error('[HLS] Error:', data.type, data.details, data)
        })

        console.log('[HLS] Loading source:', url)
        hls.loadSource(url)
        hls.attachMedia(video)
        hlsInstance = hls
        art.on('destroy', () => hls.destroy())

        // ═══════════════════════════════════════════════════════
        // SEEK HANDLER: Abort loading immediately when user seeks
        // แก้ปัญหา request ค้าง 700ms ก่อน cancel
        // ═══════════════════════════════════════════════════════
        art.on('seek', () => {
          console.log('[HLS] Seek detected - aborting current loading')
          // Stop current fragment loading immediately
          hls.stopLoad()
          // Resume loading at new position
          setTimeout(() => {
            hls.startLoad()
          }, 50)
        })
      } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
        // Native HLS support (Safari)
        video.src = url
      } else {
        art.notice.show = 'ไม่รองรับ HLS ในเบราว์เซอร์นี้'
      }
    }

    const art = new Artplayer({
      container: containerRef.current,
      url: src,
      poster: poster || '',
      autoplay: autoPlay,
      volume: 0.7,
      muted: false,
      autoSize: false,
      autoMini: false,
      loop: false,
      flip: false,
      playbackRate: true,
      aspectRatio: false,
      setting: true,
      hotkey: true,
      pip: false,
      fullscreen: true,
      fullscreenWeb: false,
      subtitleOffset: false,
      miniProgressBar: true,
      mutex: true,
      backdrop: true,
      playsInline: true,
      autoPlayback: true,
      airplay: true,
      theme: '#e50914', // Netflix red
      lang: navigator.language.startsWith('th') ? 'th' : 'en',
      i18n: {
        th: thaiI18n,
      },
      moreVideoAttr: {
        crossOrigin: 'anonymous',
        playsInline: true,
      },
      // Custom HLS handler
      customType: {
        m3u8: playM3u8,
      },
      // Native subtitle config (สำหรับ dynamicSubtitle mode เท่านั้น)
      ...(dynamicSubtitle && defaultSubtitle ? {
        subtitle: {
          url: defaultSubtitle.url,
          type: 'srt',
          encoding: 'utf-8',
          style: {
            color: '#fff',
            fontSize: '20px',
            textShadow: '0 1px 2px rgba(0,0,0,0.8)',
          },
        },
      } : {}),
      // Plugins - Multiple Subtitles + Chromecast
      plugins: [
        // Chromecast plugin (รองรับ HLS auto-detect, ส่ง token ผ่าน query param, VTT subtitles)
        artplayerPluginChromecast({
          token: streamToken,
          subtitles: subtitles.map(sub => ({
            url: sub.url,
            language: sub.language,
            name: sub.name,
          })),
        }),
        // Multiple Subtitles (ไม่ใช้เมื่อ dynamicSubtitle = true)
        ...(!dynamicSubtitle && subtitleConfig.length > 0 ? [
          artplayerPluginMultipleSubtitles({
            subtitles: subtitleConfig,
          }),
        ] : []),
      ],
      // Settings - ไม่ใส่ subtitle ใน settings แล้ว (ย้ายไป controls)
      settings: [],
      // Controls
      controls: [
        {
          name: 'skip-back',
          position: 'left',
          index: 11, // หลัง timing (10), ก่อน volume
          html: `<svg viewBox="0 0 24 24" width="22" height="22" fill="currentColor">
            <path d="M12 5V1L7 6l5 5V7c3.31 0 6 2.69 6 6s-2.69 6-6 6-6-2.69-6-6H4c0 4.42 3.58 8 8 8s8-3.58 8-8-3.58-8-8-8z"/>
            <text x="12" y="16.5" font-size="7.5" font-weight="bold" text-anchor="middle">10</text>
          </svg>`,
          tooltip: 'ย้อนกลับ 10 วินาที',
          click: function () {
            art.seek = Math.max(0, art.currentTime - 10)
          },
        },
        {
          name: 'skip-forward',
          position: 'left',
          index: 12, // หลัง skip-back
          html: `<svg viewBox="0 0 24 24" width="22" height="22" fill="currentColor">
            <path d="M12 5V1l5 5-5 5V7c-3.31 0-6 2.69-6 6s2.69 6 6 6 6-2.69 6-6h2c0 4.42-3.58 8-8 8s-8-3.58-8-8 3.58-8 8-8z"/>
            <text x="12" y="16.5" font-size="7.5" font-weight="bold" text-anchor="middle">10</text>
          </svg>`,
          tooltip: 'ข้ามไป 10 วินาที',
          click: function () {
            art.seek = Math.min(art.duration, art.currentTime + 10)
          },
        },
      ],
    })

    artRef.current = art
    artInstanceRef.current = art // Set ref for settings callbacks
    initializedRef.current = true // Mark as initialized to prevent recreation

    // Call onReady callback when player is ready
    art.on('ready', () => {
      console.log('[ArtPlayer] Ready')
      onReady?.(art)
    })

    // Debug subtitle plugin
    if (subtitles.length > 0) {
      console.log('[ArtPlayer] Subtitles config:', subtitleConfig)
      console.log('[ArtPlayer] Plugins:', art.plugins)

      art.on('ready', () => {
        console.log('[ArtPlayer] Ready - checking subtitle plugin...')
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const plugin = (art.plugins as any).multipleSubtitles
        console.log('[ArtPlayer] multipleSubtitles plugin:', plugin)

        // Track current active subtitle
        let activeSubtitleLang: string | null = null

        // Set default to show only ONE subtitle language (not all)
        if (plugin && subtitles.length > 0) {
          const defaultLang = defaultSubtitle?.language || subtitles[0]?.language
          if (defaultLang) {
            console.log('[ArtPlayer] Setting default subtitle language:', defaultLang)
            plugin.tracks([defaultLang])
            activeSubtitleLang = defaultLang
          }
        }

        // Enable subtitle display
        if (art.subtitle) {
          art.subtitle.show = true
          console.log('[ArtPlayer] Subtitle enabled')
        }

        // Add subtitle to settings (only if subtitles exist)
        if (subtitles.length > 0 && plugin) {
          const subtitleSelector = [
            { html: 'ปิด', value: '', default: !activeSubtitleLang },
            ...subtitles.map(sub => ({
              html: sub.name,
              value: sub.language,
              default: sub.language === activeSubtitleLang,
            })),
          ]

          const onSubtitleSelect = function (item: { html: string; value: string }) {
            console.log('[Subtitle] Selected:', item.html, 'lang:', item.value)

            if (item.value === '') {
              // Turn off subtitles
              plugin.tracks([])
              if (art.subtitle) {
                art.subtitle.show = false
              }
              activeSubtitleLang = null
            } else {
              // Show selected subtitle
              plugin.tracks([item.value])
              if (art.subtitle) {
                art.subtitle.show = true
              }
              activeSubtitleLang = item.value
            }

            return item.html
          }

          // Add subtitle to settings panel
          art.setting.add({
            name: 'subtitle',
            html: 'คำบรรยาย',
            icon: `<svg viewBox="0 0 24 24" width="22" height="22" fill="currentColor">
              <path d="M20 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 14H4V6h16v12zM6 10h2v2H6zm0 4h8v2H6zm10 0h2v2h-2zm-6-4h8v2h-8z"/>
            </svg>`,
            tooltip: 'เลือกคำบรรยาย',
            selector: subtitleSelector,
            onSelect: function (item) {
              return onSubtitleSelect(item as { html: string; value: string })
            },
          })

          console.log('[ArtPlayer] Subtitle setting added:', subtitles.map(s => s.name))
        }
      })
    }

    // Event listeners
    art.on('play', () => onPlay?.())
    art.on('pause', () => onPause?.())
    art.on('ended', () => onEnded?.())
    art.on('video:timeupdate', () => {
      onTimeUpdate?.(art.currentTime, art.duration)
    })


    art.on('error', (error: Error) => {
      console.error('[ArtPlayer] Error:', error)
    })

    return () => {
      if (artRef.current) {
        artRef.current.destroy(false)
        artRef.current = null
      }
      initializedRef.current = false
    }
  // Note: subtitleConfig/defaultSubtitle removed from deps to prevent player recreation
  // when subtitle blobs finish loading. Subtitles are loaded on first render only.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [src, streamToken])

  // === Dynamic Subtitle Update (for real-time editing) ===
  // เมื่อ subtitle URL เปลี่ยน (เช่น จากการแก้ไข) ให้ update player โดยไม่ต้อง recreate
  const prevSubtitleUrlRef = useRef<string | null>(null)

  useEffect(() => {
    // Only update dynamically when dynamicSubtitle mode is enabled
    if (!dynamicSubtitle) return

    const art = artRef.current
    if (!art || subtitles.length === 0) return

    // ใช้ subtitle ตัวแรก (สำหรับ editor ที่มีแค่ภาษาเดียว)
    const currentSubtitle = subtitles[0]
    if (!currentSubtitle?.url) return

    // Skip if URL hasn't changed (prevent infinite loop)
    if (prevSubtitleUrlRef.current === currentSubtitle.url) return
    prevSubtitleUrlRef.current = currentSubtitle.url

    // Update subtitle URL dynamically using native API
    try {
      if (art.subtitle) {
        art.subtitle.switch(currentSubtitle.url, { type: 'srt' })
        art.subtitle.show = true
        console.log('[VideoPlayer] Dynamic subtitle updated:', currentSubtitle.url)
      }
    } catch (error) {
      console.error('[VideoPlayer] Failed to update subtitle:', error)
    }
  }, [dynamicSubtitle, subtitles])

  return (
    <div
      ref={containerRef}
      className="artplayer-container"
    />
  )
}

