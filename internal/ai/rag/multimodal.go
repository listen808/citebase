package rag

import (
	"context"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strings"
	"sync"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/utils"
)

type MultimodalProcessor struct {
	provider provider.Provider
}

func NewMultimodalProcessor(p provider.Provider) *MultimodalProcessor {
	return &MultimodalProcessor{provider: p}
}

func (mp *MultimodalProcessor) DescribeImage(ctx context.Context, imageURL string) (string, provider.TokenUsage, error) {
	if mp.provider == nil {
		return "", provider.TokenUsage{}, fmt.Errorf("AI服务未启用")
	}

	msg := provider.NewImageMessage("user", "请详细描述这张图片的内容，包括文字、图表、数据等关键信息。用中文描述。", imageURL)

	resp, err := mp.provider.Chat(ctx, "你是一个图片内容分析助手，擅长从图片中提取文字、数据和关键信息。", []provider.Message{msg},
		provider.WithMaxTokens(1024), provider.WithTemperature(0.3))
	if err != nil {
		return "", provider.TokenUsage{}, fmt.Errorf("vision model error: %w", err)
	}

	return strings.TrimSpace(resp.Content), resp.TokenUsage, nil
}

func (mp *MultimodalProcessor) DescribeImageBase64(ctx context.Context, base64Data, mimeType string) (string, provider.TokenUsage, error) {
	if mp.provider == nil {
		return "", provider.TokenUsage{}, fmt.Errorf("AI服务未启用")
	}

	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)
	return mp.DescribeImage(ctx, dataURL)
}

func (mp *MultimodalProcessor) ExtractTextFromImage(ctx context.Context, imageURL string) (string, error) {
	if mp.provider == nil {
		return "", fmt.Errorf("AI服务未启用")
	}

	msg := provider.NewImageMessage("user", "请提取图片中的所有文字内容，按原始格式输出。如果图片中没有文字，返回空字符串。", imageURL)

	resp, err := mp.provider.Chat(ctx, "你是一个OCR助手，负责从图片中提取文字。", []provider.Message{msg},
		provider.WithMaxTokens(2048), provider.WithTemperature(0.1))
	if err != nil {
		return "", fmt.Errorf("OCR error: %w", err)
	}

	return strings.TrimSpace(resp.Content), nil
}

type ImageDescription struct {
	URL         string `json:"url"`
	Description string `json:"description"`
	DocID       int    `json:"doc_id"`
}

// BatchDescribeImages 批量并发描述图片
// P1-6 (§3.2.3): 并发处理批量图片，提升处理效率
// 并发度限制为 3（避免 API 速率限制）
func (mp *MultimodalProcessor) BatchDescribeImages(ctx context.Context, images []ImageDescription) ([]ImageDescription, error) {
	if len(images) == 0 {
		return images, nil
	}

	results := make([]ImageDescription, len(images))
	maxConcurrency := 3
	if len(images) < maxConcurrency {
		maxConcurrency = len(images)
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrency)

	for i, img := range images {
		wg.Add(1)
		go func(idx int, image ImageDescription) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			desc, _, err := mp.DescribeImage(ctx, image.URL)
			if err != nil {
				logx.Infof("[MultimodalProcessor] Error describing image %s: %v", image.URL, err)
				results[idx] = image
				return
			}
			results[idx] = ImageDescription{
				URL:         image.URL,
				Description: desc,
				DocID:       image.DocID,
			}
		}(i, img)
	}
	wg.Wait()
	return results, nil
}

type DocumentImporter struct {
	provider provider.Provider
}

func NewDocumentImporter(p provider.Provider) *DocumentImporter {
	return &DocumentImporter{provider: p}
}

type ImportedDocument struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Format  string `json:"format"`
}

func (di *DocumentImporter) ImportFromText(ctx context.Context, rawText string, format string) (*ImportedDocument, error) {
	switch strings.ToLower(format) {
	case "markdown", "md":
		return &ImportedDocument{Title: di.extractTitle(rawText), Content: rawText, Format: "markdown"}, nil
	case "text", "txt":
		markdown := di.plainTextToMarkdown(rawText)
		return &ImportedDocument{Title: di.extractTitle(rawText), Content: markdown, Format: "markdown"}, nil
	case "json":
		return di.importJSON(rawText)
	default:
		if di.provider != nil {
			return di.convertWithLLM(ctx, rawText, format)
		}
		return &ImportedDocument{Title: "导入文档", Content: rawText, Format: format}, nil
	}
}

func (di *DocumentImporter) plainTextToMarkdown(text string) string {
	lines := strings.Split(text, "\n")
	var result strings.Builder
	emptyLineCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			emptyLineCount++
			if emptyLineCount == 1 {
				result.WriteString("\n\n")
			}
			continue
		}
		emptyLineCount = 0

		if len(trimmed) <= 50 && !strings.Contains(trimmed, ".") && !strings.Contains(trimmed, ",") {
			result.WriteString("## ")
			result.WriteString(trimmed)
			result.WriteString("\n")
		} else {
			result.WriteString(trimmed)
			result.WriteString("\n")
		}
	}

	return result.String()
}

func (di *DocumentImporter) importJSON(text string) (*ImportedDocument, error) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	title := "JSON文档"
	if t, ok := data["title"].(string); ok {
		title = t
	}

	contentBytes, _ := json.MarshalIndent(data, "", "  ")
	content := "```json\n" + string(contentBytes) + "\n```"

	return &ImportedDocument{Title: title, Content: content, Format: "markdown"}, nil
}

func (di *DocumentImporter) convertWithLLM(ctx context.Context, rawText string, format string) (*ImportedDocument, error) {
	input := rawText
	if len(input) > 8000 {
		input = utils.TruncateRunes(input, 8000)
	}

	prompt := fmt.Sprintf(`请将以下%s格式的内容转换为Markdown格式。保留原始结构、标题、列表、表格等。
只输出转换后的Markdown内容，不要解释。

原始内容：
%s`, format, input)

	resp, err := di.provider.Chat(ctx, "你是一个文档格式转换助手。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(4096), provider.WithTemperature(0.2))
	if err != nil {
		return nil, fmt.Errorf("LLM conversion error: %w", err)
	}

	content := strings.TrimSpace(resp.Content)
	title := di.extractTitle(content)

	return &ImportedDocument{Title: title, Content: content, Format: "markdown"}, nil
}

func (di *DocumentImporter) extractTitle(text string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}

	if len(lines) > 0 {
		first := strings.TrimSpace(lines[0])
		if len(first) > 0 && len(first) <= 100 {
			return first
		}
	}

	return "导入文档"
}
