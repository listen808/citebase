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

// SelfEvalConfig Agent自我评估配置
type SelfEvalConfig struct {
	Enabled        bool    `yaml:"enabled" json:"enabled"`
	SampleRate     float64 `yaml:"sample_rate" json:"sample_rate"`             // 评估采样率 (0-1)
	MinConfidence  float64 `yaml:"min_confidence" json:"min_confidence"`       // 最低置信度阈值
	AutoRetryOnLow bool    `yaml:"auto_retry_on_low" json:"auto_retry_on_low"` // 低分自动重试
	MaxRetries     int     `yaml:"max_retries" json:"max_retries"`             // 最大重试次数
}

func DefaultSelfEvalConfig() SelfEvalConfig {
	return SelfEvalConfig{
		Enabled:        true,
		SampleRate:     0.3,
		MinConfidence:  0.6,
		AutoRetryOnLow: true,
		MaxRetries:     2,
	}
}

// EvalDimension 评估维度
type EvalDimension struct {
	Name   string  `json:"name"`
	Score  float64 `json:"score"` // 0-1
	Reason string  `json:"reason"`
}

// SelfEvalResult 自我评估结果
type SelfEvalResult struct {
	Question       string          `json:"question"`
	Answer         string          `json:"answer"`
	Dimensions     []EvalDimension `json:"dimensions"`
	OverallScore   float64         `json:"overall_score"`
	Confidence     float64         `json:"confidence"`
	NeedsRetry     bool            `json:"needs_retry"`
	Suggestions    []string        `json:"suggestions,omitempty"`
	EvalDurationMs int64           `json:"eval_duration_ms"`
	Timestamp      time.Time       `json:"timestamp"`
}

// AgentSelfEvaluator Agent自我评估器
type AgentSelfEvaluator struct {
	provider provider.Provider
	config   SelfEvalConfig
}

func NewAgentSelfEvaluator(p provider.Provider, cfg SelfEvalConfig) *AgentSelfEvaluator {
	return &AgentSelfEvaluator{
		provider: p,
		config:   cfg,
	}
}

// ShouldEvaluate 是否应该评估此请求 (基于采样率)
func (e *AgentSelfEvaluator) ShouldEvaluate() bool {
	if !e.config.Enabled {
		return false
	}
	return time.Now().UnixNano()%100 < int64(e.config.SampleRate*100)
}

