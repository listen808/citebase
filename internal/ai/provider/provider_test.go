package provider

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"timeout", errors.New("request timeout"), true},
		{"time out", errors.New("time out occurred"), true},
		{"deadline", errors.New("deadline exceeded"), true},
		{"429 too many requests", errors.New("429 Too Many Requests"), true},
		{"rate limit", errors.New("rate limit reached"), true},
		{"500 internal error", errors.New("500 Internal Server Error"), true},
		{"502 bad gateway", errors.New("502 Bad Gateway"), true},
		{"503 unavailable", errors.New("503 Service Unavailable"), true},
		{"504 gateway timeout", errors.New("504 Gateway Timeout"), true},
		{"connection refused", errors.New("connection refused"), true},
		{"connection reset", errors.New("connection reset by peer"), true},
		{"no such host", errors.New("no such host"), true},
		{"context deadline exceeded", errors.New("context deadline exceeded"), true},
		{"invalid request", errors.New("400 Bad Request"), false},
		{"unauthorized", errors.New("401 Unauthorized"), false},
		{"not found", errors.New("404 Not Found"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("isRetryableError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestWithRetry_SuccessOnFirstTry(t *testing.T) {
	ctx := context.Background()
	callCount := 0
	err := WithRetry(ctx, 3, 1*time.Millisecond, func() error {
		callCount++
		return nil
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestWithRetry_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	callCount := 0
	err := WithRetry(ctx, 3, 1*time.Millisecond, func() error {
		callCount++
		return errors.New("400 Bad Request")
	})
	if err == nil {
		t.Error("expected error, got nil")
	}
	if callCount != 1 {
		t.Errorf("expected 1 call (non-retryable), got %d", callCount)
	}
}

func TestWithRetry_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	callCount := 0
	err := WithRetry(ctx, 3, 100*time.Millisecond, func() error {
		callCount++
		return errors.New("timeout")
	})
	if err == nil {
		t.Error("expected error, got nil")
	}
	// 首次调用会执行，但重试会因为 context 取消而返回
	if callCount < 1 {
		t.Errorf("expected at least 1 call, got %d", callCount)
	}
}

func TestNewProvider_OpenAICompatible(t *testing.T) {
	names := []string{"openai", "deepseek", "qwen", "zhipu", "ernie", "gemini", "ollama"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			p, err := NewProvider(&ProviderConfig{
				Name:    name,
				BaseURL: "https://api.example.com/v1",
				APIKey:  "test-key",
				Model:   "test-model",
			})
			if err != nil {
				t.Errorf("NewProvider(%s) failed: %v", name, err)
			}
			if p == nil {
				t.Error("expected non-nil provider")
			}
		})
	}
}

func TestNewProvider_Anthropic(t *testing.T) {
	p, err := NewProvider(&ProviderConfig{
		Name:    "anthropic",
		BaseURL: "https://api.anthropic.com",
		APIKey:  "test-key",
		Model:   "claude-3",
	})
	if err != nil {
		t.Errorf("NewProvider(anthropic) failed: %v", err)
	}
	if p == nil {
		t.Error("expected non-nil provider")
	}
}

func TestNewProvider_Unsupported(t *testing.T) {
	_, err := NewProvider(&ProviderConfig{
		Name:    "unknown-provider",
		BaseURL: "https://api.example.com",
		APIKey:  "test-key",
		Model:   "test-model",
	})
	if err == nil {
		t.Error("expected error for unsupported provider")
	}
}

func TestNewTextMessage(t *testing.T) {
	msg := NewTextMessage("user", "hello world")
	if msg.Role != "user" {
		t.Errorf("expected role 'user', got %s", msg.Role)
	}
	content := msg.TextContent()
	if content != "hello world" {
		t.Errorf("expected content 'hello world', got %s", content)
	}
}

func TestMessage_TextContent_InvalidJSON(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: []byte("not valid json"),
	}
	content := msg.TextContent()
	if content != "" {
		t.Errorf("expected empty string for invalid JSON, got %s", content)
	}
}

