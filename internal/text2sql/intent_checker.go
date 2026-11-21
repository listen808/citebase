package text2sql

import (
	"context"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strings"
)

// IntentCheckResult 意图校验结果
type IntentCheckResult struct {
	Status      string // "answered" | "partial" | "not_answered"
	Reason      string // LLM 给出的判断理由
	ShouldRetry bool   // 是否建议重试（not_answered 时为 true）
}

// IntentChecker 意图-结果一致性校验器
// P1-3 (§4.2.6): 用 LLM 校验执行结果是否真正回答了用户问题
type IntentChecker struct {
	llmProvider LLMProvider
}

// NewIntentChecker 创建意图校验器
func NewIntentChecker(p LLMProvider) *IntentChecker {
	return &IntentChecker{llmProvider: p}
}

// Check 校验执行结果是否回答了用户问题
// question: 用户原始问题
// sql: 执行的 SQL
// execResult: 执行结果（取前 10 行作为 LLM 评估输入）
func (ic *IntentChecker) Check(ctx context.Context, question string, sql string, execResult *ExecResult) *IntentCheckResult {
	if ic == nil || ic.llmProvider == nil {
		return &IntentCheckResult{Status: "answered", Reason: "校验器未启用"}
	}
	if execResult == nil || !execResult.Success {
		return &IntentCheckResult{Status: "answered", Reason: "执行失败，跳过意图校验"}
	}

	// 构造结果摘要（前 10 行）
	resultSummary := buildResultSummary(execResult, 10)

	systemPrompt := `你是一个数据查询结果审查员。你的任务是判断 SQL 查询的执行结果是否真正回答了用户的原始问题。

判断标准：
- "answered"：结果完全回答了用户问题
- "partial"：结果部分回答了用户问题（如缺少某些字段或筛选条件不完整）
- "not_answered"：结果没有回答用户问题（如查询了错误的数据、字段完全不对）

请严格按以下 JSON 格式输出，不要输出其他内容：
{"status": "answered|partial|not_answered", "reason": "简要说明判断理由"}`

	userPrompt := fmt.Sprintf(`## 用户问题
%s

## 执行的 SQL
%s

## 执行结果（前 10 行）
%s

## 结果列
%s

请判断该结果是否回答了用户问题。`,
		question,
		sql,
		resultSummary,
		strings.Join(execResult.Columns, ", "))

	resp, err := ic.llmProvider.Chat(ctx, systemPrompt, []Message{
		NewTextMessage("user", userPrompt),
	})
	if err != nil {
		logx.Infof("[IntentCheck] LLM call failed: %v", err)
		return &IntentCheckResult{Status: "answered", Reason: "校验失败: " + err.Error()}
	}

	// 解析 JSON 响应
	content := strings.TrimSpace(resp.Content)
	// 兼容 ```json ... ``` 包裹
	if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	var result struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		logx.Infof("[IntentCheck] Parse failed: %v, content: %s", err, content)
		return &IntentCheckResult{Status: "answered", Reason: "校验结果解析失败"}
	}

	// 标准化 status
	status := strings.ToLower(strings.TrimSpace(result.Status))
	switch status {
	case "answered", "partial", "not_answered":
		// ok
	default:
		status = "answered"
	}

	return &IntentCheckResult{
		Status:      status,
		Reason:      result.Reason,
		ShouldRetry: status == "not_answered",
	}
}

// buildResultSummary 构造执行结果摘要（前 N 行）
func buildResultSummary(execResult *ExecResult, maxRows int) string {
	if execResult == nil || len(execResult.Rows) == 0 {
		return "(无数据)"
	}

	var sb strings.Builder
	rowCount := len(execResult.Rows)
	if rowCount > maxRows {
		rowCount = maxRows
	}

	for i := 0; i < rowCount; i++ {
		if i > 0 {
			sb.WriteString("\n")
		}
		row := execResult.Rows[i]
		parts := make([]string, 0, len(execResult.Columns))
		for _, col := range execResult.Columns {
			val, ok := row[col]
			if !ok {
				parts = append(parts, col+"=<null>")
				continue
			}
			parts = append(parts, fmt.Sprintf("%s=%v", col, val))
		}
		sb.WriteString(fmt.Sprintf("行%d: %s", i+1, strings.Join(parts, ", ")))
	}

	if len(execResult.Rows) > maxRows {
		sb.WriteString(fmt.Sprintf("\n... 共 %d 行（仅显示前 %d 行）", execResult.RowCount, maxRows))
	}

	return sb.String()
}
