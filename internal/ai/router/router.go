// Package router 实现多模型智能路由：根据请求特征（复杂度、是否需推理、
// 成本预算、延迟要求）在多个 Provider 之间动态选择最优模型，并支持失败回退。
package router

import (
	"context"
	logx "github.com/listen808/citebase/internal/log"
	"strings"
	"sync"

	"github.com/listen808/citebase/internal/ai/provider"
)

// Strategy 路由策略。
type Strategy string

const (
	// StrategyCost 优先低成本/低延迟模型（fast），失败再回退。
	StrategyCost Strategy = "cost"
	// StrategyQuality 优先高质量模型（reasoning/primary）。
	StrategyQuality Strategy = "quality"
	// StrategyBalanced 平衡模式：按复杂度启发式选择（默认）。
	StrategyBalanced Strategy = "balanced"
	// StrategyLatency 优先最低延迟（fast），适合流式/交互场景。
	StrategyLatency Strategy = "latency"
)

// Candidate 一个可路由的模型候选。
type Candidate struct {
	Provider  provider.Provider
	Tier      string // "primary" | "fast" | "reasoning"
	CostTier  int    // 1=最便宜 ... 3=最贵，用于成本排序
	Quality   int    // 1..3 质量评分
	LatencyMs int64  // 经验延迟（可选，0 表示未知）
	Enabled   bool
}

// Router 多模型路由器。
type Router struct {
	mu         sync.RWMutex
	candidates map[string]*Candidate // key = tier
	strategy   Strategy
	// 复杂度启发式阈值（按 prompt 长度近似）
	complexityTokens int
}

// NewRouter 创建路由器，candidates 至少包含一个 primary。
func NewRouter(primary, fast, reasoning provider.Provider) *Router {
	r := &Router{
		candidates:       make(map[string]*Candidate),
		strategy:         StrategyBalanced,
		complexityTokens: 2000,
	}
	if primary != nil {
		r.candidates["primary"] = &Candidate{Provider: primary, Tier: "primary", CostTier: 3, Quality: 3, Enabled: true}
	}
	if fast != nil {
		r.candidates["fast"] = &Candidate{Provider: fast, Tier: "fast", CostTier: 1, Quality: 1, Enabled: true}
	}
	if reasoning != nil {
		r.candidates["reasoning"] = &Candidate{Provider: reasoning, Tier: "reasoning", CostTier: 3, Quality: 3, Enabled: true}
	}
	return r
}

// SetStrategy 设置全局路由策略。
func (r *Router) SetStrategy(s Strategy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s == "" {
		s = StrategyBalanced
	}
	r.strategy = s
}

// Strategy 返回当前策略。
func (r *Router) Strategy() Strategy {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.strategy
}

// estimateComplexity 基于 prompt 长度粗略估算复杂度（token 近似）。
func estimateComplexity(prompt string) int {
	// 中文约1.5字/token，英文约4字符/token，取 2.5 均值。
	return len([]rune(prompt)) * 10 / 25
}

// Select 根据策略与上下文选择最优 Provider，返回候选与回退候选列表。
func (r *Router) Select(ctx context.Context, prompt string, opts ...SelectOption) (provider.Provider, []provider.Provider) {
	cfg := &selectConfig{}
	for _, o := range opts {
		o(cfg)
	}

	r.mu.RLock()
	strategy := r.strategy
	if cfg.strategy != "" {
		strategy = cfg.strategy
	}
	// 拷贝可用候选
	avail := make([]*Candidate, 0, len(r.candidates))
	for _, c := range r.candidates {
		if c.Enabled && c.Provider != nil {
			avail = append(avail, c)
		}
	}
	r.mu.RUnlock()

	if len(avail) == 0 {
		return nil, nil
	}

	complexity := estimateComplexity(prompt)
	if cfg.forceTier != "" {
		if c, ok := r.candidateByTier(cfg.forceTier); ok {
			return c.Provider, r.fallbacks(c, avail)
		}
	}

	var chosen *Candidate
	switch strategy {
	case StrategyCost, StrategyLatency:
		chosen = pickMin(avail, func(c *Candidate) int {
			if strategy == StrategyLatency && c.LatencyMs > 0 {
				return int(c.LatencyMs)
			}
			return c.CostTier
		})
	case StrategyQuality:
		chosen = pickMax(avail, func(c *Candidate) int { return c.Quality })
	default: // Balanced：按复杂度启发式
		if complexity > r.complexityTokens {
			// 复杂任务优先 reasoning，否则 primary
			if c, ok := r.candidateByTier("reasoning"); ok {
				chosen = c
			} else {
				chosen, _ = r.candidateByTier("primary")
			}
		} else {
			// 简单任务优先 fast 以省成本
			if c, ok := r.candidateByTier("fast"); ok {
				chosen = c
			} else {
				chosen, _ = r.candidateByTier("primary")
			}
		}
	}

	if chosen == nil {
		chosen = avail[0]
	}
	return chosen.Provider, r.fallbacks(chosen, avail)
}

// candidateByTier 返回指定 tier 的候选。
func (r *Router) candidateByTier(tier string) (*Candidate, bool) {
	c, ok := r.candidates[tier]
	return c, ok && c.Enabled && c.Provider != nil
}

