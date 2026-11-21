package text2sql

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/utils"
)

// SQLValidationResult SQL验证结果
type SQLValidationResult struct {
	IsValid      bool     `json:"is_valid"`
	Confidence   float64  `json:"confidence"`             // 置信度 0-1
	Issues       []Issue  `json:"issues"`                 // 发现的问题
	Suggestions  []string `json:"suggestions"`            // 优化建议
	CorrectedSQL string   `json:"corrected_sql"`          // 修正后的SQL（如有）
	OriginalSQL  string   `json:"original_sql,omitempty"` // 原始SQL
}

// Issue 问题详情
type Issue struct {
	Severity string `json:"severity"` // error, warning, info
	Code     string `json:"code"`     // 错误代码
	Message  string `json:"message"`  // 问题描述
	Location string `json:"location"` // 位置（如有）
}

// SQLSelfValidator SQL自我验证器
// P0增强：生成后由LLM审查并修正SQL
type SQLSelfValidator struct {
	provider provider.Provider
	// 惰性回退 provider：构造时 provider==nil 就落到这里，
	// 拿到后再做 LLM 深度验证（见 validateProvider 的判据）。
	fallback func() provider.Provider
}

// NewSQLSelfValidator 创建SQL自我验证器
func NewSQLSelfValidator(p provider.Provider) *SQLSelfValidator {
	return &SQLSelfValidator{
		provider: p,
		// globalRealProvider 由 app/server 在 AI 模块就绪后注入
		// （SetGlobalRealProvider）；Text2SQL 组件构造早于注入时，
		// 这里按惰性引用而不是值拷贝，避免 nil 被永久固化。
		fallback: func() provider.Provider { return globalRealProvider },
	}
}

// validateProvider 返回真正能用的 provider：构造时注入的优先，
// 没有就回退到惰性 provider。两者都拿不到返回 nil，
// ValidateAndCorrect 据此降级为基础验证。
func (v *SQLSelfValidator) validateProvider() provider.Provider {
	if v.provider != nil {
		return v.provider
	}
	if v.fallback != nil {
		if p := v.fallback(); p != nil {
			return p
		}
	}
	return nil
}

// ValidateAndCorrect 验证并修正SQL
func (v *SQLSelfValidator) ValidateAndCorrect(ctx context.Context, sql, question string, schemaContext string) (*SQLValidationResult, error) {
	p := v.validateProvider()
	if p == nil {
		// 无LLM时使用基础验证
		return v.basicValidation(sql), nil
	}

	// LLM深度验证
	llmResult, err := v.llmValidateWith(p, ctx, sql, question, schemaContext)
	if err != nil {
		logx.Infof("[SQLSelfValidator] LLM validation failed: %v", err)
		// fail-closed：LLM 不可用时降级为基础校验，但下调置信度并标注未经 LLM 验证，
		// 让上层（如 execution_feedback_loop）能区分"验证通过"与"降级放行"。
		basic := v.basicValidation(sql)
		basic.Confidence = 0.4
		basic.Suggestions = append(basic.Suggestions, "LLM 深度校验不可用，仅基础校验通过")
		return basic, nil
	}

	return llmResult, nil
}

// basicValidation 基础语法验证
func (v *SQLSelfValidator) basicValidation(sql string) *SQLValidationResult {
	result := &SQLValidationResult{
		IsValid:      true,
		Confidence:   0.8,
		Issues:       []Issue{},
		Suggestions:  []string{},
		CorrectedSQL: sql,
	}

	// 基础语法检查
	sqlUpper := strings.ToUpper(sql)

	// 检查是否以SELECT/DML开头
	validStarts := []string{"SELECT", "INSERT", "UPDATE", "DELETE", "WITH", "("}
	isValidStart := false
	for _, start := range validStarts {
		if strings.HasPrefix(strings.TrimSpace(sqlUpper), start) {
			isValidStart = true
			break
		}
	}

	if !isValidStart {
		result.IsValid = false
		result.Confidence = 0.0
		result.Issues = append(result.Issues, Issue{
			Severity: "error",
			Code:     "INVALID_START",
			Message:  "SQL必须以SELECT/INSERT/UPDATE/DELETE/WITH开头",
		})
	}

	// 检查括号匹配
	openCount := strings.Count(sql, "(")
	closeCount := strings.Count(sql, ")")
	if openCount != closeCount {
		result.Issues = append(result.Issues, Issue{
			Severity: "error",
			Code:     "MISMATCHED_PARENS",
			Message:  fmt.Sprintf("括号不匹配：%d个'('，%d个')'", openCount, closeCount),
		})
		result.IsValid = false
		result.Confidence = 0.0
	}

	// 检查引号匹配
	singleCount := strings.Count(sql, "'")
	if singleCount%2 != 0 {
		result.Issues = append(result.Issues, Issue{
			Severity: "error",
			Code:     "MISMATCHED_QUOTES",
			Message:  "单引号不匹配",
		})
		result.IsValid = false
		result.Confidence = 0.0
	}

	return result
}

