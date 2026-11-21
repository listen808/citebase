package main

import (
	"context"
	"encoding/json"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai"
	"github.com/listen808/citebase/internal/ai/embedding"
	"github.com/listen808/citebase/internal/ai/mcp"
	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/repository"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	schemarepo "github.com/listen808/citebase/internal/repository/schema"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/service/schema"
	searchsvc "github.com/listen808/citebase/internal/service/search"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
	"github.com/listen808/citebase/internal/text2sql"
	"github.com/listen808/citebase/internal/text2sql/trust"
	"github.com/listen808/citebase/internal/utils/config"
)

// LLMProviderAdapter 适配器，将 provider.Provider 转换为 text2sql.LLMProvider
type LLMProviderAdapter struct {
	primary   provider.Provider
	fast      provider.Provider
	reasoning provider.Provider
}

func NewLLMProviderAdapter(primary, fast, reasoning provider.Provider) *LLMProviderAdapter {
	return &LLMProviderAdapter{
		primary:   primary,
		fast:      fast,
		reasoning: reasoning,
	}
}

func (a *LLMProviderAdapter) selectProvider(tier text2sql.ProviderTier) provider.Provider {
	switch tier {
	case text2sql.TierFast:
		if a.fast != nil {
			return a.fast
		}
	case text2sql.TierReasoning:
		if a.reasoning != nil {
			return a.reasoning
		}
	}
	if a.primary != nil {
		return a.primary
	}
	if a.fast != nil {
		return a.fast
	}
	return a.reasoning
}

func (a *LLMProviderAdapter) Chat(ctx context.Context, systemPrompt string, messages []text2sql.Message) (*text2sql.ChatResponse, error) {
	providerMessages := make([]provider.Message, len(messages))
	for i, msg := range messages {
		providerMessages[i] = provider.NewTextMessage(msg.Role, msg.Content)
	}
	resp, err := a.selectProvider(text2sql.TierPrimary).Chat(ctx, systemPrompt, providerMessages)
	if err != nil {
		return nil, err
	}
	return &text2sql.ChatResponse{Content: resp.Content}, nil
}

func (a *LLMProviderAdapter) ChatWithOptions(ctx context.Context, systemPrompt string, messages []text2sql.Message, opts ...text2sql.CallOption) (*text2sql.ChatResponse, error) {
	providerMessages := make([]provider.Message, len(messages))
	for i, msg := range messages {
		providerMessages[i] = provider.NewTextMessage(msg.Role, msg.Content)
	}
	callOpts := &text2sql.CallOptions{}
	for _, opt := range opts {
		opt(callOpts)
	}
	var providerOpts []provider.Option
	if callOpts.ResponseFormat != nil {
		providerOpts = append(providerOpts, provider.WithResponseFormat(&provider.ResponseFormat{
			Type:   callOpts.ResponseFormat.Type,
			Schema: callOpts.ResponseFormat.Schema,
		}))
	}
	if callOpts.Temperature > 0 {
		providerOpts = append(providerOpts, provider.WithTemperature(callOpts.Temperature))
	}
	if callOpts.MaxTokens > 0 {
		providerOpts = append(providerOpts, provider.WithMaxTokens(callOpts.MaxTokens))
	}
	resp, err := a.selectProvider(callOpts.Tier).Chat(ctx, systemPrompt, providerMessages, providerOpts...)
	if err != nil {
		return nil, err
	}
	return &text2sql.ChatResponse{Content: resp.Content}, nil
}

func newExplainLLMCaller(p provider.Provider) trust.ExplainLLMCaller {
	return func(ctx context.Context, systemPrompt string, messages []trust.Message, schema string) (string, error) {
		providerMessages := make([]provider.Message, len(messages))
		for i, msg := range messages {
			providerMessages[i] = provider.NewTextMessage(msg.Role, msg.Content)
		}
		var schemaMap map[string]interface{}
		_ = json.Unmarshal([]byte(schema), &schemaMap)
		resp, err := p.Chat(ctx, systemPrompt, providerMessages, provider.WithResponseFormat(&provider.ResponseFormat{
			Type:   "json_object",
			Schema: schemaMap,
		}))
		if err != nil {
			return "", err
		}
		return resp.Content, nil
	}
}