func TestMessage_TextContent_NonString(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: []byte("[1, 2, 3]"),
	}
	content := msg.TextContent()
	if content != "" {
		t.Errorf("expected empty string for non-string content, got %s", content)
	}
}

func TestNewToolCallMessage(t *testing.T) {
	toolCalls := []ToolCall{
		{ID: "call_1", Name: "test_tool", Arguments: []byte(`{"key": "value"}`)},
	}
	msg := NewToolCallMessage("here's the result", toolCalls)
	if msg.Role != "assistant" {
		t.Errorf("expected role 'assistant', got %s", msg.Role)
	}
	if len(msg.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].Name != "test_tool" {
		t.Errorf("expected tool name 'test_tool', got %s", msg.ToolCalls[0].Name)
	}
}

func TestNewToolResultMessage(t *testing.T) {
	msg := NewToolResultMessage("call_1", "tool output")
	if msg.Role != "user" {
		t.Errorf("expected role 'user', got %s", msg.Role)
	}
	if msg.ToolCallID != "call_1" {
		t.Errorf("expected tool_call_id 'call_1', got %s", msg.ToolCallID)
	}
}

func TestApplyOptions(t *testing.T) {
	opts := applyOptions(
		WithMaxTokens(100),
		WithTemperature(0.7),
		WithModel("gpt-4"),
		WithTopP(0.9),
		WithStop([]string{"stop1", "stop2"}),
		WithJSONMode(),
	)
	if opts.MaxTokens != 100 {
		t.Errorf("expected MaxTokens=100, got %d", opts.MaxTokens)
	}
	if opts.Temperature != 0.7 {
		t.Errorf("expected Temperature=0.7, got %f", opts.Temperature)
	}
	if opts.Model != "gpt-4" {
		t.Errorf("expected Model=gpt-4, got %s", opts.Model)
	}
	if opts.TopP != 0.9 {
		t.Errorf("expected TopP=0.9, got %f", opts.TopP)
	}
	if len(opts.Stop) != 2 {
		t.Errorf("expected 2 stop sequences, got %d", len(opts.Stop))
	}
	if opts.ResponseFormat == nil || opts.ResponseFormat.Type != "json_object" {
		t.Error("expected JSON mode response format")
	}
}

func TestWithTools(t *testing.T) {
	tools := []ToolDefinition{
		{Name: "tool1", Description: "test tool"},
	}
	opts := applyOptions(WithTools(tools))
	if len(opts.Tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(opts.Tools))
	}
}

func TestWithToolChoice(t *testing.T) {
	opts := applyOptions(WithToolChoice("auto"))
	if opts.ToolChoice != "auto" {
		t.Errorf("expected ToolChoice=auto, got %v", opts.ToolChoice)
	}
}

func TestDefaultRecoveryStrategy_Builder(t *testing.T) {
	s := NewDefaultRecoveryStrategy().
		WithMaxRetries(5).
		WithBaseDelay(2 * time.Second).
		WithMaxDelay(60 * time.Second).
		WithFallbackModel("backup-model")

	if s.maxRetries != 5 {
		t.Errorf("expected maxRetries=5, got %d", s.maxRetries)
	}
	if s.baseDelay != 2*time.Second {
		t.Errorf("expected baseDelay=2s, got %v", s.baseDelay)
	}
	if s.maxDelay != 60*time.Second {
		t.Errorf("expected maxDelay=60s, got %v", s.maxDelay)
	}
	if s.FallbackModel() != "backup-model" {
		t.Errorf("expected fallback model 'backup-model', got %s", s.FallbackModel())
	}
}

func TestDefaultRecoveryStrategy_HandleError_NilError(t *testing.T) {
	s := NewDefaultRecoveryStrategy()
	action := s.HandleError(context.Background(), nil, 0)
	if action != RecoveryFail {
		t.Errorf("expected RecoveryFail for nil error, got %v", action)
	}
}

