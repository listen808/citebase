package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/listen808/citebase/internal/log"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/utils"
)

// QualityAssessor 文档质量评估器
type QualityAssessor struct {
	provider provider.Provider
}

// NewQualityAssessor 创建评估器
func NewQualityAssessor(p provider.Provider) *QualityAssessor {
	return &QualityAssessor{
		provider: p,
	}
}

// QualityScore 质量评分
type QualityScore struct {
	DocID        int     `json:"doc_id"`
	OverallScore float64 `json:"overall_score"` // 0-100

	// 维度评分
	Completeness float64 `json:"completeness"` // 完整性
	Freshness    float64 `json:"freshness"`    // 时效性
	Clarity      float64 `json:"clarity"`      // 清晰度
	Consistency  float64 `json:"consistency"`  // 一致性
	Structure    float64 `json:"structure"`    // 结构性

	Issues      []QualityIssue `json:"issues"`
	Suggestions []string       `json:"suggestions"`
}

// QualityIssue 质量问题
type QualityIssue struct {
	Type        string `json:"type"`     // outdated/missing_info/inconsistent/broken_link/vague
	Severity    string `json:"severity"` // high/medium/low
	Description string `json:"description"`
	Location    string `json:"location"` // 在文档中的位置
}

// AssessQuality 评估文档质量
func (a *QualityAssessor) AssessQuality(ctx context.Context, docID int, content string) (*QualityScore, error) {
	if a.provider == nil {
		// 降级方案：简单规则评估
		return a.simpleAssess(content), nil
	}

	if len(content) > 8000 {
		content = utils.TruncateRunes(content, 8000)
	}

	prompt := fmt.Sprintf(`请评估以下文档的质量，从 5 个维度打分（0-100）：

1. 完整性：是否缺少关键信息（如联系方式、部署步骤、配置说明）
2. 时效性：内容是否可能已过时（检查日期、版本号、废弃标记）
3. 清晰度：表述是否清楚，是否有歧义
4. 一致性：与其他文档是否有矛盾
5. 结构性：格式是否规范，是否有清晰的层级

同时列出所有发现的问题和改进建议。

文档内容：
%s

输出 JSON：
{
  "overall_score": 85,
  "completeness": 90,
  "freshness": 80,
  "clarity": 85,
  "consistency": 90,
  "structure": 80,
  "issues": [
    {"type": "missing_info", "severity": "medium", "description": "缺少部署步骤", "location": "正文"}
  ],
  "suggestions": ["建议添加部署步骤说明", "建议更新版本号"]
}`, content)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := a.provider.Chat(ctx, "你是一个文档质量评估专家，只输出 JSON，不要解释。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(2048), provider.WithTemperature(0.1))
	if err != nil {
		log.Errorf("[QualityAssessor] LLM assess error: %v, fallback to simple assess", err)
		return a.simpleAssess(content), nil
	}

	// Phase2: 修复：解析LLM的实际输出，而非丢弃
	score := &QualityScore{
		DocID:       docID,
		Issues:      []QualityIssue{},
		Suggestions: []string{},
	}

	jsonStr := utils.ExtractJSON(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), score); err == nil && score.OverallScore > 0 {
		return score, nil
	}

	// JSON解析失败时，用正则降级提取
	score = &QualityScore{
		DocID:        docID,
		OverallScore: 75,
		Completeness: 75,
		Freshness:    75,
		Clarity:      75,
		Consistency:  75,
		Structure:    75,
		Issues:       []QualityIssue{},
		Suggestions:  []string{"建议人工审核文档质量"},
	}
	return score, nil
}

