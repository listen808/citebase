package text2sql

import (
	"regexp"
	"strconv"
	"strings"
)

var limitRegex = regexp.MustCompile(`(?:前|top\s*|limit\s*)(\d+)`)

// SQLTemplate SQL 模板
type SQLTemplate struct {
	ID          string
	Name        string
	Intent      string   // 匹配的意图类型
	Keywords    []string // 触发关键词
	Template    string   // SQL 模板（含占位符）
	Description string
}

// TemplateParams 模板参数
type TemplateParams struct {
	Intent       string
	MetricCode   string // 语义指标代码
	Tables       []string
	SelectFields []string
	GroupBy      []string
	OrderBy      []string
	Where        []string
	Aggregate    string
	Limit        int
	JoinType     string // "LEFT", "INNER"
}

// TemplateMatchResult 模板匹配结果
type TemplateMatchResult struct {
	Template *SQLTemplate
	Score    float64
}

// SQLTemplateLibrary SQL 模板库
type SQLTemplateLibrary struct {
	templates []*SQLTemplate
}

// NewSQLTemplateLibrary 创建 SQL 模板库
func NewSQLTemplateLibrary() *SQLTemplateLibrary {
	lib := &SQLTemplateLibrary{
		templates: make([]*SQLTemplate, 0),
	}
	lib.registerBuiltinTemplates()
	return lib
}

// registerBuiltinTemplates 注册内置模板
func (l *SQLTemplateLibrary) registerBuiltinTemplates() {
	l.templates = append(l.templates, &SQLTemplate{
		ID:       "agg_group_by",
		Name:     "统计聚合",
		Intent:   "aggregation",
		Keywords: []string{"数量", "统计", "多少", "几个", "总数", "合计", "平均", "最大", "最小"},
		Template: `SELECT {{select_fields}}
FROM {{table}}
{{where}}
GROUP BY {{group_by}}`,
		Description: "按分组统计聚合",
	})

	l.templates = append(l.templates, &SQLTemplate{
		ID:       "top_n",
		Name:     "排名TopN",
		Intent:   "ranking",
		Keywords: []string{"最高", "最多", "最大", "最少", "最低", "排名", "前", "top"},
		Template: `SELECT {{select_fields}}
FROM {{table}}
{{where}}
ORDER BY {{order_by}}
LIMIT {{limit}}`,
		Description: "按指标排名取TopN",
	})

	l.templates = append(l.templates, &SQLTemplate{
		ID:       "trend",
		Name:     "趋势分析",
		Intent:   "trend",
		Keywords: []string{"趋势", "按月", "按年", "按天", "按周", "变化", "走势", "每月", "每天"},
		Template: `SELECT {{select_fields}}
FROM {{table}}
{{where}}
GROUP BY {{group_by}}
ORDER BY {{order_by}}`,
		Description: "按时间维度统计趋势",
	})

	l.templates = append(l.templates, &SQLTemplate{
		ID:       "filter",
		Name:     "条件筛选",
		Intent:   "filter",
		Keywords: []string{"筛选", "过滤", "条件", "满足", "大于", "小于", "等于", "包含"},
		Template: `SELECT {{select_fields}}
FROM {{table}}
{{where}}`,
		Description: "条件筛选查询",
	})

	l.templates = append(l.templates, &SQLTemplate{
		ID:       "multi_join",
		Name:     "多表关联",
		Intent:   "join",
		Keywords: []string{"关联", "每个", "各自", "对应", "所属"},
		Template: `SELECT {{select_fields}}
FROM {{table1}} {{alias1}}
{{join_type}} JOIN {{table2}} {{alias2}} ON {{join_condition}}
{{where}}
GROUP BY {{group_by}}`,
		Description: "多表关联查询",
	})

	l.templates = append(l.templates, &SQLTemplate{
		ID:       "compare",
		Name:     "对比分析",
		Intent:   "comparison",
		Keywords: []string{"对比", "比较", "vs", "去年", "今年", "同比", "环比"},
		Template: `SELECT {{select_fields}},
CASE WHEN {{condition}} THEN {{then_value}} ELSE {{else_value}} END AS {{alias}}
FROM {{table}}
{{where}}`,
		Description: "对比分析查询",
	})

	l.templates = append(l.templates, &SQLTemplate{
		ID:       "metric",
		Name:     "语义指标",
		Intent:   "metric",
		Keywords: []string{"指标", "KPI", "度量"},
		Template: `SELECT {{select_fields}}
FROM {{table}}
{{where}}
GROUP BY {{group_by}}`,
		Description: "语义指标查询",
	})
}

