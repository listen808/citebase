package text2sql

import "strings"

// P1-9 (§4.3.2): Few-shot 智能选择 — SQL 查询类型推断。
//
// QueryType 枚举与 ClassifyQuery（自然语言分类）已在 prompt_templates.go 定义。
// 本文件补充 inferSQLQueryType：从已有示例的 SQL 文本推断查询类型，
// 用于 Few-shot 检索时的问题类型 ↔ 示例类型匹配加权（无需新增 DB 字段）。

// inferSQLQueryType 根据 SQL 文本推断查询类型
// 用于已有示例库无 QueryType 字段时的回退判定
func inferSQLQueryType(sql string) QueryType {
	if sql == "" {
		return QueryTypeUnknown
	}
	s := strings.ToUpper(sql)

	// CTE：WITH ... AS（排除 WITH NO DATA / WITH CHECK 等非 CTE 用法）
	if strings.Contains(s, "WITH ") || strings.Contains(s, "WITH\n") {
		if !strings.Contains(s, "WITH NO DATA") && !strings.Contains(s, "WITH CHECK") {
			return QueryTypeCTE
		}
	}
	// 子查询（IN (SELECT ...) / EXISTS (SELECT ...)）
	if strings.Contains(s, "IN (SELECT") || strings.Contains(s, "EXISTS (SELECT") ||
		strings.Contains(s, "IN(SELECT") {
		return QueryTypeSubquery
	}
	// 窗口函数 / 复杂报表：OVER() / ROW_NUMBER / RANK / 同比环比
	if strings.Contains(s, "OVER(") || strings.Contains(s, "OVER (") ||
		strings.Contains(s, "ROW_NUMBER(") || strings.Contains(s, "RANK()") ||
		strings.Contains(s, "DENSE_RANK(") || strings.Contains(s, "NTILE(") {
		return QueryTypeComplexReport
	}
	// 多表关联
	if strings.Contains(s, "JOIN") {
		return QueryTypeMultiJoin
	}
	// 聚合
	if strings.Contains(s, "GROUP BY") || strings.Contains(s, "COUNT(") ||
		strings.Contains(s, "SUM(") || strings.Contains(s, "AVG(") ||
		strings.Contains(s, "MAX(") || strings.Contains(s, "MIN(") {
		return QueryTypeAggregation
	}
	return QueryTypeBasic
}
