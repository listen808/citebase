package text2sql

import (
	"context"
	"strings"
	"testing"

	"github.com/listen808/citebase/internal/ai/provider"
)

// fakeSelfValidateProvider 实现完整的 provider.Provider 接口，
// 只实现 Chat（llmValidateWith 用到的唯一方法），其余方法 panic ——
// 谁误调了它们，测试立刻红。
type fakeSelfValidateProvider struct {
	chatCalled bool
}

func (f *fakeSelfValidateProvider) Chat(ctx context.Context, system string, messages []provider.Message, opts ...provider.Option) (*provider.LLMResponse, error) {
	f.chatCalled = true
	return &provider.LLMResponse{Content: `{"is_valid": true, "confidence": 0.9, "issues": [], "suggestions": [], "corrected_sql": "SELECT 1"}`}, nil
}

func (f *fakeSelfValidateProvider) StreamChat(ctx context.Context, system string, messages []provider.Message, opts ...provider.Option) (<-chan provider.StreamChunk, error) {
	panic("not used in test")
}

func (f *fakeSelfValidateProvider) ChatWithTools(ctx context.Context, system string, messages []provider.Message, tools []provider.ToolDefinition, opts ...provider.Option) (*provider.LLMResponse, error) {
	panic("not used in test")
}

func (f *fakeSelfValidateProvider) StreamChatWithTools(ctx context.Context, system string, messages []provider.Message, tools []provider.ToolDefinition, opts ...provider.Option) (<-chan provider.StreamChunk, error) {
	panic("not used in test")
}

func (f *fakeSelfValidateProvider) GetEmbedding(ctx context.Context, text string) ([]float64, error) {
	panic("not used in test")
}

func (f *fakeSelfValidateProvider) GetEmbeddings(ctx context.Context, texts []string) ([][]float64, error) {
	panic("not used in test")
}

func (f *fakeSelfValidateProvider) Name() string { return "fake" }

func (f *fakeSelfValidateProvider) Capabilities() provider.ProviderCapabilities {
	return provider.ProviderCapabilities{}
}

// TestSelfValidatorFallbackToGlobalRealProvider 钉住：
// NewSQLSelfValidator(nil) 不再是"永远降级基础验证"——
// 构造时拿不到 provider，但全局 real provider 在调用时已注入，
// ValidateAndCorrect 必须走 LLM 深度验证（chat 被调到）。
// 此前两处调用点（execution_feedback_loop / multi_candidate_generator）
// 传 nil 且构造早于 app/server 的注入，LLM 校验整条链路静默失效，
// 0.8 的基础置信度被当成"验证通过"。
func TestSelfValidatorFallbackToGlobalRealProvider(t *testing.T) {
	fake := &fakeSelfValidateProvider{}
	prev := globalRealProvider
	globalRealProvider = fake
	t.Cleanup(func() { globalRealProvider = prev })

	v := NewSQLSelfValidator(nil)
	if v.validateProvider() == nil {
		t.Fatal("validateProvider() = nil, want fallback to globalRealProvider")
	}

	res, err := v.ValidateAndCorrect(context.Background(), "SELECT 1", "问题", "schema")
	if err != nil {
		t.Fatalf("ValidateAndCorrect error: %v", err)
	}
	if !fake.chatCalled {
		t.Error("LLM Chat was not called: validator stayed on the nil-provider degradation path")
	}
	if !strings.Contains(res.CorrectedSQL, "SELECT 1") {
		t.Errorf("CorrectedSQL = %q, want LLM-validated result", res.CorrectedSQL)
	}
}

// TestSelfValidatorNoProviderAtAll 钉住降级路径：
// 构造时与调用时都没有 provider，ValidateAndCorrect 必须走
// basicValidation（不 panic、IsValid 按语法判定）。
func TestSelfValidatorNoProviderAtAll(t *testing.T) {
	prev := globalRealProvider
	globalRealProvider = nil
	t.Cleanup(func() { globalRealProvider = prev })

	v := NewSQLSelfValidator(nil)
	if v.validateProvider() != nil {
		t.Fatal("validateProvider() != nil, want nil")
	}

	res, err := v.ValidateAndCorrect(context.Background(), "SELECT 1", "问题", "schema")
	if err != nil {
		t.Fatalf("ValidateAndCorrect error: %v", err)
	}
	if !res.IsValid {
		t.Error("SELECT 1 should pass basic validation")
	}
}
