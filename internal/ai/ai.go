package ai

import (
	"context"
	"database/sql"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/ai/analyzer"
	"github.com/listen808/citebase/internal/ai/embedding"
	"github.com/listen808/citebase/internal/ai/mcp"
	"github.com/listen808/citebase/internal/ai/observability"
	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/ai/rag"
	"github.com/listen808/citebase/internal/ai/rag/compression"
	"github.com/listen808/citebase/internal/ai/rag/multiagent"
	"github.com/listen808/citebase/internal/ai/router"
	"github.com/listen808/citebase/internal/ai/scheduler"
	"github.com/listen808/citebase/internal/ai/wiki"
	"github.com/listen808/citebase/internal/repository"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	knowledgerepo "github.com/listen808/citebase/internal/repository/knowledge"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	adminsvc "github.com/listen808/citebase/internal/service/admin"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
	"github.com/listen808/citebase/internal/text2sql"
	"github.com/listen808/citebase/internal/utils/config"
)

type Module struct {
	primaryProvider       provider.Provider
	fastProvider          provider.Provider
	reasoningProvider     provider.Provider
	embedder              embedding.Embedder
	retriever             *rag.Retriever
	summarizer            *rag.Summarizer
	sched                 *scheduler.Scheduler
	vectorSyncer          *VectorSyncer
	cronScheduler         *CronScheduler
	autoEvaluator         *rag.AutoEvaluator
	enabled               bool
	analyzerCfg           *analyzer.AnalyzerConfig
	aiSpaceName           string
	graphEnhancer         *analyzer.GraphEnhancer
	contradictionDetector *analyzer.ContradictionDetector
	adaptiveRetriever     *rag.AdaptiveRetriever
	tableGenerator        *rag.TableGenerator
	reportGenerator       *rag.ReportGenerator
	emailGenerator        *rag.EmailGenerator
	translator            *rag.Translator
	recommender           *rag.Recommender
	qualityPatrol         *rag.QualityPatrol
	mcpClient             *mcp.Client
	schemaVectorStore     *embedding.SchemaVectorStore
	// RAG增强组件
	selfRAG               *rag.SelfRAG
	hydeRetriever         *rag.HyDERetriever
	multiHopReasoner      *rag.MultiHopReasoner
	hallucinationDetector *rag.HallucinationDetector
	memoryManager         *rag.MemoryManager
	correctiveRAG         *rag.CorrectiveRAG
	// P0-长上下文组件
	realtimeDataManager *rag.RealtimeDataManager
	// P1-7 多模型智能路由器
	modelRouter *router.Router
	// 邮件服务
	emailService *adminsvc.EmailService
}

var module *Module

