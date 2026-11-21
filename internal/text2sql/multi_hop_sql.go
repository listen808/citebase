package text2sql

import (
	"context"
	"fmt"
	"strings"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/utils"
)

// MultiHopSQLReasoner 多跳SQL推理器
// 专门处理需要跨表关联的多跳查询场景
type MultiHopSQLReasoner struct {
	provider   LLMProvider
	ragAdapter *SQLRAGAdapter
	kb         *KnowledgeBase
	maxHops    int
}

// NewMultiHopSQLReasoner 创建多跳SQL推理器
func NewMultiHopSQLReasoner(p LLMProvider, ragAdapter *SQLRAGAdapter, kb *KnowledgeBase) *MultiHopSQLReasoner {
	if p == nil {
		return nil
	}
	return &MultiHopSQLReasoner{
		provider:   p,
		ragAdapter: ragAdapter,
		kb:         kb,
		maxHops:    GetText2SQLConfig().MultiHopMaxHops,
	}
}

// HopResult 单跳推理结果
type HopResult struct {
	Hop        int      `json:"hop"`         // 跳数
	SQL        string   `json:"sql"`         // 该跳的SQL
	Purpose    string   `json:"purpose"`     // 该跳的目的
	TablesUsed []string `json:"tables_used"` // 使用的表
	Reasoning  string   `json:"reasoning"`   // 推理过程
}

// MultiHopSQLResult 多跳SQL推理最终结果
type MultiHopSQLResult struct {
	FinalSQL   string      `json:"final_sql"`   // 最终合并的SQL
	CTESQL     string      `json:"cte_sql"`     // 使用CTE的SQL
	HopResults []HopResult `json:"hop_results"` // 每跳的结果
	TotalHops  int         `json:"total_hops"`  // 总跳数
	Reasoning  string      `json:"reasoning"`   // 整体推理过程
}

// Reason 执行多跳SQL推理
func (m *MultiHopSQLReasoner) Reason(ctx context.Context, question string, spaceID int) (*MultiHopSQLResult, error) {
	if m.provider == nil {
		return nil, fmt.Errorf("provider not initialized")
	}

	// 1. 分析问题是否需要多跳
	requiresHops := m.analyzeMultiHopRequirement(question)
	if !requiresHops {
		return nil, nil // 返回nil表示不需要多跳
	}

	logx.Infof("[MultiHopSQL] Question requires multi-hop reasoning: %s", utils.TruncateWithEllipsis(question, 50))

	// 2. 生成分解计划
	plan, err := m.generateDecomposePlan(ctx, question, spaceID)
	if err != nil {
		logx.Infof("[MultiHopSQL] Failed to generate decompose plan: %v", err)
		return nil, err
	}

	// 3. 执行每一跳的SQL生成
	hopResults, err := m.executeHops(ctx, question, spaceID, plan)
	if err != nil {
		return nil, err
	}

	// 4. 构建最终SQL
	finalSQL, cteSQL := m.buildFinalSQL(hopResults)

	result := &MultiHopSQLResult{
		FinalSQL:   finalSQL,
		CTESQL:     cteSQL,
		HopResults: hopResults,
		TotalHops:  len(hopResults),
	}

	return result, nil
}

// analyzeMultiHopRequirement 分析问题是否需要多跳
func (m *MultiHopSQLReasoner) analyzeMultiHopRequirement(question string) bool {
	qLower := strings.ToLower(question)

	// 需要多跳的场景
	multiHopIndicators := []string{
		"和", "以及", "关联", "结合",
		"每个", "各", "所有",
		"都", "同时",
		"先", "再", "然后",
	}

	hopCount := 0
	for _, indicator := range multiHopIndicators {
		if strings.Contains(qLower, indicator) {
			hopCount++
		}
	}

	// 出现2个及以上多跳指示器，认为需要多跳
	return hopCount >= 2
}

