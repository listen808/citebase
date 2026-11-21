package text2sql

import (
	"context"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strings"

	"github.com/listen808/citebase/internal/utils"
)

// ExecutionFeedbackLoop 执行反馈循环 - 根据SQL执行结果动态修正
// 核心思想：执行SQL后，如果结果异常（空结果、错误、怀疑有问题），自动触发修正流程
type ExecutionFeedbackLoop struct {
	generator *Generator
	executor  *SQLExecutor
	validator *SQLSelfValidator
	provider  LLMProvider
	maxRetry  int
}

// NewExecutionFeedbackLoop 创建执行反馈循环
func NewExecutionFeedbackLoop(g *Generator, e *SQLExecutor, p LLMProvider) *ExecutionFeedbackLoop {
	return &ExecutionFeedbackLoop{
		generator: g,
		executor:  e,
		provider:  p,
		maxRetry:  GetText2SQLConfig().FeedbackMaxRetry,
		// P1（清单 102）：注入校验器，避免 QuickValidate 因 validator==nil 恒“通过”
		validator: NewSQLSelfValidator(nil),
	}
}

// FeedbackLevel 反馈级别
type FeedbackLevel int

const (
	FeedbackLevelOK      FeedbackLevel = iota // 结果正常
	FeedbackLevelWarning                      // 警告：结果可疑
	FeedbackLevelError                        // 错误：执行失败
	FeedbackLevelEmpty                        // 空结果
)

// ExecutionFeedbackResult 反馈结果
type ExecutionFeedbackResult struct {
	Level      FeedbackLevel `json:"level"`       // 反馈级别
	Message    string        `json:"message"`     // 反馈信息
	FixedSQL   string        `json:"fixed_sql"`   // 修正后的SQL
	RetryCount int           `json:"retry_count"` // 已重试次数
	Actions    []string      `json:"actions"`     // 采取的行动
}

// AnalyzeResult 分析执行结果并决定是否需要修正
func (ef *ExecutionFeedbackLoop) AnalyzeResult(result *ExecResult, question string) FeedbackLevel {
	if result == nil {
		return FeedbackLevelError
	}

	// 执行错误
	if result.Error != "" {
		return FeedbackLevelError
	}

	// 空结果
	if result.RowCount == 0 {
		return FeedbackLevelEmpty
	}

	// 结果行数异常多（可能有误）
	if result.RowCount > GetText2SQLConfig().ExecutionWarningRows {
		return FeedbackLevelWarning
	}

	return FeedbackLevelOK
}

// ExecuteAndFix 执行SQL并在需要时自动修正
// 流程：执行 -> 分析结果 -> 如果异常则修正 -> 重新执行 -> 直到正常或达到最大重试次数
func (ef *ExecutionFeedbackLoop) ExecuteAndFix(
	ctx context.Context,
	sql string,
	question string,
	schemaContext string,
	spaceID uint64,
) (*ExecResult, *ExecutionFeedbackResult) {
	feedback := &ExecutionFeedbackResult{
		Level:      FeedbackLevelOK,
		RetryCount: 0,
		Actions:    []string{},
		FixedSQL:   sql,
	}

	currentSQL := sql

	for feedback.RetryCount <= ef.maxRetry {
		// 执行SQL（P1 首要任务 1：沿请求 ctx 携带真实调用者身份，无身份仍 fail-closed）
		result := ef.executor.ExecuteWithContext(ctx, currentSQL, spaceID)
		feedback.Level = ef.AnalyzeResult(result, question)

		switch feedback.Level {
		case FeedbackLevelOK:
			feedback.Message = "SQL执行成功，结果正常"
			return result, feedback

		case FeedbackLevelEmpty:
			feedback.Actions = append(feedback.Actions, "检测到空结果，尝试修正SQL")
			feedback.Message = "空结果，可能SQL有误，尝试修正"

		case FeedbackLevelError:
			feedback.Actions = append(feedback.Actions, fmt.Sprintf("执行错误: %s", result.Error))
			feedback.Message = fmt.Sprintf("执行错误: %s", result.Error)

		case FeedbackLevelWarning:
			feedback.Actions = append(feedback.Actions, "结果行数过多，可能需要LIMIT或条件优化")
			feedback.Message = "结果行数过多，可能需要优化"
		}

		// 需要修正，调用LLM修正
		feedback.RetryCount++
		fixedSQL, err := ef.fixSQLWithFeedback(ctx, currentSQL, question, schemaContext, feedback.Level, result)
		if err != nil {
			logx.Infof("[ExecutionFeedbackLoop] fixSQLWithFeedback failed: %v", err)
			feedback.Message = fmt.Sprintf("%s, 修正失败: %v", feedback.Message, err)
			return result, feedback
		}

		if fixedSQL == "" || fixedSQL == currentSQL {
			// 无法进一步修正
			feedback.Message = fmt.Sprintf("%s, 无法进一步修正", feedback.Message)
			return result, feedback
		}

		currentSQL = fixedSQL
		feedback.FixedSQL = fixedSQL
		feedback.Actions = append(feedback.Actions, fmt.Sprintf("修正SQL: %s", utils.TruncateWithEllipsis(fixedSQL, 100)))
	}

	feedback.Message = fmt.Sprintf("已达到最大重试次数(%d)，返回最后结果", ef.maxRetry)
	finalResult := ef.executor.ExecuteWithContext(ctx, currentSQL, spaceID)
	return finalResult, feedback
}

