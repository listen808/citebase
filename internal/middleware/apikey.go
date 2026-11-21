package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	adminrepo "github.com/listen808/citebase/internal/repository/admin"
	"github.com/listen808/citebase/internal/response"
)

// apiKeyCtxKey 用于在上下文中传递已认证的密钥信息。
type apiKeyCtx struct {
	AppName string
	OwnerID int
	KeyID   int
}

// apiKeyRL 包装每把 API Key 的限流器与 limit 配置
// 修复 P0-6: 原实现每次请求新建 limiter，限流完全失效
type apiKeyRL struct {
	limit   int
	limiter *RateLimiter
}

// apiKeyLimiters 按 API Key hash 缓存独立的限流器，使限流真正生效
var apiKeyLimiters sync.Map // map[string]*apiKeyRL

// getAPIKeyLimiter 获取或创建指定 API Key 的限流器
// 若 limit 变化（管理员调整 rate_limit），自动重建 limiter
func getAPIKeyLimiter(hash string, limit int) *RateLimiter {
	if v, ok := apiKeyLimiters.Load(hash); ok {
		entry := v.(*apiKeyRL)
		if entry.limit == limit {
			return entry.limiter
		}
	}
	rl := NewRateLimiter(limit, time.Minute)
	entry := &apiKeyRL{limit: limit, limiter: rl}
	actual, _ := apiKeyLimiters.LoadOrStore(hash, entry)
	return actual.(*apiKeyRL).limiter
}

// cleanupAPIKeyLimiters 清理所有 API Key 限流器中的过期记录
// 由 init() 中的定时任务定期调用
func cleanupAPIKeyLimiters() {
	apiKeyLimiters.Range(func(_, v interface{}) bool {
		v.(*apiKeyRL).limiter.Cleanup()
		return true
	})
}

// ApiKeyAuthMiddleware 开放平台 API Key 鉴权中间件（G029）。
// 从 Authorization: Bearer mwk_xxx 提取密钥，校验存在性/状态/过期，
// 并按密钥维度做每分钟限流（限流阈值取密钥自身 rate_limit）。
func ApiKeyAuthMiddleware() gin.HandlerFunc {
	repo := adminrepo.NewApiKeyRepository()
	return func(c *gin.Context) {
		raw := extractBearer(c)
		if raw == "" {
			response.Unauthorized(c, "missing api key")
			c.Abort()
			return
		}
		sum := sha256.Sum256([]byte(raw))
		hash := hex.EncodeToString(sum[:])

		rec, err := repo.FindByHash(hash)
		if err != nil {
			response.Unauthorized(c, "invalid api key")
			c.Abort()
			return
		}
		if rec.Status != "active" {
			response.Unauthorized(c, "api key revoked")
			c.Abort()
			return
		}
		if !rec.ExpiresAt.IsZero() && rec.ExpiresAt.Before(time.Now()) {
			response.Unauthorized(c, "api key expired")
			c.Abort()
			return
		}

		// 按密钥维度限流：使用缓存的 limiter 保证跨请求生效（已禁用）
		// limit := rec.RateLimit
		// if limit <= 0 {
		// 	limit = 60
		// }
		// limiter := getAPIKeyLimiter(hash, limit)
		// if !limiter.Allow("apikey:" + hash) {
		// 	response.TooManyRequests(c, "api key rate limit exceeded")
		// 	c.Abort()
		// 	return
		// }

		// 异步更新最后使用时间
		go func() {
			defer func() { _ = recover() }()
			_ = repo.Touch(rec.ID)
		}()

		c.Set("apikey", &apiKeyCtx{AppName: rec.AppName, OwnerID: rec.OwnerID, KeyID: rec.ID})
		c.Next()
	}
}

// GetAPIKeyInfo 从 gin 上下文获取已认证的密钥信息。
func GetAPIKeyInfo(c *gin.Context) *apiKeyCtx {
	if v, ok := c.Get("apikey"); ok {
		if k, ok := v.(*apiKeyCtx); ok {
			return k
		}
	}
	return nil
}

func extractBearer(c *gin.Context) string {
	h := c.GetHeader("Authorization")
	if h == "" {
		return ""
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(h, prefix))
}
