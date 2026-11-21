package text2sql

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	logx "github.com/listen808/citebase/internal/log"

	modelai "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
)

// Text2SQL 生成器：术语表映射、字段纠错、相似度匹配、反向校验。

// generateFieldAliases 为字段生成用户可能使用的自然语言称呼
// 这是 RSL-SQL 论文中 "Schema Description" 增强的核心思想：
// 帮助 LLM 将用户的自然语言表述映射到 DDL 中的实际字段名
func generateFieldAliases(fieldName, comment string) []string {
	var aliases []string
	lower := strings.ToLower(fieldName)

	// 策略1: 去除常见前后缀后的核心名
	// 例如 PARAM_VALUE → VALUE, param_code → CODE
	core := lower
	prefixes := []string{"param_", "cfg_", "sys_", "bm_", "data_", "org_", "status_", "create_", "update_", "group_", "sort_"}
	for _, p := range prefixes {
		if strings.HasPrefix(core, p) {
			core = strings.TrimPrefix(core, p)
			break
		}
	}
	suffixes := []string{"_id", "_code", "_name", "_value", "_val", "_desc", "_cd", "_date", "_idx", "_staff", "_time"}
	for _, suf := range suffixes {
		if strings.HasSuffix(core, suf) {
			core = strings.TrimSuffix(core, suf)
			break
		}
	}
	if core != lower && len(core) >= 2 {
		aliases = append(aliases, core)
	}

	// 策略2: 中文注释中提取的关键词
	// 例如注释"系统参数值" → "参数值", "值"
	if comment != "" {
		// 提取注释末尾的核心词（通常是语义最浓缩的部分）
		commentWords := []string{"值", "名", "编码", "标识", "时间", "状态", "序号", "代码", "描述"}
		for _, w := range commentWords {
			if strings.Contains(comment, w) {
				aliases = append(aliases, w)
				break
			}
		}
	}

	// 策略3: 特殊映射规则
	// 针对 Oracle DDL 中常见缩写
	specialMap := map[string][]string{
		"param_val":  {"val", "value", "参数值", "值"},
		"param_code": {"code", "编码", "参数编码"},
		"param_id":   {"id", "标识", "参数标识"},
		"param_desc": {"desc", "描述", "说明"},
		"param_name": {"name", "名称", "参数名"},
		"status_cd":  {"status", "状态", "状态码"},
		"cfg_ver_id": {"ver", "版本", "配置版本"},
		"codea":      {"code_a", "编码A"},
		"codeb":      {"code_b", "编码B"},
		"stype":      {"type", "类型"},
		"group_code": {"group", "组编码"},
		"group_name": {"group", "组名"},
		"sort_idx":   {"sort", "排序"},
	}
	if mapped, ok := specialMap[lower]; ok {
		aliases = append(aliases, mapped...)
	}

	// 去重
	seen := make(map[string]bool)
	var unique []string
	for _, a := range aliases {
		if !seen[a] && a != lower {
			seen[a] = true
			unique = append(unique, a)
		}
	}
	return unique
}

// detectMissingFields 检测用户问题中是否存在DDL中不存在的字段名，生成对照表
func detectMissingFields(text string, kb *KnowledgeBase) string {
	fieldPattern := regexp.MustCompile(`[a-zA-Z_][a-zA-Z0-9_]{1,30}`)
	candidates := fieldPattern.FindAllString(text, -1)
	if len(candidates) == 0 {
		return ""
	}
	ddlFields := make(map[string]bool)
	for _, cache := range kb.GetAllTables() {
		for _, col := range cache.ColumnsJSON {
			ddlFields[strings.ToLower(col.Name)] = true
		}
	}
	var missing []string
	seen := make(map[string]bool)
	for _, c := range candidates {
		cl := strings.ToLower(c)
		if seen[cl] {
			continue
		}
		seen[cl] = true
		if !ddlFields[cl] && len(cl) >= 3 {
			best := findClosestField(cl, kb)
			if best != "" {
				missing = append(missing, fmt.Sprintf("- 用户提到 `%s` → DDL中不存在！请使用 `%s` 代替", c, best))
			}
		}
	}
	if len(missing) == 0 {
		return ""
	}
	return "## 🚨 用户提到的字段名对照（强制使用DDL中的实际字段名）\n" + strings.Join(missing, "\n") + "\n\n"
}

