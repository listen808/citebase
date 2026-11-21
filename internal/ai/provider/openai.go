package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

type openaiMessage struct {
	Role       string                   `json:"role"`
	Content    json.RawMessage          `json:"content"`
	ToolCalls  []openaiToolCallResponse `json:"tool_calls,omitempty"`
	ToolCallID string                   `json:"tool_call_id,omitempty"`
}

type openaiToolDef struct {
	Type     string             `json:"type"`
	Function openaiToolFunction `json:"function"`
}

type openaiToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type openaiToolCallResponse struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openaiRequest struct {
	Model         string          `json:"model"`
	Messages      []openaiMessage `json:"messages"`
	MaxTokens     int             `json:"max_tokens"`
	Temperature   float64         `json:"temperature"`
	Stream        bool            `json:"stream,omitempty"`
	StreamOptions *struct {
		IncludeUsage bool `json:"include_usage"`
	} `json:"stream_options,omitempty"`
	Tools          []openaiToolDef `json:"tools,omitempty"`
	ToolChoice     interface{}     `json:"tool_choice,omitempty"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
	Stop           []string        `json:"stop,omitempty"`
	TopP           float64         `json:"top_p,omitempty"`
}

type openaiChoice struct {
	Message struct {
		Role      string                   `json:"role"`
		Content   json.RawMessage          `json:"content"`
		ToolCalls []openaiToolCallResponse `json:"tool_calls,omitempty"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
}

type openaiResponse struct {
	Choices []openaiChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
	Usage *openaiUsage `json:"usage,omitempty"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openaiEmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type openaiEmbeddingData struct {
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

type openaiEmbeddingResponse struct {
	Data  []openaiEmbeddingData `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type OpenAIProvider struct {
	baseURL     string
	apiKey      string
	model       string
	maxTokens   int
	temperature float64
	httpClient  *http.Client
}

func NewOpenAIProvider(baseURL, apiKey, model string, maxTokens int, temperature float64) *OpenAIProvider {
	baseURL = strings.TrimRight(baseURL, "/")
	// 自动补全 /v1 路径前缀（Ollama 等本地模型服务需要）
	if !strings.HasSuffix(baseURL, "/v1") {
		baseURL = baseURL + "/v1"
	}
	return &OpenAIProvider{
		baseURL:     baseURL,
		apiKey:      apiKey,
		model:       model,
		maxTokens:   maxTokens,
		temperature: temperature,
		httpClient:  &http.Client{Timeout: 300 * time.Second}, // 增加到 5 分钟，适配本地模型
	}
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) ListModels(ctx context.Context) ([]string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	bodyPreview := string(respBody)
	if len(bodyPreview) > 300 {
		bodyPreview = bodyPreview[:300]
	}
	if httpResp.StatusCode != 200 {
		return nil, fmt.Errorf("api status %d: %s", httpResp.StatusCode, bodyPreview)
	}

	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	seen := make(map[string]bool, len(resp.Data))
	models := make([]string, 0, len(resp.Data))
	for _, item := range resp.Data {
		if item.ID != "" && !seen[item.ID] {
			seen[item.ID] = true
			models = append(models, item.ID)
		}
	}
	sort.Strings(models)
	return models, nil
}

func (p *OpenAIProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		FunctionCalling: true,
		Streaming:       true,
		Vision:          true,
		JSONMode:        true,
		ContextWindow:   128000,
	}
}

