package text2sql

import (
	"context"
	"fmt"

	logx "github.com/listen808/citebase/internal/log"
	modelai "github.com/listen808/citebase/internal/model/ai"
)

// P2-3 (§4.3.4): 统一生成入口
//
// 目标：消除 5 个 Generate* 方法中重复的 KB 加载 + 文本预处理 + Trace 初始化代码。
//
// 设计原则（Karpathy: surgical changes）：
//   - 不强制所有方法走同一个 generateCore（下游 LLM 调用路径差异大）
//   - 抽取公共"准备阶段"为 prepareKB + prepareGenerationContext
//   - 现有方法可渐进式接入，不破坏已有行为
//
// 公共子集（4/5 方法都有）：
//   1. KB 实例化 + schemaRepo 注入
//   2. KB 加载（LoadFromSpace + 空表兜底 + LoadRules）
//   3. 文本预处理（sanitizeQuestionFields + applyGlossaryMapping）
//   4. TraceCollector 创建（2/5 方法有，可选）

// ModelRouterAdapter P2-5: 模型路由器适配接口
// 避免直接依赖 router 包（防止循环依赖），由 ai.go 注入实现
type ModelRouterAdapter interface {
	// SelectByContextLength 按上下文 token 数选择模型
	SelectByContextLength(tokenCount int) (LLMProvider, []LLMProvider)
	// SelectReasoning 选择推理模型（复杂查询）
	SelectReasoning() (LLMProvider, []LLMProvider)
}

// GenerateOptions 生成选项（统一入口的差异化参数）
type GenerateOptions struct {
	SpaceID           uint64
	EnableTrace       bool // 是否创建 TraceCollector
	EnableIntent      bool // 是否做 intent 检测 + ResolveReference
	EnableTemplate    bool // 是否尝试模板快速路径
	EnableSanitize    bool // 是否做 sanitizeQuestionFields + applyGlossaryMapping
	FallbackAllTables bool // KB 为空时是否回退加载全部表缓存
}

// DefaultGenerateOptions 默认选项（与 GenerateWithSpace 行为对齐）
func DefaultGenerateOptions() GenerateOptions {
	return GenerateOptions{
		EnableTrace:       true,
		EnableIntent:      true,
		EnableTemplate:    true,
		EnableSanitize:    true,
		FallbackAllTables: true,
	}
}

// GenerationContext 准备阶段的输出（传递给下游 LLM 调用）
type GenerationContext struct {
	KB             *KnowledgeBase
	ResolvedText   string
	Intent         *UserIntent
	TraceCollector *TraceCollector
	// SpaceID 透传请求空间：模板快速路径读取 schema 缓存时按空间分片取用，
	// 避免全局 pool 把跨空间同名表的字段塞进本次生成的模板上下文。
	SpaceID int
}

// prepareKB 加载知识库（公共子集：LoadFromSpace + 空表兜底 + LoadRules + schemaRepo 注入）
// 调用方传入已创建的 kb 实例（允许复用已有 kb）
func (g *Generator) prepareKB(kb *KnowledgeBase, spaceID uint64, fallbackAllTables bool) error {
	if g.schemaRepo != nil {
		kb.SetSchemaRepo(g.schemaRepo)
	}
	if spaceID > 0 {
		if err := kb.LoadFromSpace(spaceID); err != nil {
			return fmt.Errorf("加载空间知识库失败: %w", err)
		}
	}
	// 空表兜底
	if fallbackAllTables && len(kb.GetAllTables()) == 0 {
		caches, err := g.repo.GetAllTableCaches()
		if err == nil {
			for i := range caches {
				kb.tables[caches[i].TableNam] = &caches[i]
			}
		}
	}
	if err := kb.LoadRules(); err != nil {
		return fmt.Errorf("加载规则失败: %w", err)
	}
	return nil
}

// prepareGenerationContext 统一准备阶段：KB 加载 + 文本预处理 + Intent + Trace
// 返回 GenerationContext 供下游 LLM 调用使用
func (g *Generator) prepareGenerationContext(ctx context.Context, text string, opts GenerateOptions) (*GenerationContext, error) {
	// 1. KB 加载
	kb := NewKnowledgeBase(g.repo)
	if err := g.prepareKB(kb, opts.SpaceID, opts.FallbackAllTables); err != nil {
		return nil, err
	}

	// 2. 文本预处理
	resolvedText := text
	if opts.EnableSanitize {
		resolvedText = sanitizeQuestionFields(resolvedText, kb)
		resolvedText = g.applyGlossaryMapping(resolvedText, int(opts.SpaceID))
	}

	// 3. Intent 检测 + 引用解析
	var intent *UserIntent
	if opts.EnableIntent {
		g.conversationState.RecordTurn(text, "", nil)
		detected := g.conversationState.DetectIntent(text)
		g.conversationState.RecordIntent(detected)
		intent = &detected
		resolvedText = g.conversationState.ResolveReference(resolvedText)
	}

	// 4. TraceCollector
	var traceCollector *TraceCollector
	if opts.EnableTrace && g.traceRepo != nil && GetText2SQLConfig().EnableTrace {
		traceCollector = NewTraceCollector(opts.SpaceID, int(g.currentConvID.Load()), g.traceRepo)
		g.traceCollector = traceCollector
	}

	return &GenerationContext{
		KB:             kb,
		ResolvedText:   resolvedText,
		Intent:         intent,
		TraceCollector: traceCollector,
		SpaceID:        int(opts.SpaceID),
	}, nil
}

// tryTemplateFastPath 模板快速路径（命中即返回，未命中返回 nil）
// 调用方应在调用 LLM 前先检查
func (g *Generator) tryTemplateFastPath(genCtx *GenerationContext, text string) *modelai.SQLResult {
	if genCtx.Intent == nil {
		return nil
	}
	templateSQL, score := g.tryTemplateGenerate(genCtx.ResolvedText, genCtx.Intent.IntentType, genCtx.KB, genCtx.SpaceID)
	if templateSQL == "" {
		return nil
	}

	var knownTables []string
	for name := range genCtx.KB.GetAllTables() {
		knownTables = append(knownTables, name)
	}
	result := &modelai.SQLResult{
		SQL:      templateSQL,
		Tables:   extractTablesFromSQL(templateSQL, knownTables),
		Warnings: []string{fmt.Sprintf("模板生成（匹配度 %.0f%%）", score*100)},
	}

	// 记录到 conversationState
	if len(g.conversationState.SQLHistory) > 0 {
		g.conversationState.SQLHistory[len(g.conversationState.SQLHistory)-1].GeneratedSQL = result.SQL
		g.conversationState.CurrentSQL = result.SQL
	}

	// Trace
	if genCtx.TraceCollector != nil {
		genCtx.TraceCollector.Flush(text, genCtx.Intent.IntentType, result.SQL, false)
		result.TraceID = genCtx.TraceCollector.TraceID()
	}

	logx.Infof("[Text2SQL] Template fast path hit (score=%.2f)", score)
	return result
}
