package text2sql

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

// ============================================================
// 会话状态登记表（P0 安全修复：去单例化）
//
// 旧实现里 StreamHandler 把「请求级」状态直接写回进程级单例：
//   - h.agent.generator.kb = kb
//   - h.agent.conversationState.RecordTurn(...)
// 于是并发用户的对话轮次互相混写：A 的上一轮 SQL/表结构会进入 B 的提示词（跨用户
// 会话内容泄露），同时没有任何锁保护（数据竞争），SQLHistory 还会无上限增长。
// 这里按 (conversationID, userID) 隔离状态实例，并给出容量上限 + TTL 淘汰。
// ============================================================

const (
	// ConversationRegistryDefaultCapacity 同时存活的会话状态实例上限，超出按最旧淘汰
	ConversationRegistryDefaultCapacity = 1000
	// ConversationRegistryDefaultTTL 空闲超过该时长的会话状态被回收
	ConversationRegistryDefaultTTL = 30 * time.Minute
)

// ConversationRegistry 按 (conversationID, userID) 管理 ConversationState 实例，
// 带互斥锁 + LRU 淘汰 + TTL 过期，避免共享同一个状态对象。
type ConversationRegistry struct {
	capacity int
	ttl      time.Duration

	mu      sync.Mutex
	entries map[string]*list.Element
	order   *list.List // front = 最近使用
}

type conversationEntry struct {
	key      string
	state    *ConversationState
	lastUsed time.Time
}

// NewConversationRegistry 创建登记表；capacity/ttl 传非正值时使用默认值
func NewConversationRegistry(capacity int, ttl time.Duration) *ConversationRegistry {
	if capacity <= 0 {
		capacity = ConversationRegistryDefaultCapacity
	}
	if ttl <= 0 {
		ttl = ConversationRegistryDefaultTTL
	}
	return &ConversationRegistry{
		capacity: capacity,
		ttl:      ttl,
		entries:  make(map[string]*list.Element),
		order:    list.New(),
	}
}

// Get 取（或按需创建）该 (conversationID, userID) 专属的会话状态
func (r *ConversationRegistry) Get(convID, userID, spaceID int) *ConversationState {
	if r == nil {
		return NewConversationState(convID, spaceID)
	}
	key := conversationRegistryKey(convID, userID)
	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	r.evictExpiredLocked(now)

	if el, ok := r.entries[key]; ok {
		entry := el.Value.(*conversationEntry)
		entry.lastUsed = now
		r.order.MoveToFront(el)
		if entry.state.SpaceID != spaceID {
			entry.state.SpaceID = spaceID
		}
		return entry.state
	}

	state := NewConversationState(convID, spaceID)
	r.entries[key] = r.order.PushFront(&conversationEntry{key: key, state: state, lastUsed: now})
	r.evictOverflowLocked()
	return state
}

// Remove 主动丢弃某个会话状态（会话结束/切换用户时可用）
func (r *ConversationRegistry) Remove(convID, userID int) {
	if r == nil {
		return
	}
	key := conversationRegistryKey(convID, userID)
	r.mu.Lock()
	defer r.mu.Unlock()
	if el, ok := r.entries[key]; ok {
		r.order.Remove(el)
		delete(r.entries, key)
	}
}

// Len 当前存活的会话状态数量
func (r *ConversationRegistry) Len() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.entries)
}

// evictExpiredLocked 回收 TTL 过期的实例（调用方持锁）
func (r *ConversationRegistry) evictExpiredLocked(now time.Time) {
	for el := r.order.Back(); el != nil; {
		prev := el.Prev()
		entry := el.Value.(*conversationEntry)
		if now.Sub(entry.lastUsed) <= r.ttl {
			break
		}
		r.order.Remove(el)
		delete(r.entries, entry.key)
		el = prev
	}
}

// evictOverflowLocked 超出容量上限时淘汰最久未使用的实例（调用方持锁）
func (r *ConversationRegistry) evictOverflowLocked() {
	for r.order.Len() > r.capacity {
		el := r.order.Back()
		if el == nil {
			return
		}
		entry := el.Value.(*conversationEntry)
		r.order.Remove(el)
		delete(r.entries, entry.key)
	}
}

func conversationRegistryKey(convID, userID int) string {
	return fmt.Sprintf("%d:%d", convID, userID)
}

// ------------------------------------------------------------
// 全局默认登记表
// ------------------------------------------------------------

var defaultConversationRegistry = NewConversationRegistry(
	ConversationRegistryDefaultCapacity,
	ConversationRegistryDefaultTTL,
)

// ConversationStateFor 返回调用者在某会话中专属的对话状态实例。
// 调用方（StreamHandler 等请求级入口）必须用它，而不是共享 Generator.conversationState。
func ConversationStateFor(convID, userID, spaceID int) *ConversationState {
	return defaultConversationRegistry.Get(convID, userID, spaceID)
}
