package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/ai/rag"
)

// fakeTool 实现 rag.ToolExecutor，用于测试带工具的 agent 调用。
type fakeTool struct{}

func (f *fakeTool) Execute(ctx context.Context, params map[string]interface{}) (*rag.ToolResult, error) {
	return &rag.ToolResult{ToolName: "fake", Success: true, Result: "ok"}, nil
}
func (f *fakeTool) Definition() rag.ToolDefinition {
	return rag.ToolDefinition{Name: "fake", Description: "fake tool", Parameters: map[string]interface{}{}}
}

// mockProvider 返回固定内容，便于测试编排流程（不触发工具调用）。
type mockProvider struct {
	lastSystem string
	lastMsgs   []provider.Message
}

func (m *mockProvider) Chat(ctx context.Context, system string, messages []provider.Message, opts ...provider.Option) (*provider.LLMResponse, error) {
	m.lastSystem = system
	m.lastMsgs = messages
	return &provider.LLMResponse{Content: "answer"}, nil
}
func (m *mockProvider) StreamChat(ctx context.Context, system string, messages []provider.Message, opts ...provider.Option) (<-chan provider.StreamChunk, error) {
	return nil, nil
}
func (m *mockProvider) Capabilities() provider.ProviderCapabilities {
	return provider.ProviderCapabilities{FunctionCalling: true, Streaming: true}
}

func mkAgents() []*Agent {
	return []*Agent{
		{Name: "retriever", Role: "检索", SystemPrompt: "你负责检索"},
		{Name: "summarizer", Role: "总结", SystemPrompt: "你负责总结"},
	}
}

func TestRunSequential(t *testing.T) {
	p := &mockProvider{}
	orch := NewOrchestrator(p, ModeSequential, mkAgents())
	res, err := orch.Run(context.Background(), "任务", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}
	if res[0].Agent != "retriever" || res[1].Agent != "summarizer" {
		t.Fatalf("order wrong: %+v", res)
	}
}

func TestRunParallel(t *testing.T) {
	p := &mockProvider{}
	orch := NewOrchestrator(p, ModeParallel, mkAgents())
	res, err := orch.Run(context.Background(), "任务", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}
}

func TestRunRouterInvalidModeFallback(t *testing.T) {
	// 空模式应退化为 sequential
	p := &mockProvider{}
	orch := NewOrchestrator(p, "", mkAgents())
	if orch.Mode != ModeSequential {
		t.Fatalf("empty mode should fallback to sequential, got %s", orch.Mode)
	}
}

func TestRunNoAgents(t *testing.T) {
	p := &mockProvider{}
	orch := NewOrchestrator(p, ModeSequential, nil)
	if _, err := orch.Run(context.Background(), "x", nil); err == nil {
		t.Fatal("expected error when no agents")
	}
}

func TestAggregate(t *testing.T) {
	results := []Result{
		{Agent: "a", Role: "检索", Output: "结果一"},
		{Agent: "b", Role: "总结", Output: "结果二"},
		{Agent: "c", Role: "校验", Error: "失败"},
	}
	out := Aggregate(results)
	if !strings.Contains(out, "结果一") || !strings.Contains(out, "结果二") {
		t.Fatal("missing outputs in aggregate")
	}
	if !strings.Contains(out, "失败") {
		t.Fatal("missing error note in aggregate")
	}
}

func TestToolsMap(t *testing.T) {
	// 验证 agent.go 中 toolsMap 存在且可用（间接通过 callWithTools 不 panic）。
	// 这里仅确认 Orchestrator 构造对带工具的 agent 不报错。
	ft := &fakeTool{}
	a := &Agent{Name: "t", Role: "tool", Tools: []rag.ToolExecutor{ft}}
	p := &mockProvider{}
	orch := NewOrchestrator(p, ModeSequential, []*Agent{a})
	if _, err := orch.Run(context.Background(), "q", nil); err != nil {
		t.Fatalf("tool agent run failed: %v", err)
	}
}
