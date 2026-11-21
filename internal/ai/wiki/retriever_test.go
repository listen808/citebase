package wiki

import (
	"testing"

	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
)

// 摘要盲区回归：答案只在正文深处、目录摘要里没有该词，
// 纯目录导航必然漏掉这一页，全文旁路必须把它捞出来。
func TestSearchFulltextCoversSummaryBlindSpot(t *testing.T) {
	pages := []*knowledgemodel.WikiPage{
		{Slug: "logistics-plan", Title: "物流计划", Summary: "介绍该计划的资格要求与操作流程。",
			Content: "## 费率\n标准费率为每单 3.5 元，偏远地区加收 2 元。"},
		{Slug: "tax", Title: "税率政策", Summary: "税率的适用范围与合规要求。", Content: "免税门槛按单票货件金额计算。"},
	}

	hits := searchFulltextForTest(pages, "物流计划的费率是多少", 2)
	if len(hits) == 0 {
		t.Fatal("fulltext path returned nothing")
	}
	if hits[0].Slug != "logistics-plan" {
		t.Fatalf("expected logistics-plan first, got %s", hits[0].Slug)
	}
}

func TestTokenizeCJKAndLatin(t *testing.T) {
	terms := tokenize("检索增强 RAG")
	if len(terms) == 0 {
		t.Fatal("tokenize returned nothing")
	}
	joined := stringsJoin(terms)
	if !contains(joined, "检索") || !contains(joined, "rag") {
		t.Fatalf("unexpected tokens: %v", terms)
	}
}

func TestParseSlugArray(t *testing.T) {
	if got := parseSlugArray("```json\n[\"a\", \"b\"]\n```"); len(got) != 2 || got[0] != "a" {
		t.Fatalf("json array parse failed: %v", got)
	}
	if got := parseSlugArray("我建议查看 \"tax\" 和 \"rag\""); len(got) != 2 {
		t.Fatalf("fallback parse failed: %v", got)
	}
}

func searchFulltextForTest(pages []*knowledgemodel.WikiPage, q string, k int) []Retrieved {
	r := &Retriever{}
	return r.searchFulltext(pages, q, k)
}

func stringsJoin(parts []string) string {
	out := ""
	for _, p := range parts {
		out += p + " "
	}
	return out
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
