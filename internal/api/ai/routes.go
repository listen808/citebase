package ai

import (
	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/ai/facade"
	"github.com/listen808/citebase/internal/middleware"
	"github.com/listen808/citebase/internal/repository"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	aiSvc "github.com/listen808/citebase/internal/service/ai"
)

// RegisterAIRoutes 注册所有 /api/v1/ai/ 开头的路由，
// 以及 /ai/data-query/ 子组的路由。aiFacade 为外部传入的 AI Facade 实例。
func RegisterAIRoutes(v1 *gin.RouterGroup, aiFacade *facade.AIFacade) {
	chatAPI := NewChatAPI()
	aiConfigAPI := NewAIConfigAPI()
	memoryAPI := NewMemoryAPI()
	intentAPI := NewIntentAPI()
	dataSensitivityAPI := NewDataSensitivityAPI()
	observabilityAPI := NewObservabilityAPI()
	aiFacadeAPI := NewAIFacadeAPI(aiFacade, aiSvc.NewSQLHistoryService(airepo.NewSQLHistoryRepo(repository.DB)))
	sqlTrustAPI := NewSQLTrustAPI(
		airepo.NewSQLSecurityPolicyRepo(repository.DB),
		airepo.NewSQLHistoryRepo(repository.DB),
		airepo.NewAIAuditLogRepository(),
	)

	ai := v1.Group("/ai", middleware.AIEnabledGate())

	// ===== AI 健康检查与能力探测（不挂 AIEnabledGate） =====
	// health 是轻量探针，不发真实 LLM 请求；config 是前端能力探测通道（enabled 字段决定是否展示 AI UI）
	// 注意必须注册在 v1 上：挂在下面带 AIEnabledGate 的 group 上会继承门禁，
	// AI 关闭时这两条探针也跟着 503，前端能力探测永远失败（App.vue 每次页面加载
	// 都会打一发 GET /ai/config，症状是全站每个页面都多一条 503）。
	v1.GET("/ai/health", middleware.OptionalAuthMiddleware(), chatAPI.Health)
	v1.GET("/ai/config", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiConfigAPI.GetAIConfig)

	// ===== AI 意图识别（统一入口） =====
	ai.POST("/intent/detect", middleware.AuthMiddleware(), intentAPI.DetectIntent)

	// ===== AI 问答和对话 =====
	ai.POST("/ask", middleware.AuthMiddleware(), chatAPI.Ask)
	ai.POST("/stream/ask", middleware.AuthMiddleware(), chatAPI.StreamAsk)
	ai.POST("/chat", middleware.AuthMiddleware(), chatAPI.ChatWithDoc)
	ai.POST("/summary/:id", middleware.AuthMiddleware(), chatAPI.GenerateSummary)
	ai.POST("/write", middleware.AuthMiddleware(), chatAPI.WriteAssist)
	ai.POST("/image/describe", middleware.AuthMiddleware(), chatAPI.DescribeImage)
	ai.POST("/agent/ask", middleware.AuthMiddleware(), chatAPI.AgentAsk)
	ai.POST("/agent/stream/ask", middleware.AuthMiddleware(), chatAPI.AgentAskStream)
	ai.POST("/agent/orchestrate", middleware.AuthMiddleware(), chatAPI.OrchestrateAgents)
	ai.POST("/voice/transcribe", middleware.AuthMiddleware(), chatAPI.VoiceTranscribe)
	ai.POST("/voice/synthesize", middleware.AuthMiddleware(), chatAPI.VoiceSynthesize)
	ai.POST("/table/generate", middleware.AuthMiddleware(), chatAPI.GenerateTable)
	ai.POST("/report/generate", middleware.AuthMiddleware(), chatAPI.GenerateReport)
	ai.POST("/email/generate", middleware.AuthMiddleware(), chatAPI.GenerateEmail)
	ai.POST("/translate", middleware.AuthMiddleware(), chatAPI.TranslateText)
	ai.GET("/recommendations", middleware.AuthMiddleware(), chatAPI.GetRecommendations)
	ai.GET("/quality-patrol", middleware.AuthMiddleware(), chatAPI.RunQualityPatrol)
	ai.GET("/templates", middleware.AuthMiddleware(), chatAPI.ListTemplates)
	ai.POST("/templates/apply", middleware.AuthMiddleware(), chatAPI.ApplyTemplate)
	ai.GET("/vector/stats", middleware.AuthMiddleware(), chatAPI.VectorStats)
	ai.POST("/vector/sync", middleware.AuthMiddleware(), middleware.AutoRBAC(), chatAPI.VectorSyncNotify)
	ai.POST("/vector/reindex/unified", middleware.AuthMiddleware(), middleware.AutoRBAC(), chatAPI.ReindexVectorsUnified)
	// 以下 3 个端点为 P2-3 之前的旧入口，详情请使用 /ai/vector/reindex/unified，逐步 sunset
	ai.POST("/vector/reindex", middleware.AuthMiddleware(), middleware.AutoRBAC(), middleware.DeprecationMiddleware("/ai/vector/reindex/unified"), chatAPI.ReindexVectors)
	ai.POST("/vector/reindex/space/:space_id", middleware.AuthMiddleware(), middleware.AutoRBAC(), middleware.DeprecationMiddleware("/ai/vector/reindex/unified"), chatAPI.ReindexSpaceVectors)
	ai.POST("/vector/reindex/doc/:doc_id", middleware.AuthMiddleware(), middleware.AutoRBAC(), middleware.DeprecationMiddleware("/ai/vector/reindex/unified"), chatAPI.ReindexDocVector)
	ai.GET("/conversations", middleware.AuthMiddleware(), chatAPI.ListConversations)
	ai.POST("/conversations", middleware.AuthMiddleware(), chatAPI.CreateConversation)
	ai.GET("/conversations/:id", middleware.AuthMiddleware(), chatAPI.GetConversation)
	ai.PUT("/conversations/:id", middleware.AuthMiddleware(), chatAPI.UpdateConversation)
	ai.DELETE("/conversations/:id", middleware.AuthMiddleware(), chatAPI.DeleteConversation)
	ai.GET("/token/usage", middleware.AuthMiddleware(), middleware.RequireAdmin(), chatAPI.GetTokenUsage)
	ai.GET("/token/usage/by-model", middleware.AuthMiddleware(), middleware.RequireAdmin(), chatAPI.GetTokenUsageByModel)
	ai.GET("/token/usage/by-user", middleware.AuthMiddleware(), middleware.RequireAdmin(), chatAPI.GetTokenUsageByUser)
	ai.GET("/token/usage/daily", middleware.AuthMiddleware(), middleware.RequireAdmin(), chatAPI.GetTokenUsageDailyTrend)
	ai.GET("/observability/metrics", middleware.AuthMiddleware(), middleware.AutoRBAC(), middleware.RequireAdmin(), observabilityAPI.GetMetrics)
	ai.GET("/observability/traces", middleware.AuthMiddleware(), middleware.AutoRBAC(), middleware.RequireAdmin(), observabilityAPI.ListTraces)
	ai.GET("/observability/traces/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), middleware.RequireAdmin(), observabilityAPI.GetTrace)
	ai.GET("/observability/dashboard", middleware.AuthMiddleware(), middleware.AutoRBAC(), middleware.RequireAdmin(), observabilityAPI.GetDashboard)
	ai.GET("/cache/stats", middleware.AuthMiddleware(), chatAPI.GetQueryCacheStats)
	ai.POST("/cache/clear", middleware.AuthMiddleware(), middleware.AutoRBAC(), chatAPI.ClearQueryCache)
	// P0-7: Widget 端点虽然是嵌入用的公开 HTML/JS 资源，
	// 但仍需 OptionalAuthMiddleware 解析用户上下文（与 /ai/health 一致），
	// 避免完全裸暴露且后续可在 handler 中按用户身份做差异化
	ai.GET("/widget", middleware.OptionalAuthMiddleware(), chatAPI.ServeWidget)
	ai.GET("/widget.js", middleware.OptionalAuthMiddleware(), chatAPI.ServeWidgetJS)
	ai.GET("/embed-code", middleware.AuthMiddleware(), chatAPI.GetEmbedCode)
	ai.POST("/feedback", middleware.AuthMiddleware(), chatAPI.SubmitFeedback)
	ai.GET("/feedback/stats", middleware.AuthMiddleware(), chatAPI.GetFeedbackStats)
	ai.GET("/dashboard", middleware.AuthMiddleware(), chatAPI.GetAIDashboard)
	ai.POST("/auto-tag", middleware.AuthMiddleware(), middleware.AutoRBAC(), chatAPI.AutoTag)
	ai.GET("/audit-logs", middleware.AuthMiddleware(), chatAPI.ListAIAuditLogs)
	ai.POST("/ask/debug", middleware.AuthMiddleware(), middleware.RequireAdmin(), chatAPI.AskWithDebug)
	ai.GET("/workflows", middleware.AuthMiddleware(), middleware.AutoRBAC(), chatAPI.ListWorkflows)
	ai.POST("/workflows", middleware.AuthMiddleware(), middleware.AutoRBAC(), chatAPI.CreateWorkflow)
	ai.POST("/workflows/generate", middleware.AuthMiddleware(), middleware.AutoRBAC(), chatAPI.GenerateWorkflow)
	ai.PUT("/workflows/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), chatAPI.UpdateWorkflow)
	ai.POST("/workflows/:id/execute", middleware.AuthMiddleware(), middleware.AutoRBAC(), chatAPI.ExecuteWorkflow)
	ai.POST("/workflows/:id/toggle", middleware.AuthMiddleware(), middleware.AutoRBAC(), chatAPI.ToggleWorkflow)
	ai.DELETE("/workflows/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), chatAPI.DeleteWorkflow)
	ai.POST("/workflows/stop/:execId", middleware.AuthMiddleware(), middleware.AutoRBAC(), chatAPI.StopWorkflow)
	ai.POST("/import", middleware.AuthMiddleware(), middleware.AutoRBAC(), chatAPI.ImportDocument)
	ai.POST("/search/overview", middleware.AuthMiddleware(), chatAPI.SearchOverview)
	ai.POST("/multi-hop-ask", middleware.AuthMiddleware(), chatAPI.MultiHopAsk)
	ai.POST("/self-rag-ask", middleware.AuthMiddleware(), chatAPI.SelfRAGAsk)

	// ===== LLM Wiki 知识编译（P0-1） =====
	wikiAPI := NewWikiAPI()
	ai.POST("/wiki/compile", middleware.AuthMiddleware(), middleware.AutoRBAC(), wikiAPI.Compile)
	ai.GET("/wiki/pages", middleware.AuthMiddleware(), wikiAPI.ListPages)
	ai.GET("/wiki/pages/:slug", middleware.AuthMiddleware(), wikiAPI.GetPage)
	ai.POST("/wiki/article/generate", middleware.AuthMiddleware(), wikiAPI.GenerateArticle)
	ai.POST("/wiki/pages/:slug/publish", middleware.AuthMiddleware(), middleware.AutoRBAC(), wikiAPI.Publish)
	ai.POST("/wiki/pages/:slug/human-edit", middleware.AuthMiddleware(), wikiAPI.MarkHumanEdited)
	ai.GET("/wiki/index", middleware.AuthMiddleware(), wikiAPI.GetIndex)
	ai.POST("/wiki/search", middleware.AuthMiddleware(), wikiAPI.Search)
	ai.GET("/wiki/related/:slug", middleware.AuthMiddleware(), wikiAPI.GetRelated)
	ai.DELETE("/wiki/pages/:slug", middleware.AuthMiddleware(), middleware.AutoRBAC(), wikiAPI.DeletePage)
	ai.POST("/wiki/pages/:slug/supersede", middleware.AuthMiddleware(), middleware.AutoRBAC(), wikiAPI.SupersedePage)
	ai.POST("/wiki/sources/remove", middleware.AuthMiddleware(), middleware.AutoRBAC(), wikiAPI.RemoveSource)
	ai.GET("/wiki/purpose", middleware.AuthMiddleware(), wikiAPI.GetPurpose)
	ai.PUT("/wiki/purpose", middleware.AuthMiddleware(), middleware.AutoRBAC(), wikiAPI.UpdatePurpose)
	ai.GET("/wiki/schema", middleware.AuthMiddleware(), wikiAPI.GetSchema)
	ai.PUT("/wiki/schema", middleware.AuthMiddleware(), middleware.AutoRBAC(), wikiAPI.UpdateSchema)
	// 版本历史与回滚 / 写入时矛盾标注 / 好答案回填（P0）
	ai.GET("/wiki/pages/:slug/history", middleware.AuthMiddleware(), wikiAPI.ListHistory)
	ai.POST("/wiki/pages/:slug/rollback", middleware.AuthMiddleware(), middleware.AutoRBAC(), wikiAPI.RollbackPage)
	ai.POST("/wiki/pages/:slug/contradictions", middleware.AuthMiddleware(), middleware.AutoRBAC(), wikiAPI.MarkContradictions)
	ai.POST("/wiki/propose-analysis", middleware.AuthMiddleware(), middleware.AutoRBAC(), wikiAPI.ProposeAnalysis)
	ai.GET("/wiki/export", middleware.AuthMiddleware(), wikiAPI.ExportWiki)

	// ===== LLM Wiki 治理：Pin / Review / Lint / Insights（P1-P2） =====
	ai.POST("/wiki/pins", middleware.AuthMiddleware(), wikiAPI.CreatePin)
	ai.GET("/wiki/pins", middleware.AuthMiddleware(), wikiAPI.ListPins)
	ai.POST("/wiki/pins/:id/resolve", middleware.AuthMiddleware(), wikiAPI.ResolvePin)
	ai.POST("/wiki/pages/:slug/check-pins", middleware.AuthMiddleware(), wikiAPI.CheckPins)
	ai.GET("/wiki/reviews", middleware.AuthMiddleware(), wikiAPI.ListReviews)
	ai.POST("/wiki/reviews/:id/resolve", middleware.AuthMiddleware(), wikiAPI.ResolveReview)
	ai.POST("/wiki/lint", middleware.AuthMiddleware(), middleware.AutoRBAC(), wikiAPI.RunLint)
	ai.GET("/wiki/lint", middleware.AuthMiddleware(), wikiAPI.ListLint)
	ai.GET("/wiki/insights", middleware.AuthMiddleware(), wikiAPI.GetInsights)
	ai.POST("/wiki/deep-research", middleware.AuthMiddleware(), wikiAPI.RunDeepResearch)

	// ===== MCP 服务端（P2-4）：把知识库暴露给外部 Agent =====
	ai.POST("/mcp/server", middleware.AuthMiddleware(), chatAPI.HandleMCPServer)
	ai.GET("/wiki/links", middleware.AuthMiddleware(), wikiAPI.GetLinks)

	// ===== AI 配置 =====
	// GET /ai/config 已在上方免门禁注册（能力探测通道），此处只挂写操作
	ai.POST("/config/update", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiConfigAPI.UpdateAIConfig)
	ai.POST("/config/test", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiConfigAPI.TestAIConnection)
	ai.POST("/config/models", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiConfigAPI.ListAIModels)
	ai.POST("/config/schedule", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiConfigAPI.UpdateAISchedule)
	ai.GET("/config/schedule", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiConfigAPI.GetAISchedule)
	ai.GET("/analyzer/config", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiConfigAPI.GetAnalyzerConfig)
	ai.POST("/analyzer/config", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiConfigAPI.UpdateAnalyzerConfig)
	ai.GET("/rag/config", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiConfigAPI.GetRAGConfig)
	ai.POST("/rag/config", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiConfigAPI.UpdateRAGConfig)
	ai.POST("/rag/test", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiConfigAPI.TestRAGRetrieval)
	ai.POST("/ragas/evaluate", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiConfigAPI.RAGASEvaluate)
	ai.GET("/ragas/results", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiConfigAPI.RAGASResults)
	ai.GET("/ragas/stats", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiConfigAPI.RAGASStats)
	ai.DELETE("/ragas/results/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiConfigAPI.RAGASDeleteResult)

	// P0-4：RAG 回答反馈沿用 chatAPI.SubmitFeedback / GetFeedbackStats（上方 :82-83，
	// 落 aw_ai_feedback 表）。此处不再另注册同名路由。
	ai.GET("/mcp/servers", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiConfigAPI.ListMCPServers)
	ai.POST("/mcp/servers", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiConfigAPI.AddMCPServer)
	ai.DELETE("/mcp/servers/:name", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiConfigAPI.RemoveMCPServer)
	ai.GET("/mcp/tools", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiConfigAPI.ListMCPServerTools)
	ai.GET("/features/maturity", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiConfigAPI.GetFeatureMaturity)

	// ===== AI 记忆系统 =====
	ai.GET("/memory", middleware.AuthMiddleware(), memoryAPI.ListMemories)
	ai.POST("/memory", middleware.AuthMiddleware(), memoryAPI.CreateMemory)
	ai.DELETE("/memory/:id", middleware.AuthMiddleware(), memoryAPI.DeleteMemory)
	ai.POST("/memory/search", middleware.AuthMiddleware(), memoryAPI.SearchMemories)
	ai.POST("/memory/extract", middleware.AuthMiddleware(), memoryAPI.ExtractMemories)
	ai.POST("/memory/recall", middleware.AuthMiddleware(), memoryAPI.RecallMemories)
	ai.GET("/memory/stats", middleware.AuthMiddleware(), memoryAPI.GetMemoryStats)
	ai.GET("/memory/pending", middleware.AuthMiddleware(), memoryAPI.ListPendingMemories)
	ai.POST("/memory/:id/confirm", middleware.AuthMiddleware(), memoryAPI.ConfirmMemory)

	// ===== AI Data Query（统一数据查询入口） =====
	// Phase 3/4 API 实例（与上方 aiFacade 共享同一命名空间 /ai/data-query/*）
	traceAPI := NewSQLTraceAPI()
	promptVersionAPI := NewPromptVersionAPI()
	errorDashboardAPI := NewErrorDashboardAPI()
	abExperimentAPI := NewABExperimentAPI()

	aiDataQuery := ai.Group("/data-query")
	{
		// SQL 生成/执行：普通用户可用
		aiDataQuery.POST("/generate-sql", middleware.AuthMiddleware(), aiFacadeAPI.GenerateSQL)
		aiDataQuery.POST("/generate-sql-rag", middleware.AuthMiddleware(), aiFacadeAPI.GenerateSQLWithRAG)
		aiDataQuery.POST("/generate-sql-stream", middleware.AuthMiddleware(), aiFacadeAPI.GenerateSQLStream)
		aiDataQuery.POST("/generate-sql-multi", middleware.AuthMiddleware(), aiFacadeAPI.GenerateMultiCandidateSQL)
		aiDataQuery.POST("/evaluate-sql", middleware.AuthMiddleware(), aiFacadeAPI.EvaluateSQL)
		aiDataQuery.POST("/execute-sql", middleware.AuthMiddleware(), aiFacadeAPI.ExecuteSQL)
		// Rules CRUD：管理员
		aiDataQuery.GET("/rules", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiFacadeAPI.ListRules)
		aiDataQuery.POST("/rules", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiFacadeAPI.CreateRule)
		aiDataQuery.GET("/rules/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiFacadeAPI.GetRule)
		aiDataQuery.PUT("/rules/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiFacadeAPI.UpdateRule)
		aiDataQuery.DELETE("/rules/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiFacadeAPI.DeleteRule)
		// Tables：普通用户可用
		aiDataQuery.GET("/tables", middleware.AuthMiddleware(), aiFacadeAPI.ListTables)
		aiDataQuery.GET("/tables/:name", middleware.AuthMiddleware(), aiFacadeAPI.GetTableDetail)
		aiDataQuery.GET("/tables/:name/relations", middleware.AuthMiddleware(), aiFacadeAPI.GetTableRelations)
		// Relations：管理员
		aiDataQuery.POST("/relations", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiFacadeAPI.CreateRelation)
		aiDataQuery.DELETE("/relations/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiFacadeAPI.DeleteRelation)
		// Knowledge & Scan：管理员
		aiDataQuery.POST("/refresh", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiFacadeAPI.RefreshKnowledge)
		aiDataQuery.GET("/spaces/:space_id/tables", middleware.AuthMiddleware(), aiFacadeAPI.GetSpaceTables)
		aiDataQuery.POST("/spaces/:space_id/scan", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiFacadeAPI.ScanSpace)
		aiDataQuery.POST("/spaces/:space_id/scan-rag", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiFacadeAPI.ScanSpaceWithRAG)
		// Prompt Templates CRUD：管理员
		aiDataQuery.GET("/prompts", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiFacadeAPI.ListPromptTemplates)
		aiDataQuery.POST("/prompts", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiFacadeAPI.CreatePromptTemplate)
		aiDataQuery.GET("/prompts/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiFacadeAPI.GetPromptTemplateByID)
		aiDataQuery.PUT("/prompts/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiFacadeAPI.UpdatePromptTemplate)
		aiDataQuery.DELETE("/prompts/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiFacadeAPI.DeletePromptTemplate)
		aiDataQuery.PUT("/prompts/:id/default", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiFacadeAPI.SetDefaultPromptTemplate)
		aiDataQuery.PUT("/prompts/:id/toggle", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiFacadeAPI.TogglePromptTemplate)
		aiDataQuery.GET("/prompts/builtin", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiFacadeAPI.ListBuiltInPromptTemplates)
		// SQL Examples：管理员管理，普通用户可查看
		aiDataQuery.GET("/examples", middleware.AuthMiddleware(), aiFacadeAPI.ListSQLExamples)
		aiDataQuery.GET("/examples/:id", middleware.AuthMiddleware(), aiFacadeAPI.GetSQLExample)
		aiDataQuery.POST("/examples", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiFacadeAPI.CreateSQLExample)
		aiDataQuery.PUT("/examples/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiFacadeAPI.UpdateSQLExample)
		aiDataQuery.PUT("/examples/:id/verify", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiFacadeAPI.VerifySQLExample)
		aiDataQuery.DELETE("/examples/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), aiFacadeAPI.DeleteSQLExample)
		// SQL Trace：管理员
		aiDataQuery.GET("/traces/:trace_id", middleware.AuthMiddleware(), middleware.AutoRBAC(), traceAPI.GetTrace)
		aiDataQuery.GET("/traces", middleware.AuthMiddleware(), middleware.AutoRBAC(), traceAPI.ListTraces)
		// Prompt Version：管理员
		aiDataQuery.GET("/prompt-versions", middleware.AuthMiddleware(), middleware.AutoRBAC(), promptVersionAPI.List)
		aiDataQuery.POST("/prompt-versions", middleware.AuthMiddleware(), middleware.AutoRBAC(), promptVersionAPI.Create)
		aiDataQuery.PUT("/prompt-versions/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), promptVersionAPI.Update)
		aiDataQuery.DELETE("/prompt-versions/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), promptVersionAPI.Delete)
		aiDataQuery.POST("/prompt-versions/:id/activate", middleware.AuthMiddleware(), middleware.AutoRBAC(), promptVersionAPI.Activate)
		// Error Dashboard：管理员
		aiDataQuery.GET("/error-summary", middleware.AuthMiddleware(), middleware.AutoRBAC(), errorDashboardAPI.Summary)
		aiDataQuery.GET("/recent-failures", middleware.AuthMiddleware(), middleware.AutoRBAC(), errorDashboardAPI.RecentFailures)
		// AB Experiment：管理员
		aiDataQuery.GET("/ab-experiments", middleware.AuthMiddleware(), middleware.AutoRBAC(), abExperimentAPI.List)
		aiDataQuery.POST("/ab-experiments", middleware.AuthMiddleware(), middleware.AutoRBAC(), abExperimentAPI.Create)
		aiDataQuery.GET("/ab-experiments/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), abExperimentAPI.Get)
		aiDataQuery.PUT("/ab-experiments/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), abExperimentAPI.Update)
		aiDataQuery.DELETE("/ab-experiments/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), abExperimentAPI.Delete)
	}

	// ===== 数据脱敏与 PII 检测 =====
	ai.GET("/data-sensitivity", middleware.AuthMiddleware(), middleware.AutoRBAC(), dataSensitivityAPI.ListRules)
	ai.POST("/data-sensitivity", middleware.AuthMiddleware(), middleware.AutoRBAC(), dataSensitivityAPI.CreateRule)
	ai.PUT("/data-sensitivity/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), dataSensitivityAPI.UpdateRule)
	ai.DELETE("/data-sensitivity/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), dataSensitivityAPI.DeleteRule)
	ai.POST("/detect-pii", middleware.AuthMiddleware(), dataSensitivityAPI.DetectPII)

	// ===== SQL 可信闭环 =====
	ai.GET("/sql-security-policy", middleware.AuthMiddleware(), middleware.AutoRBAC(), sqlTrustAPI.GetSQLSecurityPolicy)
	ai.PUT("/sql-security-policy", middleware.AuthMiddleware(), middleware.AutoRBAC(), sqlTrustAPI.UpdateSQLSecurityPolicy)
	ai.POST("/sql-feedback", middleware.AuthMiddleware(), sqlTrustAPI.SubmitSQLFeedback)
	ai.GET("/sql-history/:id/explanation", middleware.AuthMiddleware(), sqlTrustAPI.GetSQLExplanation)
	ai.GET("/sql-audit-traces", middleware.AuthMiddleware(), middleware.AutoRBAC(), sqlTrustAPI.ListSQLAuditTraces)
	ai.GET("/sql-audit-traces/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), sqlTrustAPI.GetSQLAuditTrace)

	// ===== 语义层 API =====
	semanticAPI := NewSemanticLayerAPI()
	ai.GET("/semantic-metrics", middleware.AuthMiddleware(), middleware.AutoRBAC(), semanticAPI.ListMetrics)
	ai.POST("/semantic-metrics", middleware.AuthMiddleware(), middleware.AutoRBAC(), semanticAPI.CreateMetric)
	ai.PUT("/semantic-metrics/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), semanticAPI.UpdateMetric)
	ai.DELETE("/semantic-metrics/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), semanticAPI.DeleteMetric)
	ai.GET("/semantic-dimensions", middleware.AuthMiddleware(), middleware.AutoRBAC(), semanticAPI.ListDimensions)
	ai.POST("/semantic-dimensions", middleware.AuthMiddleware(), middleware.AutoRBAC(), semanticAPI.CreateDimension)
	ai.DELETE("/semantic-dimensions/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), semanticAPI.DeleteDimension)

	// ===== 知识库管理 API =====
	kbAPI := NewKnowledgeBaseAPI()
	ai.GET("/knowledge-bases", middleware.AuthMiddleware(), kbAPI.ListBases)
	ai.GET("/knowledge-bases/:id/chunks", middleware.AuthMiddleware(), kbAPI.ListChunks)
	ai.POST("/knowledge-bases/:id/test-retrieval", middleware.AuthMiddleware(), kbAPI.TestRetrieval)
	ai.GET("/knowledge-bases/:id/stats", middleware.AuthMiddleware(), kbAPI.GetStats)

	// ===== 外部数据源连接器 =====
	connectorAPI := NewConnectorAPI()
	ai.POST("/datasources/scrape-url", middleware.AuthMiddleware(), connectorAPI.ScrapeURL)
	ai.POST("/datasources/scrape-site", middleware.AuthMiddleware(), connectorAPI.ScrapeSite)
	ai.POST("/datasources/upload", middleware.AuthMiddleware(), connectorAPI.UploadFile)

	// ===== 统一评估框架（RAGAS 数据集 + TestCase 端到端 + A/B 对比） =====
	evalAPI := NewEvalAPI()
	ai.GET("/eval/datasets", middleware.AuthMiddleware(), middleware.AutoRBAC(), evalAPI.ListDatasets)
	ai.POST("/eval/datasets", middleware.AuthMiddleware(), middleware.AutoRBAC(), evalAPI.CreateDataset)
	ai.GET("/eval/datasets/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), evalAPI.GetDataset)
	ai.DELETE("/eval/datasets/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), evalAPI.DeleteDataset)
	ai.POST("/eval/run", middleware.AuthMiddleware(), middleware.AutoRBAC(), evalAPI.RunEval)
	ai.GET("/eval/results", middleware.AuthMiddleware(), middleware.AutoRBAC(), evalAPI.ListResults)
	ai.GET("/eval/results/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), evalAPI.GetResult)
	ai.DELETE("/eval/results/:id", middleware.AuthMiddleware(), middleware.AutoRBAC(), evalAPI.DeleteResult)
	ai.GET("/eval/testcases", middleware.AuthMiddleware(), middleware.AutoRBAC(), evalAPI.ListTestCases)
	ai.POST("/eval/testcases", middleware.AuthMiddleware(), middleware.AutoRBAC(), evalAPI.CreateTestCase)
	ai.POST("/eval/testcases/run", middleware.AuthMiddleware(), middleware.AutoRBAC(), evalAPI.RunTestCases)
	ai.POST("/eval/ab-compare", middleware.AuthMiddleware(), middleware.AutoRBAC(), evalAPI.ABCompare)
	ai.GET("/eval/auto", middleware.AuthMiddleware(), middleware.AutoRBAC(), evalAPI.GetAutoEvalStatus)
}
