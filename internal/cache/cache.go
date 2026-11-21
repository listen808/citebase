package cache

import (
	"context"
	"fmt"
	"time"
)

// Cache defines the caching interface used throughout the application.
// Implementations include Redis (production) and in-memory (fallback).
type Cache interface {
	// Get retrieves a string value by key. Returns ("", ErrCacheMiss) if not found.
	Get(ctx context.Context, key string) (string, error)

	// Set stores a value with the given expiration. Zero duration means no expiry.
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error

	// Del removes one or more keys.
	Del(ctx context.Context, keys ...string) error

	// Exists checks whether a key exists.
	Exists(ctx context.Context, key string) (bool, error)

	// GetJSON retrieves a value and unmarshals it into dest.
	GetJSON(ctx context.Context, key string, dest interface{}) error

	// SetJSON marshals the value to JSON and stores it.
	SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error

	// GetMulti retrieves multiple keys at once. Missing keys are omitted from the result map.
	GetMulti(ctx context.Context, keys ...string) (map[string]string, error)

	// Incr atomically increments a key by 1. Creates the key with value 1 if it doesn't exist.
	Incr(ctx context.Context, key string) (int64, error)

	// TTL returns the remaining time-to-live for a key.
	TTL(ctx context.Context, key string) (time.Duration, error)

	// Flush removes all keys managed by this cache instance.
	Flush(ctx context.Context) error

	// Ping checks if the cache backend is reachable.
	Ping(ctx context.Context) error

	// Stats returns cache usage statistics.
	Stats() CacheStats
}

// CacheStats holds cache usage metrics.
type CacheStats struct {
	Hits    int64   `json:"hits"`
	Misses  int64   `json:"misses"`
	HitRate float64 `json:"hit_rate"`
	Keys    int64   `json:"keys"`
}

// ErrCacheMiss is returned when a key is not found in the cache.
var ErrCacheMiss = fmt.Errorf("cache: key not found")
