package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache implements the Cache interface using Redis.
type RedisCache struct {
	client *redis.Client
	hits   atomic.Int64
	misses atomic.Int64
}

// RedisConfig holds Redis connection configuration.
type RedisConfig struct {
	Addr         string        `yaml:"addr" json:"addr"`
	Password     string        `yaml:"password" json:"password"`
	DB           int           `yaml:"db" json:"db"`
	PoolSize     int           `yaml:"pool_size" json:"pool_size"`
	MinIdleConns int           `yaml:"min_idle_conns" json:"min_idle_conns"`
	ReadTimeout  time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout"`
}

// NewRedisCache creates a new Redis-backed cache.
func NewRedisCache(cfg RedisConfig) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis: failed to connect to %s: %w", cfg.Addr, err)
	}

	return &RedisCache{client: client}, nil
}

func (c *RedisCache) Get(ctx context.Context, key string) (string, error) {
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		c.misses.Add(1)
		return "", ErrCacheMiss
	}
	if err != nil {
		return "", fmt.Errorf("redis get %s: %w", key, err)
	}
	c.hits.Add(1)
	return val, nil
}

func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	var val string
	switch v := value.(type) {
	case string:
		val = v
	case []byte:
		val = string(v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("redis set marshal %s: %w", key, err)
		}
		val = string(b)
	}
	return c.client.Set(ctx, key, val, expiration).Err()
}

func (c *RedisCache) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.client.Del(ctx, keys...).Err()
}

func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.client.Exists(ctx, key).Result()
	return n > 0, err
}

func (c *RedisCache) GetJSON(ctx context.Context, key string, dest interface{}) error {
	val, err := c.Get(ctx, key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(val), dest)
}

func (c *RedisCache) SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	b, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("redis setjson marshal %s: %w", key, err)
	}
	return c.client.Set(ctx, key, b, expiration).Err()
}

func (c *RedisCache) GetMulti(ctx context.Context, keys ...string) (map[string]string, error) {
	if len(keys) == 0 {
		return map[string]string{}, nil
	}
	vals, err := c.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(keys))
	for i, v := range vals {
		if v != nil {
			if s, ok := v.(string); ok {
				result[keys[i]] = s
			}
		}
	}
	return result, nil
}

func (c *RedisCache) Incr(ctx context.Context, key string) (int64, error) {
	return c.client.Incr(ctx, key).Result()
}

func (c *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	return c.client.TTL(ctx, key).Result()
}

func (c *RedisCache) Flush(ctx context.Context) error {
	return c.client.FlushDB(ctx).Err()
}

func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *RedisCache) Stats() CacheStats {
	hits := c.hits.Load()
	misses := c.misses.Load()
	total := hits + misses
	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}
	dbSize, _ := c.client.DBSize(context.Background()).Result()
	return CacheStats{
		Hits:    hits,
		Misses:  misses,
		HitRate: hitRate,
		Keys:    dbSize,
	}
}

// Client returns the underlying Redis client for advanced operations.
func (c *RedisCache) Client() *redis.Client {
	return c.client
}

// Close closes the Redis connection.
func (c *RedisCache) Close() error {
	return c.client.Close()
}
