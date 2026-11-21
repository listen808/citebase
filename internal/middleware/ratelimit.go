package middleware

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/response"
)

// RateLimiter 基于内存的请求频率限制器
type RateLimiter struct {
	requests map[string][]time.Time
	mu       sync.RWMutex
	limit    int
	window   time.Duration
}

// NewRateLimiter 创建频率限制器
// limit: 时间窗口内允许的最大请求数
// window: 时间窗口大小
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow 检查指定 key 是否允许请求
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// 清理过期的请求记录
	timestamps := rl.requests[key]
	valid := timestamps[:0]
	for _, t := range timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	// 检查是否超过限制
	if len(valid) >= rl.limit {
		rl.requests[key] = valid
		return false
	}

	// 记录本次请求
	valid = append(valid, now)
	rl.requests[key] = valid
	return true
}

// RetryAfter 返回距离下一个名额释放还有多久；0 表示当前没被限流。
// 给 429 的 Retry-After 头用 —— 没有它，客户端只能瞎猜要等多久，
// 要么疯狂重试（正是限流要挡的），要么干脆报错把一次可恢复的等待变成整页崩溃。
func (rl *RateLimiter) RetryAfter(key string) time.Duration {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	valid := rl.requests[key]
	if len(valid) < rl.limit {
		return 0
	}
	var oldest time.Time
	for _, t := range valid {
		if oldest.IsZero() || t.Before(oldest) {
			oldest = t
		}
	}
	if d := oldest.Add(rl.window).Sub(time.Now()); d > 0 {
		return d
	}
	return 0
}

// Cleanup 清理过期的请求记录（用于定时清理）
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.window)
	for key, timestamps := range rl.requests {
		valid := timestamps[:0]
		for _, t := range timestamps {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(rl.requests, key)
		} else {
			rl.requests[key] = valid
		}
	}
}

// Global rate limiter instances
var (
	// apiLimiter: 通用 API 限制 300 请求/分钟/IP
	apiLimiter = NewRateLimiter(300, time.Minute)
	// strictLimiter: 口令猜测入口严格限制 10 请求/分钟/IP（登录、安装向导、超管入口）
	strictLimiter = NewRateLimiter(10, time.Minute)
	// refreshLimiter: 令牌刷新单独一档 30 请求/分钟/IP。
	// 它原先和 login 共用 10/分钟，但 SPA 每次令牌过期都要 refresh —— 多开几个标签页、
	// 或一轮 73 页的巡检就会撞上，撞上的表现是整页被 ErrorBoundary 接住。
	// 口令猜测防护靠的是 login 那一档，放宽 refresh 不降低这层防护。
	refreshLimiter = NewRateLimiter(30, time.Minute)
)

func init() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			apiLimiter.Cleanup()
			strictLimiter.Cleanup()
			refreshLimiter.Cleanup()
			cleanupAPIKeyLimiters()
		}
	}()
}

// RateLimitMiddleware 通用 API 频率限制中间件：300 请求/分钟/IP。
// 健康检查与静态资源不计入，避免探针与图片加载被误伤。
func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/api/v1/health" || path == "/health" ||
			strings.HasPrefix(path, "/images/") || strings.HasPrefix(path, "/attachments/") ||
			strings.HasPrefix(path, "/assets/") || strings.HasPrefix(path, "/static/") {
			c.Next()
			return
		}
		if ip := c.ClientIP(); ip != "" && !apiLimiter.Allow(ip) {
			setRetryAfter(c, apiLimiter, ip)
			response.TooManyRequestsKey(c, response.MsgTooManyRequests)
			c.Abort()
			return
		}
		c.Next()
	}
}

// StrictRateLimitMiddleware 认证入口频率限制：10 请求/分钟/IP。
// 登录/刷新/找回密码等接口必须挂此中间件，否则可被无限次口令猜测。
func StrictRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 键含目标路径，避免不同接口互相挤占额度
		ip := c.ClientIP()
		if ip == "" {
			c.Next()
			return
		}
		key := ip + "|" + c.FullPath()
		if !strictLimiter.Allow(key) {
			setRetryAfter(c, strictLimiter, key)
			response.TooManyRequestsKey(c, response.MsgActionTooFrequent)
			c.Abort()
			return
		}
		c.Next()
	}
}

// setRetryAfter 把"还要等多久"写进响应头。取整到秒且至少 1 秒，
// 否则 0.4 秒会被浏览器当成 0，触发立刻重试。
func setRetryAfter(c *gin.Context, rl *RateLimiter, key string) {
	wait := int(rl.RetryAfter(key).Seconds())
	if wait < 1 {
		wait = 1
	}
	c.Header("Retry-After", strconv.Itoa(wait))
}

// RefreshRateLimitMiddleware 令牌刷新频率限制：30 请求/分钟/IP。
// 与 StrictRateLimitMiddleware 分开，因为 refresh 不是口令猜测面 —— 它是 SPA 的
// 常规动作（令牌一过期就发一次），多标签页/巡检连开几十页时 10/分钟必然撞。
func RefreshRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if ip == "" {
			c.Next()
			return
		}
		key := ip + "|" + c.FullPath()
		if !refreshLimiter.Allow(key) {
			setRetryAfter(c, refreshLimiter, key)
			response.TooManyRequestsKey(c, response.MsgRefreshTooFrequent)
			c.Abort()
			return
		}
		c.Next()
	}
}

// RateLimitByKeyMiddleware 基于自定义 key 的频率限制（如按用户 ID）
func RateLimitByKeyMiddleware(keyFunc func(*gin.Context) string, limit int, window time.Duration) gin.HandlerFunc {
	limiter := NewRateLimiter(limit, window)
	return func(c *gin.Context) {
		key := keyFunc(c)
		if key == "" {
			c.Next()
			return
		}
		if !limiter.Allow(key) {
			response.TooManyRequestsKey(c, response.MsgTooManyRequests)
			c.Abort()
			return
		}
		c.Next()
	}
}
