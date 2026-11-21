package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTestRedisCache starts a miniredis server and returns a RedisCache
// connected to it along with a cleanup function.
func newTestRedisCache(t *testing.T) (*RedisCache, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run: %v", err)
	}
	rc, err := NewRedisCache(RedisConfig{
		Addr:         mr.Addr(),
		PoolSize:     2,
		MinIdleConns: 1,
	})
	if err != nil {
		mr.Close()
		t.Fatalf("NewRedisCache: %v", err)
	}
	return rc, mr
}

// ---------------------------------------------------------------------------
// Table-driven tests that exercise both backends via the Cache interface.
// ---------------------------------------------------------------------------

func cacheBackends(t *testing.T) map[string]Cache {
	t.Helper()

	rc, _ := newTestRedisCache(t)
	mc := NewMemoryCache()

	return map[string]Cache{
		"Redis":  rc,
		"Memory": mc,
	}
}

func TestSetAndGet(t *testing.T) {
	for name, c := range cacheBackends(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if err := c.Set(ctx, "k1", "hello", 5*time.Minute); err != nil {
				t.Fatalf("Set: %v", err)
			}
			val, err := c.Get(ctx, "k1")
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if val != "hello" {
				t.Fatalf("expected %q, got %q", "hello", val)
			}
		})
	}
}

func TestGetMiss(t *testing.T) {
	for name, c := range cacheBackends(t) {
		t.Run(name, func(t *testing.T) {
			_, err := c.Get(context.Background(), "nonexistent")
			if err != ErrCacheMiss {
				t.Fatalf("expected ErrCacheMiss, got %v", err)
			}
		})
	}
}

func TestDel(t *testing.T) {
	for name, c := range cacheBackends(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			_ = c.Set(ctx, "del1", "a", 0)
			_ = c.Set(ctx, "del2", "b", 0)
			if err := c.Del(ctx, "del1", "del2"); err != nil {
				t.Fatalf("Del: %v", err)
			}
			if _, err := c.Get(ctx, "del1"); err != ErrCacheMiss {
				t.Fatalf("expected ErrCacheMiss after Del, got %v", err)
			}
		})
	}
}

func TestExists(t *testing.T) {
	for name, c := range cacheBackends(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			_ = c.Set(ctx, "ex1", "val", 5*time.Minute)
			ok, err := c.Exists(ctx, "ex1")
			if err != nil {
				t.Fatalf("Exists: %v", err)
			}
			if !ok {
				t.Fatal("expected key to exist")
			}
			ok, _ = c.Exists(ctx, "ex_missing")
			if ok {
				t.Fatal("expected key to not exist")
			}
		})
	}
}

func TestSetJSONAndGetJSON(t *testing.T) {
	type payload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	for name, c := range cacheBackends(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			in := payload{Name: "test", Count: 42}
			if err := c.SetJSON(ctx, "json1", in, 5*time.Minute); err != nil {
				t.Fatalf("SetJSON: %v", err)
			}
			var out payload
			if err := c.GetJSON(ctx, "json1", &out); err != nil {
				t.Fatalf("GetJSON: %v", err)
			}
			if out.Name != in.Name || out.Count != in.Count {
				t.Fatalf("expected %+v, got %+v", in, out)
			}
		})
	}
}

func TestGetMulti(t *testing.T) {
	for name, c := range cacheBackends(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			_ = c.Set(ctx, "m1", "a", 5*time.Minute)
			_ = c.Set(ctx, "m2", "b", 5*time.Minute)

			result, err := c.GetMulti(ctx, "m1", "m2", "m3")
			if err != nil {
				t.Fatalf("GetMulti: %v", err)
			}
			if len(result) != 2 {
				t.Fatalf("expected 2 results, got %d", len(result))
			}
			if result["m1"] != "a" || result["m2"] != "b" {
				t.Fatalf("unexpected values: %v", result)
			}
		})
	}
}

