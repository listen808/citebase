package delivery

import (
	"context"
	"fmt"
	"strings"
)

// EmailSender 邮件发送最小接口。adminsvc.EmailService 已实现兼容签名
// SendRaw(subject, htmlBody string, toEmails []string) error，可直接适配。
type EmailSender interface {
	// SendRaw 发送一封原始邮件。
	SendRaw(subject, htmlBody string, toEmails []string) error
	// ResolveEmails 根据用户 ID 解析目标邮箱地址；无法解析返回 nil。
	ResolveEmails(userID int) []string
}

// EmailDeliverer 把 payload 作为邮件投递。
type EmailDeliverer struct {
	sender EmailSender
	// TitlePrefix 邮件标题前缀（可选）。
	TitlePrefix string
}

// NewEmailDeliverer 用实现了 EmailSender 的服务（如 adminsvc.EmailService 适配层）构造。
func NewEmailDeliverer(sender EmailSender) *EmailDeliverer {
	return &EmailDeliverer{sender: sender}
}

// Channel 返回渠道标识。
func (d *EmailDeliverer) Channel() Channel { return ChannelEmail }

// Deliver 发送邮件。接收地址通过 ResolveEmails 从 userID 解析。
func (d *EmailDeliverer) Deliver(ctx context.Context, p Payload) error {
	if d.sender == nil {
		return errWrap(ChannelEmail, fmt.Errorf("no email sender configured"))
	}
	select {
	case <-ctx.Done():
		return errWrap(ChannelEmail, ctx.Err())
	default:
	}

	to := d.sender.ResolveEmails(p.UserID)
	if len(to) == 0 {
		return errWrap(ChannelEmail, fmt.Errorf("no email address for user %d", p.UserID))
	}

	subject := p.Title
	if d.TitlePrefix != "" {
		subject = d.TitlePrefix + " - " + p.Title
	}
	// 纯文本正文包一层简单 HTML，兼容现有邮件渲染风格。
	html := renderEmailHTML(subject, p.Body)
	if err := d.sender.SendRaw(subject, html, to); err != nil {
		return errWrap(ChannelEmail, err)
	}
	return nil
}

// renderEmailHTML 生成轻量 HTML 邮件体。
func renderEmailHTML(title, body string) string {
	escaped := strings.ReplaceAll(body, "<", "&lt;")
	escaped = strings.ReplaceAll(escaped, ">", "&gt;")
	escaped = strings.ReplaceAll(escaped, "\n", "<br/>")
	return fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"></head>
<body style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;color:#1d1d1f;padding:32px 20px;">
<div style="max-width:600px;margin:0 auto;">
<h2 style="font-size:20px;font-weight:600;">%s</h2>
<p style="color:#1d1d1f;font-size:15px;line-height:1.6;">%s</p>
</div></body></html>`, title, escaped)
}
