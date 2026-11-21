package rag

import (
	"fmt"
	"sync"
	"time"
)

// RateLimiterConfig 速率限制配置
type RateLimiterConfig struct {
	Enabled        bool `yaml:"enabled" json:"enabled"`
	RequestsPerMin int  `yaml:"requests_per_min" json:"requests_per_min"` // 每分钟请求数限制
	TokensPerDay   int  `yaml:"tokens_per_day" json:"tokens_per_day"`     // 每日Token限制
	MaxConcurrent  int  `yaml:"max_concurrent" json:"max_concurrent"`     // 最大并发数
	BurstSize      int  `yaml:"burst_size" json:"burst_size"`             // 突发容量
}

func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		Enabled:        true,
		RequestsPerMin: 60,
		TokensPerDay:   100000,
		MaxConcurrent:  10,
		BurstSize:      20,
	}
}

// UserRateState 用户速率状态
type UserRateState struct {
	RequestCount int       `json:"request_count"`
	TokenUsed    int       `json:"token_used"`
	WindowStart  time.Time `json:"window_start"`
	DayStart     time.Time `json:"day_start"`
	Concurrent   int       `json:"concurrent"`
}

// RateLimiter 速率限制器
type RateLimiter struct {
	config RateLimiterConfig
	states map[int]*UserRateState // userID -> state
	mu     sync.Mutex
}

func NewRateLimiter(cfg RateLimiterConfig) *RateLimiter {
	return &RateLimiter{
		config: cfg,
		states: make(map[int]*UserRateState),
	}
}

// CheckRequest 检查请求是否被允许
func (rl *RateLimiter) CheckRequest(userID int, tokenEstimate int) error {
	if !rl.config.Enabled {
		return nil
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	state := rl.getOrCreateState(userID)
	now := time.Now()

	// 重置分钟窗口
	if now.Sub(state.WindowStart) > time.Minute {
		state.RequestCount = 0
		state.WindowStart = now
	}

	// 重置日窗口
	if now.Sub(state.DayStart) > 24*time.Hour {
		state.TokenUsed = 0
		state.DayStart = now
	}

	// 检查请求数限制
	if state.RequestCount >= rl.config.RequestsPerMin {
		return fmt.Errorf("rate limit exceeded: %d requests per minute", rl.config.RequestsPerMin)
	}

	// 检查Token限制
	if rl.config.TokensPerDay > 0 && state.TokenUsed+tokenEstimate > rl.config.TokensPerDay {
		return fmt.Errorf("token limit exceeded: %d tokens per day", rl.config.TokensPerDay)
	}

	// 检查并发限制
	if state.Concurrent >= rl.config.MaxConcurrent {
		return fmt.Errorf("concurrent limit exceeded: %d", rl.config.MaxConcurrent)
	}

	return nil
}

// RecordRequest 记录请求
func (rl *RateLimiter) RecordRequest(userID int, tokensUsed int) {
	if !rl.config.Enabled {
		return
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	state := rl.getOrCreateState(userID)
	state.RequestCount++
	state.TokenUsed += tokensUsed
}

// AcquireConcurrent 获取并发槽
func (rl *RateLimiter) AcquireConcurrent(userID int) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	state := rl.getOrCreateState(userID)
	if state.Concurrent >= rl.config.MaxConcurrent {
		return fmt.Errorf("concurrent limit reached")
	}
	state.Concurrent++
	return nil
}

// ReleaseConcurrent 释放并发槽
func (rl *RateLimiter) ReleaseConcurrent(userID int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	state := rl.getOrCreateState(userID)
	if state.Concurrent > 0 {
		state.Concurrent--
	}
}

// GetUsage 获取用户使用情况
func (rl *RateLimiter) GetUsage(userID int) map[string]interface{} {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	state := rl.getOrCreateState(userID)
	return map[string]interface{}{
		"requests_this_minute": state.RequestCount,
		"requests_limit":       rl.config.RequestsPerMin,
		"tokens_today":         state.TokenUsed,
		"tokens_limit":         rl.config.TokensPerDay,
		"concurrent":           state.Concurrent,
		"concurrent_limit":     rl.config.MaxConcurrent,
	}
}

// GetOrCreateState 获取或创建用户状态
func (rl *RateLimiter) getOrCreateState(userID int) *UserRateState {
	state, ok := rl.states[userID]
	if !ok {
		state = &UserRateState{
			WindowStart: time.Now(),
			DayStart:    time.Now(),
		}
		rl.states[userID] = state
	}
	return state
}

// Cleanup 清理过期状态
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for userID, state := range rl.states {
		if now.Sub(state.DayStart) > 48*time.Hour {
			delete(rl.states, userID)
		}
	}
}

