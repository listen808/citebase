package schema

import (
	"fmt"

	"github.com/listen808/citebase/internal/text2sql"
)

// SchemaChunk 分块结构
type SchemaChunk struct {
	ChunkType     string            `json:"chunk_type"`
	SchemaSpaceID int               `json:"schema_space_id"`
	TableName     string            `json:"table_name"`
	ColumnName    string            `json:"column_name,omitempty"`
	Content       string            `json:"content"`
	Metadata      map[string]string `json:"metadata"`
}

// Chunker 分块器
type Chunker struct{}

func NewChunker() *Chunker {
	return &Chunker{}
}

// CreateChunks 使用 text2sql parser 解析后创建 chunks
func (c *Chunker) CreateChunks(spaceID int, ddl string) ([]*SchemaChunk, error) {
	parser := text2sql.NewDDLParser()
	cache, err := parser.ParseDDL(ddl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DDL: %w", err)
	}

	var chunks []*SchemaChunk
	meta := map[string]string{"docID": fmt.Sprintf("%d", spaceID)}

	// 表概览 chunk
	overview := fmt.Sprintf("表名: %s\n描述: %s", cache.TableNam, cache.TableComment)
	chunks = append(chunks, &SchemaChunk{
		ChunkType:     "table_overview",
		SchemaSpaceID: spaceID,
		TableName:     cache.TableNam,
		Content:       overview,
		Metadata:      meta,
	})

	return chunks, nil
}

// CreateChunksFromParsedTable 保持向后兼容
func (c *Chunker) CreateChunksFromParsedTable(spaceID int, table *ParsedTable) []*SchemaChunk {
	var chunks []*SchemaChunk
	meta := map[string]string{"docID": fmt.Sprintf("%d", spaceID)}

	overview := fmt.Sprintf("表名: %s\n描述: %s", table.TableName, table.TableComment)
	chunks = append(chunks, &SchemaChunk{
		ChunkType:     "table_overview",
		SchemaSpaceID: spaceID,
		TableName:     table.TableName,
		Content:       overview,
		Metadata:      meta,
	})

	for _, col := range table.Columns {
		colContent := fmt.Sprintf("表 %s 的列 %s: %s", table.TableName, col.Name, col.DataType)
		chunks = append(chunks, &SchemaChunk{
			ChunkType:     "column_detail",
			SchemaSpaceID: spaceID,
			TableName:     table.TableName,
			ColumnName:    col.Name,
			Content:       colContent,
			Metadata:      meta,
		})
	}

	return chunks
}
