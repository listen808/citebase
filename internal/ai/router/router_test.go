package router

import (
	"context"
	"testing"

	"github.com/listen808/citebase/internal/ai/provider"
)

type mockProvider struct {
	name          string
	contextWindow int
	chatFunc      func(ctx context.Context, system string, messages []provider.Message, opts ...provider.Option) (*provider.LLMResponse, error)
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Chat(ctx context.Context, system string, messages []provider.Message, opts ...provider.Option) (*provider.LLMResponse, error) {
	if m.chatFunc != nil {
		return m.chatFunc(ctx, system, messages, opts...)
	}
	return &provider.LLMResponse{Content: "mock response"}, nil
}
func (m *mockProvider) StreamChat(ctx context.Context, system string, messages []provider.Message, opts ...provider.Option) (<-chan provider.StreamChunk, error) {
	return nil, nil
}
func (m *mockProvider) ChatWithTools(ctx context.Context, system string, messages []provider.Message, tools []provider.ToolDefinition, opts ...provider.Option) (*provider.LLMResponse, error) {
	return nil, nil
}
func (m *mockProvider) StreamChatWithTools(ctx context.Context, system string, messages []provider.Message, tools []provider.ToolDefinition, opts ...provider.Option) (<-chan provider.StreamChunk, error) {
	return nil, nil
}
func (m *mockProvider) GetEmbedding(ctx context.Context, text string) ([]float64, error) {
	return nil, nil
}
func (m *mockProvider) GetEmbeddings(ctx context.Context, texts []string) ([][]float64, error) {
	return nil, nil
}
func (m *mockProvider) Capabilities() provider.ProviderCapabilities {
	return provider.ProviderCapabilities{ContextWindow: m.contextWindow}
}

func newMock(name string, ctxWindow int) *mockProvider {
	return &mockProvider{name: name, contextWindow: ctxWindow}
}

func TestNewRouter_Basic(t *testing.T) {
	primary := newMock("primary", 8000)
	r := NewRouter(primary, nil, nil)
	if r == nil {
		t.Fatal("expected non-nil Router")
	}
	if r.Strategy() != StrategyBalanced {
		t.Errorf("expected default strategy balanced, got %s", r.Strategy())
	}
}

func TestNewRouter_AllTiers(t *testing.T) {
	primary := newMock("primary", 8000)
	fast := newMock("fast", 4000)
	reasoning := newMock("reasoning", 32000)
	r := NewRouter(primary, fast, reasoning)
	if len(r.candidates) != 3 {
		t.Errorf("expected 3 candidates, got %d", len(r.candidates))
	}
}

func TestSetStrategy(t *testing.T) {
	primary := newMock("primary", 8000)
	r := NewRouter(primary, nil, nil)
	r.SetStrategy(StrategyCost)
	if r.Strategy() != StrategyCost {
		t.Errorf("expected cost, got %s", r.Strategy())
	}
	r.SetStrategy("")
	if r.Strategy() != StrategyBalanced {
		t.Errorf("expected balanced for empty, got %s", r.Strategy())
	}
}

func TestEstimateComplexity(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"hello", 2},
		{"你好", 0},
		{"你好世界abc", 2},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := estimateComplexity(tt.input); got != tt.want {
				t.Errorf("estimateComplexity(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestSelect_Empty(t *testing.T) {
	r := NewRouter(nil, nil, nil)
	p, fb := r.Select(context.Background(), "test")
	if p != nil {
		t.Error("expected nil provider for empty router")
	}
	if len(fb) != 0 {
		t.Errorf("expected 0 fallbacks, got %d", len(fb))
	}
}

func TestSelect_SinglePrimary(t *testing.T) {
	primary := newMock("primary", 8000)
	r := NewRouter(primary, nil, nil)
	p, fb := r.Select(context.Background(), "hi")
	if p == nil || p.Name() != "primary" {
		t.Errorf("expected primary, got %v", p)
	}
	if len(fb) != 0 {
		t.Errorf("expected 0 fallbacks, got %d", len(fb))
	}
}

func TestSelect_StrategyCost(t *testing.T) {
	primary := newMock("primary", 8000)
	fast := newMock("fast", 4000)
	r := NewRouter(primary, fast, nil)
	r.SetStrategy(StrategyCost)
	p, _ := r.Select(context.Background(), "test")
	if p == nil || p.Name() != "fast" {
		t.Errorf("cost strategy should pick fast, got %v", p)
	}
}

func TestSelect_StrategyQuality(t *testing.T) {
	primary := newMock("primary", 8000)
	fast := newMock("fast", 4000)
	r := NewRouter(primary, fast, nil)
	r.SetStrategy(StrategyQuality)
	p, _ := r.Select(context.Background(), "test")
	if p == nil || p.Name() != "primary" {
		t.Errorf("quality strategy should pick primary, got %v", p)
	}
}

func TestSelect_WithStrategyOption(t *testing.T) {
	primary := newMock("primary", 8000)
	fast := newMock("fast", 4000)
	r := NewRouter(primary, fast, nil)
	p, _ := r.Select(context.Background(), "test", WithStrategy(StrategyCost))
	if p == nil || p.Name() != "fast" {
		t.Errorf("WithStrategy(cost) should pick fast, got %v", p)
	}
}

func TestSelect_WithForceTier(t *testing.T) {
	primary := newMock("primary", 8000)
	fast := newMock("fast", 4000)
	reasoning := newMock("reasoning", 32000)
	r := NewRouter(primary, fast, reasoning)

	p, _ := r.Select(context.Background(), "test", WithForceTier("fast"))
	if p == nil || p.Name() != "fast" {
		t.Errorf("force fast should pick fast, got %v", p)
	}
	p, _ = r.Select(context.Background(), "test", WithForceTier("reasoning"))
	if p == nil || p.Name() != "reasoning" {
		t.Errorf("force reasoning should pick reasoning, got %v", p)
	}
}

func TestSelect_Balanced_Simple(t *testing.T) {
	primary := newMock("primary", 8000)
	fast := newMock("fast", 4000)
	r := NewRouter(primary, fast, nil)
	p, _ := r.Select(context.Background(), "hi")
	if p == nil || p.Name() != "fast" {
		t.Errorf("balanced simple should pick fast, got %v", p)
	}
}

func TestSelect_Balanced_Complex(t *testing.T) {
	primary := newMock("primary", 8000)
	fast := newMock("fast", 4000)
	reasoning := newMock("reasoning", 32000)
	r := NewRouter(primary, fast, reasoning)
	// complexityTokens=2000, need estimateComplexity > 2000
	// estimate = runes * 10 / 25, so need runes > 5000
	longPrompt := makeLongPrompt(6000)
	p, _ := r.Select(context.Background(), longPrompt)
	if p == nil || p.Name() != "reasoning" {
		t.Errorf("balanced complex should pick reasoning, got %v", p)
	}
}

func TestFallbacks(t *testing.T) {
	primary := newMock("primary", 8000)
	fast := newMock("fast", 4000)
	reasoning := newMock("reasoning", 32000)
	r := NewRouter(primary, fast, reasoning)
	p, fb := r.Select(context.Background(), "test", WithForceTier("fast"))
	if p.Name() != "fast" {
		t.Fatalf("expected fast, got %s", p.Name())
	}
	if len(fb) != 2 {
		t.Errorf("expected 2 fallbacks, got %d", len(fb))
	}
}

func TestPickMin(t *testing.T) {
	cands := []*Candidate{
		{Tier: "a", CostTier: 3},
		{Tier: "b", CostTier: 1},
		{Tier: "c", CostTier: 2},
	}
	result := pickMin(cands, func(c *Candidate) int { return c.CostTier })
	if result.Tier != "b" {
		t.Errorf("pickMin should pick b (cost=1), got %s", result.Tier)
	}
}

func TestPickMax(t *testing.T) {
	cands := []*Candidate{
		{Tier: "a", Quality: 2},
		{Tier: "b", Quality: 3},
		{Tier: "c", Quality: 1},
	}
	result := pickMax(cands, func(c *Candidate) int { return c.Quality })
	if result.Tier != "b" {
		t.Errorf("pickMax should pick b (quality=3), got %s", result.Tier)
	}
}

func TestSelectByContextLength_Short(t *testing.T) {
	primary := newMock("primary", 8000)
	fast := newMock("fast", 4000)
	r := NewRouter(primary, fast, nil)
	p, _ := r.SelectByContextLength(1000)
	if p == nil || p.Name() != "fast" {
		t.Errorf("short context should pick fast, got %v", p)
	}
}

func TestSelectByContextLength_Medium(t *testing.T) {
	primary := newMock("primary", 32000)
	fast := newMock("fast", 4000)
	r := NewRouter(primary, fast, nil)
	p, _ := r.SelectByContextLength(15000)
	if p == nil || p.Name() != "primary" {
		t.Errorf("medium context should pick primary, got %v", p)
	}
}

func TestSelectByContextLength_Long(t *testing.T) {
	primary := newMock("primary", 128000)
	fast := newMock("fast", 4000)
	r := NewRouter(primary, fast, nil)
	p, _ := r.SelectByContextLength(64000)
	if p == nil || p.Name() != "primary" {
		t.Errorf("long context should pick primary, got %v", p)
	}
}

func TestSelectByContextLength_VeryLong(t *testing.T) {
	primary := newMock("primary", 128000)
	fast := newMock("fast", 4000)
	r := NewRouter(primary, fast, nil)
	p, _ := r.SelectByContextLength(200000)
	if p == nil || p.Name() != "primary" {
		t.Errorf("very long context should pick max window, got %v", p)
	}
}

func TestSelectByContextLength_Empty(t *testing.T) {
	r := NewRouter(nil, nil, nil)
	p, fb := r.SelectByContextLength(1000)
	if p != nil {
		t.Error("expected nil for empty router")
	}
	if len(fb) != 0 {
		t.Errorf("expected 0 fallbacks, got %d", len(fb))
	}
}

func TestChatWithFallback_Success(t *testing.T) {
	primary := newMock("primary", 8000)
	fast := newMock("fast", 4000)
	r := NewRouter(primary, fast, nil)
	r.SetStrategy(StrategyCost)
	resp, err := r.ChatWithFallback(context.Background(), "sys", nil)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if resp == "" {
		t.Error("expected non-empty response")
	}
}

func TestChatWithFallback_NoProvider(t *testing.T) {
	r := NewRouter(nil, nil, nil)
	_, err := r.ChatWithFallback(context.Background(), "sys", nil)
	if err == nil {
		t.Error("expected error for no provider")
	}
}

func TestChatWithFallback_FallbackChain(t *testing.T) {
	callCount := 0
	failing := &mockProvider{
		name:          "failing",
		contextWindow: 4000,
		chatFunc: func(ctx context.Context, system string, messages []provider.Message, opts ...provider.Option) (*provider.LLMResponse, error) {
			callCount++
			return nil, context.DeadlineExceeded
		},
	}
	working := &mockProvider{
		name:          "working",
		contextWindow: 8000,
		chatFunc: func(ctx context.Context, system string, messages []provider.Message, opts ...provider.Option) (*provider.LLMResponse, error) {
			callCount++
			return &provider.LLMResponse{Content: "ok"}, nil
		},
	}
	r := &Router{
		candidates: map[string]*Candidate{
			"fast":    {Provider: failing, Tier: "fast", CostTier: 1, Quality: 1, Enabled: true},
			"primary": {Provider: working, Tier: "primary", CostTier: 3, Quality: 3, Enabled: true},
		},
		strategy:         StrategyCost,
		complexityTokens: 2000,
	}
	resp, err := r.ChatWithFallback(context.Background(), "sys", nil)
	if err != nil {
		t.Errorf("expected no error after fallback, got %v", err)
	}
	if resp != "ok" {
		t.Errorf("expected 'ok', got %s", resp)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls (fail+success), got %d", callCount)
	}
}

func makeLongPrompt(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}
