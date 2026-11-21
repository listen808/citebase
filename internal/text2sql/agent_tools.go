package text2sql

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/listen808/citebase/internal/ai/provider"
	modelai "github.com/listen808/citebase/internal/model/ai"
	"github.com/listen808/citebase/internal/utils"
)

// AgentTool defines a tool that the agent can call
type AgentTool struct {
	Definition provider.ToolDefinition
	Handler    func(ctx context.Context, args map[string]interface{}) (string, error)
}

// registerTools 注册所有 Agent 工具
func (a *Agent) registerTools() {
	// 1. search_schema - 搜索表结构
	a.tools["search_schema"] = &AgentTool{
		Definition: provider.ToolDefinition{
			Name:        "search_schema",
			Description: "搜索数据库表结构。输入关键词或表名，返回匹配的表及其DDL结构和列信息。必须在使用任何字段前先调用此工具确认字段名。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "搜索关键词或表名，如 'users'、'订单'、'employee'",
					},
				},
				"required": []string{"query"},
			},
		},
		Handler: a.handleSearchSchema,
	}

	// 2. get_columns - 获取指定表的列信息
	a.tools["get_columns"] = &AgentTool{
		Definition: provider.ToolDefinition{
			Name:        "get_columns",
			Description: "获取指定表的所有列信息，包括列名、数据类型、是否可空、注释等。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table_name": map[string]interface{}{
						"type":        "string",
						"description": "表名，如 'users'、'orders'",
					},
				},
				"required": []string{"table_name"},
			},
		},
		Handler: a.handleGetColumns,
	}

	// 3. get_relations - 获取表间关系
	a.tools["get_relations"] = &AgentTool{
		Definition: provider.ToolDefinition{
			Name:        "get_relations",
			Description: "获取表之间的关联关系（外键、JOIN条件），帮助理解多表查询时的连接方式。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table_name": map[string]interface{}{
						"type":        "string",
						"description": "要查询关联关系的表名，如 'orders'",
					},
				},
				"required": []string{"table_name"},
			},
		},
		Handler: a.handleGetRelations,
	}

	// 4. search_examples - 搜索相似示例
	a.tools["search_examples"] = &AgentTool{
		Definition: provider.ToolDefinition{
			Name:        "search_examples",
			Description: "搜索与当前问题相似的历史SQL查询示例，作为参考来生成更准确的SQL。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "描述查询意图的关键词，如 '统计各部门人数'、'查询销售额'",
					},
				},
				"required": []string{"query"},
			},
		},
		Handler: a.handleSearchExamples,
	}

	// 5. execute_sql - 执行SQL验证
	a.tools["execute_sql"] = &AgentTool{
		Definition: provider.ToolDefinition{
			Name:        "execute_sql",
			Description: "在数据库中执行SQL查询（仅限SELECT），用于验证SQL是否正确。成功时返回查询结果，失败时返回错误信息。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"sql": map[string]interface{}{
						"type":        "string",
						"description": "要执行的SQL查询语句（仅限SELECT）",
					},
				},
				"required": []string{"sql"},
			},
		},
		Handler: a.handleExecuteSQL,
	}

	// 6. validate_sql - 校验SQL结构
	a.tools["validate_sql"] = &AgentTool{
		Definition: provider.ToolDefinition{
			Name:        "validate_sql",
			Description: "校验SQL语句的结构合法性，检查表名、字段名是否存在，JOIN逻辑是否正确。不实际执行SQL。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"sql": map[string]interface{}{
						"type":        "string",
						"description": "要校验的SQL语句",
					},
				},
				"required": []string{"sql"},
			},
		},
		Handler: a.handleValidateSQL,
	}

	// 7. clarify - 向用户确认
	a.tools["clarify"] = &AgentTool{
		Definition: provider.ToolDefinition{
			Name:        "clarify",
			Description: "当字段名有歧义或查询意图不明确时，向用户提出问题以澄清。调用此工具后，Agent会暂停等待用户回复。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"question": map[string]interface{}{
						"type":        "string",
						"description": "向用户提出的澄清问题，如 '你说的用户是指 user_id 还是 username？'",
					},
					"options": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "可选的选项列表，供用户选择",
					},
				},
				"required": []string{"question"},
			},
		},
		Handler: a.handleClarify,
	}

	// 8. final_answer - 提交最终结果
	a.tools["final_answer"] = &AgentTool{
		Definition: provider.ToolDefinition{
			Name:        "final_answer",
			Description: "提交最终的SQL查询结果。必须在确认SQL正确无误后调用。",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"sql": map[string]interface{}{
						"type":        "string",
						"description": "最终的SQL查询语句",
					},
					"explanation": map[string]interface{}{
						"type":        "string",
						"description": "SQL生成逻辑的中文解释",
					},
					"tables": map[string]interface{}{
						"type":        "array",
						"items":       map[string]interface{}{"type": "string"},
						"description": "SQL中涉及的表名列表",
					},
				},
				"required": []string{"sql", "explanation", "tables"},
			},
		},
		Handler: a.handleFinalAnswer,
	}
}

