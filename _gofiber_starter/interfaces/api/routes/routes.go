package routes

import (
	"github.com/gofiber/fiber/v2"
	"gofiber-template/interfaces/api/handlers"
)

func SetupRoutes(app *fiber.App, h *handlers.Handlers) {
	// Setup health and root routes
	SetupHealthRoutes(app)

	// API version group
	api := app.Group("/api/v1")

	// Setup all route groups
	SetupAuthRoutes(api, h)
	SetupUserRoutes(api, h)
	SetupTaskRoutes(api, h)
	SetupFileRoutes(api, h)
	SetupJobRoutes(api, h)
	SetupVideoRoutes(api, h)
	SetupCategoryRoutes(api, h)
	SetupTranscodingRoutes(api, h)
	SetupStorageRoutes(api, h)
	SetupProgressRoutes(api, h)
	SetupWhitelistRoutes(api, h)      // Phase 6: Domain Whitelist & Ad Management
	SetupSettingRoutes(api, h)        // Admin Settings Management
	SetupSubtitleRoutes(api, h)       // Subtitle management
	SetupQueueRoutes(api, h)          // Queue management (transcode/subtitle/warmcache)
	SetupDirectUploadRoutes(api, h)   // Direct Upload via Presigned URL
	SetupReelRoutes(api, h)           // Reel Generator
	SetupGalleryAdminRoutes(api, h)   // Gallery Manual Selection (Admin)

	// Setup Monitoring routes (needs app for /api/v1/monitoring)
	SetupMonitoringRoutes(app, h)

	// Setup HLS routes (needs app, not api group)
	SetupHLSRoutes(app, h.HLSHandler)

	// Setup Embed routes (needs app for /embed/:code)
	SetupEmbedRoutes(app, h)

	// Setup WebSocket routes (needs app, not api group)
	SetupWebSocketRoutes(app)
}