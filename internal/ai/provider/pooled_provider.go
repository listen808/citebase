package provider

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// CircuitBreakerState 熔断器状态
type CircuitBreakerState string

const (
	StateClosed   CircuitBreakerState = "closed"
	StateOpen     CircuitBreakerState = "open"
	StateHalfOpen CircuitBreakerState = "half-open"
)

// CircuitBreaker 熔断器：在连续失败达到阈值后暂停请求，一段时间后尝试恢复
type CircuitBreaker struct {
	maxFailures   int
	resetTimeout  time.Duration
	failures      int
	lastFailure   time.Time
	state         CircuitBreakerState
	probeInFlight bool // P0-13: HalfOpen 状态下只允许一个探针请求通过
	mu            sync.Mutex
}

// NewCircuitBreaker 创建熔断器
// maxFailures: 触发熔断的连续失败次数
// resetTimeout: 熔断后等待多久进入半开状态
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	if maxFailures <= 0 {
		maxFailures = 5
	}
	if resetTimeout <= 0 {
		resetTimeout = 30 * time.Second
	}
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        StateClosed,
	}
}

// Allow 检查是否允许请求通过
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.lastFailure) >= cb.resetTimeout {
			// P0-13: 进入 HalfOpen 时仅放行一个探针请求
			cb.state = StateHalfOpen
			cb.probeInFlight = true
			return true
		}
		return false
	case StateHalfOpen:
		// P0-13: HalfOpen 状态下仅允许一个探针请求通过，其余请求拒绝，
		// 直到探针完成（RecordSuccess → Closed / RecordFailure → Open）
		if cb.probeInFlight {
			return false
		}
		cb.probeInFlight = true
		return true
	default:
		return true
	}
}

// RecordSuccess 记录成功调用
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.state = StateClosed
	cb.probeInFlight = false
}

// RecordFailure 记录失败调用
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailure = time.Now()

	// P0-13: HalfOpen 状态下探针请求失败，立即回到 Open 状态并重置探针标志
	if cb.state == StateHalfOpen {
		cb.state = StateOpen
		cb.probeInFlight = false
		// 不累加 failures：HalfOpen 的失败只表示探针未通过，原失败计数保留
		return
	}

	cb.failures++
	if cb.failures >= cb.maxFailures {
		cb.state = StateOpen
	}
}

// State 返回当前熔断器状态
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Failures 返回当前连续失败次数
func (cb *CircuitBreaker) Failures() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.failures
}

// Reset 重置熔断器
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.state = StateClosed
	cb.probeInFlight = false
}

// ErrCircuitOpen 熔断器打开错误
var ErrCircuitOpen = fmt.Errorf("circuit breaker is open")

// PooledProvider 带熔断器的 Provider 包装器
type PooledProvider struct {
	inner    Provider
	breaker  *CircuitBreaker
	recovery ErrorRecoveryStrategy // P1-8: 可选的错误恢复策略（设置后 Chat 走重试逻辑）
}

// NewPooledProvider 创建带熔断器的 Provider
func NewPooledProvider(inner Provider, maxFailures int, resetTimeout time.Duration) *PooledProvider {
	return &PooledProvider{
		inner:   inner,
		breaker: NewCircuitBreaker(maxFailures, resetTimeout),
	}
}

// Inner 返回内部 Provider
func (pp *PooledProvider) Inner() Provider {
	return pp.inner
}

// Breaker 返回熔断器
func (pp *PooledProvider) Breaker() *CircuitBreaker {
	return pp.breaker
}

// SetRecovery 设置错误恢复策略（P1-8）
// 设置后 Chat 将通过 ChatWithRetry 进行带指数退避的重试，并在不可重试错误时立即返回。
// 未设置时维持原有 breaker 短路逻辑，保持向后兼容。
func (pp *PooledProvider) SetRecovery(strategy ErrorRecoveryStrategy) {
	pp.recovery = strategy
}

// Recovery 返回当前恢复策略（可能为 nil）
func (pp *PooledProvider) Recovery() ErrorRecoveryStrategy {
	return pp.recovery
}

