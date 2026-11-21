package rag

import (
	"context"
	"testing"
	"time"
)

func TestParallelRetriever_PerformanceImprovement(t *testing.T) {
	// 模拟检索函数（100ms延迟）
	slowRetrieve := func(ctx context.Context, query string) ([]RetrievalResult, error) {
		time.Sleep(100 * time.Millisecond)
		return []RetrievalResult{{DocumentID: 1, Score: 0.9}}, nil
	}

	queries := []string{"q1", "q2", "q3", "q4", "q5"}

	// 串行测试
	start := time.Now()
	for _, q := range queries {
		_, _ = slowRetrieve(context.Background(), q)
	}
	serialTime := time.Since(start)

	// 并行测试
	retriever := NewParallelRetriever(3)
	start = time.Now()
	_, _ = retriever.Retrieve(context.Background(), queries, slowRetrieve)
	parallelTime := time.Since(start)

	t.Logf("Serial time: %v", serialTime)
	t.Logf("Parallel time: %v", parallelTime)
	t.Logf("Speedup: %.2fx", float64(serialTime)/float64(parallelTime))

	// 并行应该比串行快至少2倍
	if parallelTime > serialTime/2 {
		t.Errorf("parallel execution not significantly faster: serial=%v, parallel=%v", serialTime, parallelTime)
	}
}

func TestCacheIndex_PerformanceImprovement(t *testing.T) {
	index := NewCacheIndex(1000)

	// 添加1000个条目
	for i := 0; i < 1000; i++ {
		vector := make([]float32, 128)
		for j := range vector {
			vector[j] = float32(i*128+j) / 128000.0
		}
		index.Add("key"+string(rune(i)), vector)
	}

	// 测试搜索性能
	query := make([]float32, 128)
	for j := range query {
		query[j] = float32(j) / 128.0
	}

	start := time.Now()
	results := index.Search(query, 0.9, 10)
	elapsed := time.Since(start)

	t.Logf("Search time for 1000 entries: %v", elapsed)
	t.Logf("Results found: %d", len(results))

	// 搜索应该在10ms内完成
	if elapsed > 10*time.Millisecond {
		t.Errorf("search too slow: %v", elapsed)
	}
}

func BenchmarkParallelRetriever(b *testing.B) {
	retriever := NewParallelRetriever(3)

	retrieveFunc := func(ctx context.Context, query string) ([]RetrievalResult, error) {
		return []RetrievalResult{{DocumentID: 1, Score: 0.9}}, nil
	}

	queries := []string{"q1", "q2", "q3"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = retriever.Retrieve(context.Background(), queries, retrieveFunc)
	}
}

func BenchmarkCacheIndex_Search(b *testing.B) {
	index := NewCacheIndex(1000)

	for i := 0; i < 1000; i++ {
		vector := make([]float32, 128)
		for j := range vector {
			vector[j] = float32(i*128+j) / 128000.0
		}
		index.Add("key"+string(rune(i)), vector)
	}

	query := make([]float32, 128)
	for j := range query {
		query[j] = float32(j) / 128.0
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = index.Search(query, 0.9, 10)
	}
}

func BenchmarkSerialRetriever(b *testing.B) {
	retrieveFunc := func(ctx context.Context, query string) ([]RetrievalResult, error) {
		return []RetrievalResult{{DocumentID: 1, Score: 0.9}}, nil
	}

	queries := []string{"q1", "q2", "q3"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, q := range queries {
			_, _ = retrieveFunc(context.Background(), q)
		}
	}
}
