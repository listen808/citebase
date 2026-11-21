package text2sql

import (
	"context"
	"fmt"
	"strings"
	"sync"

	modelai "github.com/listen808/citebase/internal/model/ai"
)

// CandidateSQL 候选SQL
type CandidateSQL struct {
	SQL        string      `json:"sql"`
	Confidence float64     `json:"confidence"`
	Reason     string      `json:"reason"`
	Strategy   string      `json:"strategy"`    // 生成策略
	Valid      bool        `json:"valid"`       // 是否通过验证
	ExecResult *ExecResult `json:"exec_result"` // 执行结果
	Rank       int         `json:"rank"`        // 最终排名
}

// MultiCandidateGenerator 多候选SQL生成器
// P1增强：生成多个候选SQL，并行验证，选择最优
type MultiCandidateGenerator struct {
	provider      LLMProvider
	validator     *SQLSelfValidator
	executor      *SQLExecutor
	maxCandidates int
}

// NewMultiCandidateGenerator 创建多候选生成器
func NewMultiCandidateGenerator(p LLMProvider, executor *SQLExecutor) *MultiCandidateGenerator {
	return &MultiCandidateGenerator{
		provider:      p,
		validator:     NewSQLSelfValidator(nil),
		executor:      executor,
		maxCandidates: GetText2SQLConfig().NumCandidates,
	}
}

// NewMultiCandidateGeneratorWithConfig 创建带配置的生成器
func NewMultiCandidateGeneratorWithConfig(p LLMProvider, executor *SQLExecutor, maxCandidates int) *MultiCandidateGenerator {
	return &MultiCandidateGenerator{
		provider:      p,
		validator:     NewSQLSelfValidator(nil),
		executor:      executor,
		maxCandidates: maxCandidates,
	}
}

// GenerateCandidates 生成多个候选SQL
func (g *MultiCandidateGenerator) GenerateCandidates(ctx context.Context, question string, schemaContext string, tables map[string]*modelai.TableCache) ([]CandidateSQL, error) {
	// 检测查询类型，决定生成策略
	queryType := g.detectQueryStrategy(question)
	strategies := strategyPlanFor(queryType)

	// 限制候选数量
	if len(strategies) > g.maxCandidates {
		strategies = strategies[:g.maxCandidates]
	}

	// 并行生成
	var wg sync.WaitGroup
	candidates := make([]CandidateSQL, len(strategies))
	errors := make([]error, len(strategies))

	for i, strategy := range strategies {
		wg.Add(1)
		go func(idx int, stratName string) {
			defer wg.Done()
			sql, conf, reason, err := g.generateWithStrategy(ctx, question, schemaContext, tables, stratName)
			if err != nil {
				errors[idx] = err
				return
			}
			candidates[idx] = CandidateSQL{
				SQL:        sql,
				Confidence: conf,
				Reason:     reason,
				Strategy:   stratName,
				Valid:      true,
			}
		}(i, strategy)
	}

	wg.Wait()

	// 过滤错误
	validCandidates := make([]CandidateSQL, 0)
	for i, c := range candidates {
		if errors[i] == nil && c.SQL != "" {
			validCandidates = append(validCandidates, c)
		}
	}

	if len(validCandidates) == 0 {
		return nil, fmt.Errorf("all candidate generation failed")
	}

	// 去重
	validCandidates = g.deduplicate(validCandidates)

	return validCandidates, nil
}

// strategyHints 是全部生成策略到提示词的映射，也是 buildStrategyPrompt 的
// 唯一判据。strategyPlanFor 产出的每个策略名必须在这里有对应条目，
// 表驱动测试（TestStrategyPlanIsCompleteAndClean）逐条对回，
// 防止再出现 " CTE" 这类带前导空格的策略名（case 匹配不上 → 提示词为空 →
// LLM 在不知道策略约束的情况下自由发挥）。
var strategyHints = map[string]string{
	"basic":       "直接生成简洁的SQL",
	"window":      "使用窗口函数（如SUM OVER, ROW_NUMBER等）",
	"CTE":         "使用WITH子句（公用表表达式）来组织复杂逻辑",
	"inner_join":  "优先使用INNER JOIN，多表关联",
	"left_join":   "使用LEFT JOIN确保主表数据完整",
	"nested":      "使用嵌套子查询",
	"temp_table":  "模拟临时表，使用子查询模拟中间结果",
	"stepwise":    "分步骤构建，每步解决一个小问题",
	"union":       "使用UNION组合多个查询结果",
	"alternative": "使用不同的表连接顺序或聚合方式",
}

