package rag

import (
	"context"
	"strings"
	"testing"
)

func TestCompressionNoopWhenSmall(t *testing.T) {
	l := NewContextCompressionLayer(nil, DefaultCompressionConfig())
	res := l.Compress(context.Background(), "什么是知识库？", []string{"知识库是用于存储知识的系统。"}, nil)
	if res.Compressed {
		t.Fatalf("small input should not be compressed, got %+v", res)
	}
	if res.Strategy != ContextStrategyDirect {
		t.Fatalf("expected direct strategy, got %s", res.Strategy)
	}
	if res.Docs == "" {
		t.Fatal("docs should not be empty")
	}
}

func TestCompressionTriggersOnHugeDocs(t *testing.T) {
	l := NewContextCompressionLayer(nil, DefaultCompressionConfig())

	// 构造远超预算的文档（约 50 万字符）
	huge := strings.Repeat("这是一段关于知识库系统的技术文档内容，包含架构设计与使用说明。", 10000)
	docs := []string{huge}

	res := l.Compress(context.Background(), "总结文档", docs, nil)
	if !res.Compressed {
		t.Fatal("huge docs should trigger compression")
	}
	// 压缩后 token 应明显小于原始（原始约 50万/4 = 12.5万 tokens）
	if res.TotalTokens >= l.tk.CountTokens(huge) {
		t.Fatalf("compressed total %d should be < raw %d", res.TotalTokens, l.tk.CountTokens(huge))
	}
	if res.Strategy != ContextStrategyTruncate && res.Strategy != ContextStrategyMapReduce {
		t.Fatalf("expected truncate/map_reduce, got %s", res.Strategy)
	}
	// 压缩后不应超出 maxTokens
	if res.TotalTokens > l.maxTokens {
		t.Fatalf("compressed %d exceeds maxTokens %d", res.TotalTokens, l.maxTokens)
	}
}

func TestCompressionHistoryCompressed(t *testing.T) {
	// 使用很小的预算，强制触发压缩路径，验证历史被压缩。
	cfg := DefaultCompressionConfig()
	cfg.MaxTokens = 2000
	cfg.SystemTokens = 200
	cfg.QuestionPad = 200
	l := NewContextCompressionLayer(nil, cfg)

	// 构造 40 轮历史 + 较长文档，远超 2000 token 预算。
	history := make([]ChatMessage, 0, 40)
	for i := 0; i < 40; i++ {
		history = append(history, ChatMessage{
			Question: "问题" + string(rune('A'+i%26)) + "：请详细解释相关知识点的设计与实现原理",
			Answer:   "回答" + string(rune('A'+i%26)) + "：这是一段较长的技术说明内容，包含架构、流程与多个注意事项要点。",
		})
	}
	docs := []string{strings.Repeat("知识库系统文档内容，包含架构设计与接口说明。", 3000)}

	res := l.Compress(context.Background(), "继续刚才的讨论", docs, history)
	if !res.Compressed {
		t.Fatal("small budget should force compression")
	}
	// provider=nil 时历史压缩降级为保留最近 10 条
	if len(res.History) > 10 {
		t.Fatalf("history should be compressed to <=10, got %d", len(res.History))
	}
}

func TestCompressContextTextConvenience(t *testing.T) {
	cfg := DefaultCompressionConfig()
	cfg.MaxTokens = 2000
	cfg.SystemTokens = 200
	cfg.QuestionPad = 200
	l := NewContextCompressionLayer(nil, cfg)
	huge := strings.Repeat("内容", 80000) // 远超 2000 预算
	out, compressed := l.CompressContextText(context.Background(), "q", huge, nil)
	if !compressed {
		t.Fatal("huge context text should be compressed under small budget")
	}
	if len(out) >= len(huge) {
		t.Fatal("compressed output should be shorter than input")
	}
}

func TestCompressionBudgetStatus(t *testing.T) {
	l := NewContextCompressionLayer(nil, DefaultCompressionConfig())
	res := l.Compress(context.Background(), "问题", []string{"短上下文"}, nil)
	if res.Budget == nil {
		t.Fatal("budget status should be populated")
	}
	if res.Budget.TotalBudget != l.maxTokens {
		t.Fatalf("budget total %d != maxTokens %d", res.Budget.TotalBudget, l.maxTokens)
	}
}
