package delivery

import (
	"context"
	"strings"
	"testing"
	"time"
)

// fakeNotification 记录调用，用于测试。
type fakeNotification struct {
	calls []Payload
}

func (f *fakeNotification) CreateNotification(userID int, notifType, title, content string, relatedID int) error {
	f.calls = append(f.calls, Payload{UserID: userID, Title: title, Body: content, RelatedID: relatedID})
	return nil
}

// fakeEmail 记录 SendRaw 调用。
type fakeEmail struct {
	sent   []string
	emails map[int][]string
}

func (f *fakeEmail) SendRaw(subject, htmlBody string, toEmails []string) error {
	f.sent = append(f.sent, subject)
	return nil
}
func (f *fakeEmail) ResolveEmails(userID int) []string {
	if f.emails == nil {
		return nil
	}
	return f.emails[userID]
}

// fakeWebhook 返回预设目标。
type fakeWebhook struct {
	targets map[int][]WebhookTarget
}

func (f *fakeWebhook) Targets(userID int) []WebhookTarget {
	if f.targets == nil {
		return nil
	}
	return f.targets[userID]
}

func newTestService() (*Service, *fakeNotification, *fakeEmail, *fakeWebhook) {
	fn := &fakeNotification{}
	fe := &fakeEmail{emails: map[int][]string{1: {"u1@example.com"}}}
	fw := &fakeWebhook{targets: map[int][]WebhookTarget{1: {{URL: "http://hook"}}}}
	svc := NewService(NewMemSubscriber())
	svc.Register(NewNotificationDelivererWith(fn))
	svc.Register(NewEmailDeliverer(fe))
	svc.Register(NewWebhookDeliverer(fw))
	return svc, fn, fe, fw
}

func TestDeliverDefaultFallbackToNotification(t *testing.T) {
	// 无订阅源 -> 退化为站内通知
	svc := NewService(nil)
	fn := &fakeNotification{}
	svc.Register(NewNotificationDelivererWith(fn))
	svc.Register(NewEmailDeliverer(&fakeEmail{}))
	svc.Register(NewWebhookDeliverer(&fakeWebhook{}))

	res := svc.Deliver(context.Background(), Payload{Event: EventAIAnswer, Title: "t", Body: "b", UserID: 99})
	if len(res) != 1 || res[0].Channel != ChannelNotification || !res[0].OK {
		t.Fatalf("expected fallback to notification, got %+v", res)
	}
	if len(fn.calls) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(fn.calls))
	}
}

func TestDeliverRoutesBySubscription(t *testing.T) {
	svc, fn, fe, fw := newTestService()
	sub := svc.subscriber.(*MemSubscriber)
	sub.Set(1, EventAISummary, ChannelEmail, ChannelWebhook) // 仅邮件+webhook

	res := svc.Deliver(context.Background(), Payload{Event: EventAISummary, Title: "Daily", Body: "hi", UserID: 1})
	if len(res) != 2 {
		t.Fatalf("expected 2 channels, got %d: %+v", len(res), res)
	}
	if len(fn.calls) != 0 {
		t.Fatalf("notification should not be called, got %d", len(fn.calls))
	}
	if len(fe.sent) != 1 {
		t.Fatalf("email should be sent once, got %d", len(fe.sent))
	}
	if len(fw.targets[1]) != 1 {
		t.Fatalf("webhook target misconfigured")
	}
}

func TestResolveEmailsEmptySkipsEmail(t *testing.T) {
	svc := NewService(NewMemSubscriber())
	fe := &fakeEmail{} // 无 emails -> ResolveEmails 返回 nil
	svc.Register(NewEmailDeliverer(fe))
	sub := svc.subscriber.(*MemSubscriber)
	sub.Set(2, EventAIAnswer, ChannelEmail)
	res := svc.Deliver(context.Background(), Payload{Event: EventAIAnswer, Title: "x", Body: "y", UserID: 2})
	if len(res) != 1 || res[0].OK {
		t.Fatalf("email with no address should fail, got %+v", res)
	}
}

func TestRenderEmailHTML(t *testing.T) {
	html := renderEmailHTML("Title", "line1\n<tag>")
	if !strings.Contains(html, "Title") {
		t.Fatal("missing title")
	}
	if !strings.Contains(html, "<br/>") {
		t.Fatal("newline not converted")
	}
	if strings.Contains(html, "<tag>") {
		t.Fatal("html not escaped")
	}
}

func TestHMACSign(t *testing.T) {
	s := hmacHex("secret", []byte("payload"))
	if s == "" || len(s) != 64 {
		t.Fatalf("unexpected signature length: %q", s)
	}
	// 不同密钥不同签名
	s2 := hmacHex("secret2", []byte("payload"))
	if s == s2 {
		t.Fatal("different secrets must produce different signatures")
	}
}

func TestMemSubscriberCRUD(t *testing.T) {
	m := NewMemSubscriber()
	m.Set(1, EventAIInsight, ChannelNotification)
	if chs, _ := m.Subscriptions(context.Background(), 1, EventAIInsight); len(chs) != 1 {
		t.Fatal("set failed")
	}
	m.Subscribe(1, EventAIInsight, ChannelEmail)
	chs, _ := m.Subscriptions(context.Background(), 1, EventAIInsight)
	if len(chs) != 2 {
		t.Fatalf("subscribe failed, got %d", len(chs))
	}
	m.Unsubscribe(1, EventAIInsight)
	if chs, _ := m.Subscriptions(context.Background(), 1, EventAIInsight); chs != nil {
		t.Fatal("unsubscribe failed")
	}
}

func TestDeliverContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fe := &fakeEmail{emails: map[int][]string{1: {"a@b.com"}}}
	d := NewEmailDeliverer(fe)
	if err := d.Deliver(ctx, Payload{UserID: 1, Title: "t", Body: "b"}); err == nil {
		t.Fatal("expected context error")
	}
}

func TestDeliverTimeoutNotBlocking(t *testing.T) {
	// Webhook 投递不应无限阻塞（client 有超时）。
	svc := NewService(NewMemSubscriber())
	svc.Register(NewWebhookDeliverer(&fakeWebhook{targets: map[int][]WebhookTarget{
		1: {{URL: "http://127.0.0.1:1", TimeoutSec: 1}},
	}}))
	sub := svc.subscriber.(*MemSubscriber)
	sub.Set(1, EventAIAnswer, ChannelWebhook)

	done := make(chan struct{})
	go func() {
		svc.Deliver(context.Background(), Payload{Event: EventAIAnswer, Title: "t", Body: "b", UserID: 1})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("deliver blocked beyond webhook timeout")
	}
}
