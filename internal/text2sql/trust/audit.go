package trust

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	aimodel "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
)

// SQLAuditTrace Text-to-SQL 完整审计链路
type SQLAuditTrace struct {
	TraceID         string                     `json:"trace_id"`
	SpaceID         uint64                     `json:"space_id"`
	UserID          int                        `json:"user_id"`
	NLQuery         string                     `json:"nl_query"`
	Intent          string                     `json:"intent"`
	GeneratorTier   string                     `json:"generator_tier"`
	GeneratedSQL    string                     `json:"generated_sql"`
	SecurityPolicy  *aimodel.SQLSecurityPolicy `json:"security_policy,omitempty"`
	PolicyResult    *PolicyCheckResult         `json:"policy_result,omitempty"`
	ExecutionResult *ExecResult                `json:"execution_result,omitempty"`
	Explanation     *aimodel.SQLExplanation    `json:"explanation,omitempty"`
	Error           string                     `json:"error"`
	Feedback        *SQLFeedback               `json:"feedback,omitempty"`
	Model           string                     `json:"model"`
	Provider        string                     `json:"provider"`
	TokenUsage      int                        `json:"token_usage"`
	DurationMs      int64                      `json:"duration_ms"`
	Timestamp       time.Time                  `json:"timestamp"`
}

// ExecResult SQL 执行结果（与 text2sql.ExecResult 同构，避免循环导入）
type ExecResult struct {
	Success  bool                     `json:"success"`
	Columns  []string                 `json:"columns"`
	Rows     []map[string]interface{} `json:"rows"`
	RowCount int                      `json:"row_count"`
	Duration int                      `json:"duration"`
	Error    string                   `json:"error,omitempty"`
}

// SQLFeedback 用户反馈
type SQLFeedback struct {
	Rating  int    `json:"rating"`
	Comment string `json:"comment"`
}

// SQLAuditRecorder 全链路审计记录器
type SQLAuditRecorder struct {
	repo *airepo.AIAuditLogRepository
}

func NewSQLAuditRecorder(repo *airepo.AIAuditLogRepository) *SQLAuditRecorder {
	return &SQLAuditRecorder{repo: repo}
}

// Record 记录完整审计链路
func (r *SQLAuditRecorder) Record(ctx context.Context, trace *SQLAuditTrace) error {
	if r.repo == nil || trace == nil {
		return nil
	}

	detail, err := json.Marshal(trace)
	if err != nil {
		return fmt.Errorf("marshal audit trace failed: %w", err)
	}

	result := "success"
	if trace.Error != "" || (trace.ExecutionResult != nil && !trace.ExecutionResult.Success) {
		result = "failure"
	}

	answer := ""
	if trace.ExecutionResult != nil {
		answer = fmt.Sprintf("rows=%d duration=%dms", trace.ExecutionResult.RowCount, trace.ExecutionResult.Duration)
	}

	log := &aimodel.AIAuditLog{
		UserID:     trace.UserID,
		Action:     "text2sql",
		TargetID:   trace.TraceID,
		Question:   trace.NLQuery,
		Answer:     answer,
		Detail:     string(detail),
		TokenUsage: trace.TokenUsage,
		Model:      trace.Model,
		Provider:   trace.Provider,
		Result:     result,
		Reason:     trace.Error,
		DurationMs: trace.DurationMs,
	}

	return r.repo.Create(log)
}

// RecordFeedback 仅更新反馈信息
func (r *SQLAuditRecorder) RecordFeedback(ctx context.Context, traceID string, feedback *SQLFeedback) error {
	// 反馈更新通过 SQLHistory 完成，审计日志中保留完整 trace
	return nil
}
