package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	logx "github.com/listen808/citebase/internal/log"
)

// SearchEngine 搜索引擎类型
type SearchEngine string

const (
	SearchEngineDuckDuckGo SearchEngine = "duckduckgo"
	SearchEngineBing       SearchEngine = "bing"
	SearchEngineGoogle     SearchEngine = "google"
)

// WebSearchResult 网络搜索结果
type WebSearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Snippet     string `json:"snippet"`
	Source      string `json:"source"`
	PublishedAt string `json:"published_at,omitempty"`
}

// WebSearchConfig 搜索配置
type WebSearchConfig struct {
	Engine         SearchEngine
	APIKey         string // Bing/Google API Key
	MaxResults     int
	Timeout        time.Duration
	AllowedDomains []string // 允许的域名列表
	BlockedDomains []string // 禁止的域名列表
}

// DefaultWebSearchConfig 默认搜索配置
func DefaultWebSearchConfig() *WebSearchConfig {
	return &WebSearchConfig{
		Engine:     SearchEngineDuckDuckGo,
		MaxResults: 10,
		Timeout:    30 * time.Second,
	}
}

// WebSearchTool 联网搜索工具
type WebSearchTool struct {
	client *http.Client
	config *WebSearchConfig
}

// NewWebSearchTool 创建搜索工具
func NewWebSearchTool() *WebSearchTool {
	return NewWebSearchToolWithConfig(DefaultWebSearchConfig())
}

// NewWebSearchToolWithConfig 创建带配置的搜索工具
func NewWebSearchToolWithConfig(cfg *WebSearchConfig) *WebSearchTool {
	return &WebSearchTool{
		client: &http.Client{Timeout: cfg.Timeout},
		config: cfg,
	}
}

// Definition 返回工具定义
func (t *WebSearchTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "web_search",
		Description: "联网搜索互联网，获取实时信息。适用于查询新闻、实时数据、热门事件等需要最新信息的问题",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "搜索关键词，建议使用英文关键词以获得更好的搜索结果",
				},
				"max_results": map[string]interface{}{
					"type":        "integer",
					"description": "最大返回结果数，默认5",
				},
			},
			"required": []string{"query"},
		},
	}
}

// Execute 执行搜索
func (t *WebSearchTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	query, _ := params["query"].(string)
	if query == "" {
		return &ToolResult{ToolName: "web_search", Success: false, Error: "query is required"}, nil
	}

	maxResults := 5
	if mr, ok := params["max_results"].(float64); ok && mr > 0 {
		maxResults = int(mr)
	}

	results, err := t.search(ctx, query, maxResults)
	if err != nil {
		return &ToolResult{ToolName: "web_search", Success: false, Error: err.Error()}, nil
	}

	return &ToolResult{
		ToolName: "web_search",
		Success:  true,
		Result:   formatSearchResults(query, results),
	}, nil
}

// search 执行搜索
func (t *WebSearchTool) search(ctx context.Context, query string, maxResults int) ([]WebSearchResult, error) {
	var results []WebSearchResult
	var err error
	switch t.config.Engine {
	case SearchEngineDuckDuckGo:
		results, err = t.searchDuckDuckGo(ctx, query, maxResults)
	case SearchEngineBing:
		results, err = t.searchBing(ctx, query, maxResults)
	case SearchEngineGoogle:
		results, err = t.searchGoogle(ctx, query, maxResults)
	default:
		results, err = t.searchDuckDuckGo(ctx, query, maxResults)
	}
	if err != nil {
		return nil, err
	}
	// P3：按 AllowedDomains/BlockedDomains 过滤结果
	return t.filterByDomain(results), nil
}

