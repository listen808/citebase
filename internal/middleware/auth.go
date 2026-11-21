package middleware

import (
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware/jwt"
	userrepo "github.com/listen808/citebase/internal/repository/user"
	"github.com/listen808/citebase/internal/response"
	authsvc "github.com/listen808/citebase/internal/service/auth"
)

// userCacheItem 用户状态缓存条目
type userCacheItem struct {
	isForbidden int
	isDelete    int
	cachedAt    time.Time
}

const userCacheTTL = 5 * time.Minute

var userStatusCache sync.Map

// InvalidateUserCache 主动失效指定用户的用户状态缓存（P0-5）
// 在管理员禁用/删除用户、用户修改自身状态时调用，避免 5 分钟内继续命中旧状态
func InvalidateUserCache(userID int) {
	if userID <= 0 {
		return
	}
	userStatusCache.Delete(userID)
}

// AuthMiddleware JWT 认证中间件
// 支持 Authorization: Bearer <token> 和 ?token=<token>（用于 WebSocket 等无法设置 header 的场景）
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := extractToken(c)
		if tokenString == "" {
			response.Unauthorized(c, "missing authorization token")
			c.Abort()
			return
		}

		claims, err := jwt.ParseToken(tokenString)
		if err != nil {
			response.Unauthorized(c, "invalid or expired token")
			c.Abort()
			return
		}

		// 检查 token 是否已被注销（黑名单）
		if authsvc.IsTokenBlacklisted(tokenString) {
			response.Unauthorized(c, "token has been revoked")
			c.Abort()
			return
		}

		// 检查用户是否被禁用（即使 token 未过期，禁用用户不允许操作）
		// 使用内存缓存减少 DB 查询，缓存 TTL 5 分钟
		var isForbidden, isDelete int
		cacheHit := false
		if cached, ok := userStatusCache.Load(claims.UserID); ok {
			item := cached.(userCacheItem)
			if time.Since(item.cachedAt) < userCacheTTL {
				isForbidden = item.isForbidden
				isDelete = item.isDelete
				cacheHit = true
			}
		}
		if !cacheHit {
			user, err := userrepo.NewUserRepository().FindByID(claims.UserID)
			if err != nil {
				response.Unauthorized(c, "user not found")
				c.Abort()
				return
			}
			isForbidden = user.IsForbidden
			isDelete = user.IsDelete
			userStatusCache.Store(claims.UserID, userCacheItem{
				isForbidden: user.IsForbidden,
				isDelete:    user.IsDelete,
				cachedAt:    time.Now(),
			})
		}
		if isForbidden == 1 {
			response.Forbidden(c, "user is forbidden")
			c.Abort()
			return
		}
		if isDelete == 1 {
			response.Unauthorized(c, "user account deleted")
			c.Abort()
			return
		}

		// 注入 Context
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role_id", claims.RoleID)

		c.Next()
	}
}

// GetUserID 从 gin Context 安全获取用户 ID
func GetUserID(c *gin.Context) (int, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, jwt.ErrInvalidToken
	}
	uid, ok := userID.(int)
	if !ok {
		return 0, jwt.ErrInvalidToken
	}
	return uid, nil
}

// RequireUser checks whether the request is authenticated and returns the user ID.
// If not authenticated, it writes an Unauthorized response and returns false.
func RequireUser(c *gin.Context) (int, bool) {
	uid, err := GetUserID(c)
	if err != nil || uid == 0 {
		response.Unauthorized(c, "unauthorized")
		return 0, false
	}
	return uid, true
}

// IsSuperAdmin reports whether the current user is the super administrator (role_id == 1).
func IsSuperAdmin(c *gin.Context) bool {
	roleID, exists := c.Get("role_id")
	if !exists || roleID == nil {
		return false
	}
	rid, ok := roleID.(int)
	return ok && rid == 1
}

// OptionalAuthMiddleware 可选认证（有 token 解析并验证用户状态，无 token 放行）
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := extractToken(c)
		if tokenString == "" {
			c.Next()
			return
		}

		claims, err := jwt.ParseToken(tokenString)
		if err == nil {
			// 检查 token 黑名单
			if authsvc.IsTokenBlacklisted(tokenString) {
				c.Next()
				return
			}

			// 检查用户是否被禁用/删除（与 AuthMiddleware 保持一致）
			var isForbidden, isDelete int
			cacheHit := false
			if cached, ok := userStatusCache.Load(claims.UserID); ok {
				item := cached.(userCacheItem)
				if time.Since(item.cachedAt) < userCacheTTL {
					isForbidden = item.isForbidden
					isDelete = item.isDelete
					cacheHit = true
				}
			}
			if !cacheHit {
				user, err := userrepo.NewUserRepository().FindByID(claims.UserID)
				if err != nil {
					c.Next()
					return
				}
				isForbidden = user.IsForbidden
				isDelete = user.IsDelete
				userStatusCache.Store(claims.UserID, userCacheItem{
					isForbidden: user.IsForbidden,
					isDelete:    user.IsDelete,
					cachedAt:    time.Now(),
				})
			}
			if isForbidden == 1 || isDelete == 1 {
				c.Next()
				return
			}

			c.Set("user_id", claims.UserID)
			c.Set("username", claims.Username)
			c.Set("role_id", claims.RoleID)
		}

		c.Next()
	}
}

// extractToken 从 Authorization header、query 参数或 cookie 中提取 JWT
// 优先级：Authorization Bearer > ?token= query > auth_token cookie
func extractToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}
	// 支持 ?token=xxx 查询参数（用于 <img src="/images/...?token=xxx"> 等场景）
	if t := c.Query("token"); t != "" {
		return t
	}
	// 支持 cookie 认证（用于浏览器直接导航 /attachments/... 或 <a> 标签点击下载场景，
	// 此类请求不会携带 Authorization header）
	if cookie, err := c.Cookie("auth_token"); err == nil && cookie != "" {
		return cookie
	}
	return ""
}
