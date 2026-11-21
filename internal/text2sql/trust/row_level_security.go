package trust

import (
	"context"
	"fmt"
	"sort"
	"strings"

	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
)

// RowLevelSecurity 行级安全过滤器
// 根据用户权限自动注入SQL WHERE条件，确保用户只能访问有权限的数据
type RowLevelSecurity struct {
	spaceRepo *workspacerepo.SpaceRepository
	config    RLSConfig
}

type RLSConfig struct {
	Enabled     bool                    `yaml:"enabled" json:"enabled"`
	DefaultMode string                  `yaml:"default_mode" json:"default_mode"` // "strict", "permissive", "disabled"
	TableRules  map[string]RLSTableRule `yaml:"table_rules" json:"table_rules"`
}

type RLSTableRule struct {
	Column     string `yaml:"column" json:"column"`           // 过滤列名（如 space_id, user_id）
	Source     string `yaml:"source" json:"source"`           // 值来源: "space_id", "user_id", "fixed"
	FixedValue string `yaml:"fixed_value" json:"fixed_value"` // 固定值（Source=fixed时使用）
}

func DefaultRLSConfig() RLSConfig {
	return RLSConfig{
		Enabled:     true,
		DefaultMode: "strict",
		TableRules: map[string]RLSTableRule{
			"documents": {Column: "space_id", Source: "space_id"},
			"users":     {Column: "id", Source: "user_id"},
		},
	}
}

func NewRowLevelSecurity(spaceRepo *workspacerepo.SpaceRepository, cfg RLSConfig) *RowLevelSecurity {
	return &RowLevelSecurity{
		spaceRepo: spaceRepo,
		config:    cfg,
	}
}

// InjectRowLevelFilter 为SQL注入行级安全过滤条件
//
// P0 安全修复（fail-closed）：旧实现遇到无法识别的表引用形态（`FROM db.documents`、
// 反引号、换行/多空格）时静默跳过注入并返回 nil error，等价于放行未过滤 SQL；
// 现在只要某个受保护表被引用却无法完成注入，就直接返回错误，由调用方拒绝执行。
func (r *RowLevelSecurity) InjectRowLevelFilter(ctx context.Context, sqlStr string, userID int, spaceID int, userSpaces []int) (string, error) {
	if !r.config.Enabled || r.config.DefaultMode == "disabled" {
		return sqlStr, nil
	}

	// 只对SELECT语句注入行级过滤（掩码后判断，避免字符串字面量干扰）
	head := strings.ToUpper(strings.TrimSpace(maskSQLLiterals(sqlStr)))
	if !strings.HasPrefix(head, "SELECT") {
		return sqlStr, nil
	}

	// 收集用户可访问的空间ID
	if len(userSpaces) == 0 && spaceID > 0 {
		userSpaces = []int{spaceID}
	}

	// 如果用户没有任何可访问空间，注入永假条件
	if len(userSpaces) == 0 && userID > 0 {
		return r.injectFalseCondition(sqlStr)
	}

	refs, sawFromJoin := r.referencedTables(sqlStr)
	if len(refs) == 0 && sawFromJoin {
		// 有 FROM/JOIN 却解析不出任何表名：无法证明没有触及受保护表，拒绝执行
		return sqlStr, fmt.Errorf("无法解析 SQL 的表引用，拒绝执行未过滤的查询")
	}

	// 对每个配置的表规则注入过滤条件
	// 按表名排序遍历：map 的随机顺序会让同一条 SQL 每次注入出的文本不同，
	// 不利于审计与结果比对（注入语义等价，但输出应可复现）。
	modifiedSQL := sqlStr
	tables := make([]string, 0, len(r.config.TableRules))
	for table := range r.config.TableRules {
		tables = append(tables, table)
	}
	sort.Strings(tables)
	for _, table := range tables {
		rule := r.config.TableRules[table]
		if !refs[strings.ToLower(strings.TrimSpace(table))] {
			continue
		}
		filter := r.buildFilterCondition(rule, userID, userSpaces)
		if filter == "" {
			return sqlStr, fmt.Errorf("RLS 规则 %s 的 source=%q 无法生成过滤条件，拒绝执行", table, rule.Source)
		}
		injected, err := r.injectWhereCondition(modifiedSQL, filter)
		if err != nil {
			return sqlStr, fmt.Errorf("表 %s 需要行级过滤但注入失败: %w", table, err)
		}
		modifiedSQL = injected
	}

	return modifiedSQL, nil
}

