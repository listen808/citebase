package rag

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

// 这批测试原先全部拿时钟做判据（time.Sleep(100ms)、50ms 超时、"并行应该 <200ms"），
// 于是同一份代码在负载高的机器上会忽过忽不过 —— 那类 flaky 最后只能靠重跑或调大 timeout
// 掩盖，而真正要钉的性质（并行度上限、取消必须传播）反而一次都没被证过。
// 现在改成由测试自己控制的同步点：闸门、信号通道、预先取消的上下文。

func TestParallelRetriever_Retrieve(t *testing.T) {
	retriever := NewParallelRetriever(3)
	queries := []string{"q1", "q22", "q333"}

	// 这条只钉"返回值按查询原位放回、每个查询都算到"。
	// 并行度是另一条性质，由 TestParallelRetriever_MaxConcurrency 用闸门去证 ——
	// 原先这里用 elapsed < 200ms 断言"并行了"，那是把调度当判据，慢机器上必假。
	results, err := retriever.Retrieve(context.Background(), queries, func(ctx context.Context, query string) ([]RetrievalResult, error) {
		return []RetrievalResult{{DocumentID: len(query), Score: 0.9}}, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != len(queries) {
		t.Fatalf("expected %d result rows, got %d", len(queries), len(results))
	}
	for i, r := range results {
		if len(r) != 1 || r[0].DocumentID != len(queries[i]) {
			t.Errorf("results[%d] 与 queries[%d]=%q 错位：%+v", i, i, queries[i], r)
		}
	}
}

// TestParallelRetriever_MaxConcurrency 用闸门而不是 sleep 卡住并发。
// 每个 fn 报到后等放行；放行条件是"观察到了 maxConcurrent 个同时在跑"。
// 实现若超发（同时进来第 maxConcurrent+1 个），peak 就会被记大，断言变红；
// 实现若欠发，闸门由最后一个到齐的人打开，也不会死锁。
func TestParallelRetriever_MaxConcurrency(t *testing.T) {
	const maxConcurrent = 2
	retriever := NewParallelRetriever(maxConcurrent)

	var active int32
	var peak int32
	var gateOnce sync.Once
	gate := make(chan struct{})

	retrieveFunc := func(ctx context.Context, query string) ([]RetrievalResult, error) {
		cur := atomic.AddInt32(&active, 1)
		for {
			old := atomic.LoadInt32(&peak)
			if cur <= old || atomic.CompareAndSwapInt32(&peak, old, cur) {
				break
			}
		}
		if cur >= int32(maxConcurrent) {
			gateOnce.Do(func() { close(gate) })
		}
		select {
		case <-gate:
		case <-ctx.Done():
			atomic.AddInt32(&active, -1)
			return nil, ctx.Err()
		}
		atomic.AddInt32(&active, -1)
		return nil, nil
	}

	queries := make([]string, 10)
	for i := range queries {
		queries[i] = "query"
	}

	if _, err := retriever.Retrieve(context.Background(), queries, retrieveFunc); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&peak); got > maxConcurrent {
		t.Errorf("并发上限被突破：peak=%d，want <= %d", got, maxConcurrent)
	}
}

func TestParallelRetriever_EmptyQueries(t *testing.T) {
	retriever := NewParallelRetriever(3)

	retrieveFunc := func(ctx context.Context, query string) ([]RetrievalResult, error) {
		t.Errorf("空查询不该调用检索函数（被调了 %q）", query)
		return nil, nil
	}

	results, err := retriever.Retrieve(context.Background(), []string{}, retrieveFunc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for empty queries, got %v", results)
	}
}

// TestParallelRetriever_ContextCancellation 用"调用前就已取消"的上下文。
// 原先是 time.After(100ms) 对 50ms 超时 —— 两个计时器赛跑，机器一抖结论就翻。
// Retrieve 在取到信号量之后、调 fn 之前检查 ctx，所以预取消是确定性的。
func TestParallelRetriever_ContextCancellation(t *testing.T) {
	retriever := NewParallelRetriever(3)

	var called int32
	retrieveFunc := func(ctx context.Context, query string) ([]RetrievalResult, error) {
		atomic.AddInt32(&called, 1)
		return []RetrievalResult{{DocumentID: 1}}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 先取消，再发起：不依赖"超时刚好落在检索中间"

	_, err := retriever.Retrieve(ctx, []string{"q1", "q2", "q3"}, retrieveFunc)
	if err == nil {
		t.Fatal("已取消的上下文必须让 Retrieve 返回错误，而不是静返回成功")
	}
	if err != context.Canceled {
		t.Errorf("错误应为 context.Canceled，实得 %v", err)
	}
	if atomic.LoadInt32(&called) != 0 {
		t.Errorf("上下文已取消却仍调用了检索函数 %d 次", called)
	}
}
