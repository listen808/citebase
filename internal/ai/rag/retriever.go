package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai/embedding"
	"github.com/listen808/citebase/internal/ai/observability"
	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/ai/rag/compression"
	"github.com/listen808/citebase/internal/ai/rag/middleware"
	"github.com/listen808/citebase/internal/ai/shared"
	"github.com/listen808/citebase/internal/ctxkeys"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/service/doccontent"
	"github.com/listen808/citebase/internal/service/search"
	"github.com/listen808/citebase/internal/utils"
	"github.com/listen808/citebase/internal/utils/config"
)

type Source struct {
	DocumentID    int       `json:"document_id"`
	DocumentName  string    `json:"document_name"`
	SpaceID       int       `json:"space_id"`
	Score         float64   `json:"score"`
	Snippet       string    `json:"snippet"`
	StartLine     int       `json:"start_line"`
	EndLine       int       `json:"end_line"`
	HeadingPath   string    `json:"heading_path"`
	Keywords      []string  `json:"keywords"`
	CitationIndex int       `json:"citation_index"`
	UpdatedAt     int64     `json:"updated_at"`
	Embedding     []float64 `json:"embedding,omitempty"`
	SourceType    string    `json:"source_type,omitempty"`
	// 以下字段为可验证引用增强（P2-2），用于前端溯源可视化。
	Cited        bool   `json:"cited,omitempty"`         // 答案是否显式引用了该来源（[n] 标记）
	VerifyStatus string `json:"verify_status,omitempty"` // cited / unverified / not_found
	DocURL       string `json:"doc_url,omitempty"`       // 可点击跳转的文档 URL
	ParagraphID  int    `json:"paragraph_id,omitempty"`  // 段落编号（C2 段落溯源）
}

type AIAnswer struct {
	Answer             string               `json:"answer"`
	Sources            []Source             `json:"sources"`
	Citations          []Citation           `json:"citations,omitempty"`
	TokenUsage         provider.TokenUsage  `json:"token_usage,omitempty"`
	ReasoningChain     []ReasoningStep      `json:"reasoning_chain,omitempty"`      // P2-2: 推理链可视化
	Components         []GeneratedComponent `json:"components,omitempty"`           // P2-3: 生成式 UI 结构化组件
	GraphPathCitations []GraphPathCitation  `json:"graph_path_citations,omitempty"` // P3: 图谱路径引用
}

type Citation struct {
	DocumentName string `json:"document_name"`
	ParagraphID  int    `json:"paragraph_id"`
	DocumentID   int    `json:"document_id"`
	Snippet      string `json:"snippet"`
}

type ChatMessage struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// RAGRetrievalStep represents a single step in the RAG process
type RAGRetrievalStep struct {
	Step        string `json:"step"`
	Name        string `json:"name"`
	Description string `json:"description"`
	DurationMs  int64  `json:"duration_ms"`
	ResultCount int    `json:"result_count,omitempty"`
}

// RAGDebugInfo contains debugging information for the RAG process
type RAGDebugInfo struct {
	OriginalQuestion  string             `json:"original_question"`
	RewrittenQuestion string             `json:"rewritten_question,omitempty"`
	SubQuestions      []string           `json:"sub_questions,omitempty"`
	RetrievalSteps    []RAGRetrievalStep `json:"retrieval_steps"`
	VectorResults     []VectorResult     `json:"vector_results,omitempty"`
	KeywordResults    []KeywordResult    `json:"keyword_results,omitempty"`
	CombinedResults   []CombinedResult   `json:"combined_results,omitempty"`
	FinalSources      []Source           `json:"final_sources,omitempty"`
	CacheHit          bool               `json:"cache_hit"`
	TotalDurationMs   int64              `json:"total_duration_ms"`
}

// VectorResult represents a single vector search result
type VectorResult struct {
	DocID int     `json:"doc_id"`
	Score float64 `json:"score"`
	Name  string  `json:"name"`
	// SpaceID P1（清单 70）：向量命中自带归属空间，透传后 Source 不再写死 0，
	// 否则注入 user_id 的链路里权限过滤会把 HyDE 召回全部丢弃、DocURL 也失效
	SpaceID int `json:"space_id"`
}

// KeywordResult represents a single keyword search result
type KeywordResult struct {
	DocID int     `json:"doc_id"`
	Score float64 `json:"score"`
	Name  string  `json:"name"`
}

// CombinedResult represents a combined result from both searches
type CombinedResult struct {
	DocID        int     `json:"doc_id"`
	Score        float64 `json:"score"`
	VectorScore  float64 `json:"vector_score"`
	KeywordScore float64 `json:"keyword_score"`
	Name         string  `json:"name"`
}

type Retriever struct {
	provider     provider.Provider
	fastProvider provider.Provider
	embedder     embedding.Embedder
	docRepo      *workspacerepo.DocumentRepository
	spaceRepo    *workspacerepo.SpaceRepository
	docStorage   *utils.DocumentStorage
	reranker     Reranker
	compressor   *HistoryCompressor
	permChecker  *PermissionChecker
	// P1-3：文档级 ACL 查询（返回被显式 deny 的 docID 集合），nil 时只做空间级过滤
	docACL                func(userID int, docIDs []int) (map[int]bool, error)
	safetyGuard           *SafetyGuard
	hydeRetriever         *HyDERetriever
	selfRAG               *SelfRAG
	hallucinationDetector *HallucinationDetector
	multiHopReasoner      *MultiHopReasoner
	memoryManager         *MemoryManager
	compression           *ContextCompressionLayer
	useSemanticChunker    bool
	contentCache          map[int]string
	contentCacheMu        sync.RWMutex
	parallelRetriever     *ParallelRetriever
	graphExpander         WikiGraphExpander                 // P4：wiki 图谱 2-hop 扩展（可选，启动时注入）
	adaptiveRetriever     *AdaptiveRetriever                // C1：查询类型路由（可选，启动时注入）
	ctxCompressor         *compression.ContextualCompressor // A1：句级语义压缩（可选，启动时注入）
	MiddlewareChain       *middleware.Chain
}

func NewRetriever(p provider.Provider, e embedding.Embedder) *Retriever {
	docDir := "./data/markdowns"
	r := &Retriever{
		provider:           p,
		fastProvider:       nil,
		embedder:           e,
		docRepo:            workspacerepo.NewDocumentRepository(),
		spaceRepo:          workspacerepo.NewSpaceRepository(),
		docStorage:         utils.NewDocumentStorageWithDir(docDir),
		reranker:           newRerankerFromConfig(),
		compressor:         NewHistoryCompressor(nil),
		permChecker:        NewPermissionChecker(),
		safetyGuard:        NewSafetyGuard(p),
		compression:        NewContextCompressionLayer(p, DefaultCompressionConfig()),
		useSemanticChunker: e != nil,
	}
	r.MiddlewareChain = buildDefaultMiddlewareChain(r)
	return r
}

func sourcesToMiddleware(sources []Source) []middleware.Source {
	result := make([]middleware.Source, len(sources))
	for i, s := range sources {
		result[i] = middleware.Source{
			DocumentID:    s.DocumentID,
			DocumentName:  s.DocumentName,
			SpaceID:       s.SpaceID,
			Score:         s.Score,
			Snippet:       s.Snippet,
			StartLine:     s.StartLine,
			EndLine:       s.EndLine,
			HeadingPath:   s.HeadingPath,
			Keywords:      s.Keywords,
			CitationIndex: s.CitationIndex,
			UpdatedAt:     s.UpdatedAt,
			Embedding:     s.Embedding,
			SourceType:    s.SourceType,
			Cited:         s.Cited,
			VerifyStatus:  s.VerifyStatus,
			DocURL:        s.DocURL,
		}
	}
	return result
}

func sourcesFromMiddleware(sources []middleware.Source) []Source {
	result := make([]Source, len(sources))
	for i, s := range sources {
		result[i] = Source{
			DocumentID:    s.DocumentID,
			DocumentName:  s.DocumentName,
			SpaceID:       s.SpaceID,
			Score:         s.Score,
			Snippet:       s.Snippet,
			StartLine:     s.StartLine,
			EndLine:       s.EndLine,
			HeadingPath:   s.HeadingPath,
			Keywords:      s.Keywords,
			CitationIndex: s.CitationIndex,
			UpdatedAt:     s.UpdatedAt,
			Embedding:     s.Embedding,
			SourceType:    s.SourceType,
			Cited:         s.Cited,
			VerifyStatus:  s.VerifyStatus,
			DocURL:        s.DocURL,
		}
	}
	return result
}

func debugInfoToMiddleware(d *RAGDebugInfo) *middleware.RAGDebugInfo {
	if d == nil {
		return nil
	}
	steps := make([]middleware.RAGRetrievalStep, len(d.RetrievalSteps))
	for i, s := range d.RetrievalSteps {
		steps[i] = middleware.RAGRetrievalStep{
			Step:        s.Step,
			Name:        s.Name,
			Description: s.Description,
			DurationMs:  s.DurationMs,
			ResultCount: s.ResultCount,
		}
	}
	return &middleware.RAGDebugInfo{
		OriginalQuestion:  d.OriginalQuestion,
		RewrittenQuestion: d.RewrittenQuestion,
		SubQuestions:      d.SubQuestions,
		RetrievalSteps:    steps,
		CacheHit:          d.CacheHit,
		TotalDurationMs:   d.TotalDurationMs,
	}
}

func buildDefaultMiddlewareChain(r *Retriever) *middleware.Chain {
	chain := middleware.NewChain()
	var checkFn middleware.SafetyCheckFunc
	var validateFn middleware.AnswerValidateFunc
	var auditFn middleware.AuditLogFunc
	if r.safetyGuard != nil {
		checkFn = r.safetyGuard.CheckQuerySafety
		validateFn = r.safetyGuard.ValidateAnswer
		auditFn = r.safetyGuard.LogAuditEvent
	}
	chain.Use(middleware.NewSafetyGuardMiddleware(checkFn, validateFn, auditFn))
	chain.Use(middleware.NewQueryRewriteMiddleware())
	return chain
}