func (p *OpenAIProvider) Chat(ctx context.Context, system string, messages []Message, opts ...Option) (*LLMResponse, error) {
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

	reqMessages := []openaiMessage{{Role: "system", Content: mustRawJSON(system)}}
	for _, m := range messages {
		reqMessages = append(reqMessages, openaiMessage{Role: m.Role, Content: m.Content})
	}

	reqBody := openaiRequest{
		Model:       model,
		Messages:    reqMessages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	apiURL := p.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

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

	var resp openaiResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if resp.Error != nil && resp.Error.Message != "" {
		return nil, fmt.Errorf("api error: %s", resp.Error.Message)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from api")
	}

	result := &LLMResponse{
		Content: strings.TrimSpace(rawMessageToString(resp.Choices[0].Message.Content)),
		Model:   model,
	}
	if resp.Usage != nil {
		result.TokenUsage = TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}
	logAudit("openai", "chat", result.TokenUsage.TotalTokens, nil) // S19
	if result.TokenUsage.TotalTokens > 0 {
		RecordTokenUsage(userIDFromContext(ctx), result.TokenUsage.TotalTokens)
	}
	return result, nil
}

func (p *OpenAIProvider) StreamChat(ctx context.Context, system string, messages []Message, opts ...Option) (<-chan StreamChunk, error) {
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

	reqMessages := []openaiMessage{{Role: "system", Content: mustRawJSON(system)}}
	for _, m := range messages {
		reqMessages = append(reqMessages, openaiMessage{Role: m.Role, Content: m.Content})
	}

	reqBody := openaiRequest{
		Model:       model,
		Messages:    reqMessages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      true,
	}
	if strings.Contains(p.baseURL, "api.openai.com") {
		reqBody.StreamOptions = &struct {
			IncludeUsage bool `json:"include_usage"`
		}{IncludeUsage: true}
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	apiURL := p.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
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
				// P1 修复（清单 60）：sseReader 正常结束返回 (nil, nil)，这里 err!=nil
				// 必然是网络中断/扫描错误。此前伪装成 Done 会让调用方把半截回答当完整
				// 答案，也让 PooledProvider 的流式熔断统计（依赖 chunk.Error）成为死代码。
				ch <- StreamChunk{Done: true, Error: fmt.Sprintf("stream interrupted: %v", err), Usage: estimateTokenUsage(contentLen)}
				return
			}
			if event == nil || event.Data == "" || event.Data == "[DONE]" {
				ch <- StreamChunk{Done: true, Usage: estimateTokenUsage(contentLen)}
				return
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
				Usage *openaiUsage `json:"usage"`
			}
			if err := json.Unmarshal([]byte(event.Data), &chunk); err != nil {
				continue
			}
			if chunk.Usage != nil && chunk.Usage.TotalTokens > 0 {
				ch <- StreamChunk{
					Done: true,
					Usage: &TokenUsage{
						PromptTokens:     chunk.Usage.PromptTokens,
						CompletionTokens: chunk.Usage.CompletionTokens,
						TotalTokens:      chunk.Usage.TotalTokens,
					},
				}
				return
			}
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				contentLen += len(chunk.Choices[0].Delta.Content)
				ch <- StreamChunk{Content: chunk.Choices[0].Delta.Content}
			}
		}
	}()

	return ch, nil
}

func (p *OpenAIProvider) GetEmbedding(ctx context.Context, text string) ([]float64, error) {
	results, err := p.GetEmbeddings(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no embedding data returned")
	}
	return results[0], nil
}

func (p *OpenAIProvider) GetEmbeddings(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("empty texts")
	}

	model := p.model
	if model == "" {
		model = "text-embedding-3-small"
	}

	reqBody := openaiEmbeddingRequest{
		Model: model,
		Input: texts,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	apiURL := p.baseURL + "/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	httpResp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("embedding api request: %w", err)
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embedding response: %w", err)
	}

	if httpResp.StatusCode != 200 {
		return nil, fmt.Errorf("embedding api status %d: %s", httpResp.StatusCode, string(respBody))
	}

	var resp openaiEmbeddingResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse embedding response: %w", err)
	}

	if resp.Error != nil && resp.Error.Message != "" {
		return nil, fmt.Errorf("embedding api error: %s", resp.Error.Message)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding data returned")
	}

	results := make([][]float64, len(resp.Data))
	for _, d := range resp.Data {
		// P1 修复（清单 59）：index 来自外部响应体，直接下标会 panic（越界/负值），
		// 且非连续 index 会残留 nil 向量被下游当有效向量写入向量库。越界即拒绝该响应。
		if d.Index < 0 || d.Index >= len(results) {
			return nil, fmt.Errorf("embedding response contains invalid index %d (data len=%d)", d.Index, len(resp.Data))
		}
		results[d.Index] = d.Embedding
	}
	for i, r := range results {
		if r == nil {
			return nil, fmt.Errorf("embedding response missing vector for index %d", i)
		}
	}
	return results, nil
}

