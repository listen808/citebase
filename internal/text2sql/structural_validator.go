package text2sql

import (
	"fmt"
	"strings"

	"github.com/listen808/citebase/internal/ai/rag"
	"github.com/xwb1989/sqlparser"
)

type StructuralIssue struct {
	Type    string
	Message string
}

func ValidateSQLStructure(sqlStr string, schemaTables []string) []StructuralIssue {
	var issues []StructuralIssue

	if strings.TrimSpace(sqlStr) == "" {
		return append(issues, StructuralIssue{Type: "error", Message: "SQL为空"})
	}

	// Use AST-based validation via rag.ValidateSQLAST
	if err := rag.ValidateSQLAST(sqlStr); err != nil {
		return append(issues, StructuralIssue{Type: "parse_error", Message: err.Error()})
	}

	// Parse into AST for structural checks
	stmt, err := sqlparser.Parse(sqlStr)
	if err != nil {
		return append(issues, StructuralIssue{Type: "parse_error", Message: err.Error()})
	}

	sel, ok := stmt.(*sqlparser.Select)
	if !ok {
		return issues
	}

	// 1. Extract aliases defined in FROM/JOIN
	definedAliases := make(map[string]bool)
	for _, expr := range sel.From {
		collectDefinedAliases(expr, definedAliases)
	}

	// 2. Extract column references from SELECT and check alias validity
	referencedAliases := make(map[string]bool)
	collectReferencedAliases(sel.SelectExprs, referencedAliases)

	for alias := range referencedAliases {
		if !definedAliases[strings.ToUpper(alias)] && !definedAliases[alias] {
			issues = append(issues, StructuralIssue{
				Type:    "error",
				Message: fmt.Sprintf("列引用使了别名 %s，但 %s 不在 FROM/JOIN 中定义", alias, alias),
			})
		}
	}

	// 3. Check duplicate columns in SELECT
	colCounts := make(map[string]int)
	collectSelectColNames(sel.SelectExprs, colCounts)
	for col, count := range colCounts {
		if count > 1 {
			issues = append(issues, StructuralIssue{
				Type:    "error",
				Message: fmt.Sprintf("SELECT 中存在重复列: %s (出现 %d 次)", col, count),
			})
		}
	}

	// 4. Check multi-table reference without JOIN
	// 注意：definedAliases 中包含表名和别名（如 FROM BM_DATA_DICT bm → 注册了 BM_DATA_DICT 和 BM），
	// 单表带别名时 len(definedAliases) >= 2，不应误判为多表。
	// 正确做法：统计实际涉及的不同表名（排除别名）。
	distinctTables := make(map[string]bool)
	for _, expr := range sel.From {
		collectDistinctTables(expr, distinctTables)
	}
	if len(distinctTables) > 1 && !hasJoinClause(sel) {
		issues = append(issues, StructuralIssue{
			Type:    "missing_join",
			Message: "多表引用但缺少JOIN子句",
		})
	}

	// 5. Check WHERE - if SQL has filter intent but no WHERE clause
	if hasFilterIntent(sqlStr) && sel.Where == nil {
		issues = append(issues, StructuralIssue{
			Type:    "warning",
			Message: "查询涉及过滤条件，但缺少 WHERE 子句",
		})
	}

	return issues
}

// collectDefinedAliases recursively collects table names and aliases from FROM/JOIN
func collectDefinedAliases(expr sqlparser.TableExpr, aliases map[string]bool) {
	switch e := expr.(type) {
	case *sqlparser.AliasedTableExpr:
		if e.As.String() != "" {
			aliases[strings.ToUpper(e.As.String())] = true
		}
		if tableName := sqlparser.GetTableName(e.Expr); tableName.String() != "" {
			aliases[strings.ToUpper(tableName.String())] = true
		}
	case *sqlparser.JoinTableExpr:
		collectDefinedAliases(e.LeftExpr, aliases)
		collectDefinedAliases(e.RightExpr, aliases)
	case *sqlparser.ParenTableExpr:
		for _, subExpr := range e.Exprs {
			collectDefinedAliases(subExpr, aliases)
		}
	}
}

// collectReferencedAliases extracts alias qualifiers from SELECT expressions
func collectReferencedAliases(exprs sqlparser.SelectExprs, aliases map[string]bool) {
	for _, expr := range exprs {
		if ae, ok := expr.(*sqlparser.AliasedExpr); ok {
			collectColNameAliases(ae.Expr, aliases)
		}
	}
}

