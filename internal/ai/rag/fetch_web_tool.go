package rag

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/brand"
)

type FetchWebTool struct {
	client         *http.Client
	allowedDomains []string
}

func NewFetchWebTool() *FetchWebTool {
	return &FetchWebTool{
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// NewFetchWebToolWithPolicy 使用出网安全策略构造工具：URL 逐跳校验，
// 未配置白名单时至少挡住内网/元数据地址。
func NewFetchWebToolWithPolicy(allowedDomains []string) *FetchWebTool {
	return &FetchWebTool{
		client:         newOutboundClient(15*time.Second, allowedDomains),
		allowedDomains: allowedDomains,
	}
}

func (t *FetchWebTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "fetch_web",
		Description: "抓取指定URL的网页内容，返回纯文本。用于获取外部信息辅助回答",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "要抓取的网页URL",
				},
				"max_length": map[string]interface{}{
					"type":        "integer",
					"description": "返回内容最大字符数，默认3000",
				},
			},
			"required": []string{"url"},
		},
	}
}

func (t *FetchWebTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	url, _ := params["url"].(string)
	if url == "" {
		return &ToolResult{ToolName: "fetch_web", Success: false, Error: "url is required"}, nil
	}

	maxLength := 3000
	if ml, ok := params["max_length"].(float64); ok && ml > 0 {
		maxLength = int(ml)
	}

	// URL 来自模型输出，必须先做 SSRF 校验再发请求（构造函数里的 client 也校验重定向目标）
	if err := validateOutboundURL(url, t.allowedDomains); err != nil {
		return &ToolResult{ToolName: "fetch_web", Success: false, Error: "目标地址被拒绝: " + err.Error()}, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return &ToolResult{ToolName: "fetch_web", Success: false, Error: err.Error()}, nil
	}
	req.Header.Set("User-Agent", brand.UserAgentAI)

	resp, err := t.client.Do(req)
	if err != nil {
		return &ToolResult{ToolName: "fetch_web", Success: false, Error: "请求失败: " + err.Error()}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &ToolResult{ToolName: "fetch_web", Success: false, Error: fmt.Sprintf("HTTP %d", resp.StatusCode)}, nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return &ToolResult{ToolName: "fetch_web", Success: false, Error: "读取响应失败: " + err.Error()}, nil
	}

	text := string(body)
	text = stripHTMLTags(text)
	if len(text) > maxLength {
		text = text[:maxLength] + "..."
	}

	return &ToolResult{
		ToolName: "fetch_web",
		Success:  true,
		Result:   fmt.Sprintf("网页内容 (%s):\n%s", url, text),
	}, nil
}

func stripHTMLTags(html string) string {
	var result strings.Builder
	inTag := false
	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			result.WriteRune(r)
		}
	}
	text := result.String()
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "\t", " ")
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}
	return strings.TrimSpace(text)
}
