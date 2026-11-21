package text2sql

import (
	"context"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"regexp"
	"strings"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/utils"
)

// ExecutionPlan 执行计划分析
type ExecutionPlan struct {
	SQL           string      `json:"sql"`
	ExplainOutput string      `json:"explain_output"`
	Issues        []PlanIssue `json:"issues"`
	Suggestions   []string    `json:"suggestions"`
	OverallScore  float64     `json:"overall_score"`  // 0-100
	EstimatedCost string      `json:"estimated_cost"` // 低/中/高
}

// PlanIssue 执行计划问题
type PlanIssue struct {
	Severity   string `json:"severity"`   // critical, warning, info
	Type       string `json:"type"`       // full_scan, no_index, etc.
	Message    string `json:"message"`    // 问题描述
	Location   string `json:"location"`   // 问题位置
	Suggestion string `json:"suggestion"` // 修复建议
}

// ExplainAnalyzer 执行计划分析器
// P2增强：解析EXPLAIN输出，提供优化建议
type ExplainAnalyzer struct {
	provider provider.Provider
	executor *SQLExecutor
	dbType   string // mysql, postgres, etc.
}

// NewExplainAnalyzer 创建执行计划分析器
func NewExplainAnalyzer(p provider.Provider, executor *SQLExecutor, dbType string) *ExplainAnalyzer {
	if dbType == "" {
		dbType = "mysql"
	}
	return &ExplainAnalyzer{
		provider: p,
		executor: executor,
		dbType:   dbType,
	}
}

// Analyze 分析SQL执行计划（使用LLM静态分析）
func (a *ExplainAnalyzer) Analyze(ctx context.Context, sql string) (*ExecutionPlan, error) {
	// 使用LLM静态分析
	return a.llmAnalyze(ctx, sql, "")
}

// buildExplainQuery 构建EXPLAIN查询
func (a *ExplainAnalyzer) buildExplainQuery(sql string) string {
	switch a.dbType {
	case "mysql":
		return "EXPLAIN " + sql
	case "postgres":
		return "EXPLAIN (ANALYZE, BUFFERS) " + sql
	case "sqlite":
		return "EXPLAIN QUERY PLAN " + sql
	default:
		return "EXPLAIN " + sql
	}
}

// parseExplainOutput 解析EXPLAIN输出
func (a *ExplainAnalyzer) parseExplainOutput(plan *ExecutionPlan) {
	output := plan.ExplainOutput
	if output == "" {
		return
	}

	// MySQL EXPLAIN 解析
	if a.dbType == "mysql" {
		a.parseMySQLExplain(plan, output)
	}

	// PostgreSQL EXPLAIN 解析
	if a.dbType == "postgres" {
		a.parsePostgresExplain(plan, output)
	}
}

