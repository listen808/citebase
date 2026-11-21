package text2sql

import (
	"strings"
	"testing"

	modelai "github.com/listen808/citebase/internal/model/ai"
)

func TestSchemaOptimizer_CalculateTableRelevance(t *testing.T) {
	optimizer := NewSchemaOptimizer()

	// 测试用例1：表名完全匹配
	score := optimizer.CalculateTableRelevance("user", "用户表", []string{"user", "查询"})
	if score < 10 {
		t.Errorf("Expected score >= 10 for exact table name match, got %v", score)
	}

	// 测试用例2：注释匹配
	score = optimizer.CalculateTableRelevance("sys_user", "系统用户表", []string{"用户"})
	if score < 5 {
		t.Errorf("Expected score >= 5 for comment match, got %v", score)
	}

	// 测试用例3：无匹配
	score = optimizer.CalculateTableRelevance("config", "配置表", []string{"用户"})
	if score != 0 {
		t.Errorf("Expected score 0 for no match, got %v", score)
	}
}

func TestSchemaOptimizer_SelectTopTables(t *testing.T) {
	optimizer := NewSchemaOptimizer()

	tables := []TableScore{
		{Name: "user", Score: 15},
		{Name: "order", Score: 10},
		{Name: "product", Score: 8},
		{Name: "config", Score: 3},
		{Name: "log", Score: 1},
	}

	selected := optimizer.SelectTopTables(tables, 3)
	if len(selected) != 3 {
		t.Errorf("Expected 3 tables, got %d", len(selected))
	}
	if selected[0].Name != "user" {
		t.Errorf("Expected first table to be 'user', got %s", selected[0].Name)
	}
}

func TestSchemaOptimizer_BuildCompactContext(t *testing.T) {
	optimizer := NewSchemaOptimizer()

	// 测试紧凑上下文生成
	context := optimizer.BuildCompactContext("user", "用户表", []string{"id", "name", "email"})

	if len(context) == 0 {
		t.Error("Expected non-empty context")
	}
	if len(context) > 500 {
		t.Errorf("Expected compact context < 500 chars, got %d", len(context))
	}
}

func TestGenerator_BuildOptimizedSchemaContext(t *testing.T) {
	// 创建测试用的KnowledgeBase
	kb := &KnowledgeBase{
		tables: map[string]*modelai.TableCache{
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
			"order": {
				TableNam:     "order",
				TableComment: "订单表",
				DDLContent:   "CREATE TABLE order (id INT PRIMARY KEY, user_id INT, amount DECIMAL(10,2))",
				ColumnsJSON: []modelai.ColumnInfo{
					{Name: "id", Type: "INT", Comment: "订单ID", IsPK: true},
					{Name: "user_id", Type: "INT", Comment: "用户ID"},
					{Name: "amount", Type: "DECIMAL(10,2)", Comment: "金额"},
				},
			},
		},
		rels: make(map[string][]modelai.TableRelation),
	}

	optimizer := NewSchemaOptimizer()

	// 测试优化后的上下文构建
	context := optimizer.BuildOptimizedContext("查询用户订单", kb.GetAllTables(), []string{"用户", "订单"})

	if len(context) == 0 {
		t.Error("Expected non-empty context")
	}

	// 验证上下文长度在限制内
	if len(context) > 5000 {
		t.Errorf("Expected context < 5000 chars, got %d", len(context))
	}

	// 验证包含关键信息
	if !strings.Contains(context, "user") || !strings.Contains(context, "order") {
		t.Error("Expected context to contain both user and order tables")
	}
}
