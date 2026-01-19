package redis

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"

	"gofiber-template/pkg/config"
	"gofiber-template/pkg/logger"
)

// Client wraps the Redis client
type Client struct {
	rdb *redis.Client
}

// NewClient creates a new Redis client from config
func NewClient(cfg *config.RedisConfig) (*Client, error) {
	opt, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, err
	}

	if cfg.Password != "" {
		opt.Password = cfg.Password
	}
	if cfg.DB > 0 {
		opt.DB = cfg.DB
	}

	rdb := redis.NewClient(opt)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	logger.Info("Redis connected", "url", cfg.URL)

	return &Client{rdb: rdb}, nil
}

// Get retrieves a value by key
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, key).Result()
}

// Set stores a value with optional expiration
func (c *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return c.rdb.Set(ctx, key, value, expiration).Err()
}

// Del deletes one or more keys
func (c *Client) Del(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

// Exists checks if a key exists
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.rdb.Exists(ctx, key).Result()
	return n > 0, err
}

// Scan iterates over keys matching a pattern
func (c *Client) Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd {
	return c.rdb.Scan(ctx, cursor, match, count)
}

// ScanAndDelete deletes all keys matching a pattern
func (c *Client) ScanAndDelete(ctx context.Context, pattern string) (int64, error) {
	var deleted int64
	var cursor uint64 = 0

	for {
		keys, nextCursor, err := c.rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return deleted, err
		}

		if len(keys) > 0 {
			n, err := c.rdb.Del(ctx, keys...).Result()
			if err != nil {
				return deleted, err
			}
			deleted += n
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return deleted, nil
}

// Close closes the Redis connection
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Ping tests the connection
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// ═══════════════════════════════════════════════════════════════════════════════
// Distributed Locking (Singleflight Pattern)
// ═══════════════════════════════════════════════════════════════════════════════

// SetNX sets a value only if the key does not exist (for locking)
// Returns true if the lock was acquired, false if already locked
func (c *Client) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, key, value, expiration).Result()
}

// AcquireLock tries to acquire a lock with the given key
// Returns true if lock acquired, false if already locked by someone else
func (c *Client) AcquireLock(ctx context.Context, lockKey string, ttl time.Duration) (bool, error) {
	return c.SetNX(ctx, lockKey, "1", ttl)
}

// ReleaseLock releases a lock
func (c *Client) ReleaseLock(ctx context.Context, lockKey string) error {
	return c.Del(ctx, lockKey)
}

// ═══════════════════════════════════════════════════════════════════════════════
// JSON Cache Helpers
// ═══════════════════════════════════════════════════════════════════════════════

// SetJSON stores a value as JSON with expiration
func (c *Client) SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, data, expiration).Err()
}

// GetJSON retrieves a JSON value and unmarshals it into the target
// Returns redis.Nil error if key does not exist
func (c *Client) GetJSON(ctx context.Context, key string, target interface{}) error {
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// GetOrSet gets a value from cache, or calls the getter function and caches the result
// This implements the Singleflight pattern with locking
func (c *Client) GetOrSet(ctx context.Context, key string, target interface{}, ttl time.Duration, getter func() (interface{}, error)) error {
	// 1. Try to get from cache
	err := c.GetJSON(ctx, key, target)
	if err == nil {
		return nil // Cache hit
	}
	if err != redis.Nil {
		return err // Real error
	}

	// 2. Cache miss - try to acquire lock
	lockKey := "lock:" + key
	locked, err := c.AcquireLock(ctx, lockKey, 10*time.Second)
	if err != nil {
		return err
	}

	if !locked {
		// Someone else is fetching, wait and retry
		time.Sleep(100 * time.Millisecond)
		return c.GetOrSet(ctx, key, target, ttl, getter)
	}
	defer c.ReleaseLock(ctx, lockKey)

	// 3. Double-check cache (another request might have populated it)
	err = c.GetJSON(ctx, key, target)
	if err == nil {
		return nil
	}

	// 4. Fetch from source
	result, err := getter()
	if err != nil {
		return err
	}

	// 5. Cache the result
	if err := c.SetJSON(ctx, key, result, ttl); err != nil {
		logger.Warn("Failed to cache result", "key", key, "error", err)
	}

	// 6. Copy result to target
	data, _ := json.Marshal(result)
	return json.Unmarshal(data, target)
}
