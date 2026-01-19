import { useState, useRef, useEffect, useCallback } from 'react'
import { Loader2 } from 'lucide-react'
import type { PrerollConfig } from '@/features/whitelist'

interface PrerollPlayerProps {
  config?: PrerollConfig
  configs?: PrerollConfig[]
  thumbnailUrl?: string // Thumbnail ของวิดีโอหลัก (แสดงก่อนกด play)
  onComplete: () => void
  onSkip: (skipTime: number, adIndex?: number) => void
  onError: () => void
  onAdClick?: (clickUrl: string, adIndex: number) => void
}

const LOAD_TIMEOUT_MS = 10000 // 10 seconds timeout for loading

/**
 * Pre-roll Ad Player
 * แสดงโฆษณาก่อนเล่นวิดีโอหลัก พร้อมปุ่ม Skip
 * รองรับการเล่นหลายโฆษณาต่อกัน
 * รองรับทั้ง Video และ Image ads
 */
export function PrerollPlayer({ config, configs, thumbnailUrl, onComplete, onSkip, onError, onAdClick }: PrerollPlayerProps) {
  // Normalize to array: use configs if provided, otherwise wrap single config
  const prerollConfigs = configs?.length ? configs : (config ? [config] : [])
  const totalAds = prerollConfigs.length

  const videoRef = useRef<HTMLVideoElement>(null)
  const loadTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const imageTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const [hasStarted, setHasStarted] = useState(false) // ผู้ใช้กด play หรือยัง
  const [currentAdIndex, setCurrentAdIndex] = useState(0)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(0)
  const [canSkip, setCanSkip] = useState(false)
  const [isPlaying, setIsPlaying] = useState(false)
  const [isLoading, setIsLoading] = useState(false) // เริ่มเป็น false รอกด play ก่อน
  const [hasError, setHasError] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')

  // Current preroll config
  const currentConfig = prerollConfigs[currentAdIndex]

  // Check if current ad is image type
  const isImageAd = currentConfig?.type === 'image'

  // คำนวณเวลาที่เหลือก่อน skip ได้
  const skipAfter = currentConfig?.skipAfter || 0
  const timeUntilSkip = Math.max(0, skipAfter - currentTime)
  const showSkipButton = skipAfter > 0 // 0 = บังคับดูจบ

  // Clear load timeout and image timer
  const clearTimers = useCallback(() => {
    if (loadTimeoutRef.current) {
      clearTimeout(loadTimeoutRef.current)
      loadTimeoutRef.current = null
    }
    if (imageTimerRef.current) {
      clearInterval(imageTimerRef.current)
      imageTimerRef.current = null
    }
  }, [])

  // Reset state when moving to next ad
  const resetAdState = useCallback(() => {
    setCurrentTime(0)
    setDuration(0)
    setCanSkip(false)
    setIsLoading(true)
    setHasError(false)
    setErrorMessage('')
    clearTimers()
  }, [clearTimers])

  // Move to next ad or complete
  const moveToNextAd = useCallback(() => {
    if (currentAdIndex < totalAds - 1) {
      resetAdState()
      setCurrentAdIndex(prev => prev + 1)
    } else {
      onComplete()
    }
  }, [currentAdIndex, totalAds, onComplete, resetAdState])

  // Update skip availability
  useEffect(() => {
    if (skipAfter > 0 && currentTime >= skipAfter) {
      setCanSkip(true)
    }
  }, [currentTime, skipAfter])

  // Handle time update
  const handleTimeUpdate = useCallback(() => {
    if (videoRef.current) {
      setCurrentTime(videoRef.current.currentTime)
    }
  }, [])

  // Handle loaded metadata - video is ready to play
  const handleLoadedMetadata = useCallback(() => {
    if (videoRef.current) {
      const videoDuration = videoRef.current.duration
      // Validate: duration must be > 0 and not infinity
      if (videoDuration > 0 && isFinite(videoDuration)) {
        setDuration(videoDuration)
        setIsLoading(false)
        clearTimers()
        console.log(`[Ad ${currentAdIndex + 1}] Loaded, duration: ${videoDuration}s`)
      }
    }
  }, [currentAdIndex, clearTimers])

  // Handle can play - video has enough data to start
  const handleCanPlay = useCallback(() => {
    setIsLoading(false)
    clearTimers()
  }, [clearTimers])

  // Handle video ended
  const handleEnded = useCallback(() => {
    console.log(`[Ad ${currentAdIndex + 1}] Ended normally`)
    moveToNextAd()
  }, [currentAdIndex, moveToNextAd])

  // Handle video error
  const handleError = useCallback(() => {
    const video = videoRef.current
    const errorCode = video?.error?.code
    const errorMsg = video?.error?.message || 'Unknown error'

    console.error(`[Ad ${currentAdIndex + 1}] Error:`, errorCode, errorMsg)

    clearTimers()
    setIsLoading(false)
    setHasError(true)

    // Map error codes to messages
    let displayMsg = 'ไม่สามารถโหลดโฆษณาได้'
    if (errorCode === 2) displayMsg = 'Network error - ตรวจสอบการเชื่อมต่อ'
    else if (errorCode === 3) displayMsg = 'Video decode error - format ไม่รองรับ'
    else if (errorCode === 4) displayMsg = 'Video not found หรือ CORS blocked'

    setErrorMessage(displayMsg)

    // Auto-skip after 3 seconds on error
    setTimeout(() => {
      if (currentAdIndex < totalAds - 1) {
        resetAdState()
        setCurrentAdIndex(prev => prev + 1)
      } else {
        onError()
      }
    }, 3000)
  }, [currentAdIndex, totalAds, onError, resetAdState, clearTimers])

  // Handle play state
  const handlePlay = useCallback(() => {
    setIsPlaying(true)
    setIsLoading(false)
  }, [])

  const handlePause = useCallback(() => {
    setIsPlaying(false)
  }, [])

  // เริ่มเล่นโฆษณา (ผู้ใช้กด play)
  const startAds = useCallback(() => {
    setHasStarted(true)
    setIsLoading(true)
  }, [])

  // เล่น ad เมื่อ hasStarted เป็น true หรือเมื่อเปลี่ยน ad (หลังจากเริ่มแล้ว)
  useEffect(() => {
    if (!currentConfig?.url || !hasStarted) return

    // Reset states
    setIsLoading(true)
    setHasError(false)
    setErrorMessage('')
    clearTimers()

    // Set load timeout
    loadTimeoutRef.current = setTimeout(() => {
      console.warn(`[Ad ${currentAdIndex + 1}] Load timeout after ${LOAD_TIMEOUT_MS}ms`)
      setIsLoading(false)
      setHasError(true)
      setErrorMessage('โหลดโฆษณานานเกินไป')

      // Auto-skip after timeout
      setTimeout(() => {
        if (currentAdIndex < totalAds - 1) {
          resetAdState()
          setCurrentAdIndex(prev => prev + 1)
        } else {
          onError()
        }
      }, 2000)
    }, LOAD_TIMEOUT_MS)

    // Handle IMAGE ad
    if (isImageAd) {
      const imageDuration = currentConfig.duration || 10 // default 10 seconds
      setDuration(imageDuration)
      setIsPlaying(true)

      // Preload image
      const img = new Image()
      img.onload = () => {
        setIsLoading(false)
        clearTimers()
        console.log(`[Ad ${currentAdIndex + 1}] Image loaded, duration: ${imageDuration}s`)

        // Start timer for image ad
        let elapsed = 0
        imageTimerRef.current = setInterval(() => {
          elapsed += 1
          setCurrentTime(elapsed)

          if (elapsed >= imageDuration) {
            clearTimers()
            console.log(`[Ad ${currentAdIndex + 1}] Image ad ended`)
            moveToNextAd()
          }
        }, 1000)
      }
      img.onerror = () => {
        console.error(`[Ad ${currentAdIndex + 1}] Image failed to load`)
        clearTimers()
        setIsLoading(false)
        setHasError(true)
        setErrorMessage('ไม่สามารถโหลดรูปภาพโฆษณาได้')

        setTimeout(() => {
          if (currentAdIndex < totalAds - 1) {
            resetAdState()
            setCurrentAdIndex(prev => prev + 1)
          } else {
            onError()
          }
        }, 3000)
      }
      img.src = currentConfig.url
    } else {
      // Handle VIDEO ad
      const video = videoRef.current
      if (!video) return

      video.currentTime = 0
      video.muted = false // เล่นมีเสียง
      video.load()
      video.play().catch((err) => {
        console.warn('[Ad] Play failed:', err)
      })
    }

    return () => {
      clearTimers()
    }
  }, [currentAdIndex, currentConfig, isImageAd, hasStarted, clearTimers, resetAdState, totalAds, onError, moveToNextAd])

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      clearTimers()
    }
  }, [clearTimers])

  // Handle skip click
  const handleSkipClick = () => {
    if (canSkip || hasError) {
      onSkip(currentTime, currentAdIndex)
      moveToNextAd()
    }
  }

  // Handle video/image click - open link if available
  const handleMediaClick = () => {
    if (currentConfig?.clickUrl) {
      onAdClick?.(currentConfig.clickUrl, currentAdIndex)
      window.open(currentConfig.clickUrl, '_blank', 'noopener,noreferrer')
    }
  }

  // Toggle play/pause (video only, when no clickUrl)
  const handleVideoClick = () => {
    // ถ้ามี clickUrl ให้เปิดลิงก์แทน
    if (currentConfig?.clickUrl) {
      handleMediaClick()
      return
    }
    // ถ้าไม่มี clickUrl ให้ toggle play/pause
    const video = videoRef.current
    if (video && !hasError) {
      if (video.paused) {
        video.play()
      } else {
        video.pause()
      }
    }
  }

  // If no prerolls, complete immediately
  if (!currentConfig) {
    useEffect(() => {
      onComplete()
    }, [onComplete])
    return null
  }

  return (
    <div className="preroll-container">
      {/* Initial Play Overlay - รอผู้ใช้กด play */}
      {!hasStarted && (
        <div
          className="preroll-start-overlay"
          onClick={startAds}
          style={thumbnailUrl ? { backgroundImage: `url(${thumbnailUrl})` } : undefined}
        >
          <div className="preroll-start-state">
            <svg xmlns="http://www.w3.org/2000/svg" width="80" height="80" viewBox="0 0 24 24">
              <path fill="currentColor" d="M9.5 9.325v5.35q0 .575.525.875t1.025-.05l4.15-2.65q.475-.3.475-.85t-.475-.85L11.05 8.5q-.5-.35-1.025-.05t-.525.875M12 22q-2.075 0-3.9-.788t-3.175-2.137q-1.35-1.35-2.137-3.175T2 12t.788-3.9 2.137-3.175q1.35-1.35 3.175-2.137T12 2t3.9.788 3.175 2.137q1.35 1.35 2.138 3.175T22 12q0 2.075-.788 3.9t-2.137 3.175q-1.35 1.35-3.175 2.138T12 22" />
            </svg>
          </div>
        </div>
      )}

      {/* Image Ad */}
      {isImageAd ? (
        <div
          className="preroll-image-container"
          onClick={handleMediaClick}
          style={{ cursor: currentConfig.clickUrl ? 'pointer' : 'default' }}
        >
          <img
            key={`ad-img-${currentAdIndex}-${currentConfig.url}`}
            src={currentConfig.url}
            alt="Advertisement"
            className="preroll-image"
          />
        </div>
      ) : (
        /* Video Ad */
        <video
          ref={videoRef}
          key={`ad-${currentAdIndex}-${currentConfig.url}`}
          src={currentConfig.url}
          className="preroll-video"
          playsInline
          onTimeUpdate={handleTimeUpdate}
          onLoadedMetadata={handleLoadedMetadata}
          onCanPlay={handleCanPlay}
          onEnded={handleEnded}
          onError={handleError}
          onPlay={handlePlay}
          onPause={handlePause}
          onClick={handleVideoClick}
          style={{ cursor: currentConfig.clickUrl ? 'pointer' : 'default' }}
        />
      )}

      {/* Loading Overlay - แสดงเมื่อเริ่มแล้ว */}
      {hasStarted && isLoading && !hasError && (
        <div className="preroll-loading">
          <Loader2 className="h-8 w-8 animate-spin text-white" />
          <p className="text-white text-sm mt-2">กำลังโหลดโฆษณา...</p>
        </div>
      )}

      {/* Error Overlay - แสดงเมื่อเริ่มแล้ว */}
      {hasStarted && hasError && (
        <div className="preroll-error">
          <p className="text-white text-sm">{errorMessage}</p>
          <p className="text-gray-400 text-xs mt-1">ข้ามไปอัตโนมัติ...</p>
        </div>
      )}

      {/* Ad Label with counter and title - แสดงเมื่อเริ่มแล้ว */}
      {hasStarted && (
        <div className="preroll-label">
          <span>โฆษณา {totalAds > 1 ? `(${currentAdIndex + 1}/${totalAds})` : ''}</span>
          {currentConfig.title && (
            <span className="preroll-title"> • {currentConfig.title}</span>
          )}
        </div>
      )}

      {/* Progress Bar - แสดงเมื่อเริ่มแล้ว */}
      {hasStarted && !isLoading && !hasError && duration > 0 && (
        <div className="preroll-progress">
          <div
            className="preroll-progress-bar"
            style={{ width: `${(currentTime / duration) * 100}%` }}
          />
        </div>
      )}

      {/* Multi-ad Progress Dots - แสดงเมื่อเริ่มแล้ว */}
      {hasStarted && totalAds > 1 && (
        <div className="preroll-dots">
          {prerollConfigs.map((_, index) => (
            <div
              key={index}
              className={`preroll-dot ${
                index < currentAdIndex
                  ? 'preroll-dot-complete'
                  : index === currentAdIndex
                  ? 'preroll-dot-active'
                  : 'preroll-dot-pending'
              }`}
            />
          ))}
        </div>
      )}

      {/* Bottom Controls Area - แสดงเมื่อเริ่มแล้ว */}
      {hasStarted && (
        <div className="preroll-bottom-controls">
          {/* Spacer to push skip button to the right */}
          <div />

          {/* Skip Button Area */}
          <div className="preroll-skip-area">
            {hasError ? (
              <button
                className="preroll-skip-button preroll-skip-ready"
                onClick={handleSkipClick}
              >
                ข้ามเลย
              </button>
            ) : showSkipButton ? (
              canSkip ? (
                <button
                  className="preroll-skip-button preroll-skip-ready"
                  onClick={handleSkipClick}
                >
                  ข้ามโฆษณา
                </button>
              ) : (
                <div
                  className={`preroll-skip-countdown ${currentConfig.clickUrl ? 'preroll-skip-clickable' : ''}`}
                  onClick={currentConfig.clickUrl ? handleMediaClick : undefined}
                >
                  {currentConfig.clickText && (
                    <span className="preroll-skip-clicktext">{currentConfig.clickText} • </span>
                  )}
                  ข้ามได้ใน {Math.ceil(timeUntilSkip)} วินาที
                </div>
              )
            ) : (
              <div
                className={`preroll-skip-forced ${currentConfig.clickUrl ? 'preroll-skip-clickable' : ''}`}
                onClick={currentConfig.clickUrl ? handleMediaClick : undefined}
              >
                {currentConfig.clickText && (
                  <span className="preroll-skip-clicktext">{currentConfig.clickText} • </span>
                )}
                กรุณาชมโฆษณาจนจบ
              </div>
            )}
          </div>
        </div>
      )}

      {/* Play/Pause Indicator (Video only, when no clickUrl, after started) */}
      {hasStarted && !isImageAd && !isPlaying && !isLoading && !hasError && !currentConfig.clickUrl && (
        <div className="preroll-play-indicator" onClick={handleVideoClick}>
          <svg viewBox="0 0 24 24" className="preroll-play-icon">
            <path fill="currentColor" d="M8 5v14l11-7z" />
          </svg>
        </div>
      )}
    </div>
  )
}
