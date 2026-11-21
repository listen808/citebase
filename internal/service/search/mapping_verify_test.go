package search

import (
	"os"
	"testing"

	"github.com/blevesearch/bleve/v2"
)

// TestBuildIndexMappingCreatesIndex 验证 buildIndexMapping 产出的 mapping 能成功创建 bleve 索引
// （回归测试：pinyin_ngram 分析器注册失败曾导致 bleve.New 硬失败）
func TestBuildIndexMappingCreatesIndex(t *testing.T) {
	dir := t.TempDir() + "/testindex"
	mapping := buildIndexMapping()
	idx, err := bleve.New(dir, mapping)
	if err != nil {
		t.Fatalf("bleve.New failed: %v", err)
	}
	defer func() {
		idx.Close()
		os.RemoveAll(dir)
	}()

	// 验证拼音字段索引 + 查询（预切分单字母应能匹配首字母）
	doc := ToSearchDocument(1, 1, "微服务架构设计", "content body", 1, "path")
	t.Logf("pinyin index text: %q", doc.NamePinyin)
	if err := idx.Index("1", doc); err != nil {
		t.Fatalf("index document failed: %v", err)
	}

	// "w j" 首字母查询应命中 "微服务架构设计"（w j f w j g...）
	q := bleve.NewMatchQuery("w j")
	q.SetField("name_pinyin")
	res, err := idx.Search(bleve.NewSearchRequest(q))
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if res.Total == 0 {
		t.Errorf("pinyin initial match returned 0 hits, expected >= 1")
	}

	// 全拼查询 "fu wu" 也应命中
	q2 := bleve.NewMatchQuery("fu wu")
	q2.SetField("name_pinyin")
	res2, err := idx.Search(bleve.NewSearchRequest(q2))
	if err != nil {
		t.Fatalf("search full pinyin failed: %v", err)
	}
	if res2.Total == 0 {
		t.Errorf("pinyin full match returned 0 hits, expected >= 1")
	}
}