func NewRetrieverWithFastProvider(p provider.Provider, fast provider.Provider, e embedding.Embedder) *Retriever {
	r := NewRetriever(p, e)
	r.fastProvider = fast
	r.compressor = NewHistoryCompressor(fast)
	r.hydeRetriever = NewHyDERetriever(fast, e)
	r.selfRAG = NewSelfRAG(fast, r)
	r.hallucinationDetector = NewHallucinationDetector(fast)
	r.multiHopReasoner = NewMultiHopReasoner(fast, r)
	r.memoryManager = NewMemoryManager(nil, fast) // repo will be set later
	r.parallelRetriever = NewParallelRetriever(3) // 默认3并发
	return r
}

// SetMemoryManager 设置记忆管理器
func (r *Retriever) SetMemoryManager(mm *MemoryManager) {
	r.memoryManager = mm
}

// GetMemoryManager 获取记忆管理器
func (r *Retriever) GetMemoryManager() *MemoryManager {
	return r.memoryManager
}

// SetAdaptiveRetriever 注入查询类型路由器（C1）。
// 注入后 AskWithDebug 在查询重写阶段自动分类查询类型，
// 按类型调整检索深度与生成策略。
func (r *Retriever) SetAdaptiveRetriever(ar *AdaptiveRetriever) {
	r.adaptiveRetriever = ar
}

// SetContextualCompressor 注入句级语义压缩器（A1）。
// 注入后 RetrieveContextWithRAG 在构建引用上下文前，
// 按问题语义过滤每个来源中与问题最相关的句子，减少无关内容。
func (r *Retriever) SetContextualCompressor(cc *compression.ContextualCompressor) {
	r.ctxCompressor = cc
}

// ClassifyQuery 公开查询类型分类（W4 评估器用）。
// 未注入 AdaptiveRetriever 时返回 QueryTypeFactual（保守默认）。
func (r *Retriever) ClassifyQuery(ctx context.Context, query string) QueryType {
	if r.adaptiveRetriever == nil {
		return QueryTypeFactual
	}
	return r.adaptiveRetriever.ClassifyQuery(ctx, query)
}

func (r *Retriever) Ask(ctx context.Context, question string, spaceID int, limit int) (*AIAnswer, error) {
	result, _, err := r.AskWithDebug(ctx, question, spaceID, limit)
	return result, err
}

func (r *Retriever) AskWithSelfRAG(ctx context.Context, question string, spaceID int, limit int) (*AIAnswer, error) {
	if r.selfRAG != nil {
		return r.selfRAG.GenerateWithReflection(ctx, question, spaceID, limit)
	}
	return r.Ask(ctx, question, spaceID, limit)
}

func (r *Retriever) AskWithMultiHop(ctx context.Context, question string, spaceID int, limit int) (*AIAnswer, error) {
	if r.multiHopReasoner != nil {
		result, err := r.multiHopReasoner.Reason(ctx, question, spaceID, limit)
		if err == nil && result != nil {
			return &AIAnswer{
				Answer:  result.Answer,
				Sources: result.Sources,
			}, nil
		}
	}
	return r.Ask(ctx, question, spaceID, limit)
}

