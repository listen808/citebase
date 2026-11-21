package rag

import (
	"context"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
)

// DeepEvalConfig DeepEval评估配置
type DeepEvalConfig struct {
	Enabled        bool    `yaml:"enabled" json:"enabled"`
	SampleSize     int     `yaml:"sample_size" json:"sample_size"`         // 评估采样数
	MinThreshold   float64 `yaml:"min_threshold" json:"min_threshold"`     // 最低质量阈值
	EmbeddingModel string  `yaml:"embedding_model" json:"embedding_model"` // 用于评估的嵌入模型
}

func DefaultDeepEvalConfig() DeepEvalConfig {
	return DeepEvalConfig{
		Enabled:      true,
		SampleSize:   50,
		MinThreshold: 0.6,
	}
}

// EvalMetric 评估指标
type EvalMetric struct {
	Name      string  `json:"name"`
	Score     float64 `json:"score"`             // 0-1
	Threshold float64 `json:"threshold"`         // 通过阈值
	Passed    bool    `json:"passed"`            // 是否通过
	Reason    string  `json:"reason"`            // 评估原因
	Details   string  `json:"details,omitempty"` // 详细信息
}

// EvalResult 评估结果
type EvalResult struct {
	Question       string       `json:"question"`
	Answer         string       `json:"answer"`
	Contexts       []string     `json:"contexts"`
	Metrics        []EvalMetric `json:"metrics"`
	OverallScore   float64      `json:"overall_score"`
	OverallPassed  bool         `json:"overall_passed"`
	EvalDurationMs int64        `json:"eval_duration_ms"`
	Timestamp      time.Time    `json:"timestamp"`
}

// EvalReport 评估报告
type EvalReport struct {
	TotalSamples   int                `json:"total_samples"`
	PassedSamples  int                `json:"passed_samples"`
	FailedSamples  int                `json:"failed_samples"`
	PassRate       float64            `json:"pass_rate"`
	AvgScores      map[string]float64 `json:"avg_scores"`       // 各指标平均分
	MetricResults  []EvalMetric       `json:"metric_results"`   // 各指标汇总
	EvalResults    []EvalResult       `json:"eval_results"`     // 详细结果
	EvalDurationMs int64              `json:"eval_duration_ms"` // 评估耗时
	GeneratedAt    time.Time          `json:"generated_at"`
}

// DeepEvalEvaluator DeepEval评估器
type DeepEvalEvaluator struct {
	provider provider.Provider
	config   DeepEvalConfig
	rag      *Retriever // 引用RAG检索器用于评估
}

func NewDeepEvalEvaluator(p provider.Provider, cfg DeepEvalConfig, r *Retriever) *DeepEvalEvaluator {
	return &DeepEvalEvaluator{
		provider: p,
		config:   cfg,
		rag:      r,
	}
}

// EvalDataset 评估数据集
type EvalDataset struct {
	Samples []EvalSample `json:"samples"`
}

