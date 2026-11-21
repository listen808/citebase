// Package ctxkeys 定义跨层共享的 context key 类型，
// 避免 SA1029（context value 使用裸字符串字面量）。
//
// 使用方式：
//
//	ctx = ctxkeys.WithUserID(ctx, 42)
//	id, _ := ctxkeys.UserID(ctx)
package ctxkeys

import "context"

type key string

const (
	// UserIDKey 是当前用户 ID 的 context key。
	// RAG 检索器、权限过滤、agent 工具层均通过此 key 读取调用者身份。
	UserIDKey key = "user_id"

	// ChatHistoryKey 是对话历史的 context key，
	// 供 RAG 上下文压缩层评估 token 预算。
	ChatHistoryKey key = "chat_history"
)

// WithUserID 将 userID 注入 ctx。
func WithUserID(ctx context.Context, userID int) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// UserID 从 ctx 读取 userID，不存在返回 0。
func UserID(ctx context.Context) int {
	v, _ := ctx.Value(UserIDKey).(int)
	return v
}

// WithChatHistory 将对话历史注入 ctx。
func WithChatHistory(ctx context.Context, history interface{}) context.Context {
	return context.WithValue(ctx, ChatHistoryKey, history)
}

// ChatHistory 从 ctx 读取对话历史，不存在返回 nil。
func ChatHistory(ctx context.Context) interface{} {
	return ctx.Value(ChatHistoryKey)
}
