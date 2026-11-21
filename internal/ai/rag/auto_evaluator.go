package rag

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai/provider"
)

// AutoEvaluator 自动化RAG评估调度器
// 定期对RAG系统进行自动化质量评估，生成评估报告和优化建议
type AutoEvaluator struct {
	provider  provider.Provider
	retriever *Retriever
	evaluator *RAGASEvaluator
	results   []*AutoEvalResult
	mu        sync.RWMutex

	// 配置
	enabled        bool
	evalInterval   time.Duration // 评估间隔
	maxTestQueries int           // 每次评估的最大测试查询数
	minAcceptScore float64       // 最低可接受分数

	// 统计
	totalEvals     int
	avgScore       float64
	lastEvalTime   time.Time
	trendDirection string // "improving", "stable", "declining"
}

// AutoEvalResult 自动化评估结果
type AutoEvalResult struct {
	Timestamp    time.Time          `json:"timestamp"`
	TotalQueries int                `json:"total_queries"`
	AvgScore     float64            `json:"avg_score"`
	Metrics      AggregatedMetrics  `json:"metrics"`
	Issues       []EvalIssue        `json:"issues"`
	Suggestions  []string           `json:"suggestions"`
	Details      []*RAGASEvalResult `json:"details"`
	Duration     string             `json:"duration"`
}

// AggregatedMetrics 聚合指标
type AggregatedMetrics struct {
	AvgFaithfulness     float64 `json:"avg_faithfulness"`
	AvgAnswerRelevancy  float64 `json:"avg_answer_relevancy"`
	AvgContextPrecision float64 `json:"avg_context_precision"`
	AvgContextRecall    float64 `json:"avg_context_recall"`
	PassRate            float64 `json:"pass_rate"` // score >= 0.7的占比
}

// EvalIssue 评估发现的问题
type EvalIssue struct {
	Severity   string  `json:"severity"` // "critical", "warning", "info"
	Category   string  `json:"category"`
	Query      string  `json:"query"`
	Score      float64 `json:"score"`
	Detail     string  `json:"detail"`
	Suggestion string  `json:"suggestion"`
}

// AutoEvalConfig 自动评估配置
type AutoEvalConfig struct {
	Interval       time.Duration
	MaxTestQueries int
	MinAcceptScore float64
}

// DefaultAutoEvalConfig 默认配置
func DefaultAutoEvalConfig() AutoEvalConfig {
	return AutoEvalConfig{
		Interval:       24 * time.Hour,
		MaxTestQueries: 20,
		MinAcceptScore: 0.7,
	}
}

// NewAutoEvaluator 创建自动评估器
func NewAutoEvaluator(p provider.Provider, r *Retriever, eval *RAGASEvaluator, cfg AutoEvalConfig) *AutoEvaluator {
	return &AutoEvaluator{
		provider:       p,
		retriever:      r,
		evaluator:      eval,
		enabled:        true,
		evalInterval:   cfg.Interval,
		maxTestQueries: cfg.MaxTestQueries,
		minAcceptScore: cfg.MinAcceptScore,
	}
}

