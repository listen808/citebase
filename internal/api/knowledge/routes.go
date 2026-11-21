package knowledge

import (
	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware"
)

func RegisterRoutes(v1 *gin.RouterGroup) {
	analysisAPI := NewAnalysisAPI()
	aiAnalysisResultAPI := NewAIAnalysisResultAPI()

	// P0 补口：这些路由此前只挂 AuthMiddleware，PrivilegeMap 里的管理员约束（MinRole 2）
	// 根本没被执行，任意登录用户都能触发全库分析、合并聚类与改写文档评审状态。
	authRBAC := []gin.HandlerFunc{middleware.AuthMiddleware(), middleware.AutoRBAC()}

	v1.POST("/ai/analysis/start", append(authRBAC, analysisAPI.StartAnalysis)...)
	v1.GET("/ai/analysis/progress/:id", middleware.AuthMiddleware(), analysisAPI.GetAnalysisProgress)
	v1.GET("/ai/analysis/tasks", middleware.AuthMiddleware(), analysisAPI.ListAnalysisTasks)
	v1.POST("/ai/analysis/pause/:id", append(authRBAC, analysisAPI.PauseAnalysis)...)
	v1.POST("/ai/analysis/resume/:id", append(authRBAC, analysisAPI.ResumeAnalysis)...)

	v1.POST("/ai/doc/review", append(authRBAC, analysisAPI.UpdateDocReviewStatus)...)
	v1.GET("/ai/doc/mapping/:id", middleware.OptionalAuthMiddleware(), analysisAPI.GetDocMapping)

	v1.POST("/ai/analysis/detect-duplicates", append(authRBAC, analysisAPI.DetectDuplicates)...)
	v1.GET("/ai/analysis/clusters", append(authRBAC, analysisAPI.GetClusters)...)
	v1.POST("/ai/analysis/merge/:id", append(authRBAC, analysisAPI.MergeCluster)...)

	v1.GET("/ai/knowledge-graph", middleware.AuthMiddleware(), analysisAPI.GetKnowledgeGraph)
	v1.POST("/ai/entities/extract", append(authRBAC, analysisAPI.ExtractEntities)...)

	v1.GET("/ai/analysis-results/:spaceId", middleware.AuthMiddleware(), aiAnalysisResultAPI.GetResults)
	v1.GET("/ai/analysis-results/:spaceId/categories", middleware.AuthMiddleware(), aiAnalysisResultAPI.GetCategories)
	v1.GET("/ai/analysis-results/:spaceId/quality", middleware.AuthMiddleware(), aiAnalysisResultAPI.GetQualityStats)
	v1.GET("/ai/analysis-results/:spaceId/gaps", middleware.AuthMiddleware(), aiAnalysisResultAPI.GetKnowledgeGaps)
	v1.GET("/ai/analysis-results/doc/:doc_id", middleware.AuthMiddleware(), aiAnalysisResultAPI.GetResultByDocID)
}