// MatchTemplate 匹配最佳模板
func (l *SQLTemplateLibrary) MatchTemplate(params TemplateParams) *TemplateMatchResult {
	if params.Intent == "" && len(params.Tables) == 0 {
		return nil
	}

	var bestMatch *TemplateMatchResult
	bestScore := 0.0

	for _, tpl := range l.templates {
		score := l.scoreTemplate(tpl, params)
		if score > bestScore && score > 0.3 {
			bestScore = score
			bestMatch = &TemplateMatchResult{
				Template: tpl,
				Score:    score,
			}
		}
	}

	return bestMatch
}

// scoreTemplate 计算模板匹配分数
func (l *SQLTemplateLibrary) scoreTemplate(tpl *SQLTemplate, params TemplateParams) float64 {
	score := 0.0

	// 意图匹配（权重最高）
	if params.Intent != "" && params.Intent == tpl.Intent {
		score += 0.5
	}

	// 关键词匹配
	if params.Intent == "" {
		// 如果没有明确意图，通过关键词推断
		// 这里简化处理，实际使用时由调用方传入关键词
		score += 0.1
	}

	// 参数完整度
	if len(params.Tables) > 0 {
		score += 0.1
	}
	if len(params.SelectFields) > 0 {
		score += 0.1
	}
	if tpl.Intent == "aggregation" && params.Aggregate != "" {
		score += 0.2
	}
	if tpl.Intent == "ranking" && len(params.OrderBy) > 0 && params.Limit > 0 {
		score += 0.2
	}
	if tpl.Intent == "trend" && len(params.GroupBy) > 0 {
		score += 0.1
	}
	// metric 意图：指标代码存在时给高分
	if tpl.Intent == "metric" && params.MetricCode != "" {
		score += 0.4
	}

	return score
}

// GetTemplate 按 ID 获取模板
func (l *SQLTemplateLibrary) GetTemplate(id string) *SQLTemplate {
	for _, tpl := range l.templates {
		if tpl.ID == id {
			return tpl
		}
	}
	return nil
}