// RunEvaluation 执行一次自动化评估
func (ae *AutoEvaluator) RunEvaluation(ctx context.Context, testQueries []TestQuery) (*AutoEvalResult, error) {
	if !ae.enabled {
		return nil, fmt.Errorf("auto evaluator is disabled")
	}

	startTime := time.Now()
	result := &AutoEvalResult{
		Timestamp: time.Now(),
	}

	// 限制查询数量
	if len(testQueries) > ae.maxTestQueries {
		testQueries = testQueries[:ae.maxTestQueries]
	}
	result.TotalQueries = len(testQueries)

	if len(testQueries) == 0 {
		return result, nil
	}

	// 对每个测试查询进行RAG检索和评估
	var (
		totalFaithfulness float64
		totalRelevancy    float64
		totalPrecision    float64
		totalRecall       float64
		passCount         int
	)

	for i, tq := range testQueries {
		logx.Infof("[AutoEval] Processing query %d/%d: %s", i+1, len(testQueries), tq.Question)

		// 1. 执行RAG检索
		sources, contextText := ae.retriever.RetrieveContextWithRAG(ctx, tq.Question, tq.SpaceID, 5)

		// 2. 生成答案
		var answer string
		if len(sources) > 0 {
			answerResult, err := ae.retriever.Ask(ctx, tq.Question, tq.SpaceID, 5)
			if err == nil {
				answer = answerResult.Answer
			}
		}

		if answer == "" {
			answer = "无法回答"
		}

		// 3. 收集上下文
		contexts := make([]string, 0, len(sources))
		for _, s := range sources {
			contexts = append(contexts, s.Snippet)
		}
		if contextText != "" {
			contexts = append(contexts, contextText)
		}

		// 4. RAGAS评估
		evalResult, err := ae.evaluator.Evaluate(tq.Question, answer, contexts, tq.ExpectedAnswer)
		if err != nil {
			logx.Infof("[AutoEval] Eval error for query %q: %v", tq.Question, err)
			continue
		}

		result.Details = append(result.Details, evalResult)

		// 累加指标
		totalFaithfulness += evalResult.Metrics.Faithfulness
		totalRelevancy += evalResult.Metrics.AnswerRelevancy
		totalPrecision += evalResult.Metrics.ContextPrecision
		totalRecall += evalResult.Metrics.ContextRecall

		if evalResult.AvgScore >= ae.minAcceptScore {
			passCount++
		}

		// 收集低分问题
		if evalResult.AvgScore < ae.minAcceptScore {
			result.Issues = append(result.Issues, EvalIssue{
				Severity:   severityFromScore(evalResult.AvgScore),
				Category:   identifyIssueCategory(evalResult),
				Query:      tq.Question,
				Score:      evalResult.AvgScore,
				Detail:     fmt.Sprintf("综合评分 %.2f (F:%.2f R:%.2f P:%.2f)", evalResult.AvgScore, evalResult.Metrics.Faithfulness, evalResult.Metrics.AnswerRelevancy, evalResult.Metrics.ContextPrecision),
				Suggestion: generateSuggestion(evalResult),
			})
		}
	}

	// 计算聚合指标
	n := float64(len(result.Details))
	if n > 0 {
		result.Metrics = AggregatedMetrics{
			AvgFaithfulness:     math.Round(totalFaithfulness/n*1000) / 1000,
			AvgAnswerRelevancy:  math.Round(totalRelevancy/n*1000) / 1000,
			AvgContextPrecision: math.Round(totalPrecision/n*1000) / 1000,
			AvgContextRecall:    math.Round(totalRecall/n*1000) / 1000,
			PassRate:            math.Round(float64(passCount)/n*1000) / 1000,
		}
		result.AvgScore = math.Round((totalFaithfulness+totalRelevancy+totalPrecision+totalRecall)/(n*4)*1000) / 1000
	}

	// 生成建议
	result.Suggestions = ae.generateSuggestions(result)

	// 更新统计
	ae.mu.Lock()
	ae.totalEvals++
	ae.avgScore = result.AvgScore
	ae.lastEvalTime = time.Now()
	ae.results = append(ae.results, result)
	if len(ae.results) > 50 {
		ae.results = ae.results[1:]
	}
	ae.updateTrend()
	ae.mu.Unlock()

	result.Duration = time.Since(startTime).Round(time.Millisecond).String()
	logx.Infof("[AutoEval] Evaluation complete: %d queries, avg score %.3f, %d issues, duration %s",
		result.TotalQueries, result.AvgScore, len(result.Issues), result.Duration)

	return result, nil
}

// generateSuggestions 根据评估结果生成优化建议
func (ae *AutoEvaluator) generateSuggestions(result *AutoEvalResult) []string {
	var suggestions []string

	if result.Metrics.AvgFaithfulness < 0.7 {
		suggestions = append(suggestions, "Faithfulness(忠实度)偏低，建议：1)增强引用溯源机制 2)添加事实性校验步骤 3)减少LLM温度参数")
	}
	if result.Metrics.AvgAnswerRelevancy < 0.7 {
		suggestions = append(suggestions, "AnswerRelevancy(答案相关性)偏低，建议：1)优化查询重写策略 2)提升HyDE检索权重 3)增加结果重排序精度")
	}
	if result.Metrics.AvgContextPrecision < 0.7 {
		suggestions = append(suggestions, "ContextPrecision(上下文精度)偏低，建议：1)启用语义分块策略 2)调整检索top-K值 3)优化混合检索权重")
	}
	if result.Metrics.AvgContextRecall < 0.7 {
		suggestions = append(suggestions, "ContextRecall(上下文召回)偏低，建议：1)增加候选文档数量 2)启用多跳推理 3)扩展查询改写范围")
	}
	if result.Metrics.PassRate < 0.8 {
		suggestions = append(suggestions, fmt.Sprintf("整体通过率偏低(%.0f%%)，建议进行系统性RAG优化", result.Metrics.PassRate*100))
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, "当前RAG系统各项指标良好，无需重大调整")
	}

	return suggestions
}

