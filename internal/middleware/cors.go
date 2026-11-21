package middleware

import (
	"net/http"

	logx "github.com/listen808/citebase/internal/log"
	"github.com/listen808/citebase/internal/utils/config"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware 跨域中间件
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// 从配置文件读取允许的 CORS 来源
		allowedOrigins := config.AppConfig.Server.AllowedOrigins
		if allowedOrigins == nil || len(allowedOrigins) == 0 {
			// 默认允许 localhost
			allowedOrigins = []string{
				"http://localhost:3000",
				"http://localhost:3001",
				"http://127.0.0.1:3000",
				"http://127.0.0.1:3001",
			}
		}

		// 安全检查：禁止 * 与 AllowCredentials 同时使用
		hasWildcard := false
		for _, allowed := range allowedOrigins {
			if allowed == "*" {
				hasWildcard = true
				break
			}
		}
		if hasWildcard {
			logx.Warn("CORS AllowOrigins contains '*', removing wildcard for security. Use explicit origins instead.")
			// 移除通配符，只允许明确配置的域名
			filtered := make([]string, 0, len(allowedOrigins))
			for _, allowed := range allowedOrigins {
				if allowed != "*" {
					filtered = append(filtered, allowed)
				}
			}
			allowedOrigins = filtered
		}

		// 检查 Origin 是否在白名单中
		// 注意：同源请求（Origin 为空）不需要 CORS 校验，应当放行
		if origin != "" {
			isAllowed := false
			for _, allowed := range allowedOrigins {
				if allowed == origin {
					isAllowed = true
					break
				}
			}

			if isAllowed {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			} else {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}

		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Accept, Origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
