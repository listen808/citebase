package dbdump

import (
	"io"
	"strings"
	"testing"
	"time"
)

func TestSchemaFromDSN(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
		want string
		err  bool
	}{
		{"带参数", `root:pw@tcp(127.0.0.1:3306)/citebase?charset=utf8mb4&parseTime=True`, "citebase", false},
		{"无参数", `root:pw@tcp(127.0.0.1:3306)/citebase`, "citebase", false},
		{"口令含斜杠", `root:p/a@tcp(h)/citebase?x=1`, "citebase", false},
		{"缺库名", `root:pw@tcp(127.0.0.1:3306)/`, "", true},
		{"完全没有斜杠", `root:pw@tcp(127.0.0.1:3306)`, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := SchemaFromDSN(c.dsn)
			if c.err {
				if err == nil {
					t.Fatalf("期望报错，实际得到 %q", got)
				}
				if strings.Contains(err.Error(), "pw@") || strings.Contains(err.Error(), "p/a@") {
					t.Fatalf("错误信息泄漏了口令: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("意外报错: %v", err)
			}
			if got != c.want {
				t.Fatalf("得到 %q，期望 %q", got, c.want)
			}
		})
	}
}

func TestRedactKeepsHostButDropsSecret(t *testing.T) {
	got := redact(`root:s3cr3t@tcp(10.0.0.1:3306)/db`)
	if strings.Contains(got, "s3cr3t") || strings.Contains(got, "root:") {
		t.Fatalf("redact 后仍含凭据: %s", got)
	}
	if !strings.Contains(got, "10.0.0.1:3306") {
		t.Fatalf("redact 丢掉了排障需要的地址信息: %s", got)
	}
}

func TestEscapeString(t *testing.T) {
	cases := map[string]string{
		`he said "hi"`:   `he said \"hi\"`,
		`it's`:           `it\'s`,
		`back\slash`:     `back\\slash`,
		"a\nb":           `a\nb`,
		"a\r\nb":         `a\r\nb`,
		string(rune(0)):  `\0`,
		string(rune(26)): `\Z`,
		"中文 ok":          "中文 ok",
	}
	for in, want := range cases {
		if got := EscapeString(in); got != want {
			t.Errorf("EscapeString(%q) = %q，期望 %q", in, got, want)
		}
	}
}

func TestValueSQL(t *testing.T) {
	if got := valueSQL(nil); got != "NULL" {
		t.Errorf("NULL 值渲染成 %q", got)
	}
	if got := valueSQL([]byte("abc")); got != "'abc'" {
		t.Errorf("[]byte 渲染成 %q", got)
	}
	if got := valueSQL("it's"); got != "'it\\'s'" {
		t.Errorf("引号未转义: %q", got)
	}
	// 非 UTF-8 字节必须走十六进制，否则字符集转换会毁二进制列数据
	if got := valueSQL([]byte{0xff, 0xfe}); got != "0xFFFE" {
		t.Errorf("二进制渲染成 %q，期望 0xFFFE", got)
	}
	ts := time.Date(2026, 9, 27, 20, 30, 45, 123456000, time.UTC)
	if got := valueSQL(ts); got != "'2026-09-27 20:30:45.123456'" {
		t.Errorf("时间渲染成 %q，毫秒精度必须保住（表里有 datetime(3)）", got)
	}
	if got := valueSQL(true); got != "1" {
		t.Errorf("bool 渲染成 %q", got)
	}
	// 数字按字符串字面量输出即可，服务端会隐式转成数值列类型
	if got := valueSQL(42); got != "'42'" {
		t.Errorf("数字渲染成 %q", got)
	}
}

func TestQuote(t *testing.T) {
	if got := Quote("aw_user"); got != "`aw_user`" {
		t.Errorf("得到 %q", got)
	}
	if got := Quote("we`ird"); got != "`we``ird`" {
		t.Errorf("内部反引号未双写: %q", got)
	}
}

func TestDumpArgumentGuards(t *testing.T) {
	// 这三种入参都必须在建连之前失败，测试不碰数据库
	cases := []struct {
		name string
		dsn  string
		opt  Options
		want string
	}{
		{"空DSN", "", Options{}, "DSN 为空"},
		{"只导结构与只导数据互斥", `root:pw@tcp(127.0.0.1:33306)/citebase`, Options{SchemaOnly: true, NoCreateInfo: true}, "互斥"},
		{"DSN缺库名", `root:pw@tcp(127.0.0.1:33306)`, Options{}, "没有库名"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Dump(c.dsn, io.Discard, c.opt)
			if err == nil {
				t.Fatal("期望报错，实际没有")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("错误信息 %q 里没有 %q", err, c.want)
			}
			if strings.Contains(err.Error(), "pw@") {
				t.Errorf("错误信息泄漏了口令: %v", err)
			}
		})
	}
}