// Evaluate 评估RAG回答质量
func (e *AgentSelfEvaluator) Evaluate(ctx context.Context, question, answer string, sources []Source) (*SelfEvalResult, error) {
	if e.provider == nil {
		return nil, fmt.Errorf("self evaluator: provider not initialized")
	}

	startTime := time.Now()
	result := &SelfEvalResult{
		Question:  question,
		Answer:    answer,
		Timestamp: time.Now(),
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// 构建评估上下文
	contextStr := ""
	if len(sources) > 0 {
		snippets := make([]string, 0, len(sources))
		for _, s := range sources {
			snippet := s.Snippet
			if len(snippet) > 200 {
				snippet = snippet[:200] + "..."
			}
			snippets = append(snippets, fmt.Sprintf("[%s] %s", s.DocumentName, snippet))
		}
		contextStr = strings.Join(snippets, "\n")
	}

	prompt := fmt.Sprintf(`评估以下AI回答的质量。从多个维度打分（0-1分）。

用户问题: %s

AI回答: %s

参考文档:
%s

请从以下维度评估，以JSON格式输出：
{
  "dimensions": [
    {"name": "accuracy", "score": 0.8, "reason": "答案准确性说明"},
    {"name": "completeness", "score": 0.7, "reason": "答案完整性说明"},
    {"name": "relevance", "score": 0.9, "reason": "答案相关性说明"},
    {"name": "clarity", "score": 0.85, "reason": "答案清晰度说明"}
  ],
  "overall_score": 0.8,
  "confidence": 0.85,
  "suggestions": ["改进建议1", "改进建议2"]
}

维度说明:
- accuracy: 答案是否准确（基于参考文档）
- completeness: 答案是否完整覆盖了问题
- relevance: 答案是否与问题相关
- clarity: 答案是否清晰易懂

只输出JSON。`, question, answer, contextStr)

	resp, err := e.provider.Chat(ctx, "你是一个严格的AI回答质量评估专家。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(512), provider.WithTemperature(0.1))
	if err != nil {
		return nil, fmt.Errorf("self evaluation LLM call failed: %w", err)
	}

	// 解析评估结果
	evalResult := e.parseEvalResponse(resp.Content)
	if evalResult != nil {
		result.Dimensions = evalResult.Dimensions
		result.OverallScore = evalResult.OverallScore
		result.Confidence = evalResult.Confidence
		result.Suggestions = evalResult.Suggestions
	} else {
		// 回退：使用简单评估
		result.Dimensions = e.simpleEvaluate(question, answer, sources)
		result.OverallScore = e.calculateAverageScore(result.Dimensions)
		result.Confidence = 0.5
	}

	// 判断是否需要重试
	result.NeedsRetry = e.config.AutoRetryOnLow && result.OverallScore < e.config.MinConfidence
	result.EvalDurationMs = time.Since(startTime).Milliseconds()

	logx.Infof("[SelfEval] score=%.2f confidence=%.2f needs_retry=%v duration=%dms",
		result.OverallScore, result.Confidence, result.NeedsRetry, result.EvalDurationMs)

	return result, nil
}

// parseEvalResponse 解析LLM评估响应
func (e *AgentSelfEvaluator) parseEvalResponse(content string) *SelfEvalResult {
	content = strings.TrimSpace(content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return nil
	}

	jsonStr := content[start : end+1]
	var result struct {
		Dimensions   []EvalDimension `json:"dimensions"`
		OverallScore float64         `json:"overall_score"`
		Confidence   float64         `json:"confidence"`
		Suggestions  []string        `json:"suggestions"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil
	}

	return &SelfEvalResult{
		Dimensions:   result.Dimensions,
		OverallScore: result.OverallScore,
		Confidence:   result.Confidence,
		Suggestions:  result.Suggestions,
	}
}

// simpleEvaluate 简单评估（不依赖LLM）
func (e *AgentSelfEvaluator) simpleEvaluate(question, answer string, sources []Source) []EvalDimension {
	var dims []EvalDimension

	// 准确性：基于来源匹配度
	if len(sources) > 0 {
		accuracy := 0.0
		for _, s := range sources {
			accuracy += s.Score
		}
		accuracy /= float64(len(sources))
		if accuracy > 1 {
			accuracy = 1
		}
		dims = append(dims, EvalDimension{Name: "accuracy", Score: accuracy, Reason: "基于来源匹配度"})
	} else {
		dims = append(dims, EvalDimension{Name: "accuracy", Score: 0.3, Reason: "无参考来源"})
	}

	// 完整性：基于回答长度
	completeness := 0.5
	if len(answer) > 200 {
		completeness = 0.8
	} else if len(answer) > 50 {
		completeness = 0.6
	}
	dims = append(dims, EvalDimension{Name: "completeness", Score: completeness, Reason: "基于回答长度"})

	// 相关性：关键词重叠
	questionWords := make(map[string]bool)
	for _, w := range strings.Fields(strings.ToLower(question)) {
		if len(w) > 1 {
			questionWords[w] = true
		}
	}
	overlap := 0
	for _, w := range strings.Fields(strings.ToLower(answer)) {
		if questionWords[w] {
			overlap++
		}
	}
	relevance := 0.5
	if len(questionWords) > 0 {
		relevance = float64(overlap) / float64(len(questionWords))
		if relevance > 1 {
			relevance = 1
		}
	}
	dims = append(dims, EvalDimension{Name: "relevance", Score: relevance, Reason: "基于关键词重叠"})

	// 清晰度：基于句子结构
	clarity := 0.7
	if strings.Contains(answer, "。") || strings.Contains(answer, ".") {
		clarity = 0.8
	}
	dims = append(dims, EvalDimension{Name: "clarity", Score: clarity, Reason: "基于句子结构"})

	return dims
}

// calculateAverageScore 计算平均分
func (e *AgentSelfEvaluator) calculateAverageScore(dims []EvalDimension) float64 {
	if len(dims) == 0 {
		return 0
	}
	total := 0.0
	for _, d := range dims {
		total += d.Score
	}
	return total / float64(len(dims))
}

// ShouldRetry 是否应该重试
func (e *AgentSelfEvaluator) ShouldRetry(result *SelfEvalResult) bool {
	return result != nil && result.NeedsRetry
}

// FormatEvalReport 格式化评估报告
func (e *AgentSelfEvaluator) FormatEvalReport(result *SelfEvalResult) string {
	if result == nil {
		return "无评估结果"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Agent 自我评估报告\n\n"))
	sb.WriteString(fmt.Sprintf("**问题:** %s\n\n", result.Question))
	sb.WriteString(fmt.Sprintf("**总分:** %.2f | **置信度:** %.2f\n\n", result.OverallScore, result.Confidence))

	sb.WriteString("### 各维度评分\n\n")
	for _, dim := range result.Dimensions {
		bar := strings.Repeat("█", int(dim.Score*10))
		empty := strings.Repeat("░", 10-int(dim.Score*10))
		sb.WriteString(fmt.Sprintf("- **%s:** %s %.0f%% - %s\n", dim.Name, bar+empty, dim.Score*100, dim.Reason))
	}

	if len(result.Suggestions) > 0 {
		sb.WriteString("\n### 改进建议\n\n")
		for _, s := range result.Suggestions {
			sb.WriteString(fmt.Sprintf("- %s\n", s))
		}
	}

	if result.NeedsRetry {
		sb.WriteString("\n⚠️ **建议重试** - 回答质量低于阈值\n")
	}

	return sb.String()
}
