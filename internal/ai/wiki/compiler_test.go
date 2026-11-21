package wiki

import (
	"testing"

	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"税率政策":        "税率政策",
		"Tax Policy":  "tax-policy",
		"RAG / 向量检索":  "rag-向量检索",
		"  a__b!!c  ": "a-b-c",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

// 三级去重的第一级（精确）+ 第二级（模糊）必须拦住语义等价的重复建页，
// 否则并发编译会把同一知识拆成多页（AWS 实测 38% 语义重复）。
func TestMatchExisting(t *testing.T) {
	pages := []*knowledgemodel.WikiPage{
		{Slug: "tax", Title: "税率政策"},
		{Slug: "rag", Title: "检索增强生成"},
	}

	if got := matchExisting(pages, "tax", "tax"); got == nil || got.Slug != "tax" {
		t.Fatalf("exact slug match failed: %v", got)
	}
	if got := matchExisting(pages, "tax-policy", "税率政策"); got == nil || got.Slug != "tax" {
		t.Fatalf("exact title match failed: %v", got)
	}
	if got := matchExisting(pages, "tax-policy", "税率政策实施细则"); got == nil || got.Slug != "tax" {
		t.Fatalf("contains match failed: %v", got)
	}
	if got := matchExisting(pages, "brand-new", "完全无关的新概念"); got != nil {
		t.Fatalf("unrelated concept should not match, got %v", got)
	}
}

func TestContentHashStable(t *testing.T) {
	a := ContentHash("同一份源内容")
	b := ContentHash("同一份源内容")
	c := ContentHash("内容已变更")
	if a != b {
		t.Fatal("hash must be stable for identical content")
	}
	if a == c {
		t.Fatal("hash must change when content changes")
	}
	if ContentHash("") != "" {
		t.Fatal("empty content should yield empty hash")
	}
}
