// Package delivery 实现 AI 结果的"主动投递"能力。
//
// 背景：AI 问答、洞察、定时摘要等结果是被动的——用户必须主动发起对话才能看到。
// 主动投递（proactive delivery）把这些结果按用户订阅的渠道（站内通知 / 邮件 / Webhook）
// 主动推送给目标用户，是 AI 能力真正落地到工作流的关键一环。
//
// 设计要点：
//   - Deliverer 是统一的投递器接口，每个渠道（Notification / Email / Webhook）实现它。
//   - Payload 描述一次投递内容（标题、正文、接收用户、关联事件）。
//   - Deliver 按订阅（Subscription）把 Payload 路由到对应渠道并投递。
//   - 各渠道通过最小接口解耦（如 EmailSender），便于测试与替换。
package delivery

import (
	"context"
	"fmt"
	"sync"
)

// Channel 投递渠道类型。
type Channel string

const (
	ChannelNotification Channel = "notification" // 站内通知
	ChannelEmail        Channel = "email"        // 邮件
	ChannelWebhook      Channel = "webhook"      // Webhook (HTTP POST)
)

// EventType 触发主动投递的事件类型。
type EventType string

const (
	EventAIAnswer     EventType = "ai.answer"     // AI 直接回答完成
	EventAISummary    EventType = "ai.summary"    // 定时/主动摘要
	EventAIInsight    EventType = "ai.insight"    // AI 发现的新洞察
	EventDocMention   EventType = "doc.mention"   // 文档中被 AI 关联提及
	EventTaskComplete EventType = "task.complete" // 异步 AI 任务完成
)

// Payload 一次主动投递的内容。
type Payload struct {
	// Event 触发事件类型，用于匹配订阅。
	Event EventType `json:"event"`
	// Title 投递标题。
	Title string `json:"title"`
	// Body 投递正文（纯文本或 HTML，依渠道而定）。
	Body string `json:"body"`
	// UserID 目标用户 ID（站内通知/邮件按用户解析地址）。
	UserID int `json:"user_id"`
	// RelatedID 关联业务 ID（文档、对话等），可选。
	RelatedID int `json:"related_id,omitempty"`
	// Meta 透传的附加信息（如 Webhook 自定义字段）。
	Meta map[string]interface{} `json:"meta,omitempty"`
}

// Result 单次投递结果。
type Result struct {
	Channel Channel `json:"channel"`
	OK      bool    `json:"ok"`
	Err     string  `json:"err,omitempty"`
}

// Deliverer 单一渠道的投递器。
type Deliverer interface {
	// Channel 返回该投递器对应的渠道。
	Channel() Channel
	// Deliver 将 payload 投递到本渠道。ctx 用于超时/取消控制。
	Deliver(ctx context.Context, p Payload) error
}

// Subscription 描述某用户在某事件上的投递订阅。
type Subscription struct {
	UserID int       `json:"user_id"`
	Event  EventType `json:"event"`
	// Channels 该订阅启用的渠道（可为多个）。
	Channels []Channel `json:"channels"`
}

// Subscriber 订阅源，返回某用户在某事件下的订阅（用于运行时查询）。
// 实现可为内存、DB 仓储或远程配置。
type Subscriber interface {
	// Subscriptions 返回 userID 对 event 生效的渠道列表；无订阅返回 nil。
	Subscriptions(ctx context.Context, userID int, event EventType) ([]Channel, error)
}

// Service 主动投递服务，聚合多个 Deliverer 并按订阅路由。
type Service struct {
	mu         sync.RWMutex
	deliverers map[Channel]Deliverer
	subscriber Subscriber
}

// NewService 创建投递服务。subscriber 为 nil 时退化为"仅站内通知"的默认订阅。
func NewService(subscriber Subscriber) *Service {
	return &Service{
		deliverers: make(map[Channel]Deliverer),
		subscriber: subscriber,
	}
}

// Register 注册一个渠道投递器（重复注册后者覆盖前者）。
func (s *Service) Register(d Deliverer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deliverers[d.Channel()] = d
}

// Deliver 按订阅把 payload 路由到对应渠道并并发投递，返回各渠道结果。
func (s *Service) Deliver(ctx context.Context, p Payload) []Result {
	channels := s.resolveChannels(ctx, p)
	if len(channels) == 0 {
		// 无显式订阅时，退化为站内通知（保证用户至少能看到）。
		channels = []Channel{ChannelNotification}
	}

	s.mu.RLock()
	ds := make([]Deliverer, 0, len(channels))
	for _, ch := range channels {
		if d, ok := s.deliverers[ch]; ok {
			ds = append(ds, d)
		}
	}
	s.mu.RUnlock()

	results := make([]Result, len(ds))
	var wg sync.WaitGroup
	for i, d := range ds {
		wg.Add(1)
		go func(i int, d Deliverer) {
			defer wg.Done()
			r := Result{Channel: d.Channel()}
			if err := d.Deliver(ctx, p); err != nil {
				r.OK = false
				r.Err = err.Error()
			} else {
				r.OK = true
			}
			results[i] = r
		}(i, d)
	}
	wg.Wait()
	return results
}

// resolveChannels 根据订阅源决定本次投递应走哪些渠道。
func (s *Service) resolveChannels(ctx context.Context, p Payload) []Channel {
	if s.subscriber == nil {
		return nil
	}
	chs, err := s.subscriber.Subscriptions(ctx, p.UserID, p.Event)
	if err != nil || len(chs) == 0 {
		return nil
	}
	// 去重
	seen := make(map[Channel]struct{}, len(chs))
	out := chs[:0]
	for _, c := range chs {
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	return out
}

// DefaultSubscriber 内置的默认订阅源：所有事件仅站内通知。
// 用于未接入订阅仓储时的安全退化。
type DefaultSubscriber struct{}

// Subscriptions 始终返回仅站内通知。
func (DefaultSubscriber) Subscriptions(_ context.Context, _ int, _ EventType) ([]Channel, error) {
	return []Channel{ChannelNotification}, nil
}

// errWrap 统一错误包装。
func errWrap(ch Channel, err error) error {
	return fmt.Errorf("delivery[%s]: %w", ch, err)
}
