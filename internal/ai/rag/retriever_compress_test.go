package rag

import (
	"context"
	"testing"

	"github.com/listen808/citebase/internal/ctxkeys"
)

func TestRecallMemoryContextNoManager(t *testing.T) {
	// memoryManager 未配置时，应安全降级返回空串，不 panic。
	r := NewRetriever(nil, nil)
	if r.compression == nil {
		t.Fatal("compression layer should be initialized in NewRetriever")
	}
	ctx := context.Background()
	if got := r.RecallMemoryContext(ctx, "问题"); got != "" {
		t.Fatalf("expected empty memory context, got %q", got)
	}
}

func TestRecallMemoryContextNoUserID(t *testing.T) {
	// 没有 user_id 的 ctx 也应返回空串。
	r := NewRetriever(nil, nil)
	ctx := ctxkeys.WithUserID(context.Background(), 0)
	if got := r.RecallMemoryContext(ctx, "问题"); got != "" {
		t.Fatalf("expected empty when user_id=0, got %q", got)
	}
}

func TestRetrieveContextWithRAGNoPanic(t *testing.T) {
	// 无 provider/embedder 时 RetrieveContextWithRAG 不应 panic。
	r := NewRetriever(nil, nil)
	func() {
		defer func() {
			if recover() != nil {
				t.Fatal("RetrieveContextWithRAG panicked without backend")
			}
		}()
		r.RetrieveContextWithRAG(context.Background(), "测试问题", 1, 5)
	}()
}

func TestRetrieverCompressionInitialized(t *testing.T) {
	r := NewRetrieverWithFastProvider(nil, nil, nil)
	if r.compression == nil {
		t.Fatal("compression layer must be set on retriever")
	}
	if r.compression.maxTokens <= 0 {
		t.Fatal("compression maxTokens should be positive")
	}
}
