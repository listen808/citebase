package delivery

import (
	adminsvc "github.com/listen808/citebase/internal/service/admin"
)

// adminEmailAdapter 把 adminsvc.EmailService 适配为 EmailSender 接口。
// ResolveEmails 由外部注入的 resolver 提供（按 userID 解析邮箱地址），
// 这样 delivery 包无需反向依赖用户仓储，装配时由上层传入。
type adminEmailAdapter struct {
	svc      *adminsvc.EmailService
	resolver func(userID int) []string
}

// NewEmailDelivererFromService 用 adminsvc.EmailService 构造邮件投递器。
// resolver 负责把用户 ID 解析为邮箱地址列表；传 nil 时邮件渠道对无地址用户报错。
func NewEmailDelivererFromService(svc *adminsvc.EmailService, resolver func(userID int) []string) *EmailDeliverer {
	return NewEmailDeliverer(&adminEmailAdapter{svc: svc, resolver: resolver})
}

func (a *adminEmailAdapter) SendRaw(subject, htmlBody string, toEmails []string) error {
	return a.svc.SendRaw(subject, htmlBody, toEmails)
}

func (a *adminEmailAdapter) ResolveEmails(userID int) []string {
	if a.resolver == nil {
		return nil
	}
	return a.resolver(userID)
}
