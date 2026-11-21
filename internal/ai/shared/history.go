package shared

import (
	"context"
	"fmt"
	"strings"

	"github.com/listen808/citebase/internal/ai/provider"
)

// ChatMessage 统一的对话消息类型
type ChatMessage struct {
	Role    string // "user" 或 "assistant"
	Content string
}

// CompressHistory 压缩对话历史（统一实现）
// 保留最近 keepRecent 条消息，将更早的消息用 LLM 总结
func CompressHistory(ctx context.Context, p provider.Provider, history []ChatMessage, keepRecent int) ([]ChatMessage, error) {
	if len(history) <= keepRecent {
		return history, nil
	}

	older := history[:len(history)-keepRecent]
	recent := history[len(history)-keepRecent:]

	// 构建总结 prompt
	var sb strings.Builder
	sb.WriteString("请将以下对话历史总结为简洁的摘要（200-300字）：\n\n")
	for _, msg := range older {
		role := "用户"
		if msg.Role == "assistant" {
			role = "助手"
		}
		sb.WriteString(fmt.Sprintf("%s: %s\n", role, msg.Content))
	}

	summary, err := p.Chat(ctx, "你是一个对话历史总结器。请将对话历史压缩为简洁摘要。", []provider.Message{
		provider.NewTextMessage("user", sb.String()),
	})
	if err != nil {
		// 降级：直接截断
		return recent, nil
	}

	// 将摘要作为系统上下文
	compressed := []ChatMessage{
		{Role: "system", Content: fmt.Sprintf("[对话摘要]\n%s", summary.Content)},
	}
	compressed = append(compressed, recent...)
	return compressed, nil
}