// parseMySQLExplain 解析MySQL EXPLAIN
func (a *ExplainAnalyzer) parseMySQLExplain(plan *ExecutionPlan, output string) {
	// 全表扫描检测
	if strings.Contains(output, "type: ALL") {
		plan.Issues = append(plan.Issues, PlanIssue{
			Severity:   "critical",
			Type:       "full_table_scan",
			Message:    "检测到全表扫描（type: ALL）",
			Suggestion: "添加WHERE条件索引列，或创建合适索引",
		})
		plan.OverallScore -= 30
	}

	// Using filesort 检测
	if strings.Contains(output, "Using filesort") {
		plan.Issues = append(plan.Issues, PlanIssue{
			Severity:   "warning",
			Type:       "filesort",
			Message:    "检测到文件排序（Using filesort）",
			Suggestion: "在ORDER BY列上添加索引可以消除filesort",
		})
		plan.OverallScore -= 15
	}

	// Using temporary 检测
	if strings.Contains(output, "Using temporary") {
		plan.Issues = append(plan.Issues, PlanIssue{
			Severity:   "warning",
			Type:       "temporary_table",
			Message:    "使用了临时表（Using temporary）",
			Suggestion: "检查GROUP BY和DISTINCT，考虑优化查询",
		})
		plan.OverallScore -= 10
	}

	// Using join buffer 检测
	if strings.Contains(output, "Using join buffer") {
		plan.Issues = append(plan.Issues, PlanIssue{
			Severity:   "warning",
			Type:       "join_buffer",
			Message:    "使用了连接缓冲（Using join buffer）",
			Suggestion: "在JOIN列上添加索引可以避免使用缓冲",
		})
		plan.OverallScore -= 10
	}

	// rows > 1000 检测（可能的性能问题）
	rowsPattern := regexp.MustCompile(`rows:\s*(\d+)`)
	matches := rowsPattern.FindAllStringSubmatch(output, -1)
	for _, match := range matches {
		if len(match) > 1 {
			var rows int
			fmt.Sscanf(match[1], "%d", &rows)
			if rows > 10000 {
				plan.Issues = append(plan.Issues, PlanIssue{
					Severity:   "warning",
					Type:       "large_rows",
					Message:    fmt.Sprintf("扫描了 %d 行数据", rows),
					Suggestion: "添加更精确的过滤条件减少扫描行数",
				})
				plan.OverallScore -= 5
			}
		}
	}

	// 设置成本评估
	if plan.OverallScore >= 80 {
		plan.EstimatedCost = "低"
	} else if plan.OverallScore >= 50 {
		plan.EstimatedCost = "中"
	} else {
		plan.EstimatedCost = "高"
	}
}

// parsePostgresExplain 解析PostgreSQL EXPLAIN
func (a *ExplainAnalyzer) parsePostgresExplain(plan *ExecutionPlan, output string) {
	// Seq Scan 检测
	if strings.Contains(output, "Seq Scan") {
		plan.Issues = append(plan.Issues, PlanIssue{
			Severity:   "critical",
			Type:       "sequential_scan",
			Message:    "检测到顺序扫描（Seq Scan）",
			Suggestion: "在过滤列上创建索引",
		})
		plan.OverallScore -= 30
	}

	// Hash Join 检测
	if strings.Contains(output, "Hash Join") {
		plan.Suggestions = append(plan.Suggestions, "Hash Join可能占用大量内存，考虑使用NestLoop或Merge Join")
		plan.OverallScore -= 5
	}

	// Sort 检测
	if strings.Contains(output, "Sort") {
		plan.Suggestions = append(plan.Suggestions, "Sort操作可能较慢，在排序列上创建索引可以消除排序")
		plan.OverallScore -= 5
	}

	// Nested Loop 检测
	if strings.Contains(output, "Nested Loop") {
		plan.Suggestions = append(plan.Suggestions, "Nested Loop对大数据集可能较慢，确保内表有索引")
	}

	// 设置成本评估
	if plan.OverallScore >= 80 {
		plan.EstimatedCost = "低"
	} else if plan.OverallScore >= 50 {
		plan.EstimatedCost = "中"
	} else {
		plan.EstimatedCost = "高"
	}
}

// llmAnalyze 使用LLM深度分析
func (a *ExplainAnalyzer) llmAnalyze(ctx context.Context, sql, explainOutput string) (*ExecutionPlan, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("LLM provider not available")
	}

	prompt := "你是数据库性能优化专家。请分析以下SQL的执行计划并提供优化建议。\n\n" +
		"## 待分析SQL\n" + sql + "\n\n" +
		"## EXPLAIN输出\n" + explainOutput + "\n\n" +
		"## 任务\n1. 分析执行计划\n2. 识别性能问题\n3. 提供优化建议\n\n" +
		"## 输出格式（严格JSON）\n" +
		`{"issues": [{"severity": "critical/warning/info", "type": "问题类型", "message": "问题描述", "suggestion": "优化建议"}], "suggestions": ["优化建议1"], "overall_score": 0到100的数字, "estimated_cost": "低/中/高"}`

	resp, err := a.provider.Chat(ctx, "你是一个数据库性能优化专家。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(800), provider.WithTemperature(0.1))

	if err != nil {
		return nil, err
	}

	return a.parseLLMAnalysis(resp.Content)
}

