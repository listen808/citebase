package rag

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/utils/config"
)

// DatabaseQueryTool 数据库查询工具
type DatabaseQueryTool struct {
	db *sql.DB
}

func NewDatabaseQueryTool(db *sql.DB) *DatabaseQueryTool {
	return &DatabaseQueryTool{db: db}
}

func (t *DatabaseQueryTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "query_database",
		Description: "执行SQL查询语句，返回查询结果。仅支持SELECT查询，禁止执行修改操作",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"sql": map[string]interface{}{
					"type":        "string",
					"description": "SQL查询语句（仅支持SELECT）",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "返回结果行数限制，默认100，最大1000",
				},
			},
			"required": []string{"sql"},
		},
	}
}

func (t *DatabaseQueryTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	if t.db == nil {
		return &ToolResult{
			ToolName: "query_database",
			Success:  false,
			Error:    "数据库连接未初始化",
		}, nil
	}
	sqlQuery, _ := params["sql"].(string)
	if sqlQuery == "" {
		return &ToolResult{
			ToolName: "query_database",
			Success:  false,
			Error:    "缺少sql参数",
		}, nil
	}

	// 安全检查：走 SQL AST 校验（单条语句、无注释注入），再做能力约束。
	sqlUpper := strings.ToUpper(strings.TrimSpace(sqlQuery))
	if !strings.HasPrefix(sqlUpper, "SELECT") {
		return &ToolResult{
			ToolName: "query_database",
			Success:  false,
			Error:    "安全限制：仅允许执行SELECT查询语句",
		}, nil
	}
	// 旧实现只有「以 SELECT 开头 + 关键字 Contains」两层字符串判断：
	// INTO OUTFILE / LOAD_FILE() / SLEEP() / json_table 以及 mysql.* 系统表全部不挡，
	// 拿到主业务库连接就等于可以拖库或把数据库打成 DoS。
	if err := ValidateSQLAST(sqlQuery); err != nil {
		return &ToolResult{
			ToolName: "query_database",
			Success:  false,
			Error:    "安全限制：SQL 未通过语法校验（仅允许单条 SELECT，禁止注释与多语句）: " + err.Error(),
		}, nil
	}

	// 禁止危险操作
	dangerous := []string{"INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER", "TRUNCATE",
		"GRANT", "REVOKE", "SET ", "CALL", "RENAME", "REPLACE", "PREPARE", "EXECUTE",
		"INTO OUTFILE", "INTO DUMPFILE", "LOAD DATA", "LOAD_FILE", "INFORMATION_SCHEMA",
		"MYSQL.USER", "PERFORMANCE_SCHEMA", "SLEEP(", "BENCHMARK("}
	sqlCompact := strings.ToUpper(strings.Join(strings.Fields(sqlUpper), " "))
	for _, keyword := range dangerous {
		needle := strings.ToUpper(strings.Join(strings.Fields(keyword), " "))
		if needle == "" {
			continue
		}
		if strings.Contains(sqlCompact, needle) {
			return &ToolResult{
				ToolName: "query_database",
				Success:  false,
				Error:    fmt.Sprintf("安全限制：禁止执行包含 %s 的查询", strings.TrimSpace(keyword)),
			}, nil
		}
	}

	// 解析limit参数：以服务端配置为硬上限，模型给的 limit 只能更小
	limit := 100
	if max := config.AppConfig.AI.AgentTools.MaxResultRows; max > 0 {
		limit = max
	}
	if l, ok := params["limit"].(float64); ok && l > 0 {
		if int(l) < limit {
			limit = int(l)
		}
	}
	if limit > 1000 {
		limit = 1000
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	rows, err := t.db.QueryContext(ctx, sqlQuery)
	if err != nil {
		return &ToolResult{
			ToolName: "query_database",
			Success:  false,
			Error:    fmt.Sprintf("查询执行失败: %v", err),
		}, nil
	}
	defer rows.Close()

	// 获取列名
	columns, err := rows.Columns()
	if err != nil {
		return &ToolResult{
			ToolName: "query_database",
			Success:  false,
			Error:    fmt.Sprintf("获取列名失败: %v", err),
		}, nil
	}

	// 读取结果
	var results []map[string]interface{}
	count := 0
	for rows.Next() && count < limit {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return &ToolResult{
				ToolName: "query_database",
				Success:  false,
				Error:    fmt.Sprintf("读取数据失败: %v", err),
			}, nil
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			// 处理byte数组转换为字符串
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
		count++
	}

	if err := rows.Err(); err != nil {
		return &ToolResult{
			ToolName: "query_database",
			Success:  false,
			Error:    fmt.Sprintf("遍历结果失败: %v", err),
		}, nil
	}

	// 格式化输出
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("查询成功，返回 %d 行结果:\n\n", len(results)))
	sb.WriteString(fmt.Sprintf("列: %s\n\n", strings.Join(columns, ", ")))

	for i, row := range results {
		sb.WriteString(fmt.Sprintf("第 %d 行:\n", i+1))
		for _, col := range columns {
			sb.WriteString(fmt.Sprintf("  %s: %v\n", col, row[col]))
		}
		if i < len(results)-1 {
			sb.WriteString("\n")
		}
	}

	return &ToolResult{
		ToolName: "query_database",
		Success:  true,
		Result:   sb.String(),
	}, nil
}