// sanitizeQuestionFields 将用户问题中的错误字段名替换为DDL实际字段名
// 注意：此方法仅在精确匹配到DDL字段时替换，模糊匹配已弃用（太不可靠）
// 真正的字段名校验由 backwardValidateAndFix 在SQL生成后执行
func sanitizeQuestionFields(text string, kb *KnowledgeBase) string {
	result := text
	fieldPattern := regexp.MustCompile(`[a-zA-Z_][a-zA-Z0-9_]{1,30}`)
	candidates := fieldPattern.FindAllString(text, -1)
	if len(candidates) == 0 {
		return text
	}
	// 构建所有DDL字段的快速查找表
	ddlFieldsLower := make(map[string]string) // lowercase -> original case
	for _, cache := range kb.GetAllTables() {
		for _, col := range cache.ColumnsJSON {
			ddlFieldsLower[strings.ToLower(col.Name)] = col.Name
		}
	}
	// 只做精确匹配替换（case-insensitive）
	// 例如用户写 "PARAM_VALUE" 但 DDL 中是 "PARAM_VALUE" → 不替换
	// 用户写 "val" 但 DDL 中没有 "val" → 不替换（留给后向验证处理）
	type replacement struct {
		old, new string
	}
	var replacements []replacement
	seen := make(map[string]bool)
	for _, c := range candidates {
		cl := strings.ToLower(c)
		if seen[cl] || len(cl) < 3 {
			continue
		}
		seen[cl] = true
		// 如果精确匹配到 DDL 字段（忽略大小写），修正大小写
		if actual, ok := ddlFieldsLower[cl]; ok && actual != c {
			replacements = append(replacements, replacement{c, actual})
		}
	}
	// 按原字符串长度降序排序，避免部分匹配
	sort.Slice(replacements, func(i, j int) bool {
		return len(replacements[i].old) > len(replacements[j].old)
	})
	for _, r := range replacements {
		result = strings.ReplaceAll(result, r.old, r.new)
	}
	return result
}