// collectColNameAliases walks an expression and collects ColName qualifiers
func collectColNameAliases(expr sqlparser.Expr, aliases map[string]bool) {
	switch e := expr.(type) {
	case *sqlparser.ColName:
		if e.Qualifier.Name.String() != "" {
			aliases[e.Qualifier.Name.String()] = true
		}
	case *sqlparser.BinaryExpr:
		collectColNameAliases(e.Left, aliases)
		collectColNameAliases(e.Right, aliases)
	case *sqlparser.UnaryExpr:
		collectColNameAliases(e.Expr, aliases)
	case *sqlparser.FuncExpr:
		if e.Exprs != nil {
			for _, selExpr := range e.Exprs {
				if ae, ok := selExpr.(*sqlparser.AliasedExpr); ok {
					collectColNameAliases(ae.Expr, aliases)
				}
			}
		}
	case *sqlparser.CaseExpr:
		if e.Expr != nil {
			collectColNameAliases(e.Expr, aliases)
		}
		for _, when := range e.Whens {
			collectColNameAliases(when.Cond, aliases)
			collectColNameAliases(when.Val, aliases)
		}
		if e.Else != nil {
			collectColNameAliases(e.Else, aliases)
		}
	}
}

// collectSelectColNames collects raw column names for duplicate detection
func collectSelectColNames(exprs sqlparser.SelectExprs, counts map[string]int) {
	for _, expr := range exprs {
		if ae, ok := expr.(*sqlparser.AliasedExpr); ok {
			name := sqlparser.String(ae.Expr)
			counts[name]++
		}
	}
}

// collectDistinctTables 递归收集 FROM/JOIN 中涉及的不同表名（忽略别名）
func collectDistinctTables(expr sqlparser.TableExpr, tables map[string]bool) {
	switch e := expr.(type) {
	case *sqlparser.AliasedTableExpr:
		tableName := sqlparser.GetTableName(e.Expr).String()
		if tableName != "" {
			tables[strings.ToUpper(tableName)] = true
		}
	case *sqlparser.JoinTableExpr:
		collectDistinctTables(e.LeftExpr, tables)
		collectDistinctTables(e.RightExpr, tables)
	case *sqlparser.ParenTableExpr:
		for _, subExpr := range e.Exprs {
			collectDistinctTables(subExpr, tables)
		}
	}
}

// hasJoinClause checks if any FROM entry is a JOIN
func hasJoinClause(sel *sqlparser.Select) bool {
	for _, expr := range sel.From {
		if isJoinExpr(expr) {
			return true
		}
	}
	return false
}

// isJoinExpr recursively checks for JOIN nodes
func isJoinExpr(expr sqlparser.TableExpr) bool {
	switch e := expr.(type) {
	case *sqlparser.JoinTableExpr:
		return true
	case *sqlparser.ParenTableExpr:
		for _, subExpr := range e.Exprs {
			if isJoinExpr(subExpr) {
				return true
			}
		}
	}
	return false
}