// AskWithDebug is like Ask but also returns debugging information
func (r *Retriever) AskWithDebug(ctx context.Context, question string, spaceID int, limit int) (*AIAnswer, *RAGDebugInfo, error) {
	startTime := time.Now()

	obsProvider := observability.DefaultProvider()
	ctx, rootSpan := obsProvider.StartSpan(ctx, "rag.retrieve",
		observability.WithService("rag"),
		observability.WithAttributes(map[string]interface{}{
			"question": question,
			"space_id": spaceID,
			"limit":    limit,
		}),
	)
	defer func() {
		obsProvider.ObserveHistogram(observability.MetricAIRequestDuration, map[string]string{"service": "rag"}, float64(time.Since(startTime).Milliseconds()))
		obsProvider.AddGauge(observability.MetricAIActiveRequests, map[string]string{"service": "rag"}, -1)
	}()
	obsProvider.Inc(observability.MetricAIRequestsTotal, map[string]string{"service": "rag"}, 1)
	obsProvider.AddGauge(observability.MetricAIActiveRequests, map[string]string{"service": "rag"}, 1)

	if r.provider == nil {
		obsProvider.EndSpan(rootSpan, observability.SpanStatusError)
		rootSpan.ErrorMsg = "AI服务未启用"
		return nil, nil, fmt.Errorf("AI服务未启用")
	}

	if limit <= 0 || limit > 20 {
		limit = 5
	}

	debugInfo := &RAGDebugInfo{
		OriginalQuestion: question,
		RetrievalSteps:   []RAGRetrievalStep{},
	}

	ragCtx := &middleware.RAGContext{
		Ctx:        ctx,
		Question:   question,
		SpaceID:    spaceID,
		Limit:      limit,
		DebugInfo:  debugInfoToMiddleware(debugInfo),
		TokenUsage: make(map[string]int64),
		Data:       make(map[string]interface{}),
	}

	if r.MiddlewareChain != nil {
		r.MiddlewareChain.RunBeforeRetrieval(ragCtx)
		if ragCtx.Err != nil {
			debugInfo.TotalDurationMs = time.Since(startTime).Milliseconds()
			return &AIAnswer{
				Answer:  fmt.Sprintf("抱歉，您的查询被安全策略拦截：%s", ragCtx.Err.Error()),
				Sources: []Source{},
			}, debugInfo, nil
		}
		if ragCtx.SkipGeneration {
			debugInfo.TotalDurationMs = time.Since(startTime).Milliseconds()
			return &AIAnswer{
				Answer:  ragCtx.Answer,
				Sources: sourcesFromMiddleware(ragCtx.Sources),
			}, debugInfo, nil
		}
	}

	r.resetContentCache()

	// S2: 安全护栏 - 检查查询安全性
	if r.safetyGuard != nil {
		if safe, reason := r.safetyGuard.CheckQuerySafety(ctx, question); !safe {
			r.safetyGuard.LogAuditEvent(0, "query_blocked", question, reason)
			debugInfo.TotalDurationMs = time.Since(startTime).Milliseconds()
			return &AIAnswer{
				Answer:  fmt.Sprintf("抱歉，您的查询被安全策略拦截：%s", reason),
				Sources: []Source{},
			}, debugInfo, nil
		}
	}

	// S4: 查询语义缓存
	if cache := GetQueryCache(); cache != nil {
		cacheUserID := ctxUserID(ctx)
		if cached := cache.Lookup(question, spaceID, cacheUserID); cached != nil {
			logx.Infof("[RAG] Cache hit for query: %s", question)
			debugInfo.CacheHit = true
			obsProvider.SetAttribute(rootSpan, "cache_hit", true)
			obsProvider.Inc(observability.MetricAICacheHitRatio, map[string]string{"service": "rag", "type": "hit"}, 1)
			obsProvider.EndSpan(rootSpan, observability.SpanStatusOK)
			debugInfo.TotalDurationMs = time.Since(startTime).Milliseconds()
			return cached, debugInfo, nil
		}
		obsProvider.Inc(observability.MetricAICacheHitRatio, map[string]string{"service": "rag", "type": "miss"}, 1)
	}

	// S4: 问题分解 - 复杂问题分解为子问题分别检索
	subStepStart := time.Now()
	subQuestions := r.decomposeQuestion(ctx, question)
	debugInfo.SubQuestions = subQuestions
	debugInfo.RetrievalSteps = append(debugInfo.RetrievalSteps, RAGRetrievalStep{
		Step:        "decompose",
		Name:        "问题分解",
		Description: fmt.Sprintf("将问题分解为 %d 个子问题", len(subQuestions)),
		DurationMs:  time.Since(subStepStart).Milliseconds(),
		ResultCount: len(subQuestions),
	})

	if len(subQuestions) > 1 {
		answer, debug, err := r.askWithSubQuestionsWithDebug(ctx, subQuestions, question, spaceID, limit, startTime)
		if err == nil {
			if cache := GetQueryCache(); cache != nil {
				cacheUserID := ctxUserID(ctx)
				cache.Store(question, spaceID, cacheUserID, answer)
			}
		}
		return answer, debug, err
	}

	// Query rewriting step
	_, rewriteSpan := obsProvider.StartSpan(ctx, "query_rewrite", observability.WithService("rag"))
	rewriteStepStart := time.Now()
	rewrittenQuestion := r.rewriteQuery(ctx, question)
	debugInfo.RewrittenQuestion = rewrittenQuestion
	debugInfo.RetrievalSteps = append(debugInfo.RetrievalSteps, RAGRetrievalStep{
		Step:        "query_rewrite",
		Name:        "查询重写",
		Description: "优化原始查询，提升检索精度",
		DurationMs:  time.Since(rewriteStepStart).Milliseconds(),
	})
	obsProvider.EndSpan(rewriteSpan, observability.SpanStatusOK)

	// C1: 查询类型路由 — 分类后按类型调整检索深度与生成策略。
	// 未注入 AdaptiveRetriever 时跳过（向后兼容）。
	var queryType QueryType
	if r.adaptiveRetriever != nil {
		_, classifySpan := obsProvider.StartSpan(ctx, "query_classify", observability.WithService("rag"))
		classifyStepStart := time.Now()
		queryType = r.adaptiveRetriever.ClassifyQuery(ctx, question)
		logx.Infof("[RAG] Query type: %s for: %s", queryType, question)
		debugInfo.RetrievalSteps = append(debugInfo.RetrievalSteps, RAGRetrievalStep{
			Step:        "query_classify",
			Name:        "查询类型分类",
			Description: GetStrategyDescription(queryType),
			DurationMs:  time.Since(classifyStepStart).Milliseconds(),
		})
		obsProvider.EndSpan(classifySpan, observability.SpanStatusOK)
		obsProvider.SetAttribute(classifySpan, "query_type", string(queryType))

		// 按类型调整检索深度：事实型少而精，概念/创造型多而宽。
		adjustedLimit := adjustLimitByQueryType(queryType, limit)
		if adjustedLimit != limit {
			limit = adjustedLimit
			logx.Infof("[RAG] Limit adjusted %d→%d for query type %s", limit, adjustedLimit, queryType)
		}
	}

	// Hybrid retrieve step
	_, vectorSpan := obsProvider.StartSpan(ctx, "vector_search", observability.WithService("rag"))
	_, keywordSpan := obsProvider.StartSpan(ctx, "keyword_search", observability.WithService("rag"))
	hybridStepStart := time.Now()
	var sources []Source
	var vectorResults []VectorResult
	var keywordResults []KeywordResult
	var combinedResults []CombinedResult

	if r.hydeRetriever != nil {
		sources, _ = r.hybridRetrieveWithHyDE(ctx, rewrittenQuestion, question, spaceID, limit)
	} else {
		sources, vectorResults, keywordResults, combinedResults, _ = r.hybridRetrieveWithDebug(ctx, rewrittenQuestion, question, spaceID, limit)
	}

	obsProvider.EndSpan(vectorSpan, observability.SpanStatusOK)
	obsProvider.SetAttribute(vectorSpan, "result_count", len(vectorResults))
	obsProvider.EndSpan(keywordSpan, observability.SpanStatusOK)
	obsProvider.SetAttribute(keywordSpan, "result_count", len(keywordResults))

	// Populate debug info with results
	debugInfo.VectorResults = vectorResults
	debugInfo.KeywordResults = keywordResults
	debugInfo.CombinedResults = combinedResults

	debugInfo.RetrievalSteps = append(debugInfo.RetrievalSteps, RAGRetrievalStep{
		Step:        "vector_search",
		Name:        "向量搜索",
		Description: "使用语义相似度检索相关文档",
		DurationMs:  time.Since(hybridStepStart).Milliseconds(),
		ResultCount: len(vectorResults),
	})

	debugInfo.RetrievalSteps = append(debugInfo.RetrievalSteps, RAGRetrievalStep{
		Step:        "keyword_search",
		Name:        "关键词搜索",
		Description: "使用关键词匹配检索相关文档",
		DurationMs:  0, // Included in hybrid step for now
		ResultCount: len(keywordResults),
	})

	if len(sources) == 0 {
		debugInfo.TotalDurationMs = time.Since(startTime).Milliseconds()
		return &AIAnswer{
			Answer:  "未找到相关文档，无法回答该问题。",
			Sources: []Source{},
		}, debugInfo, nil
	}

	// Rerank step
	_, rerankSpan := obsProvider.StartSpan(ctx, "rerank", observability.WithService("rag"))
	rerankStepStart := time.Now()
	sources = r.rerankSources(ctx, rewrittenQuestion, sources)
	obsProvider.EndSpan(rerankSpan, observability.SpanStatusOK)
	obsProvider.SetAttribute(rerankSpan, "result_count", len(sources))
	debugInfo.RetrievalSteps = append(debugInfo.RetrievalSteps, RAGRetrievalStep{
		Step:        "rerank",
		Name:        "结果重排序",
		Description: "使用重排序模型优化结果顺序",
		DurationMs:  time.Since(rerankStepStart).Milliseconds(),
		ResultCount: len(sources),
	})

	// S1: 权限感知RAG - 按用户权限过滤无权访问的文档
	filterStepStart := time.Now()
	if userID := ctxkeys.UserID(ctx); userID > 0 && r.permChecker != nil {
		sources = r.permChecker.FilterSourcesByPermission(ctx, userID, sources)
	}
	debugInfo.RetrievalSteps = append(debugInfo.RetrievalSteps, RAGRetrievalStep{
		Step:        "filter",
		Name:        "权限过滤",
		Description: "根据用户权限过滤结果",
		DurationMs:  time.Since(filterStepStart).Milliseconds(),
		ResultCount: len(sources),
	})

	// S2: 引用溯源 - 为每个来源分配序号，并在上下文中标注
	for i := range sources {
		sources[i].CitationIndex = i + 1
		sources[i].ParagraphID = sources[i].StartLine // C2: 段落溯源锚点
	}
	debugInfo.FinalSources = sources

	// P4: wiki 图谱 2-hop 扩展 —— 命中文档 → 来源重叠页 → 相关页，
	// 作为背景上下文注入生成 prompt（不进引用编号）。
	graphStepStart := time.Now()
	graphContext := r.expandGraphContext(ctx, sources)
	if graphContext != "" {
		debugInfo.RetrievalSteps = append(debugInfo.RetrievalSteps, RAGRetrievalStep{
			Step:        "graph_expansion",
			Name:        "图谱扩展",
			Description: "沿 wiki 链接图补充关联背景上下文",
			DurationMs:  time.Since(graphStepStart).Milliseconds(),
		})
	}

	debugInfo.RetrievalSteps = append(debugInfo.RetrievalSteps, RAGRetrievalStep{
		Step:        "final",
		Name:        "最终结果",
		Description: fmt.Sprintf("共返回 %d 个文档来源", len(sources)),
		DurationMs:  0,
		ResultCount: len(sources),
	})

	ragCtx.Sources = sourcesToMiddleware(sources)
	if r.MiddlewareChain != nil {
		r.MiddlewareChain.RunAfterRetrieval(ragCtx)
		if ragCtx.Err != nil {
			debugInfo.TotalDurationMs = time.Since(startTime).Milliseconds()
			return nil, debugInfo, ragCtx.Err
		}
		sources = sourcesFromMiddleware(ragCtx.Sources)
	}

	citedContext := r.buildCitedContext(sources)
	if graphContext != "" {
		citedContext += "\n\n" + graphContext
	}

	if r.MiddlewareChain != nil {
		r.MiddlewareChain.RunBeforeGeneration(ragCtx)
		if ragCtx.Err != nil {
			debugInfo.TotalDurationMs = time.Since(startTime).Milliseconds()
			return nil, debugInfo, ragCtx.Err
		}
	}

	answer, tokenUsage, err := r.callLLM(ctx, rewrittenQuestion, citedContext)
	if err != nil {
		logx.Infof("[RAG] LLM call error: %v", err)
		debugInfo.TotalDurationMs = time.Since(startTime).Milliseconds()
		return nil, debugInfo, fmt.Errorf("AI 回答生成失败: %w", err)
	}

	// P1 修复（清单 71）：幻觉状态必须传入溯源链，此前 verify 阶段恒为 ok，
	// 答案已被判定"可能不准确"时前端仍显示"已校验"的假阳性可信信号。
	var hallucinationPersisted bool
	if r.hallucinationDetector != nil {
		_, hallucSpan := obsProvider.StartSpan(ctx, "hallucination_check", observability.WithService("rag"))
		var retryGuidance string
		var hallucReport *HallucinationReport
		answer, retryGuidance, hallucReport = r.hallucinationDetector.ValidateAnswerWithRetry(ctx, answer, sources)

		// 高置信度幻觉：追加指引后重新生成，最大重试 2 次
		maxRetries := 2
		retryCount := 0
		for retryCount = 1; retryGuidance != "" && retryCount <= maxRetries; retryCount++ {
			logx.Infof("[RAG] Hallucination detected (confidence=%.2f), retry %d/%d", hallucReport.Confidence, retryCount, maxRetries)
			retryContext := citedContext + "\n\n" + retryGuidance
			newAnswer, _, callErr := r.callLLM(ctx, rewrittenQuestion, retryContext)
			if callErr != nil {
				logx.Infof("[RAG] Retry LLM call error: %v, falling back to warning", callErr)
				break
			}
			answer = newAnswer
			answer, retryGuidance, hallucReport = r.hallucinationDetector.ValidateAnswerWithRetry(ctx, answer, sources)
		}

		// 重试后仍幻觉则降级为追加警告
		if retryGuidance != "" {
			logx.Infof("[RAG] Hallucination persists after retries, appending warning")
			hallucinationPersisted = true
			warning := "\n\n⚠️ 注意：以上回答中部分内容可能不完全准确，建议参考原始文档确认。"
			answer = answer + warning
		}
		obsProvider.EndSpan(hallucSpan, observability.SpanStatusOK)
		obsProvider.SetAttribute(hallucSpan, "retry_count", retryCount)
		obsProvider.SetAttribute(hallucSpan, "has_hallucination", retryGuidance != "")
		if hallucReport != nil {
			obsProvider.SetAttribute(hallucSpan, "confidence", hallucReport.Confidence)
		}
	}

	// S6: 数据脱敏 - AI回答后处理，过滤PII信息
	answer = utils.MaskPII(answer)

	// S2: 安全护栏 - 验证回答安全性
	if r.safetyGuard != nil {
		answer = r.safetyGuard.ValidateAnswer(ctx, answer)
	}

	// P2-2: 回填可验证引用字段（引用标记、可信度、跳转 URL）。
	enriched := EnrichAnswer(&AIAnswer{Answer: answer, Sources: sources, TokenUsage: tokenUsage}, nil)
	enriched.ReasoningChain = BuildReasoningChain(
		len(sources), len(sources),
		"基于答案中的引用标记标注每条来源的可信度",
		hallucinationPersisted,
		true, // generationOK：能走到这里说明生成调用已成功
	)
	// P2-3: 提取生成式 UI 结构化组件（图表/表格/卡片），并清理原文本中的围栏块。
	comps := ExtractComponents(answer)
	if len(comps) > 0 {
		enriched.Components = comps
		enriched.Answer = StripComponentBlocks(answer)
	}
	result := enriched

	ragCtx.Answer = result.Answer
	ragCtx.Sources = sourcesToMiddleware(result.Sources)
	if r.MiddlewareChain != nil {
		r.MiddlewareChain.RunAfterGeneration(ragCtx)
		if ragCtx.Err != nil {
			debugInfo.TotalDurationMs = time.Since(startTime).Milliseconds()
			return nil, debugInfo, ragCtx.Err
		}
		result.Answer = ragCtx.Answer
		result.Sources = sourcesFromMiddleware(ragCtx.Sources)
	}

	// S4: 存入缓存
	if cache := GetQueryCache(); cache != nil {
		cacheUserID := ctxUserID(ctx)
		cache.Store(question, spaceID, cacheUserID, result)
	}

	obsProvider.EndSpan(rootSpan, observability.SpanStatusOK)
	obsProvider.SetAttribute(rootSpan, "source_count", len(sources))
	if tokenUsage.TotalTokens > 0 {
		obsProvider.Inc(observability.MetricAITokenUsageTotal, map[string]string{"service": "rag"}, float64(tokenUsage.TotalTokens))
		obsProvider.SetAttribute(rootSpan, "total_tokens", tokenUsage.TotalTokens)
		obsProvider.SetAttribute(rootSpan, "prompt_tokens", tokenUsage.PromptTokens)
		obsProvider.SetAttribute(rootSpan, "completion_tokens", tokenUsage.CompletionTokens)
	}

	debugInfo.TotalDurationMs = time.Since(startTime).Milliseconds()
	return result, debugInfo, nil
}