// llmValidateWith 用指定 provider 做 LLM 深度验证。
func (v *SQLSelfValidator) llmValidateWith(p provider.Provider, ctx context.Context, sql, question, schemaContext string) (*SQLValidationResult, error) {
	prompt := "你是SQL质量审查专家。请验证以下SQL的正确性，并给出修正建议。\n\n" +
		"## 用户问题\n" +
		question + "\n\n" +
		"## Schema上下文\n" +
		schemaContext + "\n\n" +
		"## 待验证SQL\n" +
		"```sql\n" +
		sql + "\n" +
		"```\n\n" +
		"## 验证维度\n\n" +
		"请从以下维度验证SQL：\n\n" +
		"1. **语法正确性**：SQL语法是否正确\n" +
		"2. **字段映射**：SELECT中的字段是否存在于表中\n" +
		"3. **表引用**：FROM/JOIN中的表是否正确\n" +
		"4. **JOIN条件**：ON条件是否完整和正确\n" +
		"5. **WHERE条件**：过滤条件是否合理\n" +
		"6. **聚合逻辑**：GROUP BY和聚合函数是否匹配\n" +
		"7. **业务逻辑**：是否符合用户的业务需求\n\n" +
		"## 输出格式\n\n" +
		"请严格按以下JSON格式输出，不要输出其他内容：\n\n" +
		"{\n" +
		"  \"is_valid\": true或false,\n" +
		"  \"confidence\": 0.0到1.0的数字，\n" +
		"  \"issues\": [\n" +
		"    {\n" +
		"      \"severity\": \"error/warning/info\",\n" +
		"      \"code\": \"错误代码\",\n" +
		"      \"message\": \"问题描述\",\n" +
		"      \"location\": \"问题位置（如SELECT子句的第2个字段）\"\n" +
		"    }\n" +
		"  ],\n" +
		"  \"suggestions\": [\"建议1\", \"建议2\"],\n" +
		"  \"corrected_sql\": \"修正后的SQL（如无需修正则与原SQL相同）\"\n" +
		"}\n\n" +
		"请开始验证："

	resp, err := p.Chat(ctx, "你是一个严格的SQL质量审查专家。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(1000), provider.WithTemperature(0.1))

	if err != nil {
		return nil, err
	}

	// 解析JSON结果
	return v.parseValidationResult(resp.Content, sql)
}

// parseValidationResult 解析验证结果
func (v *SQLSelfValidator) parseValidationResult(content, originalSQL string) (*SQLValidationResult, error) {
	result := &SQLValidationResult{
		IsValid:      true,
		Confidence:   1.0,
		Issues:       []Issue{},
		Suggestions:  []string{},
		CorrectedSQL: originalSQL,
		OriginalSQL:  originalSQL,
	}

	// 提取JSON（可能包含在代码块中）
	jsonStr := utils.ExtractJSON(content)
	if jsonStr == "" {
		// 解析失败，使用基础验证
		return v.basicValidation(originalSQL), nil
	}

	var parsed struct {
		IsValid    bool    `json:"is_valid"`
		Confidence float64 `json:"confidence"`
		Issues     []struct {
			Severity string `json:"severity"`
			Code     string `json:"code"`
			Message  string `json:"message"`
			Location string `json:"location"`
		} `json:"issues"`
		Suggestions  []string `json:"suggestions"`
		CorrectedSQL string   `json:"corrected_sql"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		logx.Infof("[SQLSelfValidator] Parse validation result failed: %v", err)
		return v.basicValidation(originalSQL), nil
	}

	result.IsValid = parsed.IsValid
	result.Confidence = parsed.Confidence
	result.CorrectedSQL = parsed.CorrectedSQL

	if result.CorrectedSQL == "" {
		result.CorrectedSQL = originalSQL
	}

	for _, issue := range parsed.Issues {
		result.Issues = append(result.Issues, Issue{
			Severity: issue.Severity,
			Code:     issue.Code,
			Message:  issue.Message,
			Location: issue.Location,
		})
	}

	for _, sug := range parsed.Suggestions {
		result.Suggestions = append(result.Suggestions, sug)
	}

	return result, nil
}

// ValidateStructure 验证SQL结构
func (v *SQLSelfValidator) ValidateStructure(sql string) *SQLValidationResult {
	result := &SQLValidationResult{
		IsValid:      true,
		Confidence:   1.0,
		Issues:       []Issue{},
		Suggestions:  []string{},
		CorrectedSQL: sql,
	}

	sqlUpper := strings.ToUpper(sql)

	// 检查是否包含危险操作
	dangerousKeywords := []string{"DROP", "TRUNCATE", "ALTER", "GRANT", "REVOKE"}
	for _, kw := range dangerousKeywords {
		if strings.Contains(sqlUpper, kw) {
			result.Issues = append(result.Issues, Issue{
				Severity: "warning",
				Code:     "DANGEROUS_OPERATION",
				Message:  fmt.Sprintf("检测到危险操作：%s", kw),
			})
			result.Confidence -= 0.3
		}
	}

	// 检查SELECT完整性
	if strings.Contains(sqlUpper, "SELECT") && !strings.Contains(sqlUpper, "FROM") {
		result.Issues = append(result.Issues, Issue{
			Severity: "error",
			Code:     "MISSING_FROM",
			Message:  "SELECT语句缺少FROM子句",
		})
		result.IsValid = false
	}

	// 检查GROUP BY与聚合
	if strings.Contains(sqlUpper, "GROUP BY") {
		hasAgg := false
		aggFuncs := []string{"COUNT(", "SUM(", "AVG(", "MAX(", "MIN("}
		for _, fn := range aggFuncs {
			if strings.Contains(sqlUpper, fn) {
				hasAgg = true
				break
			}
		}
		if !hasAgg {
			result.Suggestions = append(result.Suggestions, "GROUP BY通常与聚合函数一起使用")
		}
	}

	// 检查JOIN条件
	joinCount := strings.Count(sqlUpper, "JOIN")
	onCount := strings.Count(sqlUpper, " ON ")
	if joinCount > 0 && onCount < joinCount {
		result.Issues = append(result.Issues, Issue{
			Severity: "warning",
			Code:     "MISSING_JOIN_CONDITION",
			Message:  fmt.Sprintf("检测到%d个JOIN但只有%d个ON条件", joinCount, onCount),
		})
		result.Confidence -= 0.2
	}

	return result
}

// QuickCheck 快速检查SQL
func (v *SQLSelfValidator) QuickCheck(sql string) bool {
	// 快速语法检查
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return false
	}

	// 检查基本结构
	hasSelect := strings.Contains(strings.ToUpper(sql), "SELECT")
	hasDML := strings.Contains(strings.ToUpper(sql), "INSERT") ||
		strings.Contains(strings.ToUpper(sql), "UPDATE") ||
		strings.Contains(strings.ToUpper(sql), "DELETE") ||
		strings.Contains(strings.ToUpper(sql), "WITH")

	if !hasSelect && !hasDML {
		return false
	}

	// 检查括号匹配
	openCount := strings.Count(sql, "(")
	closeCount := strings.Count(sql, ")")
	if openCount != closeCount {
		return false
	}

	return true
}

// ConfidenceLevel 置信度等级
type ConfidenceLevel string

const (
	ConfidenceHigh    ConfidenceLevel = "high"     // >= 0.9
	ConfidenceMedium  ConfidenceLevel = "medium"   // >= 0.7
	ConfidenceLow     ConfidenceLevel = "low"      // >= 0.5
	ConfidenceVeryLow ConfidenceLevel = "very_low" // < 0.5
)

// GetConfidenceLevel 获取置信度等级
func (r *SQLValidationResult) GetConfidenceLevel() ConfidenceLevel {
	switch {
	case r.Confidence >= 0.9:
		return ConfidenceHigh
	case r.Confidence >= 0.7:
		return ConfidenceMedium
	case r.Confidence >= 0.5:
		return ConfidenceLow
	default:
		return ConfidenceVeryLow
	}
}

// HasErrors 检查是否有错误级别的问题
func (r *SQLValidationResult) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Severity == "error" {
			return true
		}
	}
	return false
}

// GetErrorCount 获取错误数量
func (r *SQLValidationResult) GetErrorCount() int {
	count := 0
	for _, issue := range r.Issues {
		if issue.Severity == "error" {
			count++
		}
	}
	return count
}

// FormatReport 格式化验证报告
func (r *SQLValidationResult) FormatReport() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("验证结果：%s\n", map[bool]string{true: "通过", false: "不通过"}[r.IsValid]))
	sb.WriteString(fmt.Sprintf("置信度：%.0f%%（%s）\n", r.Confidence*100, r.GetConfidenceLevel()))

	if len(r.Issues) > 0 {
		sb.WriteString("\n发现问题：\n")
		for _, issue := range r.Issues {
			icon := "❌"
			if issue.Severity == "warning" {
				icon = "⚠️"
			} else if issue.Severity == "info" {
				icon = "ℹ️"
			}
			sb.WriteString(fmt.Sprintf("  %s [%s] %s\n", icon, issue.Code, issue.Message))
			if issue.Location != "" {
				sb.WriteString(fmt.Sprintf("     位置：%s\n", issue.Location))
			}
		}
	}

	if len(r.Suggestions) > 0 {
		sb.WriteString("\n优化建议：\n")
		for _, sug := range r.Suggestions {
			sb.WriteString(fmt.Sprintf("  • %s\n", sug))
		}
	}

	if r.CorrectedSQL != "" && r.CorrectedSQL != r.OriginalSQL {
		sb.WriteString(fmt.Sprintf("\n修正后SQL：\n```sql\n%s\n```", r.CorrectedSQL))
	}

	return sb.String()
}
