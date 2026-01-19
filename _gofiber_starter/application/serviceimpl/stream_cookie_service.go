package serviceimpl

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gofiber-template/pkg/config"
)

// StreamCookieService จัดการ signed cookie สำหรับ stream access
type StreamCookieService struct {
	secretKey    string
	cookieDomain string
	cookieMaxAge int // seconds
}

// NewStreamCookieService สร้าง StreamCookieService instance
func NewStreamCookieService(cfg *config.StreamConfig) *StreamCookieService {
	maxAge := cfg.CookieMaxAge
	if maxAge <= 0 {
		maxAge = 7200 // 2 hours default
	}

	return &StreamCookieService{
		secretKey:    cfg.CookieKey,
		cookieDomain: cfg.CookieDomain,
		cookieMaxAge: maxAge,
	}
}

// StreamCookieData เก็บข้อมูลใน cookie
type StreamCookieData struct {
	Domain    string
	ExpiresAt int64
}

// GenerateToken สร้าง signed token สำหรับ cookie
// Format: base64(domain|expiry).signature
func (s *StreamCookieService) GenerateToken(domain string) string {
	expiresAt := time.Now().Add(time.Duration(s.cookieMaxAge) * time.Second).Unix()
	return s.GenerateTokenWithExpiry(domain, expiresAt)
}

// GenerateTokenWithExpiry สร้าง token พร้อมกำหนด expiry เอง
func (s *StreamCookieService) GenerateTokenWithExpiry(domain string, expiresAt int64) string {
	// Format: domain|expiry
	data := fmt.Sprintf("%s|%d", domain, expiresAt)

	// Sign with HMAC-SHA256
	signature := s.sign(data)

	// Return: base64(data).signature
	payload := base64.URLEncoding.EncodeToString([]byte(data))
	return fmt.Sprintf("%s.%s", payload, signature)
}

// ValidateToken ตรวจสอบความถูกต้องของ token
// Returns: domain string, valid bool
func (s *StreamCookieService) ValidateToken(token string) (string, bool) {
	if token == "" {
		return "", false
	}

	// Parse token: payload.signature
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", false
	}

	payloadB64, providedSig := parts[0], parts[1]

	// Decode payload
	payload, err := base64.URLEncoding.DecodeString(payloadB64)
	if err != nil {
		return "", false
	}

	// Verify signature
	expectedSig := s.sign(string(payload))
	if !hmac.Equal([]byte(providedSig), []byte(expectedSig)) {
		return "", false
	}

	// Parse data: domain|expiry
	dataParts := strings.Split(string(payload), "|")
	if len(dataParts) != 2 {
		return "", false
	}

	domain := dataParts[0]
	expiryStr := dataParts[1]

	// Check expiry
	expiry, err := strconv.ParseInt(expiryStr, 10, 64)
	if err != nil {
		return "", false
	}

	if time.Now().Unix() > expiry {
		return "", false // Token expired
	}

	return domain, true
}

// GetCookieDomain returns the cookie domain (e.g., .suekk.com)
func (s *StreamCookieService) GetCookieDomain() string {
	return s.cookieDomain
}

// GetCookieMaxAge returns the cookie max age in seconds
func (s *StreamCookieService) GetCookieMaxAge() int {
	return s.cookieMaxAge
}

// sign creates HMAC-SHA256 signature
func (s *StreamCookieService) sign(data string) string {
	h := hmac.New(sha256.New, []byte(s.secretKey))
	h.Write([]byte(data))
	return base64.URLEncoding.EncodeToString(h.Sum(nil))
}