// updateTrend 更新趋势方向
func (ae *AutoEvaluator) updateTrend() {
	if len(ae.results) < 3 {
		ae.trendDirection = "stable"
		return
	}
	// 比较最近3次评估的平均分
	recent := ae.results[len(ae.results)-3:]
	first := recent[0].AvgScore
	last := recent[len(recent)-1].AvgScore

	if last > first+0.05 {
		ae.trendDirection = "improving"
	} else if last < first-0.05 {
		ae.trendDirection = "declining"
	} else {
		ae.trendDirection = "stable"
	}
}

// GetStats 获取评估统计
func (ae *AutoEvaluator) GetStats() map[string]interface{} {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	return map[string]interface{}{
		"enabled":        ae.enabled,
		"total_evals":    ae.totalEvals,
		"avg_score":      ae.avgScore,
		"last_eval_time": ae.lastEvalTime,
		"trend":          ae.trendDirection,
		"interval":       ae.evalInterval.String(),
	}
}

// GetLastResult 获取最近一次评估结果
func (ae *AutoEvaluator) GetLastResult() *AutoEvalResult {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	if len(ae.results) == 0 {
		return nil
	}
	return ae.results[len(ae.results)-1]
}

// TestQuery 测试查询定义
type TestQuery struct {
	Question       string `json:"question"`
	ExpectedAnswer string `json:"expected_answer"`
	SpaceID        int    `json:"space_id"`
	Category       string `json:"category"`
}

// DefaultTestQueries 获取默认测试查询集
func DefaultTestQueries() []TestQuery {
	return []TestQuery{
		{
			Question:       "如何创建新文档？",
			ExpectedAnswer: "可以通过文档编辑页面创建新文档，选择目标空间后填写标题和内容",
			Category:       "操作指南",
		},
		{
			Question:       "系统支持哪些文件格式？",
			ExpectedAnswer: "系统支持Markdown、PDF、Word、Excel等多种格式",
			Category:       "功能说明",
		},
		{
			Question:       "如何配置AI功能？",
			ExpectedAnswer: "在管理后台的AI配置页面可以配置AI Provider和API Key",
			Category:       "配置管理",
		},
		{
			Question:       "用户权限如何管理？",
			ExpectedAnswer: "管理员可以在角色管理页面配置用户角色和权限",
			Category:       "权限管理",
		},
		{
			Question:       "如何进行知识库搜索？",
			ExpectedAnswer: "在搜索栏输入关键词可以搜索知识库中的文档",
			Category:       "功能说明",
		},
	}
}

// severityFromScore 根据评分确定严重程度
func severityFromScore(score float64) string {
	if score < 0.4 {
		return "critical"
	} else if score < 0.7 {
		return "warning"
	}
	return "info"
}

// identifyIssueCategory 识别问题类别
func identifyIssueCategory(result *RAGASEvalResult) string {
	m := result.Metrics
	lowest := m.Faithfulness
	category := "faithfulness"
	if m.AnswerRelevancy < lowest {
		lowest = m.AnswerRelevancy
		category = "relevancy"
	}
	if m.ContextPrecision < lowest {
		lowest = m.ContextPrecision
		category = "precision"
	}
	if m.ContextRecall < lowest {
		category = "recall"
	}
	return category
}

// generateSuggestion 为单个评估结果生成建议
func generateSuggestion(result *RAGASEvalResult) string {
	m := result.Metrics
	if m.Faithfulness < 0.5 {
		return "答案可能包含与文档不符的信息，建议增强引用验证"
	}
	if m.AnswerRelevancy < 0.5 {
		return "答案与问题相关性不足，建议优化检索策略"
	}
	if m.ContextPrecision < 0.5 {
		return "检索结果包含不相关内容，建议调整检索参数"
	}
	if m.ContextRecall < 0.5 {
		return "检索遗漏了重要信息，建议扩展检索范围"
	}
	return "优化检索和生成策略以提升评分"
}
