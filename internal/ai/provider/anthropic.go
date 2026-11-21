package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type anthropicMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// anthropicToolMessage 用于工具调用的消息（content 可以是混合类型数组）
type anthropicToolMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

// anthropicToolRequest 用于带工具的请求（messages 支持混合 content 类型）
type anthropicToolRequest struct {
	Model       string                 `json:"model"`
	Messages    []anthropicToolMessage `json:"messages"`
	System      string                 `json:"system,omitempty"`
	MaxTokens   int                    `json:"max_tokens"`
	Temperature float64                `json:"temperature,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
	Tools       []anthropicToolDef     `json:"tools,omitempty"`
}

type anthropicToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type anthropicContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type anthropicResponse struct {
	Content    []anthropicContentBlock `json:"content"`
	StopReason string                  `json:"stop_reason,omitempty"`
	Error      *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
	Usage *anthropicUsage `json:"usage,omitempty"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type AnthropicProvider struct {
	baseURL     string
	apiKey      string
	model       string
	maxTokens   int
	temperature float64
	httpClient  *http.Client
}

func NewAnthropicProvider(baseURL, apiKey, model string, maxTokens int, temperature float64) *AnthropicProvider {
	return &AnthropicProvider{
		baseURL:     strings.TrimRight(baseURL, "/"),
		apiKey:      apiKey,
		model:       model,
		maxTokens:   maxTokens,
		temperature: temperature,
		httpClient:  &http.Client{Timeout: 120 * time.Second},
	}
}

func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

func (p *AnthropicProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		FunctionCalling: true,
		Streaming:       true,
		Vision:          true,
		JSONMode:        false,
		ContextWindow:   200000,
	}
}

func (p *AnthropicProvider) buildAPIURL() string {
	base := strings.TrimSuffix(p.baseURL, "/v1")
	return base + "/v1/messages"
}

func (p *AnthropicProvider) Chat(ctx context.Context, system string, messages []Message, opts ...Option) (*LLMResponse, error) {
	if uid := userIDFromContext(ctx); uid > 0 {
		if err := CheckQuota(uid); err != nil {
			return nil, err
		}
	}
	cfg := applyOptions(opts...)
	model := p.model
	if cfg.Model != "" {
		model = cfg.Model
	}
	maxTokens := p.maxTokens
	if cfg.MaxTokens > 0 {
		maxTokens = cfg.MaxTokens
	}
	temperature := p.temperature
	if cfg.Temperature > 0 {
		temperature = cfg.Temperature
	}

	reqMessages := make([]anthropicMessage, 0, len(messages))
	for _, m := range messages {
		reqMessages = append(reqMessages, anthropicMessage{Role: m.Role, Content: convertToAnthropicContent(m.Content)})
	}

	reqBody := anthropicRequest{
		Model:       model,
		Messages:    reqMessages,
		System:      system,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	apiURL := p.buildAPIURL()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("api request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if httpResp.StatusCode != 200 {
		return nil, fmt.Errorf("api status %d: %s", httpResp.StatusCode, string(respBody))
	}

	var resp anthropicResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if resp.Error != nil && resp.Error.Message != "" {
		return nil, fmt.Errorf("api error (%s): %s", resp.Error.Type, resp.Error.Message)
	}

	if len(resp.Content) == 0 {
		return nil, fmt.Errorf("no response from api")
	}

	result := &LLMResponse{
		Content: strings.TrimSpace(resp.Content[0].Text),
		Model:   model,
	}
	if resp.Usage != nil {
		result.TokenUsage = TokenUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		}
	}
	logAudit("anthropic", "chat", result.TokenUsage.TotalTokens, nil) // S19
	if result.TokenUsage.TotalTokens > 0 {
		RecordTokenUsage(userIDFromContext(ctx), result.TokenUsage.TotalTokens)
	}
	return result, nil
}