// generateDecomposePlan 生成分解计划
func (m *MultiHopSQLReasoner) generateDecomposePlan(ctx context.Context, question string, spaceID int) ([]string, error) {
	// 构建提示词
	prompt := fmt.Sprintf(`请将以下复杂SQL问题分解为多个简单的子问题。

## 问题
%s

## 任务
1. 分析问题包含哪些查询维度
2. 确定各维度之间的依赖关系
3. 将问题分解为2-4个简单的子问题
4. 确定执行顺序（哪些可以并行，哪些必须串行）

## 输出格式
请按以下JSON格式输出：
{
  "hops": [
    {
      "hop": 1,
      "sub_question": "第一个子问题",
      "purpose": "该子问题的目的",
      "depends_on": []
    },
    {
      "hop": 2,
      "sub_question": "第二个子问题",
      "purpose": "该子问题的目的",
      "depends_on": [1]
    }
  ]
}
`, question)

	messages := []Message{
		NewTextMessage("user", prompt),
	}

	resp, err := m.provider.ChatWithOptions(ctx, "你是一个SQL问题分解专家。", messages,
		WithMaxTokens(800),
		WithTemperature(0.3))
	if err != nil {
		return nil, fmt.Errorf("generate plan failed: %w", err)
	}

	// 解析响应，提取子问题
	subQuestions, err := m.parseSubQuestions(resp.Content)
	if err != nil {
		return nil, err
	}
	return subQuestions, nil
}

// parseSubQuestions 解析子问题
func (m *MultiHopSQLReasoner) parseSubQuestions(content string) ([]string, error) {
	// 简单实现：直接提取JSON中的sub_question
	// 实际应该用更严格的JSON解析
	lines := strings.Split(content, "\n")
	var subQuestions []string
	var currentSubQ string
	inSubQ := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, `"sub_question"`) {
			inSubQ = true
			// 提取冒号后的内容
			parts := strings.SplitAfterN(line, ":", 2)
			if len(parts) > 1 {
				currentSubQ = strings.Trim(parts[1], " \",:")
			}
			continue
		}
		if inSubQ {
			if strings.HasPrefix(line, "\"") && !strings.HasPrefix(line, "\"hop\"") {
				currentSubQ += " " + strings.Trim(line, " \",")
			}
			if strings.Contains(line, "}") {
				currentSubQ = strings.Trim(currentSubQ, " \",")
				if currentSubQ != "" {
					subQuestions = append(subQuestions, currentSubQ)
				}
				currentSubQ = ""
				inSubQ = false
			}
		}
	}

	// 如果解析失败，返回默认分解
	if len(subQuestions) == 0 {
		// 按"和"或"以及"分割
		if strings.Contains(content, "和") {
			parts := strings.Split(content, "和")
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					subQuestions = append(subQuestions, p)
				}
			}
		}
	}

	// 限制跳数
	if len(subQuestions) > m.maxHops {
		subQuestions = subQuestions[:m.maxHops]
	}

	return subQuestions, nil
}

// executeHops 执行每一跳
func (m *MultiHopSQLReasoner) executeHops(ctx context.Context, question string, spaceID int, subQuestions []string) ([]HopResult, error) {
	var hopResults []HopResult

	for i, subQ := range subQuestions {
		hopNum := i + 1

		// 获取该跳需要的Schema
		schemaCtx := ""
		if m.ragAdapter != nil {
			sources := m.ragAdapter.RetrieveRelevantSchemas(subQ, spaceID, 5)
			schemaCtx = m.buildSchemaContextFromSources(sources)
		}

		// 生成该跳的SQL
		hopSQL, reasoning, err := m.generateHopSQL(ctx, subQ, schemaCtx, hopNum, len(subQuestions))
		if err != nil {
			logx.Infof("[MultiHopSQL] Hop %d failed: %v", hopNum, err)
			continue
		}

		hopResults = append(hopResults, HopResult{
			Hop:        hopNum,
			SQL:        hopSQL,
			Purpose:    subQ,
			TablesUsed: m.extractTablesFromSQL(hopSQL),
			Reasoning:  reasoning,
		})
	}

	return hopResults, nil
}

// generateHopSQL 生成单跳SQL
func (m *MultiHopSQLReasoner) generateHopSQL(ctx context.Context, subQuestion, schemaContext string, hopNum, totalHops int) (string, string, error) {
	prompt := "请根据以下子问题生成SQL。\n\n" +
		"## 子问题（第" + fmt.Sprintf("%d/%d跳", hopNum, totalHops) + "）\n" +
		subQuestion + "\n\n" +
		"## Schema上下文\n" +
		schemaContext + "\n\n" +
		"## 要求\n" +
		"1. 只生成SELECT语句\n" +
		"2. 如果需要，可以使用WITH子句（CTE）定义中间结果\n" +
		"3. 输出格式：先给出推理过程，再给出SQL\n\n" +
		"## 输出格式\n" +
		"```\n" +
		"推理过程：...\n" +
		"SQL：...\n" +
		"```"

	messages := []Message{
		NewTextMessage("user", prompt),
	}

	resp, err := m.provider.ChatWithOptions(ctx, "你是一个SQL生成专家。", messages,
		WithMaxTokens(500),
		WithTemperature(0.2))
	if err != nil {
		return "", "", err
	}

	// 解析SQL
	sql := m.extractSQLFromResponse(resp.Content)
	reasoning := m.extractReasoning(resp.Content)

	return sql, reasoning, nil
}