func InitModule(cfg *AIConfig) {
	if !cfg.Enabled {
		module = &Module{enabled: false}
		logx.Info("[AI] Module disabled")
		return
	}

	// 启动时配置检查
	warnings := ValidateAIConfig(cfg)
	for _, w := range warnings {
		logx.Infof("[AI] WARNING: %s", w)
	}

	var primary provider.Provider
	var err error
	if cfg.Primary != nil && cfg.Primary.APIKey != "" {
		primary, err = provider.NewProviderFromConfig(
			cfg.Primary.Provider, cfg.Primary.BaseURL, cfg.Primary.APIKey,
			cfg.Primary.Model, cfg.Primary.MaxTokens, cfg.Primary.Temperature,
		)
		if err != nil {
			logx.Errorf("[AI] Failed to create primary provider: %v", err)
		}
	} else if cfg.Primary != nil && cfg.Primary.APIKey == "" {
		logx.Info("[AI] WARNING: primary provider configured but api_key is empty")
	}

	if primary == nil && cfg.Legacy != nil && cfg.Legacy.APIKey != "" {
		primary, err = provider.NewProviderFromConfig(
			cfg.Legacy.Provider, cfg.Legacy.BaseURL, cfg.Legacy.APIKey,
			cfg.Legacy.Model, cfg.Legacy.MaxTokens, cfg.Legacy.Temperature,
		)
		if err != nil {
			logx.Errorf("[AI] Failed to create legacy provider: %v", err)
		}
	}

	var fast provider.Provider
	if cfg.Fast != nil && cfg.Fast.APIKey != "" {
		fast, err = provider.NewProviderFromConfig(
			cfg.Fast.Provider, cfg.Fast.BaseURL, cfg.Fast.APIKey,
			cfg.Fast.Model, cfg.Fast.MaxTokens, cfg.Fast.Temperature,
		)
		if err != nil {
			logx.Errorf("[AI] Failed to create fast provider: %v", err)
		}
	} else if cfg.Fast != nil && cfg.Fast.APIKey == "" {
		logx.Info("[AI] WARNING: fast provider configured but api_key is empty, will fall back to primary")
	}
	if fast == nil {
		fast = primary
	}

	var reasoning provider.Provider
	if cfg.Reasoning != nil && cfg.Reasoning.APIKey != "" {
		reasoning, err = provider.NewProviderFromConfig(
			cfg.Reasoning.Provider, cfg.Reasoning.BaseURL, cfg.Reasoning.APIKey,
			cfg.Reasoning.Model, cfg.Reasoning.MaxTokens, cfg.Reasoning.Temperature,
		)
		if err != nil {
			logx.Errorf("[AI] Failed to create reasoning provider: %v", err)
		}
	} else if cfg.Reasoning != nil && cfg.Reasoning.APIKey == "" {
		logx.Info("[AI] WARNING: reasoning provider configured but api_key is empty, will fall back to primary")
	}
	if reasoning == nil {
		reasoning = primary
	}

	// 为三级 Provider 装配熔断器保护（连接池 + 超时控制）
	if primary != nil {
		primary = provider.NewPooledProvider(primary, 5, 30*time.Second)
	}
	if fast != nil {
		fast = provider.NewPooledProvider(fast, 5, 30*time.Second)
	}
	if reasoning != nil {
		reasoning = provider.NewPooledProvider(reasoning, 5, 30*time.Second)
	}

	var emb embedding.Embedder
	var embIdentity string // P0 增量缓存：embedding 栈身份（provider:model），随指纹落库

	// P6：OTLP 导出（配置开关，默认关）
	observability.InitOTLPExporter(config.AppConfig.AI.OtlpEndpoint)
	if cfg.Embedding != nil && (cfg.Embedding.APIKey != "" || cfg.Embedding.BaseURL != "") {
		embURL := cfg.Embedding.BaseURL
		embKey := cfg.Embedding.APIKey
		embModel := cfg.Embedding.Model
		embProvider := cfg.Embedding.Provider

		if embURL == "" && cfg.Legacy != nil {
			embURL = cfg.Legacy.BaseURL
		}
		if embKey == "" && cfg.Legacy != nil {
			embKey = cfg.Legacy.APIKey
		}
		if embModel == "" {
			embModel = "text-embedding-3-small"
		}
		if embProvider == "" {
			embProvider = "openai"
		}

		embProv, err := provider.NewProviderFromConfig(embProvider, embURL, embKey, embModel, 0, 0)
		if err != nil {
			logx.Errorf("[AI] Failed to create embedding provider: %v", err)
		} else {
			// embedding 也装配熔断器保护，与 primary/fast/reasoning 保持一致
			embProv = provider.NewPooledProvider(embProv, 5, 30*time.Second)
			emb = embedding.NewEmbedder(embProv)
			embIdentity = embProvider + ":" + embModel
		}
	} else if cfg.Embedding != nil && cfg.Embedding.APIKey == "" && cfg.Embedding.BaseURL == "" {
		logx.Info("[AI] WARNING: embedding section configured but api_key and base_url are both empty")
	}

	if cfg.Enabled && emb == nil {
		logx.Info("[AI] WARNING: AI is enabled but embedding is not configured.")
		logx.Info("[AI] Vector search will be DISABLED. RAG will fall back to keyword-only search.")
		logx.Info("[AI] To enable vector search, configure ai.embedding section in config.yaml.")
	}

	var schemaVectorStore *embedding.SchemaVectorStore
	if emb != nil {
		schemaVectorStore = embedding.NewSchemaVectorStore("./data/vectors", emb)
		if schemaVectorStore != nil {
			logx.Info("[AI] Schema vector store initialized (independent BoltDB+HNSW)")
		}
	}

	var retriever *rag.Retriever
	var summarizer *rag.Summarizer
	if primary != nil {
		retriever = rag.NewRetrieverWithFastProvider(primary, fast, emb)
		summarizer = rag.NewSummarizer(primary)
		// P4：wiki 图谱 2-hop 扩展进主问答链路（配置开关，默认关）
		if config.AppConfig.AI.GraphExpansion && fast != nil {
			retriever.SetGraphExpander(&wikiGraphExpanderAdapter{c: wiki.NewCompiler(fast)})
			logx.Info("[AI] Wiki graph expansion enabled for main RAG chain")
		}
	}

	// S4: 初始化查询语义缓存
	if emb != nil {
		rag.InitQueryCache(emb, 500, 24*time.Hour)
		logx.Info("[AI] Query semantic cache initialized (max=500, ttl=24h)")
	}

	// S7: 初始化Prompt模板管理器
	rag.InitPromptTemplateManager("")
	logx.Info("[AI] Prompt template manager initialized")

	// 初始化表格生成器和汇报生成器
	var tableGen *rag.TableGenerator
	var reportGen *rag.ReportGenerator
	var emailGen *rag.EmailGenerator
	var translator *rag.Translator
	var recommender *rag.Recommender
	var qualityPatrol *rag.QualityPatrol
	if primary != nil {
		tableGen = rag.NewTableGenerator(primary)
		reportGen = rag.NewReportGenerator(primary)
		emailGen = rag.NewEmailGenerator(primary)
		translator = rag.NewTranslator(primary)
		recommender = rag.NewRecommender(primary)
		qualityPatrol = rag.NewQualityPatrol(primary)
	}

	// M4: 初始化工作流引擎
	if primary != nil {
		rag.InitWorkflowEngine(primary)
		rag.GetWorkflowEngine().LoadBuiltinWorkflows()
	}

	// E3: 初始化RAGAS评估器
	if primary != nil {
		rag.InitRAGASEvaluator(primary, knowledgerepo.NewAiRagasEvalRepository())
	}

	// 初始化邮件服务
	emailSvc := adminsvc.NewEmailService()

	module = &Module{
		primaryProvider:   primary,
		fastProvider:      fast,
		reasoningProvider: reasoning,
		embedder:          emb,
		retriever:         retriever,
		summarizer:        summarizer,
		enabled:           true,
		modelRouter:       router.NewRouter(primary, fast, reasoning),
		analyzerCfg:       cfg.Analyzer,
		aiSpaceName:       cfg.AISpaceName,
		tableGenerator:    tableGen,
		reportGenerator:   reportGen,
		emailGenerator:    emailGen,
		schemaVectorStore: schemaVectorStore,
		translator:        translator,
		recommender:       recommender,
		qualityPatrol:     qualityPatrol,
		emailService:      emailSvc,
	}

	if primary != nil {
		// 初始化新增强组件
		module.graphEnhancer = analyzer.NewGraphEnhancer(fast)
		module.contradictionDetector = analyzer.NewContradictionDetector(fast)
		module.adaptiveRetriever = rag.NewAdaptiveRetriever(fast)

		// 初始化RAG增强组件
		if retriever != nil {
			module.selfRAG = rag.NewSelfRAG(primary, retriever)
			module.multiHopReasoner = rag.NewMultiHopReasoner(primary, retriever)
			// C1：将查询类型路由器注入主检索器，AskWithDebug 自动按类型调整检索深度
			if module.adaptiveRetriever != nil {
				retriever.SetAdaptiveRetriever(module.adaptiveRetriever)
			}
			logx.Info("[AI] Self-RAG and Multi-hop Reasoner initialized")
		}
		if emb != nil {
			module.hydeRetriever = rag.NewHyDERetriever(primary, emb)
			logx.Info("[AI] HyDE Retriever initialized")
			// A1：句级语义压缩器，注入检索器主链路
			if retriever != nil {
				retriever.SetContextualCompressor(compression.NewContextualCompressor(emb))
			}
		}
		// 初始化CorrectiveRAG（检索失败自纠正）
		if retriever != nil && module.hydeRetriever != nil {
			module.correctiveRAG = rag.NewCorrectiveRAG(primary, retriever, module.hydeRetriever)
			logx.Info("[AI] CorrectiveRAG initialized")
		}
		module.hallucinationDetector = rag.NewHallucinationDetector(primary)
		logx.Info("[AI] Hallucination Detector initialized")

		// 初始化 MemoryManager（跨会话学习、查询模式追踪）
		module.memoryManager = rag.NewMemoryManager(airepo.NewAIMemoryRepository(), primary)
		logx.Info("[AI] Memory Manager initialized")

		// 将带持久化仓库的 MemoryManager 注入检索器，使主链路可召回个性化记忆。
		if retriever != nil {
			retriever.SetMemoryManager(module.memoryManager)
			// P1-3：文档级 ACL。默认无记录 = 沿用空间成员权限，只有显式 deny 才拦，
			// 所以老库零迁移成本，也不会出现"配了 ACL 反而把文档全藏了"。
			retriever.SetDocumentACL(func(userID int, docIDs []int) (map[int]bool, error) {
				return workspacerepo.NewDocumentPermissionRepository().FilterDenied(userID, nil, docIDs)
			})
		}

		// P0-初始化实时数据管理器
		module.realtimeDataManager = rag.NewRealtimeDataManager()
		logx.Info("[AI] Realtime Data Manager initialized")

		// P0-10: RLS 已迁至 ai_init.go 中接入 SQLExecutor
		// 此处不再创建占位实例，避免无意义的 DB 查询

		spaceName := cfg.AISpaceName
		if spaceName == "" {
			spaceName = "AI 知识库"
		}
		module.sched = scheduler.NewScheduler(primary, fast, reasoning, spaceName, cfg.SystemUserID, cfg.Analyzer)

		vs := embedding.GetVectorStoreInterface()
		if vs != nil && emb != nil {
			module.vectorSyncer = NewVectorSyncer(vs, emb, embIdentity)
			module.vectorSyncer.Start()
		}

		if cfg.Schedule != "" || module.sched != nil {
			schedule := cfg.Schedule
			if schedule == "" {
				schedule = "0 2 * * *"
			}
			module.cronScheduler = NewCronScheduler(module.sched, schedule)
			// P2-2: 注入配置的定时分析范围
			if cfg.ScheduleTaskType != "" || len(cfg.ScheduleSpaceIDs) > 0 {
				module.cronScheduler.SetScheduleScope(cfg.ScheduleTaskType, cfg.ScheduleSpaceIDs)
			}
			// P0-3: 装配自动化评测。AutoEvaluator 此前写了却从未实例化，
			// 评测只能靠人工点按钮，等于没有回归基线。
			setupAutoEval(module, retriever, primary)
			_ = module.cronScheduler.Start()
		}
	}

	// Git 同步定时调度已随割接 P3 下线（PullFromGit 写 data/markdowns 与正文落库冲突）

	logx.Infof("[AI] Module initialized (primary=%s, fast=%s, reasoning=%s, embedding=%v)",
		providerName(primary), providerName(fast), providerName(reasoning), emb != nil)

	// 注册 Agent 工具到全局注册中心
	if primary != nil {
		toolRegistry := rag.GetToolRegistry()
		rag.RegisterBuiltinTools(toolRegistry, emailSvc)
		// 需要外部依赖的工具单独注册，且受同一套开关约束：
		// query_database 直连业务库执行模型自撰 SQL，默认不注册。
		if config.AppConfig.AI.AgentTools.EnableDatabaseQuery {
			var dbConn *sql.DB
			if repository.DB != nil {
				dbConn, _ = repository.DB.DB()
			}
			toolRegistry.Register("query_database", rag.NewDatabaseQueryTool(dbConn))
			logx.Warn("[AI] query_database 已启用：请为业务库配置只读账号并收紧行数上限")
		}
	}

	// 初始化 MCP 客户端
	if cfg.MCP != nil && cfg.MCP.Enabled && len(cfg.MCP.Servers) > 0 {
		mcpClient := mcp.NewClient()
		ctx := context.Background()
		for _, serverCfg := range cfg.MCP.Servers {
			if err := mcpClient.AddServer(ctx, serverCfg); err != nil {
				logx.Infof("[AI] Failed to add MCP server %q: %v", serverCfg.Name, err)
			}
		}
		module.mcpClient = mcpClient

		// 注册 MCP 工具到全局工具注册中心
		toolRegistry := rag.GetToolRegistry()
		mcpTools := mcpClient.GetAllTools()
		for _, tool := range mcpTools {
			fullName := tool.Name
			toolRegistry.Register(fullName, rag.NewMCPToolAdapter(mcpClient, fullName))
		}
		if len(mcpTools) > 0 {
			logx.Infof("[AI] MCP tools registered: %d tools from %d servers", len(mcpTools), len(cfg.MCP.Servers))
		}
	}

	// S1: 注册向量同步回调，避免service→ai循环导入
	if module.vectorSyncer != nil {
		vs := module.vectorSyncer
		workspacesvc.VectorSyncFunc = func(docID, spaceID int, docName, content, opType string) {
			vs.Enqueue(VectorSyncOp{
				DocID:   docID,
				SpaceID: spaceID,
				DocName: docName,
				Content: content,
				OpType:  opType,
			})
		}
		// P0：重建索引入口走强制通道，绕过指纹判重——
		// 管理员点"重建"就必须真的重建，即使内容指纹没变（索引可能已损坏）。
		workspacesvc.VectorSyncReindexFunc = func(docID, spaceID int, docName, content, opType string) {
			vs.EnqueueForce(VectorSyncOp{
				DocID:   docID,
				SpaceID: spaceID,
				DocName: docName,
				Content: content,
				OpType:  opType,
			})
		}
	}

	// P1 #3: 注册 Wiki 重编译回调 —— 源文档内容变更后自动重编译受影响的 wiki 页面
	if primary != nil {
		workspacesvc.WikiRecompileFunc = func(docID, spaceID int) {
			module.recompileWikiDoc(docID, spaceID, primary)
		}
		// 源文档下架级联：入回收站/彻底删除时摘除该文档贡献的知识（独占页删除、
		// 共享页摘溯源、死链清理），否则删掉文档后知识页仍对查询可见。
		workspacesvc.WikiRemoveSourceFunc = func(spaceID, docID int) {
			module.removeWikiSource(spaceID, docID)
		}
		logx.Info("[AI] Wiki recompile callback registered (immutability cascade)")
	}

	// S19: 注册AI审计日志回调
	provider.AuditLogFunc = func(providerName, action string, tokenUsage int, err error) {
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		logx.Infof("[AI-Audit] provider=%s action=%s tokens=%d err=%s", providerName, action, tokenUsage, errStr)
	}

	// 注册Text2SQL的RAG检索器适配器
	if retriever != nil {
		text2sql.SetRAGRetriever(&ragRetrieverAdapter{retriever: retriever})
		logx.Info("[AI] Text2SQL RAG retriever adapter registered")
	}

	// P2-5: 注入多模型路由器到 Text2SQL（复杂查询走 reasoning tier）
	if module.modelRouter != nil {
		text2sql.SetModelRouterAdapter(&text2sqlRouterAdapter{router: module.modelRouter})
		logx.Info("[AI] Text2SQL model router adapter registered (P2-5 reasoning routing)")
	}

	// 初始化 Text2SQL 独立配置段
	text2sql.InitText2SQLConfig(config.AppConfig.Text2SQL)
	logx.Info("[AI] Text2SQL config initialized")

	// 异步预热查询缓存（从审计日志加载高频查询）
	go func() {
		cache := rag.GetQueryCache()
		if cache == nil {
			return
		}
		auditRepo := airepo.NewAIAuditLogRepository()
		recentLogs, _, err := auditRepo.FindByAction("ask", 20, 0)
		if err != nil || len(recentLogs) == 0 {
			return
		}

		var entries []rag.PreloadEntry
		seen := make(map[string]bool)
		for _, al := range recentLogs {
			if al.Question == "" || al.Answer == "" {
				continue
			}
			key := fmt.Sprintf("%d:%d:%s", 0, 0, al.Question)
			if seen[key] {
				continue
			}
			seen[key] = true
			entries = append(entries, rag.PreloadEntry{
				Question: al.Question,
				SpaceID:  0,
				UserID:   0,
				Answer: &rag.AIAnswer{
					Answer: al.Answer,
				},
			})
		}

		if len(entries) > 0 {
			cache.Preload(entries)
			logx.Infof("[AI] Query cache preheated with %d entries", len(entries))
		}
	}()
}

