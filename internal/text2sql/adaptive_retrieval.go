package text2sql

import (
	"strings"
)

// RetrievalStrategy 检索策略
type RetrievalStrategy int

const (
	StrategySimple   RetrievalStrategy = iota // 简单问题：直接检索
	StrategyStandard                          // 标准问题：结构化+向量
	StrategyDeep                              // 复杂问题：深度检索+多跳
	StrategyExpert                            // 专家级：穷尽式检索
)

// RetrievalConfig 检索配置
type RetrievalConfig struct {
	Strategy       RetrievalStrategy // 检索策略
	MaxTables      int               // 最大检索表数
	MaxDepth       int               // 最大检索深度
	UseMultiHop    bool              // 是否使用多跳检索
	UseHyDE        bool              // 是否使用HyDE
	TopK           int               // 返回TopK结果
	ScoreThreshold float64           // 分数阈值
}

// DefaultRetrievalConfig 返回默认配置
func DefaultRetrievalConfig() *RetrievalConfig {
	return &RetrievalConfig{
		Strategy:       StrategyStandard,
		MaxTables:      10,
		MaxDepth:       2,
		UseMultiHop:    false,
		UseHyDE:        false,
		TopK:           5,
		ScoreThreshold: 0.3,
	}
}

// AdaptiveRetrievalStrategy 自适应检索策略
// 根据问题复杂度自动选择检索策略
type AdaptiveRetrievalStrategy struct {
	ragAdapter *SQLRAGAdapter
	config     *RetrievalConfig
}

// NewAdaptiveRetrievalStrategy 创建自适应检索策略
func NewAdaptiveRetrievalStrategy(ragAdapter *SQLRAGAdapter) *AdaptiveRetrievalStrategy {
	return &AdaptiveRetrievalStrategy{
		ragAdapter: ragAdapter,
		config:     DefaultRetrievalConfig(),
	}
}

// DetermineStrategy 分析问题复杂度，确定检索策略
func (ars *AdaptiveRetrievalStrategy) DetermineStrategy(question string) *RetrievalConfig {
	config := DefaultRetrievalConfig()

	qLower := strings.ToLower(question)
	words := strings.Fields(qLower)

	// 计算复杂度评分
	complexityScore := ars.calculateComplexityScore(qLower, words)

	// 根据复杂度选择策略
	if complexityScore <= 2 {
		// 简单问题
		config.Strategy = StrategySimple
		config.MaxTables = 3
		config.TopK = 3
		config.ScoreThreshold = 0.5

	} else if complexityScore <= 5 {
		// 标准问题
		config.Strategy = StrategyStandard
		config.MaxTables = 8
		config.TopK = 5

	} else if complexityScore <= 8 {
		// 复杂问题
		config.Strategy = StrategyDeep
		config.MaxTables = 12
		config.MaxDepth = 3
		config.UseMultiHop = true
		config.TopK = 7

	} else {
		// 专家级问题
		config.Strategy = StrategyExpert
		config.MaxTables = 20
		config.MaxDepth = 4
		config.UseMultiHop = true
		config.UseHyDE = true
		config.TopK = 10
	}

	return config
}

// calculateComplexityScore 计算问题复杂度评分
func (ars *AdaptiveRetrievalStrategy) calculateComplexityScore(qLower string, words []string) int {
	score := 0

	// 多表关联指标
	joinIndicators := []string{"和", "以及", "关联", "join", "连接", "结合"}
	for _, indicator := range joinIndicators {
		if strings.Contains(qLower, indicator) {
			score += 2
			break
		}
	}

	// 聚合查询指标
	aggIndicators := []string{"累计", "总计", "平均", "最大", "最小", "排名", "top", "每", "各"}
	for _, indicator := range aggIndicators {
		if strings.Contains(qLower, indicator) {
			score += 1
			break
		}
	}

	// 时间维度指标
	timeIndicators := []string{"每月", "每周", "每年", "季度", "同比", "环比", "趋势"}
	for _, indicator := range timeIndicators {
		if strings.Contains(qLower, indicator) {
			score += 2
			break
		}
	}

	// 嵌套查询指标
	subqueryIndicators := []string{"所有", "满足", "至少", "同时", "而且"}
	for _, indicator := range subqueryIndicators {
		if strings.Contains(qLower, indicator) {
			score += 2
			break
		}
	}

	// 问题长度（越长通常越复杂）
	if len(words) > 15 {
		score += 2
	} else if len(words) > 8 {
		score += 1
	}

	// 包含"多少"、"如何"等复杂疑问词
	complexQuestion := []string{"多少", "如何", "为什么", "哪个", "哪些"}
	for _, indicator := range complexQuestion {
		if strings.Contains(qLower, indicator) {
			score += 1
			break
		}
	}

	return score
}