func (r *Retriever) RetrieveContextOnly(question string, spaceID int, limit int) ([]Source, string) {
	if limit <= 0 || limit > 20 {
		limit = 5
	}
	r.resetContentCache()
	return r.retrieveContext(question, spaceID, limit)
}

// RetrieveScoped 检索 + 权限过滤，供工作流检索节点使用。
//
// 工作流执行链没有用户身份维度，检索必须走这里而不是通用工具节点：
// 结果按 ctx 携带的用户身份做权限过滤，避免提示注入把检索变成越权读文档。
func (r *Retriever) RetrieveScoped(ctx context.Context, question string, spaceID int, limit int) ([]Source, error) {
	if r == nil {
		return nil, fmt.Errorf("检索器未初始化")
	}
	if limit <= 0 || limit > 20 {
		limit = 5
	}
	r.resetContentCache()
	sources, _ := r.retrieveContext(question, spaceID, limit)
	return r.filterSourcesForCtx(ctx, sources), nil
}

func (r *Retriever) RetrieveContextWithRAG(ctx context.Context, question string, spaceID int, limit int) ([]Source, string) {
	if limit <= 0 || limit > 20 {
		limit = 5
	}

	r.resetContentCache()

	rewrittenQuestion := r.rewriteQuery(ctx, question)
	sources, _ := r.hybridRetrieve(ctx, rewrittenQuestion, question, spaceID, limit)

	if len(sources) == 0 {
		return sources, ""
	}

	sources = r.rerankSources(ctx, rewrittenQuestion, sources)

	if userID := ctxkeys.UserID(ctx); userID > 0 && r.permChecker != nil {
		sources = r.permChecker.FilterSourcesByPermission(ctx, userID, sources)
	}

	for i := range sources {
		sources[i].CitationIndex = i + 1
		sources[i].ParagraphID = sources[i].StartLine // C2: 段落溯源锚点
	}

	// A1: 句级语义压缩 — 按问题语义过滤每个来源中与问题最相关的句子。
	// 无 embedder 或来源过少时自动跳过（ContextualCompressor 内部判空）。
	// 压缩器按原始顺序保留高分来源，返回的压缩列表与原始列表是子序列关系。
	if r.ctxCompressor != nil && len(sources) > 1 {
		sources = r.applyContextualCompression(ctx, question, sources)
		for i := range sources {
			sources[i].CitationIndex = i + 1
			sources[i].ParagraphID = sources[i].StartLine // C2: 段落溯源锚点
		}
	}

	citedContext := r.buildCitedContext(sources)

	// 上下文压缩层：当检索上下文 + 历史超出预算时自动压缩，避免撑爆 prompt。
	if r.compression != nil && len(reqHistoryFromCtx(ctx)) > 0 {
		if compressed, _ := r.compression.CompressContextText(ctx, question, citedContext, reqHistoryFromCtx(ctx)); compressed != citedContext {
			citedContext = compressed
		}
	} else if r.compression != nil {
		// 即便没有历史，超长文档上下文也应压缩
		if r.tkEstimate(citedContext) > r.compression.maxTokens/2 {
			if compressed, _ := r.compression.CompressContextText(ctx, question, citedContext, nil); compressed != citedContext {
				citedContext = compressed
			}
		}
	}

	return sources, citedContext
}

// reqHistoryFromCtx 从 ctx 中读取上游注入的对话历史，用于压缩层评估 token 预算。无则返回 nil。
func reqHistoryFromCtx(ctx context.Context) []ChatMessage {
	if h, ok := ctxkeys.ChatHistory(ctx).([]ChatMessage); ok {
		return h
	}
	return nil
}

// tkEstimate 估算文本 token 数（压缩层 tokenizer 的便捷包装）
func (r *Retriever) tkEstimate(text string) int {
	if r.compression != nil {
		return r.compression.tk.CountTokens(text)
	}
	return len([]rune(text)) / 4
}

// RecallMemoryContext 召回与当前问题相关的个性化用户记忆上下文。
// 基于 ctx 中的 user_id 进行个性化检索；无 userID 或记忆管理器未配置时返回空串。
func (r *Retriever) RecallMemoryContext(ctx context.Context, question string) string {
	userID := ctxkeys.UserID(ctx)
	if userID <= 0 || r.memoryManager == nil {
		return ""
	}
	// P7：主链路接入向量语义召回（此前 RecallWithVector 已实现却未接线，
	// 主链路一直走 LLM 评分召回）；无 embedder 时保持原路径。
	var memories []MemoryRelevance
	if r.embedder != nil {
		memories = r.memoryManager.RecallWithVector(ctx, uint64(userID), question, "", 6, r.embedder)
	}
	if len(memories) == 0 {
		memories = r.memoryManager.RecallWithRelevance(ctx, uint64(userID), question, "", 6)
	}
	if len(memories) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## 用户个性化记忆（用于理解用户偏好与历史背景）\n")
	for i, m := range memories {
		typ := m.Memory.MemoryType
		content := m.Memory.Content
		if content == "" {
			content = m.Memory.Summary
		}
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, typ, content))
	}
	return sb.String()
}

func (r *Retriever) AskWithHistory(ctx context.Context, question string, spaceID int, limit int, history []ChatMessage) (*AIAnswer, error) {
	if r.provider == nil {
		return nil, fmt.Errorf("AI服务未启用")
	}

	if limit <= 0 || limit > 20 {
		limit = 5
	}

	r.resetContentCache()

	// S2: 安全护栏检查
	if r.safetyGuard != nil {
		safe, reason := r.safetyGuard.CheckQuerySafety(ctx, question)
		if !safe {
			r.safetyGuard.LogAuditEvent(0, "query_blocked", question, reason)
			return &AIAnswer{Answer: "抱歉，该问题无法回答。" + reason, Sources: []Source{}}, nil
		}
	}

	// S4: 查询语义缓存
	cache := GetQueryCache()
	if cache != nil {
		cacheUserID := ctxUserID(ctx)
		if cached := cache.Lookup(question, spaceID, cacheUserID); cached != nil {
			return cached, nil
		}
	}

	if len(history) > 10 {
		history = history[len(history)-10:]
	}

	// S6: 对话摘要压缩 - 当历史>8条时，用fast model压缩前N条
	if r.compressor != nil {
		history = r.compressor.CompressHistory(ctx, history)
	}

	sources, _ := r.retrieveContext(question, spaceID, limit)

	if len(sources) == 0 {
		return &AIAnswer{Answer: "未找到相关文档，无法回答该问题。", Sources: []Source{}}, nil
	}

	sources = r.rerankSources(ctx, question, sources)

	// S1: 权限过滤
	userID := ctxUserID(ctx)
	if userID > 0 && r.permChecker != nil {
		sources = r.permChecker.FilterSourcesByPermission(ctx, userID, sources)
	}

	// 引用溯源标注
	for i := range sources {
		sources[i].CitationIndex = i + 1
		sources[i].ParagraphID = sources[i].StartLine // C2: 段落溯源锚点
	}
	citedContext := buildCitedContext(sources)

	answer, tokenUsage, err := r.callLLMWithHistory(ctx, question, citedContext, history)
	if err != nil {
		return nil, fmt.Errorf("AI 回答生成失败: %w", err)
	}

	answer = utils.MaskPII(answer)

	if r.safetyGuard != nil {
		answer = r.safetyGuard.ValidateAnswer(ctx, answer)
	}

	result := &AIAnswer{Answer: answer, Sources: sources, TokenUsage: tokenUsage}

	// S4: 存入缓存
	if cache != nil {
		cacheUserID := ctxUserID(ctx)
		cache.Store(question, spaceID, cacheUserID, result)
	}

	return result, nil
}