// backwardValidateAndFix 后向 Schema Linking：解析生成的 SQL，逐字段比对 DDL，
// 发现不存在的字段时自动映射到最接近的 DDL 字段并替换
// 这是 RSL-SQL 论文中 "Backward Schema Linking" 的核心思想
func (g *Generator) backwardValidateAndFix(sql string, kb *KnowledgeBase) (string, []string) {
	if sql == "" || g.llmProvider == nil {
		return sql, nil
	}

	// 1. 从 SQL AST 提取所有字段引用
	refs := ExtractColumnRefs(sql)
	if len(refs) == 0 {
		return sql, nil
	}

	// 2. 构建别名→表名映射
	aliasMap := BuildAliasToTableMap(sql)

	// 3. 构建每张表的字段集合（lowercase → original）
	tableFields := make(map[string]map[string]string) // tableName -> {lowercase: original}
	for name, cache := range kb.GetAllTables() {
		fields := make(map[string]string)
		for _, col := range cache.ColumnsJSON {
			fields[strings.ToLower(col.Name)] = col.Name
		}
		tableFields[strings.ToLower(name)] = fields
	}

	// 4. 逐字段校验
	type fieldFix struct {
		alias   string
		wrong   string
		correct string
		table   string
	}
	var fixes []fieldFix
	var warnings []string

	for _, ref := range refs {
		if ref.ColumnName == "*" {
			continue // SELECT * 不校验
		}
		// 确定字段所属的表
		tableName := ""
		if ref.TableAlias != "" {
			tableName = aliasMap[strings.ToUpper(ref.TableAlias)]
			if tableName == "" {
				tableName = aliasMap[ref.TableAlias]
			}
		}
		// 如果无法确定表，检查所有表的字段
		if tableName == "" {
			// 在所有表中查找
			found := false
			for _, fields := range tableFields {
				if _, ok := fields[strings.ToLower(ref.ColumnName)]; ok {
					found = true
					break
				}
			}
			if !found {
				warnings = append(warnings, fmt.Sprintf("字段 %s 在DDL中不存在", ref.ColumnName))
			}
			continue
		}

		// 检查字段是否在表中存在
		fields, ok := tableFields[strings.ToLower(tableName)]
		if !ok {
			continue // 表不在知识库中，无法校验
		}
		if _, ok := fields[strings.ToLower(ref.ColumnName)]; ok {
			continue // 字段存在，OK
		}

		// 字段不存在！先尝试 Business Glossary 确定性映射
		correct := g.lookupGlossary(ref.ColumnName, tableName, kb)
		if correct == "" {
			// 再尝试启发式匹配
			correct = findBestMatchingField(ref.ColumnName, tableName, kb)
		}
		if correct != "" {
			fixes = append(fixes, fieldFix{
				alias:   ref.TableAlias,
				wrong:   ref.ColumnName,
				correct: correct,
				table:   tableName,
			})
			warnings = append(warnings, fmt.Sprintf("字段 %s.%s 在DDL中不存在，已修正为 %s.%s",
				ref.TableAlias, ref.ColumnName, ref.TableAlias, correct))
		} else {
			warnings = append(warnings, fmt.Sprintf("字段 %s.%s 在 %s 表的DDL中不存在且无法自动修正",
				ref.TableAlias, ref.ColumnName, tableName))
		}
	}

	if len(fixes) == 0 {
		return sql, warnings
	}

	// 5. 对 SQL 进行文本替换（从最长到最短，避免部分匹配）
	// 跳过单引号字符串常量，避免误替换 SQL 字面量中的内容
	sort.Slice(fixes, func(i, j int) bool {
		return len(fixes[i].wrong) > len(fixes[j].wrong)
	})
	fixedSQL := sql
	for _, fix := range fixes {
		oldRef := fix.alias + "." + fix.wrong
		newRef := fix.alias + "." + fix.correct
		fixedSQL = replaceOutsideStringLiterals(fixedSQL, oldRef, newRef)
		// 也替换无别名的情况（不推荐但可能存在）
		if fix.alias == "" {
			fixedSQL = replaceOutsideStringLiterals(fixedSQL, fix.wrong, fix.correct)
		}
	}

	// 6. 如果修正后仍有错误字段，调用 LLM 做最终修正
	remainingRefs := ExtractColumnRefs(fixedSQL)
	stillInvalid := false
	for _, ref := range remainingRefs {
		if ref.ColumnName == "*" {
			continue
		}
		tableName := ""
		if ref.TableAlias != "" {
			tableName = aliasMap[strings.ToUpper(ref.TableAlias)]
		}
		if tableName == "" {
			continue
		}
		fields, ok := tableFields[strings.ToLower(tableName)]
		if !ok {
			continue
		}
		if _, ok := fields[strings.ToLower(ref.ColumnName)]; !ok {
			stillInvalid = true
			break
		}
	}

	if stillInvalid {
		llmFixed := g.llmFixFields(fixedSQL, kb, aliasMap, warnings)
		if llmFixed != "" {
			return llmFixed, warnings
		}
	}

	return fixedSQL, warnings
}

