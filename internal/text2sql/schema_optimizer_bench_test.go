package text2sql

import (
	"strings"
	"testing"
	"time"

	modelai "github.com/listen808/citebase/internal/model/ai"
)

func TestSchemaOptimizer_ContextLengthReduction(t *testing.T) {
	// 模拟真实场景的表数据
	tables := map[string]*modelai.TableCache{
		"user": {
			TableNam:     "user",
			TableComment: "用户表",
			DDLContent:   "CREATE TABLE user (id INT PRIMARY KEY, name VARCHAR(100), email VARCHAR(200), created_at TIMESTAMP)",
			ColumnsJSON: []modelai.ColumnInfo{
				{Name: "id", Type: "INT", Comment: "用户ID", IsPK: true},
				{Name: "name", Type: "VARCHAR(100)", Comment: "用户名"},
				{Name: "email", Type: "VARCHAR(200)", Comment: "邮箱"},
				{Name: "created_at", Type: "TIMESTAMP", Comment: "创建时间"},
			},
		},
		"order": {
			TableNam:     "order",
			TableComment: "订单表",
			DDLContent:   "CREATE TABLE order (id INT PRIMARY KEY, user_id INT, amount DECIMAL(10,2), status VARCHAR(20))",
			ColumnsJSON: []modelai.ColumnInfo{
				{Name: "id", Type: "INT", Comment: "订单ID", IsPK: true},
				{Name: "user_id", Type: "INT", Comment: "用户ID"},
				{Name: "amount", Type: "DECIMAL(10,2)", Comment: "金额"},
				{Name: "status", Type: "VARCHAR(20)", Comment: "状态"},
			},
		},
	}

	// 测试优化后的上下文长度
	optimizer := NewSchemaOptimizer()
	keywords := []string{"用户", "订单"}

	start := time.Now()
	optimizedContext := optimizer.BuildOptimizedContext("查询用户订单", tables, keywords)
	duration := time.Since(start)

	t.Logf("Optimized context length: %d chars", len(optimizedContext))
	t.Logf("Generation time: %v", duration)

	// 验证上下文长度在限制内
	if len(optimizedContext) > 5000 {
		t.Errorf("Expected optimized context < 5000 chars, got %d", len(optimizedContext))
	}

	// 验证包含关键信息
	if !strings.Contains(optimizedContext, "user") || !strings.Contains(optimizedContext, "order") {
		t.Error("Expected context to contain both user and order tables")
	}
}

func BenchmarkSchemaOptimizer_BuildOptimizedContext(b *testing.B) {
	tables := map[string]*modelai.TableCache{
		"user": {
			TableNam:     "user",
			TableComment: "用户表",
			DDLContent:   "CREATE TABLE user (id INT PRIMARY KEY, name VARCHAR(100), email VARCHAR(200))",
			ColumnsJSON: []modelai.ColumnInfo{
				{Name: "id", Type: "INT", Comment: "用户ID", IsPK: true},
				{Name: "name", Type: "VARCHAR(100)", Comment: "用户名"},
				{Name: "email", Type: "VARCHAR(200)", Comment: "邮箱"},
			},
		},
	}

	optimizer := NewSchemaOptimizer()
	keywords := []string{"用户"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		optimizer.BuildOptimizedContext("查询用户", tables, keywords)
	}
}

func TestSchemaOptimizer_LargeTableSet(t *testing.T) {
	// 测试大量表的情况
	tables := make(map[string]*modelai.TableCache)
	for i := 0; i < 50; i++ {
		tableName := "table_" + string(rune('a'+i%26))
		tables[tableName] = &modelai.TableCache{
			TableNam:     tableName,
			TableComment: "表注释 " + tableName,
			DDLContent:   "CREATE TABLE " + tableName + " (id INT PRIMARY KEY, name VARCHAR(100))",
			ColumnsJSON: []modelai.ColumnInfo{
				{Name: "id", Type: "INT", Comment: "ID", IsPK: true},
				{Name: "name", Type: "VARCHAR(100)", Comment: "名称"},
			},
		}
	}

	optimizer := NewSchemaOptimizer()
	keywords := []string{"table"}

	start := time.Now()
	optimizedContext := optimizer.BuildOptimizedContext("查询表", tables, keywords)
	duration := time.Since(start)

	t.Logf("Large table set - Optimized context length: %d chars", len(optimizedContext))
	t.Logf("Large table set - Generation time: %v", duration)

	// 验证上下文长度在限制内
	if len(optimizedContext) > 10000 {
		t.Errorf("Expected optimized context < 10000 chars, got %d", len(optimizedContext))
	}
}

func TestSchemaOptimizer_KeywordMatching(t *testing.T) {
	// 测试关键词匹配
	tables := map[string]*modelai.TableCache{
		"user": {
			TableNam:     "user",
			TableComment: "用户表",
			DDLContent:   "CREATE TABLE user (id INT PRIMARY KEY, name VARCHAR(100))",
			ColumnsJSON: []modelai.ColumnInfo{
				{Name: "id", Type: "INT", Comment: "用户ID", IsPK: true},
				{Name: "name", Type: "VARCHAR(100)", Comment: "用户名"},
			},
		},
		"product": {
			TableNam:     "product",
			TableComment: "产品表",
			DDLContent:   "CREATE TABLE product (id INT PRIMARY KEY, name VARCHAR(100))",
			ColumnsJSON: []modelai.ColumnInfo{
				{Name: "id", Type: "INT", Comment: "产品ID", IsPK: true},
				{Name: "name", Type: "VARCHAR(100)", Comment: "产品名称"},
			},
		},
	}

	optimizer := NewSchemaOptimizer()

	// 测试查询"用户"应该优先返回user表
	keywords := []string{"用户"}
	context := optimizer.BuildOptimizedContext("查询用户", tables, keywords)

	if !strings.Contains(context, "user") {
		t.Error("Expected context to contain user table for query about users")
	}

	// 测试查询"产品"应该优先返回product表
	keywords = []string{"产品"}
	context = optimizer.BuildOptimizedContext("查询产品", tables, keywords)

	if !strings.Contains(context, "product") {
		t.Error("Expected context to contain product table for query about products")
	}
}
