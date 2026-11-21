package rag

import (
	"context"
	"fmt"
	"strings"

	adminsvc "github.com/listen808/citebase/internal/service/admin"
)

type SendEmailTool struct {
	emailService *adminsvc.EmailService
}

func NewSendEmailTool(emailService *adminsvc.EmailService) *SendEmailTool {
	return &SendEmailTool{
		emailService: emailService,
	}
}

func (t *SendEmailTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "send_email",
		Description: "发送邮件通知给指定收件人。需要系统已配置邮件服务器",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"to": map[string]interface{}{
					"type":        "string",
					"description": "收件人邮箱（多个用逗号分隔）",
				},
				"subject": map[string]interface{}{
					"type":        "string",
					"description": "邮件主题",
				},
				"body": map[string]interface{}{
					"type":        "string",
					"description": "邮件正文（支持Markdown格式）",
				},
			},
			"required": []string{"to", "subject", "body"},
		},
	}
}

func (t *SendEmailTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	to, _ := params["to"].(string)
	subject, _ := params["subject"].(string)
	body, _ := params["body"].(string)

	if to == "" || subject == "" || body == "" {
		return &ToolResult{ToolName: "send_email", Success: false, Error: "to, subject, body are required"}, nil
	}

	recipients := strings.Split(to, ",")
	for i := range recipients {
		recipients[i] = strings.TrimSpace(recipients[i])
	}

	if t.emailService == nil {
		return &ToolResult{ToolName: "send_email", Success: false, Error: "邮件服务未初始化"}, nil
	}

	if err := t.emailService.SendRaw(subject, body, recipients); err != nil {
		return &ToolResult{
			ToolName: "send_email",
			Success:  false,
			Error:    fmt.Sprintf("邮件发送失败: %v", err),
		}, nil
	}

	return &ToolResult{
		ToolName: "send_email",
		Success:  true,
		Result:   fmt.Sprintf("邮件发送成功: 收件人=%v, 主题=%s", recipients, subject),
	}, nil
}
