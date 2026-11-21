package rag

import (
	"context"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
)

// QueryType 查询类型
type QueryType string

const (
	QueryTypeFactual    QueryType = "factual"    // 事实性查询
	QueryTypeConceptual QueryType = "conceptual" // 概念性查询
	QueryTypeProcedural QueryType = "procedural" // 步骤性查询
	QueryTypeComparison QueryType = "comparison" // 比较性查询
	QueryTypeCreative   QueryType = "creative"   // 创造性查询
)

// AdaptiveRetriever 自适应检索器
type AdaptiveRetriever struct {
	provider   provider.Provider
	classifier *QueryClassifier
}

// NewAdaptiveRetriever 创建自适应检索器
func NewAdaptiveRetriever(p provider.Provider) *AdaptiveRetriever {
	return &AdaptiveRetriever{
		provider:   p,
		classifier: NewQueryClassifier(p),
	}
}

// ClassifyQuery 分类查询
func (ar *AdaptiveRetriever) ClassifyQuery(ctx context.Context, query string) QueryType {
	return ar.classifier.Classify(ctx, query)
}

// QueryClassifier 查询分类器
type QueryClassifier struct {
	provider provider.Provider
}

// NewQueryClassifier 创建查询分类器
func NewQueryClassifier(p provider.Provider) *QueryClassifier {
	return &QueryClassifier{provider: p}
}

// Classify 分类查询
func (qc *QueryClassifier) Classify(ctx context.Context, query string) QueryType {
	if qc.provider == nil {
		return QueryTypeFactual
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	resp, err := qc.provider.Chat(ctx, `将以下查询分类为以下类型之一：
- factual: 事实性查询（什么是、谁是、何时）
- conceptual: 概念性查询（解释、原理、为什么）
- procedural: 步骤性查询（如何、步骤、教程）
- comparison: 比较性查询（对比、区别、优劣）
- creative: 创造性查询（建议、方案、设计）

只输出类型名称。`, []provider.Message{
		provider.NewTextMessage("user", query),
	}, provider.WithMaxTokens(32), provider.WithTemperature(0.1))
	if err != nil {
		return QueryTypeFactual
	}

	switch QueryType(strings.TrimSpace(strings.ToLower(resp.Content))) {
	case QueryTypeConceptual:
		return QueryTypeConceptual
	case QueryTypeProcedural:
		return QueryTypeProcedural
	case QueryTypeComparison:
		return QueryTypeComparison
	case QueryTypeCreative:
		return QueryTypeCreative
	default:
		return QueryTypeFactual
	}
}

// GetStrategyDescription 获取策略描述
func GetStrategyDescription(qt QueryType) string {
	descriptions := map[QueryType]string{
		QueryTypeFactual:    "精确匹配，强调事实准确性",
		QueryTypeConceptual: "扩展相关概念，提供深入解释",
		QueryTypeProcedural: "查找教程类文档，注重步骤清晰",
		QueryTypeComparison: "查找多个相关文档，进行对比分析",
		QueryTypeCreative:   "更宽松的检索，激发创意",
	}
	if desc, ok := descriptions[qt]; ok {
		return desc
	}
	return "默认检索策略"
}

// RetrievalStrategy 检索策略
type RetrievalStrategy string

const (
	StrategyDirectAnswer RetrievalStrategy = "direct"
	StrategyBasicRAG     RetrievalStrategy = "basic_rag"
	StrategyHyDE         RetrievalStrategy = "hyde"
	StrategyMultiHop     RetrievalStrategy = "multi_hop"
	StrategyCorrective   RetrievalStrategy = "corrective"
)

// SelectStrategy 根据查询类型选择检索策略
func (ar *AdaptiveRetriever) SelectStrategy(ctx context.Context, question string) RetrievalStrategy {
	qt := ar.ClassifyQuery(ctx, question)
	switch qt {
	case QueryTypeFactual:
		return StrategyBasicRAG
	case QueryTypeConceptual:
		return StrategyHyDE
	case QueryTypeProcedural:
		return StrategyBasicRAG
	case QueryTypeComparison:
		return StrategyMultiHop
	case QueryTypeCreative:
		return StrategyHyDE
	default:
		return StrategyBasicRAG
	}
}

// NeedsRetrieval 判断是否需要检索
func (ar *AdaptiveRetriever) NeedsRetrieval(ctx context.Context, question string) bool {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	resp, err := ar.provider.Chat(ctx, `判断以下问题是否需要从知识库检索信息才能回答。
- 如果问题是通用常识、问候、简单计算等不需要知识库回答，输出 "no"
- 如果问题需要参考特定文档、数据、流程等知识库内容，输出 "yes"
只输出 "yes" 或 "no"。`, []provider.Message{
		provider.NewTextMessage("user", question),
	}, provider.WithMaxTokens(8), provider.WithTemperature(0.1))
	if err != nil {
		return true
	}
	return strings.TrimSpace(strings.ToLower(resp.Content)) == "yes"
}
