package middleware

import (
	"context"
	"fmt"
)

type SafetyCheckFunc func(ctx context.Context, query string) (safe bool, reason string)
type AnswerValidateFunc func(ctx context.Context, answer string) string
type AuditLogFunc func(userID int, eventType, query, reason string)

type SafetyGuardMiddleware struct {
	CheckQuery  SafetyCheckFunc
	ValidateAns AnswerValidateFunc
	LogAudit    AuditLogFunc
}

func NewSafetyGuardMiddleware(checkFn SafetyCheckFunc, validateFn AnswerValidateFunc, auditFn AuditLogFunc) *SafetyGuardMiddleware {
	return &SafetyGuardMiddleware{
		CheckQuery:  checkFn,
		ValidateAns: validateFn,
		LogAudit:    auditFn,
	}
}

func (m *SafetyGuardMiddleware) Name() string {
	return "safety_guard"
}

func (m *SafetyGuardMiddleware) BeforeRetrieval(ctx *RAGContext) {
	if m.CheckQuery == nil {
		return
	}
	safe, reason := m.CheckQuery(ctx.Ctx, ctx.Question)
	if !safe {
		if m.LogAudit != nil {
			m.LogAudit(0, "query_blocked", ctx.Question, reason)
		}
		ctx.Err = fmt.Errorf("query blocked by safety policy: %s", reason)
	}
}

func (m *SafetyGuardMiddleware) AfterRetrieval(ctx *RAGContext) {
}

func (m *SafetyGuardMiddleware) BeforeGeneration(ctx *RAGContext) {
}

func (m *SafetyGuardMiddleware) AfterGeneration(ctx *RAGContext) {
	if m.ValidateAns == nil || ctx.Answer == "" {
		return
	}
	ctx.Answer = m.ValidateAns(ctx.Ctx, ctx.Answer)
}
