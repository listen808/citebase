package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
)

type TableGenerator struct {
	provider provider.Provider
}

func NewTableGenerator(p provider.Provider) *TableGenerator {
	return &TableGenerator{provider: p}
}

type TableRequest struct {
	Description string `json:"description"`
	SourceText  string `json:"source_text"`
	TableType   string `json:"table_type"`
	MaxRows     int    `json:"max_rows"`
	MaxColumns  int    `json:"max_columns"`
}

type TableResult struct {
	Markdown    string     `json:"markdown"`
	Headers     []string   `json:"headers"`
	Rows        [][]string `json:"rows"`
	ChartConfig string     `json:"chart_config,omitempty"`
}

func (g *TableGenerator) Generate(ctx context.Context, req TableRequest) (*TableResult, provider.TokenUsage, error) {
	if g.provider == nil {
		return nil, provider.TokenUsage{}, fmt.Errorf("AI服务未启用")
	}

	if req.MaxRows == 0 {
		req.MaxRows = 50
	}
	if req.MaxColumns == 0 {
		req.MaxColumns = 10
	}

	systemPrompt := `你是一个专业的表格生成助手。根据用户的描述，生成规范的Markdown表格。

要求：
1. 列名清晰、语义明确
2. 数据准确、内容完整
3. 如涉及知识库内容，标注来源
4. 严格输出JSON格式，不要输出其他内容

输出格式：
{
  "markdown": "完整的Markdown表格文本",
  "headers": ["列名1", "列名2"],
  "rows": [["值1", "值2"]],
  "chart_config": "如果数据适合可视化，返回ECharts配置JSON；否则留空"
}`

	var userPrompt string
	if req.SourceText != "" {
		sourceText := req.SourceText
		if len(sourceText) > 8000 {
			sourceText = sourceText[:8000] + "..."
		}
		userPrompt = fmt.Sprintf(`请根据以下文本内容生成表格：

文本内容：
%s

要求：%s
表格类型：%s
最大行数：%d
最大列数：%d`, sourceText, req.Description, req.TableType, req.MaxRows, req.MaxColumns)
	} else {
		userPrompt = fmt.Sprintf(`请根据以下描述生成表格：

描述：%s
表格类型：%s
最大行数：%d
最大列数：%d`, req.Description, req.TableType, req.MaxRows, req.MaxColumns)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	resp, err := g.provider.Chat(timeoutCtx, systemPrompt, []provider.Message{
		provider.NewTextMessage("user", userPrompt),
	}, provider.WithMaxTokens(4096), provider.WithTemperature(0.3))
	if err != nil {
		return nil, provider.TokenUsage{}, err
	}

	var result TableResult
	jsonStr := extractJSONFromText(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		result.Markdown = resp.Content
		result.Headers = []string{}
		result.Rows = [][]string{}
	}

	return &result, resp.TokenUsage, nil
}

func (g *TableGenerator) FromDocument(ctx context.Context, docContent string, description string) (*TableResult, provider.TokenUsage, error) {
	return g.Generate(ctx, TableRequest{
		Description: description,
		SourceText:  docContent,
		TableType:   "data",
	})
}

func extractJSONFromText(text string) string {
	start := strings.Index(text, "{")
	if start == -1 {
		return text
	}
	depth := 0
	for i := start; i < len(text); i++ {
		if text[i] == '{' {
			depth++
		} else if text[i] == '}' {
			depth--
			if depth == 0 {
				return text[start : i+1]
			}
		}
	}
	return text[start:]
}