// RetrieveWithStrategy 根据策略执行检索
func (ars *AdaptiveRetrievalStrategy) RetrieveWithStrategy(
	question string,
	spaceID int,
	config *RetrievalConfig,
) []RAGSchemaSource {
	if config == nil {
		config = ars.DetermineStrategy(question)
	}

	switch config.Strategy {
	case StrategySimple:
		return ars.simpleRetrieve(question, spaceID, config)

	case StrategyStandard:
		return ars.standardRetrieve(question, spaceID, config)

	case StrategyDeep:
		return ars.deepRetrieve(question, spaceID, config)

	case StrategyExpert:
		return ars.expertRetrieve(question, spaceID, config)

	default:
		return ars.standardRetrieve(question, spaceID, config)
	}
}

// simpleRetrieve 简单检索：只检索最相关的几张表
func (ars *AdaptiveRetrievalStrategy) simpleRetrieve(
	question string,
	spaceID int,
	config *RetrievalConfig,
) []RAGSchemaSource {
	// 只用结构化检索，不使用向量
	if ars.ragAdapter == nil {
		return nil
	}

	sources := ars.ragAdapter.structuredRetrieve(question, config.MaxTables)

	// 过滤低分结果
	var filtered []RAGSchemaSource
	for _, src := range sources {
		if src.Score >= config.ScoreThreshold {
			filtered = append(filtered, src)
		}
	}

	return filtered
}

// standardRetrieve 标准检索：结构化+向量融合
func (ars *AdaptiveRetrievalStrategy) standardRetrieve(
	question string,
	spaceID int,
	config *RetrievalConfig,
) []RAGSchemaSource {
	if ars.ragAdapter == nil {
		return nil
	}

	// 使用RRF融合的标准检索
	return ars.ragAdapter.RetrieveRelevantSchemas(question, spaceID, config.TopK)
}

// deepRetrieve 深度检索：包含多跳推理
func (ars *AdaptiveRetrievalStrategy) deepRetrieve(
	question string,
	spaceID int,
	config *RetrievalConfig,
) []RAGSchemaSource {
	if ars.ragAdapter == nil {
		return nil
	}

	// 首先进行标准检索
	sources := ars.ragAdapter.RetrieveRelevantSchemas(question, spaceID, config.TopK)

	// 如果检测到多表关联，进行二跳检索
	if config.UseMultiHop && len(sources) > 1 {
		sources = ars.expandWithMultiHop(question, spaceID, sources, config)
	}

	return sources
}

// expertRetrieve 专家级检索：穷尽式检索+HyDE
func (ars *AdaptiveRetrievalStrategy) expertRetrieve(
	question string,
	spaceID int,
	config *RetrievalConfig,
) []RAGSchemaSource {
	if ars.ragAdapter == nil {
		return nil
	}

	// 扩大检索范围
	sources := ars.ragAdapter.RetrieveRelevantSchemas(question, spaceID, config.MaxTables)

	// 使用HyDE扩展检索
	if config.UseHyDE {
		sources = ars.expandWithHyDE(question, spaceID, sources, config)
	}

	// 多跳扩展
	if config.UseMultiHop {
		sources = ars.expandWithMultiHop(question, spaceID, sources, config)
	}

	// 过滤并返回TopK
	if len(sources) > config.TopK {
		sources = sources[:config.TopK]
	}

	return sources
}