func (pp *PooledProvider) Chat(ctx context.Context, system string, messages []Message, opts ...Option) (*LLMResponse, error) {
	if !pp.breaker.Allow() {
		return nil, ErrCircuitOpen
	}
	// P1-8: 若配置了恢复策略，走带重试的 Chat（内部仍受 breaker 保护）
	if pp.recovery != nil {
		for attempt := 0; ; attempt++ {
			resp, err := pp.inner.Chat(ctx, system, messages, opts...)
			if err == nil {
				pp.breaker.RecordSuccess()
				return resp, nil
			}
			pp.breaker.RecordFailure()
			if !pp.recovery.ShouldRetry(err, attempt) {
				return nil, err
			}
			delay := pp.recovery.GetRetryDelay(attempt)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	resp, err := pp.inner.Chat(ctx, system, messages, opts...)
	if err != nil {
		pp.breaker.RecordFailure()
		return nil, err
	}
	pp.breaker.RecordSuccess()
	return resp, nil
}

func (pp *PooledProvider) StreamChat(ctx context.Context, system string, messages []Message, opts ...Option) (<-chan StreamChunk, error) {
	if !pp.breaker.Allow() {
		return nil, ErrCircuitOpen
	}
	ch, err := pp.inner.StreamChat(ctx, system, messages, opts...)
	if err != nil {
		pp.breaker.RecordFailure()
		return nil, err
	}
	return pp.wrapStream(ch), nil
}

// wrapStream 包装流式 channel：channel 内出现错误 chunk 时记失败并在流末尾记一次失败
// 兜底（消费方可能提前弃读，错误 chunk 无人接收）。
// 修复前的写法在 channel 耗尽后无条件 RecordSuccess —— 中途 chunk.Error 造成的失败
// 会被这次「假成功」冲掉，熔断器对持续出错的 provider 永远不开路。
func (pp *PooledProvider) wrapStream(ch <-chan StreamChunk) <-chan StreamChunk {
	wrapped := make(chan StreamChunk, cap(ch))
	go func() {
		defer close(wrapped)
		hadError := false
		for chunk := range ch {
			if chunk.Error != "" {
				hadError = true
				pp.breaker.RecordFailure()
			}
			wrapped <- chunk
		}
		if hadError {
			// 错误 chunk 已记过失败，这里不再补记（避免一次错误双重计数）；
			// 但也不能记成功 —— 直接返回。
			return
		}
		pp.breaker.RecordSuccess()
	}()
	return wrapped
}

func (pp *PooledProvider) ChatWithTools(ctx context.Context, system string, messages []Message, tools []ToolDefinition, opts ...Option) (*LLMResponse, error) {
	if !pp.breaker.Allow() {
		return nil, ErrCircuitOpen
	}
	resp, err := pp.inner.ChatWithTools(ctx, system, messages, tools, opts...)
	if err != nil {
		pp.breaker.RecordFailure()
		return nil, err
	}
	pp.breaker.RecordSuccess()
	return resp, nil
}

func (pp *PooledProvider) StreamChatWithTools(ctx context.Context, system string, messages []Message, tools []ToolDefinition, opts ...Option) (<-chan StreamChunk, error) {
	if !pp.breaker.Allow() {
		return nil, ErrCircuitOpen
	}
	ch, err := pp.inner.StreamChatWithTools(ctx, system, messages, tools, opts...)
	if err != nil {
		pp.breaker.RecordFailure()
		return nil, err
	}
	return pp.wrapStream(ch), nil
}

func (pp *PooledProvider) GetEmbedding(ctx context.Context, text string) ([]float64, error) {
	if !pp.breaker.Allow() {
		return nil, ErrCircuitOpen
	}
	vec, err := pp.inner.GetEmbedding(ctx, text)
	if err != nil {
		pp.breaker.RecordFailure()
		return nil, err
	}
	pp.breaker.RecordSuccess()
	return vec, nil
}

func (pp *PooledProvider) GetEmbeddings(ctx context.Context, texts []string) ([][]float64, error) {
	if !pp.breaker.Allow() {
		return nil, ErrCircuitOpen
	}
	vecs, err := pp.inner.GetEmbeddings(ctx, texts)
	if err != nil {
		pp.breaker.RecordFailure()
		return nil, err
	}
	pp.breaker.RecordSuccess()
	return vecs, nil
}

func (pp *PooledProvider) Name() string {
	return pp.inner.Name()
}

func (pp *PooledProvider) Capabilities() ProviderCapabilities {
	return pp.inner.Capabilities()
}
