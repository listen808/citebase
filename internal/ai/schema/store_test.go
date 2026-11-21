package schema

import "testing"

func TestSplitName_Empty(t *testing.T) {
	result := splitName("")
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %v", result)
	}
}

func TestSplitName_Single(t *testing.T) {
	result := splitName("hello")
	if len(result) != 1 {
		t.Fatalf("expected 1 part, got %d", len(result))
	}
	if result[0] != "hello" {
		t.Errorf("expected 'hello', got %s", result[0])
	}
}

func TestSplitName_TwoParts(t *testing.T) {
	result := splitName("schema:table_overview")
	if len(result) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(result))
	}
	if result[0] != "schema" {
		t.Errorf("part 0: expected 'schema', got %s", result[0])
	}
	if result[1] != "table_overview" {
		t.Errorf("part 1: expected 'table_overview', got %s", result[1])
	}
}

func TestSplitName_ThreeParts(t *testing.T) {
	// "schema:column_detail:users"
	// split by :
	// schema | column_detail | users
	result := splitName("schema:column_detail:users")
	if len(result) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(result))
	}
	if result[0] != "schema" {
		t.Errorf("part 0: expected 'schema', got %s", result[0])
	}
	if result[1] != "column_detail" {
		t.Errorf("part 1: expected 'column_detail', got %s", result[1])
	}
	if result[2] != "users" {
		t.Errorf("part 2: expected 'users', got %s", result[2])
	}
}

func TestSplitName_FourParts(t *testing.T) {
	result := splitName("schema:column_detail:orders:id")
	if len(result) != 4 {
		t.Fatalf("expected 4 parts, got %d", len(result))
	}
	expected := []string{"schema", "column_detail", "orders", "id"}
	for i, exp := range expected {
		if result[i] != exp {
			t.Errorf("part %d: expected %q, got %q", i, exp, result[i])
		}
	}
}

func TestSplitName_LeadingColon(t *testing.T) {
	result := splitName(":abc:def")
	if len(result) != 3 {
		t.Fatalf("expected 3 parts (empty + abc + def), got %d", len(result))
	}
	if result[0] != "" {
		t.Errorf("part 0: expected empty, got %q", result[0])
	}
}

func TestSplitName_TrailingColon(t *testing.T) {
	// With trailing colon, current becomes "" at end, but splitName only appends current != ""
	// Wait let me check:
	// for "abc:" -> a b c :
	// current = "abc"
	// then ch == ':', append "abc", current = ""
	// then loop ends, current is "", so not appended
	// So result = ["abc"]
	result := splitName("abc:")
	if len(result) != 1 {
		t.Fatalf("expected 1 part, got %d", len(result))
	}
	if result[0] != "abc" {
		t.Errorf("expected 'abc', got %s", result[0])
	}
}

func TestSplitName_OnlyColons(t *testing.T) {
	result := splitName("::")
	// first : -> append "" (current=""), current=""
	// second : -> append "" (current=""), current=""
	// end: current="" not appended
	if len(result) != 2 {
		t.Fatalf("expected 2 empty parts, got %d", len(result))
	}
}

func TestVectorStore_IsReady_Nil(t *testing.T) {
	vs := &VectorStore{schemaVS: nil}
	if vs.IsReady() {
		t.Error("expected IsReady=false for nil schemaVS")
	}
}

func TestNewVectorStore(t *testing.T) {
	vs := NewVectorStore(nil)
	if vs == nil {
		t.Fatal("expected non-nil VectorStore")
	}
	if vs.IsReady() {
		t.Error("expected IsReady=false when passed nil")
	}
}

func TestSearchResult_Struct(t *testing.T) {
	sr := &SearchResult{
		ChunkType:  "table_overview",
		TableName:  "users",
		ColumnName: "id",
		Content:    "test",
		Score:      0.95,
	}
	if sr.ChunkType != "table_overview" {
		t.Error("ChunkType mismatch")
	}
	if sr.Score != 0.95 {
		t.Error("Score mismatch")
	}
}
