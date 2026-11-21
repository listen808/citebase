package rag

import (
	"context"
	"fmt"
	"strings"
)

// DocumentAugmenter 文档增强器
type DocumentAugmenter struct {
	provider providerInterface
}

// providerInterface provider 抽象接口
type providerInterface interface {
	Chat(ctx context.Context, messages []providerMsg) (string, error)
}

type providerMsg struct {
	Role    string
	Content string
}

// AugmentedDocument 增强后的文档
type AugmentedDocument struct {
	DocID         int            `json:"doc_id"`
	QA            []QA           `json:"qa"`
	ParagraphTags []ParagraphTag `json:"paragraph_tags"`
}

// QA 问答对
type QA struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// ParagraphTag 段落标签
type ParagraphTag struct {
	ParagraphID string   `json:"paragraph_id"`
	Tags        []string `json:"tags"`
	Summary     string   `json:"summary"`
}

// NewDocumentAugmenter 创建文档增强器
func NewDocumentAugmenter(p providerInterface) *DocumentAugmenter {
	return &DocumentAugmenter{provider: p}
}

// AugmentDocument 增强文档
// 1. 生成 Q&A 对（补充检索源）
// 2. 为每个段落生成摘要标签
func (a *DocumentAugmenter) AugmentDocument(
	ctx context.Context,
	docID int,
	content string,
) (*AugmentedDocument, error) {
	if a.provider == nil {
		return &AugmentedDocument{DocID: docID}, nil
	}

	result := &AugmentedDocument{DocID: docID}

	// 1. 生成 Q&A 对
	qa, err := a.generateQA(ctx, content)
	if err == nil {
		result.QA = qa
	}

	// 2. 为段落生成标签
	paragraphs := strings.Split(content, "\n\n")
	for i, p := range paragraphs {
		p = strings.TrimSpace(p)
		if len(p) < 20 {
			continue
		}
		tags, summary, err := a.generateTags(ctx, p)
		if err == nil {
			result.ParagraphTags = append(result.ParagraphTags, ParagraphTag{
				ParagraphID: fmt.Sprintf("doc%d-para%d", docID, i),
				Tags:        tags,
				Summary:     summary,
			})
		}
	}

	return result, nil
}

// generateQA 生成问答对
func (a *DocumentAugmenter) generateQA(ctx context.Context, content string) ([]QA, error) {
	prompt := fmt.Sprintf(`基于以下文档内容，生成 3-5 个高质量的问答对，用于补充知识库检索。

文档内容：
%s

要求：
1. 问题应覆盖文档的核心知识点
2. 答案应简洁准确，基于文档内容
3. 输出 JSON 数组格式：[{"question":"...","answer":"..."}]

输出：`, truncateText(content, 2000))

	resp, err := a.provider.Chat(ctx, []providerMsg{
		{Role: "system", Content: "你是问答对生成助手，只输出JSON数组。"},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil, err
	}

	// 简单解析（实际应使用 json.Unmarshal）
	return parseQA(resp), nil
}

// generateTags 生成段落标签和摘要
func (a *DocumentAugmenter) generateTags(ctx context.Context, paragraph string) ([]string, string, error) {
	prompt := fmt.Sprintf(`为以下段落生成 3-5 个标签和一个简短摘要（50字以内）。

段落：
%s

输出格式：
标签：tag1, tag2, tag3
摘要：xxx`, truncateText(paragraph, 500))

	resp, err := a.provider.Chat(ctx, []providerMsg{
		{Role: "system", Content: "你是文档标注助手。"},
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil, "", err
	}

	tags := parseTags(resp)
	summary := parseSummary(resp)
	return tags, summary, nil
}

// parseQA 简单解析问答对
func parseQA(text string) []QA {
	var qa []QA
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines)-1; i += 2 {
		q := strings.TrimSpace(lines[i])
		a := strings.TrimSpace(lines[i+1])
		if q != "" && a != "" {
			qa = append(qa, QA{Question: q, Answer: a})
		}
	}
	return qa
}

// parseTags 解析标签
func parseTags(text string) []string {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "标签：") || strings.HasPrefix(line, "标签:") {
			tagsStr := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "标签："), "标签:"))
			return strings.Split(tagsStr, ",")
		}
	}
	return nil
}

// parseSummary 解析摘要
func parseSummary(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "摘要：") || strings.HasPrefix(line, "摘要:") {
			return strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "摘要："), "摘要:"))
		}
	}
	return ""
}

// truncateText 截断文本
func truncateText(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