func (r *Retriever) retrieveContext(question string, spaceID int, limit int) ([]Source, string) {
	orQuery := strings.ReplaceAll(question, " ", "|")
	keywords := extractKeywords(question)

	maxContextChars := config.AppConfig.AI.MaxContextChars

	var sources []Source
	var contextParts []string
	totalChars := 0

	vectorResults := embedding.VectorSearch(question, spaceID, limit, r.embedder)
	if len(vectorResults) == 0 {
		logx.Infof("[RAG] Vector search returned 0 results (retrieveContext) for query: %.50s", question)
	}
	if len(vectorResults) > 0 {
		for _, vr := range vectorResults {
			content := r.readDocumentContent(vr.DocID)
			snippet, startLine, endLine, headingPath := extractSnippetWithLocation(content, 500)
			sources = append(sources, Source{
				DocumentID:   vr.DocID,
				DocumentName: vr.Name,
				SpaceID:      vr.SpaceID,
				Score:        vr.Score,
				Snippet:      snippet,
				StartLine:    startLine,
				EndLine:      endLine,
				HeadingPath:  headingPath,
				Keywords:     keywords,
			})
			if content != "" {
				header := fmt.Sprintf("## 文档: %s (ID: %d)\n", vr.Name, vr.DocID)
				chunked := r.chunkContent(content)
				available := maxContextChars - totalChars - len(header)
				if available <= 0 {
					continue
				}
				if len(chunked) > available {
					chunked = chunked[:available] + "..."
				}
				contextParts = append(contextParts, header+chunked)
				totalChars += len(header) + len(chunked)
			}
			if totalChars >= maxContextChars {
				break
			}
		}
	}

	if len(sources) == 0 {
		result, err := search.SearchDocuments(orQuery, spaceID, limit)
		if err != nil {
			logx.Infof("[RAG] Keyword search error: %v", err)
		} else if result != nil && result.Hits != nil && len(result.Hits) > 0 {
			for _, hit := range result.Hits {
				if hit == nil || hit.Fields == nil {
					continue
				}
				var docID int
				if id, ok := hit.Fields["id"].(float64); ok {
					docID = int(id)
				}
				if docID == 0 {
					continue
				}
				docName, _ := hit.Fields["name"].(string)
				spaceIDVal := 0
				if sid, ok := hit.Fields["space_id"].(float64); ok {
					spaceIDVal = int(sid)
				}
				content := r.readDocumentContent(docID)
				snippet, startLine, endLine, headingPath := extractSnippetWithLocation(content, 500)
				sources = append(sources, Source{
					DocumentID: docID, DocumentName: docName, SpaceID: spaceIDVal, Score: hit.Score, Snippet: snippet,
					StartLine: startLine, EndLine: endLine, HeadingPath: headingPath, Keywords: keywords,
				})
				if content != "" {
					header := fmt.Sprintf("## 文档: %s (ID: %d)\n", docName, docID)
					chunked := r.chunkContent(content)
					available := maxContextChars - totalChars - len(header)
					if available <= 0 {
						continue
					}
					if len(chunked) > available {
						chunked = chunked[:available] + "..."
					}
					contextParts = append(contextParts, header+chunked)
					totalChars += len(header) + len(chunked)
				}
				if totalChars >= maxContextChars {
					break
				}
			}
		}
	}

	contextText := strings.Join(contextParts, "\n\n---\n\n")
	return sources, contextText
}

func (r *Retriever) chunkContent(content string) string {
	var chunker Chunker
	if r.useSemanticChunker && r.embedder != nil {
		chunker = NewAdaptiveChunker(1500, r.embedder, StrategyAuto)
	} else {
		chunker = NewAdaptiveChunker(1500, nil, StrategyAuto)
	}
	chunks := chunker.Chunk(content)
	if len(chunks) == 0 {
		return content
	}
	var result strings.Builder
	for i, c := range chunks {
		if i > 0 {
			result.WriteString("\n\n")
		}
		result.WriteString(c.Content)
	}
	return result.String()
}

func extractKeywords(question string) []string {
	words := strings.Fields(question)
	var keywords []string
	for _, w := range words {
		if len([]rune(w)) >= 2 {
			keywords = append(keywords, w)
		}
	}
	if len(keywords) > 10 {
		keywords = keywords[:10]
	}
	return keywords
}

func extractSnippetWithLocation(content string, maxLen int) (snippet string, startLine int, endLine int, headingPath string) {
	return extractSnippetWithLocationAndQuery(content, maxLen, "")
}

func extractSnippetWithLocationAndQuery(content string, maxLen int, query string) (snippet string, startLine int, endLine int, headingPath string) {
	if content == "" {
		return "", 0, 0, ""
	}

	lines := strings.Split(content, "\n")
	var currentHeading string
	var headingParts []string
	bestStart := 0
	bestEnd := 0
	bestScore := 0

	queryTerms := strings.Fields(strings.ToLower(query))

	for i, line := range lines {
		if strings.HasPrefix(line, "# ") {
			headingParts = []string{strings.TrimSpace(strings.TrimPrefix(line, "# "))}
			currentHeading = headingParts[0]
		} else if strings.HasPrefix(line, "## ") {
			h := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if len(headingParts) > 0 {
				headingParts = append(headingParts[:1], h)
			} else {
				headingParts = []string{h}
			}
			currentHeading = strings.Join(headingParts, " > ")
		} else if strings.HasPrefix(line, "### ") {
			h := strings.TrimSpace(strings.TrimPrefix(line, "### "))
			if len(headingParts) >= 2 {
				headingParts = append(headingParts[:2], h)
			} else if len(headingParts) == 1 {
				headingParts = append(headingParts, h)
			} else {
				headingParts = []string{h}
			}
			currentHeading = strings.Join(headingParts, " > ")
		}

		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 20 && !strings.HasPrefix(trimmed, "#") {
			score := len(trimmed)
			if len(queryTerms) > 0 {
				lowerLine := strings.ToLower(trimmed)
				for _, term := range queryTerms {
					if strings.Contains(lowerLine, term) {
						score += 200
					}
				}
			}
			if score > bestScore {
				bestScore = score
				bestStart = i
				bestEnd = i + 1
				headingPath = currentHeading
			}
		}
	}

	if bestScore == 0 {
		bestStart = 0
		bestEnd = min(10, len(lines))
	}

	for bestEnd < len(lines) && len(strings.Join(lines[bestStart:bestEnd], "\n")) < maxLen {
		bestEnd++
	}

	snippet = strings.Join(lines[bestStart:bestEnd], "\n")
	if len(snippet) > maxLen {
		runes := []rune(snippet)
		if len(runes) > maxLen {
			snippet = string(runes[:maxLen]) + "..."
		}
	}

	return snippet, bestStart + 1, bestEnd, headingPath
}

// sourceWindow 生成侧上下文：按召回定位取窗口，而不是从文档开头截断。
//
// 索引侧是 chunk 级，但生成侧原先一律 readDocumentContent 取整篇再截前 N 个 rune，
// 于是命中落在文档中后部时，喂给模型的是文档开头——分块做得再好也被抹平。
// 行号语义见 extractSnippetWithLocationAndQuery：StartLine 为 1-based 起始行，
// EndLine 为 1-based 结束行的开区间下界。
func (r *Retriever) sourceWindow(src Source, maxRunes int) string {
	content := r.readDocumentContent(src.DocumentID)
	if content == "" {
		return ""
	}
	if maxRunes <= 0 {
		maxRunes = 4000 // 复用 truncateRunes，它对 n<=0 会返回空串
	}
	// ponytail: 无定位信息（HyDE/图扩展等未走定位抽取的分支）时回退旧行为
	if src.StartLine <= 0 || src.EndLine <= src.StartLine-1 {
		return truncateRunes(content, maxRunes)
	}
	lines := strings.Split(content, "\n")
	// 上下各多带 2 行，避免把命中的标题/表格表头切在外面
	start := max(0, src.StartLine-1-2)
	end := min(len(lines), src.EndLine+2)
	if start >= end {
		return truncateRunes(content, maxRunes)
	}
	return truncateRunes(strings.Join(lines[start:end], "\n"), maxRunes)
}

func (r *Retriever) callLLM(ctx context.Context, question string, context string) (string, provider.TokenUsage, error) {
	return r.callLLMWithHistory(ctx, question, context, nil)
}

func (r *Retriever) callLLMWithHistory(ctx context.Context, question string, context string, history []ChatMessage) (string, provider.TokenUsage, error) {
	// S7: Prompt模板化 - 尝试从模板管理器获取
	ptm := GetPromptTemplateManager()
	var systemPrompt string

	if sysTpl, _, ok := ptm.Render("rag_system", map[string]string{
		"context":  context,
		"question": question,
	}); ok {
		systemPrompt = sysTpl
	} else {
		systemPrompt = fmt.Sprintf(`你是一个知识库AI助手。请基于以下提供的文档内容回答问题。
如果文档内容不足以回答问题，请如实说明你无法从已有知识中找到答案。
请用中文回答，保持简洁清晰。

文档内容：
%s`, context)
	}

	// S2: 安全护栏 - 在system prompt中注入安全规则
	if r.safetyGuard != nil {
		permCtx := ""
		if userID := ctxkeys.UserID(ctx); userID > 0 {
			if roleID, ok2 := ctx.Value("role_id").(int); ok2 {
				permCtx = r.permChecker.BuildPermissionContext(userID, roleID)
			}
		}
		systemPrompt = r.safetyGuard.BuildSafeSystemPrompt(systemPrompt, permCtx)
	}

	messages := make([]provider.Message, 0, len(history)*2+1)
	for _, h := range history {
		messages = append(messages, provider.NewTextMessage("user", h.Question))
		messages = append(messages, provider.NewTextMessage("assistant", h.Answer))
	}
	messages = append(messages, provider.NewTextMessage("user", question))

	resp, err := r.provider.Chat(ctx, systemPrompt, messages)
	if err != nil {
		return "", provider.TokenUsage{}, err
	}
	return resp.Content, resp.TokenUsage, nil
}

func (r *Retriever) ReadDocumentContent(docID int) string {
	return r.readDocumentContent(docID)
}

