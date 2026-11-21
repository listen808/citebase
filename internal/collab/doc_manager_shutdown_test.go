package collab

import (
	"testing"
	"time"
)

// TestDocManagerStopIsIdempotent 钉住重复关停不再 panic。
// 原实现是裸 close(dm.stopCh)：第二次调用就是 close of closed channel，
// 而优雅关停路径（Server.Shutdown 与包级 ShutdownGlobal）本来就都可能撞上同一个单例。
func TestDocManagerStopIsIdempotent(t *testing.T) {
	dm := GetDocManager()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("DocManager.Stop() 重复调用 panic: %v", r)
		}
	}()

	dm.Stop()
	dm.Stop()
}

// TestDocManagerStopWaitsForFlush 钉住"Stop 返回时刷盘已经结束了"。
//
// 原实现 close 完立刻返回，调用方以为自己保存完了，实际 persistLoop 还在另一个
// 协程里跑 —— 协作编辑的最后几秒内容就这么丢在进程退出路上。
// 这条测试用时长做判据：没有任何待刷内容时，Stop 应当在毫秒级返回（等到了
// persistedCh 关闭），而不是靠 select 的超时分支走完 10 秒。
func TestDocManagerStopWaitsForFlush(t *testing.T) {
	dm := GetDocManager()
	// 先排一条脏标记，让刷盘分支真的被走到（pending 非空）。
	// 队列容量 100，这里不消费也不会阻塞。
	select {
	case dm.persistCh <- 999999:
	default:
	}

	start := time.Now()
	dm.Stop()
	elapsed := time.Since(start)

	if elapsed >= 5*time.Second {
		t.Errorf("Stop 用了 %v —— 说明它没等到落盘循环自己关闭信号，而是走到了超时兜底", elapsed)
	}

	select {
	case <-dm.persistedCh:
	default:
		t.Error("Stop 返回时 persistedCh 还没关闭：调用方无法据此认为数据已落盘")
	}
}
