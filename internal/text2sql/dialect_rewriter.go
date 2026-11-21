package text2sql

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// P2-1 (§4.3.1): 跨方言 SQL 改写
//
// 目标：LLM 生成的 SQL 可能混入非 MySQL 方言语法（Oracle ROWNUM、PostgreSQL ::、ILIKE 等），
//   本模块在 postProcessSQL 后做确定性改写，确保 SQL 能在目标数据库执行。
//
// 设计原则：
//   - 仅做确定性映射，不调用 LLM
//   - 按目标 DBType 选择改写规则集
//   - 原 SQL 已是目标方言时直接返回（无副作用）

// DialectRewriter 方言改写器
type DialectRewriter struct {
	targetDialect string // 目标方言：mysql / postgres / oracle
}

// NewDialectRewriter 创建方言改写器
func NewDialectRewriter(targetDialect string) *DialectRewriter {
	d := strings.ToLower(strings.TrimSpace(targetDialect))
	if d == "" {
		d = "mysql"
	}
	return &DialectRewriter{targetDialect: d}
}

// Rewrite 按目标方言改写 SQL
// 返回改写后的 SQL 与应用的改写规则列表（供 Trace/Warnings 使用）
func (dr *DialectRewriter) Rewrite(sql string) (string, []string) {
	if sql == "" {
		return sql, nil
	}
	var applied []string
	switch dr.targetDialect {
	case "mysql":
		sql, applied = dr.rewriteToMySQL(sql)
	case "postgres", "postgresql":
		sql, applied = dr.rewriteToPostgres(sql)
	case "oracle":
		// Oracle 方言保留原样，不做强制改写
		return sql, nil
	}
	return sql, applied
}

// rewriteToMySQL 将非 MySQL 方言改写为 MySQL 兼容语法
func (dr *DialectRewriter) rewriteToMySQL(sql string) (string, []string) {
	var applied []string
	original := sql

	// 1. Oracle ROWNUM <= N → LIMIT N
	//    匹配：WHERE ROWNUM <= 10 / WHERE ROWNUM < 10
	reRownum := regexp.MustCompile(`(?i)\bWHERE\s+ROWNUM\s*<=\s*(\d+)`)
	if reRownum.MatchString(sql) {
		m := reRownum.FindStringSubmatch(sql)
		sql = reRownum.ReplaceAllString(sql, "WHERE 1=1")
		sql += fmt.Sprintf(" LIMIT %s", m[1])
		applied = append(applied, fmt.Sprintf("Oracle ROWNUM <= %s → LIMIT %s", m[1], m[1]))
	}
	reRownumLt := regexp.MustCompile(`(?i)\bWHERE\s+ROWNUM\s*<\s*(\d+)`)
	if reRownumLt.MatchString(sql) {
		m := reRownumLt.FindStringSubmatch(sql)
		// ROWNUM < N 语义为前 N-1 行，对应 LIMIT N-1
		n, _ := strconv.Atoi(m[1])
		if n > 0 {
			n--
		}
		sql = reRownumLt.ReplaceAllString(sql, "WHERE 1=1")
		sql += fmt.Sprintf(" LIMIT %d", n)
		applied = append(applied, fmt.Sprintf("Oracle ROWNUM < %s → LIMIT %d", m[1], n))
	}

	// 2. PostgreSQL :: 类型转换 → CAST(... AS ...)
	//    匹配：'value'::text / column::int / expr::bigint
	reCast := regexp.MustCompile(`(?i)([\w.'"]+|'[^']*')::(\w+)`)
	for reCast.MatchString(sql) {
		m := reCast.FindStringSubmatch(sql)
		expr := m[1]
		typ := strings.ToLower(m[2])
		// MySQL 类型映射
		mysqlType := mapPostgresTypeToMySQL(typ)
		replacement := fmt.Sprintf("CAST(%s AS %s)", expr, mysqlType)
		sql = strings.Replace(sql, m[0], replacement, 1)
		applied = append(applied, fmt.Sprintf("PostgreSQL ::%s → CAST AS %s", typ, mysqlType))
	}

	// 3. PostgreSQL ILIKE → LIKE（MySQL 默认不区分大小写，等同 ILIKE）
	reILike := regexp.MustCompile(`(?i)\bILIKE\b`)
	if reILike.MatchString(sql) {
		sql = reILike.ReplaceAllString(sql, "LIKE")
		applied = append(applied, "PostgreSQL ILIKE → LIKE（MySQL 默认大小写不敏感）")
	}

	// 4. PostgreSQL ~ / ~* 正则匹配 → REGEXP / REGEXP_LIKE（MySQL 8.0+）
	//    简单场景：column ~ 'pattern' → column REGEXP 'pattern'
	reRegex := regexp.MustCompile(`(?i)([\w.]+)\s*~\s*'([^']+)'`)
	if reRegex.MatchString(sql) {
		sql = reRegex.ReplaceAllString(sql, "$1 REGEXP '$2'")
		applied = append(applied, "PostgreSQL ~ → REGEXP")
	}
	reRegexI := regexp.MustCompile(`(?i)([\w.]+)\s*~\*\s*'([^']+)'`)
	if reRegexI.MatchString(sql) {
		sql = reRegexI.ReplaceAllString(sql, "$1 REGEXP '$2'")
		applied = append(applied, "PostgreSQL ~* → REGEXP（大小写不敏感）")
	}

	// 5. Oracle SYSDATE → NOW()
	reSysdate := regexp.MustCompile(`(?i)\bSYSDATE\b`)
	if reSysdate.MatchString(sql) {
		sql = reSysdate.ReplaceAllString(sql, "NOW()")
		applied = append(applied, "Oracle SYSDATE → NOW()")
	}

	// 6. Oracle NVL(a, b) → IFNULL(a, b)
	reNVL := regexp.MustCompile(`(?i)\bNVL\s*\(`)
	if reNVL.MatchString(sql) {
		sql = reNVL.ReplaceAllString(sql, "IFNULL(")
		applied = append(applied, "Oracle NVL → IFNULL")
	}

	// 7. Oracle TO_DATE(s, 'fmt') → STR_TO_DATE(s, 'fmt')
	//    仅做函数名替换，格式串需用户自行适配（MySQL %Y-%m-%d vs Oracle YYYY-MM-DD）
	reToDate := regexp.MustCompile(`(?i)\bTO_DATE\s*\(`)
	if reToDate.MatchString(sql) {
		sql = reToDate.ReplaceAllString(sql, "STR_TO_DATE(")
		applied = append(applied, "Oracle TO_DATE → STR_TO_DATE（注意格式串差异）")
	}

	if sql == original {
		return sql, nil
	}
	return sql, applied
}

