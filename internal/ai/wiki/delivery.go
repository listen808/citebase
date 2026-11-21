package wiki

import (
	"context"
	"sync"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai/delivery"
)

// delivery 整包此前已完整实现但生产代码零调用点。这里接上：
// 审核项入队、pin 冲突、体检发现问题时主动通知到人，
// 否则这些需要人工判断的项会静默堆在队列里没人知道。
var (
	deliverOnce sync.Once
	deliverSvc  *delivery.Service
)

func deliveryService() *delivery.Service {
	deliverOnce.Do(func() {
		svc := delivery.NewService(nil)
		svc.Register(delivery.NewNotificationDeliverer(nil))
		deliverSvc = svc
	})
	return deliverSvc
}

// notify 发送一条站内通知。失败只记日志：通知失败不应阻断编译主流程。
func notify(ctx context.Context, event delivery.EventType, title, body string, userID, relatedID int) {
	if userID <= 0 {
		return
	}
	svc := deliveryService()
	if svc == nil {
		return
	}
	results := svc.Deliver(ctx, delivery.Payload{
		Event:     event,
		Title:     title,
		Body:      body,
		UserID:    userID,
		RelatedID: relatedID,
	})
	for _, r := range results {
		if !r.OK {
			logx.Infof("[WikiDelivery] notify %s failed on %s: %s", event, r.Channel, r.Err)
		}
	}
}