// initSearchIndex 初始化全文搜索索引，按需自动重建
func initSearchIndex() {
	logx.Info("[Step 3/7] Initializing search index...")
	if err := searchsvc.InitIndex(config.AppConfig.Storage.IndexDir); err != nil {
		logx.Warnf("Failed to initialize search index: %v", err)
		return
	}
	logx.Info("[Step 3/7] Search index initialized successfully")

	stats, _ := searchsvc.GetIndexStats()
	indexedCount := parseDocCount(stats)
	isEmptyIndex := indexedCount == 0

	docRepo := workspacerepo.NewDocumentRepository()
	dbDocCount, _ := docRepo.CountAll()

	needsReindex := searchsvc.IndexRebuilt || isEmptyIndex
	if !needsReindex && dbDocCount > 10 && indexedCount > 0 && indexedCount < uint64(dbDocCount)/2 {
		logx.Infof("[Main] Index has %d documents but DB has %d, triggering incremental reindex...", indexedCount, dbDocCount)
		needsReindex = true
	} else if !needsReindex && dbDocCount > 0 && indexedCount == 0 {
		logx.Infof("[Main] Index is empty but DB has %d documents, triggering reindex...", dbDocCount)
		needsReindex = true
	} else if !needsReindex && indexedCount > 0 && searchsvc.IndexAppearsCorrupt() {
		// 索引看似正常（doc_count>0）但全量查询返回 0 → 段数据损坏，强制重建
		logx.Warnf("[Main] Index reports %d docs but full-text query returns 0, treating as corrupted, forcing reindex...", indexedCount)
		needsReindex = true
	}

	if needsReindex {
		logx.Info("[Main] Triggering auto reindex...")
		go func() {
			defer func() { _ = recover() }()
			svc := searchsvc.NewSearchService()
			count, err := svc.ReindexAllDocuments(nil)
			if err != nil {
				logx.Errorf("Auto reindex failed: %v", err)
			} else {
				logx.Infof("Auto reindex completed: %d documents indexed", count)
			}
		}()
	}
}

func parseDocCount(stats map[string]interface{}) uint64 {
	if stats == nil {
		return 0
	}
	switch v := stats["doc_count"].(type) {
	case uint64:
		return v
	case int64:
		return uint64(v)
	case int:
		return uint64(v)
	}
	return 0
}

// initVectorStore 初始化向量存储
func initVectorStore() {
	logx.Info("[Step 4/7] Initializing vector store...")
	if err := embedding.InitVectorStore("./data/vectors"); err != nil {
		logx.Warnf("Failed to initialize vector store: %v", err)
	} else {
		logx.Info("[Step 4/7] Vector store initialized successfully")
	}
}

// initAI 构建 AI 配置、初始化 AI 模块、注册全局 LLM provider 和 Token 配额
func initAI() *ai.Module {
	aiCfg := ai.BuildAIConfig(
		config.AppConfig.AI.Enabled,
		config.AppConfig.AI.Provider,
		config.AppConfig.AI.BaseURL,
		config.AppConfig.AI.APIKey,
		config.AppConfig.AI.Model,
		config.AppConfig.AI.MaxTokens,
		config.AppConfig.AI.Temperature,
		config.AppConfig.AI.EmbeddingURL,
		config.AppConfig.AI.EmbeddingKey,
		config.AppConfig.AI.EmbeddingModel,
	)
	if cfg := config.AppConfig.AI.Primary; cfg != nil {
		aiCfg.Primary = cfg
	}
	if cfg := config.AppConfig.AI.Fast; cfg != nil {
		aiCfg.Fast = cfg
	}
	if cfg := config.AppConfig.AI.Reasoning; cfg != nil {
		aiCfg.Reasoning = cfg
	}
	if cfg := config.AppConfig.AI.Embedding; cfg != nil {
		aiCfg.Embedding = cfg
	}
	aiCfg.MCP = buildMCPConfig()

	ai.InitModule(aiCfg)

	aiModule := ai.GetModule()
	if aiModule != nil && aiModule.PrimaryProvider() != nil {
		llmAdapter := NewLLMProviderAdapter(
			aiModule.PrimaryProvider(),
			aiModule.FastProvider(),
			aiModule.ReasoningProvider(),
		)
		text2sql.SetGlobalLLMProvider(llmAdapter)
		// P0-11: 注入真实 provider 以启用 Agent 流式输出
		text2sql.SetGlobalRealProvider(aiModule.PrimaryProvider())
	}

	tokenUsageRepo := airepo.NewTokenUsageRepository()
	provider.InitTokenQuota(config.AppConfig.AI.DailyTokenLimit, config.AppConfig.AI.PerUserTokenLimit, tokenUsageRepo)

	return aiModule
}