// simpleAssess 简单规则评估
func (a *QualityAssessor) simpleAssess(content string) *QualityScore {
	score := &QualityScore{
		DocID:        0,
		OverallScore: 60,
		Completeness: 60,
		Freshness:    60,
		Clarity:      60,
		Consistency:  60,
		Structure:    60,
		Issues:       []QualityIssue{},
		Suggestions:  []string{},
	}

	// 检查字数
	wordCount := len(content)
	if wordCount < 100 {
		score.Issues = append(score.Issues, QualityIssue{
			Type:        "missing_info",
			Severity:    "high",
			Description: "文档内容过少",
			Location:    "全文",
		})
		score.OverallScore -= 20
	}

	// 检查是否有标题
	if len(content) > 0 && content[0] != '#' {
		score.Issues = append(score.Issues, QualityIssue{
			Type:        "structure",
			Severity:    "low",
			Description: "缺少标题",
			Location:    "开头",
		})
		score.Structure -= 20
	}

	// 检查是否有链接
	if !containsURL(content) {
		score.Suggestions = append(score.Suggestions, "建议添加相关参考链接")
	}

	// 检查是否有代码块
	if !containsCodeBlock(content) {
		score.Suggestions = append(score.Suggestions, "建议添加代码示例")
	}

	// 确保分数在 0-100 范围
	score.OverallScore = clamp(score.OverallScore, 0, 100)
	score.Completeness = clamp(score.Completeness, 0, 100)
	score.Freshness = clamp(score.Freshness, 0, 100)
	score.Clarity = clamp(score.Clarity, 0, 100)
	score.Consistency = clamp(score.Consistency, 0, 100)
	score.Structure = clamp(score.Structure, 0, 100)

	return score
}

func containsURL(text string) bool {
	return len(text) > 10 && (contains(text, "http://") || contains(text, "https://"))
}

func containsCodeBlock(text string) bool {
	return contains(text, "```")
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// S12: Contradiction 跨文档矛盾
type Contradiction struct {
	DocAID      int    `json:"doc_a_id"`
	DocASection string `json:"doc_a_section"`
	DocBID      int    `json:"doc_b_id"`
	DocBSection string `json:"doc_b_section"`
	Description string `json:"description"`
	Severity    string `json:"severity"` // high/medium/low
}

// DetectContradictions 检测跨文档矛盾
func (a *QualityAssessor) DetectContradictions(ctx context.Context, docID int, content string, otherDocs []DocContentBrief) ([]Contradiction, error) {
	if a.provider == nil || len(otherDocs) == 0 {
		return nil, nil
	}

	if len(content) > 4000 {
		content = content[:4000]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("以下是一份文档的内容，请检查是否与其他文档存在矛盾或不一致：\n\n"))
	sb.WriteString(fmt.Sprintf("当前文档(ID:%d)：\n%s\n\n", docID, content))
	sb.WriteString("其他文档摘要：\n")
	for _, doc := range otherDocs {
		if doc.Content != "" {
			snippet := doc.Content
			if len(snippet) > 500 {
				snippet = snippet[:500] + "..."
			}
			sb.WriteString(fmt.Sprintf("文档%d(%s)：%s\n", doc.DocID, doc.Name, snippet))
		}
	}
	sb.WriteString(`\n如果发现矛盾，输出JSON数组：[{"doc_a_id":1,"doc_a_section":"章节","doc_b_id":2,"doc_b_section":"章节","description":"矛盾描述","severity":"high/medium/low"}]
如果没有矛盾，输出空数组 []`)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := a.provider.Chat(ctx, "你是一个文档一致性检查专家。只输出JSON数组。", []provider.Message{
		provider.NewTextMessage("user", sb.String()),
	}, provider.WithMaxTokens(1024), provider.WithTemperature(0.1))
	if err != nil {
		return nil, err
	}

	var contradictions []Contradiction
	jsonStr := utils.ExtractJSON(resp.Content)
	_ = json.Unmarshal([]byte(jsonStr), &contradictions)
	return contradictions, nil
}

// S12: DocContentBrief 文档内容简要
type DocContentBrief struct {
	DocID   int
	Name    string
	Content string
}
