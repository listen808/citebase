package dbdump

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"
)

// collectStatements 把文本切成语句，用于测词法切分本身（不碰数据库）。
func collectStatements(src string) ([]string, error) {
	sc := newScanner(bufio.NewReader(strings.NewReader(src)))
	var out []string
	for {
		s, err := sc.next()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return out, err
		}
		out = append(out, s)
	}
}

func TestSplitStatements(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "字符串里的分号不算结束",
			src:  "INSERT INTO t VALUES ('a;b'),(2);\n",
			want: []string{"INSERT INTO t VALUES ('a;b'),(2)"},
		},
		{
			name: "双写的引号不闭合字符串",
			src:  "INSERT INTO t VALUES ('it''s;'),(2);SELECT 3;",
			want: []string{"INSERT INTO t VALUES ('it''s;'),(2)", "SELECT 3"},
		},
		{
			name: "反斜杠转义的引号不闭合字符串",
			src:  `INSERT INTO t VALUES ('a\';b');`,
			want: []string{`INSERT INTO t VALUES ('a\';b')`},
		},
		{
			name: "反引号标识符里的分号",
			src:  "SELECT `a;b` FROM t;",
			want: []string{"SELECT `a;b` FROM t"},
		},
		{
			name: "行注释与井号注释",
			src:  "-- 表 t\nCREATE TABLE t (a int);\n# 下一句\nSELECT 1;\n",
			want: []string{"CREATE TABLE t (a int)", "SELECT 1"},
		},
		{
			name: "块注释整体丢弃且不粘连两侧的括号",
			src:  "SELECT/* x */1;",
			want: []string{"SELECT 1"},
		},
		{
			name: "块注释里的分号不算结束",
			src:  "/* a;b */ SELECT 1;-- 尾注释",
			want: []string{"SELECT 1"},
		},
		{
			name: "a--b 里的双横线不是注释",
			src:  "SELECT 1--2;",
			want: []string{"SELECT 1--2"},
		},
		{
			name: "结尾没有分号也算一条",
			src:  "SET NAMES utf8mb4",
			want: []string{"SET NAMES utf8mb4"},
		},
		{
			name: "连续分号与纯注释不产生空语句",
			src:  ";;\n-- 只有注释\n;\n",
			want: nil,
		},
		{
			name: "导出文件的多行 INSERT",
			src:  "INSERT INTO `t` (`a`, `b`) VALUES\n(1,'x'),\n(2,'y');\n",
			want: []string{"INSERT INTO `t` (`a`, `b`) VALUES\n(1,'x'),\n(2,'y')"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := collectStatements(c.src)
			if err != nil {
				t.Fatalf("意外报错: %v", err)
			}
			if strings.Join(got, "\x00") != strings.Join(c.want, "\x00") {
				t.Fatalf("得到 %q，期望 %q", got, c.want)
			}
		})
	}
}

func TestSplitStatementsRejects(t *testing.T) {
	cases := map[string]string{
		"字符串未闭合":      "INSERT INTO t VALUES ('abc);",
		"反引号未闭合":      "SELECT `abc;",
		"块注释未闭合":      "SELECT 1; /* 没完",
		"含 DELIMITER": "DELIMITER ;;\nCREATE PROCEDURE p() BEGIN SELECT 1; END;;",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := collectStatements(src); err == nil {
				t.Fatal("期望报错，实际没有")
			}
		})
	}
}

func TestStatementTarget(t *testing.T) {
	cases := []struct {
		stmt  string
		verb  string
		table string
	}{
		{"CREATE TABLE `aw_user` (\n  `id` int\n)", "CREATE", "aw_user"},
		{"CREATE TABLE IF NOT EXISTS `citebase`.`aw_user` (`id` int)", "CREATE", "aw_user"},
		{"DROP TABLE IF EXISTS `aw_user`", "DROP", "aw_user"},
		{"INSERT INTO `aw_log` (`a`) VALUES (1)", "INSERT", "aw_log"},
		{"INSERT IGNORE aw_x VALUES (1)", "INSERT", "aw_x"},
		{"INSERT INTO aw_document_content VALUES (1,'a;b')", "INSERT", "aw_document_content"},
		{"TRUNCATE aw_t", "TRUNCATE", "aw_t"},
		{"UPDATE `aw_t` SET a = 1", "UPDATE", "aw_t"},
		{"REPLACE INTO `aw_t` VALUES (1)", "REPLACE", "aw_t"},
		{"SET NAMES utf8mb4", "SET", ""},
		{"SET SESSION FOREIGN_KEY_CHECKS = 0", "SET", ""},
		{"COMMIT", "COMMIT", ""},
		{"CREATE TABLE `we``ird` (`id` int)", "CREATE", "we`ird"},
	}
	for _, c := range cases {
		verb, table := statementTarget(c.stmt)
		if verb != c.verb || table != c.table {
			t.Errorf("%q → (%q,%q)，期望 (%q,%q)", c.stmt, verb, table, c.verb, c.table)
		}
	}
}

func TestRestoreArgumentGuards(t *testing.T) {
	if _, err := Restore("", strings.NewReader("SELECT 1"), RestoreOptions{}); err == nil {
		t.Error("空 DSN 应当报错")
	}
	if _, err := Restore("root:pw@tcp(127.0.0.1:1)/db", nil, RestoreOptions{}); err == nil {
		t.Error("数据源为空应当报错")
	}
	// 缺库名的 DSN 必须在建连之前就失败
	if _, err := Restore("root:pw@tcp(127.0.0.1:1)", strings.NewReader("SELECT 1"), RestoreOptions{}); err == nil {
		t.Error("DSN 缺库名应当报错")
	}
}