func TestIncr(t *testing.T) {
	for name, c := range cacheBackends(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			v1, err := c.Incr(ctx, "counter")
			if err != nil {
				t.Fatalf("Incr: %v", err)
			}
			if v1 != 1 {
				t.Fatalf("expected 1, got %d", v1)
			}
			v2, err := c.Incr(ctx, "counter")
			if err != nil {
				t.Fatalf("Incr: %v", err)
			}
			if v2 != 2 {
				t.Fatalf("expected 2, got %d", v2)
			}
		})
	}
}

func TestTTL(t *testing.T) {
	for name, c := range cacheBackends(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			_ = c.Set(ctx, "ttl1", "val", 10*time.Minute)
			d, err := c.TTL(ctx, "ttl1")
			if err != nil {
				t.Fatalf("TTL: %v", err)
			}
			if d < 9*time.Minute || d > 10*time.Minute {
				t.Fatalf("expected ~10m TTL, got %v", d)
			}

			// Non-existent key.
			d2, err := c.TTL(ctx, "ttl_missing")
			if err != nil {
				t.Fatalf("TTL missing: %v", err)
			}
			if d2 != -2 {
				t.Fatalf("expected -2 for missing key, got %v", d2)
			}
		})
	}
}

func TestFlush(t *testing.T) {
	for name, c := range cacheBackends(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			_ = c.Set(ctx, "f1", "a", 0)
			_ = c.Set(ctx, "f2", "b", 0)
			if err := c.Flush(ctx); err != nil {
				t.Fatalf("Flush: %v", err)
			}
			if _, err := c.Get(ctx, "f1"); err != ErrCacheMiss {
				t.Fatalf("expected ErrCacheMiss after Flush, got %v", err)
			}
		})
	}
}

func TestPing(t *testing.T) {
	for name, c := range cacheBackends(t) {
		t.Run(name, func(t *testing.T) {
			if err := c.Ping(context.Background()); err != nil {
				t.Fatalf("Ping: %v", err)
			}
		})
	}
}

func TestStats(t *testing.T) {
	for name, c := range cacheBackends(t) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			_ = c.Set(ctx, "s1", "v", 5*time.Minute)

			// 1 hit
			_, _ = c.Get(ctx, "s1")
			// 1 miss
			_, _ = c.Get(ctx, "s_missing")

			stats := c.Stats()
			if stats.Hits != 1 {
				t.Fatalf("expected 1 hit, got %d", stats.Hits)
			}
			if stats.Misses != 1 {
				t.Fatalf("expected 1 miss, got %d", stats.Misses)
			}
			if stats.HitRate != 0.5 {
				t.Fatalf("expected hit rate 0.5, got %f", stats.HitRate)
			}
		})
	}
}

func TestTTLExpiration(t *testing.T) {
	// Use a short TTL and verify the memory cache expires entries.
	mc := NewMemoryCache()
	defer mc.Close()
	ctx := context.Background()

	_ = mc.Set(ctx, "expire_me", "val", 50*time.Millisecond)
	val, err := mc.Get(ctx, "expire_me")
	if err != nil || val != "val" {
		t.Fatalf("expected value before expiry, got %q err=%v", val, err)
	}

	time.Sleep(100 * time.Millisecond)

	_, err = mc.Get(ctx, "expire_me")
	if err != ErrCacheMiss {
		t.Fatalf("expected ErrCacheMiss after expiry, got %v", err)
	}
}

// TestKeyHelpers validates the key generation helper functions.
func TestKeyHelpers(t *testing.T) {
	tests := []struct {
		got      string
		expected string
	}{
		{UserKey(42), "citebase:user:42"},
		{UserByNameKey("admin"), "citebase:user:name:admin"},
		{DocumentKey(100), "citebase:doc:100"},
		{DocContentKey(100), "citebase:doc:content:100"},
		{SpaceKey(7), "citebase:space:7"},
		{CategoryTreeKey(3), "citebase:cat:tree:3"},
		{SearchKey("abc123"), "citebase:search:abc123"},
		{SessionKey("tok"), "citebase:session:tok"},
		{RateLimitKey("1.2.3.4", "login"), "citebase:ratelimit:1.2.3.4:login"},
	}
	for _, tt := range tests {
		if tt.got != tt.expected {
			t.Errorf("key helper: got %q, want %q", tt.got, tt.expected)
		}
	}
}