// convertTools 将 ToolDefinition 转换为 OpenAI tools 参数格式
func convertTools(tools []ToolDefinition) []openaiToolDef {
	result := make([]openaiToolDef, 0, len(tools))
	for _, t := range tools {
		result = append(result, openaiToolDef{
			Type: "function",
			Function: openaiToolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}
	return result
}

// convertToolCalls 将 OpenAI 响应中的 tool_calls 转换为 ToolCall
func convertToolCalls(tc []openaiToolCallResponse) []ToolCall {
	result := make([]ToolCall, 0, len(tc))
	for _, t := range tc {
		result = append(result, ToolCall{
			ID:        t.ID,
			Name:      t.Function.Name,
			Arguments: json.RawMessage(t.Function.Arguments),
		})
	}
	return result
}

func (p *OpenAIProvider) ChatWithTools(ctx context.Context, system string, messages []Message, tools []ToolDefinition, opts ...Option) (*LLMResponse, error) {
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

	reqMessages := []openaiMessage{{Role: "system", Content: mustRawJSON(system)}}
	for _, m := range messages {
		reqMessages = append(reqMessages, openaiMessage{Role: m.Role, Content: m.Content})
	}

	reqBody := openaiRequest{
		Model:       model,
		Messages:    reqMessages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}

	if len(tools) > 0 {
		reqBody.Tools = convertTools(tools)
	}
	if cfg.ToolChoice != nil {
		reqBody.ToolChoice = cfg.ToolChoice
	}
	if cfg.ResponseFormat != nil {
		reqBody.ResponseFormat = cfg.ResponseFormat
	}
	if len(cfg.Stop) > 0 {
		reqBody.Stop = cfg.Stop
	}
	if cfg.TopP > 0 {
		reqBody.TopP = cfg.TopP
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	apiURL := p.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

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

	var resp openaiResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if resp.Error != nil && resp.Error.Message != "" {
		return nil, fmt.Errorf("api error: %s", resp.Error.Message)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from api")
	}

	result := &LLMResponse{
		Content: strings.TrimSpace(rawMessageToString(resp.Choices[0].Message.Content)),
		Model:   model,
	}
	if len(resp.Choices[0].Message.ToolCalls) > 0 {
		result.ToolCalls = convertToolCalls(resp.Choices[0].Message.ToolCalls)
	}
	if resp.Usage != nil {
		result.TokenUsage = TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}
	logAudit("openai", "chat_with_tools", result.TokenUsage.TotalTokens, nil)
	if result.TokenUsage.TotalTokens > 0 {
		RecordTokenUsage(userIDFromContext(ctx), result.TokenUsage.TotalTokens)
	}
	return result, nil
}

func (p *OpenAIProvider) StreamChatWithTools(ctx context.Context, system string, messages []Message, tools []ToolDefinition, opts ...Option) (<-chan StreamChunk, error) {
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

	reqMessages := []openaiMessage{{Role: "system", Content: mustRawJSON(system)}}
	for _, m := range messages {
		reqMessages = append(reqMessages, openaiMessage{Role: m.Role, Content: m.Content})
	}

	reqBody := openaiRequest{
		Model:       model,
		Messages:    reqMessages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      true,
	}
	if strings.Contains(p.baseURL, "api.openai.com") {
		reqBody.StreamOptions = &struct {
			IncludeUsage bool `json:"include_usage"`
		}{IncludeUsage: true}
	}
	if len(tools) > 0 {
		reqBody.Tools = convertTools(tools)
	}
	if cfg.ToolChoice != nil {
		reqBody.ToolChoice = cfg.ToolChoice
	}
	if cfg.ResponseFormat != nil {
		reqBody.ResponseFormat = cfg.ResponseFormat
	}
	if len(cfg.Stop) > 0 {
		reqBody.Stop = cfg.Stop
	}
	if cfg.TopP > 0 {
		reqBody.TopP = cfg.TopP
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	apiURL := p.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
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
		var toolCallBuffer map[int]*ToolCall // index -> accumulated ToolCall
		decoder := newSSEReader(httpResp.Body)
		for {
			event, err := decoder.Next()
			if err != nil {
				// P1 修复（清单 60）：sseReader 正常结束返回 (nil, nil)，这里 err!=nil
				// 必然是网络中断/扫描错误。此前伪装成 Done 会让调用方把半截回答当完整
				// 答案，也让 PooledProvider 的流式熔断统计（依赖 chunk.Error）成为死代码。
				ch <- StreamChunk{Done: true, Error: fmt.Sprintf("stream interrupted: %v", err), Usage: estimateTokenUsage(contentLen)}
				return
			}
			if event == nil || event.Data == "" || event.Data == "[DONE]" {
				ch <- StreamChunk{Done: true, Usage: estimateTokenUsage(contentLen)}
				return
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content   string `json:"content"`
						ToolCalls []struct {
							Index    int    `json:"index"`
							ID       string `json:"id"`
							Type     string `json:"type"`
							Function struct {
								Name      string `json:"name"`
								Arguments string `json:"arguments"`
							} `json:"function"`
						} `json:"tool_calls"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
				Usage *openaiUsage `json:"usage"`
			}
			if err := json.Unmarshal([]byte(event.Data), &chunk); err != nil {
				continue
			}
			if chunk.Usage != nil && chunk.Usage.TotalTokens > 0 {
				ch <- StreamChunk{
					Done: true,
					Usage: &TokenUsage{
						PromptTokens:     chunk.Usage.PromptTokens,
						CompletionTokens: chunk.Usage.CompletionTokens,
						TotalTokens:      chunk.Usage.TotalTokens,
					},
				}
				return
			}
			if len(chunk.Choices) > 0 {
				choice := chunk.Choices[0]
				if choice.Delta.Content != "" {
					contentLen += len(choice.Delta.Content)
					ch <- StreamChunk{Content: choice.Delta.Content}
				}
				// Accumulate tool call chunks
				if len(choice.Delta.ToolCalls) > 0 {
					if toolCallBuffer == nil {
						toolCallBuffer = make(map[int]*ToolCall)
					}
					for _, tc := range choice.Delta.ToolCalls {
						existing, ok := toolCallBuffer[tc.Index]
						if !ok {
							existing = &ToolCall{
								ID:   tc.ID,
								Name: tc.Function.Name,
							}
							toolCallBuffer[tc.Index] = existing
						}
						if tc.ID != "" {
							existing.ID = tc.ID
						}
						if tc.Function.Name != "" {
							existing.Name = tc.Function.Name
						}
						existing.Arguments = append(existing.Arguments, []byte(tc.Function.Arguments)...)
					}
				}
				// When finish_reason is "tool_calls", emit accumulated tool calls
				if choice.FinishReason == "tool_calls" {
					for i := 0; i < len(toolCallBuffer); i++ {
						if tc, ok := toolCallBuffer[i]; ok {
							ch <- StreamChunk{ToolCall: tc}
						}
					}
					ch <- StreamChunk{Done: true, Usage: estimateTokenUsage(contentLen)}
					return
				}
			}
		}
	}()

	return ch, nil
}

func mustRawJSON(s string) json.RawMessage {
	b, _ := json.Marshal(s)
	return json.RawMessage(b)
}

func rawMessageToString(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}
