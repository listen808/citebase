package schema

import (
	"github.com/gin-gonic/gin"
	texttosqlapi "github.com/listen808/citebase/internal/api/ai/texttosql"
	"github.com/listen808/citebase/internal/middleware"
)

// RegisterRoutes 注册 Schema 和 Text-to-SQL 相关路由
func RegisterRoutes(v1 *gin.RouterGroup) {
	schemaAPI := NewSchemaAPI()
	datasourceAPI := texttosqlapi.NewDataSourceAPI()
	sqlHistoryAPI := texttosqlapi.NewSQLHistoryAPI()

	authGroup := v1.Group("")
	authGroup.Use(middleware.AuthMiddleware(), middleware.AutoRBAC())
	{
		authGroup.POST("/schema-spaces", schemaAPI.CreateSchemaSpace)
		authGroup.GET("/schema-spaces", schemaAPI.ListSchemaSpaces)
		authGroup.GET("/schema-spaces/:id", schemaAPI.GetSchemaSpace)
		authGroup.DELETE("/schema-spaces/:id", schemaAPI.DeleteSchemaSpace)
		authGroup.GET("/schema-spaces/:id/tables", schemaAPI.ListTables)
		authGroup.GET("/schema-spaces/:id/tables/:table_name", schemaAPI.GetTableDetail)
		authGroup.POST("/schema-spaces/:id/scan", schemaAPI.StartScan)
		authGroup.GET("/schema-spaces/:id/debug-scan", schemaAPI.DebugScan)
		authGroup.GET("/schema-spaces/:id/scan-tasks", schemaAPI.ListScanTasks)
		authGroup.DELETE("/schema-spaces/:id/scan-tasks", schemaAPI.DeleteAllScanTasks)
		authGroup.GET("/schema-scan-tasks/:task_id", schemaAPI.GetScanTaskStatus)
		authGroup.DELETE("/schema-scan-tasks/:task_id", schemaAPI.DeleteScanTask)
		authGroup.POST("/schema-scan-tasks/:task_id/pause", schemaAPI.PauseScanTask)
		authGroup.POST("/schema-scan-tasks/:task_id/resume", schemaAPI.ResumeScanTask)
		authGroup.POST("/schema-scan-tasks/:task_id/cancel", schemaAPI.CancelScanTask)
		authGroup.GET("/schema-spaces/:id/table-cache", schemaAPI.ListTableCache)
		authGroup.POST("/schema-spaces/:id/refresh-knowledge", schemaAPI.RefreshKnowledge)
		authGroup.GET("/schema-spaces/:id/stats", schemaAPI.GetSchemaStats)
		authGroup.POST("/schema-spaces/:id/ask", schemaAPI.AskQuestion)
		authGroup.GET("/schema-spaces/:id/rules", schemaAPI.ListBusinessRules)
		authGroup.POST("/schema-spaces/:id/rules", schemaAPI.CreateBusinessRule)
		authGroup.DELETE("/schema-spaces/:id/rules/:rule_id", schemaAPI.DeleteBusinessRule)
		authGroup.POST("/schema-export-csv", schemaAPI.ExportCSV)
	}

	authTextGroup := v1.Group("")
	// 必须挂 AutoRBAC：数据源管理（改 Host/口令、让服务端主动连库）需管理员；
	// SQL 历史在 PrivilegeMap 中显式为 MinRole 3，任何登录用户可用，归属由 Service 层强制校验。
	authTextGroup.Use(middleware.AuthMiddleware(), middleware.AutoRBAC())
	{
		authTextGroup.GET("/datasources", datasourceAPI.List)
		authTextGroup.POST("/datasources", datasourceAPI.Create)
		authTextGroup.GET("/datasources/:id", datasourceAPI.Get)
		authTextGroup.PUT("/datasources/:id", datasourceAPI.Update)
		authTextGroup.DELETE("/datasources/:id", datasourceAPI.Delete)
		authTextGroup.POST("/datasources/test-connection", datasourceAPI.TestConnection)
		authTextGroup.POST("/datasources/:id/sync", datasourceAPI.Sync)
		authTextGroup.GET("/sql-history", sqlHistoryAPI.List)
		authTextGroup.GET("/sql-history/favorites", sqlHistoryAPI.ListFavorites)
		authTextGroup.POST("/sql-history/save", sqlHistoryAPI.Save)
		authTextGroup.GET("/sql-history/:id", sqlHistoryAPI.Get)
		authTextGroup.PUT("/sql-history/:id/favorite", sqlHistoryAPI.ToggleFavorite)
		authTextGroup.PUT("/sql-history/:id/rate", sqlHistoryAPI.Rate)
		authTextGroup.DELETE("/sql-history/:id", sqlHistoryAPI.Delete)
	}
}
