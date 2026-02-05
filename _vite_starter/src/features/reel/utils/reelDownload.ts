import { apiClient } from '@/lib/api-client'
import { toast } from 'sonner'

interface StreamAccessResponse {
  token: string
  cdn_base_url: string
}

/**
 * Get stream access token for a video
 */
async function getStreamToken(videoCode: string): Promise<string | null> {
  try {
    const response = await apiClient.get<StreamAccessResponse>(`/api/v1/hls/${videoCode}/access`)
    return response.token
  } catch (error) {
    console.error('Failed to get stream token:', error)
    return null
  }
}

/**
 * ดาวน์โหลด reel ผ่าน authenticated CDN
 * @param videoCode - video code สำหรับดึง stream token
 * @param outputUrl - URL ของ reel จาก API (reel.outputUrl)
 * @param title - ชื่อไฟล์สำหรับดาวน์โหลด
 */
export async function downloadReel(videoCode: string, outputUrl: string, title?: string): Promise<boolean> {
  try {
    if (!outputUrl) {
      toast.error('ไม่พบไฟล์ reel')
      return false
    }

    // 1. Get stream token
    const token = await getStreamToken(videoCode)
    if (!token) {
      toast.error('ไม่สามารถดาวน์โหลดได้')
      return false
    }

    // 2. Fetch reel with token (add cache buster to bypass CDN/browser cache)
    const cacheBuster = Date.now()
    const url = `${outputUrl}?t=${cacheBuster}`
    const response = await fetch(url, {
      headers: {
        'X-Stream-Token': token,
      },
      cache: 'no-store',
    })

    if (!response.ok) {
      throw new Error('Failed to fetch reel')
    }

    // 3. Create blob and trigger download
    const blob = await response.blob()
    const blobUrl = URL.createObjectURL(blob)

    const a = document.createElement('a')
    a.href = blobUrl
    a.download = `${title || 'reel'}.mp4`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)

    setTimeout(() => URL.revokeObjectURL(blobUrl), 1000)

    toast.success('ดาวน์โหลดสำเร็จ')
    return true
  } catch (error) {
    console.error('Download error:', error)
    toast.error('ดาวน์โหลดไม่สำเร็จ')
    return false
  }
}

/**
 * ดึง reel เป็น blob URL สำหรับ preview
 * @param videoCode - video code สำหรับดึง stream token
 * @param outputUrl - URL ของ reel จาก API (reel.outputUrl)
 */
export async function getReelBlobUrl(videoCode: string, outputUrl: string): Promise<string | null> {
  try {
    if (!outputUrl) {
      toast.error('ไม่พบไฟล์ reel')
      return null
    }

    // 1. Get stream token
    const token = await getStreamToken(videoCode)
    if (!token) {
      toast.error('ไม่สามารถโหลดได้')
      return null
    }

    // 2. Fetch reel with token (add cache buster to bypass CDN/browser cache)
    const cacheBuster = Date.now()
    const url = `${outputUrl}?t=${cacheBuster}`
    const response = await fetch(url, {
      headers: {
        'X-Stream-Token': token,
      },
      cache: 'no-store',
    })

    if (!response.ok) {
      throw new Error('Failed to fetch reel')
    }

    // 3. Create blob URL
    const blob = await response.blob()
    return URL.createObjectURL(blob)
  } catch (error) {
    console.error('Fetch error:', error)
    toast.error('โหลดไม่สำเร็จ')
    return null
  }
}
