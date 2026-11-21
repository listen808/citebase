package text2sql

import (
	"regexp"
	"strings"

	modelai "github.com/listen808/citebase/internal/model/ai"
)

type DDLParser struct{}

func NewDDLParser() *DDLParser {
	return &DDLParser{}
}

func (p *DDLParser) ParseDDL(ddl string) (*modelai.TableCache, error) {
	cache := &modelai.TableCache{
		DDLContent: ddl,
	}

	cache.TableNam = p.extractTableName(ddl)
	cache.TableComment = p.extractTableComment(ddl)
	cache.ColumnsJSON = p.extractColumns(ddl)
	cache.ConstraintsJSON = p.extractConstraints(ddl, cache.TableNam)

	return cache, nil
}

func (p *DDLParser) extractTableName(ddl string) string {
	re := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?\"?\w+\"?\s*\.\s*\"(\w+)\"`)
	matches := re.FindStringSubmatch(ddl)
	if len(matches) > 1 {
		return matches[1]
	}

	re = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?\"(\w+)\"`)
	matches = re.FindStringSubmatch(ddl)
	if len(matches) > 1 {
		return matches[1]
	}

	re = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?` + "`" + `(\w+)` + "`")
	matches = re.FindStringSubmatch(ddl)
	if len(matches) > 1 {
		return matches[1]
	}

	re = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)`)
	matches = re.FindStringSubmatch(ddl)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

func (p *DDLParser) extractTableComment(ddl string) string {
	re := regexp.MustCompile(`(?i)COMMENT\s*=\s*'([^']*)'`)
	matches := re.FindStringSubmatch(ddl)
	if len(matches) > 1 {
		return matches[1]
	}

	re = regexp.MustCompile(`(?i)COMMENT\s+ON\s+TABLE\s+[\w".]+\s+IS\s+'([^']*)'`)
	matches = re.FindStringSubmatch(ddl)
	if len(matches) > 1 {
		return matches[1]
	}

	return ""
}

func (p *DDLParser) extractColumns(ddl string) modelai.ColumnInfoList {
	var columns modelai.ColumnInfoList

	lines := strings.Split(ddl, "\n")
	pkColumns := p.extractPKColumns(ddl)
	columnComments := p.extractOracleColumnComments(ddl)

	inBody := false
	foundCreate := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		upper := strings.ToUpper(line)

		if strings.HasPrefix(upper, "COMMENT ON") {
			continue
		}

		if strings.Contains(upper, "CREATE") && strings.Contains(upper, "TABLE") {
			foundCreate = true
			if strings.Contains(line, "(") {
				inBody = true
				idx := strings.Index(line, "(")
				afterParen := strings.TrimSpace(line[idx+1:])
				if afterParen != "" {
					col := p.parseColumnLine(afterParen)
					if col != nil {
						p.applyColumnExtras(col, pkColumns, columnComments)
						columns = append(columns, *col)
					}
				}
			}
			continue
		}

		if foundCreate && !inBody {
			if strings.Contains(line, "(") {
				inBody = true
				idx := strings.Index(line, "(")
				afterParen := strings.TrimSpace(line[idx+1:])
				if afterParen != "" {
					col := p.parseColumnLine(afterParen)
					if col != nil {
						p.applyColumnExtras(col, pkColumns, columnComments)
						columns = append(columns, *col)
					}
				}
				continue
			}
		}

		if !inBody {
			continue
		}
		if line == ")" || strings.HasPrefix(line, ")") {
			break
		}

		col := p.parseColumnLine(line)
		if col != nil {
			p.applyColumnExtras(col, pkColumns, columnComments)
			columns = append(columns, *col)
		}
	}

	return columns
}

func (p *DDLParser) applyColumnExtras(col *modelai.ColumnInfo, pkColumns map[string]bool, columnComments map[string]string) {
	// 只在 pkColumns 中有该列 或 列已标记为 PK 时才设置
	if pkColumns[col.Name] {
		col.IsPK = true
	}
	if col.Comment == "" {
		upperName := strings.ToUpper(col.Name)
		if comment, ok := columnComments[upperName]; ok {
			col.Comment = comment
		}
	}
}

func (p *DDLParser) extractOracleColumnComments(ddl string) map[string]string {
	comments := make(map[string]string)

	re := regexp.MustCompile(`(?i)COMMENT\s+ON\s+COLUMN\s+[\w".]+\.(\w+)\s+IS\s+'([^']*)'`)
	matches := re.FindAllStringSubmatch(ddl, -1)
	for _, match := range matches {
		if len(match) > 2 {
			columnName := strings.ToUpper(match[1])
			comments[columnName] = match[2]
		}
	}

	return comments
}

