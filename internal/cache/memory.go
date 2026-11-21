package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// entry represents a cached item with optional expiration.
type entry struct {
	value     string
	expiresAt time.Time // zero value means no expiry
}

func (e entry) expired() bool {
	if e.expiresAt.IsZero() {
		return false
	}
	return time.Now().After(e.expiresAt)
}

// MemoryCache implements the Cache interface using an in-process map.
// This is the fallback when Redis is unavailable.
type MemoryCache struct {
	mu     sync.RWMutex
	data   map[string]entry
	hits   atomic.Int64
	misses atomic.Int64
	stopCh chan struct{}
}

// NewMemoryCache creates a new in-memory cache with background cleanup.
func NewMemoryCache() *MemoryCache {
	c := &MemoryCache{
		data:   make(map[string]entry),
		stopCh: make(chan struct{}),
	}
	go c.cleanup()
	return c
}

// cleanup periodically removes expired entries.
func (c *MemoryCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			for k, v := range c.data {
				if v.expired() {
					delete(c.data, k)
				}
			}
			c.mu.Unlock()
		case <-c.stopCh:
			return
		}
	}
}

// Close stops the background cleanup goroutine.
func (c *MemoryCache) Close() {
	close(c.stopCh)
}

func (c *MemoryCache) Get(_ context.Context, key string) (string, error) {
	c.mu.RLock()
	e, ok := c.data[key]
	c.mu.RUnlock()
	if !ok || e.expired() {
		c.misses.Add(1)
		if ok && e.expired() {
			c.mu.Lock()
			delete(c.data, key)
			c.mu.Unlock()
		}
		return "", ErrCacheMiss
	}
	c.hits.Add(1)
	return e.value, nil
}

func (c *MemoryCache) Set(_ context.Context, key string, value interface{}, expiration time.Duration) error {
	var val string
	switch v := value.(type) {
	case string:
		val = v
	case []byte:
		val = string(v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		val = string(b)
	}
	var expiresAt time.Time
	if expiration > 0 {
		expiresAt = time.Now().Add(expiration)
	}
	c.mu.Lock()
	c.data[key] = entry{value: val, expiresAt: expiresAt}
	c.mu.Unlock()
	return nil
}

func (c *MemoryCache) Del(_ context.Context, keys ...string) error {
	c.mu.Lock()
	for _, key := range keys {
		delete(c.data, key)
	}
	c.mu.Unlock()
	return nil
}

func (c *MemoryCache) Exists(_ context.Context, key string) (bool, error) {
	c.mu.RLock()
	e, ok := c.data[key]
	c.mu.RUnlock()
	if !ok || e.expired() {
		return false, nil
	}
	return true, nil
}

func (c *MemoryCache) GetJSON(ctx context.Context, key string, dest interface{}) error {
	val, err := c.Get(ctx, key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(val), dest)
}

func (c *MemoryCache) SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.Set(ctx, key, string(b), expiration)
}

func (c *MemoryCache) GetMulti(ctx context.Context, keys ...string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		val, err := c.Get(ctx, key)
		if err == nil {
			result[key] = val
		}
	}
	return result, nil
}

func (c *MemoryCache) Incr(_ context.Context, key string) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.data[key]
	if !ok || e.expired() {
		c.data[key] = entry{value: "1"}
		return 1, nil
	}
	var n int64
	_, _ = fmt.Sscanf(e.value, "%d", &n)
	n++
	c.data[key] = entry{value: fmt.Sprintf("%d", n), expiresAt: e.expiresAt}
	return n, nil
}

func (c *MemoryCache) TTL(_ context.Context, key string) (time.Duration, error) {
	c.mu.RLock()
	e, ok := c.data[key]
	c.mu.RUnlock()
	if !ok {
		return -2, nil // key does not exist
	}
	if e.expiresAt.IsZero() {
		return -1, nil // no expiry
	}
	remaining := time.Until(e.expiresAt)
	if remaining < 0 {
		return -2, nil // expired
	}
	return remaining, nil
}

func (c *MemoryCache) Flush(_ context.Context) error {
	c.mu.Lock()
	c.data = make(map[string]entry)
	c.mu.Unlock()
	return nil
}

func (c *MemoryCache) Ping(_ context.Context) error {
	return nil
}

func (c *MemoryCache) Stats() CacheStats {
	c.mu.RLock()
	keys := int64(len(c.data))
	c.mu.RUnlock()
	hits := c.hits.Load()
	misses := c.misses.Load()
	total := hits + misses
	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}
	return CacheStats{
		Hits:    hits,
		Misses:  misses,
		HitRate: hitRate,
		Keys:    keys,
	}
}
