import { useEffect, useRef, useMemo } from 'react'
import Artplayer from 'artplayer'
import Hls from 'hls.js'
import artplayerPluginMultipleSubtitles from 'artplayer-plugin-multiple-subtitles'

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
  onPlay?: () => void
  onPause?: () => void
  onEnded?: () => void
  onTimeUpdate?: (currentTime: number, duration: number) => void
}

export function VideoPlayer({
  src,
  poster,
  streamToken,
  autoPlay = false,
  subtitles = [],
  onPlay,
  onPause,
  onEnded,
  onTimeUpdate,
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

  useEffect(() => {
    if (!containerRef.current || !src) return

    // Store HLS instance
    let hlsInstance: HlsInstance | null = null

    // HLS.js custom loader with token support
    const playM3u8 = (video: HTMLVideoElement, url: string, art: Artplayer) => {
      if (Hls.isSupported()) {
        if (hlsInstance) hlsInstance.destroy()
        const hls = new Hls({
          // ═══════════════════════════════════════════════════════
          // SEEK FAST OPTIMIZATION - Phase 1
          // ═══════════════════════════════════════════════════════

          // Web Worker (offload parsing from UI thread)
          enableWorker: true,

          // Buffer Management (Critical for Seek Performance)
          maxBufferLength: 60,              // Prefetch 60 seconds ahead
          maxMaxBufferLength: 120,          // Max buffer cap
          backBufferLength: 90,             // Keep 90 sec history for backward seek
          maxBufferSize: 80 * 1000 * 1000,  // 80 MB max buffer
          maxBufferHole: 0.5,               // Allow 0.5s gaps

          // Performance & Fast Start
          startLevel: -1,                   // Auto-select quality
          capLevelToPlayerSize: true,       // Don't load 1080p on small player

          // ABR (Adaptive Bitrate)
          abrEwmaDefaultEstimate: 5_000_000,  // 5 Mbps default estimate
          abrBandWidthFactor: 0.95,           // Use 95% of measured bandwidth
          abrBandWidthUpFactor: 0.7,          // Conservative when upgrading quality
          testBandwidth: true,

          // Timeout settings (เพิ่มสำหรับ Server จริง)
          manifestLoadingTimeOut: 20000,      // 20 วินาที
          fragLoadingTimeOut: 20000,          // 20 วินาที
          levelLoadingTimeOut: 20000,         // 20 วินาที

          // Buffer Hole Recovery (สำคัญมาก!)
          skipBufferHole: true,               // ข้ามช่องว่างอัตโนมัติ

          // Reliability & Retry
          manifestLoadingMaxRetry: 4,
          levelLoadingMaxRetry: 4,
          fragLoadingMaxRetry: 6,
          fragLoadingMaxRetryTimeout: 64000,

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

          // Add quality control button with selector
          art.controls.add({
            name: 'quality',
            position: 'right',
            index: 10,
            html: '<span class="art-quality-label">AUTO</span>',
            style: {
              fontSize: '12px',
              padding: '0 8px',
              fontWeight: 'bold',
            },
            selector: [
              { html: 'อัตโนมัติ', value: -1, default: true },
              ...sortedLevels.map((level) => ({
                html: `${level.height}p`,
                value: data.levels.findIndex(l => l.height === level.height),
              })),
            ],
            onSelect: function (item) {
              const selected = item as { html: string; value: number }
              console.log('[Quality] Selected:', selected.html, 'level:', selected.value)
              hls.currentLevel = selected.value
              isAutoMode = selected.value === -1
              // Update button label
              const labelEl = art.template.$player.querySelector('.art-quality-label')
              if (labelEl) {
                if (isAutoMode) {
                  const currentLevel = hls.levels[hls.currentLevel]
                  labelEl.textContent = currentLevel ? `AUTO ${currentLevel.height}p` : 'AUTO'
                } else {
                  labelEl.textContent = selected.html
                }
              }
              return selected.html
            },
          })

          console.log('[ArtPlayer] Quality control added:', data.levels.map(l => `${l.height}p`))

        })

        // Update label when HLS switches level (for auto mode)
        hls.on(Hls.Events.LEVEL_SWITCHED, (_event, data) => {
          if (isAutoMode) {
            const level = hls.levels[data.level]
            const labelEl = art.template.$player.querySelector('.art-quality-label')
            if (labelEl && level) {
              labelEl.textContent = `AUTO ${level.height}p`
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
      subtitleOffset: true,
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
      // Plugins - Multiple Subtitles
      plugins: subtitleConfig.length > 0 ? [
        artplayerPluginMultipleSubtitles({
          subtitles: subtitleConfig,
        }),
      ] : [],
      // Settings - ไม่ใส่ subtitle ใน settings แล้ว (ย้ายไป controls)
      settings: [],
      // Controls
      controls: [
        {
          name: 'skip-back',
          position: 'left',
          index: 1,
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
          index: 3,
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

        // Add subtitle control button (only if subtitles exist)
        if (subtitles.length > 0 && plugin) {
          const getSubtitleLabel = (lang: string | null) => {
            if (!lang) return 'ปิด'
            const sub = subtitles.find(s => s.language === lang)
            return sub?.name || lang
          }

          art.controls.add({
            name: 'subtitle',
            position: 'right',
            index: 20, // After quality selector
            html: `<span class="art-subtitle-label">${getSubtitleLabel(activeSubtitleLang)}</span>`,
            style: {
              fontSize: '12px',
              padding: '0 8px',
              fontWeight: 'bold',
            },
            selector: [
              { html: 'ปิด', value: '', default: !activeSubtitleLang },
              ...subtitles.map(sub => ({
                html: sub.name,
                value: sub.language,
                default: sub.language === activeSubtitleLang,
              })),
            ],
            onSelect: function (item) {
              const selected = item as { html: string; value: string }
              console.log('[Subtitle] Selected:', selected.html, 'lang:', selected.value)

              // Update label
              const labelEl = art.template.$player.querySelector('.art-subtitle-label')
              if (labelEl) {
                labelEl.textContent = selected.html
              }

              if (selected.value === '') {
                // Turn off subtitles
                plugin.tracks([])
                if (art.subtitle) {
                  art.subtitle.show = false
                }
                activeSubtitleLang = null
              } else {
                // Show selected subtitle
                plugin.tracks([selected.value])
                if (art.subtitle) {
                  art.subtitle.show = true
                }
                activeSubtitleLang = selected.value
              }

              return selected.html
            },
          })

          console.log('[ArtPlayer] Subtitle control added:', subtitles.map(s => s.name))
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
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [src, poster, streamToken, autoPlay, subtitleConfig, defaultSubtitle, onPlay, onPause, onEnded, onTimeUpdate])

  return (
    <div
      ref={containerRef}
      className="artplayer-container"
    />
  )
}

