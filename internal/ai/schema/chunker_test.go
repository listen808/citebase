package schema

import "testing"

func TestNewChunker(t *testing.T) {
	c := NewChunker()
	if c == nil {
		t.Fatal("expected non-nil Chunker")
	}
}

func TestCreateChunksFromParsedTable_EmptyColumns(t *testing.T) {
	c := NewChunker()
	table := &ParsedTable{
		TableName:    "users",
		TableComment: "用户表",
	}
	chunks := c.CreateChunksFromParsedTable(1, table)
	if len(chunks) != 1 {
		t.Errorf("expected 1 chunk (overview only), got %d", len(chunks))
	}
	if chunks[0].ChunkType != "table_overview" {
		t.Errorf("expected table_overview, got %s", chunks[0].ChunkType)
	}
	if chunks[0].TableName != "users" {
		t.Errorf("expected table name 'users', got %s", chunks[0].TableName)
	}
	if chunks[0].SchemaSpaceID != 1 {
		t.Errorf("expected space id 1, got %d", chunks[0].SchemaSpaceID)
	}
}

func TestCreateChunksFromParsedTable_WithColumns(t *testing.T) {
	c := NewChunker()
	table := &ParsedTable{
		TableName:    "orders",
		TableComment: "订单表",
		Columns: []ParsedColumn{
			{Name: "id", DataType: "bigint"},
			{Name: "user_id", DataType: "bigint"},
			{Name: "amount", DataType: "decimal"},
		},
	}
	chunks := c.CreateChunksFromParsedTable(42, table)
	// 1 overview + 3 columns = 4
	expected := 4
	if len(chunks) != expected {
		t.Errorf("expected %d chunks, got %d", expected, len(chunks))
	}

	// first chunk should be overview
	if chunks[0].ChunkType != "table_overview" {
		t.Errorf("first chunk should be table_overview, got %s", chunks[0].ChunkType)
	}

	// remaining chunks should be column details
	colChunks := chunks[1:]
	for i, col := range table.Columns {
		if colChunks[i].ChunkType != "column_detail" {
			t.Errorf("chunk %d: expected column_detail, got %s", i, colChunks[i].ChunkType)
		}
		if colChunks[i].ColumnName != col.Name {
			t.Errorf("chunk %d: expected column %s, got %s", i, col.Name, colChunks[i].ColumnName)
		}
		if colChunks[i].TableName != "orders" {
			t.Errorf("chunk %d: expected table orders, got %s", i, colChunks[i].TableName)
		}
	}
}

func TestCreateChunksFromParsedTable_Metadata(t *testing.T) {
	c := NewChunker()
	table := &ParsedTable{
		TableName:    "t1",
		TableComment: "",
		Columns:      []ParsedColumn{{Name: "c1", DataType: "int"}},
	}
	chunks := c.CreateChunksFromParsedTable(99, table)
	for i, ch := range chunks {
		if ch.Metadata == nil {
			t.Errorf("chunk %d: expected non-nil metadata", i)
			continue
		}
		if ch.Metadata["docID"] != "99" {
			t.Errorf("chunk %d: expected docID=99, got %s", i, ch.Metadata["docID"])
		}
	}
}

func TestCreateChunksFromParsedTable_ContentOverview(t *testing.T) {
	c := NewChunker()
	table := &ParsedTable{
		TableName:    "products",
		TableComment: "商品信息表",
	}
	chunks := c.CreateChunksFromParsedTable(1, table)
	expectedContent := "表名: products\n描述: 商品信息表"
	if chunks[0].Content != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, chunks[0].Content)
	}
}

func TestCreateChunksFromParsedTable_ColumnContent(t *testing.T) {
	c := NewChunker()
	table := &ParsedTable{
		TableName: "products",
		Columns: []ParsedColumn{
			{Name: "price", DataType: "decimal(10,2)"},
		},
	}
	chunks := c.CreateChunksFromParsedTable(1, table)
	expected := "表 products 的列 price: decimal(10,2)"
	if chunks[1].Content != expected {
		t.Errorf("expected %q, got %q", expected, chunks[1].Content)
	}
}

func TestSchemaChunk_Struct(t *testing.T) {
	ch := &SchemaChunk{
		ChunkType:     "table_overview",
		SchemaSpaceID: 1,
		TableName:     "t",
		ColumnName:    "c",
		Content:       "content",
		Metadata:      map[string]string{"k": "v"},
	}
	if ch.ChunkType != "table_overview" {
		t.Error("ChunkType mismatch")
	}
	if ch.Metadata["k"] != "v" {
		t.Error("Metadata mismatch")
	}
}