// CheckAccess 检查用户是否有权访问指定空间的数据
func (r *RowLevelSecurity) CheckAccess(ctx context.Context, userID int, spaceID int) bool {
	if spaceID <= 0 {
		return true
	}

	space, err := r.spaceRepo.FindByID(spaceID)
	if err != nil || space == nil {
		return false
	}

	if space.IsPublic() || (userID > 0 && space.IsLoggedInVisible()) {
		return true
	}

	if userID <= 0 {
		return false
	}

	member, err := r.spaceRepo.FindSpaceUser(spaceID, userID)
	return err == nil && member != nil
}

// GetUserAccessibleSpaces 获取用户可访问的空间列表
func (r *RowLevelSecurity) GetUserAccessibleSpaces(ctx context.Context, userID int) []int {
	if userID <= 0 {
		return nil
	}

	allSpaces, _, err := r.spaceRepo.FindAll(0, 10000, "", "", "")
	if err != nil {
		return nil
	}

	var accessible []int
	for _, space := range allSpaces {
		if space.IsPublic() || space.IsLoggedInVisible() {
			accessible = append(accessible, space.ID)
			continue
		}
		member, err := r.spaceRepo.FindSpaceUser(space.ID, userID)
		if err == nil && member != nil {
			accessible = append(accessible, space.ID)
		}
	}

	return accessible
}

// buildFilterCondition 构建过滤条件
func (r *RowLevelSecurity) buildFilterCondition(rule RLSTableRule, userID int, userSpaces []int) string {
	switch rule.Source {
	case "space_id":
		if len(userSpaces) == 0 {
			return fmt.Sprintf("%s IN (0)", rule.Column) // 永假
		}
		ids := make([]string, len(userSpaces))
		for i, id := range userSpaces {
			ids[i] = fmt.Sprintf("%d", id)
		}
		return fmt.Sprintf("%s IN (%s)", rule.Column, strings.Join(ids, ","))
	case "user_id":
		if userID <= 0 {
			return fmt.Sprintf("%s = 0", rule.Column) // 永假
		}
		return fmt.Sprintf("%s = %d", rule.Column, userID)
	case "fixed":
		return fmt.Sprintf("%s = '%s'", rule.Column, rule.FixedValue)
	default:
		return ""
	}
}

// referencedTables 提取 SQL 中 FROM / JOIN 之后出现的表名集合（小写、去反引号/双引号/
// 中括号包裹、去 `db.` / `schema.` 限定名前缀、大小写不敏感、允许任意空白与换行）。
// 第二个返回值表示 SQL 里确实出现过 FROM/JOIN，供上层判定“解析不出表名”这种异常形态。
//
// 它取代了旧的 tableReferenced：后者只做 `strings.Contains(upperSQL, " FROM "+表名)`
// 这类单空格拼接匹配，遇到 `FROM db.documents`、带反引号的表名、换行或多空格
// 就漏判，随后静默不注入（fail-open）。
func (r *RowLevelSecurity) referencedTables(sqlStr string) (map[string]bool, bool) {
	masked := maskSQLLiterals(sqlStr)
	up := strings.ToUpper(masked)
	toks := sqlWordTokens(up)

	sawFromJoin := false
	refs := make(map[string]bool)
	for i, tok := range toks {
		if tok.text != "FROM" && tok.text != "JOIN" && tok.text != "STRAIGHT_JOIN" {
			continue
		}
		sawFromJoin = true
		// FROM 后可以是逗号分隔的多表（`FROM a, b`）；逐个取每段第一个名字。
		// 子查询/派生表（`FROM (SELECT ...)`）的下一个词是 SELECT，会被终止词表挡住，
		// 其内层 FROM 由本循环在后续词元上单独扫到。
		for j := i + 1; j < len(toks); {
			cur := toks[j]
			if rlsStopWords[cur.text] {
				break
			}
			name, next := qualifiedNameAt(up, toks, j)
			if name != "" {
				refs[name] = true
			}
			// 跳过别名等后续词，直到逗号（新表项）或子句关键字
			j = next
			for j < len(toks) {
				if toks[j].text == "," {
					j++
					break
				}
				if rlsStopWords[toks[j].text] {
					j = len(toks)
					break
				}
				j++
			}
		}
	}
	return refs, sawFromJoin
}

// qualifiedNameAt 从第 i 个词元开始读取 `a.b.table` 形式的限定名，返回最后一段（小写）
// 以及消费到的下一个词元下标。反引号/双引号/中括号包裹的名字会被还原。
func qualifiedNameAt(up string, toks []sqlToken, i int) (string, int) {
	last := strings.Trim(up[toks[i].start:toks[i].end], "`\"'[]")
	j := i
	for j+1 < len(toks) {
		gap := up[toks[j].end:toks[j+1].start]
		if strings.TrimSpace(gap) != "." {
			break
		}
		j++
		last = strings.Trim(up[toks[j].start:toks[j].end], "`\"'[]")
	}
	return strings.ToLower(strings.TrimSpace(last)), j + 1
}

