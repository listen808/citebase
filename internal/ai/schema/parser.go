package schema

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/listen808/citebase/internal/text2sql"
)

// ParsedTable 保留类型定义以兼容现有代码
type ParsedTable struct {
	TableName    string
	TableComment string
	Engine       string
	Charset      string
	PrimaryKey   []string
	Columns      []ParsedColumn
	ForeignKeys  []ParsedForeignKey
	RawDDL       string
}

type ParsedColumn struct {
	Name         string
	DataType     string
	Length       int
	Nullable     bool
	DefaultValue string
	Comment      string
	IsPrimaryKey bool
	IsForeignKey bool
	IsIndex      bool
	Ordinal      int
}

type ParsedForeignKey struct {
	ColumnName    string
	RefTableName  string
	RefColumnName string
}

// DDLParser 薄包装器，委托给 text2sql.DDLParser
type DDLParser struct {
	inner *text2sql.DDLParser
}

func NewDDLParser() *DDLParser {
	return &DDLParser{inner: text2sql.NewDDLParser()}
}

// Parse 使用 text2sql parser 解析，转换为 ParsedTable
func (p *DDLParser) Parse(ddl string) (*ParsedTable, error) {
	cache, err := p.inner.ParseDDL(ddl)
	if err != nil {
		return nil, err
	}

	// 转换为 ParsedTable 格式
	pt := &ParsedTable{
		TableName:    cache.TableNam,
		TableComment: cache.TableComment,
		RawDDL:       ddl,
		PrimaryKey:   []string{},
		Columns:      []ParsedColumn{},
		ForeignKeys:  []ParsedForeignKey{},
	}

	// 从 DDL 中提取 Engine 和 Charset
	reEngine := regexp.MustCompile(`(?i)ENGINE\s*=\s*(\w+)`)
	if m := reEngine.FindStringSubmatch(ddl); len(m) > 1 {
		pt.Engine = m[1]
	}
	reCharset := regexp.MustCompile(`(?i)DEFAULT\s+CHARSET\s*=\s*(\w+)`)
	if m := reCharset.FindStringSubmatch(ddl); len(m) > 1 {
		pt.Charset = m[1]
	}

	// 转换列信息
	ordinal := 1
	for _, col := range cache.ColumnsJSON {
		// 解析数据类型和长度
		dataType, length := parseColumnType(col.Type)

		pt.Columns = append(pt.Columns, ParsedColumn{
			Name:         col.Name,
			DataType:     dataType,
			Length:       length,
			Nullable:     col.IsNullable,
			DefaultValue: col.DefaultValue,
			Comment:      col.Comment,
			IsPrimaryKey: col.IsPK,
			Ordinal:      ordinal,
		})
		ordinal++
	}

	// 标记主键字段
	for i := range pt.Columns {
		if pt.Columns[i].IsPrimaryKey {
			pt.PrimaryKey = append(pt.PrimaryKey, pt.Columns[i].Name)
		}
	}

	// 转换外键约束
	for _, c := range cache.ConstraintsJSON {
		if c.ConstraintType == "FOREIGN KEY" || c.ConstraintType == "FOREIGN" {
			for _, colName := range c.Columns {
				refCol := ""
				if len(c.RefColumns) > 0 {
					refCol = c.RefColumns[0]
				}
				pt.ForeignKeys = append(pt.ForeignKeys, ParsedForeignKey{
					ColumnName:    colName,
					RefTableName:  c.RefTable,
					RefColumnName: refCol,
				})
			}
		}
	}

	// 标记外键字段
	fkSet := make(map[string]bool)
	for _, fk := range pt.ForeignKeys {
		fkSet[fk.ColumnName] = true
	}
	for i := range pt.Columns {
		if fkSet[pt.Columns[i].Name] {
			pt.Columns[i].IsForeignKey = true
		}
	}

	return pt, nil
}

// parseColumnType 解析数据类型和长度
// 例如 "VARCHAR(50)" -> ("VARCHAR", 50)
// 例如 "INT" -> ("INT", 0)
func parseColumnType(typeStr string) (string, int) {
	typeStr = strings.TrimSpace(typeStr)

	// 查找括号
	idx := strings.Index(typeStr, "(")
	if idx == -1 {
		return typeStr, 0
	}

	dataType := typeStr[:idx]
	lengthStr := strings.Trim(typeStr[idx:], "()")

	length, _ := strconv.Atoi(lengthStr)
	return dataType, length
}

// ExtractSQLBlocks 从Markdown内容中提取SQL代码块
func ExtractSQLBlocks(markdown string) []string {
	re := regexp.MustCompile("(?s)```sql\\s*(.*?)\\s*```")
	matches := re.FindAllStringSubmatch(markdown, -1)
	var result []string
	for _, m := range matches {
		if len(m) > 1 {
			result = append(result, m[1])
		}
	}
	return result
}