// lookupGlossary 从 Business Glossary 中查找确定性的字段映射
// 这是消除 LLM 字段幻觉的最可靠方式：不让 LLM 猜字段名
func (g *Generator) lookupGlossary(wrongField, tableName string, kb *KnowledgeBase) string {
	if g.glossaryRepo == nil {
		return ""
	}

	wrongLower := strings.ToLower(wrongField)
	tableLower := strings.ToLower(tableName)

	items, err := g.glossaryRepo.FindByTerm(0, wrongField)
	if err != nil {
		return ""
	}
	for _, item := range items {
		if strings.ToLower(item.Term) == wrongLower && strings.ToLower(item.TblName) == tableLower {
			return item.ColName
		}
		if strings.ToLower(item.Term) == wrongLower && item.TblName == "" {
			return item.ColName
		}
	}

	itemsByTable, err := g.glossaryRepo.FindByTable(0, tableName)
	if err != nil {
		return ""
	}
	for _, item := range itemsByTable {
		if strings.ToLower(item.Term) == wrongLower {
			return item.ColName
		}
	}

	return ""
}

func (g *Generator) AutoGenerateGlossary(kb *KnowledgeBase, spaceID int) (int, error) {
	if g.glossaryRepo == nil {
		return 0, fmt.Errorf("glossary repo not configured")
	}

	existing, err := g.glossaryRepo.ListBySpace(spaceID)
	if err != nil {
		return 0, err
	}
	existingSet := make(map[string]bool)
	for _, item := range existing {
		key := strings.ToLower(item.Term) + "." + strings.ToLower(item.TblName)
		existingSet[key] = true
	}

	count := 0
	for name, cache := range kb.GetAllTables() {
		for _, col := range cache.ColumnsJSON {
			if col.Comment == "" {
				continue
			}
			aliases := generateFieldAliases(col.Name, col.Comment)
			for _, alias := range aliases {
				key := strings.ToLower(alias) + "." + strings.ToLower(name)
				if existingSet[key] {
					continue
				}
				if len(alias) < 2 {
					continue
				}
				item := &modelai.BusinessGlossary{
					SpaceID:     spaceID,
					Term:        alias,
					TblName:     name,
					ColName:     col.Name,
					Description: fmt.Sprintf("自动生成: %s.%s 的别名", name, col.Name),
				}
				if err := g.glossaryRepo.Create(item); err != nil {
					continue
				}
				count++
				existingSet[key] = true
			}
		}
	}

	return count, nil
}

// applyGlossaryMapping 在问题送入 LLM 前，用 Business Glossary 做确定性术语替换
// 这是 Dify/Vanna 的核心策略：不让 LLM 猜字段名
func (g *Generator) applyGlossaryMapping(text string, spaceID int) string {
	if g.glossaryRepo == nil {
		return text
	}

	var items []*modelai.BusinessGlossary
	if spaceID > 0 {
		glossaryItems, err := g.glossaryRepo.ListBySpace(spaceID)
		if err != nil || len(glossaryItems) == 0 {
			return text
		}
		items = glossaryItems
	} else {
		return text
	}

	type replacement struct {
		term   string
		mapped string
	}
	var replacements []replacement

	for _, item := range items {
		mapped := item.TblName + "." + item.ColName
		replacements = append(replacements, replacement{term: item.Term, mapped: mapped})
	}

	sort.Slice(replacements, func(i, j int) bool {
		return len(replacements[i].term) > len(replacements[j].term)
	})

	result := text
	for _, r := range replacements {
		// 使用正则词边界匹配，避免部分匹配（如 "code" 匹配 "encode"）
		pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(r.term) + `\b`)
		if pattern.MatchString(result) {
			result = pattern.ReplaceAllString(result, r.mapped)
		}
	}
	return result
}

