package queue

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// generateID 测试可见的轻量 ID 生成（复用包内未导出函数需暴露，这里直接内联简单实现）。
func testID() string {
	return time.Now().Format("20060102150405.000000000")
}

func newStartedQueue() *MemoryQueue {
	q := NewMemoryQueue(4)
	q.Start()
	return q
}

func TestPublishSubscribe(t *testing.T) {
	q := newStartedQueue()
	defer q.Stop(context.Background())

	var got int32
	sub, err := q.Subscribe("topic.a", func(_ context.Context, msg Message) error {
		atomic.StoreInt32(&got, 1)
		if string(msg.Payload) != "hello" {
			t.Errorf("unexpected payload: %q", msg.Payload)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	if err := q.Publish(context.Background(), "topic.a", []byte("hello"), nil); err != nil {
		t.Fatalf("publish: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for atomic.LoadInt32(&got) == 0 {
		select {
		case <-deadline:
			t.Fatal("handler not invoked in time")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestMultipleHandlers(t *testing.T) {
	q := newStartedQueue()
	defer q.Stop(context.Background())

	var count int32
	var wg sync.WaitGroup
	wg.Add(3)
	for i := 0; i < 3; i++ {
		_, err := q.Subscribe("topic.b", func(_ context.Context, _ Message) error {
			atomic.AddInt32(&count, 1)
			wg.Done()
			return nil
		})
		if err != nil {
			t.Fatalf("subscribe: %v", err)
		}
	}

	if err := q.Publish(context.Background(), "topic.b", []byte("x"), nil); err != nil {
		t.Fatalf("publish: %v", err)
	}
	wg.Wait()
	if c := atomic.LoadInt32(&count); c != 3 {
		t.Fatalf("expected 3 handlers, got %d", c)
	}
}

func TestPublishBeforeStartReturnsErr(t *testing.T) {
	q := NewMemoryQueue(2)
	if err := q.Publish(context.Background(), "t", []byte("x"), nil); err != ErrClosed {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	q := newStartedQueue()
	defer q.Stop(context.Background())

	var count int32
	sub, _ := q.Subscribe("topic.c", func(_ context.Context, _ Message) error {
		atomic.AddInt32(&count, 1)
		return nil
	})
	sub.Unsubscribe()

	_ = q.Publish(context.Background(), "topic.c", []byte("x"), nil)
	time.Sleep(200 * time.Millisecond)
	if c := atomic.LoadInt32(&count); c != 0 {
		t.Fatalf("expected 0 after unsubscribe, got %d", c)
	}
}

func TestStartIdempotent(t *testing.T) {
	q := NewMemoryQueue(2)
	q.Start()
	q.Start() // 第二次应为幂等，不会 panic 或重复启动
	q.Stop(context.Background())
	_ = testID()
}

// TestStopWaitsForInFlightHandlers 钉住 Stop() 注释里那句"等待在途消息处理完成"。
// 原实现只等 worker 循环退出：dispatch 为每个 handler 另起协程，而那份协程
// 从来没记进 wg —— 于是 Stop 返回时 handler 还在跑，关停的最后一步
// （关 HTTP、进程退出）就可能把还没做完的后台任务一起带走。
func TestStopWaitsForInFlightHandlers(t *testing.T) {
	q := newStartedQueue()

	var finished int32
	release := make(chan struct{})
	if _, err := q.Subscribe("topic.slow", func(_ context.Context, _ Message) error {
		<-release
		atomic.StoreInt32(&finished, 1)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := q.Publish(context.Background(), "topic.slow", []byte("x"), nil); err != nil {
		t.Fatal(err)
	}
	// 让 worker 真的把消息派发出去（handler 卡在 release 上）。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&finished) == 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		break
	}

	stopDone := make(chan error, 1)
	go func() { stopDone <- q.Stop(context.Background()) }()

	// Stop 必须还没返回 —— handler 还堵着。
	select {
	case err := <-stopDone:
		close(release)
		t.Fatalf("Stop 在 handler 还没结束时就返回了（err=%v）：wg 没管住 dispatch 起的协程", err)
	case <-time.After(200 * time.Millisecond):
	}

	close(release)
	select {
	case err := <-stopDone:
		if err != nil {
			t.Errorf("Stop 返回错误: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("放开 handler 之后 Stop 仍不返回")
	}
	if atomic.LoadInt32(&finished) != 1 {
		t.Error("Stop 返回时 handler 并未完成")
	}
}

// TestHandlersSeeCancelOnStop 钉住 handler 收到的是可取消的上下文。
// 原先每条消息都拿 context.Background()，关停信号对 handler 完全不可见 ——
// 一个会轮询外部服务的 handler 只能等到进程被系统收走。
func TestHandlersSeeCancelOnStop(t *testing.T) {
	q := newStartedQueue()

	ctxDone := make(chan struct{})
	if _, err := q.Subscribe("topic.ctx", func(ctx context.Context, _ Message) error {
		select {
		case <-ctx.Done():
			close(ctxDone)
		case <-time.After(5 * time.Second):
			return nil
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := q.Publish(context.Background(), "topic.ctx", []byte("x"), nil); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)

	if err := q.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	select {
	case <-ctxDone:
	case <-time.After(2 * time.Second):
		t.Error("handler 的 context 在 Stop 时没有被取消")
	}
}