func TestDefaultRecoveryStrategy_HandleError_ContextCanceled(t *testing.T) {
	s := NewDefaultRecoveryStrategy()
	action := s.HandleError(context.Background(), context.Canceled, 0)
	if action != RecoveryFail {
		t.Errorf("expected RecoveryFail for context canceled, got %v", action)
	}
}

func TestDefaultRecoveryStrategy_HandleError_401(t *testing.T) {
	s := NewDefaultRecoveryStrategy()
	action := s.HandleError(context.Background(), errors.New("401 Unauthorized"), 0)
	if action != RecoveryFail {
		t.Errorf("expected RecoveryFail for 401, got %v", action)
	}
}

func TestDefaultRecoveryStrategy_HandleError_429_Retry(t *testing.T) {
	s := NewDefaultRecoveryStrategy().WithMaxRetries(3)
	action := s.HandleError(context.Background(), errors.New("429 Too Many Requests"), 1)
	if action != RecoveryRetry {
		t.Errorf("expected RecoveryRetry for 429 (attempt 1), got %v", action)
	}
}

func TestDefaultRecoveryStrategy_HandleError_429_Fallback(t *testing.T) {
	s := NewDefaultRecoveryStrategy().WithMaxRetries(2).WithFallbackModel("backup")
	action := s.HandleError(context.Background(), errors.New("429 Too Many Requests"), 2)
	if action != RecoveryFallback {
		t.Errorf("expected RecoveryFallback for 429 (max retries reached + fallback), got %v", action)
	}
}

func TestDefaultRecoveryStrategy_HandleError_500_Retry(t *testing.T) {
	s := NewDefaultRecoveryStrategy().WithMaxRetries(3)
	action := s.HandleError(context.Background(), errors.New("500 Internal Server Error"), 0)
	if action != RecoveryRetry {
		t.Errorf("expected RecoveryRetry for 500, got %v", action)
	}
}

func TestDefaultRecoveryStrategy_HandleError_Timeout_Retry(t *testing.T) {
	s := NewDefaultRecoveryStrategy().WithMaxRetries(3)
	action := s.HandleError(context.Background(), errors.New("connection timeout"), 0)
	if action != RecoveryRetry {
		t.Errorf("expected RecoveryRetry for timeout, got %v", action)
	}
}

