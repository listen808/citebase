package delivery

import (
	"context"
	"sync"
)

// MemSubscriber 基于内存的订阅源，适合默认接入、单实例与测试。
// 生产环境应替换为基于 DB/配置的订阅仓储（实现 Subscriber 接口）。
type MemSubscriber struct {
	mu  sync.RWMutex
	sub map[string][]Channel // key: userID:event -> channels
}

// NewMemSubscriber 创建空的内存订阅源。
func NewMemSubscriber() *MemSubscriber {
	return &MemSubscriber{sub: make(map[string][]Channel)}
}

func memKey(userID int, event EventType) string {
	return fmtInt(userID) + ":" + string(event)
}

// Set 设置某用户在某事件下的订阅渠道（覆盖式）。
func (m *MemSubscriber) Set(userID int, event EventType, channels ...Channel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sub[memKey(userID, event)] = channels
}

// Subscribe 追加（不去重）订阅渠道。
func (m *MemSubscriber) Subscribe(userID int, event EventType, channels ...Channel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := memKey(userID, event)
	m.sub[key] = append(m.sub[key], channels...)
}

// Unsubscribe 取消某用户在某事件下的全部渠道订阅。
func (m *MemSubscriber) Unsubscribe(userID int, event EventType) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sub, memKey(userID, event))
}

// Subscriptions 实现 Subscriber 接口。
func (m *MemSubscriber) Subscriptions(_ context.Context, userID int, event EventType) ([]Channel, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	chs, ok := m.sub[memKey(userID, event)]
	if !ok {
		return nil, nil
	}
	out := make([]Channel, len(chs))
	copy(out, chs)
	return out, nil
}