// extractSQLFromResponse 从响应中提取SQL
func (m *MultiHopSQLReasoner) extractSQLFromResponse(content string) string {
	lines := strings.Split(content, "\n")
	var sqlLines []string
	inSQL := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SQL") && strings.Contains(line, ":") {
			inSQL = true
			continue
		}
		if inSQL {
			if strings.HasPrefix(line, "```") {
				break
			}
			if strings.HasPrefix(line, "SELECT") || strings.HasPrefix(line, "WITH") || strings.HasPrefix(line, "(") {
				sqlLines = append(sqlLines, line)
			}
		}
	}

	if len(sqlLines) == 0 {
		// 尝试直接从代码块提取
		inCodeBlock := false
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "```") {
				inCodeBlock = !inCodeBlock
				continue
			}
			if inCodeBlock {
				sqlLines = append(sqlLines, line)
			}
		}
	}

	return strings.TrimSpace(strings.Join(sqlLines, "\n"))
}

// extractReasoning 从响应中提取推理过程
func (m *MultiHopSQLReasoner) extractReasoning(content string) string {
	lines := strings.Split(content, "\n")
	var reasoningLines []string
	inReasoning := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "推理过程") || strings.HasPrefix(line, "reasoning") {
			inReasoning = true
			parts := strings.SplitAfterN(line, ":", 2)
			if len(parts) > 1 {
				reasoningLines = append(reasoningLines, strings.Trim(parts[1], " \":"))
			}
			continue
		}
		if inReasoning {
			if strings.HasPrefix(line, "SQL") || strings.HasPrefix(line, "```") {
				break
			}
			reasoningLines = append(reasoningLines, line)
		}
	}

	return strings.TrimSpace(strings.Join(reasoningLines, " "))
}

// buildSchemaContextFromSources 从检索结果构建Schema上下文
func (m *MultiHopSQLReasoner) buildSchemaContextFromSources(sources []RAGSchemaSource) string {
	if len(sources) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, src := range sources {
		if src.RichText != "" {
			sb.WriteString(src.RichText)
			sb.WriteString("\n\n")
		} else {
			sb.WriteString(fmt.Sprintf("表: %s\n", src.TableName))
		}
	}

	return sb.String()
}

// extractTablesFromSQL 从SQL中提取表名
func (m *MultiHopSQLReasoner) extractTablesFromSQL(sql string) []string {
	// 简单实现：查找FROM和JOIN后的表名
	lines := strings.Split(sql, "\n")
	var tables []string
	seen := make(map[string]bool)

	keywords := []string{"FROM", "JOIN", "LEFT JOIN", "RIGHT JOIN", "INNER JOIN", "FROM"}

	for _, line := range lines {
		line = strings.ToUpper(line)
		for _, kw := range keywords {
			if strings.Contains(line, kw) {
				// 简单提取：取kw后的第一个单词
				parts := strings.SplitAfterN(line, kw, 2)
				if len(parts) > 1 {
					fields := strings.Fields(parts[1])
					if len(fields) == 0 {
						continue
					}
					table := fields[0]
					table = strings.Trim(table, " (),")
					if table != "" && !seen[table] {
						seen[table] = true
						tables = append(tables, table)
					}
				}
			}
		}
	}

	return tables
}

// buildFinalSQL 构建最终SQL
func (m *MultiHopSQLReasoner) buildFinalSQL(hopResults []HopResult) (string, string) {
	if len(hopResults) == 0 {
		return "", ""
	}

	if len(hopResults) == 1 {
		return hopResults[0].SQL, hopResults[0].SQL
	}

	// 多跳：构建CTE SQL
	var cteParts []string
	for i, hr := range hopResults {
		cteParts = append(cteParts, fmt.Sprintf("step%d AS (\n  %s\n)", i+1, hr.SQL))
	}

	cteSQL := fmt.Sprintf("WITH\n%s\nSELECT * FROM step%d",
		strings.Join(cteParts, ",\n"),
		len(hopResults))

	// 最终SQL通常使用最后一步的结果
	finalSQL := hopResults[len(hopResults)-1].SQL

	return finalSQL, cteSQL
}
