package rag

import (
	"context"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strings"

	"github.com/listen808/citebase/internal/ai/provider"
)

type AutoTagger struct {
	provider provider.Provider
}

func NewAutoTagger(p provider.Provider) *AutoTagger {
	return &AutoTagger{provider: p}
}

type TagResult struct {
	Tags       []string `json:"tags"`
	Category   string   `json:"category"`
	Confidence float64  `json:"confidence"`
}

func (at *AutoTagger) GenerateTags(ctx context.Context, content string) (*TagResult, error) {
	if at.provider == nil {
		return nil, fmt.Errorf("AI服务未启用")
	}

	if len(content) > 4000 {
		content = content[:4000]
	}

	prompt := fmt.Sprintf(`请为以下文档内容生成标签和分类。输出JSON格式：
{
  "tags": ["标签1", "标签2", "标签3"],
  "category": "分类名",
  "confidence": 0.9
}

要求：
- 生成5-10个标签，涵盖主题、技术、业务领域
- 分类为高层级类别（如"技术文档"、"产品设计"、"运维手册"等）
- confidence为分类的置信度(0-1)

文档内容：
%s`, content)

	resp, err := at.provider.Chat(ctx, "你是一个文档标签和分类助手。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(512), provider.WithTemperature(0.2))
	if err != nil {
		return nil, fmt.Errorf("LLM tag generation error: %w", err)
	}

	trimmed := strings.TrimSpace(resp.Content)
	if idx := strings.Index(trimmed, "{"); idx >= 0 {
		trimmed = trimmed[idx:]
		if last := strings.LastIndex(trimmed, "}"); last >= 0 {
			trimmed = trimmed[:last+1]
		}
	}

	var result TagResult
	if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
		logx.Infof("[AutoTagger] JSON parse error: %v", err)
		return &TagResult{Tags: []string{}, Category: "未分类", Confidence: 0}, nil
	}

	if len(result.Tags) > 15 {
		result.Tags = result.Tags[:15]
	}

	return &result, nil
}

type SearchOverview struct {
	Overview  string   `json:"overview"`
	KeyPoints []string `json:"key_points"`
}

func (at *AutoTagger) GenerateSearchOverview(ctx context.Context, query string, searchResults []Source) (*SearchOverview, error) {
	if at.provider == nil {
		return nil, fmt.Errorf("AI服务未启用")
	}

	if len(searchResults) == 0 {
		return &SearchOverview{Overview: "未找到相关结果。", KeyPoints: []string{}}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("搜索查询: %s\n\n相关文档摘要:\n", query))
	for i, src := range searchResults {
		if i >= 5 {
			break
		}
		snippet := src.Snippet
		if len(snippet) > 300 {
			snippet = snippet[:300] + "..."
		}
		sb.WriteString(fmt.Sprintf("%d. %s: %s\n", i+1, src.DocumentName, snippet))
	}

	prompt := fmt.Sprintf(`基于以上搜索结果，生成一个简洁的AI概览（100字以内），以及3-5个关键要点。
输出JSON格式：
{
  "overview": "概览文本",
  "key_points": ["要点1", "要点2", "要点3"]
}

%s`, sb.String())

	resp, err := at.provider.Chat(ctx, "你是一个搜索结果摘要助手。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(512), provider.WithTemperature(0.3))
	if err != nil {
		return nil, fmt.Errorf("LLM overview error: %w", err)
	}

	trimmed := strings.TrimSpace(resp.Content)
	if idx := strings.Index(trimmed, "{"); idx >= 0 {
		trimmed = trimmed[idx:]
		if last := strings.LastIndex(trimmed, "}"); last >= 0 {
			trimmed = trimmed[:last+1]
		}
	}

	var result SearchOverview
	if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
		logx.Infof("[AutoTagger] Overview JSON parse error: %v", err)
		return &SearchOverview{Overview: resp.Content, KeyPoints: []string{}}, nil
	}

	return &result, nil
}
