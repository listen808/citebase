package rag

import (
	"strings"
	"testing"
)

// W6：自然语言生成工作流。这里锁死纯函数部分 —— JSON 片段抽取与 DAG 结构校验。
// 模型调用本身不在此测（需要 provider），但「模型输出夹带杂文也能抽出合法 JSON」
// 与「坏 DAG 必须被拒」是审批流的安全边界，必须钉住。

func TestExtractJSONSegment(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "bare object",
			in:   `{"a":1}`,
			want: `{"a":1}`,
		},
		{
			name: "wrapped in prose",
			in:   "好的，以下是工作流：\n{\"name\":\"x\",\"nodes\":[]}\n请查收。",
			want: `{"name":"x","nodes":[]}`,
		},
		{
			name: "fenced code block",
			in:   "```json\n{\"a\":1}\n```",
			want: `{"a":1}`,
		},
		{
			name: "braces inside string literals",
			in:   `{"prompt":"use {x} and {y}"}`,
			want: `{"prompt":"use {x} and {y}"}`,
		},
		{
			name: "escaped quote inside string",
			in:   `{"s":"he said \"hi\" {not}"}`,
			want: `{"s":"he said \"hi\" {not}"}`,
		},
		{
			name: "nested objects",
			in:   `{"a":{"b":1},"c":[1,2]}`,
			want: `{"a":{"b":1},"c":[1,2]}`,
		},
		{
			name: "no object at all",
			in:   "I cannot produce JSON right now.",
			want: "",
		},
		{
			name: "unbalanced truncated",
			in:   `{"a": {"b": 1`,
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractJSONSegment(tc.in); got != tc.want {
				t.Fatalf("extractJSONSegment(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseGeneratedWorkflow(t *testing.T) {
	// 合法：含 start/llm/retrieval/end，id/name 齐全。
	content := `{"name":"流程","description":"d","nodes":[
		{"id":"start","type":"start","name":"开始","next_nodes":["r"]},
		{"id":"r","type":"retrieval","name":"检索","config":{"query":"{{question}}","top_k":3},"next_nodes":["l"]},
		{"id":"l","type":"llm","name":"总结","config":{"system":"s","prompt":"{{r}}"},"next_nodes":["end"]},
		{"id":"end","type":"end","name":"结束","next_nodes":[]}]}`
	wf, err := parseGeneratedWorkflow(content, "fallback-desc")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if wf.Active {
		t.Fatal("generated draft must be inactive")
	}
	if wf.Name != "流程" {
		t.Fatalf("name = %q, want 流程", wf.Name)
	}
	if wf.Description != "d" {
		t.Fatalf("description = %q, want d", wf.Description)
	}
	if len(wf.Nodes) != 4 {
		t.Fatalf("nodes = %d, want 4", len(wf.Nodes))
	}
	if wf.ID == "" || !strings.HasPrefix(wf.ID, "wf_gen_") {
		t.Fatalf("bad generated id: %q", wf.ID)
	}
	// 校验必须通过（结构合法）。
	if err := validateGeneratedWorkflow(wf); err != nil {
		t.Fatalf("valid draft failed validation: %v", err)
	}
}

func TestParseGeneratedWorkflowFallbacks(t *testing.T) {
	// name/description 缺失 → 兜底；id/name 缺失 → 补默认。
	content := `{"nodes":[
		{"type":"start","next_nodes":["a"]},
		{"id":"a","type":"llm","config":{"prompt":"hi"},"next_nodes":["end"]},
		{"id":"end","type":"end"}]}`
	wf, err := parseGeneratedWorkflow(content, "兜底描述")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wf.Name != "AI 生成工作流" {
		t.Fatalf("fallback name = %q", wf.Name)
	}
	if wf.Description != "兜底描述" {
		t.Fatalf("fallback description = %q", wf.Description)
	}
	// 第一个节点 id 被补成 node_1，name 同 id。
	if wf.Nodes[0].ID != "node_1" || wf.Nodes[0].Name != "node_1" {
		t.Fatalf("node_1 backfill wrong: id=%q name=%q", wf.Nodes[0].ID, wf.Nodes[0].Name)
	}
	// end 节点 NextNodes 被补成空数组而非 nil。
	if wf.Nodes[2].NextNodes == nil {
		t.Fatal("end.NextNodes should be empty slice, not nil")
	}
}

func TestParseGeneratedWorkflowRejectsNoJSON(t *testing.T) {
	if _, err := parseGeneratedWorkflow("no json here", "d"); err == nil {
		t.Fatal("expected error for non-JSON output")
	}
	if _, err := parseGeneratedWorkflow(`{"name":"x"}`, "d"); err == nil {
		t.Fatal("expected error for missing nodes")
	}
}

func TestValidateGeneratedWorkflow(t *testing.T) {
	valid := func() *WorkflowDefinition {
		return &WorkflowDefinition{
			Nodes: []WorkflowNode{
				{ID: "start", Type: NodeTypeStart, NextNodes: []string{"a"}},
				{ID: "a", Type: NodeTypeLLM, Config: map[string]interface{}{"prompt": "p"}, NextNodes: []string{"end"}},
				{ID: "end", Type: NodeTypeEnd},
			},
		}
	}

	t.Run("accepts linear dag", func(t *testing.T) {
		if err := validateGeneratedWorkflow(valid()); err != nil {
			t.Fatalf("valid dag rejected: %v", err)
		}
	})

	t.Run("rejects duplicate id", func(t *testing.T) {
		wf := valid()
		wf.Nodes[1].ID = "start"
		if err := validateGeneratedWorkflow(wf); err == nil {
			t.Fatal("expected duplicate id error")
		}
	})

	t.Run("rejects unsupported type", func(t *testing.T) {
		wf := valid()
		wf.Nodes[1].Type = NodeTypeParallel
		if err := validateGeneratedWorkflow(wf); err == nil {
			t.Fatal("expected unsupported type error")
		}
	})

	t.Run("rejects llm without prompt", func(t *testing.T) {
		wf := valid()
		wf.Nodes[1].Config = map[string]interface{}{}
		if err := validateGeneratedWorkflow(wf); err == nil {
			t.Fatal("expected llm prompt error")
		}
	})

	t.Run("rejects retrieval without query", func(t *testing.T) {
		wf := valid()
		wf.Nodes[1].Type = NodeTypeRetrieval
		wf.Nodes[1].Config = map[string]interface{}{"top_k": 3}
		if err := validateGeneratedWorkflow(wf); err == nil {
			t.Fatal("expected retrieval query error")
		}
	})

	t.Run("rejects unknown next ref", func(t *testing.T) {
		wf := valid()
		wf.Nodes[1].NextNodes = []string{"ghost"}
		if err := validateGeneratedWorkflow(wf); err == nil {
			t.Fatal("expected unknown next ref error")
		}
	})

	t.Run("rejects two starts", func(t *testing.T) {
		wf := valid()
		wf.Nodes[2].Type = NodeTypeStart
		wf.Nodes[1].NextNodes = []string{"end"}
		if err := validateGeneratedWorkflow(wf); err == nil {
			t.Fatal("expected start count error")
		}
	})

	t.Run("rejects cycle", func(t *testing.T) {
		wf := &WorkflowDefinition{
			Nodes: []WorkflowNode{
				{ID: "start", Type: NodeTypeStart, NextNodes: []string{"a"}},
				{ID: "a", Type: NodeTypeLLM, Config: map[string]interface{}{"prompt": "p"}, NextNodes: []string{"b"}},
				{ID: "b", Type: NodeTypeLLM, Config: map[string]interface{}{"prompt": "p"}, NextNodes: []string{"a"}},
			},
		}
		if err := validateGeneratedWorkflow(wf); err == nil {
			t.Fatal("expected cycle/unreachable error")
		}
	})

	t.Run("rejects unreachable tail", func(t *testing.T) {
		wf := &WorkflowDefinition{
			Nodes: []WorkflowNode{
				{ID: "start", Type: NodeTypeStart, NextNodes: []string{"a"}},
				{ID: "a", Type: NodeTypeLLM, Config: map[string]interface{}{"prompt": "p"}, NextNodes: []string{"end"}},
				{ID: "end", Type: NodeTypeEnd},
				{ID: "orphan", Type: NodeTypeLLM, Config: map[string]interface{}{"prompt": "p"}},
			},
		}
		if err := validateGeneratedWorkflow(wf); err == nil {
			t.Fatal("expected unreachable node error")
		}
	})
}
