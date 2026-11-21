package text2sql

import (
	"fmt"
	"strings"

	modelai "github.com/listen808/citebase/internal/model/ai"
)

// SQLSchemaChunk 代表一个表的完整描述，用于 RAG 嵌入
type SQLSchemaChunk struct {
	TableName    string `json:"table_name"`
	TableComment string `json:"table_comment"`
	DDLContent   string `json:"ddl_content"`
	RichText     string `json:"rich_text"`
	ChunkType    string `json:"chunk_type"` // "table" 或 "field"
	FieldName    string `json:"field_name,omitempty"`
	DocID        int    `json:"doc_id"`
	SpaceID      int    `json:"space_id"`
	DocName      string `json:"doc_name"`
}

// SQLSchemaChunker 从文档中提取表级别的 chunks
type SQLSchemaChunker struct {
	parser *DDLParser
}

// NewSQLSchemaChunker 创建 SQL Schema 分块器
func NewSQLSchemaChunker() *SQLSchemaChunker {
	return &SQLSchemaChunker{
		parser: NewDDLParser(),
	}
}

// ChunkDocument 从 Markdown 内容中提取每个 CREATE TABLE 作为一个独立 chunk
func (c *SQLSchemaChunker) ChunkDocument(content string, docID, spaceID int, docName string) []SQLSchemaChunk {
	ddls := c.parser.ExtractDDLs(content)
	if len(ddls) == 0 {
		return nil
	}

	var chunks []SQLSchemaChunk
	for _, ddl := range ddls {
		cache, err := c.parser.ParseDDL(ddl)
		if err != nil || cache.TableNam == "" {
			continue
		}

		tableChunk := SQLSchemaChunk{
			TableName:    cache.TableNam,
			TableComment: cache.TableComment,
			DDLContent:   ddl,
			RichText:     c.BuildRichText(cache, nil),
			ChunkType:    "table",
			DocID:        docID,
			SpaceID:      spaceID,
			DocName:      docName,
		}
		chunks = append(chunks, tableChunk)

		fieldChunks := c.ChunkFields(cache, docID, spaceID, docName)
		chunks = append(chunks, fieldChunks...)
	}

	return chunks
}

// ChunkFields 将表的每个字段生成为独立 chunk
func (c *SQLSchemaChunker) ChunkFields(cache *modelai.TableCache, docID, spaceID int, docName string) []SQLSchemaChunk {
	var chunks []SQLSchemaChunk

	for _, col := range cache.ColumnsJSON {
		fieldText := c.buildFieldRichText(cache, col)
		chunk := SQLSchemaChunk{
			TableName:    cache.TableNam,
			TableComment: cache.TableComment,
			RichText:     fieldText,
			ChunkType:    "field",
			FieldName:    col.Name,
			DocID:        docID,
			SpaceID:      spaceID,
			DocName:      docName,
		}
		chunks = append(chunks, chunk)
	}

	return chunks
}

// buildFieldRichText 构建单个字段的丰富文本
func (c *SQLSchemaChunker) buildFieldRichText(cache *modelai.TableCache, col modelai.ColumnInfo) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("表: %s", cache.TableNam))
	if cache.TableComment != "" {
		parts = append(parts, fmt.Sprintf("表注释: %s", cache.TableComment))
	}

	line := fmt.Sprintf("字段: %s (%s)", col.Name, col.Type)
	if col.Comment != "" {
		line += fmt.Sprintf(" - %s", col.Comment)
	}
	if col.IsPK {
		line += " [主键]"
	}
	if !col.IsNullable {
		line += " [非空]"
	}
	if col.IsAutoInc {
		line += " [自增]"
	}
	if col.DefaultValue != "" {
		line += fmt.Sprintf(" [默认: %s]", col.DefaultValue)
	}
	parts = append(parts, line)

	if col.Comment != "" {
		parts = append(parts, fmt.Sprintf("业务含义: %s", col.Comment))
	}

	return strings.Join(parts, "\n")
}