// filterByDomain 按配置的域名白/黑名单过滤搜索结果
func (t *WebSearchTool) filterByDomain(results []WebSearchResult) []WebSearchResult {
	if len(t.config.AllowedDomains) == 0 && len(t.config.BlockedDomains) == 0 {
		return results
	}
	filtered := make([]WebSearchResult, 0, len(results))
	for _, r := range results {
		domain := extractDomainFromURL(r.URL)
		if domain == "" {
			continue
		}
		allowed := true
		if len(t.config.AllowedDomains) > 0 {
			allowed = false
			for _, d := range t.config.AllowedDomains {
				if domain == d || strings.HasSuffix(domain, "."+d) {
					allowed = true
					break
				}
			}
		}
		if !allowed {
			continue
		}
		blocked := false
		for _, d := range t.config.BlockedDomains {
			if domain == d || strings.HasSuffix(domain, "."+d) {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

func extractDomainFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// searchDuckDuckGo 使用 DuckDuckGo 搜索
func (t *WebSearchTool) searchDuckDuckGo(ctx context.Context, query string, maxResults int) ([]WebSearchResult, error) {
	// 使用 DuckDuckGo HTML 页面解析
	baseURL := "https://duckduckgo.com/html/"
	data := url.Values{}
	data.Set("q", query)

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"?"+data.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("search failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	return t.parseDuckDuckGoHTML(string(body), maxResults)
}

// parseDuckDuckGoHTML 解析 DuckDuckGo HTML 结果
func (t *WebSearchTool) parseDuckDuckGoHTML(html string, maxResults int) ([]WebSearchResult, error) {
	var results []WebSearchResult

	// 简单的 HTML 解析，提取搜索结果
	lines := strings.Split(html, "\n")
	var currentResult *WebSearchResult

	for _, line := range lines {
		// 查找结果标题
		if strings.Contains(line, "result__title") {
			// 提取标题
			title := extractHTMLContent(line, "result__title")
			if title != "" {
				currentResult = &WebSearchResult{Title: strings.TrimSpace(title)}
			}
		}

		// 查找结果链接
		if currentResult != nil && strings.Contains(line, "result__url") {
			href := extractHTMLAttribute(line, "href")
			if href != "" {
				currentResult.URL = href
			}
		}

		// 查找结果摘要
		if currentResult != nil && strings.Contains(line, "result__snippet") {
			snippet := extractHTMLContent(line, "result__snippet")
			if snippet != "" {
				currentResult.Snippet = strings.TrimSpace(snippet)
				currentResult.Source = "DuckDuckGo"
				results = append(results, *currentResult)
				currentResult = nil

				if len(results) >= maxResults {
					break
				}
			}
		}
	}

	return results, nil
}

// searchBing 使用 Bing API 搜索
func (t *WebSearchTool) searchBing(ctx context.Context, query string, maxResults int) ([]WebSearchResult, error) {
	if t.config.APIKey == "" {
		return nil, fmt.Errorf("Bing API key is required")
	}

	apiURL := fmt.Sprintf("https://api.bing.microsoft.com/v7.0/search?q=%s&count=%d", url.QueryEscape(query), maxResults)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", t.config.APIKey)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var bingResp struct {
		WebPages struct {
			Value []struct {
				Name    string `json:"name"`
				URL     string `json:"url"`
				Snippet string `json:"snippet"`
			} `json:"value"`
		} `json:"webPages"`
	}

	if err := json.Unmarshal(body, &bingResp); err != nil {
		return nil, err
	}

	results := make([]WebSearchResult, 0, len(bingResp.WebPages.Value))
	for _, item := range bingResp.WebPages.Value {
		results = append(results, WebSearchResult{
			Title:   item.Name,
			URL:     item.URL,
			Snippet: item.Snippet,
			Source:  "Bing",
		})
	}

	return results, nil
}

// searchGoogle 使用 Google Custom Search API 搜索
func (t *WebSearchTool) searchGoogle(ctx context.Context, query string, maxResults int) ([]WebSearchResult, error) {
	if t.config.APIKey == "" {
		return nil, fmt.Errorf("Google API key is required")
	}

	apiURL := fmt.Sprintf("https://www.googleapis.com/customsearch/v1?key=%s&q=%s&num=%d",
		t.config.APIKey, url.QueryEscape(query), maxResults)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var googleResp struct {
		Items []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"items"`
	}

	if err := json.Unmarshal(body, &googleResp); err != nil {
		return nil, err
	}

	results := make([]WebSearchResult, 0, len(googleResp.Items))
	for _, item := range googleResp.Items {
		results = append(results, WebSearchResult{
			Title:   item.Title,
			URL:     item.Link,
			Snippet: item.Snippet,
			Source:  "Google",
		})
	}

	return results, nil
}

// formatSearchResults 格式化搜索结果
func formatSearchResults(query string, results []WebSearchResult) string {
	if len(results) == 0 {
		return fmt.Sprintf("搜索「%s」未找到相关结果", query)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("搜索「%s」找到 %d 条结果：\n\n", query, len(results)))

	for i, r := range results {
		b.WriteString(fmt.Sprintf("【结果 %d】\n", i+1))
		b.WriteString(fmt.Sprintf("标题：%s\n", r.Title))
		b.WriteString(fmt.Sprintf("链接：%s\n", r.URL))
		b.WriteString(fmt.Sprintf("摘要：%s\n\n", r.Snippet))
	}

	b.WriteString("提示：可以结合搜索结果回答用户问题，如果需要更详细的信息可以使用 fetch_web 工具抓取网页内容。")

	return b.String()
}

// extractHTMLContent 提取 HTML 标签内容
func extractHTMLContent(html, className string) string {
	// 简单实现，查找 class="xxx" 后的内容
	start := strings.Index(html, `class="`+className+`"`)
	if start == -1 {
		return ""
	}

	// 查找 > 后的内容
	contentStart := strings.Index(html[start:], ">")
	if contentStart == -1 {
		return ""
	}
	contentStart += start + 1

	// 查找 </a> 结束
	contentEnd := strings.Index(html[contentStart:], "</")
	if contentEnd == -1 {
		return ""
	}

	return html[contentStart : contentStart+contentEnd]
}

// extractHTMLAttribute 提取 HTML 属性值
func extractHTMLAttribute(html, attrName string) string {
	// 查找 href="..."
	start := strings.Index(html, attrName+`="`)
	if start == -1 {
		return ""
	}
	start += len(attrName) + 2

	end := strings.Index(html[start:], `"`)
	if end == -1 {
		return ""
	}

	return html[start : start+end]
}

// UpdateConfig 更新搜索配置
func (t *WebSearchTool) UpdateConfig(cfg *WebSearchConfig) {
	t.config = cfg
	t.client.Timeout = cfg.Timeout
}

// GetConfig 获取当前配置
func (t *WebSearchTool) GetConfig() *WebSearchConfig {
	return t.config
}

// RegisterToAgent 注册到 Agent
func (t *WebSearchTool) RegisterToAgent(agent *AgentChat) {
	agent.RegisterTool("web_search", t)
	logx.Infof("[WebSearchTool] Registered to agent")
}