// fixSQLWithFeedback 根据执行反馈修正SQL
func (ef *ExecutionFeedbackLoop) fixSQLWithFeedback(
	ctx context.Context,
	sql string,
	question string,
	schemaContext string,
	level FeedbackLevel,
	result *ExecResult,
) (string, error) {
	prompt := ef.buildFeedbackPrompt(sql, question, schemaContext, level, result)

	messages := []Message{
		NewTextMessage("user", prompt),
	}

	resp, err := ef.provider.ChatWithOptions(ctx, "你是一个SQL修正专家。", messages,
		WithMaxTokens(500),
		WithTemperature(0.2))
	if err != nil {
		return "", err
	}

	// 提取SQL
	return ef.extractSQL(resp.Content), nil
}

// buildFeedbackPrompt 构建反馈修正提示词
func (ef *ExecutionFeedbackLoop) buildFeedbackPrompt(
	sql string,
	question string,
	schemaContext string,
	level FeedbackLevel,
	result *ExecResult,
) string {
	var levelDesc string
	var action string

	switch level {
	case FeedbackLevelEmpty:
		levelDesc = "空结果（0行）"
		action = "请检查SQL是否正确：WHERE条件是否太严格、JOIN条件是否正确、表是否真的有数据"
	case FeedbackLevelError:
		levelDesc = fmt.Sprintf("执行错误: %s", result.Error)
		action = "请修复SQL语法错误或修正查询逻辑"
	case FeedbackLevelWarning:
		levelDesc = fmt.Sprintf("结果行数过多（%d行）", result.RowCount)
		action = "请添加合适的LIMIT或更精确的过滤条件"
	default:
		return ""
	}

	return "请根据以下执行反馈修正SQL。\n\n" +
		"## 用户问题\n" +
		question + "\n\n" +
		"## 当前SQL\n" +
		"```sql\n" +
		sql + "\n" +
		"```\n\n" +
		"## Schema上下文\n" +
		schemaContext + "\n\n" +
		"## 执行反馈\n" +
		"- 状态：" + levelDesc + "\n" +
		"- 操作建议：" + action + "\n\n" +
		"## 任务\n" +
		"1. 分析为什么SQL没有返回预期结果\n" +
		"2. 根据反馈修正SQL\n" +
		"3. 只返回修正后的SQL语句（不需要解释）\n\n" +
		"## 输出格式\n" +
		"```sql\n" +
		"修正后的SQL\n" +
		"```"
}

// extractSQL 从响应中提取SQL
func (ef *ExecutionFeedbackLoop) extractSQL(content string) string {
	lines := strings.Split(content, "\n")
	var sqlLines []string
	inCodeBlock := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		if inCodeBlock || strings.HasPrefix(line, "SELECT") || strings.HasPrefix(line, "WITH") {
			sqlLines = append(sqlLines, line)
		}
	}

	if len(sqlLines) == 0 {
		return ""
	}

	return strings.TrimSpace(strings.Join(sqlLines, "\n"))
}

// QuickValidate 快速验证SQL（不执行）
// P1（清单 102）：validator 缺失时返回“未通过”（fail-closed），调用方必须据此
// 降级处理；旧实现返回 (true,"") 让“没配置校验”伪装成“校验通过”。
func (ef *ExecutionFeedbackLoop) QuickValidate(sql, question, schemaContext string) (bool, string) {
	if ef.validator == nil {
		return false, "SQL 校验器未配置，无法确认 SQL 有效性"
	}

	result, err := ef.validator.ValidateAndCorrect(context.Background(), sql, question, schemaContext)
	if err != nil {
		return false, fmt.Sprintf("验证出错: %v", err)
	}

	if !result.IsValid {
		return false, fmt.Sprintf("SQL无效: %v", result.Issues)
	}

	if result.Confidence < 0.7 {
		return false, fmt.Sprintf("置信度低(%.2f): %v", result.Confidence, result.Suggestions)
	}

	return true, ""
}
