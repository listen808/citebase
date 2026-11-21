package rag

import (
	"context"
	"sync"
)

// RetrievalResult 检索结果
type RetrievalResult struct {
	DocumentID   int
	DocumentName string
	// SpaceID P1（清单 17）：来源空间随结果透传，否则并行分支重建 Source 时
	// SpaceID=0，DocURL 变成 /space/0/doc/N 且事后无法判定归属空间
	SpaceID int
	Score   float64
	Snippet string
}

// RetrieveFunc 检索函数类型
type RetrieveFunc func(ctx context.Context, query string) ([]RetrievalResult, error)

// ParallelRetriever 并行检索器
type ParallelRetriever struct {
	maxConcurrency int
}

// NewParallelRetriever 创建并行检索器
func NewParallelRetriever(maxConcurrency int) *ParallelRetriever {
	if maxConcurrency <= 0 {
		maxConcurrency = 3
	}
	return &ParallelRetriever{
		maxConcurrency: maxConcurrency,
	}
}

// Retrieve 并行执行多个检索查询
func (p *ParallelRetriever) Retrieve(ctx context.Context, queries []string, fn RetrieveFunc) ([][]RetrievalResult, error) {
	if len(queries) == 0 {
		return nil, nil
	}

	results := make([][]RetrievalResult, len(queries))
	errors := make([]error, len(queries))

	// 使用信号量控制并发
	sem := make(chan struct{}, p.maxConcurrency)
	var wg sync.WaitGroup

	for i, query := range queries {
		wg.Add(1)
		go func(idx int, q string) {
			defer wg.Done()

			// 获取信号量
			sem <- struct{}{}
			defer func() { <-sem }()

			// 检查上下文取消
			select {
			case <-ctx.Done():
				errors[idx] = ctx.Err()
				return
			default:
			}

			result, err := fn(ctx, q)
			results[idx] = result
			errors[idx] = err
		}(i, query)
	}

	wg.Wait()

	// 检查错误
	for _, err := range errors {
		if err != nil {
			return results, err
		}
	}

	return results, nil
}
