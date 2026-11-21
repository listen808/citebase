package text2sql

import (
	"testing"
)

func TestSpaceScanner_ScanDocument(t *testing.T) {
	parser := NewDDLParser()

	content := `
# 用户表

这是一个用户表的定义。

` + "```sql" + `
CREATE TABLE ` + "`users`" + ` (
  ` + "`id`" + ` bigint NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  ` + "`username`" + ` varchar(50) NOT NULL COMMENT '用户名',
  ` + "`email`" + ` varchar(100) DEFAULT NULL COMMENT '邮箱',
  PRIMARY KEY (` + "`id`" + `)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户表';
` + "```" + `

# 订单表

` + "```sql" + `
CREATE TABLE ` + "`orders`" + ` (
  ` + "`id`" + ` bigint NOT NULL AUTO_INCREMENT COMMENT '订单ID',
  ` + "`user_id`" + ` bigint NOT NULL COMMENT '用户ID',
  ` + "`amount`" + ` decimal(10,2) NOT NULL COMMENT '订单金额',
  PRIMARY KEY (` + "`id`" + `)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单表';
` + "```" + `
`

	tables, err := parser.ParseFromMarkdown(content)
	if err != nil {
		t.Fatalf("ParseFromMarkdown failed: %v", err)
	}

	if len(tables) != 2 {
		t.Fatalf("Expected 2 tables, got %d", len(tables))
	}

	if tables[0].TableNam != "users" {
		t.Errorf("Expected table name 'users', got '%s'", tables[0].TableNam)
	}
	if tables[0].TableComment != "用户表" {
		t.Errorf("Expected table comment '用户表', got '%s'", tables[0].TableComment)
	}
	if len(tables[0].ColumnsJSON) != 3 {
		t.Errorf("Expected 3 columns for users table, got %d", len(tables[0].ColumnsJSON))
	}

	if tables[1].TableNam != "orders" {
		t.Errorf("Expected table name 'orders', got '%s'", tables[1].TableNam)
	}
}

func TestDDLParser_ExtractDDLs(t *testing.T) {
	parser := NewDDLParser()

	content := `
# 数据库设计

` + "```sql" + `
CREATE TABLE ` + "`users`" + ` (
  ` + "`id`" + ` bigint NOT NULL
) COMMENT='用户表';
` + "```" + `

一些说明文字...

` + "```sql" + `
CREATE TABLE ` + "`orders`" + ` (
  ` + "`id`" + ` bigint NOT NULL
) COMMENT='订单表';
` + "```" + `
`

	ddls := parser.ExtractDDLs(content)
	if len(ddls) != 2 {
		t.Fatalf("Expected 2 DDLs, got %d", len(ddls))
	}
}

func TestDDLParser_ExtractDDLs_NoSQL(t *testing.T) {
	parser := NewDDLParser()

	content := `
# 标题

一些普通文字，没有 SQL 代码块。
`

	ddls := parser.ExtractDDLs(content)
	if len(ddls) != 0 {
		t.Fatalf("Expected 0 DDLs, got %d", len(ddls))
	}
}

func TestDDLParser_ExtractDDLs_NonCreateTable(t *testing.T) {
	parser := NewDDLParser()

	content := `
` + "```sql" + `
SELECT * FROM users;
` + "```" + `
`

	ddls := parser.ExtractDDLs(content)
	if len(ddls) != 0 {
		t.Fatalf("Expected 0 DDLs, got %d", len(ddls))
	}
}

func TestDDLParser_ParseFromMarkdown_EmptyContent(t *testing.T) {
	parser := NewDDLParser()

	tables, err := parser.ParseFromMarkdown("")
	if err != nil {
		t.Fatalf("ParseFromMarkdown failed: %v", err)
	}
	if len(tables) != 0 {
		t.Fatalf("Expected 0 tables, got %d", len(tables))
	}
}