type ragRetrieverAdapter struct {
	retriever *rag.Retriever
}

func (a *ragRetrieverAdapter) RetrieveContextWithRAG(question string, spaceID int, limit int) ([]text2sql.RAGSource, string) {
	ctx := context.Background()
	sources, contextStr := a.retriever.RetrieveContextWithRAG(ctx, question, spaceID, limit)
	var result []text2sql.RAGSource
	for _, s := range sources {
		result = append(result, text2sql.RAGSource{
			DocumentID:   s.DocumentID,
			DocumentName: s.DocumentName,
			SpaceID:      s.SpaceID,
			Snippet:      s.Snippet,
		})
	}
	return result, contextStr
}

// text2sqlRouterAdapter P2-5: 将 router.Router 适配为 text2sql.ModelRouterAdapter
// 避免在 text2sql 包中直接引用 router 包（防止循环依赖）
type text2sqlRouterAdapter struct {
	router *router.Router
}

// providerToLLMAdapter 将单个 provider.Provider 包装为 text2sql.LLMProvider
// 复用 ai_init.go 中 LLMProviderAdapter 的消息转换逻辑（简化版，固定使用该 provider）
type providerToLLMAdapter struct {
	p provider.Provider
}

func (a *providerToLLMAdapter) Chat(ctx context.Context, systemPrompt string, messages []text2sql.Message) (*text2sql.ChatResponse, error) {
	providerMessages := make([]provider.Message, len(messages))
	for i, msg := range messages {
		providerMessages[i] = provider.NewTextMessage(msg.Role, msg.Content)
	}
	resp, err := a.p.Chat(ctx, systemPrompt, providerMessages)
	if err != nil {
		return nil, err
	}
	return &text2sql.ChatResponse{Content: resp.Content}, nil
}

