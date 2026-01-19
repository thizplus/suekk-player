package settings

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"

	"gofiber-template/domain/models"
	"gofiber-template/domain/repositories"
	"gofiber-template/pkg/logger"
)

// SettingsCache เก็บ settings ใน memory พร้อม fallback chain
// Fallback Order: ENV → Database → Hardcoded Default
type SettingsCache struct {
	mu         sync.RWMutex
	settings   map[string]map[string]string // category -> key -> value
	loadedAt   time.Time
	ttl        time.Duration
	repo       repositories.SettingRepository
}

var (
	globalCache *SettingsCache
	once        sync.Once
)

// InitCache สร้าง global cache instance
func InitCache(repo repositories.SettingRepository) *SettingsCache {
	once.Do(func() {
		globalCache = &SettingsCache{
			settings: make(map[string]map[string]string),
			ttl:      5 * time.Minute,
			repo:     repo,
		}
		// Load initial settings
		globalCache.Reload(context.Background())
	})
	return globalCache
}

// GetCache ดึง global cache instance
func GetCache() *SettingsCache {
	return globalCache
}

// Reload โหลด settings จาก database ใหม่
func (c *SettingsCache) Reload(ctx context.Context) error {
	if c.repo == nil {
		logger.WarnContext(ctx, "Settings repo not initialized, using defaults only")
		return nil
	}

	settings, err := c.repo.GetAll(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to load settings from DB", "error", err)
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Clear and rebuild
	c.settings = make(map[string]map[string]string)
	for _, s := range settings {
		if c.settings[s.Category] == nil {
			c.settings[s.Category] = make(map[string]string)
		}
		c.settings[s.Category][s.Key] = s.Value
	}
	c.loadedAt = time.Now()

	logger.InfoContext(ctx, "Settings cache reloaded", "count", len(settings))
	return nil
}

// Invalidate ล้าง cache ของ category
func (c *SettingsCache) Invalidate(category string) {
	c.mu.Lock()
	delete(c.settings, category)
	c.mu.Unlock()
}

// InvalidateAll ล้าง cache ทั้งหมด
func (c *SettingsCache) InvalidateAll() {
	c.mu.Lock()
	c.settings = make(map[string]map[string]string)
	c.mu.Unlock()
}

// Get ดึงค่า setting ด้วย fallback chain: ENV → DB → Default
func (c *SettingsCache) Get(category, key string) string {
	fullKey := category + "." + key

	// Level 1: Check ENV override
	if envKey, ok := EnvMapping[fullKey]; ok {
		if v := os.Getenv(envKey); v != "" {
			return v
		}
	}

	// Level 2: Check DB (via cache)
	c.mu.RLock()
	if cat, ok := c.settings[category]; ok {
		if v, ok := cat[key]; ok {
			c.mu.RUnlock()
			return v
		}
	}
	c.mu.RUnlock()

	// Level 3: Hardcoded default
	if cat, ok := DefaultSettings[category]; ok {
		if def, ok := cat[key]; ok {
			return def.Value
		}
	}

	return ""
}

// GetWithSource ดึงค่า setting พร้อมบอก source
func (c *SettingsCache) GetWithSource(category, key string) (string, models.SettingSource) {
	fullKey := category + "." + key

	// Level 1: Check ENV override
	if envKey, ok := EnvMapping[fullKey]; ok {
		if v := os.Getenv(envKey); v != "" {
			return v, models.SettingSourceEnv
		}
	}

	// Level 2: Check DB (via cache)
	c.mu.RLock()
	if cat, ok := c.settings[category]; ok {
		if v, ok := cat[key]; ok {
			c.mu.RUnlock()
			return v, models.SettingSourceDatabase
		}
	}
	c.mu.RUnlock()

	// Level 3: Hardcoded default
	if cat, ok := DefaultSettings[category]; ok {
		if def, ok := cat[key]; ok {
			return def.Value, models.SettingSourceDefault
		}
	}

	return "", models.SettingSourceDefault
}

// GetInt ดึงค่าเป็น int
func (c *SettingsCache) GetInt(category, key string, fallback int) int {
	v := c.Get(category, key)
	if v == "" {
		return fallback
	}
	if i, err := strconv.Atoi(v); err == nil {
		return i
	}
	return fallback
}

// GetBool ดึงค่าเป็น bool
func (c *SettingsCache) GetBool(category, key string, fallback bool) bool {
	v := c.Get(category, key)
	if v == "" {
		return fallback
	}
	return v == "true" || v == "1"
}

// GetFloat64 ดึงค่าเป็น float64
func (c *SettingsCache) GetFloat64(category, key string, fallback float64) float64 {
	v := c.Get(category, key)
	if v == "" {
		return fallback
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	return fallback
}

// IsEnvOverridden ตรวจสอบว่า setting ถูก override โดย ENV หรือไม่
func (c *SettingsCache) IsEnvOverridden(category, key string) bool {
	fullKey := category + "." + key
	if envKey, ok := EnvMapping[fullKey]; ok {
		return os.Getenv(envKey) != ""
	}
	return false
}

// GetEnvKey ดึงชื่อ ENV variable ของ setting
func (c *SettingsCache) GetEnvKey(category, key string) string {
	fullKey := category + "." + key
	return EnvMapping[fullKey]
}

// GetAllForCategory ดึงค่าทั้งหมดของ category
func (c *SettingsCache) GetAllForCategory(category string) map[string]string {
	result := make(map[string]string)

	// Start with defaults
	if cat, ok := DefaultSettings[category]; ok {
		for key, def := range cat {
			result[key] = def.Value
		}
	}

	// Override with DB values
	c.mu.RLock()
	if cat, ok := c.settings[category]; ok {
		for key, value := range cat {
			result[key] = value
		}
	}
	c.mu.RUnlock()

	// Override with ENV values
	for key := range result {
		fullKey := category + "." + key
		if envKey, ok := EnvMapping[fullKey]; ok {
			if v := os.Getenv(envKey); v != "" {
				result[key] = v
			}
		}
	}

	return result
}

// NeedsReload ตรวจสอบว่าต้อง reload หรือไม่
func (c *SettingsCache) NeedsReload() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.Since(c.loadedAt) > c.ttl
}

// Set อัพเดทค่าใน cache (ใช้หลังจาก save ลง DB)
func (c *SettingsCache) Set(category, key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.settings[category] == nil {
		c.settings[category] = make(map[string]string)
	}
	c.settings[category][key] = value
}
