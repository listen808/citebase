package auth

import (
	"github.com/gin-gonic/gin"
	userapi "github.com/listen808/citebase/internal/api/user"
	"github.com/listen808/citebase/internal/middleware"
)

// RegisterRoutes 注册认证相关路由（公开路由 + 需要认证的路由）
// authGroup 用于包裹需要 AuthMiddleware + AutoRBAC 的路由
func RegisterRoutes(v1 *gin.RouterGroup) {
	authAPI := NewAuthAPI()
	authSSOAPI := NewAuthSSOAPI()
	oidcAPI := NewOIDCAPI()
	installAPI := NewInstallAPI()
	authorAPI := NewAuthorAPI()

	userAPI := userapi.NewUserAPI()
	roleAPI := userapi.NewRoleAPI()
	contactAPI := userapi.NewContactAPI()

	strictRateLimit := middleware.StrictRateLimitMiddleware()
	// 令牌刷新单独一档：它不是口令猜测面，而是 SPA 的常规动作。
	refreshRateLimit := middleware.RefreshRateLimitMiddleware()

	// 安装向导（公开，但必须限流：init-system 可以创建超级管理员）
	v1.GET("/install/state", strictRateLimit, installAPI.GetInstallState)
	v1.POST("/install/init-db", strictRateLimit, installAPI.InitDatabase)
	v1.POST("/install/init-system", strictRateLimit, installAPI.InitSystem)

	// 超级管理员入口（公开，严格频率限制）
	v1.GET("/author/config", authorAPI.GetAuthorConfig)
	v1.POST("/author/login", strictRateLimit, authorAPI.Login)
	v1.POST("/author/sso-login", strictRateLimit, authorAPI.AuthorSSOLogin)

	// 认证相关（公开，严格频率限制）
	v1.POST("/auth/login", strictRateLimit, authAPI.Login)
	v1.POST("/auth/refresh", refreshRateLimit, authAPI.RefreshToken)
	v1.POST("/auth/logout", strictRateLimit, authAPI.Logout)
	v1.GET("/auth/sso/:id/login", authSSOAPI.SSOLogin)
	// 曾另挂一份 POST 指向同一 handler：SSOLogin 只读 c.Param("id")、与请求方法无关，
	// 前端（api/modules/auth/sso.ts）与 RBAC 规则都只用 GET，POST 无任何调用方。
	v1.GET("/auth/sso/callback", strictRateLimit, authSSOAPI.SSOCallback)
	v1.GET("/auth-sso/used", authSSOAPI.GetUsedAuthSSO)

	// OIDC 认证（公开）
	v1.GET("/auth/oidc/providers", oidcAPI.GetProviders)
	v1.GET("/auth/oidc/:id/login", strictRateLimit, oidcAPI.Login)
	v1.GET("/auth/oidc/callback", strictRateLimit, oidcAPI.Callback)

	// 需要认证 + RBAC
	// P1 修复：AccessLog 已提升为全局中间件（server.go setupGin），此处移除避免重复记录
	authGroup := v1.Group("")
	authGroup.Use(middleware.AuthMiddleware(), middleware.AutoRBAC())
	{
		authGroup.GET("/auth/me", authAPI.GetCurrentUser)
		authGroup.PUT("/auth/me", authAPI.UpdateCurrentUser)
		authGroup.PUT("/auth/me/password", authAPI.ChangeOwnPassword)
		// 个性化偏好跟账号走（外观 × 材质 × 透明变体）。只读写 ctx 里的 uid，不接受 id 参数，
		// 所以它是 self-service 面 —— rbac_coverage_test.go 的 sensitivePublic 里必须同步登记一条。
		authGroup.PUT("/auth/me/preferences", authAPI.UpdateMyPreferences)
		authGroup.GET("/auth-sso", authSSOAPI.GetAuthSSOs)
		authGroup.GET("/auth-sso/:id", authSSOAPI.GetAuthSSOByID)
		authGroup.POST("/auth-sso", authSSOAPI.CreateAuthSSO)
		authGroup.PUT("/auth-sso/:id", authSSOAPI.UpdateAuthSSO)
		authGroup.DELETE("/auth-sso/:id", authSSOAPI.DeleteAuthSSO)
		authGroup.PUT("/auth-sso/:id/default", authSSOAPI.SetDefaultSSO)

		// OIDC 提供方管理
		authGroup.GET("/auth/oidc/admin/providers", oidcAPI.AdminGetProviders)
		authGroup.GET("/auth/oidc/admin/providers/:id", oidcAPI.AdminGetProvider)
		authGroup.POST("/auth/oidc/admin/providers", oidcAPI.AdminCreateProvider)
		authGroup.PUT("/auth/oidc/admin/providers/:id", oidcAPI.AdminUpdateProvider)
		authGroup.DELETE("/auth/oidc/admin/providers/:id", oidcAPI.AdminDeleteProvider)

		// 角色管理
		authGroup.GET("/roles", roleAPI.GetRoles)
		authGroup.POST("/roles", roleAPI.CreateRole)
		authGroup.PUT("/roles/:id", roleAPI.UpdateRole)
		authGroup.DELETE("/roles/:id", roleAPI.DeleteRole)
		authGroup.GET("/roles/:id/privileges", roleAPI.GetRolePrivileges)
		authGroup.GET("/privileges", roleAPI.GetPrivileges)
		authGroup.POST("/roles/:id/privileges", roleAPI.AssignPrivileges)
		authGroup.GET("/roles/:id/users", roleAPI.GetRoleUsers)
		authGroup.POST("/roles/:id/users", roleAPI.AddUser)
		authGroup.POST("/roles/:id/reset-users", roleAPI.ResetUsers)
		authGroup.POST("/privileges", roleAPI.CreatePrivilege)
		authGroup.PUT("/privileges/:id", roleAPI.UpdatePrivilege)
		authGroup.DELETE("/privileges/:id", roleAPI.DeletePrivilege)

		// 用户管理
		authGroup.GET("/users", userAPI.GetUserList)
		authGroup.GET("/users/export", userAPI.ExportUsersCSV)
		authGroup.POST("/users", userAPI.CreateUser)
		authGroup.GET("/users/:id", userAPI.GetUserByID)
		authGroup.PUT("/users/:id", userAPI.UpdateUser)
		authGroup.DELETE("/users/:id", userAPI.DeleteUser)
		authGroup.PUT("/users/:id/password", userAPI.ChangePassword)
		authGroup.PUT("/users/:id/forbidden", userAPI.ToggleUserForbidden)
		authGroup.GET("/users/for-import", userAPI.GetUsersForImport)

		// 联系人管理
		authGroup.GET("/contacts", contactAPI.GetContacts)
		authGroup.GET("/contacts/all", contactAPI.GetAllContacts)
		authGroup.POST("/contacts", contactAPI.SaveContact)
		authGroup.PUT("/contacts/:id", contactAPI.UpdateContact)
		authGroup.DELETE("/contacts/:id", contactAPI.DeleteContact)
		authGroup.GET("/contacts/:id", contactAPI.GetContact)
		authGroup.POST("/contacts/import-users", contactAPI.ImportContactsFromUsers)
	}
}