func (a *providerToLLMAdapter) ChatWithOptions(ctx context.Context, systemPrompt string, messages []text2sql.Message, opts ...text2sql.CallOption) (*text2sql.ChatResponse, error) {
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
	resp, err := a.p.Chat(ctx, systemPrompt, providerMessages, providerOpts...)
	if err != nil {
		return nil, err
	}
	return &text2sql.ChatResponse{Content: resp.Content}, nil
}

// SelectByContextLength 实现 text2sql.ModelRouterAdapter
func (a *text2sqlRouterAdapter) SelectByContextLength(tokenCount int) (text2sql.LLMProvider, []text2sql.LLMProvider) {
	p, fallbacks := a.router.SelectByContextLength(tokenCount)
	if p == nil {
		return nil, nil
	}
	primary := &providerToLLMAdapter{p: p}
	var fb []text2sql.LLMProvider
	for _, f := range fallbacks {
		fb = append(fb, &providerToLLMAdapter{p: f})
	}
	return primary, fb
}

// SelectReasoning 选择推理模型
func (a *text2sqlRouterAdapter) SelectReasoning() (text2sql.LLMProvider, []text2sql.LLMProvider) {
	p, fallbacks := a.router.Select(context.Background(), "", router.WithForceTier("reasoning"))
	if p == nil {
		return nil, nil
	}
	primary := &providerToLLMAdapter{p: p}
	var fb []text2sql.LLMProvider
	for _, f := range fallbacks {
		fb = append(fb, &providerToLLMAdapter{p: f})
	}
	return primary, fb
}