func TestDefaultRecoveryStrategy_ShouldRetry(t *testing.T) {
	s := NewDefaultRecoveryStrategy().WithMaxRetries(3)

	tests := []struct {
		name     string
		err      error
		attempt  int
		expected bool
	}{
		{"nil error", nil, 0, false},
		{"401 unauthorized", errors.New("401 Unauthorized"), 0, false},
		{"403 forbidden", errors.New("403 Forbidden"), 0, false},
		{"500 server error attempt 0", errors.New("500 error"), 0, true},
		{"500 server error attempt 2", errors.New("500 error"), 2, true},
		{"500 server error max retries", errors.New("500 error"), 3, false},
		{"context canceled", context.Canceled, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.ShouldRetry(tt.err, tt.attempt)
			if result != tt.expected {
				t.Errorf("ShouldRetry() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDefaultRecoveryStrategy_GetRetryDelay(t *testing.T) {
	s := NewDefaultRecoveryStrategy().WithBaseDelay(1 * time.Second).WithMaxDelay(30 * time.Second)

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
		{5, 30 * time.Second}, // 超过 maxDelay
		{6, 30 * time.Second}, // 超过 maxDelay
	}

	for _, tt := range tests {
		t.Run(string(rune('0'+tt.attempt)), func(t *testing.T) {
			delay := s.GetRetryDelay(tt.attempt)
			if delay != tt.expected {
				t.Errorf("GetRetryDelay(%d) = %v, want %v", tt.attempt, delay, tt.expected)
			}
		})
	}
}

func TestHTTPError(t *testing.T) {
	err := NewHTTPError(404, "Not Found")
	if err.Error() != "Not Found" {
		t.Errorf("expected error message 'Not Found', got %s", err.Error())
	}

	var httpErr interface{ StatusCode() int }
	if !errors.As(err, &httpErr) {
		t.Error("expected error to implement StatusCode()")
	}
	if httpErr.StatusCode() != 404 {
		t.Errorf("expected status code 404, got %d", httpErr.StatusCode())
	}
}

func TestIsHTTPStatusError(t *testing.T) {
	err404 := NewHTTPError(404, "Not Found")
	err500 := NewHTTPError(500, "Internal Error")

	if !isHTTPStatusError(err404, 404) {
		t.Error("expected isHTTPStatusError 404 = true")
	}
	if isHTTPStatusError(err404, 500) {
		t.Error("expected isHTTPStatusError 500 = false")
	}
	if !isHTTPStatusError(err500, 500) {
		t.Error("expected isHTTPStatusError 500 = true")
	}
	if isHTTPStatusError(nil, 404) {
		t.Error("expected isHTTPStatusError nil = false")
	}
}

func TestContextWithUserID(t *testing.T) {
	ctx := ContextWithUserID(context.Background(), 123)
	userID := userIDFromContext(ctx)
	if userID != 123 {
		t.Errorf("expected userID=123, got %d", userID)
	}
}

func TestUserIDFromContext_Default(t *testing.T) {
	userID := userIDFromContext(context.Background())
	if userID != 0 {
		t.Errorf("expected userID=0 for empty context, got %d", userID)
	}
}

func TestEstimateTokenUsage(t *testing.T) {
	tests := []struct {
		contentLen int
		expected   int
	}{
		{0, 0},
		{-1, 0},
		{1, 1},
		{25, 10},
		{100, 40},
		{1000, 400},
	}

	for _, tt := range tests {
		t.Run(string(rune('0'+tt.contentLen%10)), func(t *testing.T) {
			result := estimateTokenUsage(tt.contentLen)
			if tt.contentLen <= 0 {
				if result != nil {
					t.Errorf("expected nil for contentLen=%d", tt.contentLen)
				}
			} else {
				if result == nil {
					t.Errorf("expected non-nil for contentLen=%d", tt.contentLen)
				} else if result.CompletionTokens != tt.expected {
					t.Errorf("contentLen=%d: expected %d tokens, got %d", tt.contentLen, tt.expected, result.CompletionTokens)
				}
			}
		})
	}
}

func TestTokenUsageStore_NoInit(t *testing.T) {
	// 未初始化时，CheckQuota 应该返回 nil
	err := CheckQuota(123)
	if err != nil {
		t.Errorf("expected nil error when quota not initialized, got %v", err)
	}

	total, byUser := GetTokenUsage()
	if total != 0 {
		t.Errorf("expected total=0 when not initialized, got %d", total)
	}
	if byUser != nil {
		t.Errorf("expected nil byUser when not initialized, got %v", byUser)
	}
}

func TestInitTokenQuota_MemoryMode(t *testing.T) {
	InitTokenQuota(1000, 100, nil)
	defer StopTokenQuota()

	err := CheckQuota(1)
	if err != nil {
		t.Errorf("expected nil error within quota, got %v", err)
	}

	RecordTokenUsage(1, 50)

	total, byUser := GetTokenUsage()
	if total != 50 {
		t.Errorf("expected total=50, got %d", total)
	}
	if byUser[1] != 50 {
		t.Errorf("expected user 1 = 50, got %d", byUser[1])
	}
}

func TestCheckQuota_DailyLimit(t *testing.T) {
	InitTokenQuota(100, 50, nil)
	defer StopTokenQuota()

	RecordTokenUsage(1, 100)

	err := CheckQuota(1)
	if err == nil {
		t.Error("expected error when daily quota exceeded")
	}
}

func TestCheckQuota_PerUserLimit(t *testing.T) {
	InitTokenQuota(1000, 50, nil)
	defer StopTokenQuota()

	RecordTokenUsage(1, 50)

	err := CheckQuota(1)
	if err == nil {
		t.Error("expected error when per-user quota exceeded")
	}

	// 用户2应该还能使用
	err = CheckQuota(2)
	if err != nil {
		t.Errorf("expected user 2 to have quota, got %v", err)
	}
}
