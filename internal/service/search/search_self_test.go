package search

import (
	"os"
	"strconv"
	"testing"

	"github.com/blevesearch/bleve/v2"
	modelsearch "github.com/listen808/citebase/internal/model/search"
)

// TestSelfContainedIndexSearch 在独立临时目录用与生产完全相同的代码路径建索引并查询，
// 以区分：(A) 索引代码本身有 bug（连临时索引都搜不到）还是 (B) 运行时磁盘索引被破坏/写入丢失。
func TestSelfContainedIndexSearch(t *testing.T) {
	dir, err := os.MkdirTemp("", "citebase-self-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	mapping := buildIndexMapping()
	idx, err := bleve.New(dir, mapping)
	if err != nil {
		t.Fatalf("bleve.New failed: %v", err)
	}

	docs := []modelsearch.SearchDocument{
		{ID: 1, SpaceID: 1, Name: "sample-space-doc", Content: "示例文档：网关切换后的统计口径说明", NamePinyin: PinyinIndexText("sample-space-doc"), Type: 1, Path: "1,2"},
		{ID: 2, SpaceID: 1, Name: "网关监控", Content: "统计网关各个时间段范围的访问情况", NamePinyin: PinyinIndexText("网关监控"), Type: 1, Path: "1,3"},
		{ID: 3, SpaceID: 1, Name: "工作文档介绍", Content: "这是一篇普通文档", NamePinyin: PinyinIndexText("工作文档介绍"), Type: 1, Path: "1,4"},
	}
	for _, d := range docs {
		if err := idx.Index(strconv.Itoa(d.ID), d); err != nil {
			t.Fatalf("index doc %d failed: %v", d.ID, err)
		}
	}
	if err := idx.Close(); err != nil {
		t.Logf("close warn: %v", err)
	}
	// 重新打开以模拟运行时场景
	idx, err = bleve.Open(dir)
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	defer idx.Close()

	count, _ := idx.DocCount()
	t.Logf("doc count: %d", count)

	// 用生产代码路径测试（把全局 index 指向临时索引）
	orig := index
	index = idx
	defer func() { index = orig }()

	runQuery := func(q, ttype string) {
		res, err := SearchDocumentsWithOptions(&SearchOptions{Query: q, Type: ttype, Limit: 10})
		if err != nil {
			t.Logf("[%s/%s] error: %v", ttype, q, err)
			return
		}
		t.Logf("[%s/%s] total=%d", ttype, q, res.Total)
	}

	runQuery("示例", SearchTypeAll)
	runQuery("示例", SearchTypeTitle)
	runQuery("网关", SearchTypeContent)
	runQuery("nginx", SearchTypeAll)
	runQuery("网关监控", SearchTypeAll)
	runQuery("fu wu", SearchTypeAll)
}

// TestSpaceFilterEqualHit 回归测试：空间过滤（单空间 SpaceID / 多空间 SpaceIDs）必须命中。
// 曾用 bleve.NewNumericRangeQuery(&v,&v)，其 max 默认 exclusive，[v,v) 空区间恒 0 命中，
// 导致目录树搜索（带 space_id）与权限过滤（注入 SpaceIDs）静默返回空。
func TestSpaceFilterEqualHit(t *testing.T) {
	dir, err := os.MkdirTemp("", "citebase-space-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	mapping := buildIndexMapping()
	idx, err := bleve.New(dir, mapping)
	if err != nil {
		t.Fatalf("bleve.New failed: %v", err)
	}
	defer idx.Close()

	docs := []modelsearch.SearchDocument{
		{ID: 1, SpaceID: 1, Name: "sample-space-doc", Content: "示例文档：网关切换后的统计口径说明", NamePinyin: PinyinIndexText("sample-space-doc"), Type: 1, Path: "1,2"},
		{ID: 2, SpaceID: 4, Name: "另一空间文档", Content: "网关监控正文内容", NamePinyin: PinyinIndexText("另一空间文档"), Type: 1, Path: "0,4"},
	}
	for _, d := range docs {
		if err := idx.Index(strconv.Itoa(d.ID), d); err != nil {
			t.Fatalf("index doc %d failed: %v", d.ID, err)
		}
	}

	orig := index
	index = idx
	defer func() { index = orig }()

	// 单空间过滤：space_id=1 仅命中 doc1
	res, err := SearchDocumentsWithOptions(&SearchOptions{Query: "示例", Type: SearchTypeAll, SpaceID: 1, Limit: 10})
	if err != nil {
		t.Fatalf("space filter search failed: %v", err)
	}
	if res == nil || res.Total != 1 {
		t.Fatalf("SpaceID=1 filter: expected 1 hit, got %v", func() int {
			if res == nil {
				return -1
			}
			return int(res.Total)
		}())
	}
	t.Logf("SpaceID=1 filter hits=%d", res.Total)

	// 多空间过滤：SpaceIDs=[1,4] 命中两条
	res2, err := SearchDocumentsWithOptions(&SearchOptions{Query: "", Type: SearchTypeAll, SpaceIDs: []int{1, 4}, Limit: 10})
	if err != nil {
		t.Fatalf("multi-space search failed: %v", err)
	}
	if res2 == nil || res2.Total != 2 {
		t.Fatalf("SpaceIDs=[1,4]: expected 2 hits, got %v", func() int {
			if res2 == nil {
				return -1
			}
			return int(res2.Total)
		}())
	}
	t.Logf("SpaceIDs=[1,4] hits=%d", res2.Total)
}
