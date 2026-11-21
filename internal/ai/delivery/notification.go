package delivery

import (
	"context"
	"fmt"

	socialmodel "github.com/listen808/citebase/internal/model/social"
	socialsvc "github.com/listen808/citebase/internal/service/social"
)

// NotificationSender 站内通知发送最小接口（由 socialsvc.NotificationService 实现）。
// 用接口解耦，便于在测试中注入假实现。
type NotificationSender interface {
	CreateNotification(userID int, notifType, title, content string, relatedID int) error
}

// NotificationDeliverer 把 payload 投递为站内通知。
type NotificationDeliverer struct {
	sender NotificationSender
	// Type 写入通知表的类型，默认 system。
	Type string
}

// NewNotificationDeliverer 用 socialsvc.NotificationService 构造站内通知投递器。
func NewNotificationDeliverer(svc *socialsvc.NotificationService) *NotificationDeliverer {
	return &NotificationDeliverer{sender: svc, Type: socialmodel.NotifTypeSystem}
}

// NewNotificationDelivererWith 用自定义发送器构造（测试用）。
func NewNotificationDelivererWith(sender NotificationSender) *NotificationDeliverer {
	return &NotificationDeliverer{sender: sender, Type: socialmodel.NotifTypeSystem}
}

// Channel 返回渠道标识。
func (d *NotificationDeliverer) Channel() Channel { return ChannelNotification }

// Deliver 创建一条站内通知。
func (d *NotificationDeliverer) Deliver(ctx context.Context, p Payload) error {
	if d.sender == nil {
		return errWrap(ChannelNotification, fmt.Errorf("no notification sender configured"))
	}
	select {
	case <-ctx.Done():
		return errWrap(ChannelNotification, ctx.Err())
	default:
	}
	content := p.Body
	if len(content) > 1000 {
		content = content[:1000] // 适配 aw_notification.content 长度
	}
	if err := d.sender.CreateNotification(p.UserID, d.Type, p.Title, content, p.RelatedID); err != nil {
		return errWrap(ChannelNotification, err)
	}
	return nil
}
