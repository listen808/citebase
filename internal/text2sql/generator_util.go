package text2sql

import (
	"context"

	logx "github.com/listen808/citebase/internal/log"
	"regexp"
	"strings"
)

// Text2SQL 生成器：SQL 清洗、关键字判定、字符串工具。

// extractKeywords 从问题中提取关键词
func extractKeywords(text string) []string {
	// P1增强：先提取完整的下划线命名标识符（表名/列名），避免被标点替换破坏
	// 例如 "bm_config_version" 在标点替换后会变成 "bm config version"，丢失完整表名
	tableNameRe := regexp.MustCompile(`[a-z][a-zA-Z0-9_]{3,}`)
	identifiers := tableNameRe.FindAllString(strings.ToLower(text), -1)

	text = strings.ToLower(text)
	text = regexp.MustCompile(`[[:punct:]]`).ReplaceAllString(text, " ")

	var tokens []string

	words := strings.Fields(text)
	for _, w := range words {
		w = strings.TrimSpace(w)
		if len(w) >= 2 {
			tokens = append(tokens, w)
		}
	}

	chineseRe := regexp.MustCompile(`[\p{Han}]{2,}`)
	chineseSegments := chineseRe.FindAllString(text, -1)
	for _, seg := range chineseSegments {
		if len([]rune(seg)) <= 6 {
			tokens = append(tokens, seg)
		}
		runes := []rune(seg)
		for i := 0; i+1 < len(runes); i++ {
			tokens = append(tokens, string(runes[i:i+2]))
		}
		for i := 0; i+2 < len(runes); i++ {
			tokens = append(tokens, string(runes[i:i+3]))
		}
	}

	camelRe := regexp.MustCompile(`[a-z][a-z0-9_]*`)
	camelTokens := camelRe.FindAllString(text, -1)
	for _, t := range camelTokens {
		for _, part := range strings.Split(t, "_") {
			if len(part) >= 2 {
				tokens = append(tokens, part)
			}
		}
	}

	stopWords := map[string]bool{
		"查询": true, "数据": true, "一个": true, "这个": true, "那个": true,
		"的": true, "了": true, "是": true, "在": true, "有": true,
		"和": true, "与": true, "或": true, "把": true, "被": true,
		"让": true, "将": true, "会": true, "能": true, "可": true,
		"要": true, "想": true, "请": true, "给": true, "对": true,
		"什么": true, "怎么": true, "哪些": true, "多少": true,
		"可以": true, "需要": true, "已经": true, "应该": true,
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"in": true, "on": true, "at": true, "to": true, "for": true,
		"of": true, "and": true, "or": true, "with": true, "from": true,
		"show": true, "get": true, "find": true, "list": true, "all": true,
		"select": true, "where": true, "having": true, "group": true,
	}

	var keywords []string
	seen := make(map[string]bool)

	// P1增强：优先添加完整标识符，确保表名不被拆散，同时过滤停用词
	for _, ident := range identifiers {
		if !seen[ident] && !stopWords[ident] {
			seen[ident] = true
			keywords = append(keywords, ident)
		}
	}

	for _, w := range tokens {
		w = strings.TrimSpace(w)
		if len(w) < 2 || stopWords[w] || seen[w] {
			continue
		}
		seen[w] = true
		keywords = append(keywords, w)
	}
	return keywords
}

// stripCommonAffixes 去除常见的前后缀
func cleanTableName(name string) string {
	name = strings.Trim(name, "\"'`")
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		part := name[idx+1:]
		part = strings.Trim(part, "\"'`")
		if part != "" {
			return part
		}
	}
	return name
}

func cleanColumnName(name string) string {
	name = strings.Trim(name, "\"'`")
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		part := name[idx+1:]
		part = strings.Trim(part, "\"'`")
		if part != "" {
			return part
		}
	}
	return name
}

