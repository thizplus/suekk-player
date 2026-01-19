package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ProfileDomain เก็บ domain ที่ผูกกับ WhitelistProfile
// 1 Profile สามารถมีหลาย domains
type ProfileDomain struct {
	ID        uuid.UUID `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	ProfileID uuid.UUID `gorm:"type:uuid;not null;index"`
	Domain    string    `gorm:"size:255;not null"` // เช่น "game1.com", "*.game1.com"
	CreatedAt time.Time

	// Relations
	Profile *WhitelistProfile `gorm:"foreignKey:ProfileID"`
}

func (ProfileDomain) TableName() string {
	return "profile_domains"
}

// MatchesDomain ตรวจสอบว่า domain ตรงกับ pattern หรือไม่
// รองรับ:
// - Exact match: "game1.com"
// - Wildcard: "*.game1.com" (matches sub.game1.com, www.game1.com, และ game1.com)
func (d *ProfileDomain) MatchesDomain(domain string) bool {
	return MatchDomain(d.Domain, domain)
}

// MatchDomain ตรวจสอบว่า domain ตรงกับ pattern หรือไม่
func MatchDomain(pattern, domain string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	domain = strings.ToLower(strings.TrimSpace(domain))

	if pattern == "" || domain == "" {
		return false
	}

	// Wildcard match: "*.game1.com"
	if strings.HasPrefix(pattern, "*.") {
		baseDomain := pattern[2:] // "game1.com" (ตัด *. ออก)
		suffix := pattern[1:]     // ".game1.com"

		// Match: sub.game1.com, www.game1.com
		if strings.HasSuffix(domain, suffix) {
			return true
		}
		// Match ตัว base domain ด้วย (game1.com เฉยๆ)
		if domain == baseDomain {
			return true
		}
		return false
	}

	// Exact match or with/without www
	return domain == pattern ||
		domain == "www."+pattern ||
		"www."+domain == pattern
}

// NormalizeDomain ทำให้ domain อยู่ในรูปแบบมาตรฐาน
func NormalizeDomain(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	// ลบ protocol ถ้ามี
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https://")
	// ลบ trailing slash
	domain = strings.TrimSuffix(domain, "/")
	// ลบ path ถ้ามี
	if idx := strings.Index(domain, "/"); idx != -1 {
		domain = domain[:idx]
	}
	return domain
}

// ExtractDomainFromURL ดึง domain จาก URL (Referer หรือ Origin)
func ExtractDomainFromURL(url string) string {
	if url == "" {
		return ""
	}

	// ลบ protocol
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")

	// ลบ path และ query string
	if idx := strings.Index(url, "/"); idx != -1 {
		url = url[:idx]
	}
	if idx := strings.Index(url, "?"); idx != -1 {
		url = url[:idx]
	}

	// ลบ port ถ้ามี
	if idx := strings.Index(url, ":"); idx != -1 {
		url = url[:idx]
	}

	return strings.ToLower(strings.TrimSpace(url))
}
