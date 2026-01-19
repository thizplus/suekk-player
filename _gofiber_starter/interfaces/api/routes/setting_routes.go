package routes

import (
	"github.com/gofiber/fiber/v2"
	"gofiber-template/interfaces/api/handlers"
	"gofiber-template/interfaces/api/middleware"
)

// SetupSettingRoutes กำหนด routes สำหรับ Admin Settings
// ต้อง Protected (login แล้ว) เท่านั้น
func SetupSettingRoutes(api fiber.Router, h *handlers.Handlers) {
	// All settings routes require authentication
	settings := api.Group("/settings", middleware.Protected())

	// Get all settings (grouped by category)
	// GET /api/v1/settings
	settings.Get("/", h.SettingHandler.GetAll)

	// Get categories list
	// GET /api/v1/settings/categories
	settings.Get("/categories", h.SettingHandler.GetCategories)

	// Get audit logs
	// GET /api/v1/settings/audit-logs
	settings.Get("/audit-logs", h.SettingHandler.GetAuditLogs)

	// Reload cache
	// POST /api/v1/settings/reload-cache
	settings.Post("/reload-cache", h.SettingHandler.ReloadCache)

	// Get settings by category
	// GET /api/v1/settings/:category
	settings.Get("/:category", h.SettingHandler.GetByCategory)

	// Update settings by category
	// PUT /api/v1/settings/:category
	settings.Put("/:category", h.SettingHandler.Update)

	// Reset settings to defaults
	// POST /api/v1/settings/:category/reset
	settings.Post("/:category/reset", h.SettingHandler.ResetToDefaults)
}