func buildMCPConfig() *ai.AIMCPConfig {
	if config.AppConfig.AI.MCP != nil && config.AppConfig.AI.MCP.Enabled {
		cfg := &ai.AIMCPConfig{Enabled: true}
		for _, s := range config.AppConfig.AI.MCP.Servers {
			cfg.Servers = append(cfg.Servers, mcp.ServerConfig{
				Name: s.Name, Transport: s.Transport, Command: s.Command,
				Args: s.Args, Env: s.Env, Endpoint: s.Endpoint,
			})
		}
		return cfg
	}
	if len(config.AppConfig.AI.MCPServers) > 0 {
		cfg := &ai.AIMCPConfig{Enabled: true}
		for _, s := range config.AppConfig.AI.MCPServers {
			cfg.Servers = append(cfg.Servers, mcp.ServerConfig{
				Name: s.Name, Transport: s.Transport, Command: s.Command,
				Args: s.Args, Env: s.Env, Endpoint: s.Endpoint,
			})
		}
		return cfg
	}
	return nil
}

// initText2SQL 初始化 Schema Intelligence + Text2SQL 全部依赖
func initText2SQL(aiModule *ai.Module) *schema.SchemaService {
	schemaService := schema.NewSchemaService(aiModule.GetSchemaVectorStore())

	textToSQLRepo := airepo.NewTextToSQLRepo(repository.DB)
	if err := schemaService.SetTextToSQLDependencies(textToSQLRepo); err != nil {
		logx.Warnf("Failed to init TextToSQL Dependencies: %v", err)
	}
	schemaService.SetTextToSQLPromptTemplateRepo(airepo.NewPromptTemplateRepo(repository.DB))
	schemaService.SetTextToSQLFeedbackRepo(airepo.NewText2SQLFeedbackRepo(repository.DB))
	schemaService.SetTextToSQLTraceRepo(airepo.NewSQLTraceRepo(repository.DB))

	semanticMetricRepo := airepo.NewSemanticMetricRepo(repository.DB)
	semanticDimensionRepo := airepo.NewSemanticDimensionRepo(repository.DB)
	columnStatsRepo := airepo.NewColumnStatsRepo(repository.DB)
	schemaService.SetTextToSQLSemanticRepos(semanticMetricRepo, semanticDimensionRepo, columnStatsRepo)
	schemaService.SetTextToSQLPromptVersionRepo(airepo.NewPromptVersionRepo(repository.DB))

	if workspacesvc.VectorSyncFunc != nil {
		schemaService.SetVectorSyncCallback(workspacesvc.VectorSyncFunc)
	}

	if aiModule != nil && aiModule.Embedder() != nil {
		sqlRAGAdapter := text2sql.NewSQLRAGAdapter(aiModule.Embedder(), aiModule.GetSchemaVectorStore(), textToSQLRepo)
		schemaService.SetTextToSQLRAGAdapter(sqlRAGAdapter)
	}

	dataSourceRepo := airepo.NewDataSourceRepo(repository.DB)
	sqlExecutor := text2sql.NewSQLExecutor(dataSourceRepo)
	maskingEngine := text2sql.NewMaskingEngine(airepo.NewDataSensitivityRepo(repository.DB))
	sqlExecutor.SetMaskingEngine(maskingEngine)
	policyEngine := trust.NewPolicyEngine(airepo.NewSQLSecurityPolicyRepo(repository.DB))
	sqlExecutor.SetPolicyEngine(policyEngine)
	// P0-10: 接入行级安全过滤器
	rls := trust.NewRowLevelSecurity(workspacerepo.NewSpaceRepository(), trust.DefaultRLSConfig())
	sqlExecutor.SetRLS(rls)

	var explainer *trust.SQLExplainer
	if aiModule != nil && aiModule.PrimaryProvider() != nil {
		explainer = trust.NewSQLExplainer(newExplainLLMCaller(aiModule.PrimaryProvider()))
	}
	auditor := trust.NewSQLAuditRecorder(airepo.NewAIAuditLogRepository())
	textToSQLGenerator := schemaService.GetTextToSQLGenerator()
	if textToSQLGenerator != nil {
		textToSQLGenerator.SetExplainer(explainer)
		textToSQLGenerator.SetAuditor(auditor)
		textToSQLGenerator.SetSchemaRepo(schemarepo.NewSchemaRepository())
	}

	schemaService.SetTextToSQLExecutor(sqlExecutor)
	schemaService.SetSchemaStatsCollector(columnStatsRepo, dataSourceRepo)

	if aiModule != nil && aiModule.GetSchemaVectorStore() != nil {
		schemaLinker := text2sql.NewSchemaLinker(aiModule.GetSchemaVectorStore(), schemarepo.NewSchemaRepository())
		schemaService.SetSchemaLinker(schemaLinker)
	}

	logx.Info("[Main] Text2SQL fully initialized")
	return schemaService
}
