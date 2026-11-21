package rag

import (
	"strings"
	"testing"
)

func TestExtractComponentsChart(t *testing.T) {
	answer := "这是分析结论。\n\n```chart\n{\"title\":{\"text\":\"销量\"},\"xAxis\":{\"type\":\"category\",\"data\":[\"一月\"]},\"yAxis\":{\"type\":\"value\"},\"series\":[{\"type\":\"bar\",\"data\":[10]}]}\n```"
	comps := ExtractComponents(answer)
	if len(comps) != 1 {
		t.Fatalf("expected 1 component, got %d", len(comps))
	}
	if comps[0].Kind != ComponentChart || comps[0].ChartOption == nil {
		t.Fatalf("expected chart component, got %+v", comps[0])
	}
}

func TestExtractComponentsTable(t *testing.T) {
	answer := "数据如下：\n\n```table\n{\"columns\":[\"名称\",\"值\"],\"rows\":[[\"A\",1],[\"B\",2]]}\n```"
	comps := ExtractComponents(answer)
	if len(comps) != 1 || comps[0].Kind != ComponentTable {
		t.Fatalf("expected table component, got %+v", comps)
	}
	if len(comps[0].TableColumns) != 2 || len(comps[0].TableRows) != 2 {
		t.Fatalf("bad table data: %+v", comps[0])
	}
}

func TestExtractComponentsCard(t *testing.T) {
	answer := "```card\n{\"title\":\"摘要\",\"fields\":{\"状态\":\"正常\",\"负责人\":\"张三\"}}\n```"
	comps := ExtractComponents(answer)
	if len(comps) != 1 || comps[0].Kind != ComponentCard {
		t.Fatalf("expected card, got %+v", comps)
	}
	if comps[0].CardFields["状态"] != "正常" {
		t.Fatalf("bad card fields: %+v", comps[0])
	}
}

func TestExtractComponentsNone(t *testing.T) {
	answer := "纯文本回答，没有任何结构化块。"
	if comps := ExtractComponents(answer); comps != nil {
		t.Fatalf("expected nil, got %+v", comps)
	}
}

func TestStripComponentBlocks(t *testing.T) {
	answer := "前缀文字\n\n```chart\n{}\n```\n后缀文字"
	clean := StripComponentBlocks(answer)
	if strings.Contains(clean, "```") {
		t.Fatalf("fences not stripped: %q", clean)
	}
	if !strings.Contains(clean, "前缀文字") || !strings.Contains(clean, "后缀文字") {
		t.Fatalf("text lost: %q", clean)
	}
}

func TestExtractComponentsMixed(t *testing.T) {
	answer := "结论。\n\n```chart\n{\"xAxis\":{}}\n```\n\n中间说明\n\n```table\n{\"columns\":[\"c\"],\"rows\":[[1]]}\n```"
	comps := ExtractComponents(answer)
	if len(comps) != 2 {
		t.Fatalf("expected 2 components, got %d: %+v", len(comps), comps)
	}
	if comps[0].Kind != ComponentChart || comps[1].Kind != ComponentTable {
		t.Fatalf("order/type wrong: %+v", comps)
	}
}
