package routes

import (
	"github.com/gofiber/fiber/v2"
	"gofiber-template/interfaces/api/handlers"
	"gofiber-template/interfaces/api/middleware"
)

// SetupWhitelistRoutes กำหนด routes สำหรับ Whitelist Profile & Ad Stats Management
func SetupWhitelistRoutes(api fiber.Router, h *handlers.Handlers) {
	// ==================== Public Routes (ต้องอยู่ก่อน protected) ====================
	// Ad Impression Recording (called by embed player)
	api.Post("/ads/impression", h.WhitelistHandler.RecordAdImpression)

	// Embed Config (called by embed player to get watermark/preroll settings)
	api.Get("/embed/config", h.WhitelistHandler.GetEmbedConfig)

	// ==================== Protected Routes (Admin Only) ====================
	whitelist := api.Group("/whitelist", middleware.Protected())

	// Profile Management
	profiles := whitelist.Group("/profiles")
	profiles.Post("/", h.WhitelistHandler.CreateProfile)
	profiles.Get("/", h.WhitelistHandler.ListProfiles)
	profiles.Get("/:id", h.WhitelistHandler.GetProfile)
	profiles.Put("/:id", h.WhitelistHandler.UpdateProfile)
	profiles.Delete("/:id", h.WhitelistHandler.DeleteProfile)

	// Domain Management
	profiles.Post("/:id/domains", h.WhitelistHandler.AddDomain)
	whitelist.Delete("/domains/:id", h.WhitelistHandler.RemoveDomain)

	// Preroll Ads Management
	profiles.Post("/:id/prerolls", h.WhitelistHandler.AddPrerollAd)
	profiles.Get("/:id/prerolls", h.WhitelistHandler.GetPrerollAdsByProfile)
	profiles.Put("/:id/prerolls/reorder", h.WhitelistHandler.ReorderPrerollAds)
	whitelist.Put("/prerolls/:id", h.WhitelistHandler.UpdatePrerollAd)
	whitelist.Delete("/prerolls/:id", h.WhitelistHandler.DeletePrerollAd)

	// Cache Management
	cache := whitelist.Group("/cache")
	cache.Post("/clear", h.WhitelistHandler.ClearAllCache)
	cache.Delete("/domain/:domain", h.WhitelistHandler.ClearDomainCache)

	// ==================== Ad Statistics (Protected) ====================
	ads := api.Group("/ads", middleware.Protected())
	ads.Get("/stats", h.WhitelistHandler.GetAdStats)
	ads.Get("/stats/profile/:id", h.WhitelistHandler.GetAdStatsByProfile)
	ads.Get("/stats/devices", h.WhitelistHandler.GetDeviceStats)
	ads.Get("/stats/ranking", h.WhitelistHandler.GetProfileRanking)
	ads.Get("/stats/skip-distribution", h.WhitelistHandler.GetSkipTimeDistribution)
}
