package rag

import (
	"strings"
	"testing"
)

func TestVerifyCitationsCited(t *testing.T) {
	sources := []Source{
		{DocumentID: 10, CitationIndex: 1, Snippet: "Citebase 支持全文检索"},
		{DocumentID: 11, CitationIndex: 2, Snippet: "知识库按空间隔离"},
	}
	answer := "Citebase 支持全文检索[1]，且知识库按空间隔离[2]。"
	out := VerifyCitations(answer, sources, nil)
	if !out[0].Cited || out[0].VerifyStatus != VerifyStatusCited {
		t.Fatalf("source1 should be cited, got %+v", out[0])
	}
	if !out[1].Cited || out[1].VerifyStatus != VerifyStatusCited {
		t.Fatalf("source2 should be cited, got %+v", out[1])
	}
	if out[0].DocURL != "/space/0/doc/10" {
		t.Fatalf("unexpected doc url: %s", out[0].DocURL)
	}
}

func TestVerifyCitationsUnverified(t *testing.T) {
	sources := []Source{{DocumentID: 20, CitationIndex: 1, Snippet: "未引用的片段"}}
	answer := "这是一段没有引用标记的答案。"
	out := VerifyCitations(answer, sources, nil)
	if out[0].Cited {
		t.Fatal("should not be cited")
	}
	if out[0].VerifyStatus != VerifyStatusUnverified {
		t.Fatalf("expected unverified, got %s", out[0].VerifyStatus)
	}
}

func TestVerifyCitationsNotFoundWithOriginal(t *testing.T) {
	// 答案未显式引用 [1]，且片段不在原文 -> not_found
	sources := []Source{{DocumentID: 30, CitationIndex: 1, Snippet: "这段在原文中不存在"}}
	answer := "这是一段没有引用标记的答案。"
	original := map[int]string{30: "原文内容是关于 Wiki 的安装与配置步骤。"}
	out := VerifyCitations(answer, sources, original)
	if out[0].VerifyStatus != VerifyStatusNotFound {
		t.Fatalf("expected not_found, got %s", out[0].VerifyStatus)
	}
}

func TestVerifyCitationsFoundWithOriginal(t *testing.T) {
	// 答案未显式引用 [1]，但片段能在原文找到 -> unverified
	sources := []Source{{DocumentID: 31, CitationIndex: 1, Snippet: "Wiki 的安装与配置步骤"}}
	answer := "这是一段没有引用标记的答案。"
	original := map[int]string{31: "本文介绍 Wiki 的安装与配置步骤，包含数据库初始化。"}
	out := VerifyCitations(answer, sources, original)
	if out[0].VerifyStatus != VerifyStatusUnverified {
		t.Fatalf("expected unverified, got %s", out[0].VerifyStatus)
	}
}

func TestSnippetFoundIn(t *testing.T) {
	original := "Citebase 是一个企业知识库，支持全文检索与权限控制。"
	if !snippetFoundIn(original, "Citebase 是一个企业知识库，支持全文检索与权限控制流程") {
		t.Fatal("long snippet with overlap should be found")
	}
	if snippetFoundIn(original, "完全不相关的其他内容xyz") {
		t.Fatal("unrelated snippet must not be found")
	}
}

func TestDocURLFor(t *testing.T) {
	if DocURLFor(5, 100) != "/space/5/doc/100" {
		t.Fatal("bad url")
	}
	if DocURLFor(0, 0) != "" {
		t.Fatal("zero doc id should yield empty url")
	}
}

func TestBuildReasoningChain(t *testing.T) {
	chain := BuildReasoningChain(8, 5, "note", false, true)
	if len(chain) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(chain))
	}
	if chain[0].Stage != "retrieve" || chain[0].Count != 8 {
		t.Fatal("bad retrieve step")
	}
	if chain[1].Stage != "rerank" || chain[1].Count != 5 {
		t.Fatal("bad rerank step")
	}
	if !strings.Contains(chain[3].Detail, "note") {
		t.Fatal("verify detail missing")
	}
}

func TestEnrichAnswerNil(t *testing.T) {
	if EnrichAnswer(nil, nil) != nil {
		t.Fatal("nil should stay nil")
	}
}