func (p *DDLParser) extractPKColumns(ddl string) map[string]bool {
	pkMap := make(map[string]bool)

	re := regexp.MustCompile(`(?i)PRIMARY\s+KEY\s*\(([^)]+)\)`)
	matches := re.FindStringSubmatch(ddl)
	if len(matches) > 1 {
		fields := strings.Split(matches[1], ",")
		for _, f := range fields {
			f = strings.TrimSpace(f)
			f = strings.Trim(f, "`\"")
			pkMap[f] = true
		}
	}

	return pkMap
}

func (p *DDLParser) parseColumnLine(line string) *modelai.ColumnInfo {
	line = strings.TrimRight(line, ",")

	upper := strings.ToUpper(line)
	if strings.HasPrefix(upper, "PRIMARY") ||
		strings.HasPrefix(upper, "UNIQUE") ||
		strings.HasPrefix(upper, "KEY") ||
		strings.HasPrefix(upper, "INDEX") ||
		strings.HasPrefix(upper, "CONSTRAINT") ||
		strings.HasPrefix(upper, "CHECK") ||
		strings.HasPrefix(upper, "USING") ||
		strings.HasPrefix(upper, "STORAGE") ||
		strings.HasPrefix(upper, "TABLESPACE") ||
		strings.HasPrefix(upper, "LOB") ||
		strings.HasPrefix(upper, "PARALLEL") ||
		strings.HasPrefix(upper, "NOCOMPRESS") ||
		strings.HasPrefix(upper, "COMPRESS") ||
		strings.HasPrefix(upper, "LOGGING") ||
		strings.HasPrefix(upper, "NOLOGGING") ||
		strings.HasPrefix(upper, "CACHE") ||
		strings.HasPrefix(upper, "NOCACHE") ||
		strings.HasPrefix(upper, "PCTFREE") ||
		strings.HasPrefix(upper, "PCTUSED") ||
		strings.HasPrefix(upper, "INITRANS") ||
		strings.HasPrefix(upper, "MAXTRANS") ||
		strings.HasPrefix(upper, "INITIAL") ||
		strings.HasPrefix(upper, "NEXT") ||
		strings.HasPrefix(upper, "MINEXTENTS") ||
		strings.HasPrefix(upper, "MAXEXTENTS") ||
		strings.HasPrefix(upper, "PCTINCREASE") ||
		strings.HasPrefix(upper, "FREELISTS") ||
		strings.HasPrefix(upper, "FREELIST") ||
		strings.HasPrefix(upper, "BUFFER_POOL") ||
		strings.HasPrefix(upper, "FLASH_CACHE") ||
		strings.HasPrefix(upper, "CELL_FLASH") ||
		strings.HasPrefix(upper, "SEGMENT") ||
		strings.HasPrefix(upper, "ENABLE") ||
		strings.HasPrefix(upper, "DEFERRABLE") ||
		strings.HasPrefix(upper, "INITIALLY") ||
		strings.HasPrefix(upper, "ORGANIZATION") ||
		strings.HasPrefix(upper, "OVERFLOW") ||
		strings.HasPrefix(upper, "PCTTHRESHOLD") ||
		strings.HasPrefix(upper, "RETENTION") ||
		strings.HasPrefix(upper, "KEEP") ||
		strings.HasPrefix(upper, "DUPLICATES") ||
		strings.HasPrefix(upper, "CHUNK") ||
		strings.HasPrefix(upper, "DEALLOCATE") ||
		strings.HasPrefix(upper, "SIZE") ||
		strings.HasPrefix(upper, "AUTO") ||
		strings.HasPrefix(upper, "ALLOCATE") {
		return nil
	}

	trimmed := strings.TrimSpace(line)
	if trimmed == "" || trimmed == ")" || trimmed == ";" {
		return nil
	}

	re := regexp.MustCompile(`[` + "`" + `"]?(\w+)["` + "`" + `]?\s+(\w+(?:\([^)]*\))?)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) < 2 {
		return nil
	}

	col := &modelai.ColumnInfo{
		Name: matches[1],
		Type: matches[2],
	}

	upperLine := strings.ToUpper(line)

	// 检查内联 PRIMARY KEY
	if strings.Contains(upperLine, "PRIMARY KEY") {
		col.IsPK = true
	}

	col.IsNullable = !strings.Contains(upperLine, "NOT NULL") &&
		!strings.Contains(upperLine, "NOTNULL")

	if strings.Contains(upperLine, "AUTO_INCREMENT") {
		col.IsAutoInc = true
	}

	defaultRe := regexp.MustCompile(`(?i)DEFAULT\s+('(?:[^']|'')*'|\d+(?:\.\d+)?|NULL|CURRENT_TIMESTAMP(?:\s*\([^)]*\))?|TRUE|FALSE|b'(?:[01]|.*)'|NULL(?:ABLE)?(?:;|,|\s|$))`)
	defaultMatches := defaultRe.FindStringSubmatch(line)
	if len(defaultMatches) > 1 {
		col.DefaultValue = defaultMatches[1]
	}

	commentRe := regexp.MustCompile(`(?i)COMMENT\s+'([^']*)'`)
	commentMatches := commentRe.FindStringSubmatch(line)
	if len(commentMatches) > 1 {
		col.Comment = commentMatches[1]
	}

	charsetRe := regexp.MustCompile(`(?i)(?:CHARACTER\s+SET|CHARSET)\s+(\w+)`)
	charsetMatches := charsetRe.FindStringSubmatch(line)
	if len(charsetMatches) > 1 {
		col.CharSet = charsetMatches[1]
	}

	return col
}