// rlsStopWords FROM/JOIN 表引用列表的终止关键字
var rlsStopWords = map[string]bool{
	"WHERE": true, "GROUP": true, "ORDER": true, "HAVING": true, "LIMIT": true,
	"UNION": true, "INTERSECT": true, "EXCEPT": true, "ON": true, "USING": true,
	"SELECT": true, "SET": true, "VALUES": true, "FOR": true, "LOCK": true,
	"AS": true, "OFFSET": true,
}

// injectWhereCondition 注入WHERE条件
//
// 旧实现在原 WHERE 末尾直接拼 ` AND (cond)` 而没给原条件加括号：
// `WHERE a=1 OR b=2` 会变成 `WHERE a=1 OR b=2 AND (cond)`，由于 AND 的优先级高于 OR，
// 过滤条件只约束了 `b=2` 这一支，`a=1` 的行全部逃逸（OR 优先级绕过）。
// 现在统一注入 `WHERE (原条件) AND (RLS条件)`；无 WHERE 时注入 `WHERE (RLS条件)`。
// 无法安全定位插入点（括号不配对、找不到 FROM/WHERE 子句）时返回错误（fail-closed）。
func (r *RowLevelSecurity) injectWhereCondition(sqlStr, condition string) (string, error) {
	masked := maskSQLLiterals(sqlStr)
	up := strings.ToUpper(masked)
	depths, err := parenDepths(masked)
	if err != nil {
		return sqlStr, err
	}

	// 顶层集合运算（UNION / INTERSECT / EXCEPT）会把一条语句拆成多个 SELECT 块，
	// 单点注入只会过滤到其中一个分支，其余分支的数据照样能查出来。
	// 这里不再赌“只过滤左支”，直接返回错误让调用方拒绝执行（fail-closed）。
	if idx := firstTopLevelWordAny(up, depths, []string{"UNION", "INTERSECT", "EXCEPT"}); idx >= 0 {
		return sqlStr, fmt.Errorf("SQL 含顶层集合运算（UNION/INTERSECT/EXCEPT），无法为每个分支注入行级过滤，拒绝执行")
	}

	// 已有顶层 WHERE 子句：把原条件整体括号化后再 AND 上过滤条件
	if idx := firstTopLevelWord(up, depths, "WHERE"); idx >= 0 {
		end := clauseEndIndex(up, depths, idx+len("WHERE"))
		origCond := strings.TrimSpace(sqlStr[idx+len("WHERE") : end])
		if origCond == "" {
			return sqlStr, fmt.Errorf("WHERE 子句为空，无法安全注入行级过滤条件")
		}
		out := sqlStr[:idx] + " WHERE (" + origCond + ") AND (" + condition + ")"
		if tail := strings.TrimSpace(sqlStr[end:]); tail != "" {
			out += " " + tail
		}
		return out, nil
	}

	// 没有 WHERE 子句：在顶层 FROM/JOIN 区域末尾插入
	fromIdx := firstTopLevelWord(up, depths, "FROM")
	if fromIdx < 0 {
		return sqlStr, fmt.Errorf("SQL 中未定位到顶层 FROM 子句，无法注入行级过滤条件")
	}
	end := clauseEndIndex(up, depths, fromIdx)
	out := strings.TrimRight(sqlStr[:end], " \t\r\n") + " WHERE (" + condition + ")"
	if tail := strings.TrimSpace(sqlStr[end:]); tail != "" {
		out += " " + tail
	}
	return out, nil
}

// clauseEndIndex 找到 start 之后第一个顶层子句边界（GROUP BY/ORDER BY/HAVING/LIMIT/
// UNION 等集合运算/锁定子句/语句结束），允许任意空白与换行
func clauseEndIndex(up string, depths []int, start int) int {
	if start < 0 {
		start = 0
	}
	end := len(up)
	for _, tok := range sqlWordTokens(up[start:]) {
		abs := start + tok.start
		if depths[abs] != 0 {
			continue
		}
		switch tok.text {
		case "GROUP", "ORDER", "HAVING", "LIMIT", "UNION", "INTERSECT", "EXCEPT", "OFFSET", "FOR", "LOCK":
			return abs
		}
	}
	if idx := indexOfTopLevelRune(up, depths, ';', start); idx >= 0 {
		if idx < end {
			end = idx
		}
	}
	return end
}

// injectFalseCondition 注入永假条件（当用户无任何权限时）
// 与 injectWhereCondition 共用同一套子句定位逻辑，避免把 AND 1=0 直接贴到
// ORDER BY / LIMIT 之后，也避免 OR 优先级绕过。
func (r *RowLevelSecurity) injectFalseCondition(sqlStr string) (string, error) {
	return r.injectWhereCondition(sqlStr, "1=0")
}

