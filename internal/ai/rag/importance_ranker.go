package rag

import (
	"context"
	"strings"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai/provider"
)

// ImportanceRanker 内容重要性排序器
// 基于问题相关性、位置、密度等多维度评估内容块的重要性
type ImportanceRanker struct {
	provider provider.Provider
}

// NewImportanceRanker 创建重要性排序器
func NewImportanceRanker(p provider.Provider) *ImportanceRanker {
	return &ImportanceRanker{provider: p}
}

// EvaluateImportance 评估单个内容块的重要性
// question: 用户问题
// content: 待评估的内容块
// 返回: 重要性评分 0-1
func (r *ImportanceRanker) EvaluateImportance(ctx context.Context, question, content string) float64 {
	if r.provider == nil {
		// 无 LLM 时使用启发式评分
		return r.heuristicScore(question, content)
	}

	// 使用 LLM 进行语义相关性评分
	return r.llmScore(ctx, question, content)
}

// heuristicScore 使用启发式算法评估重要性
func (r *ImportanceRanker) heuristicScore(question, content string) float64 {
	score := 0.5 // 基础分

	questionLower := strings.ToLower(question)
	contentLower := strings.ToLower(content)

	// 1. 关键词匹配评分
	keywords := extractQuestionKeywords(questionLower)
	matchCount := 0
	for _, kw := range keywords {
		if strings.Contains(contentLower, kw) {
			matchCount++
		}
	}
	if len(keywords) > 0 {
		score += float64(matchCount) / float64(len(keywords)) * 0.3
	}

	// 2. 问句类型匹配
	if strings.Contains(questionLower, "how") || strings.Contains(questionLower, "如何") {
		if strings.Contains(contentLower, "step") || strings.Contains(contentLower, "步骤") || strings.Contains(contentLower, "first") {
			score += 0.1
		}
	}
	if strings.Contains(questionLower, "why") || strings.Contains(questionLower, "为什么") {
		if strings.Contains(contentLower, "because") || strings.Contains(contentLower, "原因") || strings.Contains(contentLower, "reason") {
			score += 0.1
		}
	}
	if strings.Contains(questionLower, "what") || strings.Contains(questionLower, "是什么") {
		if strings.Contains(contentLower, "是") && strings.Contains(contentLower, "定义") {
			score += 0.1
		}
	}

	// 3. 数字和日期加分（通常是关键信息）
	digitCount := countDigits(content)
	if digitCount > 0 {
		score += 0.05
	}

	// 4. 长度惩罚（过长的内容可能被稀释）
	lengthPenalty := float64(len(content)) / 2000.0
	if lengthPenalty > 1 {
		lengthPenalty = 1
	}
	score -= lengthPenalty * 0.1

	// 确保分数在 0-1 范围内
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}

// llmScore 使用 LLM 评估重要性
func (r *ImportanceRanker) llmScore(ctx context.Context, question, content string) float64 {
	// 截断内容避免过长
	truncated := content
	if len(truncated) > 1000 {
		truncated = truncated[:1000] + "..."
	}

	prompt := `请评估以下内容与问题的相关性，输出一个 0-1 的分数（只输出数字，不要其他内容）。

问题：` + question + `

内容：` + truncated + `

评分标准：
- 0.0-0.3: 内容与问题几乎无关
- 0.4-0.6: 内容与问题有一定相关性
- 0.7-1.0: 内容与问题高度相关，直接包含答案

输出格式：只输出一个数字，保留两位小数`

	resp, err := r.provider.Chat(ctx, "你是一个信息相关性评估专家。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(10), provider.WithTemperature(0.1))

	if err != nil {
		logx.Infof("[ImportanceRanker] LLM score error: %v", err)
		return r.heuristicScore(question, content)
	}

	// 解析分数
	content = strings.TrimSpace(resp.Content)
	var score float64
	if _, err := parseFloat(content, &score); err != nil {
		logx.Infof("[ImportanceRanker] Parse score failed: %s, falling back to heuristic", content)
		return r.heuristicScore(question, content)
	}

	// 确保分数在有效范围
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	return score
}

// RankChunks 对多个内容块进行排序
// 返回: 按重要性降序排列的内容块
func (r *ImportanceRanker) RankChunks(ctx context.Context, question string, chunks []ChunkedContent) []ChunkedContent {
	// 评估每个块的重要性
	for i := range chunks {
		chunks[i].Importance = r.EvaluateImportance(ctx, question, chunks[i].Text)
	}

	// 冒泡排序
	for i := 0; i < len(chunks)-1; i++ {
		for j := 0; j < len(chunks)-i-1; j++ {
			if chunks[j].Importance < chunks[j+1].Importance {
				chunks[j], chunks[j+1] = chunks[j+1], chunks[j]
			}
		}
	}

	return chunks
}

// BatchEvaluate 批量评估多个内容块的重要性
func (r *ImportanceRanker) BatchEvaluate(ctx context.Context, question string, contents []string) []float64 {
	scores := make([]float64, len(contents))

	for i, content := range contents {
		scores[i] = r.EvaluateImportance(ctx, question, content)
	}

	return scores
}

// extractQuestionKeywords 从问句中提取关键词
func extractQuestionKeywords(question string) []string {
	// 停用词列表
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "shall": true, "can": true,
		"to": true, "of": true, "in": true, "for": true, "on": true,
		"with": true, "at": true, "by": true, "from": true, "as": true,
		"and": true, "or": true, "but": true, "if": true, "then": true,
		"else": true, "when": true, "up": true, "down": true, "out": true,
		"what": true, "which": true, "who": true, "whom": true, "whose": true,
		"where": true, "why": true, "how": true, "的": true, "是": true,
		"在": true, "和": true, "了": true, "我": true, "你": true,
		"他": true, "她": true, "它": true, "们": true, "这": true,
		"那": true, "有": true, "个": true, "就": true, "也": true,
		"都": true, "要": true, "会": true, "可以": true, "能": true,
	}

	var keywords []string
	words := strings.Fields(question)

	for _, word := range words {
		word = strings.ToLower(word)
		// 移除标点
		word = strings.Trim(word, ".,!?;:\"'")
		// 跳过停用词和过短的词
		if len(word) > 2 && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

// countDigits 统计字符串中的数字个数
func countDigits(s string) int {
	count := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			count++
		}
	}
	return count
}

// parseFloat 简单的浮点数解析
func parseFloat(s string, result *float64) (int, error) {
	var n int
	var frac float64 = 1.0
	var afterDot bool

	*result = 0

	for _, c := range s {
		if c >= '0' && c <= '9' {
			if !afterDot {
				n = n*10 + int(c-'0')
			} else {
				frac /= 10
				*result += float64(c-'0') * frac
			}
		} else if c == '.' {
			if afterDot {
				break
			}
			afterDot = true
			*result = float64(n)
		} else {
			// 遇到非数字字符，停止解析
			break
		}
	}

	if !afterDot {
		*result = float64(n)
	}

	return 0, nil
}