func providerName(p provider.Provider) string {
	if p == nil {
		return "none"
	}
	return p.Name()
}

func GetModule() *Module {
	if module == nil {
		return &Module{enabled: false}
	}
	return module
}

// Shutdown 停掉模块持有的常驻调度器。
//
// 加这条之前 CronScheduler.Stop() 是**零调用方**：
// 进程收到 SIGTERM 后 HTTP 会优雅关停、队列会排空，但 robfig/cron 的调度器
// 一路照跑到进程被系统收走 —— 于是关停瞬间仍可能启一次分析，
// 那类任务带着一个正在关闭的数据库连接去跑，留下的就是"关停时报一堆错"的日志，
// 严重时是半途而废的分析任务把空间状态停在中间态。
// 字段可能没建（AI 未启用、cron 表达式非法），所以判空。
// （GitSyncScheduler 已随割接 P3 下线删除。）
func (m *Module) Shutdown() {
	if m == nil {
		return
	}
	if m.cronScheduler != nil {
		m.cronScheduler.Stop()
	}
}

// recompileWikiDoc P1 #3: 源文档内容变更后，重新分析并编译该文档到 wiki。
// 在独立 goroutine 中调用（由 WikiRecompileFunc 包装），失败只记日志。
func (m *Module) recompileWikiDoc(docID, spaceID int, p provider.Provider) {
	defer func() {
		if r := recover(); r != nil {
			logx.Warnf("[WikiRecompile] panic for doc %d: %v", docID, r)
		}
	}()

	docs, err := analyzer.NewDocReader(0).ReadDocumentsByIDs([]int{docID})
	if err != nil || len(docs) == 0 || docs[0] == nil {
		logx.Infof("[WikiRecompile] cannot read doc %d: %v", docID, err)
		return
	}
	d := docs[0]
	if d.Content == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	candidates := wiki.AnalyzeDocumentToCandidates(ctx, p, d.DocID, d.DocName, d.Content)
	if len(candidates) == 0 {
		return
	}

	compiler := wiki.NewCompiler(p)
	plan, err := compiler.Plan(ctx, spaceID, candidates)
	if err != nil {
		logx.Infof("[WikiRecompile] plan failed for doc %d space %d: %v", docID, spaceID, err)
		return
	}
	cost, err := compiler.Generate(ctx, spaceID, candidates, 2)
	if err != nil {
		logx.Infof("[WikiRecompile] generate failed for doc %d space %d: %v", docID, spaceID, err)
		return
	}

	var logLines []string
	for _, slug := range plan.Created {
		logLines = append(logLines, "重编译新建：`"+slug+"`")
	}
	for _, slug := range plan.Updated {
		logLines = append(logLines, "重编译更新：`"+slug+"`（源文档变更）")
	}
	if cost != nil && cost.TotalTokens > 0 {
		logLines = append(logLines, fmt.Sprintf("Token 消耗：prompt %d / completion %d / 共 %d",
			cost.PromptTokens, cost.CompletionTokens, cost.TotalTokens))
	}
	compiler.AppendCompileLog(spaceID, logLines)
	_ = compiler.RefreshOverview(ctx, spaceID)

	// 置信度治理
	for _, slug := range append(append([]string{}, plan.Created...), plan.Updated...) {
		compiler.CheckConfidence(ctx, spaceID, slug)
	}

	logx.Infof("[WikiRecompile] doc %d space %d: created=%d updated=%d skipped=%d",
		docID, spaceID, len(plan.Created), len(plan.Updated), plan.Skipped)
}

