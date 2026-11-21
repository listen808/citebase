package text2sql

import (
	"context"
	"fmt"
	"time"

	modelai "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
)

// Text2SQL 生成器：推理链追踪采集（TraceCollector）与 context traceID 传递。

// TraceCollector SQL 生成链路追踪收集器
type TraceCollector struct {
	traceID        string
	spaceID        uint64
	conversationID int
	repo           *airepo.SQLTraceRepo
	steps          []TraceStep
	startTime      time.Time
}

// TraceStep 单个追踪步骤
type TraceStep struct {
	Name       string
	Input      string
	Output     string
	DurationMs int
	Status     string
	ErrorMsg   string
}

// NewTraceCollector 创建 Trace 收集器
func NewTraceCollector(spaceID uint64, conversationID int, repo *airepo.SQLTraceRepo) *TraceCollector {
	return &TraceCollector{
		traceID:        fmt.Sprintf("trace_%d_%d", time.Now().UnixNano(), spaceID),
		spaceID:        spaceID,
		conversationID: conversationID,
		repo:           repo,
		steps:          make([]TraceStep, 0),
		startTime:      time.Now(),
	}
}

// TraceID 获取追踪ID
func (tc *TraceCollector) TraceID() string {
	return tc.traceID
}

// AddStep 添加追踪步骤
func (tc *TraceCollector) AddStep(name, input, output, status string, durationMs int, errMsg string) {
	tc.steps = append(tc.steps, TraceStep{
		Name:       name,
		Input:      input,
		Output:     output,
		DurationMs: durationMs,
		Status:     status,
		ErrorMsg:   errMsg,
	})
}

// Flush 保存追踪记录
func (tc *TraceCollector) Flush(userQuery, intent, finalSQL string, execOK bool) {
	if tc.repo == nil {
		return
	}
	durationMs := int(time.Since(tc.startTime).Milliseconds())

	// 转换steps
	traceSteps := make([]modelai.SQLTraceStep, len(tc.steps))
	for i, s := range tc.steps {
		traceSteps[i] = modelai.SQLTraceStep{
			Name:       s.Name,
			Input:      s.Input,
			Output:     s.Output,
			DurationMs: s.DurationMs,
			Status:     s.Status,
			ErrorMsg:   s.ErrorMsg,
		}
	}

	trace := &modelai.SQLTrace{
		TraceID:        tc.traceID,
		SpaceID:        tc.spaceID,
		ConversationID: tc.conversationID,
		UserQuery:      userQuery,
		Intent:         intent,
		Steps:          traceSteps,
		FinalSQL:       finalSQL,
		ExecutionOK:    execOK,
		DurationMs:     durationMs,
	}

	_ = tc.repo.Create(trace)
}

// SetTraceCollector 设置 Trace 收集器
func (g *Generator) SetTraceCollector(repo *airepo.SQLTraceRepo) {
	g.traceRepo = repo
}

// traceStep 记录一个追踪步骤（如果启用且收集器存在）
// name: 步骤名（如 schema_linking/llm_call/validate/execute）
// input/output: 步骤的输入/输出摘要
// status: "ok" / "error" / "skip"
// errMsg: 错误信息（status=error 时填写）
func (g *Generator) traceStep(name, input, output, status, errMsg string) {
	if g.traceCollector == nil || !GetText2SQLConfig().EnableTraceSteps {
		return
	}
	// 截断过长的输入/输出，避免 trace 表膨胀
	if len(input) > 500 {
		input = input[:500] + "..."
	}
	if len(output) > 500 {
		output = output[:500] + "..."
	}
	g.traceCollector.AddStep(name, input, output, status, 0, errMsg)
}

// traceStepTimed 记录一个带耗时的追踪步骤
func (g *Generator) traceStepTimed(name, input, output, status, errMsg string, durationMs int) {
	if g.traceCollector == nil || !GetText2SQLConfig().EnableTraceSteps {
		return
	}
	if len(input) > 500 {
		input = input[:500] + "..."
	}
	if len(output) > 500 {
		output = output[:500] + "..."
	}
	g.traceCollector.AddStep(name, input, output, status, durationMs, errMsg)
}

// traceIDKey context key for trace ID
type traceIDKey struct{}

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

func traceIDFromContext(ctx context.Context) string {
	if v := ctx.Value(traceIDKey{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