// ============================================================
// Token 成本追踪
// ============================================================

// TokenCostEntry Token成本记录
type TokenCostEntry struct {
	UserID           int       `json:"user_id"`
	Model            string    `json:"model"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	CostUSD          float64   `json:"cost_usd"`
	Timestamp        time.Time `json:"timestamp"`
}

// TokenCostTracker Token成本追踪器
type TokenCostTracker struct {
	entries []TokenCostEntry
	mu      sync.Mutex
	config  CostTrackerConfig
}

type CostTrackerConfig struct {
	Enabled    bool               `yaml:"enabled" json:"enabled"`
	MaxEntries int                `yaml:"max_entries" json:"max_entries"`
	ModelCosts map[string]float64 `yaml:"model_costs" json:"model_costs"` // 模型每1K token成本(USD)
}

func DefaultCostTrackerConfig() CostTrackerConfig {
	return CostTrackerConfig{
		Enabled:    true,
		MaxEntries: 10000,
		ModelCosts: map[string]float64{
			"gpt-4":           0.03,
			"gpt-4-turbo":     0.01,
			"gpt-3.5-turbo":   0.0005,
			"claude-3-opus":   0.015,
			"claude-3-sonnet": 0.003,
			"deepseek-chat":   0.0001,
		},
	}
}

func NewTokenCostTracker(cfg CostTrackerConfig) *TokenCostTracker {
	return &TokenCostTracker{
		entries: make([]TokenCostEntry, 0, cfg.MaxEntries),
		config:  cfg,
	}
}

// Record 记录Token使用
func (t *TokenCostTracker) Record(userID int, model string, promptTokens, completionTokens int) {
	if !t.config.Enabled {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	totalTokens := promptTokens + completionTokens
	costPer1K := t.config.ModelCosts[model]
	costUSD := float64(totalTokens) / 1000.0 * costPer1K

	entry := TokenCostEntry{
		UserID:           userID,
		Model:            model,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		CostUSD:          costUSD,
		Timestamp:        time.Now(),
	}

	t.entries = append(t.entries, entry)

	// 限制记录数量
	if len(t.entries) > t.config.MaxEntries {
		t.entries = t.entries[len(t.entries)-t.config.MaxEntries:]
	}
}

// GetUserCost 获取用户总成本
func (t *TokenCostTracker) GetUserCost(userID int) float64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	total := 0.0
	for _, e := range t.entries {
		if e.UserID == userID {
			total += e.CostUSD
		}
	}
	return total
}

// GetDailyCost 获取每日总成本
func (t *TokenCostTracker) GetDailyCost(date time.Time) float64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	total := 0.0
	for _, e := range t.entries {
		if e.Timestamp.Year() == date.Year() &&
			e.Timestamp.Month() == date.Month() &&
			e.Timestamp.Day() == date.Day() {
			total += e.CostUSD
		}
	}
	return total
}

// GetModelUsage 获取模型使用统计
func (t *TokenCostTracker) GetModelUsage() map[string]int {
	t.mu.Lock()
	defer t.mu.Unlock()

	usage := make(map[string]int)
	for _, e := range t.entries {
		usage[e.Model] += e.TotalTokens
	}
	return usage
}