// findBestMatchingField 在指定表的 DDL 中查找与错误字段名最接近的实际字段
// 使用多策略匹配：1) 子串包含 2) 去除常见前后缀后匹配 3) Levenshtein 编辑距离
func findBestMatchingField(wrongField, tableName string, kb *KnowledgeBase) string {
	cache, ok := kb.tables[tableName]
	if !ok {
		// 尝试 case-insensitive 查找
		for name, c := range kb.tables {
			if strings.EqualFold(name, tableName) {
				cache = c
				ok = true
				break
			}
		}
	}
	if !ok {
		return ""
	}

	wrongLower := strings.ToLower(wrongField)
	type candidate struct {
		name  string
		score int
	}
	var candidates []candidate

	for _, col := range cache.ColumnsJSON {
		colLower := strings.ToLower(col.Name)
		if colLower == wrongLower {
			return col.Name
		}

		score := 0

		if strings.Contains(colLower, wrongLower) {
			score += 10
			if strings.HasPrefix(colLower, wrongLower) || strings.HasSuffix(colLower, wrongLower) {
				score += 5
			}
			lenDiff := len(colLower) - len(wrongLower)
			if lenDiff <= 4 {
				score += (4 - lenDiff)
			}
		}

		if strings.Contains(wrongLower, colLower) {
			score += 3
		}

		strippedWrong := stripCommonAffixes(wrongLower)
		strippedCol := stripCommonAffixes(colLower)
		if strippedWrong != wrongLower && strippedCol != colLower {
			if strippedWrong == strippedCol {
				score += 15
			} else if strings.Contains(strippedCol, strippedWrong) || strings.Contains(strippedWrong, strippedCol) {
				score += 8
			}
		}

		if col.Comment != "" && strings.Contains(strings.ToLower(col.Comment), wrongLower) {
			score += 6
		}

		editDist := levenshteinDistance(wrongLower, colLower)
		maxLen := len(wrongLower)
		if len(colLower) > maxLen {
			maxLen = len(colLower)
		}
		if maxLen > 0 {
			similarity := 1.0 - float64(editDist)/float64(maxLen)
			if similarity >= 0.6 {
				score += int(similarity * 10)
			}
		}

		if col.Comment != "" {
			commentLower := strings.ToLower(col.Comment)
			if strings.Contains(commentLower, wrongLower) || strings.Contains(wrongLower, commentLower) {
				score += 4
			}
			commentWords := extractKeywords(col.Comment)
			wrongWords := extractKeywords(wrongField)
			overlap := 0
			for _, cw := range commentWords {
				for _, ww := range wrongWords {
					if cw == ww {
						overlap++
					}
				}
			}
			if overlap > 0 {
				score += overlap * 3
			}
		}

		if score > 0 {
			candidates = append(candidates, candidate{name: col.Name, score: score})
		}
	}

	if len(candidates) == 0 {
		return ""
	}

	// 选出得分最高的
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	// 只有当最高分 >= 8 时才返回（避免误匹配）
	if candidates[0].score >= 8 {
		return candidates[0].name
	}
	return ""
}

// llmFixFields 当自动修正无法解决所有字段名问题时，调用 LLM 做最终修正
func (g *Generator) llmFixFields(sql string, kb *KnowledgeBase, aliasMap map[string]string, warnings []string) string {
	// 构建 DDL 摘要
	var ddlSummary strings.Builder
	for alias, tableName := range aliasMap {
		cache, ok := kb.tables[tableName]
		if !ok {
			for name, c := range kb.tables {
				if strings.EqualFold(name, tableName) {
					cache = c
					ok = true
					break
				}
			}
		}
		if !ok {
			continue
		}
		ddlSummary.WriteString(fmt.Sprintf("表 %s (别名 %s) 的字段: ", tableName, alias))
		var fieldNames []string
		for _, col := range cache.ColumnsJSON {
			fieldNames = append(fieldNames, col.Name)
		}
		ddlSummary.WriteString(strings.Join(fieldNames, ", "))
		ddlSummary.WriteString("\n")
	}

	prompt := "以下 SQL 中存在 DDL 中不存在的字段名，请根据提供的 DDL 字段列表修正。\n\n" +
		"## 需要修正的 SQL\n" + "```sql\n" + sql + "\n```\n\n" +
		"## 各表的 DDL 字段列表\n" + ddlSummary.String() + "\n\n" +
		"## 已知问题\n" + strings.Join(warnings, "\n") + "\n\n" +
		"## 要求\n" +
		"1. 只修正字段名，不要改变 SQL 的逻辑结构\n" +
		"2. 每个错误字段名替换为 DDL 中最接近的语义等价字段\n" +
		"3. 只输出修正后的完整 SQL，不要解释\n" +
		"4. 使用 ```sql ``` 包裹"

	ctx := context.Background()
	resp, err := g.llmProvider.Chat(ctx, "你是SQL修正专家。只输出修正后的SQL，不要解释。", []Message{
		NewTextMessage("user", prompt),
	})
	if err != nil {
		return ""
	}
	logx.Infof("[Text2SQL-DIAG] llmFixFields LLM response: content=%q, len=%d", resp.Content, len(resp.Content))

	// 提取 SQL 代码块
	sqlRegex := regexp.MustCompile("(?s)```sql\\s*(.*?)\\s*```")
	if m := sqlRegex.FindStringSubmatch(resp.Content); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	// 尝试提取 SELECT
	lines := strings.Split(resp.Content, "\n")
	var sqlLines []string
	inSQL := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.ToUpper(line))
		if strings.HasPrefix(trimmed, "SELECT") {
			inSQL = true
		}
		if inSQL {
			sqlLines = append(sqlLines, line)
			if strings.HasSuffix(trimmed, ";") && !strings.HasPrefix(trimmed, "SELECT") {
				break
			}
		}
	}
	if len(sqlLines) > 0 {
		return strings.Join(sqlLines, "\n")
	}
	return ""
}

