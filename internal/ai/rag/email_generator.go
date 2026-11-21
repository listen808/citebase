package rag

import (
	"context"
	"fmt"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
)

type EmailGenerator struct {
	provider provider.Provider
}

func NewEmailGenerator(p provider.Provider) *EmailGenerator {
	return &EmailGenerator{provider: p}
}

type EmailType string

const (
	EmailTypeNotify  EmailType = "notify"
	EmailTypeShare   EmailType = "share"
	EmailTypeMeeting EmailType = "meeting"
	EmailTypeWeekly  EmailType = "weekly_digest"
	EmailTypeCustom  EmailType = "custom"
)

type EmailRequest struct {
	Type       EmailType `json:"type"`
	Subject    string    `json:"subject"`
	DocContent string    `json:"doc_content"`
	DocTitle   string    `json:"doc_title"`
	Recipients []string  `json:"recipients"`
	Tone       string    `json:"tone"` // formal, semi-formal, casual
	Extra      string    `json:"extra"`
	Language   string    `json:"language"`
}

type EmailResult struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func (g *EmailGenerator) Generate(ctx context.Context, req EmailRequest) (*EmailResult, provider.TokenUsage, error) {
	if g.provider == nil {
		return nil, provider.TokenUsage{}, fmt.Errorf("AI服务未启用")
	}

	if req.Tone == "" {
		req.Tone = "semi-formal"
	}
	if req.Language == "" {
		req.Language = "zh"
	}

	systemPrompt := fmt.Sprintf(`你是一个专业的邮件撰写助手。根据提供的素材生成规范的邮件。

邮件类型：%s
语气：%s
语言：%s

要求：
1. 邮件主题简洁明了
2. 正文结构清晰（称呼、正文、结尾、签名）
3. 语气符合要求
4. 如有文档内容，提炼关键信息
5. 保持专业但不过于生硬

请直接输出JSON格式：
{
  "subject": "邮件主题",
  "body": "邮件正文（支持Markdown格式）"
}`, g.getTypeName(req.Type), req.Tone, req.Language)

	var userPrompt string
	switch req.Type {
	case EmailTypeNotify:
		userPrompt = fmt.Sprintf("请生成一封通知邮件。\n主题参考：%s\n通知内容：\n%s\n%s",
			req.Subject, req.DocContent, g.buildExtra(req.Extra))
	case EmailTypeShare:
		userPrompt = fmt.Sprintf("请生成一封知识分享邮件，将以下文档内容分享给团队。\n文档标题：%s\n文档内容：\n%s\n%s",
			req.DocTitle, req.DocContent, g.buildExtra(req.Extra))
	case EmailTypeMeeting:
		userPrompt = fmt.Sprintf("请根据以下会议记录生成会议纪要邮件。\n会议记录：\n%s\n%s",
			req.DocContent, g.buildExtra(req.Extra))
	case EmailTypeWeekly:
		userPrompt = fmt.Sprintf("请根据以下工作内容生成周报邮件。\n本周工作：\n%s\n%s",
			req.DocContent, g.buildExtra(req.Extra))
	default:
		userPrompt = fmt.Sprintf("请生成一封邮件。\n主题：%s\n内容：\n%s\n%s",
			req.Subject, req.DocContent, g.buildExtra(req.Extra))
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	resp, err := g.provider.Chat(timeoutCtx, systemPrompt, []provider.Message{
		provider.NewTextMessage("user", userPrompt),
	}, provider.WithMaxTokens(4096), provider.WithTemperature(0.5))
	if err != nil {
		return nil, provider.TokenUsage{}, err
	}

	var result EmailResult
	jsonStr := extractJSONFromText(resp.Content)
	if err := parseJSON(jsonStr, &result); err != nil {
		result.Subject = req.Subject
		result.Body = resp.Content
	}

	return &result, resp.TokenUsage, nil
}

func (g *EmailGenerator) FromDocument(ctx context.Context, docTitle, docContent string, emailType EmailType) (*EmailResult, provider.TokenUsage, error) {
	return g.Generate(ctx, EmailRequest{
		Type:       emailType,
		Subject:    docTitle,
		DocTitle:   docTitle,
		DocContent: docContent,
	})
}

func (g *EmailGenerator) getTypeName(t EmailType) string {
	names := map[EmailType]string{
		EmailTypeNotify:  "通知邮件",
		EmailTypeShare:   "知识分享邮件",
		EmailTypeMeeting: "会议纪要邮件",
		EmailTypeWeekly:  "周报邮件",
		EmailTypeCustom:  "自定义邮件",
	}
	if name, ok := names[t]; ok {
		return name
	}
	return "邮件"
}

func (g *EmailGenerator) buildExtra(extra string) string {
	if extra == "" {
		return ""
	}
	return fmt.Sprintf("额外要求：%s", extra)
}
