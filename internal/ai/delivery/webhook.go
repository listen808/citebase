package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WebhookTarget 一个 Webhook 投递目标（URL + 可选签名头）。
type WebhookTarget struct {
	URL        string            `json:"url"`
	Headers    map[string]string `json:"headers,omitempty"`
	Secret     string            `json:"secret,omitempty"` // 可选，用于 X-Signature 校验
	TimeoutSec int               `json:"timeout_sec,omitempty"`
}

// WebhookSender 解析某用户关联的 Webhook 目标列表。
type WebhookSender interface {
	// Targets 返回 userID 订阅的 Webhook 目标；无则返回 nil。
	Targets(userID int) []WebhookTarget
}

// WebhookDeliverer 把 payload 通过 HTTP POST(JSON) 投递到订阅的 Webhook。
type WebhookDeliverer struct {
	sender WebhookSender
	client *http.Client
}

// NewWebhookDeliverer 用 WebhookSender 构造 Webhook 投递器。
func NewWebhookDeliverer(sender WebhookSender) *WebhookDeliverer {
	return &WebhookDeliverer{
		sender: sender,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Channel 返回渠道标识。
func (d *WebhookDeliverer) Channel() Channel { return ChannelWebhook }

// Deliver 向所有目标 Webhook 并发 POST JSON；任一失败即返回错误。
func (d *WebhookDeliverer) Deliver(ctx context.Context, p Payload) error {
	if d.sender == nil {
		return errWrap(ChannelWebhook, fmt.Errorf("no webhook sender configured"))
	}
	targets := d.sender.Targets(p.UserID)
	if len(targets) == 0 {
		return errWrap(ChannelWebhook, fmt.Errorf("no webhook target for user %d", p.UserID))
	}

	body, err := json.Marshal(p)
	if err != nil {
		return errWrap(ChannelWebhook, err)
	}

	var firstErr error
	for _, t := range targets {
		if err := d.post(ctx, t, body); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (d *WebhookDeliverer) post(ctx context.Context, t WebhookTarget, body []byte) error {
	timeout := time.Duration(t.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	c := &http.Client{Timeout: timeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.URL, bytes.NewReader(body))
	if err != nil {
		return errWrap(ChannelWebhook, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "citebase-ai-delivery/1.0")
	for k, v := range t.Headers {
		req.Header.Set(k, v)
	}
	if t.Secret != "" {
		req.Header.Set("X-Signature", signPayload(t.Secret, body))
	}

	resp, err := c.Do(req)
	if err != nil {
		return errWrap(ChannelWebhook, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errWrap(ChannelWebhook, fmt.Errorf("webhook returned status %d", resp.StatusCode))
	}
	return nil
}

// signPayload 用 HMAC-SHA256 生成签名（Secret 非空时启用校验）。
func signPayload(secret string, body []byte) string {
	if secret == "" {
		return ""
	}
	return hmacHex(secret, body)
}
