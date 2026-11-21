package rag

import (
	"testing"
)

func TestCacheIndex_AddAndSearch(t *testing.T) {
	index := NewCacheIndex(128)

	// 添加条目
	index.Add("query1", []float32{0.1, 0.2, 0.3})
	index.Add("query2", []float32{0.4, 0.5, 0.6})
	index.Add("query3", []float32{0.7, 0.8, 0.9})

	// 搜索相似向量
	query := []float32{0.1, 0.2, 0.3}
	results := index.Search(query, 0.9, 5)

	if len(results) == 0 {
		t.Error("expected at least 1 result")
	}

	if results[0].Key != "query1" {
		t.Errorf("expected first result to be 'query1', got %s", results[0].Key)
	}
}

func TestCacheIndex_Remove(t *testing.T) {
	index := NewCacheIndex(128)

	index.Add("query1", []float32{0.1, 0.2, 0.3})
	index.Remove("query1")

	results := index.Search([]float32{0.1, 0.2, 0.3}, 0.9, 5)
	if len(results) != 0 {
		t.Error("expected 0 results after removal")
	}
}

func TestCacheIndex_LRUEviction(t *testing.T) {
	index := NewCacheIndex(2) // 最大容量2

	index.Add("query1", []float32{0.1, 0.2, 0.3})
	index.Add("query2", []float32{0.4, 0.5, 0.6})
	index.Add("query3", []float32{0.7, 0.8, 0.9}) // 应该淘汰query1

	if index.Size() != 2 {
		t.Errorf("expected size 2, got %d", index.Size())
	}

	// query1应该被淘汰
	results := index.Search([]float32{0.1, 0.2, 0.3}, 0.9, 5)
	for _, r := range results {
		if r.Key == "query1" {
			t.Error("query1 should have been evicted")
		}
	}
}

func TestCacheIndex_Update(t *testing.T) {
	index := NewCacheIndex(128)

	// 添加条目
	index.Add("query1", []float32{0.1, 0.2, 0.3})

	// 更新条目（使用差异大的向量）
	index.Add("query1", []float32{0.9, 0.8, 0.7})

	if index.Size() != 1 {
		t.Errorf("expected size 1 after update, got %d", index.Size())
	}

	// 搜索旧向量应该不匹配（相似度低）
	results := index.Search([]float32{0.1, 0.2, 0.3}, 0.9, 5)
	for _, r := range results {
		if r.Key == "query1" {
			t.Error("query1 should have been updated with new vector")
		}
	}

	// 搜索新向量应该匹配
	results = index.Search([]float32{0.9, 0.8, 0.7}, 0.9, 5)
	if len(results) == 0 {
		t.Error("expected match with updated vector")
	}
}

func TestCacheIndex_EmptyIndex(t *testing.T) {
	index := NewCacheIndex(128)

	results := index.Search([]float32{0.1, 0.2, 0.3}, 0.9, 5)
	if len(results) != 0 {
		t.Error("expected 0 results from empty index")
	}
}

func TestCacheIndex_LimitResults(t *testing.T) {
	index := NewCacheIndex(128)

	// 添加多个相似条目
	for i := 0; i < 10; i++ {
		vector := []float32{float32(i) * 0.1, float32(i) * 0.1, float32(i) * 0.1}
		index.Add("query"+string(rune('0'+i)), vector)
	}

	// 搜索并限制结果数量
	query := []float32{0.5, 0.5, 0.5}
	results := index.Search(query, 0.0, 3) // threshold=0.0 表示返回所有

	if len(results) > 3 {
		t.Errorf("expected at most 3 results, got %d", len(results))
	}
}
