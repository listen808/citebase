package provider

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

// isRetryableError 判断错误是否可重试
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := strings.ToLower(err.Error())
	retryableSignals := []string{
		"timeout", "time out", "deadline",
		"429", "too many requests", "rate limit",
		"500", "502", "503", "504",
		"service unavailable", "internal server error",
		"bad gateway", "gateway timeout",
		"connection refused", "connection reset", "no such host",
		"temporary failure", "try again later",
		"context deadline exceeded",
	}
	for _, signal := range retryableSignals {
		if strings.Contains(errMsg, signal) {
			return true
		}
	}
	return false
}

// WithRetry 对 LLM 调用进行指数退避重试
// maxRetries: 最大重试次数（不含首次）
// baseDelay: 基础延迟（每次翻倍）
func WithRetry(ctx context.Context, maxRetries int, baseDelay time.Duration, fn func() error) error {
	var err error
	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			backoff := time.Duration(math.Pow(2, float64(i-1))) * baseDelay
			select {
			case <-ctx.Done():
				return fmt.Errorf("retry cancelled: %w", ctx.Err())
			case <-time.After(backoff):
			}
		}
		err = fn()
		if err == nil {
			return nil
		}
		if !isRetryableError(err) {
			return fmt.Errorf("non-retryable error: %w", err)
		}
	}
	return fmt.Errorf("all %d retries exhausted: %w", maxRetries, err)
}
