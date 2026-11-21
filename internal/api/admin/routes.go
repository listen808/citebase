package admin

import (
	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware"
)

// RegisterRoutes 注册管理后台相关路由（邮件/链接/插件/日志/统计/系统配置/备份）
func RegisterRoutes(v1 *gin.RouterGroup) {
	emailAPI := NewEmailAPI()
	configAPI := NewConfigAPI()
	statsAPI := NewStatsAPI()
	logAPI := NewLogAPI()
	complianceAPI := NewComplianceAPI()
	apiKeyAPI := NewApiKeyAPI()
	exportAPI := NewExportAPI()
	linkAPI := NewLinkAPI()
	pluginAPI := NewPluginAPI()
	backupAPI := NewBackupAPI()

	// 需要认证 + RBAC
	// P1 修复：AccessLog 已提升为全局中间件（server.go setupGin），此处移除避免重复记录
	authGroup := v1.Group("")
	authGroup.Use(middleware.AuthMiddleware(), middleware.AutoRBAC())
	{
		// 邮件管理
		authGroup.GET("/emails", emailAPI.GetEmails)
		authGroup.POST("/emails", emailAPI.CreateEmail)
		authGroup.GET("/emails/:id", emailAPI.GetEmail)
		authGroup.PUT("/emails/:id", emailAPI.UpdateEmail)
		authGroup.DELETE("/emails/:id", emailAPI.DeleteEmail)
		authGroup.PUT("/emails/:id/default", emailAPI.SetDefaultEmail)
		authGroup.POST("/emails/test", emailAPI.TestEmail)

		// 系统配置
		authGroup.GET("/configs", configAPI.GetConfigs)
		authGroup.PUT("/configs", configAPI.UpdateConfig)
		authGroup.GET("/configs/value", configAPI.GetConfigValue)

		// 统计与监控
		authGroup.GET("/stats/dashboard", statsAPI.GetDashboardStats)
		authGroup.GET("/stats/server", statsAPI.GetServerStatus)
		authGroup.GET("/stats/time", statsAPI.GetServerTime)
		authGroup.GET("/stats/collect-doc-rank", statsAPI.GetCollectDocRank)

		// 合规报告（G038）
		authGroup.GET("/stats/compliance-report", complianceAPI.GetComplianceReport)
		authGroup.GET("/stats/compliance-report/export", complianceAPI.ExportComplianceReport)

		// 数据导出中心（G037）
		authGroup.GET("/stats/export/:module", exportAPI.ExportData)

		// 开放平台密钥管理（G029）
		authGroup.POST("/open/api-keys", apiKeyAPI.CreateApiKey)
		authGroup.GET("/open/api-keys", apiKeyAPI.ListApiKeys)
		authGroup.POST("/open/api-keys/:id/revoke", apiKeyAPI.RevokeApiKey)

		// 日志
		authGroup.GET("/logs/activity", logAPI.GetUserActivity)
		authGroup.GET("/logs/system", logAPI.GetSystemLogs)
		authGroup.GET("/logs/:id", logAPI.GetLogDetail)
		authGroup.GET("/logs/activity/:id", logAPI.GetActivityLogDetail)
		authGroup.GET("/logs/users/:id/activity", logAPI.GetUserActivityByID)
		authGroup.GET("/logs/recent-errors", logAPI.GetRecentErrors)

		// 快捷链接管理
		authGroup.GET("/links", linkAPI.GetLinks)
		authGroup.GET("/links/all", linkAPI.GetAllLinks)
		authGroup.POST("/links", linkAPI.SaveLink)
		authGroup.PUT("/links/:id", linkAPI.UpdateLink)
		authGroup.DELETE("/links/:id", linkAPI.DeleteLink)
		// 打开计数：任何登录用户可上报（见 PrivilegeMap 的 MinRole 3），
		// 不挂管理员前缀的默认拒绝，否则将来链接摆到用户面前时计数会全 403。
		authGroup.POST("/links/:id/click", linkAPI.RecordClick)

		// 插件管理
		authGroup.GET("/plugins", pluginAPI.GetPlugins)
		authGroup.POST("/plugins", pluginAPI.CreatePlugin)
		authGroup.GET("/plugins/:id", pluginAPI.GetPlugin)
		authGroup.PUT("/plugins/:id", pluginAPI.UpdatePlugin)
		authGroup.PUT("/plugins/:id/config", pluginAPI.UpdatePluginConfig)
		authGroup.PUT("/plugins/:id/toggle", pluginAPI.TogglePlugin)
		authGroup.DELETE("/plugins/:id", pluginAPI.DeletePlugin)

		// 备份与恢复
		authGroup.GET("/backups", backupAPI.ListBackups)
		authGroup.POST("/backups", backupAPI.CreateBackup)
		authGroup.POST("/backups/import", backupAPI.ImportBackup)
		authGroup.GET("/backups/:id/download", backupAPI.DownloadBackup)
		authGroup.DELETE("/backups/:id", backupAPI.DeleteBackup)
		authGroup.POST("/backups/:id/restore", backupAPI.RestoreBackup)
	}
}
