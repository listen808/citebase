package provider

import (
	"context"
	"errors"
	"testing"
	"time"
)

// stubProvider 只实现测试用到的流式方法，其余方法 panic（测试不应触达）。
type stubProvider struct {
	streamChunks []StreamChunk
	streamErr    error
}

func (s *stubProvider) Chat(ctx context.Context, system string, messages []Message, opts ...Option) (*LLMResponse, error) {
	panic("not used in test")
}

func (s *stubProvider) StreamChat(ctx context.Context, system string, messages []Message, opts ...Option) (<-chan StreamChunk, error) {
	if s.streamErr != nil {
		return nil, s.streamErr
	}
	ch := make(chan StreamChunk, len(s.streamChunks))
	for _, c := range s.streamChunks {
		ch <- c
	}
	close(ch)
	return ch, nil
}

func (s *stubProvider) ChatWithTools(ctx context.Context, system string, messages []Message, tools []ToolDefinition, opts ...Option) (*LLMResponse, error) {
	panic("not used in test")
}

func (s *stubProvider) StreamChatWithTools(ctx context.Context, system string, messages []Message, tools []ToolDefinition, opts ...Option) (<-chan StreamChunk, error) {
	if s.streamErr != nil {
		return nil, s.streamErr
	}
	ch := make(chan StreamChunk, len(s.streamChunks))
	for _, c := range s.streamChunks {
		ch <- c
	}
	close(ch)
	return ch, nil
}

func (s *stubProvider) GetEmbedding(ctx context.Context, text string) ([]float64, error) {
	panic("not used in test")
}

func (s *stubProvider) GetEmbeddings(ctx context.Context, texts []string) ([][]float64, error) {
	panic("not used in test")
}

func (s *stubProvider) Name() string {
	return "stub"
}

func (s *stubProvider) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{}
}

func drainStream(t *testing.T, ch <-chan StreamChunk) []StreamChunk {
	t.Helper()
	var out []StreamChunk
	for c := range ch {
		out = append(out, c)
	}
	return out
}

// 回归测试：流中出现 Error chunk 后，熔断器必须记失败，且不得在流结束时记假成功。
// 修复前的写法在 channel 耗尽后无条件 RecordSuccess —— 连续 5 次「带错误的流」
// 本应触发熔断（maxFailures=5），旧代码下熔断器失败计数恒为 0，永远不会开路。
func TestStreamChatMidStreamErrorRecordsFailure(t *testing.T) {
	inner := &stubProvider{
		streamChunks: []StreamChunk{
			{Content: "ok"},
			{Error: "mid-stream boom"},
			{Content: "tail"},
		},
	}
	// 短 resetTimeout 无关紧要，这里只验证状态与计数
	pp := NewPooledProvider(inner, 5, time.Hour)

	for i := 0; i < 5; i++ {
		ch, err := pp.StreamChat(context.Background(), "", nil)
		if err != nil {
			t.Fatalf("StreamChat 返回错误: %v", err)
		}
		chunks := drainStream(t, ch)
		if len(chunks) != 3 {
			t.Fatalf("第 %d 轮期望 3 个 chunk，得到 %d", i+1, len(chunks))
		}
	}

	if got := pp.Breaker().Failures(); got != 5 {
		t.Fatalf("期望失败计数 5，得到 %d", got)
	}
	if got := pp.Breaker().State(); got != StateOpen {
		t.Fatalf("期望熔断器开路（open），得到 %s", got)
	}
}

// 对照组：无错误的流结束时记成功，失败计数清零。
func TestStreamChatCleanStreamRecordsSuccess(t *testing.T) {
	inner := &stubProvider{
		streamChunks: []StreamChunk{{Content: "ok"}, {Content: "done", Done: true}},
	}
	pp := NewPooledProvider(inner, 5, time.Hour)
	// 先制造一次失败，验证成功能清零
	pp.Breaker().RecordFailure()
	if got := pp.Breaker().Failures(); got != 1 {
		t.Fatalf("预置失败后计数应为 1，得到 %d", got)
	}

	ch, err := pp.StreamChat(context.Background(), "", nil)
	if err != nil {
		t.Fatalf("StreamChat 返回错误: %v", err)
	}
	drainStream(t, ch)

	if got := pp.Breaker().Failures(); got != 0 {
		t.Fatalf("干净流后失败计数应清零，得到 %d", got)
	}
	if got := pp.Breaker().State(); got != StateClosed {
		t.Fatalf("干净流后熔断器应关闭（closed），得到 %s", got)
	}
}

// 启动阶段错误（StreamChat 直接返回 error）记失败。
func TestStreamChatStartupErrorRecordsFailure(t *testing.T) {
	inner := &stubProvider{streamErr: errors.New("connect refused")}
	pp := NewPooledProvider(inner, 1, time.Hour)

	_, err := pp.StreamChat(context.Background(), "", nil)
	if err == nil {
		t.Fatal("期望返回错误")
	}
	if got := pp.Breaker().Failures(); got != 1 {
		t.Fatalf("期望失败计数 1，得到 %d", got)
	}
}

// StreamChatWithTools 的流包装与 StreamChat 共用 wrapStream，行为应一致。
func TestStreamChatWithToolsMidStreamErrorRecordsFailure(t *testing.T) {
	inner := &stubProvider{
		streamChunks: []StreamChunk{{Content: "x"}, {Error: "boom"}},
	}
	pp := NewPooledProvider(inner, 5, time.Hour)

	for i := 0; i < 5; i++ {
		ch, err := pp.StreamChatWithTools(context.Background(), "", nil, nil)
		if err != nil {
			t.Fatalf("StreamChatWithTools 返回错误: %v", err)
		}
		drainStream(t, ch)
	}
	if got := pp.Breaker().State(); got != StateOpen {
		t.Fatalf("期望熔断器开路（open），得到 %s", got)
	}
}
