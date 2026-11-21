package rag

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	logx "github.com/listen808/citebase/internal/log"
)

// CommandHandler 斜杠命令/模式匹配处理器
type CommandHandler func(ctx context.Context, args string, chatCtx *ChatContext) (handled bool, response string, err error)

// PatternRule 模式匹配规则
type PatternRule struct {
	Pattern string // 正则表达式
	Handler CommandHandler
	Name    string
}

// ChatContext 对话上下文
type ChatContext struct {
	UserID   int
	SpaceID  int
	History  []ChatMessage
	Question string
}

// HandleResult 处理结果
type HandleResult struct {
	Handled   bool
	Response  string
	Component UIComponent
	Err       error
}

// WorkflowHandler 前置拦截器：在 LLM 调用前尝试确定性处理
type WorkflowHandler struct {
	commands map[string]CommandHandler
	patterns []PatternRule
}

// NewWorkflowHandler 创建默认工作流处理器（注册内置命令）
func NewWorkflowHandler() *WorkflowHandler {
	w := &WorkflowHandler{commands: make(map[string]CommandHandler)}
	w.RegisterCommand("help", w.handleHelp)
	w.RegisterCommand("reset", w.handleReset)
	w.RegisterCommand("report", w.handleReport)
	w.RegisterCommand("summary", w.handleSummary)
	return w
}

// RegisterCommand 注册斜杠命令
func (w *WorkflowHandler) RegisterCommand(name string, handler CommandHandler) {
	w.commands[strings.ToLower(name)] = handler
}

// RegisterPattern 注册模式匹配规则
func (w *WorkflowHandler) RegisterPattern(name, pattern string, handler CommandHandler) {
	w.patterns = append(w.patterns, PatternRule{Pattern: pattern, Handler: handler, Name: name})
}

// TryHandle 尝试处理用户消息
func (w *WorkflowHandler) TryHandle(ctx context.Context, chatCtx *ChatContext) HandleResult {
	q := strings.TrimSpace(chatCtx.Question)
	if q == "" {
		return HandleResult{Handled: false}
	}

	// 1. 检查斜杠命令
	if strings.HasPrefix(q, "/") {
		parts := strings.SplitN(q[1:], " ", 2)
		cmd := strings.ToLower(parts[0])
		args := ""
		if len(parts) > 1 {
			args = parts[1]
		}
		if handler, ok := w.commands[cmd]; ok {
			handled, response, err := handler(ctx, args, chatCtx)
			return HandleResult{Handled: handled, Response: response, Err: err}
		}
	}

	// 2. 检查模式匹配
	for _, rule := range w.patterns {
		matched, err := regexp.MatchString(rule.Pattern, q)
		if err != nil {
			// P3：正则编译错误应记录日志，避免静默失效
			logx.Infof("[WorkflowHandler] invalid pattern %q: %v", rule.Pattern, err)
			continue
		}
		if matched {

			handled, response, err := rule.Handler(ctx, q, chatCtx)
			return HandleResult{Handled: handled, Response: response, Err: err}
		}
	}

	return HandleResult{Handled: false}
}

// ===== 内置命令 =====

// handleHelp 显示帮助信息
func (w *WorkflowHandler) handleHelp(ctx context.Context, args string, chatCtx *ChatContext) (bool, string, error) {
	help := "## AI 助手帮助\n\n" +
		"**快捷命令：**\n" +
		"- `/help` — 显示此帮助信息\n" +
		"- `/reset` — 重置对话\n" +
		"- `/report` — 生成空间报告\n" +
		"- `/summary` — 总结当前空间\n\n" +
		"**快捷操作：**\n" +
		"- 知识库问答：直接输入问题\n" +
		"- 生成 SQL：输入\"查询xxx数据\"\n" +
		"- 分析文档：输入\"分析文档xxx\"\n" +
		"- 翻译文本：输入\"翻译：xxx\"\n\n" +
		"您也可以直接描述需求，我会自动判断意图并调用相应工具。"
	return true, help, nil
}

// handleReset 重置对话
func (w *WorkflowHandler) handleReset(ctx context.Context, args string, chatCtx *ChatContext) (bool, string, error) {
	return true, "对话已重置。请开始新的提问。", nil
}

// handleReport 生成报告
func (w *WorkflowHandler) handleReport(ctx context.Context, args string, chatCtx *ChatContext) (bool, string, error) {
	if chatCtx.SpaceID <= 0 {
		return true, "请先选择一个空间再生成报告。", nil
	}
	return false, "", nil // 未拦截，交由 LLM 处理
}

// handleSummary 总结空间
func (w *WorkflowHandler) handleSummary(ctx context.Context, args string, chatCtx *ChatContext) (bool, string, error) {
	if chatCtx.SpaceID <= 0 {
		return true, "请先选择一个空间再生成摘要。", nil
	}
	return false, "", nil // 未拦截，交由 LLM 处理
}

// StarterUI 返回新对话时的欢迎组件
func (w *WorkflowHandler) StarterUI(spaceName string) UIComponent {
	buttons := []ButtonItem{
		{Label: "知识库问答", Variant: "primary", Action: ButtonAction{Action: "quick_ask", Payload: map[string]interface{}{"prompt": "请介绍一下这个知识库"}}},
		{Label: "生成 SQL", Variant: "secondary", Action: ButtonAction{Action: "quick_sql", Payload: map[string]interface{}{"prompt": "查询用户表数据"}}},
		{Label: "分析文档", Variant: "outline", Action: ButtonAction{Action: "quick_analyze", Payload: map[string]interface{}{}}},
		{Label: "翻译文本", Variant: "ghost", Action: ButtonAction{Action: "quick_translate", Payload: map[string]interface{}{}}},
	}
	statusItems := []StatusItem{
		{Label: "模式", Value: "RAG + Agent", Color: "success"},
		{Label: "空间", Value: spaceName, Color: "info"},
	}
	statusCard := &StatusCardComponent{
		BaseComponent: BaseComponent{CompID: "starter-status", CompType: CompTypeStatusCard, Title: "当前状态"},
		Items:         statusItems,
	}
	_ = statusCard
	btnGroup := &ButtonGroupComponent{
		BaseComponent: BaseComponent{CompID: "starter-buttons", CompType: CompTypeButtonGroup, Title: "快捷操作"},
		Buttons:       buttons,
	}
	return btnGroup
}

// String 返回格式化字符串（调试用）
func (h HandleResult) String() string {
	return fmt.Sprintf("HandleResult{Handled: %v, Response: %q, Err: %v}", h.Handled, h.Response, h.Err)
}
