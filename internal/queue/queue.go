// Package queue 提供通用异步消息队列抽象（P1-G034 消息队列基础）。
//
// 当前内置基于内存的发布/订阅实现（MemoryQueue），后续可无缝替换为
// Redis / NATS / Kafka 等外部 broker，只要实现 Queue 接口即可。
// 该基础是 G013 增量更新、G028 Webhook 等异步能力的前置依赖。
package queue

import (
	"context"
	"errors"
	"time"
)

// Message 一条消息。
type Message struct {
	ID        string            // 唯一 ID（由队列生成）
	Topic     string            // 主题
	Payload   []byte            // 负载
	Headers   map[string]string // 元数据
	CreatedAt time.Time         // 创建时间
}

// Handler 消息处理函数；返回非 nil 错误表示处理失败（由队列记录，不阻塞其他订阅者）。
type Handler func(ctx context.Context, msg Message) error

// Subscription 订阅句柄，用于取消订阅。
type Subscription interface {
	// Unsubscribe 取消订阅；之后不会再收到该主题的消息。
	Unsubscribe() error
}

// Queue 消息队列接口。
type Queue interface {
	// Publish 向指定主题发布一条消息。
	Publish(ctx context.Context, topic string, payload []byte, headers map[string]string) error
	// Subscribe 订阅主题，返回订阅句柄。同一主题可注册多个 handler，均会收到消息。
	Subscribe(topic string, handler Handler) (Subscription, error)
	// Start 启动后台 worker，开始消费消息（幂等）。
	Start()
	// Stop 优雅停止：停止接收新消息并等待在途消息处理完成（受 ctx 约束）。
	Stop(ctx context.Context) error
}

var (
	// ErrClosed 队列已停止。
	ErrClosed = errors.New("queue: closed")
	// ErrNilHandler handler 为空。
	ErrNilHandler = errors.New("queue: nil handler")
	// ErrEmptyTopic 主题为空白。
	ErrEmptyTopic = errors.New("queue: empty topic")
)
