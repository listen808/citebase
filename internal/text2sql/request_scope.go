package text2sql

import (
	"context"

	"github.com/listen808/citebase/internal/ctxkeys"
)

// ============================================================
// 请求级作用域的上下文传递（P1 首要任务 1/2）
//
// 旧实现（BindRequestScope）在进程级共享的 Agent/Generator 字段上加锁替换
// 请求级 kb/会话状态：锁只保护“替换”这一瞬间，生成过程中并发请求 B 仍会读到
// 请求 A 刚换进去的 kb/state，等于一个请求借用另一个请求的身份与数据。
// 这里改为把 RequestScope 放进 context 沿生成/执行链显式传递：
// 每个请求的 goroutine 只看到自己的 scope，天然无竞争、无跨请求借用。
// ============================================================

// RequestScope 一次请求的生成上下文：调用者身份 + 空间 + 该空间知识库 + 该用户会话状态
type RequestScope struct {
	// UserID 真实调用者 ID；0 表示无身份，下游执行链按 fail-closed 拒绝试执行
	UserID int
	// SpaceID 本次请求绑定的空间（Agent execute_sql 等工具据此定位数据源，禁止再用常量 0）
	SpaceID uint64
	// KB 该空间的请求级知识库（不再写回 Generator.kb 进程级字段）
	KB *KnowledgeBase
	// State 该 (conversationID, userID) 专属的会话状态
	State *ConversationState
}

// requestScopeCtxKey 使用空结构体作 key，避免与其他包的字符串 key 冲突
type requestScopeCtxKey struct{}

// WithRequestScope 把请求作用域挂到 ctx 上，沿生成链传递
func WithRequestScope(ctx context.Context, scope *RequestScope) context.Context {
	if ctx == nil || scope == nil {
		return ctx
	}
	return context.WithValue(ctx, requestScopeCtxKey{}, scope)
}

// RequestScopeFromContext 读取请求作用域；无绑定时返回 nil
func RequestScopeFromContext(ctx context.Context) *RequestScope {
	if ctx == nil {
		return nil
	}
	if s, ok := ctx.Value(requestScopeCtxKey{}).(*RequestScope); ok {
		return s
	}
	return nil
}

// UserIDFromContext 解析调用者身份：优先取请求作用域，其次兼容
// ctxkeys 共享 key。返回 0 表示无身份，执行链将按 fail-closed 拒绝。
func UserIDFromContext(ctx context.Context) int {
	if s := RequestScopeFromContext(ctx); s != nil && s.UserID > 0 {
		return s.UserID
	}
	if ctx != nil {
		if v := ctxkeys.UserID(ctx); v > 0 {
			return v
		}
	}
	return 0
}

// SpaceIDFromContext 解析本次请求绑定的空间 ID；无绑定返回 0
func SpaceIDFromContext(ctx context.Context) uint64 {
	if s := RequestScopeFromContext(ctx); s != nil && s.SpaceID > 0 {
		return s.SpaceID
	}
	return 0
}

// kbFromContext 返回本次请求的知识库；未绑定时回退 Generator.kb（进程级初始化值，
// 仅由启动装配写入，不再在请求路径被并发替换）。
func (g *Generator) kbFromContext(ctx context.Context) *KnowledgeBase {
	if s := RequestScopeFromContext(ctx); s != nil && s.KB != nil {
		return s.KB
	}
	return g.kb
}

// convStateFrom 返回本次请求专属的会话状态；未绑定时回退 Agent 初始化时的实例
func (a *Agent) convStateFrom(ctx context.Context) *ConversationState {
	if s := RequestScopeFromContext(ctx); s != nil && s.State != nil {
		return s.State
	}
	return a.conversationState
}