func (r *Retriever) readDocumentContent(docID int) string {
	r.contentCacheMu.RLock()
	if r.contentCache != nil {
		if cached, ok := r.contentCache[docID]; ok {
			r.contentCacheMu.RUnlock()
			return cached
		}
	}
	r.contentCacheMu.RUnlock()

	doc, err := r.docRepo.FindByID(docID)
	if err != nil || doc == nil {
		return ""
	}
	if doc.Type != 1 {
		return ""
	}

	pageFile := getPageFilePath(doc, r.docRepo, r.spaceRepo, r.docStorage)
	content, err := doccontent.Get(doc.ID, r.docStorage, pageFile)
	if err != nil || content == "" {
		latestVersion, verErr := r.docRepo.GetLatestVersion(docID)
		if verErr == nil && latestVersion > 0 {
			versionContent, verErr := r.docRepo.GetVersionContent(docID, latestVersion)
			if verErr == nil {
				content = versionContent
			}
		}
	}
	if content == "" {
		return ""
	}

	r.contentCacheMu.Lock()
	if r.contentCache == nil {
		r.contentCache = make(map[int]string)
	}
	r.contentCache[docID] = content
	r.contentCacheMu.Unlock()
	return content
}

func (r *Retriever) PermissionChecker() *PermissionChecker {
	return r.permChecker
}

func (r *Retriever) SafetyGuard() *SafetyGuard {
	return r.safetyGuard
}

func (r *Retriever) Reranker() Reranker {
	return r.reranker
}

func (r *Retriever) resetContentCache() {
	r.contentCacheMu.Lock()
	r.contentCache = make(map[int]string)
	r.contentCacheMu.Unlock()
}

func (r *Retriever) rerankSources(ctx context.Context, question string, sources []Source) []Source {
	if len(sources) <= 1 {
		return sources
	}
	if r.reranker != nil {
		return r.reranker.Rerank(ctx, question, sources)
	}
	return sources
}

func parseJSON(data string, v interface{}) error {
	return json.Unmarshal([]byte(data), v)
}

