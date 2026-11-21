package rag

import (
	"context"
	"strings"
	"testing"
)

// 改造前 deep_research 的三个"检索源"是纯 LLM 生成，报告没有任何真实来源支撑。
// 这里锁死新行为：拿不到检索源时必须显式声明"无来源"，绝不能让下游 LLM 编造。
func TestExecuteRetrievalNodeWithoutRetriever(t *testing.T) {
	we := &WorkflowEngine{}
	scope := &execScope{results: map[string]interface{}{"question": "物流费率"}}
	node := &WorkflowNode{ID: "s", Type: NodeTypeRetrieval, Config: map[string]interface{}{
		"query": "{{question}}",
	}}

	out, err := we.executeRetrievalNode(context.Background(), node, scope)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, _ := out.(string)
	if !strings.Contains(s, "无检索结果") {
		t.Fatalf("expected explicit no-source marker, got %q", s)
	}
}

func TestExecuteRetrievalNodeEmptyAndHit(t *testing.T) {
	we := &WorkflowEngine{}

	empty := &execScope{results: map[string]interface{}{"question": "q"},
		retriever: func(ctx context.Context, query string, topK int) ([]RetrievedSnippet, error) {
			return nil, nil
		}}
	out, _ := we.executeRetrievalNode(context.Background(),
		&WorkflowNode{ID: "s", Type: NodeTypeRetrieval, Config: map[string]interface{}{"query": "{{question}}"}}, empty)
	if s, _ := out.(string); !strings.Contains(s, "无检索结果") {
		t.Fatalf("empty result must be explicit, got %q", s)
	}

	hit := &execScope{results: map[string]interface{}{"question": "q"},
		retriever: func(ctx context.Context, query string, topK int) ([]RetrievedSnippet, error) {
			return []RetrievedSnippet{{DocumentID: 42, Title: "物流计划", Content: "费率 3.5 元"}}, nil
		}}
	out, _ = we.executeRetrievalNode(context.Background(),
		&WorkflowNode{ID: "s", Type: NodeTypeRetrieval, Config: map[string]interface{}{"query": "{{question}}"}}, hit)
	s, _ := out.(string)
	if !strings.Contains(s, "doc_id=42") || !strings.Contains(s, "物流计划") {
		t.Fatalf("expected retrieved snippet in output, got %q", s)
	}
}
