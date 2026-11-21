package text2sql

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	modelai "github.com/listen808/citebase/internal/model/ai"
	schemamodel "github.com/listen808/citebase/internal/model/schema"
	airepo "github.com/listen808/citebase/internal/repository/ai"
)

// Text2SQL 生成器：schema 上下文构建、schema-linking、表/字段/关系上下文。

// buildSchemaContext 构建 Schema 上下文（含问题相关性过滤 + 硬长度限制）
func (g *Generator) buildSchemaContext(text string, kb *KnowledgeBase) string {
	keywords := extractKeywords(text)
	if len(keywords) == 0 {
		keywords = []string{"id", "name"}
	}

	type scoredTable struct {
		name  string
		cache *modelai.TableCache
		score int
	}

	var scored []scoredTable
	for name, cache := range kb.GetAllTables() {
		score := 0
		lowerName := strings.ToLower(name)
		for _, kw := range keywords {
			if strings.Contains(lowerName, kw) {
				score += 10
			}
			if strings.Contains(strings.ToLower(cache.TableComment), kw) {
				score += 5
			}
			for _, col := range cache.ColumnsJSON {
				if strings.Contains(strings.ToLower(col.Name), kw) {
					score += 3
				}
				if strings.Contains(strings.ToLower(col.Comment), kw) {
					score += 2
				}
			}
		}
		scored = append(scored, scoredTable{name, cache, score})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	maxTables := GetText2SQLConfig().SchemaMaxTables
	if len(scored) > maxTables {
		scored = scored[:maxTables]
	}

	// 构建类 RAG 文档风格的表结构上下文（DDL + 自然语言描述）
	var parts []string
	parts = append(parts, "以下是与当前问题相关的数据库表结构信息，请仔细分析后生成SQL：\n")

	maxTotalLen := GetText2SQLConfig().SchemaMaxChars
	var totalLen int

	for i, st := range scored {
		tableBlock := g.buildTableContextBlock(st, i+1)
		potentialLen := len(tableBlock) + 20
		if totalLen+potentialLen > maxTotalLen {
			break
		}
		parts = append(parts, tableBlock)
		totalLen += potentialLen
	}

	// 关联关系
	relations := kb.GetAllRelations()
	if len(relations) > 0 {
		relBlock := "\n**表间关联关系**：\n"
		relCount := 0
		for _, rel := range relations {
			relLine := fmt.Sprintf("- `%s.%s` → `%s.%s` (%s)\n",
				rel.SourceTable, rel.SourceField,
				rel.TargetTable, rel.TargetField,
				rel.RelationType)
			if totalLen+len(relBlock)+len(relLine) > maxTotalLen {
				break
			}
			relBlock += relLine
			relCount++
		}
		if relCount > 0 {
			parts = append(parts, relBlock)
		}
	}

	return strings.Join(parts, "\n---\n")
}

// buildTableContextBlock 为单张表构建精简的上下文块
// 优化版本：减少Token消耗，保留关键信息
func (g *Generator) buildTableContextBlock(st struct {
	name  string
	cache *modelai.TableCache
	score int
}, idx int) string {
	var sb strings.Builder

	alias := suggestAlias(st.name)

	// 简化表标题
	sb.WriteString(fmt.Sprintf("### 表 %d: `%s` (`%s`)\n", idx, st.name, alias))

	// 精简DDL（截断到1500字符）
	if st.cache.DDLContent != "" {
		ddl := st.cache.DDLContent
		if len(ddl) > 1500 {
			ddl = ddl[:1500] + "\n-- ... (truncated)"
		}
		sb.WriteString(fmt.Sprintf("```sql\n%s\n```\n", ddl))
	}

	// 精简字段详情（只包含关键字段）
	if len(st.cache.ColumnsJSON) > 0 {
		sb.WriteString("**关键字段**:\n")
		for _, col := range st.cache.ColumnsJSON {
			// 只包含主键、外键和有注释的字段
			if col.IsPK || strings.HasSuffix(strings.ToLower(col.Name), "_id") || col.Comment != "" {
				RAINTHint := ""
				if col.IsPK {
					RAINTHint = " [PK]"
				}
				sb.WriteString(fmt.Sprintf("- `%s`%s: %s\n", col.Name, RAINTHint, col.Comment))
			}
		}
	}

	// 简化索引信息（只显示前3个）
	if len(st.cache.ConstraintsJSON) > 0 {
		var indexes []string
		for i, c := range st.cache.ConstraintsJSON {
			if i >= 3 {
				break
			}
			if c.ConstraintType == "INDEX" || c.ConstraintType == "UNIQUE" {
				idxInfo := fmt.Sprintf("`%s`(%s)", c.ConstraintName, strings.Join(c.Columns, ", "))
				indexes = append(indexes, idxInfo)
			}
		}
		if len(indexes) > 0 {
			sb.WriteString(fmt.Sprintf("**索引**: %s\n", strings.Join(indexes, ", ")))
		}
	}

	return sb.String()
}

// extractTablesFromSQL 从 SQL 中提取表名
func (g *Generator) extractTablesFromSQL(sql string, kb *KnowledgeBase) []string {
	var tables []string
	seen := make(map[string]bool)

	sqlLower := strings.ToLower(sql)
	for name := range kb.GetAllTables() {
		if strings.Contains(sqlLower, strings.ToLower(name)) {
			if !seen[name] {
				tables = append(tables, name)
				seen[name] = true
			}
		}
	}

	return tables
}

// SetSemanticRepos 设置语义层仓储
func (g *Generator) SetSemanticRepos(metricRepo *airepo.SemanticMetricRepo, dimensionRepo *airepo.SemanticDimensionRepo, statsRepo *airepo.ColumnStatsRepo) {
	g.metricRepo = metricRepo
	g.dimensionRepo = dimensionRepo
	g.statsRepo = statsRepo
}

func (g *Generator) buildRelationsContext(kb *KnowledgeBase) string {
	var parts []string
	relations := kb.GetAllRelations()
	if len(relations) == 0 {
		return "暂无关系信息"
	}
	for _, rel := range relations {
		parts = append(parts, fmt.Sprintf("- %s.%s → %s.%s [%s]",
			rel.SourceTable, rel.SourceField, rel.TargetTable, rel.TargetField, rel.RelationType))
	}
	return strings.Join(parts, "\n")
}

// SetSchemaLinker 设置 Schema Linker
func (g *Generator) SetSchemaLinker(linker *SchemaLinker) {
	g.schemaLinker = linker
}

func (g *Generator) SetSchemaRepo(repo SchemaRepoInterface) {
	g.schemaRepo = repo
	// 同步注入到 KnowledgeBase，使其 LoadFromSpace/LoadAll/MergeAllTables 直接从 aw_schema_* 加载
	g.kb.SetSchemaRepo(repo)
}

func (g *Generator) LoadFromSchemaSpace(kb *KnowledgeBase, wikiSpaceID int) int {
	if g.schemaRepo == nil {
		return 0
	}

	schemaSpace, err := g.schemaRepo.FindSpaceByWikiSpaceID(wikiSpaceID)
	if err != nil {
		allSpaces, _, spaceErr := g.schemaRepo.ListSpaces(1, 100)
		if spaceErr != nil || len(allSpaces) == 0 {
			return 0
		}
		schemaSpace = allSpaces[0]
	}
	schemaSpaceID := schemaSpace.ID

	tables, err := g.schemaRepo.ListAllTables(schemaSpaceID)
	if err != nil || len(tables) == 0 {
		return 0
	}

	count := 0
	for _, t := range tables {
		cleanName := cleanTableName(t.Name)
		if kb.GetTable(cleanName) != nil {
			continue
		}

		columns, err := g.schemaRepo.ListColumnsByTableName(schemaSpaceID, t.Name)
		if err != nil {
			continue
		}

		colInfos := make(modelai.ColumnInfoList, 0, len(columns))
		for _, col := range columns {
			colInfos = append(colInfos, modelai.ColumnInfo{
				Name:       cleanColumnName(col.ColumnName),
				Type:       col.DataType,
				Comment:    col.ColumnComment,
				IsPK:       col.IsPrimaryKey == 1,
				IsNullable: col.Nullable == 1,
			})
		}

		cache := &modelai.TableCache{
			TableNam:     cleanName,
			TableComment: t.TableComment,
			DDLContent:   t.RawDDL,
			ColumnsJSON:  colInfos,
		}
		kb.tables[cleanName] = cache
		count++
	}

	relations, err := g.schemaRepo.ListRelations(schemaSpaceID)
	if err == nil {
		for _, rel := range relations {
			srcTable := cleanTableName(rel.SourceTable)
			kb.rels[srcTable] = append(kb.rels[srcTable], modelai.TableRelation{
				SourceTable:  srcTable,
				SourceField:  cleanColumnName(rel.SourceColumn),
				TargetTable:  cleanTableName(rel.TargetTable),
				TargetField:  cleanColumnName(rel.TargetColumn),
				RelationType: rel.RelationType,
			})
		}
	}

	return count
}

// resolveSchemaSpaceID 将 wiki spaceID 转换为 schema_space_id
// SchemaSpace.ID（自增主键）通常 ≠ SchemaSpace.SpaceID（wiki space_id），必须做转换
func (g *Generator) resolveSchemaSpaceID(spaceID int) int {
	if g.schemaRepo == nil {
		return spaceID
	}
	schemaSpace, err := g.schemaRepo.FindSpaceByWikiSpaceID(spaceID)
	if err != nil || schemaSpace == nil {
		// 兜底：取第一个 Schema 空间（与 LoadFromSchemaSpace 行为一致）
		allSpaces, _, spaceErr := g.schemaRepo.ListSpaces(1, 100)
		if spaceErr != nil || len(allSpaces) == 0 {
			return spaceID
		}
		return allSpaces[0].ID
	}
	return schemaSpace.ID
}

// buildTableFieldMap 构建 tableName -> {lowercase -> original} 的字段映射
func buildTableFieldMap(kb *KnowledgeBase) map[string]map[string]string {
	tableFields := make(map[string]map[string]string)
	for name, cache := range kb.GetAllTables() {
		fields := make(map[string]string)
		for _, col := range cache.ColumnsJSON {
			fields[strings.ToLower(col.Name)] = col.Name
		}
		tableFields[strings.ToLower(name)] = fields
	}
	return tableFields
}

// buildLinkedSchemaContext 基于 LinkedSchema 构建精简 Schema 上下文
func (g *Generator) buildLinkedSchemaContext(linked *LinkedSchema) string {
	var parts []string
	parts = append(parts, "### 相关表结构（基于 Schema Linker 语义检索）\n")
	for _, t := range linked.Tables {
		comment := ""
		if t.TableComment != "" {
			comment = fmt.Sprintf(" (%s)", t.TableComment)
		}
		parts = append(parts, fmt.Sprintf("#### %s%s", t.Name, comment))
		parts = append(parts, "")
		cols := linked.Columns[t.Name]
		if len(cols) > 0 {
			parts = append(parts, "| 字段名 | 类型 | 可空 | 注释 | 可能被称作 |")
			parts = append(parts, "|--------|------|------|------|----------|")
			for _, col := range cols {
				nullable := ""
				if col.Nullable == 0 {
					nullable = "NOT NULL"
				}
				colComment := col.ColumnComment
				if colComment == "" {
					colComment = "-"
				}
				// 生成"可能被称作"列
				aliases := generateFieldAliases(col.ColumnName, col.ColumnComment)
				aliasStr := "-"
				if len(aliases) > 0 {
					aliasStr = strings.Join(aliases, ", ")
				}
				parts = append(parts, fmt.Sprintf("| `%s` | %s | %s | %s | %s |", col.ColumnName, col.DataType, nullable, colComment, aliasStr))
			}
			parts = append(parts, "")
		}
	}
	return strings.Join(parts, "\n")
}

// buildLinkedRelationsContext 基于 LinkedSchema 构建关系上下文
func (g *Generator) buildLinkedRelationsContext(linked *LinkedSchema) string {
	if len(linked.Relations) == 0 {
		return "暂无关系信息"
	}
	var parts []string
	for _, rel := range linked.Relations {
		parts = append(parts, fmt.Sprintf("- %s.%s → %s.%s [%s]", rel.SourceTable, rel.SourceColumn, rel.TargetTable, rel.TargetColumn, rel.RelationType))
	}
	return strings.Join(parts, "\n")
}

// extractTableNamesFromSQL 从SQL语句中提取表名
func extractTableNamesFromSQL(sqlStr string) []string {
	if sqlStr == "" {
		return nil
	}
	upper := strings.ToUpper(sqlStr)
	tables := make(map[string]bool)

	// FROM table_name
	fromRe := regexp.MustCompile(`(?i)\bFROM\s+[` + "`" + `"]?(\w+)[` + "`" + `"]?`)
	for _, m := range fromRe.FindAllStringSubmatch(upper, -1) {
		if len(m) > 1 {
			t := strings.ToUpper(m[1])
			if !isSQLKeyword(t) {
				tables[t] = true
			}
		}
	}

	// JOIN table_name
	joinRe := regexp.MustCompile(`(?i)\bJOIN\s+[` + "`" + `"]?(\w+)[` + "`" + `"]?`)
	for _, m := range joinRe.FindAllStringSubmatch(upper, -1) {
		if len(m) > 1 {
			t := strings.ToUpper(m[1])
			if !isSQLKeyword(t) {
				tables[t] = true
			}
		}
	}

	var result []string
	for t := range tables {
		result = append(result, t)
	}
	return result
}

// tableSummary 表概要
type tableSummary struct {
	name    string
	summary string
}

// extractTableSummary 从 schemaCtx 提取表名和简短摘要
func extractTableSummary(schemaCtx string) []tableSummary {
	lines := strings.Split(schemaCtx, "\n")
	var tables []tableSummary
	for _, line := range lines {
		if strings.HasPrefix(line, "#### ") {
			name := strings.TrimPrefix(line, "#### ")
			// 提取表名（第一个空格前的部分，如果有别名取别名前）
			parts := strings.SplitN(name, " ", 2)
			tableName := strings.TrimSpace(parts[0])
			summary := ""
			if len(parts) > 1 {
				summary = strings.TrimSpace(parts[1])
			}
			if len(summary) > 80 {
				summary = summary[:80] + "..."
			}
			tables = append(tables, tableSummary{name: tableName, summary: summary})
		}
	}
	return tables
}

// retrieveSchemaContext 统一的Schema上下文检索：优先Schema Linker向量检索 → 回退LLM增强关键词匹配
func (g *Generator) retrieveSchemaContext(ctx context.Context, text string, kb *KnowledgeBase, spaceID int) (string, string) {
	// 【关键】从用户原话中提取显式表名（不依赖向量检索），强制保留
	mentionedTables := extractMentionedTables(text, kb)
	if len(mentionedTables) > 0 {
		// 将用户提到的表名注入到搜索文本中，确保向量检索能优先召回
		mentionedNames := make([]string, 0, len(mentionedTables))
		for _, t := range mentionedTables {
			mentionedNames = append(mentionedNames, t.TableNam)
		}
		text = text + "\n[User-mentioned tables (MANDATORY): " + strings.Join(mentionedNames, ", ") + "]"
	}

	// 优先使用 Schema Linker 语义向量检索
	if g.schemaLinker != nil && spaceID > 0 {
		schemaSpaceID := g.resolveSchemaSpaceID(spaceID)
		linked, err := g.schemaLinker.Link(ctx, text, schemaSpaceID)
		if err == nil && len(linked.Tables) > 0 {
			// 【关键】用户显式提到的表必须出现在结果里，即使向量检索没找到
			linkedTableNames := make(map[string]bool, len(linked.Tables))
			for _, t := range linked.Tables {
				linkedTableNames[strings.ToLower(t.Name)] = true
			}
			for _, mt := range mentionedTables {
				if !linkedTableNames[strings.ToLower(mt.TableNam)] {
					// 强制加入用户提到的表（用 schemamodel.SchemaTable 结构）
					linked.Tables = append(linked.Tables, &schemamodel.SchemaTable{
						Name:         mt.TableNam,
						TableComment: mt.TableComment,
						RawDDL:       mt.DDLContent,
					})
				}
			}
			// 从 kb 中获取DDL内容增强上下文
			for _, t := range linked.Tables {
				if cache, ok := kb.tables[t.Name]; ok && cache.DDLContent != "" {
					// DDL已经在kb中了，buildSchemaContext会使用
					_ = cache
				}
			}
			schemaCtx := g.buildSchemaContext(text, kb)
			relCtx := g.buildLinkedRelationsContext(linked)
			return schemaCtx, relCtx
		}
	}
	// 回退：关键词匹配（buildSchemaContext 内部会读取 kb 全部表）
	return g.buildSchemaContext(text, kb), g.buildRelationsContext(kb)
}

// extractMentionedTables 从用户文本中提取显式提到的表名（不依赖语义检索）
// 匹配规则：
//  1. 精确匹配：表名（大小写不敏感）出现在文本中
//  2. 别名匹配：移除下划线后匹配（用户可能写 "BM DATA DICT" 或 "BM-DATA-DICT"）
func extractMentionedTables(text string, kb *KnowledgeBase) []*modelai.TableCache {
	if kb == nil {
		return nil
	}
	upperText := strings.ToUpper(text)
	var matched []*modelai.TableCache
	seen := make(map[string]bool)

	for name, cache := range kb.tables {
		if seen[name] {
			continue
		}
		upperName := strings.ToUpper(name)
		// 1. 精确大小写不敏感匹配
		if strings.Contains(upperText, upperName) {
			matched = append(matched, cache)
			seen[name] = true
			continue
		}
		// 2. 移除下划线后匹配
		strippedText := strings.ReplaceAll(upperText, "_", "")
		strippedName := strings.ReplaceAll(upperName, "_", "")
		if len(strippedName) > 3 && strings.Contains(strippedText, strippedName) {
			matched = append(matched, cache)
			seen[name] = true
		}
	}
	return matched
}

// extractTablesFromSQL 从SQL中提取涉及的表名
func extractTablesFromSQL(sqlStr string, knownTables []string) []string {
	upper := strings.ToUpper(sqlStr)
	var tables []string
	seen := make(map[string]bool)
	for _, t := range knownTables {
		if strings.Contains(upper, strings.ToUpper(t)) && !seen[t] {
			tables = append(tables, t)
			seen[t] = true
		}
	}
	return tables
}

// buildAvailableFieldsMap 构建可用字段映射表（用于 AST 修正）
func (g *Generator) buildAvailableFieldsMap(kb *KnowledgeBase) map[string][]string {
	fieldMap := make(map[string][]string)

	if kb == nil {
		return fieldMap
	}

	// 遍历知识库中的所有表
	for tableName, tableCache := range kb.tables {
		fields := make([]string, 0, len(tableCache.ColumnsJSON))
		for _, col := range tableCache.ColumnsJSON {
			fields = append(fields, col.Name)
		}
		fieldMap[tableName] = fields
	}

	return fieldMap
}

// buildSemanticContext 构建语义层 + 列统计上下文，注入到生成 prompt
func (g *Generator) buildSemanticContext(spaceID uint64, kb *KnowledgeBase) string {
	if kb == nil || spaceID == 0 {
		return ""
	}

	var parts []string

	// 语义指标
	if g.metricRepo != nil {
		metrics, _, err := g.metricRepo.ListBySpace(spaceID, 1, 100)
		if err == nil && len(metrics) > 0 {
			parts = append(parts, "### 预定义语义指标（优先使用这些指标定义）")
			for _, m := range metrics {
				line := fmt.Sprintf("- %s(%s): %s", m.Name, m.Code, m.Formula)
				if m.DefaultTable != "" {
					line += fmt.Sprintf(" [默认表: %s]", m.DefaultTable)
				}
				if m.Dimensions != "" {
					line += fmt.Sprintf(" [常用维度: %s]", m.Dimensions)
				}
				parts = append(parts, line)
			}
		}
	}

	// 语义维度
	if g.dimensionRepo != nil {
		dims, _, err := g.dimensionRepo.ListBySpace(spaceID, 1, 100)
		if err == nil && len(dims) > 0 {
			parts = append(parts, "### 预定义语义维度")
			for _, d := range dims {
				parts = append(parts, fmt.Sprintf("- %s(%s): 表 %s.%s (%s) — %s", d.Name, d.Code, d.Table, d.Field, d.ValueType, d.Description))
			}
		}
	}

	// 列级统计
	if g.statsRepo != nil {
		parts = append(parts, "### 列级统计参考")
		for tableName := range kb.GetAllTables() {
			stats, err := g.statsRepo.GetByTable(spaceID, tableName)
			if err != nil || len(stats) == 0 {
				continue
			}
			for _, s := range stats {
				line := fmt.Sprintf("- %s.%s: %s, 总行数 %d, 去重值 %d", tableName, s.ColumnName, s.DataType, s.RowCount, s.DistinctCount)
				if s.MinValue != "" {
					line += fmt.Sprintf(", 最小 %s", s.MinValue)
				}
				if s.MaxValue != "" {
					line += fmt.Sprintf(", 最大 %s", s.MaxValue)
				}
				if s.SampleValues != "" {
					line += fmt.Sprintf(", 示例值 %s", s.SampleValues)
				}
				parts = append(parts, line)
			}
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}