// parseLLMAnalysis 解析LLM分析结果
func (a *ExplainAnalyzer) parseLLMAnalysis(content string) (*ExecutionPlan, error) {
	result := &ExecutionPlan{
		Issues:       []PlanIssue{},
		Suggestions:  []string{},
		OverallScore: 100,
	}

	// 提取JSON
	jsonStr := utils.ExtractJSON(content)
	if jsonStr == "" {
		logx.Infof("[ExplainAnalyzer] Failed to extract JSON from LLM response")
		return result, nil
	}

	var parsed struct {
		Issues []struct {
			Severity   string `json:"severity"`
			Type       string `json:"type"`
			Message    string `json:"message"`
			Suggestion string `json:"suggestion"`
		} `json:"issues"`
		Suggestions   []string `json:"suggestions"`
		OverallScore  float64  `json:"overall_score"`
		EstimatedCost string   `json:"estimated_cost"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		logx.Infof("[ExplainAnalyzer] Parse LLM analysis failed: %v", err)
		return result, nil
	}

	for _, issue := range parsed.Issues {
		result.Issues = append(result.Issues, PlanIssue{
			Severity:   issue.Severity,
			Type:       issue.Type,
			Message:    issue.Message,
			Suggestion: issue.Suggestion,
		})
	}

	for _, sug := range parsed.Suggestions {
		result.Suggestions = append(result.Suggestions, sug)
	}

	if parsed.OverallScore > 0 {
		result.OverallScore = parsed.OverallScore
	}

	if parsed.EstimatedCost != "" {
		result.EstimatedCost = parsed.EstimatedCost
	}

	return result, nil
}

// FormatReport 格式化分析报告
func (p *ExecutionPlan) FormatReport() string {
	var sb strings.Builder

	sb.WriteString("## SQL执行计划分析\n\n")

	// 评分
	scoreIcon := "🟢"
	if p.OverallScore < 80 {
		scoreIcon = "🟡"
	}
	if p.OverallScore < 50 {
		scoreIcon = "🔴"
	}

	sb.WriteString(fmt.Sprintf("### 性能评分：%s %.0f/100\n", scoreIcon, p.OverallScore))
	sb.WriteString(fmt.Sprintf("### 预估成本：%s\n\n", p.EstimatedCost))

	// 问题列表
	if len(p.Issues) > 0 {
		sb.WriteString("### 发现的问题\n\n")
		for _, issue := range p.Issues {
			icon := "ℹ️"
			if issue.Severity == "critical" {
				icon = "🔴"
			} else if issue.Severity == "warning" {
				icon = "🟡"
			}

			sb.WriteString(fmt.Sprintf("%s **[%s]** %s\n", icon, strings.ToUpper(issue.Severity), issue.Message))
			if issue.Suggestion != "" {
				sb.WriteString(fmt.Sprintf("   💡 建议：%s\n", issue.Suggestion))
			}
			sb.WriteString("\n")
		}
	}

	// 优化建议
	if len(p.Suggestions) > 0 {
		sb.WriteString("### 优化建议\n\n")
		for i, sug := range p.Suggestions {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, sug))
		}
	}

	// EXPLAIN输出
	if p.ExplainOutput != "" {
		sb.WriteString("\n### EXPLAIN输出\n\n")
		sb.WriteString("```\n" + p.ExplainOutput + "```\n")
	}

	return sb.String()
}

// GenerateIndexSuggestions 生成索引建议
func (a *ExplainAnalyzer) GenerateIndexSuggestions(plan *ExecutionPlan, sql string) []IndexSuggestion {
	suggestions := []IndexSuggestion{}

	// 基于问题生成索引建议
	for _, issue := range plan.Issues {
		switch issue.Type {
		case "full_table_scan", "sequential_scan":
			suggestions = append(suggestions, IndexSuggestion{
				Columns:  a.extractWhereColumns(sql),
				Reason:   "消除全表扫描",
				Priority: "high",
			})
		case "filesort":
			suggestions = append(suggestions, IndexSuggestion{
				Columns:  a.extractOrderByColumns(sql),
				Reason:   "消除文件排序",
				Priority: "medium",
			})
		case "join_buffer":
			suggestions = append(suggestions, IndexSuggestion{
				Columns:  a.extractJoinColumns(sql),
				Reason:   "优化连接性能",
				Priority: "medium",
			})
		}
	}

	return suggestions
}

// IndexSuggestion 索引建议
type IndexSuggestion struct {
	Columns  []string `json:"columns"`
	Reason   string   `json:"reason"`
	Priority string   `json:"priority"` // high, medium, low
}

// extractWhereColumns 提取WHERE列
func (a *ExplainAnalyzer) extractWhereColumns(sql string) []string {
	// 简单的正则提取，实际使用需要更复杂的SQL解析
	wherePattern := regexp.MustCompile(`WHERE\s+(\w+)\s*`)
	matches := wherePattern.FindAllStringSubmatch(sql, -1)

	var columns []string
	for _, match := range matches {
		if len(match) > 1 {
			columns = append(columns, match[1])
		}
	}

	return columns
}

// extractOrderByColumns 提取ORDER BY列
func (a *ExplainAnalyzer) extractOrderByColumns(sql string) []string {
	orderPattern := regexp.MustCompile(`ORDER\s+BY\s+([\w,\s]+)`)
	matches := orderPattern.FindAllStringSubmatch(sql, -1)

	var columns []string
	for _, match := range matches {
		if len(match) > 1 {
			parts := strings.Split(match[1], ",")
			for _, p := range parts {
				col := strings.TrimSpace(p)
				// 移除ASC/DESC和表别名
				col = strings.TrimSpace(strings.Split(col, ".")[len(strings.Split(col, "."))-1])
				col = strings.TrimSpace(strings.Split(col, " ")[0])
				if col != "" {
					columns = append(columns, col)
				}
			}
		}
	}

	return columns
}

// extractJoinColumns 提取JOIN列
func (a *ExplainAnalyzer) extractJoinColumns(sql string) []string {
	joinPattern := regexp.MustCompile(`JOIN\s+\w+\s+ON\s+[\w.]+\s*=\s*([\w.]+)`)
	matches := joinPattern.FindAllStringSubmatch(sql, -1)

	var columns []string
	for _, match := range matches {
		if len(match) > 1 {
			col := strings.TrimSpace(strings.Split(match[1], ".")[len(strings.Split(match[1], "."))-1])
			if col != "" {
				columns = append(columns, col)
			}
		}
	}

	return columns
}

// FormatIndexSuggestions 格式化索引建议
func FormatIndexSuggestions(suggestions []IndexSuggestion, dbType string) string {
	if len(suggestions) == 0 {
		return "暂无索引建议"
	}

	var sb strings.Builder
	sb.WriteString("### 索引建议\n\n")

	for i, sug := range suggestions {
		priorityIcon := "🔴"
		if sug.Priority == "medium" {
			priorityIcon = "🟡"
		} else if sug.Priority == "low" {
			priorityIcon = "🟢"
		}

		sb.WriteString(fmt.Sprintf("%d. %s **优先级：%s**\n", i+1, priorityIcon, sug.Priority))
		sb.WriteString(fmt.Sprintf("   原因：%s\n", sug.Reason))

		if len(sug.Columns) > 0 {
			colList := strings.Join(sug.Columns, ", ")
			switch dbType {
			case "mysql":
				sb.WriteString(fmt.Sprintf("   建议索引：`CREATE INDEX idx ON table (%s);`\n", colList))
			case "postgres":
				sb.WriteString(fmt.Sprintf("   建议索引：`CREATE INDEX idx ON table (%s);`\n", colList))
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}