// extractConstraints 提取表级约束（唯一键、外键、索引、检查约束）
func (p *DDLParser) extractConstraints(ddl string, tableName string) modelai.TableConstraintList {
	var constraints modelai.TableConstraintList

	uniqueConstraints := p.extractUniqueConstraints(ddl, tableName)
	constraints = append(constraints, uniqueConstraints...)

	fkConstraints := p.extractForeignKeys(ddl, tableName)
	constraints = append(constraints, fkConstraints...)

	indexConstraints := p.extractIndexes(ddl, tableName)
	constraints = append(constraints, indexConstraints...)

	return constraints
}

// extractUniqueConstraints 提取 UNIQUE 约束
func (p *DDLParser) extractUniqueConstraints(ddl string, tableName string) []modelai.TableConstraint {
	var constraints []modelai.TableConstraint

	uniques := p.extractConstraintBlocks(ddl, `(?i)(?:CONSTRAINT\s+(\w+)\s+)?UNIQUE\s*(?:KEY\s+(\w+))?\s*\(([^)]+)\)`)
	for _, m := range uniques {
		name := m[1]
		if name == "" {
			name = m[2]
		}
		if name == "" {
			name = tableName + "_unique"
		}
		constraints = append(constraints, modelai.TableConstraint{
			TableName:      tableName,
			ConstraintType: "UNIQUE",
			ConstraintName: name,
			Columns:        p.parseConstraintColumns(m[3]),
		})
	}

	uniqueInlineRe := regexp.MustCompile(`(?i)UNIQUE\s+\(([^)]+)\)`)
	inlineMatches := uniqueInlineRe.FindAllStringSubmatch(ddl, -1)
	for _, m := range inlineMatches {
		alreadyExists := false
		cols := p.parseConstraintColumns(m[1])
		for _, c := range constraints {
			if c.ConstraintType == "UNIQUE" && equalStringSlice(c.Columns, cols) {
				alreadyExists = true
				break
			}
		}
		if !alreadyExists {
			constraints = append(constraints, modelai.TableConstraint{
				TableName:      tableName,
				ConstraintType: "UNIQUE",
				ConstraintName: tableName + "_unique",
				Columns:        cols,
			})
		}
	}

	return constraints
}

// extractConstraintBlocks 提取通用的约束块
func (p *DDLParser) extractConstraintBlocks(ddl string, pattern string) [][]string {
	re := regexp.MustCompile(pattern)
	return re.FindAllStringSubmatch(ddl, -1)
}

func (p *DDLParser) parseConstraintColumns(colStr string) []string {
	cols := strings.Split(colStr, ",")
	var result []string
	for _, c := range cols {
		c = strings.TrimSpace(c)
		c = strings.Trim(c, "`\" ")
		if c != "" {
			result = append(result, c)
		}
	}
	return result
}

// extractForeignKeys 提取外键约束
func (p *DDLParser) extractForeignKeys(ddl string, tableName string) []modelai.TableConstraint {
	var constraints []modelai.TableConstraint

	fkRe := regexp.MustCompile(`(?i)(?:CONSTRAINT\s+(\w+)\s+)?FOREIGN\s+KEY\s*\(([^)]+)\)\s*REFERENCES\s+` + "`?" + `?(\w+)` + "`?" + `?\s*\(([^)]+)\)`)
	matches := fkRe.FindAllStringSubmatch(ddl, -1)
	for _, m := range matches {
		constraints = append(constraints, modelai.TableConstraint{
			TableName:      tableName,
			ConstraintType: "FOREIGN",
			ConstraintName: m[1],
			Columns:        p.parseConstraintColumns(m[2]),
			RefTable:       strings.Trim(m[3], "`\""),
			RefColumns:     p.parseConstraintColumns(m[4]),
		})
	}

	return constraints
}