// FillTemplate 填充模板参数生成 SQL
func (l *SQLTemplateLibrary) FillTemplate(tpl *SQLTemplate, params TemplateParams) string {
	sql := tpl.Template

	// 替换表名
	if len(params.Tables) > 0 {
		sql = strings.ReplaceAll(sql, "{{table}}", params.Tables[0])
		sql = strings.ReplaceAll(sql, "{{table1}}", params.Tables[0])
		if len(params.Tables) > 1 {
			sql = strings.ReplaceAll(sql, "{{table2}}", params.Tables[1])
		}
	}

	// 替换别名
	if len(params.Tables) > 0 {
		sql = strings.ReplaceAll(sql, "{{alias1}}", string(params.Tables[0][0]))
		if len(params.Tables) > 1 {
			sql = strings.ReplaceAll(sql, "{{alias2}}", string(params.Tables[1][0]))
		}
	}

	// 替换 SELECT 字段
	if len(params.SelectFields) > 0 {
		sql = strings.ReplaceAll(sql, "{{select_fields}}", strings.Join(params.SelectFields, ", "))
	} else {
		sql = strings.ReplaceAll(sql, "{{select_fields}}", "*")
	}

	// 替换 GROUP BY
	if len(params.GroupBy) > 0 {
		sql = strings.ReplaceAll(sql, "{{group_by}}", strings.Join(params.GroupBy, ", "))
	}

	// 替换 ORDER BY
	if len(params.OrderBy) > 0 {
		sql = strings.ReplaceAll(sql, "{{order_by}}", strings.Join(params.OrderBy, ", "))
	}

	// 替换 WHERE
	if len(params.Where) > 0 {
		sql = strings.ReplaceAll(sql, "{{where}}", "WHERE "+strings.Join(params.Where, " AND "))
	} else {
		sql = strings.ReplaceAll(sql, "{{where}}", "")
	}

	// 替换聚合
	if params.Aggregate != "" {
		sql = strings.ReplaceAll(sql, "{{aggregate}}", params.Aggregate)
	}

	// 替换 LIMIT
	if params.Limit > 0 {
		sql = strings.ReplaceAll(sql, "{{limit}}", strconv.Itoa(params.Limit))
	}

	// 替换 JOIN 类型
	if params.JoinType != "" {
		sql = strings.ReplaceAll(sql, "{{join_type}}", params.JoinType)
	} else {
		sql = strings.ReplaceAll(sql, "{{join_type}}", "LEFT")
	}

	// 清理多余空行
	lines := strings.Split(sql, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" || len(cleaned) > 0 && cleaned[len(cleaned)-1] != "" {
			cleaned = append(cleaned, line)
		}
	}

	// P3：使用 cleaned 而非原始 sql，真正清理多余空行
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

// ApplyTemplate 尝试用模板生成 SQL；无法填充关键参数时返回空
func (l *SQLTemplateLibrary) ApplyTemplate(text string, intent string, tables []string, fields []string, kb *KnowledgeBase) (string, float64) {
	if len(tables) == 0 {
		return "", 0
	}

	params := TemplateParams{
		Intent: intent,
		Tables: tables,
	}

	switch intent {
	case "aggregation":
		if len(fields) == 0 {
			return "", 0
		}
		params.SelectFields = []string{fields[0] + ", COUNT(*) AS cnt"}
		params.GroupBy = []string{fields[0]}
	case "ranking":
		if len(fields) == 0 {
			return "", 0
		}
		params.SelectFields = fields
		params.OrderBy = []string{fields[0] + " DESC"}
		params.Limit = 10
	case "filter":
		if len(fields) == 0 {
			params.SelectFields = []string{"*"}
		} else {
			params.SelectFields = fields
		}
	case "trend":
		if len(fields) == 0 {
			return "", 0
		}
		params.SelectFields = []string{"DATE_FORMAT(" + fields[0] + ", '%Y-%m') AS period, COUNT(*) AS cnt"}
		params.GroupBy = []string{"period"}
		params.OrderBy = []string{"period ASC"}
	case "metric":
		// 命中语义指标：params.MetricCode 和 params.SelectFields/GroupBy 应该由调用方在检测到 metric 意图后预先设置
		// ApplyTemplate 直接使用预设的参数进行模板匹配
		if params.MetricCode == "" || len(params.SelectFields) == 0 {
			return "", 0
		}
	default:
		return "", 0
	}

	match := l.MatchTemplate(params)
	if match == nil || match.Score < 0.7 {
		return "", 0
	}

	if limit := extractLimitFromText(text); limit > 0 {
		params.Limit = limit
	}

	sql := l.FillTemplate(match.Template, params)
	if sql == "" || strings.Contains(sql, "{{") {
		return "", 0
	}
	return sql, match.Score
}

// extractLimitFromText 从查询文本中提取数量限制
func extractLimitFromText(text string) int {
	matches := limitRegex.FindStringSubmatch(text)
	if len(matches) < 2 {
		return 0
	}
	n, err := strconv.Atoi(matches[1])
	if err != nil || n <= 0 {
		return 0
	}
	return n
}
