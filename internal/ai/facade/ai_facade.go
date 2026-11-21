package facade

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai"
	"github.com/listen808/citebase/internal/ctxkeys"
	modelai "github.com/listen808/citebase/internal/model/ai"
	"github.com/listen808/citebase/internal/service/schema"
	"github.com/listen808/citebase/internal/text2sql"
	"github.com/listen808/citebase/internal/text2sql/trust"
)

type AIFacade struct {
	aiModule      *ai.Module
	schemaService *schema.SchemaService
	textToSQLGen  *text2sql.Generator
}

func NewAIFacade(aiModule *ai.Module, schemaService *schema.SchemaService, textToSQLGen *text2sql.Generator) *AIFacade {
	return &AIFacade{
		aiModule:      aiModule,
		schemaService: schemaService,
		textToSQLGen:  textToSQLGen,
	}
}

func (f *AIFacade) GenerateSQL(ctx context.Context, question string, spaceID uint64) (*modelai.SQLResult, *text2sql.ClarificationResult, error) {
	return f.GenerateSQLWithHistory(ctx, question, spaceID, nil)
}

func (f *AIFacade) GenerateSQLWithHistory(ctx context.Context, question string, spaceID uint64, history []text2sql.ConversationMessage) (*modelai.SQLResult, *text2sql.ClarificationResult, error) {
	gen := f.getTextToSQLGenerator()
	if gen == nil {
		return nil, nil, fmt.Errorf("text2sql generator not available")
	}

	traceID := newTraceID()
	ctx = text2sql.WithTraceID(ctx, traceID)

	start := time.Now()
	result, clarify, err := gen.GenerateWithConversation(ctx, question, spaceID, history)
	duration := time.Since(start).Milliseconds()

	if err == nil && result != nil {
		f.recordGenerationAudit(ctx, traceID, question, spaceID, result, duration, nil)
	} else if err != nil {
		f.recordGenerationAudit(ctx, traceID, question, spaceID, nil, duration, err)
	}

	return result, clarify, err
}

func (f *AIFacade) GenerateSQLWithRAG(ctx context.Context, question string, spaceID uint64) (*modelai.SQLResult, error) {
	if f.schemaService != nil {
		// 安全收口：沿调用链透传 ctx（携带调用者身份），供 provider 层做 Token 配额预检
		ragResult, err := f.schemaService.TextToSQLGenerateSQLWithRAGContext(ctx, question, spaceID)
		if err != nil {
			return nil, err
		}
		return ragResult.SQLResult, nil
	}
	return nil, fmt.Errorf("schema service not available")
}

func (f *AIFacade) ExecuteSQL(ctx context.Context, sqlStr string, spaceID uint64, traceID string, userID int) (*text2sql.ExecResult, error) {
	if traceID == "" {
		traceID = newTraceID()
	}

	if f.textToSQLGen != nil {
		executor := f.textToSQLGen.GetExecutor()
		if executor != nil {
			start := time.Now()
			result := executor.ExecuteOnSpaceWithUser(sqlStr, spaceID, userID)
			duration := time.Since(start).Milliseconds()

			var execErr error
			if !result.Success {
				execErr = fmt.Errorf("%s", result.Error)
			}

			f.recordExecutionAudit(ctx, traceID, sqlStr, spaceID, result, duration, execErr)

			if execErr != nil {
				return result, execErr
			}
			return result, nil
		}
	}
	return nil, fmt.Errorf("SQL executor not available")
}

func (f *AIFacade) GetSchemaService() *schema.SchemaService {
	return f.schemaService
}

func (f *AIFacade) GetAIModule() *ai.Module {
	return f.aiModule
}

func (f *AIFacade) GetTextToSQLGenerator() *text2sql.Generator {
	return f.textToSQLGen
}

func (f *AIFacade) getTextToSQLGenerator() *text2sql.Generator {
	if f.textToSQLGen != nil {
		return f.textToSQLGen
	}
	if f.schemaService != nil {
		return f.schemaService.GetTextToSQLGenerator()
	}
	return nil
}

func (f *AIFacade) recordGenerationAudit(ctx context.Context, traceID, question string, spaceID uint64, result *modelai.SQLResult, durationMs int64, err error) {
	auditor := f.getAuditor()
	if auditor == nil {
		return
	}

	trace := &trust.SQLAuditTrace{
		TraceID:       traceID,
		SpaceID:       spaceID,
		NLQuery:       question,
		Intent:        "sql",
		GeneratorTier: "",
		Timestamp:     time.Now(),
		DurationMs:    durationMs,
		// P1（清单 53）：审计必须能归属到人；身份沿 ctx 透传（provider.ContextWithUserID
		// 写入的字符串 key），缺失时记 0 并可被检出，而不是整表 UserID=0 无法追责。
		UserID: userIDFromCtx(ctx),
	}

	if result != nil {
		trace.GeneratedSQL = result.SQL
		trace.Explanation = result.Explanation
		trace.GeneratorTier = result.GeneratorTier
	}
	if err != nil {
		trace.Error = err.Error()
	}

	if err := auditor.Record(ctx, trace); err != nil {
		// P1（清单 53）：审计写入失败不再被吞掉——至少降级为告警日志留痕
		logx.Infof("[AIFacade] generation audit write failed (trace=%s): %v", traceID, err)
	}
}

func (f *AIFacade) recordExecutionAudit(ctx context.Context, traceID, sqlStr string, spaceID uint64, result *text2sql.ExecResult, durationMs int64, err error) {
	auditor := f.getAuditor()
	if auditor == nil || result == nil {
		return
	}

	trace := &trust.SQLAuditTrace{
		TraceID:      traceID,
		SpaceID:      spaceID,
		GeneratedSQL: sqlStr,
		Timestamp:    time.Now(),
		DurationMs:   durationMs,
		UserID:       userIDFromCtx(ctx),
	}

	trace.ExecutionResult = &trust.ExecResult{
		Success: result.Success,
		Columns: result.Columns,
		// P1（清单 52）：审计明细只留前 N 行采样 + 真实行数，不再把整张结果集
		// （可达数千行业务数据）复制进 AIAuditLog.Detail——审计表读取权限通常
		// 宽于业务数据源，全量留存等于二次扩大泄露面且无界增长。
		Rows:     sampleAuditRows(result.Rows, auditRowSampleLimit),
		RowCount: result.RowCount,
		Duration: result.Duration,
		Error:    result.Error,
	}
	if err != nil {
		trace.Error = err.Error()
	}

	if err := auditor.Record(ctx, trace); err != nil {
		// P1（清单 53）：审计写入失败降级为告警日志，不再静默吞掉
		logx.Infof("[AIFacade] execution audit write failed (trace=%s): %v", traceID, err)
	}
}

// auditRowSampleLimit 审计留证的结果行采样上限
const auditRowSampleLimit = 20

// sampleAuditRows 截取前 n 行用于审计留证（RowCount 仍记录真实行数）
func sampleAuditRows(rows []map[string]interface{}, n int) []map[string]interface{} {
	if rows == nil || len(rows) <= n {
		return rows
	}
	return rows[:n]
}

// userIDFromCtx 读取调用者身份（ctxkeys 共享 key）。
func userIDFromCtx(ctx context.Context) int {
	return ctxkeys.UserID(ctx)
}

func (f *AIFacade) getAuditor() *trust.SQLAuditRecorder {
	gen := f.getTextToSQLGenerator()
	if gen != nil {
		return gen.GetAuditor()
	}
	return nil
}

func newTraceID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Int63())
}