// strategyPlanFor 按查询类型返回候选策略清单（顺序即优先级，
// 受 maxCandidates 截断）。五个分支的清单由
// TestStrategyPlanIsCompleteAndClean 逐条钉住。
func strategyPlanFor(queryType string) []string {
	switch queryType {
	case "aggregation":
		return []string{"basic", "window", "CTE"}
	case "join":
		return []string{"inner_join", "left_join", "CTE"}
	case "subquery":
		return []string{"nested", "CTE", "temp_table"}
	case "complex":
		return []string{"stepwise", "CTE", "union"}
	default:
		return []string{"basic", "alternative", "CTE"}
	}
}

// generateWithStrategy 使用特定策略生成SQL
func (g *MultiCandidateGenerator) generateWithStrategy(ctx context.Context, question string, schemaContext string, tables map[string]*modelai.TableCache, strategy string) (string, float64, string, error) {
	prompt := g.buildStrategyPrompt(question, schemaContext, strategy)

	resp, err := g.provider.ChatWithOptions(ctx, "你是一个SQL生成专家。", []Message{
		NewTextMessage("user", prompt),
	}, WithMaxTokens(GetText2SQLConfig().MaxTokens), WithTemperature(GetText2SQLConfig().TemperatureCandidate))

	if err != nil {
		return "", 0, "", err
	}

	sql := extractSQLFromReply(resp.Content)
	if sql == "" {
		return "", 0, "", fmt.Errorf("failed to extract SQL from response")
	}

	// 简单评估置信度
	confidence := g.estimateConfidence(sql, strategy)

	return sql, confidence, fmt.Sprintf("使用%s策略生成", strategy), nil
}

// buildStrategyPrompt 构建策略提示词
func (g *MultiCandidateGenerator) buildStrategyPrompt(question string, schemaContext string, strategy string) string {
	strategyHint := strategyHints[strategy]

	return "请根据以下问题，使用指定的策略生成SQL。\n\n" +
		"## 问题\n" +
		question + "\n\n" +
		"## Schema\n" +
		schemaContext + "\n\n" +
		"## 指定策略\n" +
		strategyHint + "\n\n" +
		"## 输出格式\n" +
		"请只输出SQL语句，不需要其他解释。"
}

// detectQueryStrategy 检测查询类型
func (g *MultiCandidateGenerator) detectQueryStrategy(question string) string {
	qLower := strings.ToLower(question)

	// 聚合查询
	aggKeywords := []string{"累计", "排名", "分组统计", "窗口", "over(", "running"}
	for _, kw := range aggKeywords {
		if strings.Contains(qLower, kw) {
			return "aggregation"
		}
	}

	// 多表查询
	joinKeywords := []string{"和", "以及", "关联", "join"}
	for _, kw := range joinKeywords {
		if strings.Contains(qLower, kw) {
			return "join"
		}
	}

	// 子查询
	subKeywords := []string{"所有", "同时", "满足", "至少"}
	for _, kw := range subKeywords {
		if strings.Contains(qLower, kw) {
			return "subquery"
		}
	}

	// 复杂查询
	complexKeywords := []string{"每月", "每季度", "趋势", "同比", "环比", "多维度"}
	for _, kw := range complexKeywords {
		if strings.Contains(qLower, kw) {
			return "complex"
		}
	}

	return "basic"
}

// estimateConfidence 估算置信度
func (g *MultiCandidateGenerator) estimateConfidence(sql, strategy string) float64 {
	confidence := 0.7

	// SQL完整性检查
	if strings.Contains(sql, "SELECT") && strings.Contains(sql, "FROM") {
		confidence += 0.1
	}

	// 有聚合
	if strings.Contains(strings.ToUpper(sql), "GROUP BY") {
		confidence += 0.05
	}

	// 有JOIN
	if strings.Contains(strings.ToUpper(sql), "JOIN") {
		confidence += 0.05
	}

	// 使用了高级特性
	if strings.Contains(strings.ToUpper(sql), "WITH") || strings.Contains(strings.ToUpper(sql), "OVER") {
		confidence += 0.05
	}

	// 长度适中
	length := len(sql)
	if length > 100 && length < 500 {
		confidence += 0.05
	}

	return minFloat(confidence, 1.0)
}

