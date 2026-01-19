package services

// StreamCookieService interface สำหรับจัดการ stream token (Hybrid Shield)
type StreamCookieService interface {
	// GenerateToken สร้าง signed token สำหรับ stream access
	// Token ประกอบด้วย domain และ expiry time
	GenerateToken(domain string) string

	// ValidateToken ตรวจสอบความถูกต้องของ token
	// Returns: domain string, valid bool
	ValidateToken(token string) (string, bool)

	// GetCookieDomain returns the cookie domain (e.g., .suekk.com)
	GetCookieDomain() string

	// GetCookieMaxAge returns the cookie max age in seconds
	GetCookieMaxAge() int
}
