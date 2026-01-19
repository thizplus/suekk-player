package routes

import (
	"github.com/gofiber/fiber/v2"
	"gofiber-template/interfaces/api/handlers"
	"gofiber-template/interfaces/api/middleware"
)

func SetupEmbedRoutes(app *fiber.App, h *handlers.Handlers) {
	// Embed player route (public)
	// GET /embed/:code - Serve embed player HTML
	// Note: /embed/:code ไม่ต้องตรวจสอบ whitelist เพราะเป็น HTML page
	// การตรวจสอบจะทำใน frontend เมื่อ player โหลด config
	app.Get("/embed/:code", h.EmbedHandler.ServeEmbed)

	// Embed API routes
	embed := app.Group("/api/v1/embed")

	// GET /api/v1/embed/:code/info - Get video info for embed
	// ใช้ whitelist middleware ถ้ามี WhitelistService
	if h.WhitelistHandler != nil {
		embedWhitelistMw := middleware.EmbedWhitelist(middleware.EmbedWhitelistConfig{
			WhitelistService:    h.WhitelistHandler.GetWhitelistService(),
			StreamCookieService: h.StreamCookieService, // Set signed cookie for CDN access
			AllowWithoutOrigin:  false,                 // บังคับให้มี Origin/Referer
		})
		embed.Get("/:code/info", embedWhitelistMw, h.EmbedHandler.GetEmbedInfo)
	} else {
		// Fallback without whitelist check
		embed.Get("/:code/info", h.EmbedHandler.GetEmbedInfo)
	}

	// GET /api/v1/embed/:code/code - Get embed code snippets (admin only - no whitelist needed)
	embed.Get("/:code/code", h.EmbedHandler.GetEmbedCode)
}