// findClosestField 在所有表的 DDL 中查找最接近的字段名（已弃用，使用 findBestMatchingField 代替）
func findClosestField(fieldName string, kb *KnowledgeBase) string {
	var best string
	bestScore := 0
	for _, cache := range kb.GetAllTables() {
		for _, col := range cache.ColumnsJSON {
			score := fieldSimilarity(fieldName, col.Name)
			if score > bestScore {
				bestScore = score
				best = col.Name
			}
		}
	}
	if bestScore > 2 && best != "" && !strings.EqualFold(best, fieldName) {
		return best
	}
	return ""
}

func fieldSimilarity(user, actual string) int {
	u, a := strings.ToLower(user), strings.ToLower(actual)
	if u == a {
		return 0
	}
	score := 0
	if strings.Contains(a, u) {
		score += 5
	}
	if strings.Contains(u, a) {
		score += 3
	}
	chars := make(map[rune]bool)
	for _, c := range u {
		chars[c] = true
	}
	for _, c := range a {
		if chars[c] {
			score++
		}
	}
	return score
}

// SetGlossaryRepo 设置业务术语映射仓库
func (g *Generator) SetGlossaryRepo(repo *airepo.BusinessGlossaryRepo) {
	g.glossaryRepo = repo
}

func (g *Generator) GetGlossaryRepo() *airepo.BusinessGlossaryRepo {
	return g.glossaryRepo
}

// replaceOutsideStringLiterals 在 SQL 中替换文本，但跳过单引号字符串常量
// 防止替换 WHERE p.param_val LIKE '%val%' 中的 'val'
// 标识符替换时检查词边界：只替换前后都不是字母/数字/下划线的位置，
// 避免 val 命中 value_code、total_val 等更长标识符的子串。
func replaceOutsideStringLiterals(sql, old, new string) string {
	if !strings.Contains(sql, old) {
		return sql
	}
	isIdentChar := func(b byte) bool {
		return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
	}
	var result strings.Builder
	inString := false
	i := 0
	for i < len(sql) {
		ch := sql[i]
		if ch == '\'' && (i == 0 || sql[i-1] != '\\') {
			inString = !inString
			result.WriteByte(ch)
			i++
			continue
		}
		if !inString && strings.HasPrefix(sql[i:], old) {
			// 词边界检查：替换文本前后不能紧邻标识符字符，否则会破坏更长标识符
			prevIsIdent := i > 0 && isIdentChar(sql[i-1])
			nextIdx := i + len(old)
			nextIsIdent := nextIdx < len(sql) && isIdentChar(sql[nextIdx])
			if !prevIsIdent && !nextIsIdent {
				result.WriteString(new)
				i += len(old)
				continue
			}
		}
		result.WriteByte(ch)
		i++
	}
	return result.String()
}