// rewriteToPostgres 将非 PostgreSQL 方言改写为 PostgreSQL 兼容语法
func (dr *DialectRewriter) rewriteToPostgres(sql string) (string, []string) {
	var applied []string
	original := sql

	// MySQL IFNULL → COALESCE
	reIfNull := regexp.MustCompile(`(?i)\bIFNULL\s*\(`)
	if reIfNull.MatchString(sql) {
		sql = reIfNull.ReplaceAllString(sql, "COALESCE(")
		applied = append(applied, "MySQL IFNULL → COALESCE")
	}

	// MySQL NOW() → CURRENT_TIMESTAMP（PG 也支持 NOW()，但 CURRENT_TIMESTAMP 更标准）
	// 此处不做强制改写，NOW() 在 PG 中可用

	// MySQL LIMIT N OFFSET M 语法已兼容 PG，无需改写
	// MySQL 反引号 ` → 双引号 "
	reBacktick := regexp.MustCompile("`")
	if reBacktick.MatchString(sql) {
		sql = reBacktick.ReplaceAllString(sql, "\"")
		applied = append(applied, "MySQL 反引号 → PostgreSQL 双引号")
	}

	if sql == original {
		return sql, nil
	}
	return sql, applied
}

// mapPostgresTypeToMySQL PostgreSQL 类型名 → MySQL 类型名映射
func mapPostgresTypeToMySQL(pgType string) string {
	switch strings.ToLower(pgType) {
	case "int", "int4", "integer":
		return "SIGNED"
	case "int8", "bigint":
		return "UNSIGNED"
	case "float", "float4", "real":
		return "DECIMAL"
	case "float8", "double":
		return "DECIMAL"
	case "text", "varchar", "char":
		return "CHAR"
	case "bool", "boolean":
		return "UNSIGNED"
	case "date":
		return "DATE"
	case "timestamp", "timestamptz":
		return "DATETIME"
	default:
		// 未知类型原样返回，让 MySQL 自行处理
		return strings.ToUpper(pgType)
	}
}
