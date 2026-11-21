package text2sql

import (
	"context"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strings"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/utils"
)

// SubQuery 子查询步骤
type SubQuery struct {
	Step       int         `json:"step"`        // 步骤编号
	SQL        string      `json:"sql"`         // 该步骤的SQL
	Purpose    string      `json:"purpose"`     // 该步骤的目的
	InputFrom  []int       `json:"input_from"`  // 依赖的前置步骤
	OutputAs   string      `json:"output_as"`   // 输出别名
	Validated  bool        `json:"validated"`   // 是否验证通过
	ExecResult *ExecResult `json:"exec_result"` // 执行结果
}

// QueryDecomposer 查询分解器
// P1增强：将复杂问题分解为多个简单步骤
type QueryDecomposer struct {
	provider  provider.Provider
	validator *SQLSelfValidator
	executor  *SQLExecutor
}

// NewQueryDecomposer 创建查询分解器
func NewQueryDecomposer(p provider.Provider, executor *SQLExecutor) *QueryDecomposer {
	return &QueryDecomposer{
		provider:  p,
		validator: NewSQLSelfValidator(p),
		executor:  executor,
	}
}

// DecomposeResult 分解结果
type DecomposeResult struct {
	Steps     []SubQuery `json:"steps"`     // 分解后的步骤
	FinalSQL  string     `json:"final_sql"` // 最终合并的SQL
	Assembled bool       `json:"assembled"` // 是否已合并
	Complex   bool       `json:"complex"`   // 是否被识别为复杂查询
}

// Decompose 将复杂问题分解
func (d *QueryDecomposer) Decompose(ctx context.Context, question string, schemaContext string) (*DecomposeResult, error) {
	// 1. 分析是否需要分解
	needsDecompose, reason := d.analyzeComplexity(question)
	if !needsDecompose {
		return &DecomposeResult{
			Steps:     []SubQuery{},
			Assembled: true,
			Complex:   false,
		}, nil
	}

	// 2. 生成分解计划
	plan, err := d.generatePlan(ctx, question, schemaContext, reason)
	if err != nil {
		logx.Infof("[QueryDecomposer] Generate plan failed: %v", err)
		// 降级：不分解，直接返回
		return &DecomposeResult{
			Steps:     []SubQuery{},
			Assembled: true,
			Complex:   false,
		}, nil
	}

	return plan, nil
}

// analyzeComplexity 分析问题复杂度
func (d *QueryDecomposer) analyzeComplexity(question string) (bool, string) {
	qLower := strings.ToLower(question)

	// 复杂度指标
	complexityScore := 0

	// 多时间维度
	if strings.Contains(qLower, "每月") || strings.Contains(qLower, "每季度") || strings.Contains(qLower, "每年") {
		complexityScore += 2
	}

	// 多聚合维度
	if strings.Contains(qLower, "同比") || strings.Contains(qLower, "环比") {
		complexityScore += 2
	}

	// 嵌套条件
	if strings.Contains(qLower, "同时") || strings.Contains(qLower, "并且") || strings.Contains(qLower, "而且") {
		complexityScore += 1
	}

	// 多表关联
	if strings.Contains(qLower, "和") && strings.Contains(qLower, "及") {
		complexityScore += 2
	}

	// 复杂统计
	if strings.Contains(qLower, "排名") || strings.Contains(qLower, "占比") || strings.Contains(qLower, "累计") {
		complexityScore += 2
	}

	// CTE适用场景
	cteKeywords := []string{"累计", "递归", "多步骤", "中间结果"}
	for _, kw := range cteKeywords {
		if strings.Contains(qLower, kw) {
			complexityScore += 1
		}
	}

	if complexityScore >= 3 {
		return true, fmt.Sprintf("复杂度评分: %d，需要分解处理", complexityScore)
	}

	return false, ""
}