// handleSearchSchema 处理 search_schema 工具调用
func (a *Agent) handleSearchSchema(ctx context.Context, args map[string]interface{}) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "请提供搜索关键词", nil
	}

	kb := a.generator.kbFromContext(ctx)
	if kb == nil || len(kb.GetAllTables()) == 0 {
		return "知识库中暂无表结构信息，请先导入Schema", nil
	}

	var results []string
	queryLower := strings.ToLower(query)

	for tableName := range kb.GetAllTables() {
		nameLower := strings.ToLower(tableName)
		// 模糊匹配表名
		if strings.Contains(nameLower, queryLower) || strings.Contains(queryLower, nameLower) {
			tableCache := kb.GetTable(tableName)
			if tableCache != nil {
				results = append(results, formatTableInfo(tableName, tableCache))
			}
		}
	}

	// 如果没有精确匹配，返回所有表名供参考
	if len(results) == 0 {
		allTables := kb.GetAllTables()
		tableNames := make([]string, 0, len(allTables))
		for name := range allTables {
			tableNames = append(tableNames, name)
		}
		return fmt.Sprintf("未找到匹配 '%s' 的表。可用表: %s\n请使用 get_columns 查看具体表结构。", query, strings.Join(tableNames, ", ")), nil
	}

	return strings.Join(results, "\n\n"), nil
}

// handleGetColumns 处理 get_columns 工具调用
func (a *Agent) handleGetColumns(ctx context.Context, args map[string]interface{}) (string, error) {
	tableName, _ := args["table_name"].(string)
	if tableName == "" {
		return "请指定表名", nil
	}

	kb := a.generator.kbFromContext(ctx)
	tableCache := kb.GetTable(tableName)
	if tableCache == nil {
		// 尝试模糊匹配
		for name := range kb.GetAllTables() {
			if strings.EqualFold(name, tableName) {
				tableCache = kb.GetTable(name)
				tableName = name
				break
			}
		}
		if tableCache == nil {
			return fmt.Sprintf("未找到表 '%s'", tableName), nil
		}
	}

	return formatTableInfo(tableName, tableCache), nil
}

// handleGetRelations 处理 get_relations 工具调用
func (a *Agent) handleGetRelations(ctx context.Context, args map[string]interface{}) (string, error) {
	tableName, _ := args["table_name"].(string)
	if tableName == "" {
		return "请指定表名", nil
	}

	kb := a.generator.kbFromContext(ctx)
	rels := kb.GetRelations(tableName)
	if len(rels) == 0 {
		return fmt.Sprintf("表 '%s' 没有配置的关联关系", tableName), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("表 '%s' 的关联关系：\n", tableName))
	for _, rel := range rels {
		sb.WriteString(fmt.Sprintf("- %s.%s → %s.%s (类型: %s)\n",
			rel.SourceTable, rel.SourceField, rel.TargetTable, rel.TargetField, rel.RelationType))
	}
	return sb.String(), nil
}

// handleSearchExamples 处理 search_examples 工具调用
func (a *Agent) handleSearchExamples(ctx context.Context, args map[string]interface{}) (string, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return "请提供搜索关键词", nil
	}

	if a.generator.exampleVS == nil {
		return "示例检索服务未初始化", nil
	}

	// 使用已有的 example vector store；限定在请求绑定空间内检索，避免把其他空间
	// 的示例 SQL 回灌进本会话 prompt（跨空间信息泄露）
	results, err := a.generator.exampleVS.RetrieveRelevantExamples(ctx, query, int(SpaceIDFromContext(ctx)), 3)
	if err != nil || len(results) == 0 {
		return fmt.Sprintf("未找到与 '%s' 相关的历史示例", query), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("找到 %d 个相关示例：\n", len(results)))
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("\n### 示例 %d (相似度: %.2f)\n", i+1, r.Score))
		sb.WriteString(fmt.Sprintf("问题: %s\n", r.Question))
		sb.WriteString(fmt.Sprintf("SQL:\n```sql\n%s\n```\n", r.SQL))
	}
	return sb.String(), nil
}

