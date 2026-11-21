package schema

import (
	"testing"
)

func TestDDLParser_Parse(t *testing.T) {
	parser := NewDDLParser()

	ddl := `CREATE TABLE users (
		id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
		username VARCHAR(50) NOT NULL COMMENT '用户名',
		email VARCHAR(255) NOT NULL DEFAULT '' COMMENT '邮箱',
		status TINYINT DEFAULT 1 COMMENT '状态:0=禁用,1=正常',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT fk_user_profile FOREIGN KEY (id) REFERENCES profiles (user_id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户表';`

	table, err := parser.Parse(ddl)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	t.Logf("Table name: %s", table.TableName)
	t.Logf("Table comment: %s", table.TableComment)
	t.Logf("Columns: %d", len(table.Columns))
	t.Logf("Primary key: %v", table.PrimaryKey)
	t.Logf("Foreign keys: %d", len(table.ForeignKeys))

	if table.TableName != "users" {
		t.Errorf("expected table name 'users', got '%s'", table.TableName)
	}
	if table.TableComment != "用户表" {
		t.Errorf("expected comment '用户表', got '%s'", table.TableComment)
	}
	if len(table.Columns) != 5 {
		t.Errorf("expected 5 columns, got %d", len(table.Columns))
	}
	if len(table.PrimaryKey) != 1 || table.PrimaryKey[0] != "id" {
		t.Errorf("expected primary key 'id', got %v", table.PrimaryKey)
	}
	if len(table.ForeignKeys) != 1 {
		t.Errorf("expected 1 foreign key, got %d", len(table.ForeignKeys))
	}

	// 检查字段
	foundUsername := false
	for _, col := range table.Columns {
		if col.Name == "username" {
			foundUsername = true
			if col.DataType != "VARCHAR" {
				t.Errorf("expected VARCHAR, got %s", col.DataType)
			}
			if col.Length != 50 {
				t.Errorf("expected length 50, got %d", col.Length)
			}
			if col.Nullable {
				t.Error("expected NOT NULL")
			}
			if col.Comment != "用户名" {
				t.Errorf("expected comment '用户名', got '%s'", col.Comment)
			}
		}
	}
	if !foundUsername {
		t.Error("username column not found")
	}
}

func TestExtractSQLBlocks(t *testing.T) {
	markdown := "# 数据库设计\n\n## 用户表\n\n```sql\nCREATE TABLE users (\n\tid INT PRIMARY KEY\n);\n```\n\n一些说明文字\n\n```sql\nCREATE TABLE orders (\n\tid INT PRIMARY KEY\n);\n```\n"

	blocks := ExtractSQLBlocks(markdown)
	if len(blocks) != 2 {
		t.Errorf("expected 2 SQL blocks, got %d", len(blocks))
	}
}