// removeWikiSource P1: 源文档下架后级联清理 wiki（独占页删除 / 共享页摘溯源 / 死链清理）。
// 在独立 goroutine 中调用，失败只记日志 —— 文档删除是主流程，不能因此回滚。
func (m *Module) removeWikiSource(spaceID, docID int) {
	defer func() {
		if r := recover(); r != nil {
			logx.Warnf("[WikiCascade] panic for doc %d: %v", docID, r)
		}
	}()

	compiler := wiki.NewCompiler(m.primaryProvider)
	res, err := compiler.RemoveSource(spaceID, docID)
	if err != nil {
		logx.Warnf("[WikiCascade] remove source failed for doc %d space %d: %v", docID, spaceID, err)
		return
	}
	if len(res.RemovedPages) == 0 && len(res.PrunedPages) == 0 && res.CleanedLinks == 0 {
		return
	}
	logx.Infof("[WikiCascade] doc %d space %d: removed=%d pruned=%d links=%d",
		docID, spaceID, len(res.RemovedPages), len(res.PrunedPages), res.CleanedLinks)
}

func (m *Module) Enabled() bool {
	return m.enabled
}

func (m *Module) PrimaryProvider() provider.Provider {
	return m.primaryProvider
}

func (m *Module) FastProvider() provider.Provider {
	return m.fastProvider
}

func (m *Module) ReasoningProvider() provider.Provider {
	return m.reasoningProvider
}

