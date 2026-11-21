package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/brand"
)

// APICallConfig API调用配置
type APICallConfig struct {
	Method         string
	URL            string
	Headers        map[string]string
	Body           interface{}
	Timeout        time.Duration
	AllowedDomains []string // 允许调用的域名白名单
}

// DefaultAPICallConfig 默认配置
func DefaultAPICallConfig() *APICallConfig {
	return &APICallConfig{
		Method:  "GET",
		Timeout: 30 * time.Second,
	}
}

// APICallTool 通用API调用工具
// 允许AI代理调用外部HTTP API获取实时数据
type APICallTool struct {
	client         *http.Client
	allowedDomains []string
}

// NewAPICallTool 创建API调用工具
func NewAPICallTool() *APICallTool {
	return &APICallTool{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// NewAPICallToolWithDomains 创建带域名白名单的工具：出网目标由模型给出，
// 因此使用受 SSRF 策略约束的客户端（重定向逐跳复校验）。
func NewAPICallToolWithDomains(domains []string) *APICallTool {
	return &APICallTool{
		client:         newOutboundClient(30*time.Second, domains),
		allowedDomains: domains,
	}
}

// Definition 返回工具定义
func (t *APICallTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "api_call",
		Description: "调用外部HTTP API获取实时数据。适用于查询天气、股票、汇率、数据库等需要实时外部数据的场景",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "API的完整URL地址",
				},
				"method": map[string]interface{}{
					"type":        "string",
					"description": "HTTP方法：GET、POST、PUT、DELETE",
				},
				"headers": map[string]interface{}{
					"type":        "object",
					"description": "HTTP请求头，如 {\"Authorization\": \"Bearer xxx\"}",
				},
				"body": map[string]interface{}{
					"type":        "object",
					"description": "请求体（用于POST/PUT），JSON格式",
				},
			},
			"required": []string{"url"},
		},
	}
}

// Execute 执行API调用
func (t *APICallTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// 解析参数
	url, _ := params["url"].(string)
	if url == "" {
		return &ToolResult{ToolName: "api_call", Success: false, Error: "url is required"}, nil
	}

	// 检查域名白名单
	if !t.isAllowedDomain(url) {
		return &ToolResult{ToolName: "api_call", Success: false, Error: fmt.Sprintf("domain not allowed: %s", url)}, nil
	}

	method := "GET"
	if m, ok := params["method"].(string); ok && m != "" {
		method = strings.ToUpper(m)
	}

	var body interface{}
	if b, ok := params["body"]; ok {
		body = b
	}

	headers := make(map[string]string)
	if h, ok := params["headers"].(map[string]interface{}); ok {
		for k, v := range h {
			if vs, ok := v.(string); ok {
				headers[k] = vs
			}
		}
	}

	// 执行调用
	result, err := t.execute(ctx, method, url, headers, body)
	if err != nil {
		return &ToolResult{ToolName: "api_call", Success: false, Error: err.Error()}, nil
	}

	return &ToolResult{
		ToolName: "api_call",
		Success:  true,
		Result:   result,
	}, nil
}

// isAllowedDomain 检查目标 URL 是否允许访问。
// 旧实现在「未配置白名单」时直接返回 true，等于对内网与云元数据地址完全不设防；
// 现在改为：有白名单按白名单匹配，没白名单也必须通过内网/元数据地址拦截。
func (t *APICallTool) isAllowedDomain(urlStr string) bool {
	return validateOutboundURL(urlStr, t.allowedDomains) == nil
}

// execute 执行HTTP请求
func (t *APICallTool) execute(ctx context.Context, method, url string, headers map[string]string, body interface{}) (string, error) {
	var reqBody io.Reader
	if body != nil && (method == "POST" || method == "PUT" || method == "PATCH") {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return "", fmt.Errorf("failed to marshal body: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
		if _, ok := headers["Content-Type"]; !ok {
			headers["Content-Type"] = "application/json"
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("User-Agent", brand.UserAgentAI)
	req.Header.Set("Accept", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024)) // 限制 1MB
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// 格式化 JSON 响应
	var prettyJSON interface{}
	if err := json.Unmarshal(respBody, &prettyJSON); err == nil {
		formatted, _ := json.MarshalIndent(prettyJSON, "", "  ")
		return string(formatted), nil
	}

	// 非 JSON 响应直接返回
	return string(respBody), nil
}

// SetAllowedDomains 设置域名白名单
func (t *APICallTool) SetAllowedDomains(domains []string) {
	t.allowedDomains = domains
}

// AddAllowedDomain 添加允许的域名
func (t *APICallTool) AddAllowedDomain(domain string) {
	t.allowedDomains = append(t.allowedDomains, domain)
}