type EvalSample struct {
	Question string                 `json:"question"`
	Expected string                 `json:"expected"` // 期望答案（可选）
	Contexts []string               `json:"contexts"` // 期望的上下文（可选）
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Evaluate 执行评估
func (e *DeepEvalEvaluator) Evaluate(ctx context.Context, dataset *EvalDataset) (*EvalReport, error) {
	if !e.config.Enabled {
		return nil, fmt.Errorf("DeepEval evaluator is disabled")
	}

	startTime := time.Now()
	report := &EvalReport{
		TotalSamples: len(dataset.Samples),
		AvgScores:    make(map[string]float64),
		GeneratedAt:  time.Now(),
	}

	metricCounts := make(map[string]float64)
	metricSums := make(map[string]float64)

	// 限制采样数
	samples := dataset.Samples
	if len(samples) > e.config.SampleSize {
		samples = samples[:e.config.SampleSize]
	}

	for i, sample := range samples {
		logx.Infof("[DeepEval] Evaluating sample %d/%d: %s", i+1, len(samples), truncateText(sample.Question, 50))

		result, err := e.evaluateSample(ctx, sample)
		if err != nil {
			logx.Infof("[DeepEval] Failed to evaluate sample %d: %v", i, err)
			continue
		}

		report.EvalResults = append(report.EvalResults, *result)

		if result.OverallPassed {
			report.PassedSamples++
		} else {
			report.FailedSamples++
		}

		// 累计指标
		for _, metric := range result.Metrics {
			metricSums[metric.Name] += metric.Score
			metricCounts[metric.Name]++
		}
	}

	// 计算平均分
	for name, sum := range metricSums {
		report.AvgScores[name] = sum / metricCounts[name]
	}

	// 生成指标汇总
	for name, avg := range report.AvgScores {
		report.MetricResults = append(report.MetricResults, EvalMetric{
			Name:      name,
			Score:     avg,
			Threshold: e.config.MinThreshold,
			Passed:    avg >= e.config.MinThreshold,
		})
	}

	if report.TotalSamples > 0 {
		report.PassRate = float64(report.PassedSamples) / float64(report.TotalSamples)
	}

	report.EvalDurationMs = time.Since(startTime).Milliseconds()

	logx.Infof("[DeepEval] Evaluation complete: %d/%d passed (%.1f%%), duration: %dms",
		report.PassedSamples, report.TotalSamples, report.PassRate*100, report.EvalDurationMs)

	return report, nil
}

// evaluateSample 评估单个样本
func (e *DeepEvalEvaluator) evaluateSample(ctx context.Context, sample EvalSample) (*EvalResult, error) {
	startTime := time.Now()

	result := &EvalResult{
		Question:  sample.Question,
		Timestamp: time.Now(),
	}

	// 1. 执行RAG检索获取上下文
	if e.rag != nil {
		sources, _ := e.rag.hybridRetrieve(ctx, sample.Question, sample.Question, 0, 5)
		for _, src := range sources {
			result.Contexts = append(result.Contexts, src.Snippet)
		}
	}

	// 2. 生成答案
	if e.provider != nil && len(result.Contexts) > 0 {
		contextStr := strings.Join(result.Contexts, "\n\n")
		prompt := fmt.Sprintf("基于以下上下文回答问题。\n\n上下文：\n%s\n\n问题：%s\n\n回答：", contextStr, sample.Question)

		resp, err := e.provider.Chat(ctx, "你是一个知识问答助手。", []provider.Message{
			provider.NewTextMessage("user", prompt),
		}, provider.WithMaxTokens(512), provider.WithTemperature(0.3))
		if err == nil {
			result.Answer = resp.Content
		}
	}

	// 3. 计算各评估指标
	result.Metrics = e.computeMetrics(ctx, sample, result)

	// 4. 计算总分
	if len(result.Metrics) > 0 {
		totalScore := 0.0
		allPassed := true
		for _, m := range result.Metrics {
			totalScore += m.Score
			if !m.Passed {
				allPassed = false
			}
		}
		result.OverallScore = totalScore / float64(len(result.Metrics))
		result.OverallPassed = allPassed && result.OverallScore >= e.config.MinThreshold
	}

	result.EvalDurationMs = time.Since(startTime).Milliseconds()
	return result, nil
}

// computeMetrics 计算评估指标
func (e *DeepEvalEvaluator) computeMetrics(ctx context.Context, sample EvalSample, result *EvalResult) []EvalMetric {
	var metrics []EvalMetric

	// 1. Faithfulness (忠实度) - 答案是否基于上下文
	faithfulness := e.evaluateFaithfulness(ctx, sample, result)
	metrics = append(metrics, faithfulness)

	// 2. Answer Relevancy (答案相关性) - 答案是否回答了问题
	answerRelevancy := e.evaluateAnswerRelevancy(ctx, sample, result)
	metrics = append(metrics, answerRelevancy)

	// 3. Context Precision (上下文精确度) - 检索的上下文是否精确
	contextPrecision := e.evaluateContextPrecision(ctx, sample, result)
	metrics = append(metrics, contextPrecision)

	// 4. Context Recall (上下文召回率) - 是否检索到了所有相关信息
	contextRecall := e.evaluateContextRecall(ctx, sample, result)
	metrics = append(metrics, contextRecall)

	return metrics
}

// evaluateFaithfulness 评估忠实度
func (e *DeepEvalEvaluator) evaluateFaithfulness(ctx context.Context, sample EvalSample, result *EvalResult) EvalMetric {
	metric := EvalMetric{
		Name:      "faithfulness",
		Threshold: e.config.MinThreshold,
	}

	if result.Answer == "" || len(result.Contexts) == 0 {
		metric.Score = 0
		metric.Reason = "No answer or context available"
		metric.Passed = false
		return metric
	}

	if e.provider == nil {
		// 简单的基于关键词的忠实度评估
		metric.Score = e.simpleFaithfulnessScore(sample, result)
		metric.Reason = "Simple keyword-based evaluation"
		metric.Passed = metric.Score >= metric.Threshold
		return metric
	}

	// 使用LLM评估忠实度
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	contextStr := strings.Join(result.Contexts, "\n\n")
	prompt := fmt.Sprintf(`评估以下答案是否忠实于给定的上下文。

上下文：
%s

问题：%s

答案：%s

请评估答案是否仅基于上下文内容，没有添加上下文之外的信息。
评分0-1，1表示完全忠实。
请以JSON格式输出：{"score": 0.8, "reason": "原因"}`,
		contextStr, sample.Question, result.Answer)

	resp, err := e.provider.Chat(ctx, "你是一个严格的评估专家。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(256), provider.WithTemperature(0.1))
	if err != nil {
		metric.Score = e.simpleFaithfulnessScore(sample, result)
		metric.Reason = fmt.Sprintf("LLM evaluation failed: %v", err)
	} else {
		score, reason := parseScoreFromLLM(resp.Content)
		metric.Score = score
		metric.Reason = reason
	}

	metric.Passed = metric.Score >= metric.Threshold
	return metric
}

// evaluateAnswerRelevancy 评估答案相关性
func (e *DeepEvalEvaluator) evaluateAnswerRelevancy(ctx context.Context, sample EvalSample, result *EvalResult) EvalMetric {
	metric := EvalMetric{
		Name:      "answer_relevancy",
		Threshold: e.config.MinThreshold,
	}

	if result.Answer == "" {
		metric.Score = 0
		metric.Reason = "No answer generated"
		metric.Passed = false
		return metric
	}

	if e.provider == nil {
		metric.Score = e.simpleRelevancyScore(sample, result)
		metric.Reason = "Simple keyword-based evaluation"
		metric.Passed = metric.Score >= metric.Threshold
		return metric
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	prompt := fmt.Sprintf(`评估以下答案与问题的相关性。

问题：%s

答案：%s

评分0-1，1表示完全相关。
请以JSON格式输出：{"score": 0.8, "reason": "原因"}`,
		sample.Question, result.Answer)

	resp, err := e.provider.Chat(ctx, "你是一个严格的评估专家。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(256), provider.WithTemperature(0.1))
	if err != nil {
		metric.Score = e.simpleRelevancyScore(sample, result)
		metric.Reason = fmt.Sprintf("LLM evaluation failed: %v", err)
	} else {
		score, reason := parseScoreFromLLM(resp.Content)
		metric.Score = score
		metric.Reason = reason
	}

	metric.Passed = metric.Score >= metric.Threshold
	return metric
}

// evaluateContextPrecision 评估上下文精确度
func (e *DeepEvalEvaluator) evaluateContextPrecision(ctx context.Context, sample EvalSample, result *EvalResult) EvalMetric {
	metric := EvalMetric{
		Name:      "context_precision",
		Threshold: e.config.MinThreshold,
	}

	if len(result.Contexts) == 0 {
		metric.Score = 0
		metric.Reason = "No contexts retrieved"
		metric.Passed = false
		return metric
	}

	// 计算上下文与问题的语义相关度
	questionWords := make(map[string]bool)
	for _, word := range strings.Fields(strings.ToLower(sample.Question)) {
		questionWords[word] = true
	}

	relevantCount := 0
	for _, ctx := range result.Contexts {
		ctxWords := make(map[string]bool)
		for _, word := range strings.Fields(strings.ToLower(ctx)) {
			ctxWords[word] = true
		}
		overlap := 0
		for word := range questionWords {
			if ctxWords[word] {
				overlap++
			}
		}
		if overlap > 0 {
			relevantCount++
		}
	}

	metric.Score = float64(relevantCount) / float64(len(result.Contexts))
	metric.Reason = fmt.Sprintf("%d/%d contexts are relevant", relevantCount, len(result.Contexts))
	metric.Passed = metric.Score >= metric.Threshold
	return metric
}

// evaluateContextRecall 评估上下文召回率
func (e *DeepEvalEvaluator) evaluateContextRecall(ctx context.Context, sample EvalSample, result *EvalResult) EvalMetric {
	metric := EvalMetric{
		Name:      "context_recall",
		Threshold: e.config.MinThreshold,
	}

	if len(result.Contexts) == 0 {
		metric.Score = 0
		metric.Reason = "No contexts retrieved"
		metric.Passed = false
		return metric
	}

	// 如果有期望上下文，计算召回率
	if len(sample.Contexts) > 0 {
		found := 0
		for _, expected := range sample.Contexts {
			for _, actual := range result.Contexts {
				if strings.Contains(actual, expected) || strings.Contains(expected, actual) {
					found++
					break
				}
			}
		}
		metric.Score = float64(found) / float64(len(sample.Contexts))
		metric.Reason = fmt.Sprintf("Found %d/%d expected contexts", found, len(sample.Contexts))
	} else {
		// P0-3：旧实现在无期望上下文时直接给 0.7 —— 一个凭空的"默认分数"会混入
		// 聚合指标，让通过率虚高。没有比对基准就是无法评估，按 0 计并在 Reason 里写明。
		if result.Answer != "" && len(result.Contexts) > 0 {
			metric.Score = 0
			metric.Reason = "No expected contexts to compare; cannot measure recall (scored 0, not 0.7)"
		} else {
			metric.Score = 0
			metric.Reason = "No answer or contexts"
		}
	}

	metric.Passed = metric.Score >= metric.Threshold
	return metric
}

// simpleFaithfulnessScore 简单的忠实度评分（基于关键词匹配）
func (e *DeepEvalEvaluator) simpleFaithfulnessScore(sample EvalSample, result *EvalResult) float64 {
	if result.Answer == "" || len(result.Contexts) == 0 {
		return 0
	}

	contextStr := strings.ToLower(strings.Join(result.Contexts, " "))
	answerWords := strings.Fields(strings.ToLower(result.Answer))

	contained := 0
	for _, word := range answerWords {
		if len(word) > 1 && strings.Contains(contextStr, word) {
			contained++
		}
	}

	if len(answerWords) == 0 {
		return 0
	}
	return float64(contained) / float64(len(answerWords))
}

// simpleRelevancyScore 简单的相关性评分
func (e *DeepEvalEvaluator) simpleRelevancyScore(sample EvalSample, result *EvalResult) float64 {
	if result.Answer == "" {
		return 0
	}

	questionWords := make(map[string]bool)
	for _, word := range strings.Fields(strings.ToLower(sample.Question)) {
		if len(word) > 1 {
			questionWords[word] = true
		}
	}

	answerWords := strings.Fields(strings.ToLower(result.Answer))
	overlap := 0
	for _, word := range answerWords {
		if questionWords[word] {
			overlap++
		}
	}

	if len(questionWords) == 0 {
		return 0
	}
	score := float64(overlap) / float64(len(questionWords))
	if score > 1 {
		score = 1
	}
	return score
}

func parseScoreFromLLM(content string) (float64, string) {
	content = strings.TrimSpace(content)

	// 尝试解析JSON
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		jsonStr := content[start : end+1]
		var result struct {
			Score  float64 `json:"score"`
			Reason string  `json:"reason"`
		}
		if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
			if result.Score < 0 {
				result.Score = 0
			}
			if result.Score > 1 {
				result.Score = 1
			}
			return result.Score, result.Reason
		}
	}

	// 回退：从文本中提取分数
	if strings.Contains(content, "1.0") || strings.Contains(content, "1,0") {
		return 1.0, content
	}
	if strings.Contains(content, "0.0") || strings.Contains(content, "0,0") {
		return 0.0, content
	}

	return 0.5, content
}
