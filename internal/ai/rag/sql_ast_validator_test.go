package rag

import (
	"testing"
)

func TestValidateSQLAST(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		wantErr error
	}{
		{
			name:    "valid_select",
			sql:     "SELECT id, name FROM users WHERE age > 18",
			wantErr: nil,
		},
		{
			name:    "select_with_join",
			sql:     "SELECT u.id, u.name, d.name FROM users u JOIN departments d ON u.dept_id = d.id",
			wantErr: nil,
		},
		{
			name:    "select_with_group_by",
			sql:     "SELECT dept_id, COUNT(*) as cnt FROM users GROUP BY dept_id HAVING cnt > 5",
			wantErr: nil,
		},
		{
			name:    "select_with_subquery",
			sql:     "SELECT * FROM users WHERE id IN (SELECT user_id FROM orders)",
			wantErr: nil,
		},
		{
			name:    "semicolon_injection",
			sql:     "SELECT * FROM users; DROP TABLE users",
			wantErr: ErrMultipleStatements,
		},
		{
			name:    "drop_table",
			sql:     "DROP TABLE users",
			wantErr: ErrNotSelect,
		},
		{
			name:    "insert",
			sql:     "INSERT INTO users (id, name) VALUES (1, 'admin')",
			wantErr: ErrNotSelect,
		},
		{
			name:    "update",
			sql:     "UPDATE users SET name = 'hacked' WHERE id = 1",
			wantErr: ErrNotSelect,
		},
		{
			name:    "delete",
			sql:     "DELETE FROM users WHERE id = 1",
			wantErr: ErrNotSelect,
		},
		{
			name:    "multiple_selects",
			sql:     "SELECT 1; SELECT 2",
			wantErr: ErrMultipleStatements,
		},
		{
			name:    "comment_injection",
			sql:     "SELECT * FROM users WHERE name = 'admin'--' AND pass = 'x'",
			wantErr: ErrSQLCommentInjection,
		},
		{
			name:    "block_comment",
			sql:     "SELECT * FROM /* comment */ users",
			wantErr: ErrSQLCommentInjection,
		},
		{
			name:    "empty_sql",
			sql:     "",
			wantErr: ErrEmptySQL,
		},
		{
			name:    "whitespace_only",
			sql:     "   ",
			wantErr: ErrEmptySQL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSQLAST(tt.sql)
			if err != tt.wantErr {
				t.Errorf("ValidateSQLAST() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