func (p *AnthropicProvider) StreamChat(ctx context.Context, system string, messages []Message, opts ...Option) (<-chan StreamChunk, error) {
	if uid := userIDFromContext(ctx); uid > 0 {
		if err := CheckQuota(uid); err != nil {
			return nil, err
		}
	}
	cfg := applyOptions(opts...)
	model := p.model
	if cfg.Model != "" {
		model = cfg.Model
	}
	maxTokens := p.maxTokens
	if cfg.MaxTokens > 0 {
		maxTokens = cfg.MaxTokens
	}
	temperature := p.temperature
	if cfg.Temperature > 0 {
		temperature = cfg.Temperature
	}

	reqMessages := make([]anthropicMessage, 0, len(messages))
	for _, m := range messages {
		reqMessages = append(reqMessages, anthropicMessage{Role: m.Role, Content: convertToAnthropicContent(m.Content)})
	}

	reqBody := anthropicRequest{
		Model:       model,
		Messages:    reqMessages,
		System:      system,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      true,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	apiURL := p.buildAPIURL()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Accept", "text/event-stream")

	httpResp, err := p.httpClient.Do(httpReq) //nolint:bodyclose // body closed in streaming goroutine
	if err != nil {
		return nil, fmt.Errorf("api request: %w", err)
	}

	if httpResp.StatusCode != 200 {
		defer httpResp.Body.Close()
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("api status %d: %s", httpResp.StatusCode, string(respBody))
	}

	ch := make(chan StreamChunk, 64)
	go func() {
		defer close(ch)
		defer httpResp.Body.Close()
		var contentLen int
		decoder := newSSEReader(httpResp.Body)
		for {
			event, err := decoder.Next()
			if err != nil {
				// P1 修复（清单 60）：中断不再伪装成正常结束（同 openai.go 注释）
				ch <- StreamChunk{Done: true, Error: fmt.Sprintf("stream interrupted: %v", err), Usage: estimateTokenUsage(contentLen)}
				return
			}
			if event == nil || event.Data == "" || event.Data == "[DONE]" {
				ch <- StreamChunk{Done: true, Usage: estimateTokenUsage(contentLen)}
				return
			}
			var chunk struct {
				Type  string `json:"type"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
				Message struct {
					Usage *anthropicUsage `json:"usage"`
				} `json:"message"`
				Usage *anthropicUsage `json:"usage"`
			}
			if err := json.Unmarshal([]byte(event.Data), &chunk); err != nil {
				continue
			}
			if chunk.Type == "content_block_delta" && chunk.Delta.Text != "" {
				contentLen += len(chunk.Delta.Text)
				ch <- StreamChunk{Content: chunk.Delta.Text}
			}
			if chunk.Type == "message_stop" {
				usage := chunk.Usage
				if usage == nil {
					usage = chunk.Message.Usage
				}
				if usage != nil {
					total := usage.InputTokens + usage.OutputTokens
					if total > 0 {
						ch <- StreamChunk{
							Done: true,
							Usage: &TokenUsage{
								PromptTokens:     usage.InputTokens,
								CompletionTokens: usage.OutputTokens,
								TotalTokens:      total,
							},
						}
						return
					}
				}
				ch <- StreamChunk{Done: true, Usage: estimateTokenUsage(contentLen)}
				return
			}
		}
	}()

	return ch, nil
}

func (p *AnthropicProvider) GetEmbedding(ctx context.Context, text string) ([]float64, error) {
	return nil, fmt.Errorf("embedding not supported by anthropic provider")
}

func (p *AnthropicProvider) GetEmbeddings(ctx context.Context, texts []string) ([][]float64, error) {
	return nil, fmt.Errorf("embedding not supported by anthropic provider")
}

// convertAnthropicTools 将 ToolDefinition 转换为 Anthropic tools 格式（input_schema）
func convertAnthropicTools(tools []ToolDefinition) []anthropicToolDef {
	result := make([]anthropicToolDef, 0, len(tools))
	for _, t := range tools {
		result = append(result, anthropicToolDef{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.Parameters,
		})
	}
	return result
}

// buildAnthropicToolMessage 将通用 Message 转换为 Anthropic 格式，处理 tool_use 和 tool_result
func buildAnthropicToolMessage(m Message) anthropicToolMessage {
	msg := anthropicToolMessage{Role: m.Role}

	// assistant 消息包含 tool_use 块
	if m.Role == "assistant" && len(m.ToolCalls) > 0 {
		blocks := make([]interface{}, 0, len(m.ToolCalls)+1)
		// 添加文本块（如果有内容）
		text := extractTextFromRaw(m.Content)
		if text != "" {
			blocks = append(blocks, map[string]interface{}{"type": "text", "text": text})
		}
		// 添加 tool_use 块
		for _, tc := range m.ToolCalls {
			blocks = append(blocks, map[string]interface{}{
				"type":  "tool_use",
				"id":    tc.ID,
				"name":  tc.Name,
				"input": tc.Arguments,
			})
		}
		msg.Content = blocks
		return msg
	}

	// user 消息包含 tool_result 块
	if m.Role == "user" && m.ToolCallID != "" {
		text := extractTextFromRaw(m.Content)
		msg.Content = []interface{}{
			map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": m.ToolCallID,
				"content":     text,
			},
		}
		return msg
	}

	// 普通消息，转换多模态 content（如有图片则转为 Anthropic 格式）
	msg.Content = convertToAnthropicContent(m.Content)
	return msg
}

// extractTextFromRaw 从 json.RawMessage 中提取纯文本
func extractTextFromRaw(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

// convertToAnthropicContent 将 OpenAI 格式的多模态 content 转换为 Anthropic 格式
// OpenAI: [{"type":"text","text":"..."},{"type":"image_url","image_url":{"url":"data:image/jpeg;base64,..."}}]
// Anthropic: [{"type":"text","text":"..."},{"type":"image","source":{"type":"base64","media_type":"image/jpeg","data":"..."}}]
// 如果不是数组格式（纯文本），直接返回原始 content
func convertToAnthropicContent(content json.RawMessage) json.RawMessage {
	// 尝试解析为数组
	var parts []map[string]interface{}
	if err := json.Unmarshal(content, &parts); err != nil {
		// 不是数组，直接返回（纯文本消息）
		return content
	}

	// 检查是否包含 image_url 类型（需要转换）
	hasImage := false
	for _, part := range parts {
		if part["type"] == "image_url" {
			hasImage = true
			break
		}
	}
	if !hasImage {
		return content
	}

	// 转换为 Anthropic 格式
	result := make([]interface{}, 0, len(parts))
	for _, part := range parts {
		if part["type"] == "image_url" {
			imageURLData, ok := part["image_url"].(map[string]interface{})
			if !ok {
				continue
			}
			url, _ := imageURLData["url"].(string)
			if url == "" {
				continue
			}
			// 解析 data URL: data:image/jpeg;base64,/9j/4AAQ...
			mediaType, base64Data := parseDataURI(url)
			if mediaType == "" {
				mediaType = "image/jpeg"
			}
			result = append(result, map[string]interface{}{
				"type": "image",
				"source": map[string]interface{}{
					"type":       "base64",
					"media_type": mediaType,
					"data":       base64Data,
				},
			})
		} else {
			result = append(result, part)
		}
	}

	data, _ := json.Marshal(result)
	return json.RawMessage(data)
}

// parseDataURI 从 data URI 中提取 media type 和 base64 数据
func parseDataURI(uri string) (mediaType, base64Data string) {
	if !strings.HasPrefix(uri, "data:") {
		return "", uri
	}
	// data:image/jpeg;base64,/9j/4AAQ...
	rest := uri[5:] // 去掉 "data:"
	idx := strings.Index(rest, ";base64,")
	if idx < 0 {
		return "", uri
	}
	mediaType = rest[:idx]
	base64Data = rest[idx+8:] // 去掉 ";base64,"
	return
}

func (p *AnthropicProvider) ChatWithTools(ctx context.Context, system string, messages []Message, tools []ToolDefinition, opts ...Option) (*LLMResponse, error) {
	if uid := userIDFromContext(ctx); uid > 0 {
		if err := CheckQuota(uid); err != nil {
			return nil, err
		}
	}
	cfg := applyOptions(opts...)
	model := p.model
	if cfg.Model != "" {
		model = cfg.Model
	}
	maxTokens := p.maxTokens
	if cfg.MaxTokens > 0 {
		maxTokens = cfg.MaxTokens
	}
	temperature := p.temperature
	if cfg.Temperature > 0 {
		temperature = cfg.Temperature
	}

	reqMessages := make([]anthropicToolMessage, 0, len(messages))
	for _, m := range messages {
		reqMessages = append(reqMessages, buildAnthropicToolMessage(m))
	}

	reqBody := anthropicToolRequest{
		Model:       model,
		Messages:    reqMessages,
		System:      system,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Tools:       convertAnthropicTools(tools),
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	apiURL := p.buildAPIURL()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("api request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if httpResp.StatusCode != 200 {
		return nil, fmt.Errorf("api status %d: %s", httpResp.StatusCode, string(respBody))
	}

	var resp anthropicResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if resp.Error != nil && resp.Error.Message != "" {
		return nil, fmt.Errorf("api error (%s): %s", resp.Error.Type, resp.Error.Message)
	}

	if len(resp.Content) == 0 {
		return nil, fmt.Errorf("no response from api")
	}

	result := &LLMResponse{Model: model}

	// 遍历 content blocks，提取文本和 tool_use
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			if result.Content == "" {
				result.Content = block.Text
			} else {
				result.Content += "\n" + block.Text
			}
		case "tool_use":
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: block.Input,
			})
		}
	}

	result.Content = strings.TrimSpace(result.Content)

	if resp.Usage != nil {
		result.TokenUsage = TokenUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		}
	}
	logAudit("anthropic", "chat_with_tools", result.TokenUsage.TotalTokens, nil)
	if result.TokenUsage.TotalTokens > 0 {
		RecordTokenUsage(userIDFromContext(ctx), result.TokenUsage.TotalTokens)
	}
	return result, nil
}

func (p *AnthropicProvider) StreamChatWithTools(ctx context.Context, system string, messages []Message, tools []ToolDefinition, opts ...Option) (<-chan StreamChunk, error) {
	if uid := userIDFromContext(ctx); uid > 0 {
		if err := CheckQuota(uid); err != nil {
			return nil, err
		}
	}
	cfg := applyOptions(opts...)
	model := p.model
	if cfg.Model != "" {
		model = cfg.Model
	}
	maxTokens := p.maxTokens
	if cfg.MaxTokens > 0 {
		maxTokens = cfg.MaxTokens
	}
	temperature := p.temperature
	if cfg.Temperature > 0 {
		temperature = cfg.Temperature
	}

	reqMessages := make([]anthropicToolMessage, 0, len(messages))
	for _, m := range messages {
		reqMessages = append(reqMessages, buildAnthropicToolMessage(m))
	}

	reqBody := anthropicToolRequest{
		Model:       model,
		Messages:    reqMessages,
		System:      system,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      true,
		Tools:       convertAnthropicTools(tools),
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	apiURL := p.buildAPIURL()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Accept", "text/event-stream")

	httpResp, err := p.httpClient.Do(httpReq) //nolint:bodyclose // body closed in streaming goroutine
	if err != nil {
		return nil, fmt.Errorf("api request: %w", err)
	}

	if httpResp.StatusCode != 200 {
		defer httpResp.Body.Close()
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, fmt.Errorf("api status %d: %s", httpResp.StatusCode, string(respBody))
	}

	ch := make(chan StreamChunk, 64)
	go func() {
		defer close(ch)
		defer httpResp.Body.Close()
		var contentLen int
		// 用于累积 tool_use 块的输入 JSON
		var currentToolCallID string
		var currentToolCallName string
		var currentToolInput strings.Builder
		var toolCalls []ToolCall

		decoder := newSSEReader(httpResp.Body)
		for {
			event, err := decoder.Next()
			if err != nil {
				// 发送已累积的 tool calls
				for _, tc := range toolCalls {
					ch <- StreamChunk{ToolCall: &tc}
				}
				// P1 修复（清单 60）：带 Error，不再伪装成正常结束
				ch <- StreamChunk{Done: true, Error: fmt.Sprintf("stream interrupted: %v", err), Usage: estimateTokenUsage(contentLen)}
				return
			}
			if event == nil || event.Data == "" || event.Data == "[DONE]" {
				for _, tc := range toolCalls {
					ch <- StreamChunk{ToolCall: &tc}
				}
				ch <- StreamChunk{Done: true, Usage: estimateTokenUsage(contentLen)}
				return
			}

			var chunk struct {
				Type  string `json:"type"`
				Index int    `json:"index,omitempty"`
				Delta struct {
					Type        string `json:"type"`
					Text        string `json:"text,omitempty"`
					PartialJSON string `json:"partial_json,omitempty"`
				} `json:"delta"`
				ContentBlock struct {
					Type string `json:"type"`
					ID   string `json:"id,omitempty"`
					Name string `json:"name,omitempty"`
				} `json:"content_block"`
				Message struct {
					Usage *anthropicUsage `json:"usage"`
				} `json:"message"`
				Usage *anthropicUsage `json:"usage"`
			}
			if err := json.Unmarshal([]byte(event.Data), &chunk); err != nil {
				continue
			}

			switch chunk.Type {
			case "content_block_start":
				if chunk.ContentBlock.Type == "tool_use" {
					currentToolCallID = chunk.ContentBlock.ID
					currentToolCallName = chunk.ContentBlock.Name
					currentToolInput.Reset()
				}
			case "content_block_delta":
				if chunk.Delta.Type == "text_delta" && chunk.Delta.Text != "" {
					contentLen += len(chunk.Delta.Text)
					ch <- StreamChunk{Content: chunk.Delta.Text}
				} else if chunk.Delta.Type == "input_json_delta" && chunk.Delta.PartialJSON != "" {
					currentToolInput.WriteString(chunk.Delta.PartialJSON)
				}
			case "content_block_stop":
				if currentToolCallID != "" {
					inputBytes := json.RawMessage(currentToolInput.String())
					toolCalls = append(toolCalls, ToolCall{
						ID:        currentToolCallID,
						Name:      currentToolCallName,
						Arguments: inputBytes,
					})
					currentToolCallID = ""
					currentToolCallName = ""
					currentToolInput.Reset()
				}
			case "message_delta":
				// message_delta 可能包含 stop_reason
			case "message_stop":
				usage := chunk.Usage
				if usage == nil {
					usage = chunk.Message.Usage
				}
				for _, tc := range toolCalls {
					ch <- StreamChunk{ToolCall: &tc}
				}
				if usage != nil && usage.InputTokens+usage.OutputTokens > 0 {
					ch <- StreamChunk{
						Done: true,
						Usage: &TokenUsage{
							PromptTokens:     usage.InputTokens,
							CompletionTokens: usage.OutputTokens,
							TotalTokens:      usage.InputTokens + usage.OutputTokens,
						},
					}
				} else {
					ch <- StreamChunk{Done: true, Usage: estimateTokenUsage(contentLen)}
				}
				return
			}
		}
	}()

	return ch, nil
}