// hasFilterIntent checks if the SQL text suggests filtering intent
func hasFilterIntent(sql string) bool {
	filterWords := []string{"包含", "含有", "过滤", "筛选", "查找", "搜索", "where", "filter", "like", "大于", "小于", "等于", "caftp", "相同", "一样", "范围", "期间"}
	lower := strings.ToLower(sql)
	for _, w := range filterWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

// ColumnRef 表示 SQL 中的字段引用（表别名.字段名）
type ColumnRef struct {
	TableAlias string // 表别名（如 p, d）
	ColumnName string // 字段名（如 val, PARAM_VALUE）
}

// ExtractColumnRefs 从 SQL AST 中提取所有字段引用（SELECT/WHERE/JOIN ON/GROUP BY/ORDER BY/HAVING）
func ExtractColumnRefs(sqlStr string) []ColumnRef {
	stmt, err := sqlparser.Parse(sqlStr)
	if err != nil {
		return nil
	}
	sel, ok := stmt.(*sqlparser.Select)
	if !ok {
		return nil
	}

	var refs []ColumnRef
	seen := make(map[string]bool)

	collectExprRefs := func(expr sqlparser.Expr) {
		walkExprForColRefs(expr, func(ref ColumnRef) {
			key := ref.TableAlias + "." + ref.ColumnName
			if !seen[key] {
				seen[key] = true
				refs = append(refs, ref)
			}
		})
	}

	// SELECT columns
	for _, expr := range sel.SelectExprs {
		if ae, ok := expr.(*sqlparser.AliasedExpr); ok {
			collectExprRefs(ae.Expr)
		}
	}

	// WHERE
	if sel.Where != nil {
		collectExprRefs(sel.Where.Expr)
	}

	// JOIN ON conditions
	walkFromForJoinConditions(sel.From, func(ref ColumnRef) {
		key := ref.TableAlias + "." + ref.ColumnName
		if !seen[key] {
			seen[key] = true
			refs = append(refs, ref)
		}
	})

	// GROUP BY
	for _, expr := range sel.GroupBy {
		collectExprRefs(expr)
	}

	// HAVING
	if sel.Having != nil {
		collectExprRefs(sel.Having.Expr)
	}

	// ORDER BY
	for _, order := range sel.OrderBy {
		collectExprRefs(order.Expr)
	}

	return refs
}

// walkExprForColRefs 递归遍历表达式，提取 ColName 类型的字段引用
func walkExprForColRefs(expr sqlparser.Expr, fn func(ColumnRef)) {
	switch e := expr.(type) {
	case *sqlparser.ColName:
		alias := e.Qualifier.Name.String()
		colName := e.Name.String()
		if colName != "" && colName != "*" {
			fn(ColumnRef{TableAlias: alias, ColumnName: colName})
		}
	case *sqlparser.BinaryExpr:
		walkExprForColRefs(e.Left, fn)
		walkExprForColRefs(e.Right, fn)
	case *sqlparser.UnaryExpr:
		walkExprForColRefs(e.Expr, fn)
	case *sqlparser.FuncExpr:
		if e.Exprs != nil {
			for _, selExpr := range e.Exprs {
				if ae, ok := selExpr.(*sqlparser.AliasedExpr); ok {
					walkExprForColRefs(ae.Expr, fn)
				}
			}
		}
	case *sqlparser.CaseExpr:
		if e.Expr != nil {
			walkExprForColRefs(e.Expr, fn)
		}
		for _, when := range e.Whens {
			walkExprForColRefs(when.Cond, fn)
			walkExprForColRefs(when.Val, fn)
		}
		if e.Else != nil {
			walkExprForColRefs(e.Else, fn)
		}
	case *sqlparser.ComparisonExpr:
		walkExprForColRefs(e.Left, fn)
		walkExprForColRefs(e.Right, fn)
	case *sqlparser.IsExpr:
		walkExprForColRefs(e.Expr, fn)
	case *sqlparser.NotExpr:
		walkExprForColRefs(e.Expr, fn)
	case *sqlparser.ParenExpr:
		walkExprForColRefs(e.Expr, fn)
	case *sqlparser.RangeCond:
		walkExprForColRefs(e.Left, fn)
		walkExprForColRefs(e.From, fn)
		walkExprForColRefs(e.To, fn)
	}
}

// walkFromForJoinConditions 遍历 FROM/JOIN，提取 ON 条件中的字段引用
func walkFromForJoinConditions(from sqlparser.TableExprs, fn func(ColumnRef)) {
	for _, expr := range from {
		walkTableExprForJoinConditions(expr, fn)
	}
}

func walkTableExprForJoinConditions(expr sqlparser.TableExpr, fn func(ColumnRef)) {
	switch e := expr.(type) {
	case *sqlparser.JoinTableExpr:
		walkExprForColRefs(e.Condition.On, fn)
		walkTableExprForJoinConditions(e.LeftExpr, fn)
		walkTableExprForJoinConditions(e.RightExpr, fn)
	case *sqlparser.ParenTableExpr:
		for _, subExpr := range e.Exprs {
			walkTableExprForJoinConditions(subExpr, fn)
		}
	}
}

// BuildAliasToTableMap 从 SQL 的 FROM/JOIN 中提取别名到表名的映射
func BuildAliasToTableMap(sqlStr string) map[string]string {
	stmt, err := sqlparser.Parse(sqlStr)
	if err != nil {
		return nil
	}
	sel, ok := stmt.(*sqlparser.Select)
	if !ok {
		return nil
	}

	aliasMap := make(map[string]string)
	for _, expr := range sel.From {
		collectAliasToTable(expr, aliasMap)
	}
	return aliasMap
}

func collectAliasToTable(expr sqlparser.TableExpr, aliasMap map[string]string) {
	switch e := expr.(type) {
	case *sqlparser.AliasedTableExpr:
		tableName := sqlparser.GetTableName(e.Expr).String()
		aliasName := e.As.String()
		if aliasName != "" {
			aliasMap[strings.ToUpper(aliasName)] = tableName
			aliasMap[aliasName] = tableName
		}
		if tableName != "" {
			aliasMap[strings.ToUpper(tableName)] = tableName
			aliasMap[tableName] = tableName
		}
	case *sqlparser.JoinTableExpr:
		collectAliasToTable(e.LeftExpr, aliasMap)
		collectAliasToTable(e.RightExpr, aliasMap)
	case *sqlparser.ParenTableExpr:
		for _, subExpr := range e.Exprs {
			collectAliasToTable(subExpr, aliasMap)
		}
	}
}

// FieldIssue SQL 字段校验问题
type FieldIssue struct {
	TableAlias string // 表别名
	TableName  string // 实际表名
	FieldName  string // 字段名
	IssueType  string // "missing_field" | "missing_table" | "ok"
	Message    string // 问题描述
}

// ValidateSQLFields 对 SQL 中的每个字段引用做确定性 DDL 校验
// 这是"最后一道防线"：即使 LLM 产生了幻觉，也能在返回用户前捕获
// tableFields: tableName -> {lowercase -> originalCase}
func ValidateSQLFields(sqlStr string, tableFields map[string]map[string]string) []FieldIssue {
	var issues []FieldIssue

	refs := ExtractColumnRefs(sqlStr)
	if len(refs) == 0 {
		// fail-closed：refs 为空可能是解析失败或非 SELECT 语句（UNION/INTO 等），
		// 旧实现静默返回空 issues（等价于"校验通过"），让幻觉字段最可能出现的复杂 SQL 被放过。
		// 这里尝试解析判定原因，若确实无法解析则返回警告让上层决策。
		stmt, parseErr := sqlparser.Parse(sqlStr)
		if parseErr != nil {
			issues = append(issues, FieldIssue{
				FieldName: "",
				IssueType: "parse_error",
				Message:   "SQL 解析失败，无法验证字段引用，请检查 SQL 语法",
			})
		} else if _, ok := stmt.(*sqlparser.Select); !ok {
			issues = append(issues, FieldIssue{
				FieldName: "",
				IssueType: "unsupported_statement",
				Message:   "非 SELECT 语句（如 UNION/INSERT），字段校验已降级",
			})
		}
		return issues
	}

	aliasMap := BuildAliasToTableMap(sqlStr)

	for _, ref := range refs {
		if ref.ColumnName == "*" {
			continue
		}

		tableName := ""
		if ref.TableAlias != "" {
			tableName = aliasMap[strings.ToUpper(ref.TableAlias)]
			if tableName == "" {
				tableName = aliasMap[ref.TableAlias]
			}
		}

		// 无法确定表的字段，跳过（已在 backwardValidateAndFix 中处理）
		if tableName == "" {
			continue
		}

		fields, ok := tableFields[strings.ToLower(tableName)]
		if !ok {
			issues = append(issues, FieldIssue{
				TableAlias: ref.TableAlias,
				TableName:  tableName,
				FieldName:  ref.ColumnName,
				IssueType:  "missing_table",
				Message:    fmt.Sprintf("表 %s 不在知识库中，请先通过「Schema导入」功能导入该表的DDL，或调用 /api/v1/text2sql/scan-docs 扫描文档中的DDL", tableName),
			})
			continue
		}

		if _, ok := fields[strings.ToLower(ref.ColumnName)]; !ok {
			issues = append(issues, FieldIssue{
				TableAlias: ref.TableAlias,
				TableName:  tableName,
				FieldName:  ref.ColumnName,
				IssueType:  "missing_field",
				Message:    fmt.Sprintf("字段 %s.%s 在表 %s 的 DDL 中不存在", ref.TableAlias, ref.ColumnName, tableName),
			})
		}
	}

	return issues
}