func (r *Retriever) rewriteQuery(ctx context.Context, question string) string {
	if r.provider == nil {
		return question
	}
	// P1-8：Adaptive 快链路——短而明确的查询直接检索，跳过一次 LLM 重写调用。
	// 这类问题改写收益接近零，却要为每次问答白付几百 ms 与 token；
	// 复杂长查询才走慢链路（重写 + 多跳）。
	if runes := []rune(strings.TrimSpace(question)); len(runes) <= 12 && !strings.ContainsAny(question, "？?，,；;") {
		return question
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := r.provider.Chat(ctx, `请改写以下用户查询，使其更适合在知识库中检索。补充同义词、消除歧义、扩展缩写。只输出改写后的查询，不要解释。`, []provider.Message{
		provider.NewTextMessage("user", question),
	}, provider.WithMaxTokens(256), provider.WithTemperature(0.2))
	if err != nil {
		return question
	}

	rewritten := strings.TrimSpace(resp.Content)
	if rewritten == "" || len(rewritten) > len(question)*3 {
		return question
	}
	return rewritten
}

func (r *Retriever) hybridRetrieve(ctx context.Context, rewrittenQuestion, originalQuestion string, spaceID int, limit int) ([]Source, string) {
	sources, _, _, _, contextText := r.hybridRetrieveInternal(ctx, rewrittenQuestion, originalQuestion, spaceID, limit, false)
	return sources, contextText
}

func (r *Retriever) containsSource(sources []Source, docID int) bool {
	for _, s := range sources {
		if s.DocumentID == docID {
			return true
		}
	}
	return false
}

func (r *Retriever) hybridRetrieveWithHyDE(ctx context.Context, rewrittenQuestion, originalQuestion string, spaceID int, limit int) ([]Source, string) {
	sources, contextText := r.hybridRetrieve(ctx, rewrittenQuestion, originalQuestion, spaceID, limit)

	if len(sources) < 3 && r.hydeRetriever != nil {
		hydeResults, err := r.hydeRetriever.RetrieveWithHyDE(ctx, originalQuestion, spaceID, limit)
		if err == nil && len(hydeResults) > 0 {
			hydeSources := make([]Source, 0, len(hydeResults))
			for _, vr := range hydeResults {
				content := r.readDocumentContent(vr.DocID)
				snippet, startLine, endLine, headingPath := extractSnippetWithLocationAndQuery(content, 500, originalQuestion)
				hydeSources = append(hydeSources, Source{
					DocumentID:   vr.DocID,
					DocumentName: vr.Name,
					SpaceID:      vr.SpaceID,
					Score:        vr.Score,
					Snippet:      snippet,
					StartLine:    startLine,
					EndLine:      endLine,
					HeadingPath:  headingPath,
				})
			}
			// P0-2: HyDE cross-space 泄漏——RetrieveWithHyDE 在 spaceID=0 时检索所有空间，
			// 直接合并会把用户无权访问的 HyDE 结果混入。在合并前执行权限过滤，与主检索分支保持一致。
			hydeSources = r.filterSourcesForCtx(ctx, hydeSources)
			sources = r.mergeSourcesWithRRF(sources, hydeSources, limit)
		}
	}

	return sources, contextText
}

func (r *Retriever) mergeSourcesWithRRF(vectorResults, keywordResults []Source, limit int) []Source {
	itemsVec := make([]shared.RRFItem, len(vectorResults))
	for i, item := range vectorResults {
		itemsVec[i] = shared.RRFItem{Key: fmt.Sprintf("%d", item.DocumentID), Score: item.Score, Data: item}
	}
	itemsKw := make([]shared.RRFItem, len(keywordResults))
	for i, item := range keywordResults {
		itemsKw[i] = shared.RRFItem{Key: fmt.Sprintf("%d", item.DocumentID), Score: item.Score, Data: item}
	}

	merged := shared.RRFMerge([][]shared.RRFItem{itemsVec, itemsKw}, 60.0, limit)
	result := make([]Source, len(merged))
	for i, item := range merged {
		result[i] = item.Data.(Source)
	}
	return result
}

// hybridRetrieveWithDebug returns sources, vector results, keyword results, combined results, and context text
func (r *Retriever) hybridRetrieveWithDebug(ctx context.Context, rewrittenQuestion, originalQuestion string, spaceID int, limit int) ([]Source, []VectorResult, []KeywordResult, []CombinedResult, string) {
	sources, vResults, kResults, cResults, contextText := r.hybridRetrieveInternal(ctx, rewrittenQuestion, originalQuestion, spaceID, limit, true)
	return sources, vResults, kResults, cResults, contextText
}

func (r *Retriever) hybridRetrieveInternal(ctx context.Context, rewrittenQuestion, originalQuestion string, spaceID int, limit int, includeDebug bool) ([]Source, []VectorResult, []KeywordResult, []CombinedResult, string) {
	vectorWeight := config.AppConfig.AI.VectorWeight
	keywordWeight := config.AppConfig.AI.KeywordWeight
	maxContextChars := config.AppConfig.AI.MaxContextChars
	searchTopK := config.AppConfig.AI.SearchTopK
	similarityThreshold := config.AppConfig.AI.SimilarityThreshold

	if searchTopK > 0 {
		limit = min(limit, searchTopK)
	}

	vectorResults := embedding.VectorSearch(rewrittenQuestion, spaceID, limit, r.embedder)

	if len(vectorResults) == 0 {
		logx.Infof("[RAG] Vector search returned 0 results for query: %.50s", rewrittenQuestion)
	} else {
		logx.Infof("[RAG] Vector search returned %d results for query: %.50s", len(vectorResults), rewrittenQuestion)
	}

	vectorSources := make(map[int]Source)
	vectorRawScores := make(map[int]float64)
	var vectorScoreList []float64
	var vectorResultsForDebug []VectorResult
	for _, vr := range vectorResults {
		if vr.Score < similarityThreshold {
			continue
		}
		content := r.readDocumentContent(vr.DocID)
		snippet, startLine, endLine, headingPath := extractSnippetWithLocationAndQuery(content, 500, rewrittenQuestion)
		vectorSources[vr.DocID] = Source{
			DocumentID:   vr.DocID,
			DocumentName: vr.Name,
			SpaceID:      vr.SpaceID,
			Score:        0,
			Snippet:      snippet,
			StartLine:    startLine,
			EndLine:      endLine,
			HeadingPath:  headingPath,
		}
		vectorRawScores[vr.DocID] = vr.Score
		vectorScoreList = append(vectorScoreList, vr.Score)

		if includeDebug {
			vectorResultsForDebug = append(vectorResultsForDebug, VectorResult{
				DocID: vr.DocID,
				Score: vr.Score,
				Name:  vr.Name,
			})
		}
	}

	orQuery := strings.ReplaceAll(rewrittenQuestion, " ", "|")
	result, _ := search.SearchDocuments(orQuery, spaceID, limit)

	keywordRawScores := make(map[int]float64)
	var keywordScoreList []float64
	var keywordResultsForDebug []KeywordResult
	if result != nil && result.Hits != nil {
		for _, hit := range result.Hits {
			if hit == nil || hit.Fields == nil {
				continue
			}
			docID := 0
			if id, ok := hit.Fields["id"].(float64); ok {
				docID = int(id)
			}
			if docID == 0 {
				continue
			}
			docName, _ := hit.Fields["name"].(string)
			sid := 0
			if s, ok := hit.Fields["space_id"].(float64); ok {
				sid = int(s)
			}
			content := r.readDocumentContent(docID)
			snippet, startLine, endLine, headingPath := extractSnippetWithLocationAndQuery(content, 500, rewrittenQuestion)

			keywordRawScores[docID] = hit.Score
			keywordScoreList = append(keywordScoreList, hit.Score)

			if _, exists := vectorSources[docID]; !exists {
				vectorSources[docID] = Source{
					DocumentID:   docID,
					DocumentName: docName,
					SpaceID:      sid,
					Score:        0,
					Snippet:      snippet,
					StartLine:    startLine,
					EndLine:      endLine,
					HeadingPath:  headingPath,
				}
			}

			if includeDebug {
				keywordResultsForDebug = append(keywordResultsForDebug, KeywordResult{
					DocID: docID,
					Score: hit.Score,
					Name:  docName,
				})
			}
		}
	}

	// Normalize vector and keyword scores to [0,1], then apply weights
	vMin, vMax := 0.0, 1.0
	if len(vectorScoreList) > 0 {
		vMin, vMax = minMaxSlice(vectorScoreList)
	}
	kMin, kMax := 0.0, 1.0
	if len(keywordScoreList) > 0 {
		kMin, kMax = minMaxSlice(keywordScoreList)
	}

	var combinedResultsForDebug []CombinedResult
	for docID, src := range vectorSources {
		vScore := 0.0
		if raw, ok := vectorRawScores[docID]; ok {
			vScore = normalizeScore(raw, vMin, vMax) * vectorWeight
		}
		kScore := 0.0
		if raw, ok := keywordRawScores[docID]; ok {
			kScore = normalizeScore(raw, kMin, kMax) * keywordWeight
		}
		src.Score = vScore + kScore
		vectorSources[docID] = src

		if includeDebug {
			combinedResultsForDebug = append(combinedResultsForDebug, CombinedResult{
				DocID:        docID,
				Score:        src.Score,
				VectorScore:  vScore,
				KeywordScore: kScore,
				Name:         src.DocumentName,
			})
		}
	}

	var allSources []Source
	for _, src := range vectorSources {
		allSources = append(allSources, src)
	}

	docIDs := make([]int, 0, len(allSources))
	for _, src := range allSources {
		docIDs = append(docIDs, src.DocumentID)
	}
	if len(docIDs) > 0 && r.docRepo != nil {
		docs, _ := r.docRepo.FindByIDs(docIDs)
		updateTimeMap := make(map[int]int64, len(docs))
		for _, d := range docs {
			updateTimeMap[d.ID] = int64(d.UpdateTime)
		}
		for i := range allSources {
			if t, ok := updateTimeMap[allSources[i].DocumentID]; ok {
				allSources[i].UpdatedAt = t
			}
		}
	}

	sort.Slice(allSources, func(i, j int) bool {
		return allSources[i].Score > allSources[j].Score
	})

	// Sort debug combined results too
	if includeDebug {
		sort.Slice(combinedResultsForDebug, func(i, j int) bool {
			return combinedResultsForDebug[i].Score > combinedResultsForDebug[j].Score
		})
	}

	// P1-3：检索后二次校验——按调用者可见空间 + 文档级 ACL 再过滤一遍。
	// 图扩展 / HyDE 是旁路召回，前置过滤挡不住它们，必须在合并后统一裁一次。
	allSources = r.filterSourcesForCtx(ctx, allSources)

	if len(allSources) > limit {
		allSources = allSources[:limit]
	}
	if includeDebug && len(combinedResultsForDebug) > limit {
		combinedResultsForDebug = combinedResultsForDebug[:limit]
	}

	var contextParts []string
	totalChars := 0
	for _, src := range allSources {
		header := fmt.Sprintf("## 文档: %s (ID: %d)\n", src.DocumentName, src.DocumentID)
		available := maxContextChars - totalChars - len(header)
		if available <= 0 {
			break
		}
		content := r.sourceWindow(src, available)
		if content == "" {
			continue
		}
		contextParts = append(contextParts, header+content)
		totalChars += len(header) + len(content)
		if totalChars >= maxContextChars {
			break
		}
	}

	return allSources, vectorResultsForDebug, keywordResultsForDebug, combinedResultsForDebug, strings.Join(contextParts, "\n\n---\n\n")
}

// askWithSubQuestionsWithDebug handles sub-questions with debug info
func (r *Retriever) askWithSubQuestionsWithDebug(ctx context.Context, subQuestions []string, originalQuestion string, spaceID int, limit int, startTime time.Time) (*AIAnswer, *RAGDebugInfo, error) {
	debugInfo := &RAGDebugInfo{
		OriginalQuestion: originalQuestion,
		SubQuestions:     subQuestions,
		RetrievalSteps:   []RAGRetrievalStep{},
	}

	var allSources []Source
	var contextParts []string
	seenDocs := make(map[int]bool)

	subLimit := limit
	if len(subQuestions) > 1 {
		subLimit = max(3, limit/len(subQuestions))
	}

	// 使用并行检索器
	if r.parallelRetriever != nil && len(subQuestions) > 1 {
		retrieveFunc := func(ctx context.Context, sq string) ([]RetrievalResult, error) {
			rewritten := r.rewriteQuery(ctx, sq)
			sources, _, _, _, _ := r.hybridRetrieveInternal(ctx, rewritten, sq, spaceID, subLimit, false)
			// 与主分支一致：召回结果必须按调用者权限过滤。过滤要在这里做——
			// 下面转成 RetrievalResult 之后 SpaceID 就丢了，事后无法再判定归属空间。
			sources = r.filterSourcesForCtx(ctx, sources)

			var results []RetrievalResult
			for _, s := range sources {
				results = append(results, RetrievalResult{
					DocumentID:   s.DocumentID,
					DocumentName: s.DocumentName,
					SpaceID:      s.SpaceID,
					Score:        s.Score,
					Snippet:      s.Snippet,
				})
			}
			return results, nil
		}

		results, err := r.parallelRetriever.Retrieve(ctx, subQuestions, retrieveFunc)
		if err == nil {
			for i, resultBatch := range results {
				sq := subQuestions[i]
				for _, result := range resultBatch {
					doc := Source{
						DocumentID:   result.DocumentID,
						DocumentName: result.DocumentName,
						SpaceID:      result.SpaceID,
						Score:        result.Score,
						Snippet:      result.Snippet,
					}
					if !seenDocs[doc.DocumentID] {
						seenDocs[doc.DocumentID] = true
						allSources = append(allSources, doc)
					}
				}
				// 构建上下文
				rewritten := r.rewriteQuery(ctx, sq)
				srcs, _, _, _, ctxText := r.hybridRetrieveInternal(ctx, rewritten, sq, spaceID, subLimit, false)
				if perm := r.filterSourcesForCtx(ctx, srcs); len(perm) != len(srcs) {
					// 有权重被过滤掉时必须换用过滤后的来源构建上下文，
					// 否则无权文档正文仍会进入 prompt 并被复述出来。
					ctxText = r.buildCitedContext(perm)
				}
				if ctxText != "" {
					contextParts = append(contextParts, fmt.Sprintf("### 子问题: %s\n%s", sq, ctxText))
				}
			}
		} else {
			// 回退到串行检索
			r.retrieveSerially(ctx, subQuestions, spaceID, subLimit, seenDocs, &allSources, &contextParts)
		}
	} else {
		// 串行检索
		r.retrieveSerially(ctx, subQuestions, spaceID, subLimit, seenDocs, &allSources, &contextParts)
	}

	if len(allSources) == 0 {
		debugInfo.TotalDurationMs = time.Since(startTime).Milliseconds()
		return &AIAnswer{Answer: "未找到相关文档，无法回答该问题。", Sources: []Source{}}, debugInfo, nil
	}

	allSources = r.rerankSources(ctx, strings.Join(subQuestions, " "), allSources)
	if len(allSources) > limit {
		allSources = allSources[:limit]
	}

	for i := range allSources {
		allSources[i].CitationIndex = i + 1
		allSources[i].ParagraphID = allSources[i].StartLine // C2: 段落溯源锚点
	}
	debugInfo.FinalSources = allSources

	debugInfo.RetrievalSteps = append(debugInfo.RetrievalSteps, RAGRetrievalStep{
		Step:        "final",
		Name:        "最终结果",
		Description: fmt.Sprintf("共返回 %d 个文档来源", len(allSources)),
		ResultCount: len(allSources),
	})

	fullContext := strings.Join(contextParts, "\n\n---\n\n")
	answer, tokenUsage, err := r.callLLM(ctx, strings.Join(subQuestions, "；"), fullContext)
	if err != nil {
		debugInfo.TotalDurationMs = time.Since(startTime).Milliseconds()
		return nil, debugInfo, fmt.Errorf("AI 回答生成失败: %w", err)
	}

	answer = utils.MaskPII(answer)
	debugInfo.TotalDurationMs = time.Since(startTime).Milliseconds()
	return &AIAnswer{Answer: answer, Sources: allSources, TokenUsage: tokenUsage}, debugInfo, nil
}

// retrieveSerially 串行检索（回退方案）
func (r *Retriever) retrieveSerially(ctx context.Context, subQuestions []string, spaceID int, subLimit int, seenDocs map[int]bool, allSources *[]Source, contextParts *[]string) {
	for _, sq := range subQuestions {
		rewritten := r.rewriteQuery(ctx, sq)
		sources, _, _, _, ctxText := r.hybridRetrieveInternal(ctx, rewritten, sq, spaceID, subLimit, false)
		filtered := r.filterSourcesForCtx(ctx, sources)
		if len(filtered) != len(sources) {
			// 有来源被权限过滤掉时，上下文必须按过滤后的来源重建。
			ctxText = r.buildCitedContext(filtered)
		}
		sources = filtered

		for _, s := range sources {
			if !seenDocs[s.DocumentID] {
				seenDocs[s.DocumentID] = true
				*allSources = append(*allSources, s)
			}
		}
		if ctxText != "" {
			*contextParts = append(*contextParts, fmt.Sprintf("### 子问题: %s\n%s", sq, ctxText))
		}
	}
}

// filterSourcesForCtx 按当前调用者（ctx 中的 user_id）过滤召回来源，与主检索分支保持同一套
// 权限语义；未注入权限校验器时原样返回。
func (r *Retriever) filterSourcesForCtx(ctx context.Context, sources []Source) []Source {
	if len(sources) == 0 {
		return sources
	}
	if r.permChecker != nil {
		sources = r.permChecker.FilterSourcesByPermission(ctx, ctxUserID(ctx), sources)
	}
	// P1-3：检索后二次校验文档级 ACL。只要无权限内容进了上下文，风险就已经发生，
	// 光靠检索前过滤不够——图扩展、HyDE 等旁路召回可能绕过前置过滤。
	return r.filterDeniedDocs(ctx, sources)
}

// SetDocumentACL P1-3：注入文档级 ACL 查询。fn 返回"被显式 deny 的 docID 集合"。
// 不注入时行为与之前完全一致（只有空间成员级权限）。
func (r *Retriever) SetDocumentACL(fn func(userID int, docIDs []int) (map[int]bool, error)) {
	r.docACL = fn
}

func (r *Retriever) filterDeniedDocs(ctx context.Context, sources []Source) []Source {
	if r.docACL == nil || len(sources) == 0 {
		return sources
	}
	userID := ctxUserID(ctx)
	if userID <= 0 {
		return sources
	}
	docIDs := make([]int, 0, len(sources))
	for _, s := range sources {
		docIDs = append(docIDs, s.DocumentID)
	}
	denied, err := r.docACL(userID, docIDs)
	if err != nil {
		// 权限查询失败不能放行：宁可少给结果，也不能把受限文档带进上下文
		logx.Warnf("[RAG] document ACL lookup failed for user %d, dropping all candidates: %v", userID, err)
		return nil
	}
	if len(denied) == 0 {
		return sources
	}
	kept := make([]Source, 0, len(sources))
	dropped := 0
	for _, s := range sources {
		if denied[s.DocumentID] {
			dropped++
			continue
		}
		kept = append(kept, s)
	}
	if dropped > 0 {
		logx.Infof("[RAG] document ACL dropped %d/%d sources for user %d", dropped, len(sources), userID)
	}
	return kept
}

func (r *Retriever) decomposeQuestion(ctx context.Context, question string) []string {
	if r.provider == nil {
		return []string{question}
	}

	// S4: 仅当问题较长或包含复合词时才分解
	runes := []rune(question)
	if len(runes) < 20 {
		return []string{question}
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	resp, err := r.provider.Chat(ctx, `如果以下问题可以分解为多个子问题，请分解。输出JSON数组，每个元素是一个子问题。如果问题简单不需要分解，输出单元素数组。
示例：["什么是JWT？", "JWT的刷新机制是什么？"]`, []provider.Message{
		provider.NewTextMessage("user", question),
	}, provider.WithMaxTokens(512), provider.WithTemperature(0.2))
	if err != nil {
		return []string{question}
	}

	var subQuestions []string
	trimmed := strings.TrimSpace(resp.Content)
	if idx := strings.Index(trimmed, "["); idx >= 0 {
		trimmed = trimmed[idx:]
		if last := strings.LastIndex(trimmed, "]"); last >= 0 {
			trimmed = trimmed[:last+1]
		}
	}
	if err := json.Unmarshal([]byte(trimmed), &subQuestions); err != nil || len(subQuestions) == 0 {
		return []string{question}
	}
	// 单元素数组等于未分解
	if len(subQuestions) == 1 && subQuestions[0] == question {
		return []string{question}
	}
	// P1（清单 72）：子问题个数由模型输出决定（可被文档提示注入放大），每个子问题
	// 都会触发一次改写 + 全量检索。钳制到 4 个，成本与延迟不再随模型输出线性放大。
	if len(subQuestions) > 4 {
		subQuestions = subQuestions[:4]
	}
	return subQuestions
}

// S2+S5: buildCitedContext 构建带引用标注+上下文窗口增强的上下文
func (r *Retriever) buildCitedContext(sources []Source) string {
	var parts []string
	for _, src := range sources {
		content := r.sourceWindow(src, 4000)
		if content == "" {
			continue
		}

		// S5: 上下文窗口增强 - 注入文档标题和标题路径前缀（Anthropic contextual retrieval）
		var contextPrefix strings.Builder
		contextPrefix.WriteString(fmt.Sprintf("## [来源%d] 文档: %s (ID: %d)\n", src.CitationIndex, src.DocumentName, src.DocumentID))
		if src.HeadingPath != "" {
			contextPrefix.WriteString(fmt.Sprintf("> 文档位置: %s\n\n", src.HeadingPath))
		}
		// 注入文档标题作为上下文，提升检索召回率
		docTitle := src.DocumentName
		if docTitle != "" {
			content = fmt.Sprintf("<document_title>%s</document_title>\n%s", docTitle, content)
		}
		if src.HeadingPath != "" {
			content = fmt.Sprintf("<section_path>%s</section_path>\n%s", src.HeadingPath, content)
		}

		parts = append(parts, contextPrefix.String()+content)
	}
	return strings.Join(parts, "\n\n---\n\n")
}

// S4: askWithSubQuestions 对子问题分别检索后合并回答
func (r *Retriever) askWithSubQuestions(ctx context.Context, subQuestions []string, spaceID int, limit int) (*AIAnswer, error) {
	var allSources []Source
	var contextParts []string
	seenDocs := make(map[int]bool)

	subLimit := limit
	if len(subQuestions) > 1 {
		subLimit = max(3, limit/len(subQuestions))
	}

	for _, sq := range subQuestions {
		rewritten := r.rewriteQuery(ctx, sq)
		sources, ctxText := r.hybridRetrieve(ctx, rewritten, sq, spaceID, subLimit)

		for _, s := range sources {
			if !seenDocs[s.DocumentID] {
				seenDocs[s.DocumentID] = true
				allSources = append(allSources, s)
			}
		}
		if ctxText != "" {
			contextParts = append(contextParts, fmt.Sprintf("### 子问题: %s\n%s", sq, ctxText))
		}
	}

	if len(allSources) == 0 {
		return &AIAnswer{Answer: "未找到相关文档，无法回答该问题。", Sources: []Source{}}, nil
	}

	allSources = r.rerankSources(ctx, strings.Join(subQuestions, " "), allSources)
	if len(allSources) > limit {
		allSources = allSources[:limit]
	}

	for i := range allSources {
		allSources[i].CitationIndex = i + 1
		allSources[i].ParagraphID = allSources[i].StartLine // C2: 段落溯源锚点
	}

	fullContext := strings.Join(contextParts, "\n\n---\n\n")
	answer, tokenUsage, err := r.callLLM(ctx, strings.Join(subQuestions, "；"), fullContext)
	if err != nil {
		return nil, fmt.Errorf("AI 回答生成失败: %w", err)
	}

	answer = utils.MaskPII(answer)
	return &AIAnswer{Answer: answer, Sources: allSources, TokenUsage: tokenUsage}, nil
}

func buildCitedContext(sources []Source) string {
	var sb strings.Builder
	for _, src := range sources {
		sb.WriteString(fmt.Sprintf("[来源%d] %s\n", src.CitationIndex, src.DocumentName))
		if src.Snippet != "" {
			sb.WriteString(src.Snippet)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func normalizeScore(score, min, max float64) float64 {
	if max == min {
		return 1.0
	}
	return (score - min) / (max - min)
}

func minMaxSlice(scores []float64) (float64, float64) {
	if len(scores) == 0 {
		return 0, 1
	}
	min, max := scores[0], scores[0]
	for _, s := range scores[1:] {
		if s < min {
			min = s
		}
		if s > max {
			max = s
		}
	}
	return min, max
}

func parseCitations(answer string, sources []Source) []Citation {
	re := regexp.MustCompile(`\[([^¶\]]+)¶(\d+)\]`)
	matches := re.FindAllStringSubmatch(answer, -1)
	var citations []Citation
	for _, m := range matches {
		name := m[1]
		paraID := 0
		fmt.Sscanf(m[2], "%d", &paraID)
		for _, s := range sources {
			if s.DocumentName == name {
				citations = append(citations, Citation{
					DocumentName: name,
					ParagraphID:  paraID,
					DocumentID:   s.DocumentID,
					Snippet:      s.Snippet,
				})
				break
			}
		}
	}
	return citations
}

// applyContextualCompression 对来源列表做句级语义压缩（A1）。
// 将 Source 转为 compression.Source，调用压缩器，再按原始顺序回填保留的来源。
// 压缩器按原始顺序过滤低分来源，所以结果与原始列表是子序列关系。
func (r *Retriever) applyContextualCompression(ctx context.Context, question string, sources []Source) []Source {
	miniSources := make([]compression.Source, len(sources))
	for i, s := range sources {
		miniSources[i] = compression.Source{Snippet: s.Snippet, Score: s.Score}
	}
	compressedMini, stats, err := r.ctxCompressor.Compress(ctx, question, miniSources)
	if err != nil || len(compressedMini) == 0 {
		return sources
	}

	// 压缩器通过 KeptIndices 记录了哪些原始来源被保留，直接按索引回填。
	if len(stats.KeptIndices) != len(compressedMini) {
		logx.Warnf("[RAG] ContextualCompressor: KeptIndices length %d != compressed %d", len(stats.KeptIndices), len(compressedMini))
		return sources
	}

	kept := make([]Source, 0, len(stats.KeptIndices))
	for i, origIdx := range stats.KeptIndices {
		s := sources[origIdx]
		s.Snippet = compressedMini[i].Snippet
		s.Score = compressedMini[i].Score
		kept = append(kept, s)
	}

	if len(kept) < len(sources) {
		logx.Infof("[RAG] ContextualCompressor: %d→%d chunks (saved %.0f%%)",
			stats.ChunksProcessed, len(kept), stats.SavedPercent)
	}
	return kept
}

// adjustLimitByQueryType 按查询类型调整检索深度（C1）。
// 事实型少而精（最多 3），概念/比较/创造型多而宽（最多 10），步骤型取默认。
func adjustLimitByQueryType(qt QueryType, baseLimit int) int {
	const maxFactual = 3
	const maxBroad = 10
	switch qt {
	case QueryTypeFactual:
		if baseLimit > maxFactual {
			return maxFactual
		}
	case QueryTypeConceptual, QueryTypeComparison, QueryTypeCreative:
		if baseLimit < maxBroad && baseLimit > 0 {
			return maxBroad
		}
	}
	return baseLimit
}