// SelectByContextLength P2-4 (§3.3.1): 按上下文 token 数选择模型
// 路由策略：
//   - tokenCount < 8k → fast（若有）
//   - 8k ≤ tokenCount < 32k → primary
//   - 32k ≤ tokenCount < 128k → primary（需支持长上下文）
//   - tokenCount ≥ 128k → 优先选择 ContextWindow 最大的 provider
//
// 返回选择的 provider 和回退列表。若所有 provider 的 ContextWindow 都不足，仍返回最大的（让 LLM 自行截断）。
func (r *Router) SelectByContextLength(tokenCount int) (provider.Provider, []provider.Provider) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	avail := make([]*Candidate, 0, len(r.candidates))
	for _, c := range r.candidates {
		if c.Enabled && c.Provider != nil {
			avail = append(avail, c)
		}
	}
	if len(avail) == 0 {
		return nil, nil
	}

	// 按上下文窗口降序排序（最大窗口优先）
	sortByContextWindow(avail)

	var chosen *Candidate
	switch {
	case tokenCount < 8000:
		// 短上下文优先 fast
		if c, ok := r.candidateByTier("fast"); ok {
			chosen = c
		}
	case tokenCount < 32000:
		// 中等上下文用 primary
		if c, ok := r.candidateByTier("primary"); ok {
			chosen = c
		}
	case tokenCount < 128000:
		// 长上下文：选 ContextWindow >= tokenCount 的 primary，否则取最大窗口
		chosen = pickByContextWindow(avail, tokenCount)
	default:
		// 超长上下文（>128k）：必须选最大窗口的 provider
		chosen = avail[0] // avail 已按 ContextWindow 降序排序
	}

	if chosen == nil {
		chosen = avail[0]
	}
	return chosen.Provider, r.fallbacks(chosen, avail)
}

// sortByContextWindow 按 ContextWindow 降序排序（原地）
func sortByContextWindow(cands []*Candidate) {
	for i := 1; i < len(cands); i++ {
		for j := i; j > 0; j-- {
			cwJ := cands[j].Provider.Capabilities().ContextWindow
			cwJMinus1 := cands[j-1].Provider.Capabilities().ContextWindow
			if cwJ > cwJMinus1 {
				cands[j], cands[j-1] = cands[j-1], cands[j]
			} else {
				break
			}
		}
	}
}

// pickByContextWindow 选择第一个 ContextWindow >= required 的候选；若无，返回最大窗口的
func pickByContextWindow(cands []*Candidate, required int) *Candidate {
	for _, c := range cands {
		if c.Provider.Capabilities().ContextWindow >= required {
			return c
		}
	}
	// 所有 provider 都不够大，返回最大的
	return cands[0]
}

// fallbacks 返回除 chosen 外的其余候选（用于失败回退）。
func (r *Router) fallbacks(chosen *Candidate, avail []*Candidate) []provider.Provider {
	out := make([]provider.Provider, 0, len(avail)-1)
	for _, c := range avail {
		if c.Tier != chosen.Tier {
			out = append(out, c.Provider)
		}
	}
	return out
}

// ChatWithFallback 通过路由选择模型并执行 Chat，失败时自动回退到下一个候选。
func (r *Router) ChatWithFallback(ctx context.Context, system string, messages []provider.Message, opts ...Option) (string, error) {
	p, fallbacks := r.Select(ctx, system)
	if p == nil {
		return "", errNoProvider
	}

	chain := append([]provider.Provider{p}, fallbacks...)
	var lastErr error
	for _, cand := range chain {
		resp, err := cand.Chat(ctx, system, messages, toProviderOpts(opts...)...)
		if err != nil {
			lastErr = err
			logx.Infof("[router] provider %s failed, trying fallback: %v", cand.Name(), err)
			continue
		}
		if resp != nil && strings.TrimSpace(resp.Content) != "" {
			return resp.Content, nil
		}
		lastErr = errEmptyResponse
	}
	return "", lastErr
}

// selectConfig 单次选择的覆盖配置。
type selectConfig struct {
	strategy  Strategy
	forceTier string
}

// SelectOption 单次选择的可选参数。
type SelectOption func(*selectConfig)

// WithStrategy 覆盖本次选择的策略。
func WithStrategy(s Strategy) SelectOption {
	return func(c *selectConfig) { c.strategy = s }
}

// WithForceTier 强制使用指定 tier（primary/fast/reasoning）。
func WithForceTier(tier string) SelectOption {
	return func(c *selectConfig) { c.forceTier = tier }
}

func pickMin(cands []*Candidate, score func(*Candidate) int) *Candidate {
	best := cands[0]
	bestScore := score(best)
	for _, c := range cands[1:] {
		if s := score(c); s < bestScore {
			best, bestScore = c, s
		}
	}
	return best
}

func pickMax(cands []*Candidate, score func(*Candidate) int) *Candidate {
	best := cands[0]
	bestScore := score(best)
	for _, c := range cands[1:] {
		if s := score(c); s > bestScore {
			best, bestScore = c, s
		}
	}
	return best
}