func sanitizeSQL(sql string) string {
	sql = strings.ReplaceAll(sql, "\\\r\n", " ")
	sql = strings.ReplaceAll(sql, "\\\n", " ")
	backslashEOL := regexp.MustCompile(`\\\s*\n`)
	sql = backslashEOL.ReplaceAllString(sql, " ")
	re := regexp.MustCompile(`"[A-Za-z_][A-Za-z0-9_]*"\."[A-Za-z_][A-Za-z0-9_]*"`)
	sql = re.ReplaceAllStringFunc(sql, func(match string) string {
		parts := strings.SplitN(match, ".", 2)
		if len(parts) == 2 {
			table := strings.Trim(parts[1], "\"'`")
			return table
		}
		return match
	})

	// P1: 自动移除冗余 CAST(xxx AS CHAR) — MySQL 对数值类型 LIKE 会隐式转换
	// LLM 经常生成 CAST(b.CFG_VER_ID AS CHAR) LIKE '800%'
	// 实际上 MySQL 中 b.CFG_VER_ID LIKE '800%' 可以直接工作
	castRe := regexp.MustCompile(`(?i)CAST\(\s*(\w+\.\w+)\s+AS\s+CHAR(?:\(\d+\))?\s*\)`)
	if castRe.MatchString(sql) {
		logx.Infof("[sanitizeSQL] Removing redundant CAST(... AS CHAR) — MySQL handles numeric LIKE implicitly")
		sql = castRe.ReplaceAllString(sql, "$1")
	}

	sql = strings.TrimRight(sql, " \t\n\r")
	sql = strings.TrimRight(sql, ";")
	sql = strings.TrimSpace(sql)
	if sql != "" && !strings.HasSuffix(sql, ";") {
		sql += ";"
	}
	return sql
}

func stripCommonAffixes(s string) string {
	// 去除常见前缀
	prefixes := []string{"param_", "cfg_", "sys_", "bm_", "data_", "org_", "status_"}
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			s = strings.TrimPrefix(s, p)
			break
		}
	}
	// 去除常见后缀
	suffixes := []string{"_id", "_code", "_name", "_value", "_val", "_desc", "_cd", "_date", "_idx"}
	for _, suf := range suffixes {
		if strings.HasSuffix(s, suf) {
			s = strings.TrimSuffix(s, suf)
			break
		}
	}
	return s
}