// extractIndexes 提取索引定义
func (p *DDLParser) extractIndexes(ddl string, tableName string) []modelai.TableConstraint {
	var constraints []modelai.TableConstraint

	indexRe := regexp.MustCompile(`(?i)(?:KEY|INDEX)\s+(?:` + "`" + `?(\w+)` + "`" + `?)?\s*\(([^)]+)\)(?:\s*USING\s+(\w+))?(?:\s*COMMENT\s+'([^']*)')?`)
	matches := indexRe.FindAllStringSubmatch(ddl, -1)
	for _, m := range matches {
		indexName := m[1]
		if indexName == "" {
			indexName = tableName + "_idx"
		}
		indexType := m[3]
		if indexType == "" {
			indexType = "BTREE"
		}
		constraints = append(constraints, modelai.TableConstraint{
			TableName:      tableName,
			ConstraintType: "INDEX",
			ConstraintName: indexName,
			Columns:        p.parseConstraintColumns(m[2]),
			IndexType:      indexType,
			Comment:        m[4],
		})
	}

	return constraints
}

func (p *DDLParser) InferRelations(caches []modelai.TableCache) []modelai.TableRelation {
	var relations []modelai.TableRelation

	tableIndex := make(map[string]*modelai.TableCache)
	for i := range caches {
		tableIndex[caches[i].TableNam] = &caches[i]
	}

	tableRelationMap := make(map[string]bool)

	for _, cache := range caches {
		for _, constraint := range cache.ConstraintsJSON {
			if constraint.ConstraintType == "FOREIGN" {
				key := constraint.TableName + "." + constraint.RefTable + "." + strings.Join(constraint.Columns, ",") + "." + strings.Join(constraint.RefColumns, ",")
				if tableRelationMap[key] {
					continue
				}
				tableRelationMap[key] = true

				rel := modelai.TableRelation{
					SourceTable:  constraint.TableName,
					SourceField:  strings.Join(constraint.Columns, ","),
					TargetTable:  constraint.RefTable,
					TargetField:  strings.Join(constraint.RefColumns, ","),
					RelationType: "many_to_one",
					Confidence:   1.0,
				}
				relations = append(relations, rel)
			}
		}

		for _, col := range cache.ColumnsJSON {
			if p.isIDField(col.Name) {
				continue
			}

			if strings.HasSuffix(col.Name, "_id") {
				relatedTable := strings.TrimSuffix(col.Name, "_id")
				relatedTable = p.toTableName(relatedTable)

				if _, exists := tableIndex[relatedTable]; exists {
					key := cache.TableNam + "." + relatedTable + "." + col.Name + ".id"
					if tableRelationMap[key] {
						continue
					}
					tableRelationMap[key] = true

					rel := modelai.TableRelation{
						SourceTable:  cache.TableNam,
						SourceField:  col.Name,
						TargetTable:  relatedTable,
						TargetField:  "id",
						RelationType: "many_to_one",
						Confidence:   0.9,
					}
					relations = append(relations, rel)
				}
			}
		}
	}

	return relations
}

func (p *DDLParser) isIDField(name string) bool {
	return name == "id" || name == "ID"
}

func (p *DDLParser) toTableName(singular string) string {
	re := regexp.MustCompile(`y$`)
	if re.MatchString(singular) {
		return re.ReplaceAllString(singular, "ies")
	}
	re2 := regexp.MustCompile(`(s|x|ch|sh|z)$`)
	if re2.MatchString(singular) {
		return singular + "es"
	}
	return singular + "s"
}

// ExtractDDLs 从 Markdown 内容中提取所有 CREATE TABLE 语句
func (p *DDLParser) ExtractDDLs(content string) []string {
	var ddls []string

	sqlBlockRe := regexp.MustCompile("(?s)```sql\\s*(.*?)\\s*```")
	matches := sqlBlockRe.FindAllStringSubmatch(content, -1)

	createTableRe := regexp.MustCompile("(?i)CREATE\\s+TABLE\\s+")

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		sqlContent := match[1]

		if createTableRe.MatchString(sqlContent) {
			ddls = append(ddls, sqlContent)
		}
	}

	return ddls
}

// ParseFromMarkdown 从 Markdown 内容中提取所有表结构
func (p *DDLParser) ParseFromMarkdown(content string) ([]*modelai.TableCache, error) {
	ddls := p.ExtractDDLs(content)

	var tables []*modelai.TableCache
	for _, ddl := range ddls {
		cache, err := p.ParseDDL(ddl)
		if err != nil {
			continue
		}
		if cache.TableNam != "" {
			tables = append(tables, cache)
		}
	}

	return tables, nil
}

// equalStringSlice 比较两个字符串切片是否相等（忽略顺序）
func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ma := make(map[string]int, len(a))
	for _, v := range a {
		ma[v]++
	}
	for _, v := range b {
		ma[v]--
		if ma[v] < 0 {
			return false
		}
	}
	return true
}
