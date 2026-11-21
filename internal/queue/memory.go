package queue

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	logx "github.com/listen808/citebase/internal/log"
)

// idSeq 全局消息 ID 自增序列。
var idSeq int64

// defaultWorkers 默认 worker 数量。
const defaultWorkers = 8

// delivery 内部投递单元。
type delivery struct {
	topic string
	msg   Message
}

// handlerEntry 订阅项。
type handlerEntry struct {
	id      int64
	handler Handler
}

// MemoryQueue 基于内存通道的发布/订阅实现。
type MemoryQueue struct {
	workers int
	ch      chan delivery
	mu      sync.RWMutex
	subs    map[string][]*handlerEntry
	// nextID 用 atomic.Int64 而非裸 int64：32 位平台（GOARCH=386）上
	// 结构体字段的 64 位原子操作要求 8 字节对齐，裸字段可能落在 4 字节边界
	// 上触发 unaligned atomic panic；atomic.Int64 内部保证对齐。
	nextID  atomic.Int64
	running int32 // 0=未启动/已停止，1=运行中
	wg      sync.WaitGroup
	// handlerWg 单独计数派发的 handler 协程。
	// 原先只有 worker 循环进了 wg，而 dispatch 每次投递都另起协程且不记账 ——
	// 于是 Stop() 那句"等待在途消息处理完成"其实只是在等消费循环退出，
	// handler 还在跑，进程就已经往下走关停最后一步了。
	handlerWg sync.WaitGroup
	// handlerCtx 在 Stop 时取消：handler 拿到的不再是 context.Background()，
	// 而是能被告知"该收尾了"的上下文。不取消也不会立刻出问题，但那等于
	// 关停信号对 handler 完全不可见，长任务只能等到进程被收走。
	handlerCtx    context.Context
	handlerCancel context.CancelFunc
}

// NewMemoryQueue 创建内存队列。workers<=0 时使用默认值。
func NewMemoryQueue(workers int) *MemoryQueue {
	if workers <= 0 {
		workers = defaultWorkers
	}
	return &MemoryQueue{
		workers: workers,
		ch:      make(chan delivery, 1024),
		subs:    make(map[string][]*handlerEntry),
	}
}

// Subscribe 注册主题订阅。
func (q *MemoryQueue) Subscribe(topic string, handler Handler) (Subscription, error) {
	if topic == "" {
		return nil, ErrEmptyTopic
	}
	if handler == nil {
		return nil, ErrNilHandler
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	id := q.nextID.Add(1)
	q.subs[topic] = append(q.subs[topic], &handlerEntry{id: id, handler: handler})
	return &memSub{q: q, topic: topic, id: id}, nil
}

// Publish 发布消息。队列未启动或已停止时返回 ErrClosed。
func (q *MemoryQueue) Publish(ctx context.Context, topic string, payload []byte, headers map[string]string) error {
	if topic == "" {
		return ErrEmptyTopic
	}
	if atomic.LoadInt32(&q.running) == 0 {
		return ErrClosed
	}
	msg := Message{
		ID:        generateID(),
		Topic:     topic,
		Payload:   cloneBytes(payload),
		Headers:   cloneHeaders(headers),
		CreatedAt: time.Now(),
	}
	select {
	case q.ch <- delivery{topic: topic, msg: msg}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Start 启动 worker 消费循环（幂等）。
func (q *MemoryQueue) Start() {
	if !atomic.CompareAndSwapInt32(&q.running, 0, 1) {
		return
	}
	q.handlerCtx, q.handlerCancel = context.WithCancel(context.Background())
	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go q.loop()
	}
	logx.Infof("[queue] memory queue started with %d workers", q.workers)
}

// Stop 停止接收新消息并等待在途消息处理完成。
func (q *MemoryQueue) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&q.running, 1, 0) {
		return nil
	}
	// 先取消 handler 的上下文，再关通道：正在跑的 handler 收到取消信号才有机会收尾，
	// 顺序反过来的话最后一批消息的 handler 会拿着永不取消的 ctx 跑到底。
	if q.handlerCancel != nil {
		q.handlerCancel()
	}
	close(q.ch)
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		q.handlerWg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// loop 单个 worker 消费循环。
func (q *MemoryQueue) loop() {
	defer q.wg.Done()
	for d := range q.ch {
		q.dispatch(d)
	}
}

// dispatch 将消息投递给该主题的所有 handler（并发执行，互不阻塞）。
func (q *MemoryQueue) dispatch(d delivery) {
	q.mu.RLock()
	entries := make([]*handlerEntry, len(q.subs[d.topic]))
	copy(entries, q.subs[d.topic])
	q.mu.RUnlock()
	ctx := q.handlerCtx
	if ctx == nil {
		ctx = context.Background()
	}
	for _, e := range entries {
		e := e
		// 每个 handler 协程都记进 handlerWg，Stop() 才真的等得到它们。
		q.handlerWg.Add(1)
		go func() {
			defer q.handlerWg.Done()
			defer func() {
				if r := recover(); r != nil {
					logx.Errorf("[queue] handler panic on topic %q: %v", d.topic, r)
				}
			}()
			if err := e.handler(ctx, d.msg); err != nil {
				// 关停时取消上下文会让 handler 返回 ctx.Canceled，
				// 那是预期的收尾，不该混进错误日志里让人以为队列出故障了。
				if errors.Is(err, context.Canceled) {
					logx.Infof("[queue] handler on topic %q 在关停时被取消（msg %q）", d.topic, d.msg.ID)
					return
				}
				logx.Errorf("[queue] handler error on topic %q msg %q: %v", d.topic, d.msg.ID, err)
			}
		}()
	}
}

// memSub 内存订阅句柄。
type memSub struct {
	q     *MemoryQueue
	topic string
	id    int64
}

// Unsubscribe 取消订阅。
func (s *memSub) Unsubscribe() error {
	s.q.mu.Lock()
	defer s.q.mu.Unlock()
	entries := s.q.subs[s.topic]
	for i, e := range entries {
		if e.id == s.id {
			s.q.subs[s.topic] = append(entries[:i], entries[i+1:]...)
			break
		}
	}
	if len(s.q.subs[s.topic]) == 0 {
		delete(s.q.subs, s.topic)
	}
	return nil
}

// generateID 生成全局唯一消息 ID。
func generateID() string {
	return time.Now().Format("20060102150405.000000000") + "-" +
		strconv.FormatInt(atomic.AddInt64(&idSeq, 1), 36)
}

// DefaultQueue 进程级默认队列实例。
var DefaultQueue Queue = NewMemoryQueue(0)

// Start 启动默认队列（供 server 生命周期调用）。
func Start() { DefaultQueue.Start() }

// Stop 停止默认队列（供 server 生命周期调用）。
func Stop(ctx context.Context) error { return DefaultQueue.Stop(ctx) }

func cloneBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

func cloneHeaders(h map[string]string) map[string]string {
	if h == nil {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		out[k] = v
	}
	return out
}
