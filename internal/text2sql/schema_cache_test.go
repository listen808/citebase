package text2sql

import (
	"testing"
)

func TestNewSchemaCachePool(t *testing.T) {
	pool := NewSchemaCachePool(1, 100)
	if pool == nil {
		t.Fatal("NewSchemaCachePool returned nil")
	}
	if pool.ConversationID != 1 {
		t.Errorf("ConversationID = %d, want 1", pool.ConversationID)
	}
	if pool.SpaceID != 100 {
		t.Errorf("SpaceID = %d, want 100", pool.SpaceID)
	}
	if pool.Tables == nil {
		t.Error("Tables should be initialized")
	}
}

func TestCacheTable(t *testing.T) {
	pool := NewSchemaCachePool(1, 100)
	columns := []ColumnInfo{
		{Name: "id", Type: "INT"},
		{Name: "name", Type: "VARCHAR"},
	}
	pool.CacheTable(100, "employees", columns, false)

	if len(pool.Tables) != 1 {
		t.Fatalf("Tables length = %d, want 1", len(pool.Tables))
	}
	cached := pool.Tables[schemaCacheKey(100, "employees")]
	if cached == nil {
		t.Fatal("Table 'employees' not found in cache")
	}
	if len(cached.Columns) != 2 {
		t.Errorf("Columns length = %d, want 2", len(cached.Columns))
	}
}

func TestGetCachedTable(t *testing.T) {
	pool := NewSchemaCachePool(1, 100)
	columns := []ColumnInfo{{Name: "id", Type: "INT"}}
	pool.CacheTable(100, "employees", columns, false)

	cached, ok := pool.GetCachedTable(100, "employees")
	if !ok {
		t.Fatal("GetCachedTable should return true for cached table")
	}
	if cached.Name != "employees" {
		t.Errorf("Name = %q, want %q", cached.Name, "employees")
	}

	_, ok = pool.GetCachedTable(100, "departments")
	if ok {
		t.Error("GetCachedTable should return false for uncached table")
	}
}

// 同名表在不同空间的分片必须互相隔离（跨空间串用回归）
func TestSchemaCacheSpaceIsolation(t *testing.T) {
	pool := NewSchemaCachePool(1, 100)
	pool.CacheTable(1, "orders", []ColumnInfo{{Name: "col_a", Type: "INT"}}, false)
	pool.CacheTable(2, "orders", []ColumnInfo{{Name: "col_b", Type: "INT"}}, false)

	c1, ok := pool.GetCachedTable(1, "orders")
	if !ok || len(c1.Columns) != 1 || c1.Columns[0].Name != "col_a" {
		t.Fatalf("space 1 'orders' = %+v, want col_a", c1)
	}
	c2, ok := pool.GetCachedTable(2, "orders")
	if !ok || len(c2.Columns) != 1 || c2.Columns[0].Name != "col_b" {
		t.Fatalf("space 2 'orders' = %+v, want col_b", c2)
	}
	names := pool.GetCachedTables(1)
	if len(names) != 1 || names[0] != "orders" {
		t.Errorf("GetCachedTables(1) = %v, want [orders]", names)
	}
}

func TestHasField(t *testing.T) {
	pool := NewSchemaCachePool(1, 100)
	columns := []ColumnInfo{
		{Name: "id", Type: "INT"},
		{Name: "name", Type: "VARCHAR"},
	}
	pool.CacheTable(100, "employees", columns, false)

	if !pool.HasField(100, "employees", "id") {
		t.Error("HasField should return true for existing field")
	}
	if pool.HasField(100, "employees", "salary") {
		t.Error("HasField should return false for non-existing field")
	}
	if pool.HasField(100, "departments", "id") {
		t.Error("HasField should return false for non-existing table")
	}
}

func TestConfirmTable(t *testing.T) {
	pool := NewSchemaCachePool(1, 100)
	columns := []ColumnInfo{{Name: "id", Type: "INT"}}
	pool.CacheTable(100, "employees", columns, false)

	pool.ConfirmTable(100, "employees")
	cached := pool.Tables[schemaCacheKey(100, "employees")]
	if !cached.Confirmed {
		t.Error("Table should be confirmed after ConfirmTable")
	}
}

func TestGetMissingFields(t *testing.T) {
	pool := NewSchemaCachePool(1, 100)
	columns := []ColumnInfo{
		{Name: "id", Type: "INT"},
		{Name: "name", Type: "VARCHAR"},
	}
	pool.CacheTable(100, "employees", columns, false)

	missing := pool.GetMissingFields(100, "employees", []string{"id", "name", "salary"})
	if len(missing) != 1 || missing[0] != "salary" {
		t.Errorf("GetMissingFields = %v, want [salary]", missing)
	}

	missing = pool.GetMissingFields(100, "employees", []string{"id", "name"})
	if len(missing) != 0 {
		t.Errorf("GetMissingFields should be empty, got %v", missing)
	}
}