// BuildRichText 生成包含表名、注释、字段列表、约束、关联关系的结构化文本
func (c *SQLSchemaChunker) BuildRichText(cache *modelai.TableCache, relations []modelai.TableRelation) string {
	var parts []string

	if cache.TableComment != "" {
		parts = append(parts, fmt.Sprintf("表名: %s (%s)", cache.TableNam, cache.TableComment))
	} else {
		parts = append(parts, fmt.Sprintf("表名: %s", cache.TableNam))
	}

	if len(cache.ColumnsJSON) > 0 {
		parts = append(parts, "字段:")
		for _, col := range cache.ColumnsJSON {
			line := fmt.Sprintf("- %s %s", col.Name, col.Type)
			if col.IsPK {
				line += " [PK]"
			}
			if !col.IsNullable {
				line += " [NOT NULL]"
			}
			if col.IsAutoInc {
				line += " [AUTO_INCREMENT]"
			}
			if col.DefaultValue != "" {
				line += fmt.Sprintf(" [DEFAULT: %s]", col.DefaultValue)
			}
			if col.Comment != "" {
				line += " " + col.Comment
			}
			parts = append(parts, line)
		}
	}

	if len(cache.ConstraintsJSON) > 0 {
		hasNonIndex := false
		for _, ct := range cache.ConstraintsJSON {
			if ct.ConstraintType != "INDEX" {
				hasNonIndex = true
				break
			}
		}
		if hasNonIndex {
			parts = append(parts, "约束:")
			for _, ct := range cache.ConstraintsJSON {
				if ct.ConstraintType == "INDEX" {
					continue
				}
				cols := strings.Join(ct.Columns, ",")
				switch ct.ConstraintType {
				case "UNIQUE":
					parts = append(parts, fmt.Sprintf("- UNIQUE (%s)", cols))
				case "FOREIGN":
					refCols := strings.Join(ct.RefColumns, ",")
					parts = append(parts, fmt.Sprintf("- FOREIGN KEY (%s) REFERENCES %s(%s)", cols, ct.RefTable, refCols))
				}
			}
		}

		hasIndex := false
		for _, ct := range cache.ConstraintsJSON {
			if ct.ConstraintType == "INDEX" {
				hasIndex = true
				break
			}
		}
		if hasIndex {
			parts = append(parts, "索引:")
			for _, ct := range cache.ConstraintsJSON {
				if ct.ConstraintType != "INDEX" {
					continue
				}
				cols := strings.Join(ct.Columns, ",")
				line := fmt.Sprintf("- INDEX %s (%s) USING %s", ct.ConstraintName, cols, ct.IndexType)
				if ct.Comment != "" {
					line += " " + ct.Comment
				}
				parts = append(parts, line)
			}
		}
	}

	if len(relations) > 0 {
		parts = append(parts, "关联关系:")
		for _, rel := range relations {
			line := fmt.Sprintf("- %s.%s → %s.%s (%s)",
				rel.SourceTable, rel.SourceField,
				rel.TargetTable, rel.TargetField,
				rel.RelationType)
			parts = append(parts, line)
		}
	}

	return strings.Join(parts, "\n")
}

// BuildRichTextWithRelations 使用知识库中的关系构建丰富文本
func (c *SQLSchemaChunker) BuildRichTextWithRelations(cache *modelai.TableCache, allRelations []modelai.TableRelation) string {
	var tableRelations []modelai.TableRelation
	for _, rel := range allRelations {
		if rel.SourceTable == cache.TableNam || rel.TargetTable == cache.TableNam {
			tableRelations = append(tableRelations, rel)
		}
	}
	return c.BuildRichText(cache, tableRelations)
}

// BuildIndexRichText 构建索引层面的丰富文本，用于查询优化建议
func (c *SQLSchemaChunker) BuildIndexRichText(cache *modelai.TableCache) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("表: %s", cache.TableNam))

	if len(cache.ConstraintsJSON) > 0 {
		for _, ct := range cache.ConstraintsJSON {
			if ct.ConstraintType != "INDEX" {
				continue
			}
			parts = append(parts, fmt.Sprintf("索引 %s: 字段 (%s), 类型: %s",
				ct.ConstraintName, strings.Join(ct.Columns, ","), ct.IndexType))
		}
	}

	return strings.Join(parts, "\n")
}
