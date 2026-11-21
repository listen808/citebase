package search

import (
	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware"
)

// RegisterRoutes 注册搜索相关路由
func RegisterRoutes(v1 *gin.RouterGroup) {
	searchAPI := NewSearchAPI()

	// 公开（可选认证）搜索接口
	v1.GET("/search/fulltext", middleware.OptionalAuthMiddleware(), searchAPI.SearchDocuments)
	v1.POST("/search/hybrid", middleware.OptionalAuthMiddleware(), searchAPI.HybridSearch)
	v1.GET("/search/stats", middleware.OptionalAuthMiddleware(), searchAPI.GetIndexStats)
	v1.GET("/search/suggest", middleware.OptionalAuthMiddleware(), searchAPI.SearchSuggest)
	v1.GET("/search/synonyms", middleware.OptionalAuthMiddleware(), searchAPI.GetSynonyms)
	v1.GET("/search/expand", middleware.OptionalAuthMiddleware(), searchAPI.ExpandQueryWithSynonyms)
	v1.GET("/search/tags", middleware.OptionalAuthMiddleware(), searchAPI.GetTags)

	// 需要认证 + RBAC（索引写操作与含文件系统路径的诊断信息仅限管理员）
	authGroup := v1.Group("")
	authGroup.Use(middleware.AuthMiddleware(), middleware.AutoRBAC())
	{
		authGroup.POST("/search/reindex", searchAPI.ReindexAll)
		authGroup.GET("/search/reindex/status", searchAPI.GetReindexStatus)
		authGroup.GET("/search/init", searchAPI.InitIndexDiagnostic)
		authGroup.POST("/search/index/:id", searchAPI.IndexDocument)
		// 监控统计：既含索引目录绝对路径，也聚合 aw_search_log 里的查询原文与 user_id
		authGroup.GET("/search/monitor", searchAPI.GetSearchMonitor)
		// 单文档索引诊断：返回绝对文件路径与正文前 200 字符，且不查空间权限 ——
		// 挂在 AuthMiddleware 下等于任何登录用户可读任意私有文档正文。
		// 唯一消费者是管理端 SearchMonitorView，所以抬到管理员档不影响任何用户侧功能。
		authGroup.GET("/search/debug/:id", searchAPI.DebugIndexDocument)
	}
}