// handleExecuteSQL 处理 execute_sql 工具调用
func (a *Agent) handleExecuteSQL(ctx context.Context, args map[string]interface{}) (string, error) {
	sqlStr, _ := args["sql"].(string)
	if sqlStr == "" {
		return "请提供SQL语句", nil
	}

	if a.generator.executor == nil {
		// 如果没有 executor，返回结构校验结果
		kb := a.generator.kbFromContext(ctx)
		var schemaTables []string
		for name := range kb.GetAllTables() {
			schemaTables = append(schemaTables, name)
		}
		issues := ValidateSQLStructure(sqlStr, schemaTables)
		if len(issues) == 0 {
			return "SQL结构校验通过，但无法执行（未配置数据源）", nil
		}
		var sb strings.Builder
		sb.WriteString("SQL结构校验发现问题：\n")
		for _, iss := range issues {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", iss.Type, iss.Message))
		}
		return sb.String(), nil
	}

	// P1 首要任务 1 / 清单 30：spaceID 与调用者身份沿请求 ctx 传递，
	// 不再硬编码 0（旧写法使策略/数据源/RLS 全部落到 space 0 或整体被拒）。
	spaceID := SpaceIDFromContext(ctx)
	if spaceID == 0 {
		return "当前会话未绑定空间，无法执行 SQL（仅可返回结构校验结果）", nil
	}
	execResult := a.generator.executor.ExecuteWithContext(ctx, sqlStr, spaceID)
	if execResult.Success {
		b, _ := json.MarshalIndent(execResult, "", "  ")
		return fmt.Sprintf("SQL执行成功！\n结果:\n%s", string(b)), nil
	}
	return fmt.Sprintf("SQL执行失败: %s", execResult.Error), nil
}

// handleValidateSQL 处理 validate_sql 工具调用
func (a *Agent) handleValidateSQL(ctx context.Context, args map[string]interface{}) (string, error) {
	sqlStr, _ := args["sql"].(string)
	if sqlStr == "" {
		return "请提供SQL语句", nil
	}

	kb := a.generator.kbFromContext(ctx)
	var schemaTables []string
	for name := range kb.GetAllTables() {
		schemaTables = append(schemaTables, name)
	}

	issues := ValidateSQLStructure(sqlStr, schemaTables)
	if len(issues) == 0 {
		return "SQL结构校验通过，语法正确", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("SQL结构校验发现 %d 个问题：\n", len(issues)))
	for _, iss := range issues {
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", iss.Type, iss.Message))
	}
	return sb.String(), nil
}

// handleClarify 处理 clarify 工具调用
func (a *Agent) handleClarify(ctx context.Context, args map[string]interface{}) (string, error) {
	question, _ := args["question"].(string)
	if question == "" {
		return "请提供澄清问题", nil
	}

	var sb strings.Builder
	sb.WriteString(question)

	if options, ok := args["options"].([]interface{}); ok && len(options) > 0 {
		sb.WriteString("\n\n可选答案：\n")
		for i, opt := range options {
			if s, ok := opt.(string); ok {
				sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, s))
			}
		}
	}

	return sb.String(), nil
}

// handleFinalAnswer 处理 final_answer 工具调用
func (a *Agent) handleFinalAnswer(ctx context.Context, args map[string]interface{}) (string, error) {
	sql, _ := args["sql"].(string)
	if sql == "" {
		return "请提供SQL语句", nil
	}
	return fmt.Sprintf("最终答案已提交。SQL: %s", utils.TruncateWithEllipsis(sql, 100)), nil
}

// formatTableInfo 格式化表信息
func formatTableInfo(tableName string, cache *modelai.TableCache) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 表: %s\n", tableName))
	if cache.TableComment != "" {
		sb.WriteString(fmt.Sprintf("注释: %s\n", cache.TableComment))
	}
	if cache.DDLContent != "" {
		sb.WriteString(fmt.Sprintf("DDL:\n```sql\n%s\n```\n", cache.DDLContent))
	}
	if len(cache.ColumnsJSON) > 0 {
		sb.WriteString("列信息:\n")
		sb.WriteString("| 列名 | 类型 | 可空 | 注释 |\n")
		sb.WriteString("|------|------|------|------|\n")
		for _, col := range cache.ColumnsJSON {
			nullable := "YES"
			if !col.IsNullable {
				nullable = "NO"
			}
			comment := col.Comment
			if comment == "" {
				comment = "-"
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", col.Name, col.Type, nullable, comment))
		}
	}
	return sb.String()
}