// extractSQL 从响应中提取SQL
// extractSQLFromReply 从 LLM 回复中剥离 markdown 代码块/前后废话，取出 SQL 主体。
func extractSQLFromReply(content string) string {
	// 尝试从代码块中提取
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
		return strings.TrimSpace(content)
	}

	return strings.TrimSpace(strings.Join(sqlLines, "\n"))
}

// deduplicate 去重
func (g *MultiCandidateGenerator) deduplicate(candidates []CandidateSQL) []CandidateSQL {
	seen := make(map[string]bool)
	result := make([]CandidateSQL, 0)

	for _, c := range candidates {
		// 标准化SQL进行比较
		normalized := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(c.SQL, " ", ""), "\n", ""))
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, c)
		}
	}

	return result
}

// ValidateCandidates 验证候选SQL
func (g *MultiCandidateGenerator) ValidateCandidates(ctx context.Context, candidates []CandidateSQL, question string, schemaContext string) []CandidateSQL {
	for i := range candidates {
		// 结构验证
		structResult := g.validator.ValidateStructure(candidates[i].SQL)
		candidates[i].Valid = structResult.IsValid

		// 如果验证失败，降低置信度
		if !structResult.IsValid {
			candidates[i].Confidence *= 0.5
		}
	}

	return candidates
}

// SelectBest 选择最优候选
func (g *MultiCandidateGenerator) SelectBest(candidates []CandidateSQL) *CandidateSQL {
	if len(candidates) == 0 {
		return nil
	}

	// 按置信度排序
	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			// 首先按有效性排序
			if candidates[i].Valid != candidates[j].Valid {
				if candidates[i].Valid {
					candidates[i], candidates[j] = candidates[j], candidates[i]
				}
				continue
			}

			// 然后按置信度
			if candidates[i].Confidence < candidates[j].Confidence {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	// 选择最优
	best := &candidates[0]

	// 设置排名
	for i := range candidates {
		candidates[i].Rank = i + 1
	}

	return best
}

// GenerateBestWithCandidates 生成最优SQL（同时返回所有候选）
func (g *MultiCandidateGenerator) GenerateBestWithCandidates(ctx context.Context, question string, schemaContext string, tables map[string]*modelai.TableCache) (*CandidateSQL, []CandidateSQL, error) {
	// 1. 生成候选
	candidates, err := g.GenerateCandidates(ctx, question, schemaContext, tables)
	if err != nil {
		return nil, nil, err
	}

	// 2. 验证候选
	candidates = g.ValidateCandidates(ctx, candidates, question, schemaContext)

	// 3. 选择最优
	best := g.SelectBest(candidates)

	return best, candidates, nil
}

// FormatCandidatesReport 格式化候选报告
func (g *MultiCandidateGenerator) FormatCandidatesReport(candidates []CandidateSQL) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("共生成 %d 个候选SQL：\n\n", len(candidates)))

	for i, c := range candidates {
		status := "✅"
		if !c.Valid {
			status = "❌"
		}

		sb.WriteString(fmt.Sprintf("### 候选 %d %s\n", i+1, status))
		sb.WriteString(fmt.Sprintf("策略：%s\n", c.Strategy))
		sb.WriteString(fmt.Sprintf("置信度：%.0f%%\n", c.Confidence*100))
		sb.WriteString(fmt.Sprintf("原因：%s\n", c.Reason))
		sb.WriteString(fmt.Sprintf("SQL：\n```sql\n%s\n```\n\n", c.SQL))

		if c.ExecResult != nil {
			if c.ExecResult.Error != "" {
				sb.WriteString(fmt.Sprintf("执行结果：❌ %s\n\n", c.ExecResult.Error))
			} else {
				sb.WriteString(fmt.Sprintf("执行结果：✅ %d 行\n\n", c.ExecResult.RowCount))
			}
		}
	}

	return sb.String()
}

// minFloat 取最小值
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