// ============================================================
// SQL 词法辅助（掩码 / 词元 / 括号深度）
// ============================================================

// maskSQLLiterals 返回与 sql 等长的“掩码串”：单引号/双引号字符串字面量与注释
// （-- 、# 、/* */）的正文替换为空格，位置与原串严格一一对应，索引可直接映射回原串。
// 反引号标识符不掩（其中承载的正是表名/列名），仅在其内部出现转义时保留长度。
func maskSQLLiterals(sql string) string {
	out := []byte(sql)
	for i := 0; i < len(out); {
		switch c := out[i]; c {
		case '\'', '"':
			j := i + 1
			for j < len(out) {
				if out[j] == '\\' {
					out[j] = ' '
					if j+1 < len(out) {
						out[j+1] = ' '
					}
					j += 2
					continue
				}
				if out[j] == c {
					if j+1 < len(out) && out[j+1] == c { // 连续两个引号 = 转义
						out[j] = ' '
						out[j+1] = ' '
						j += 2
						continue
					}
					break
				}
				out[j] = ' '
				j++
			}
			i = j + 1
		case '-':
			if i+1 < len(out) && out[i+1] == '-' {
				i = maskLineComment(out, i)
			} else {
				i++
			}
		case '#':
			i = maskLineComment(out, i)
		case '/':
			if i+1 < len(out) && out[i+1] == '*' {
				out[i], out[i+1] = ' ', ' '
				j := i + 2
				for j < len(out) {
					if out[j] == '*' && j+1 < len(out) && out[j+1] == '/' {
						out[j], out[j+1] = ' ', ' '
						j += 2
						break
					}
					out[j] = ' '
					j++
				}
				i = j
			} else {
				i++
			}
		default:
			i++
		}
	}
	return string(out)
}

// maskLineComment 把 -- / # 之后的行注释正文替换为空格（保留换行），返回续扫位置
func maskLineComment(out []byte, i int) int {
	for i < len(out) && out[i] != '\n' {
		out[i] = ' '
		i++
	}
	return i
}

// sqlToken 词元在掩码串中的位置（[start, end)）
type sqlToken struct {
	text  string
	start int
	end   int
}

// sqlWordTokens 按 SQL 词法切出标识符/关键字词元（保留反引号内容以便提取表名）
func sqlWordTokens(s string) []sqlToken {
	var toks []sqlToken
	isWord := func(b byte) bool {
		return b == '_' || b == '$' || b == '@' ||
			(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
	}
	for i := 0; i < len(s); {
		if s[i] == '`' { // 反引号标识符整体作为一个词元
			j := i + 1
			for j < len(s) && s[j] != '`' {
				j++
			}
			if j+1 <= len(s) {
				toks = append(toks, sqlToken{text: strings.ToUpper(s[i : j+1]), start: i, end: j + 1})
			}
			i = j + 1
			continue
		}
		if isWord(s[i]) {
			j := i
			for j < len(s) && isWord(s[j]) {
				j++
			}
			toks = append(toks, sqlToken{text: s[i:j], start: i, end: j})
			i = j
			continue
		}
		i++
	}
	return toks
}

// parenDepths 计算每个字符处的括号深度；括号不配对时返回错误（fail-closed）
func parenDepths(masked string) ([]int, error) {
	depths := make([]int, len(masked)+1)
	depth := 0
	for i := 0; i < len(masked); i++ {
		depths[i] = depth
		switch masked[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("SQL 括号不配对：第 %d 个字符处出现多余的 ')'", i+1)
			}
		}
	}
	depths[len(masked)] = depth
	if depth != 0 {
		return nil, fmt.Errorf("SQL 括号不配对：缺少 %d 个 ')'", depth)
	}
	return depths, nil
}

// firstTopLevelWord 返回关键字在顶层（括号深度 0）首次出现的字节下标，找不到返回 -1
func firstTopLevelWord(up string, depths []int, word string) int {
	return firstTopLevelWordAny(up, depths, []string{word})
}

// firstTopLevelWordAny 返回任一关键字在顶层首次出现的字节下标，找不到返回 -1
func firstTopLevelWordAny(up string, depths []int, words []string) int {
	wanted := make(map[string]bool, len(words))
	for _, w := range words {
		wanted[w] = true
	}
	for _, tok := range sqlWordTokens(up) {
		if !wanted[tok.text] || depths[tok.start] != 0 {
			continue
		}
		return tok.start
	}
	return -1
}

// indexOfTopLevelRune 返回顶层（深度 0）首次出现某字符的下标，找不到返回 -1
func indexOfTopLevelRune(s string, depths []int, target byte, from int) int {
	for i := from; i < len(s); i++ {
		if s[i] == target && depths[i] == 0 {
			return i
		}
	}
	return -1
}
