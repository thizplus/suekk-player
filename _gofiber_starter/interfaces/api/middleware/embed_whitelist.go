package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"gofiber-template/application/serviceimpl"
	"gofiber-template/domain/models"
	"gofiber-template/domain/services"
	"gofiber-template/pkg/logger"
)

// EmbedWhitelistConfig กำหนดค่าสำหรับ middleware
type EmbedWhitelistConfig struct {
	// WhitelistService สำหรับตรวจสอบ domain
	WhitelistService services.WhitelistService

	// StreamCookieService สำหรับสร้าง signed cookie (optional)
	StreamCookieService *serviceimpl.StreamCookieService

	// AllowWithoutOrigin อนุญาตให้เข้าถึงถ้าไม่มี Origin/Referer (direct access)
	AllowWithoutOrigin bool

	// SkipPaths paths ที่ไม่ต้องตรวจสอบ whitelist
	SkipPaths []string
}

// ContextKey สำหรับเก็บข้อมูลใน context
const (
	ContextKeyWhitelistProfile = "whitelist_profile"
	ContextKeyEmbedDomain      = "embed_domain"
)

// EmbedWhitelist สร้าง middleware สำหรับตรวจสอบ domain whitelist
func EmbedWhitelist(config EmbedWhitelistConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.UserContext()

		// Skip paths ที่กำหนด
		path := c.Path()
		for _, skipPath := range config.SkipPaths {
			if strings.HasPrefix(path, skipPath) {
				return c.Next()
			}
		}

		// รับ domain จาก Origin หรือ Referer header
		origin := c.Get("Origin")
		referer := c.Get("Referer")

		domain := extractDomain(origin)
		if domain == "" {
			domain = extractDomain(referer)
		}

		// ถ้าไม่มี Origin/Referer
		if domain == "" {
			if config.AllowWithoutOrigin {
				logger.InfoContext(ctx, "Embed access without Origin/Referer (allowed)",
					"path", path,
					"ip", c.IP(),
				)
				return c.Next()
			}

			logger.WarnContext(ctx, "Embed access denied - no Origin/Referer",
				"path", path,
				"ip", c.IP(),
			)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"error": fiber.Map{
					"code":    "FORBIDDEN",
					"message": "Origin or Referer header required",
				},
			})
		}

		// เก็บ domain ใน context
		c.Locals(ContextKeyEmbedDomain, domain)

		// ตรวจสอบ whitelist
		allowed, profile, err := config.WhitelistService.IsDomainAllowed(ctx, domain)
		if err != nil || !allowed {
			logger.WarnContext(ctx, "Domain not in whitelist",
				"domain", domain,
				"path", path,
				"ip", c.IP(),
			)

			// Set restrictive headers
			c.Set("X-Frame-Options", "DENY")
			c.Set("Content-Security-Policy", "frame-ancestors 'none'")

			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"error": fiber.Map{
					"code":    "DOMAIN_NOT_ALLOWED",
					"message": "Domain not in whitelist",
				},
			})
		}

		// เก็บ profile ใน context
		c.Locals(ContextKeyWhitelistProfile, profile)

		// Set security headers based on profile
		setSecurityHeaders(c, profile, domain)

		// Set signed cookie for CDN access (if StreamCookieService is configured)
		if config.StreamCookieService != nil {
			setStreamCookie(c, config.StreamCookieService, domain)
		}

		logger.InfoContext(ctx, "Embed access allowed",
			"domain", domain,
			"profile_id", profile.ID,
			"profile_name", profile.Name,
			"path", path,
		)

		return c.Next()
	}
}

// setStreamCookie set signed cookie สำหรับ CDN access
// Cookie นี้จะถูกส่งไปกับ HLS requests เพื่อให้ Cloudflare WAF อนุญาต
func setStreamCookie(c *fiber.Ctx, cookieSvc *serviceimpl.StreamCookieService, domain string) {
	// สร้าง signed token
	token := cookieSvc.GenerateToken(domain)

	// ตั้งค่า cookie
	cookie := &fiber.Cookie{
		Name:     "suekk_stream",
		Value:    token,
		Domain:   cookieSvc.GetCookieDomain(), // wildcard domain: .suekk.com
		Path:     "/",
		MaxAge:   cookieSvc.GetCookieMaxAge(), // seconds
		Secure:   true,                        // HTTPS only
		HTTPOnly: true,                        // ป้องกัน XSS
		SameSite: "None",                      // จำเป็นสำหรับ cross-origin iframe
	}

	c.Cookie(cookie)

	// Log (debug mode only)
	logger.Debug("Stream cookie set",
		"domain", domain,
		"cookie_domain", cookieSvc.GetCookieDomain(),
		"max_age", cookieSvc.GetCookieMaxAge(),
	)
}

// setSecurityHeaders กำหนด security headers ตาม profile
func setSecurityHeaders(c *fiber.Ctx, profile *models.WhitelistProfile, domain string) {
	// ไม่ใช้ X-Frame-Options เพราะไม่รองรับ multiple domains
	// ใช้ CSP frame-ancestors แทน (modern browsers)
	// ลบ X-Frame-Options header ออกเพื่อป้องกัน conflict
	c.Response().Header.Del("X-Frame-Options")

	// Content-Security-Policy - frame-ancestors
	// อนุญาตให้ embed ใน domain ที่ whitelist
	csp := "frame-ancestors 'self' https://" + domain + " http://" + domain

	// เพิ่ม www variant
	if !strings.HasPrefix(domain, "www.") {
		csp += " https://www." + domain + " http://www." + domain
	}

	// เพิ่ม localhost สำหรับ development
	csp += " http://localhost:* https://localhost:*"

	c.Set("Content-Security-Policy", csp)

	// CORS headers
	c.Set("Access-Control-Allow-Origin", "https://"+domain)

	// Other security headers
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
}

// extractDomain แยก domain จาก URL
func extractDomain(urlStr string) string {
	if urlStr == "" {
		return ""
	}

	// Remove protocol
	domain := urlStr
	if strings.HasPrefix(domain, "https://") {
		domain = domain[8:]
	} else if strings.HasPrefix(domain, "http://") {
		domain = domain[7:]
	}

	// Remove path
	if idx := strings.Index(domain, "/"); idx != -1 {
		domain = domain[:idx]
	}

	// Remove port
	if idx := strings.Index(domain, ":"); idx != -1 {
		domain = domain[:idx]
	}

	return strings.ToLower(domain)
}

// GetWhitelistProfile ดึง profile จาก context
func GetWhitelistProfile(c *fiber.Ctx) *models.WhitelistProfile {
	profile, ok := c.Locals(ContextKeyWhitelistProfile).(*models.WhitelistProfile)
	if !ok {
		return nil
	}
	return profile
}

// GetEmbedDomain ดึง domain จาก context
func GetEmbedDomain(c *fiber.Ctx) string {
	domain, ok := c.Locals(ContextKeyEmbedDomain).(string)
	if !ok {
		return ""
	}
	return domain
}