// levenshteinDistance 计算两个字符串的编辑距离
func levenshteinDistance(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	// 优化：使用单行数组
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// postProcessSQL 统一的 SQL 后处理：Self-Reflection + 后向字段校验 + 确定性字段验证
// 返回处理后的 SQL 和是否有致命错误。
// ctx 携带调用者身份，供 selfReflect 的 LLM 调用走 provider 层 Token 配额预检。
func (g *Generator) postProcessSQL(ctx context.Context, text, sql, schemaContext, relationsContext string, kb *KnowledgeBase, schemaTables []string) (string, []string, bool) {
	return g.postProcessSQLEx(ctx, text, sql, schemaContext, relationsContext, kb, schemaTables, false)
}

func (g *Generator) postProcessSQLEx(ctx context.Context, text, sql, schemaContext, relationsContext string, kb *KnowledgeBase, schemaTables []string, skipReflect bool) (string, []string, bool) {
	var warnings []string
	hasError := false

	sql = sanitizeSQL(sql)
	logx.Infof("[Text2SQL-DIAG] Step2a sanitizeSQL: sql=%q, len=%d", sql, len(sql))

	// P1: selfReflect 条件化 — 仅在后向校验发现字段问题或明确要求时才调用
	// 原逻辑：每次都调 LLM 自反思（18K tokens/次），成功路径浪费巨大
	// 新逻辑：先做快速规则校验，仅在发现问题时调用 selfReflect
	if skipReflect {
		logx.Infof("[Text2SQL-DIAG] Step2b selfReflect: SKIPPED (multi-turn)")
	} else {
		// 先跑后向校验，发现字段问题才调用 selfReflect
		fixedSQL, fieldWarnings := g.backwardValidateAndFix(sql, kb)
		if fixedSQL != sql {
			logx.Infof("[Text2SQL-DIAG] Step2c backwardValidateAndFix: BEFORE=%q AFTER=%q", sql, fixedSQL)
			sql = fixedSQL
		}
		warnings = append(warnings, fieldWarnings...)

		// 扩展 selfReflect 触发条件：除了字段问题，还覆盖语义问题
		needsReflection := false
		for _, w := range fieldWarnings {
			if strings.Contains(w, "不存在") || strings.Contains(w, "missing") {
				needsReflection = true
				break
			}
		}
		upperSQL := strings.ToUpper(sql)
		// 多表查询无显式 JOIN 条件
		if !needsReflection {
			tableCount := strings.Count(upperSQL, " FROM ") + strings.Count(upperSQL, " JOIN ")
			if tableCount >= 2 && !strings.Contains(upperSQL, " ON ") {
				needsReflection = true
			}
		}
		// 聚合查询缺少 GROUP BY
		if !needsReflection {
			hasAgg := strings.Contains(upperSQL, "COUNT(") || strings.Contains(upperSQL, "SUM(") ||
				strings.Contains(upperSQL, "AVG(") || strings.Contains(upperSQL, "MAX(") || strings.Contains(upperSQL, "MIN(")
			hasGroupBy := strings.Contains(upperSQL, "GROUP BY")
			if hasAgg && !hasGroupBy && strings.Contains(upperSQL, " FROM ") {
				needsReflection = true
			}
		}

		if needsReflection {
			logx.Infof("[Text2SQL-DIAG] Step2b selfReflect: triggered (field/semantic issues detected)")
			corrected, err := g.selfReflect(ctx, text, sql, schemaContext, relationsContext)
			if err == nil && corrected != "" && corrected != sql {
				logx.Infof("[Text2SQL-DIAG] Step2b selfReflect: BEFORE=%q AFTER=%q", sql, corrected)
				sql = corrected
			}
		} else {
			logx.Infof("[Text2SQL-DIAG] Step2b selfReflect: SKIPPED (no field/semantic issues detected, SQL is clean)")
		}
	}

	// 3. 确定性字段校验（最后一道防线）：即使自动修正后，再次逐字段验证
	tableFields := buildTableFieldMap(kb)
	fieldIssues := ValidateSQLFields(sql, tableFields)
	for _, fi := range fieldIssues {
		if fi.IssueType == "missing_field" || fi.IssueType == "missing_table" {
			warnings = append(warnings, fi.Message)
			hasError = true
			logx.Infof("[Text2SQL-DIAG] Step2d ValidateSQLFields ERROR: type=%s table=%s field=%s msg=%s", fi.IssueType, fi.TableName, fi.FieldName, fi.Message)
		}
	}

	issues := ValidateSQLStructure(sql, schemaTables)
	for _, iss := range issues {
		logx.Infof("[Text2SQL-DIAG] Step2e ValidateSQLStructure: type=%s msg=%s", iss.Type, iss.Message)
		if iss.Type == "error" {
			hasError = true
		}
		warnings = append(warnings, iss.Message)
	}

	// P2-1: 跨方言 SQL 改写（Oracle ROWNUM / PostgreSQL :: 等非 MySQL 语法）
	if g.dialectRewriter != nil {
		rewritten, rewriteWarnings := g.dialectRewriter.Rewrite(sql)
		if rewritten != sql {
			sql = rewritten
			warnings = append(warnings, rewriteWarnings...)
			logx.Infof("[Text2SQL-DIAG] Step2f DialectRewrite: applied %d rules", len(rewriteWarnings))
		}
	}

	logx.Infof("[Text2SQL-DIAG] postProcessSQL FINAL: hasError=%v sql=%q", hasError, sql)

	return sql, warnings, hasError
}

// isSQLKeyword 判断是否是SQL关键字（不是表名）
func isSQLKeyword(word string) bool {
	keywords := map[string]bool{
		"SELECT": true, "FROM": true, "WHERE": true, "JOIN": true,
		"INNER": true, "LEFT": true, "RIGHT": true, "OUTER": true,
		"ON": true, "AND": true, "OR": true, "NOT": true,
		"INSERT": true, "UPDATE": true, "DELETE": true, "CREATE": true,
		"DROP": true, "ALTER": true, "TABLE": true, "INDEX": true,
		"AS": true, "INTO": true, "VALUES": true, "SET": true,
		"GROUP": true, "BY": true, "ORDER": true, "HAVING": true,
		"LIMIT": true, "OFFSET": true, "UNION": true, "ALL": true,
		"DISTINCT": true, "EXISTS": true, "IN": true, "BETWEEN": true,
		"LIKE": true, "IS": true, "NULL": true, "TRUE": true, "FALSE": true,
		"ASC": true, "DESC": true, "CASE": true, "WHEN": true, "THEN": true,
		"ELSE": true, "END": true, "IF": true, "COUNT": true, "SUM": true,
		"AVG": true, "MAX": true, "MIN": true,
	}
	return keywords[word]
}