// ModelRouter 返回多模型智能路由器（P1-7）。
func (m *Module) ModelRouter() *router.Router {
	return m.modelRouter
}

// NewOrchestrator 构建多智能体编排器（P0-2/P0-3），使用智能路由选择模型，
// 并以 workspace 服务做空间权限检查。返回 nil 表示 AI 未启用。
func (m *Module) NewOrchestrator() *multiagent.Orchestrator {
	if m.primaryProvider == nil || m.retriever == nil {
		return nil
	}
	// 推理型任务优先使用 reasoning provider，否则回退 primary。
	reasoning := m.reasoningProvider
	if reasoning == nil {
		reasoning = m.primaryProvider
	}
	spaceSvc := workspacesvc.NewSpaceService()
	return multiagent.NewOrchestrator(m.modelRouter, reasoning, m.retriever, spaceSvc, multiagent.DefaultConfig())
}

func (m *Module) Embedder() embedding.Embedder {
	return m.embedder
}

func (m *Module) Retriever() *rag.Retriever {
	return m.retriever
}

func (m *Module) Summarizer() *rag.Summarizer {
	return m.summarizer
}

func (m *Module) VectorStore() embedding.VectorStoreInterface {
	return embedding.GetVectorStoreInterface()
}

func (m *Module) SafetyGuard() *rag.SafetyGuard {
	if m.retriever != nil {
		return m.retriever.SafetyGuard()
	}
	return nil
}

func (m *Module) Scheduler() *scheduler.Scheduler {
	return m.sched
}

func (m *Module) Analyzer() *analyzer.Analyzer {
	if m.sched == nil {
		return nil
	}
	return m.sched.GetAnalyzer()
}

func (m *Module) VectorSyncer() *VectorSyncer {
	return m.vectorSyncer
}

func (m *Module) CronScheduler() *CronScheduler {
	return m.cronScheduler
}

// defaultEvalSchedule 每天 03:00 跑自动评测（避开 02:00 的知识分析）
const defaultEvalSchedule = "0 3 * * *"

// setupAutoEval P0-3：把自动化评测接上定时调度。
// AutoEvaluator 此前写完了却从未实例化，评测只能靠人工点一次按钮，等于没有回归基线。
func setupAutoEval(module *Module, retriever *rag.Retriever, primary provider.Provider) {
	if module == nil || module.cronScheduler == nil || retriever == nil || primary == nil {
		return
	}
	eval := rag.GetRAGASEvaluator()
	if eval == nil {
		return
	}
	module.autoEvaluator = rag.NewAutoEvaluator(primary, retriever, eval, rag.DefaultAutoEvalConfig())
	module.cronScheduler.SetEvalJob(defaultEvalSchedule, module.runAutoEval)
	logx.Infof("[AI] Auto evaluator scheduled: %s", defaultEvalSchedule)
}

// runAutoEval 执行一次自动评测。
// 内置基线问题测的是"检索链路整体是否可用"，不是业务准确率；
// 要测业务准确率，得往 /ai/eval/datasets 灌真实问题集。
func (m *Module) runAutoEval(ctx context.Context) {
	if m == nil || m.autoEvaluator == nil {
		return
	}
	result, err := m.autoEvaluator.RunEvaluation(ctx, rag.DefaultTestQueries())
	if err != nil {
		logx.Warnf("[AutoEval] scheduled evaluation failed: %v", err)
		return
	}
	logx.Infof("[AutoEval] done: queries=%d avg=%.3f pass_rate=%.3f issues=%d duration=%s",
		result.TotalQueries, result.AvgScore, result.Metrics.PassRate, len(result.Issues), result.Duration)
}

// AutoEvaluator 返回自动化评测器（未装配时为 nil）
func (m *Module) AutoEvaluator() *rag.AutoEvaluator {
	if m == nil {
		return nil
	}
	return m.autoEvaluator
}

func (m *Module) AISpaceName() string {
	return m.aiSpaceName
}

func (m *Module) GraphEnhancer() *analyzer.GraphEnhancer {
	if m == nil {
		return nil
	}
	return m.graphEnhancer
}

func (m *Module) ContradictionDetector() *analyzer.ContradictionDetector {
	if m == nil {
		return nil
	}
	return m.contradictionDetector
}

func (m *Module) AdaptiveRetriever() *rag.AdaptiveRetriever {
	if m == nil {
		return nil
	}
	return m.adaptiveRetriever
}

func (m *Module) TableGenerator() *rag.TableGenerator {
	return m.tableGenerator
}

func (m *Module) ReportGenerator() *rag.ReportGenerator {
	return m.reportGenerator
}

func (m *Module) EmailGenerator() *rag.EmailGenerator {
	return m.emailGenerator
}

func (m *Module) Translator() *rag.Translator {
	return m.translator
}

// RAG增强组件getter方法
func (m *Module) SelfRAG() *rag.SelfRAG {
	if m == nil {
		return nil
	}
	return m.selfRAG
}