// expandWithMultiHop 使用多跳推理扩展检索结果
func (ars *AdaptiveRetrievalStrategy) expandWithMultiHop(
	question string,
	spaceID int,
	sources []RAGSchemaSource,
	config *RetrievalConfig,
) []RAGSchemaSource {
	// 检测是否需要多跳
	if !ars.requiresMultiHop(question) {
		return sources
	}

	// 收集已检索到的表
	seenTables := make(map[string]bool)
	for _, src := range sources {
		seenTables[strings.ToLower(src.TableName)] = true
	}

	// 二跳检索：查找与已检索表相关的表
	hopKeywords := ars.extractHopKeywords(question)
	var expandedSources []RAGSchemaSource
	expandedSources = append(expandedSources, sources...)

	for _, keyword := range hopKeywords {
		hopQuery := keyword + " " + question
		hopSources := ars.ragAdapter.structuredRetrieve(hopQuery, config.MaxTables/2)

		for _, src := range hopSources {
			if !seenTables[strings.ToLower(src.TableName)] {
				seenTables[strings.ToLower(src.TableName)] = true
				// 降低跳结果的分数
				src.Score *= 0.8
				expandedSources = append(expandedSources, src)
			}
		}
	}

	return expandedSources
}

// expandWithHyDE 使用HyDE扩展检索
func (ars *AdaptiveRetrievalStrategy) expandWithHyDE(
	question string,
	spaceID int,
	sources []RAGSchemaSource,
	config *RetrievalConfig,
) []RAGSchemaSource {
	// 注意：这里只是用问题扩展检索，不调用实际的HyDE模型
	// 实际HyDE需要LLM生成假设文档

	// 简化实现：用问题分解扩展检索
	expandedSources := sources
	subQuestions := ars.decomposeQuestion(question)

	for _, subQ := range subQuestions {
		subSources := ars.ragAdapter.structuredRetrieve(subQ, config.MaxTables/2)
		seenTables := make(map[string]bool)
		for _, src := range expandedSources {
			seenTables[strings.ToLower(src.TableName)] = true
		}

		for _, src := range subSources {
			if !seenTables[strings.ToLower(src.TableName)] {
				seenTables[strings.ToLower(src.TableName)] = true
				src.Score *= 0.7 // 降低分解问题的结果分数
				expandedSources = append(expandedSources, src)
			}
		}
	}

	return expandedSources
}

// requiresMultiHop 检测是否需要多跳检索
func (ars *AdaptiveRetrievalStrategy) requiresMultiHop(question string) bool {
	qLower := strings.ToLower(question)

	// 多跳关键词
	multiHopIndicators := []string{
		"和", "以及", "关联", "结合",
		"都", "全部", "所有",
		"每个", "各",
	}

	count := 0
	for _, indicator := range multiHopIndicators {
		if strings.Contains(qLower, indicator) {
			count++
		}
	}

	return count >= 2
}

// extractHopKeywords 提取用于多跳检索的关键词
func (ars *AdaptiveRetrievalStrategy) extractHopKeywords(question string) []string {
	var keywords []string
	qLower := strings.ToLower(question)

	// 提取名词作为候选关键词
	words := strings.Fields(qLower)
	for _, word := range words {
		if len(word) > 2 && !isStopWord(word) {
			keywords = append(keywords, word)
		}
	}

	// 只返回前3个关键词
	if len(keywords) > 3 {
		keywords = keywords[:3]
	}

	return keywords
}

// decomposeQuestion 将问题分解为子问题
func (ars *AdaptiveRetrievalStrategy) decomposeQuestion(question string) []string {
	var subQuestions []string

	// 按常见分隔符分解
	if strings.Contains(question, "和") {
		parts := strings.Split(question, "和")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				subQuestions = append(subQuestions, part)
			}
		}
	}

	if strings.Contains(question, "以及") {
		parts := strings.Split(question, "以及")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				subQuestions = append(subQuestions, part)
			}
		}
	}

	// 如果没有分解成功，返回空
	if len(subQuestions) < 2 {
		return nil
	}

	return subQuestions
}

// isStopWord 判断是否为停用词
func isStopWord(word string) bool {
	stopWords := map[string]bool{
		"的": true, "了": true, "是": true, "在": true, "和": true,
		"我": true, "你": true, "他": true, "她": true, "它": true,
		"这": true, "那": true, "有": true, "个": true, "吗": true,
		"呢": true, "吧": true, "啊": true, "哦": true, "嗯": true,
	}
	return stopWords[word]
}
