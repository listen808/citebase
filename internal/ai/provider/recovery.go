package provider

import (
	"context"
	"errors"
	"strings"
	"time"
)

// RecoveryAction 恢复动作
type RecoveryAction string

const (
	RecoveryRetry    RecoveryAction = "retry"
	RecoveryFallback RecoveryAction = "fallback"
	RecoveryFail     RecoveryAction = "fail"
)

// ErrorRecoveryStrategy 错误恢复策略接口
type ErrorRecoveryStrategy interface {
	HandleError(ctx context.Context, err error, attempt int) RecoveryAction
	ShouldRetry(err error, attempt int) bool
	GetRetryDelay(attempt int) time.Duration
}

// DefaultRecoveryStrategy 默认恢复策略
type DefaultRecoveryStrategy struct {
	maxRetries    int
	baseDelay     time.Duration
	maxDelay      time.Duration
	fallbackModel string
}

// NewDefaultRecoveryStrategy 创建默认恢复策略
func NewDefaultRecoveryStrategy() *DefaultRecoveryStrategy {
	return &DefaultRecoveryStrategy{
		maxRetries:    3,
		baseDelay:     1 * time.Second,
		maxDelay:      30 * time.Second,
		fallbackModel: "",
	}
}

// WithMaxRetries 设置最大重试次数
func (s *DefaultRecoveryStrategy) WithMaxRetries(n int) *DefaultRecoveryStrategy {
	s.maxRetries = n
	return s
}

// WithBaseDelay 设置基础延迟
func (s *DefaultRecoveryStrategy) WithBaseDelay(d time.Duration) *DefaultRecoveryStrategy {
	s.baseDelay = d
	return s
}

// WithMaxDelay 设置最大延迟
func (s *DefaultRecoveryStrategy) WithMaxDelay(d time.Duration) *DefaultRecoveryStrategy {
	s.maxDelay = d
	return s
}

// WithFallbackModel 设置降级模型
func (s *DefaultRecoveryStrategy) WithFallbackModel(model string) *DefaultRecoveryStrategy {
	s.fallbackModel = model
	return s
}

// HandleError 根据错误决定恢复动作
func (s *DefaultRecoveryStrategy) HandleError(ctx context.Context, err error, attempt int) RecoveryAction {
	if err == nil {
		return RecoveryFail
	}

	// context 已取消 → 直接失败
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return RecoveryFail
	}

	errStr := strings.ToLower(err.Error())

	// 401/403 认证错误 → 直接失败
	if strings.Contains(errStr, "401") || strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "403") || strings.Contains(errStr, "forbidden") {
		return RecoveryFail
	}

	// 429 限流 → 重试，使用更长延迟
	if strings.Contains(errStr, "429") || strings.Contains(errStr, "rate limit") {
		if attempt < s.maxRetries {
			return RecoveryRetry
		}
		if s.fallbackModel != "" {
			return RecoveryFallback
		}
		return RecoveryFail
	}

	// 500/502/503 服务器错误 → 重试
	if strings.Contains(errStr, "500") || strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") || strings.Contains(errStr, "server error") ||
		strings.Contains(errStr, "internal error") {
		if attempt < s.maxRetries {
			return RecoveryRetry
		}
		if s.fallbackModel != "" {
			return RecoveryFallback
		}
		return RecoveryFail
	}

	// 网络超时 → 重试
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") || strings.Contains(errStr, "eof") {
		if attempt < s.maxRetries {
			return RecoveryRetry
		}
		if s.fallbackModel != "" {
			return RecoveryFallback
		}
		return RecoveryFail
	}

	// 其他错误 → 重试到上限
	if attempt < s.maxRetries {
		return RecoveryRetry
	}
	return RecoveryFail
}

// ShouldRetry 是否应该重试
func (s *DefaultRecoveryStrategy) ShouldRetry(err error, attempt int) bool {
	if attempt >= s.maxRetries {
		return false
	}
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "401") || strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "403") || strings.Contains(errStr, "forbidden") {
		return false
	}
	return true
}

// GetRetryDelay 获取重试延迟（指数退避）
func (s *DefaultRecoveryStrategy) GetRetryDelay(attempt int) time.Duration {
	delay := s.baseDelay * time.Duration(1<<uint(attempt))
	if delay > s.maxDelay {
		return s.maxDelay
	}
	return delay
}

// FallbackModel 返回降级模型名
func (s *DefaultRecoveryStrategy) FallbackModel() string {
	return s.fallbackModel
}

// isHTTPStatusError 判断是否为指定 HTTP 状态码错误
func isHTTPStatusError(err error, statusCode int) bool {
	if err == nil {
		return false
	}
	var httpErr interface{ StatusCode() int }
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode() == statusCode
	}
	return false
}

// httpResponseError HTTP 响应错误
// 注意：字段名不能与方法名 StatusCode 冲突，故字段小写
type httpResponseError struct {
	code int
	Body string
}

func (e *httpResponseError) Error() string {
	return e.Body
}

func (e *httpResponseError) StatusCode() int {
	return e.code
}

// NewHTTPError 创建 HTTP 错误
func NewHTTPError(statusCode int, body string) error {
	return &httpResponseError{code: statusCode, Body: body}
}

// FallbackProvider 降级 provider
type FallbackProvider struct {
	primary  Provider
	fallback Provider
	strategy ErrorRecoveryStrategy
}

// NewFallbackProvider 创建降级 provider
func NewFallbackProvider(primary, fallback Provider, strategy ErrorRecoveryStrategy) *FallbackProvider {
	if strategy == nil {
		strategy = NewDefaultRecoveryStrategy()
	}
	return &FallbackProvider{primary: primary, fallback: fallback, strategy: strategy}
}

// ChatWithFallback 带降级的 Chat（system prompt 为空，由调用方在 messages 中构造）
func (f *FallbackProvider) ChatWithFallback(ctx context.Context, messages []Message) (string, error) {
	for attempt := 0; ; attempt++ {
		resp, err := f.primary.Chat(ctx, "", messages)
		if err == nil {
			return resp.Content, nil
		}

		action := f.strategy.HandleError(ctx, err, attempt)
		switch action {
		case RecoveryRetry:
			select {
			case <-time.After(f.strategy.GetRetryDelay(attempt)):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		case RecoveryFallback:
			if f.fallback != nil {
				resp, err := f.fallback.Chat(ctx, "", messages)
				if err != nil {
					return "", err
				}
				return resp.Content, nil
			}
			return "", err
		case RecoveryFail:
			return "", err
		}
	}
}

// ChatWithRetry 带重试的 Chat（不降级）
func ChatWithRetry(ctx context.Context, p Provider, system string, messages []Message, strategy ErrorRecoveryStrategy) (string, error) {
	if strategy == nil {
		strategy = NewDefaultRecoveryStrategy()
	}
	for attempt := 0; ; attempt++ {
		resp, err := p.Chat(ctx, system, messages)
		if err == nil {
			return resp.Content, nil
		}

		if !strategy.ShouldRetry(err, attempt) {
			return "", err
		}

		delay := strategy.GetRetryDelay(attempt)
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

// Compile-time assertion: httpResponseError 实现错误接口
var _ error = (*httpResponseError)(nil)
var _ interface{ StatusCode() int } = (*httpResponseError)(nil)
