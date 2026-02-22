package routes

import (
	"github.com/gofiber/fiber/v2"
	"gofiber-template/interfaces/api/handlers"
)

func SetupHLSRoutes(app *fiber.App, h *handlers.HLSHandler) {
	// HLS Access API - returns JWT token + CDN URL
	// Client จะใช้ URL ที่ได้ไปเรียก Cloudflare CDN
	hls := app.Group("/api/v1/hls")

	// GET /api/v1/hls/:code/access - รับ JWT token และ playlist URL
	// Response: { playlist_url, token, expires_at }
	hls.Get("/:code/access", h.GetAccess)

	// GET /api/v1/hls/:code/gallery - รับ presigned URLs สำหรับ gallery images ทั้งหมด
	// Response: { urls: string[], count, expires_at }
	hls.Get("/:code/gallery", h.GetGalleryUrls)

	// GET /api/v1/hls/verify - Verify token (debug endpoint)
	hls.Get("/verify", h.VerifyToken)

	// HLS Streaming - Cloudflare CDN จะ proxy มาที่นี่
	// URL: cdn.suekk.com/hls/{code}/* → api:8080/hls/{code}/*
	// GET /hls/:code/master.m3u8, /hls/:code/480p/*.ts
	app.Get("/hls/:code/*", h.ServeHLS)

	// Subtitle Streaming - Cloudflare CDN จะ proxy มาที่นี่
	// URL: cdn.suekk.com/subtitles/{code}/* → api:8080/subtitles/{code}/*
	// GET /subtitles/:code/th.srt, /subtitles/:code/ja.srt
	app.Get("/subtitles/:code/*", h.ServeSubtitle)

	// Reel Streaming - CDN จะ proxy มาที่นี่
	// URL: cdn.suekk.com/stream/reels/{reelId}/* → api:8080/stream/reels/{reelId}/*
	// GET /stream/reels/:reelId/output.mp4, /stream/reels/:reelId/thumb.jpg
	// Storage: reels/{reelId}/output.mp4
	app.Get("/stream/reels/:code/*", h.ServeReel) // :code is actually reel_id

	// Gallery Streaming - CDN จะ proxy มาที่นี่
	// URL: cdn.suekk.com/gallery/{code}/* → api:8080/gallery/{code}/*
	// GET /gallery/:code/001.jpg, /gallery/:code/002.jpg
	// Storage: gallery/{code}/001.jpg
	app.Get("/gallery/:code/*", h.ServeGallery)
}