// generatePlan 生成分解计划
func (d *QueryDecomposer) generatePlan(ctx context.Context, question string, schemaContext string, reason string) (*DecomposeResult, error) {
	prompt := "请将以下复杂问题分解为多个简单的SQL步骤。\n\n" +
		"## 问题\n" +
		question + "\n\n" +
		"## 原因\n" +
		reason + "\n\n" +
		"## Schema\n" +
		schemaContext + "\n\n" +
		"## 任务\n" +
		"1. 分析问题的子任务\n" +
		"2. 将问题分解为2-5个简单的SQL步骤\n" +
		"3. 每步骤应该能独立执行并产生有意义的结果\n" +
		"4. 最终通过CTE或子查询组合所有步骤\n\n" +
		"## 输出格式（严格JSON）：\n" +
		"{\n" +
		"  \"steps\": [\n" +
		"    {\n" +
		"      \"step\": 1,\n" +
		"      \"purpose\": \"该步骤的目的描述\",\n" +
		"      \"sql\": \"该步骤的SQL\",\n" +
		"      \"output_as\": \"输出结果使用的别名\",\n" +
		"      \"input_from\": []\n" +
		"    },\n" +
		"    ...\n" +
		"  ],\n" +
		"  \"final_sql\": \"最终合并的完整SQL\"\n" +
		"}\n\n" +
		"注意事项：\n" +
		"- 每个步骤的SQL应该尽可能简单\n" +
		"- 步骤之间有依赖关系时，later步骤的input_from应包含earlier步骤的编号\n" +
		"- 最终SQL应该使用WITH子句（CTE）组合所有步骤"

	resp, err := d.provider.Chat(ctx, "你是一个SQL分解专家。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(1500), provider.WithTemperature(0.2))

	if err != nil {
		return nil, err
	}

	return d.parsePlan(resp.Content)
}

// parsePlan 解析分解计划
func (d *QueryDecomposer) parsePlan(content string) (*DecomposeResult, error) {
	result := &DecomposeResult{
		Steps:     []SubQuery{},
		Assembled: false,
		Complex:   true,
	}

	// 提取JSON
	jsonStr := utils.ExtractJSON(content)
	if jsonStr == "" {
		logx.Infof("[QueryDecomposer] Failed to extract JSON from response")
		return result, nil
	}

	var parsed struct {
		Steps []struct {
			Step      int    `json:"step"`
			Purpose   string `json:"purpose"`
			SQL       string `json:"sql"`
			OutputAs  string `json:"output_as"`
			InputFrom []int  `json:"input_from"`
		} `json:"steps"`
		FinalSQL string `json:"final_sql"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		logx.Infof("[QueryDecomposer] Parse plan failed: %v", err)
		return result, nil
	}

	for _, s := range parsed.Steps {
		result.Steps = append(result.Steps, SubQuery{
			Step:      s.Step,
			Purpose:   s.Purpose,
			SQL:       s.SQL,
			OutputAs:  s.OutputAs,
			InputFrom: s.InputFrom,
			Validated: true,
		})
	}

	result.FinalSQL = parsed.FinalSQL
	result.Assembled = parsed.FinalSQL != ""

	return result, nil
}

// ValidateSteps 验证每个步骤
func (d *QueryDecomposer) ValidateSteps(ctx context.Context, result *DecomposeResult, question string, schemaContext string) *DecomposeResult {
	for i := range result.Steps {
		// 结构验证
		validation := d.validator.ValidateStructure(result.Steps[i].SQL)
		result.Steps[i].Validated = validation.IsValid
	}

	return result
}

// BuildFinalSQL 构建最终SQL
func (d *QueryDecomposer) BuildFinalSQL(result *DecomposeResult) string {
	if len(result.Steps) == 0 {
		return ""
	}

	// 如果已有最终SQL，直接返回
	if result.FinalSQL != "" {
		return result.FinalSQL
	}

	// 构建WITH子句
	var sb strings.Builder
	sb.WriteString("WITH ")

	for i, step := range result.Steps {
		if i > 0 {
			sb.WriteString(", ")
		}
		outputName := step.OutputAs
		if outputName == "" {
			outputName = fmt.Sprintf("step_%d", step.Step)
		}
		sb.WriteString(outputName)
		sb.WriteString(" AS (")
		sb.WriteString(step.SQL)
		sb.WriteString(")")
	}

	sb.WriteString("\nSELECT * FROM ")
	sb.WriteString(result.Steps[len(result.Steps)-1].OutputAs)
	if result.Steps[len(result.Steps)-1].OutputAs == "" {
		sb.WriteString(fmt.Sprintf("step_%d", len(result.Steps)))
	}

	result.FinalSQL = sb.String()
	result.Assembled = true

	return result.FinalSQL
}

// FormatDecomposeReport 格式化分解报告
func (d *QueryDecomposer) FormatDecomposeReport(result *DecomposeResult) string {
	var sb strings.Builder

	if !result.Complex {
		sb.WriteString("查询复杂度较低，无需分解\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("查询已分解为 %d 个步骤：\n\n", len(result.Steps)))

	for i, step := range result.Steps {
		status := "✅"
		if !step.Validated {
			status = "❌"
		}

		sb.WriteString(fmt.Sprintf("### 步骤 %d %s\n", i+1, status))
		sb.WriteString(fmt.Sprintf("目的：%s\n", step.Purpose))

		if step.InputFrom != nil && len(step.InputFrom) > 0 {
			sb.WriteString(fmt.Sprintf("依赖步骤：%v\n", step.InputFrom))
		}

		sb.WriteString(fmt.Sprintf("SQL：\n```sql\n%s\n```\n", step.SQL))

		if step.ExecResult != nil {
			if step.ExecResult.Error != "" {
				sb.WriteString(fmt.Sprintf("执行结果：❌ %s\n", step.ExecResult.Error))
			} else {
				sb.WriteString(fmt.Sprintf("执行结果：✅ %d 行\n", step.ExecResult.RowCount))
			}
		}

		sb.WriteString("\n")
	}

	if result.FinalSQL != "" {
		sb.WriteString("### 最终合并SQL\n")
		sb.WriteString(fmt.Sprintf("```sql\n%s\n```\n", result.FinalSQL))
	}

	return sb.String()
}

// DetermineStrategy 确定最佳策略
// 返回：是否使用分解策略，以及使用哪种分解策略
func (d *QueryDecomposer) DetermineStrategy(question string) (bool, string) {
	qLower := strings.ToLower(question)

	// 明确需要分解的场景
	needDecompose := false
	strategy := "single" // single, cte, stepwise, union

	// 多层嵌套
	if strings.Contains(qLower, "每个") && strings.Contains(qLower, "的") {
		needDecompose = true
		strategy = "cte"
	}

	// 累计计算
	if strings.Contains(qLower, "累计") || strings.Contains(qLower, "running") {
		needDecompose = true
		strategy = "cte"
	}

	// 同比环比
	if strings.Contains(qLower, "同比") || strings.Contains(qLower, "环比") {
		needDecompose = true
		strategy = "stepwise"
	}

	// 多维度分析
	if strings.Contains(qLower, "每月") && strings.Contains(qLower, "每个") {
		needDecompose = true
		strategy = "stepwise"
	}

	// 排名topN
	if strings.Contains(qLower, "排名") || strings.Contains(qLower, "top") {
		needDecompose = true
		strategy = "cte"
	}

	return needDecompose, strategy
}

// OptimizeWithStrategy 根据策略优化SQL
func (d *QueryDecomposer) OptimizeWithStrategy(ctx context.Context, sql, question, strategy, schemaContext string) (string, error) {
	if strategy == "single" {
		return sql, nil
	}

	prompt := "请将以下SQL使用" + strategy + "策略进行优化。\n\n" +
		"## 问题\n" +
		question + "\n\n" +
		"## 当前SQL\n" +
		"```sql\n" +
		sql + "\n" +
		"```\n\n" +
		"## Schema\n" +
		schemaContext + "\n\n" +
		"## 策略说明\n" +
		"- cte: 使用WITH子句组织复杂逻辑\n" +
		"- stepwise: 分步骤构建，每步验证\n" +
		"- union: 使用UNION组合多个查询\n\n" +
		"## 任务\n" +
		"根据指定策略优化SQL，输出优化后的版本\n\n" +
		"## 输出格式\n" +
		"{\n" +
		"  \"optimized_sql\": \"优化后的SQL\",\n" +
		"  \"explanation\": \"优化说明\"\n" +
		"}"

	resp, err := d.provider.Chat(ctx, "你是一个SQL优化专家。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(800), provider.WithTemperature(0.1))

	if err != nil {
		return sql, err
	}

	// 提取SQL
	optimizedSQL := extractSQLFromReply(resp.Content)
	if optimizedSQL == "" {
		return sql, nil
	}

	return optimizedSQL, nil
}

// extractSQL 从响应中提取SQL