func (m *Module) HyDERetriever() *rag.HyDERetriever {
	if m == nil {
		return nil
	}
	return m.hydeRetriever
}

func (m *Module) MultiHopReasoner() *rag.MultiHopReasoner {
	if m == nil {
		return nil
	}
	return m.multiHopReasoner
}

func (m *Module) HallucinationDetector() *rag.HallucinationDetector {
	if m == nil {
		return nil
	}
	return m.hallucinationDetector
}

func (m *Module) MemoryManager() *rag.MemoryManager {
	if m == nil {
		return nil
	}
	return m.memoryManager
}

func (m *Module) CorrectiveRAG() *rag.CorrectiveRAG {
	if m == nil {
		return nil
	}
	return m.correctiveRAG
}

func (m *Module) Recommender() *rag.Recommender {
	return m.recommender
}

func (m *Module) QualityPatrol() *rag.QualityPatrol {
	return m.qualityPatrol
}

func (m *Module) MCPClient() *mcp.Client {
	return m.mcpClient
}

func (m *Module) SetMCPClient(client *mcp.Client) {
	m.mcpClient = client
}

// RealtimeDataManager 返回实时数据管理器
func (m *Module) RealtimeDataManager() *rag.RealtimeDataManager {
	if m == nil {
		return nil
	}
	return m.realtimeDataManager
}

// GetAnalyzerConfig 返回分析器配置（深拷贝）
func (m *Module) GetAnalyzerConfig() *analyzer.AnalyzerConfig {
	if m.analyzerCfg != nil {
		cfg := *m.analyzerCfg
		return &cfg
	}
	return analyzer.DefaultAnalyzerConfig()
}

// UpdateAnalyzerConfig 更新分析器配置
func (m *Module) UpdateAnalyzerConfig(cfg *analyzer.AnalyzerConfig) {
	m.analyzerCfg = cfg
}

// ProviderConfig 复用 utils/config 中的统一配置结构
type ProviderConfig = config.AIProviderConfig

// EmbeddingConfig 复用 utils/config 中的统一配置结构
type EmbeddingConfig = config.AIEmbeddingConfig

type AIMCPConfig struct {
	Enabled bool               `mapstructure:"enabled"`
	Servers []mcp.ServerConfig `mapstructure:"servers"`
}

type AIConfig struct {
	Enabled      bool                     `mapstructure:"enabled"`
	Legacy       *ProviderConfig          `mapstructure:"-"`
	Primary      *ProviderConfig          `mapstructure:"primary"`
	Fast         *ProviderConfig          `mapstructure:"fast"`
	Reasoning    *ProviderConfig          `mapstructure:"reasoning"`
	Embedding    *EmbeddingConfig         `mapstructure:"embedding"`
	Analyzer     *analyzer.AnalyzerConfig `mapstructure:"analyzer"`
	AISpaceName  string                   `mapstructure:"ai_space_name"`
	SystemUserID int                      `mapstructure:"system_user_id"`
	Schedule     string                   `mapstructure:"schedule"`
	// P2-2: 定时分析范围配置（可选，默认增量全库）
	ScheduleTaskType string `mapstructure:"schedule_task_type"` // full/incremental/space_only/quick，默认 incremental
	ScheduleSpaceIDs []int  `mapstructure:"schedule_space_ids"` // 当 task_type=space_only 时的目标空间
	// Git 同步配置（git_sync_enabled/git_sync_interval）已随割接 P3 下线删除
	MCP         *AIMCPConfig `mapstructure:"mcp"`
	MinMaturity string       `mapstructure:"min_maturity"`
}

func BuildAIConfig(enabled bool, providerName, baseURL, apiKey, model string, maxTokens int, temperature float64, embURL, embKey, embModel string) *AIConfig {
	cfg := &AIConfig{
		Enabled:      enabled,
		Analyzer:     analyzer.DefaultAnalyzerConfig(),
		AISpaceName:  "AI 知识库",
		SystemUserID: 1,
	}

	if apiKey != "" {
		cfg.Legacy = &ProviderConfig{
			Provider:    providerName,
			BaseURL:     baseURL,
			APIKey:      apiKey,
			Model:       model,
			MaxTokens:   maxTokens,
			Temperature: temperature,
		}
		cfg.Primary = cfg.Legacy
	}

	if embURL != "" || embKey != "" {
		url := embURL
		if url == "" {
			url = strings.TrimRight(baseURL, "/")
		}
		key := embKey
		if key == "" {
			key = apiKey
		}
		m := embModel
		if m == "" {
			m = "text-embedding-3-small"
		}
		cfg.Embedding = &EmbeddingConfig{
			BaseURL: url,
			APIKey:  key,
			Model:   m,
		}
	}

	return cfg
}

func (m *Module) GetSchemaVectorStore() *embedding.SchemaVectorStore {
	return m.schemaVectorStore
}
