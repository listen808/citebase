package text2sql

import (
	"strings"
	"testing"
)

// TestIdentNeutralizesBacktick 钉住统计采集那三条 fmt.Sprintf 拼出来的 SQL。
// 表名与列名原先裸拼进去，而它们不是源码写死的常量：表名来自扫描结果、
// 也在管理页可改，列名来自导入的 DDL。一个带反引号的"表名"就能改写整条语句，
// 且这条语句跑在数据源配置的那套凭据上。
func TestIdentNeutralizesBacktick(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"users", "`users`", false},
		{"aw_user", "`aw_user`", false},
		{"a`b", "`a``b`", false},
		// 首尾的反引号同样要双写；只处理中间那一段的实现会在这里漏掉闭合位。
		{"` , 1 FROM mysql.user -- ", "``` , 1 FROM mysql.user --`", false},
		{"", "", true},
		{"   ", "", true},
		{"a\x00b", "", true},
		{"a\nb", "", true},
	}
	for _, c := range cases {
		got, err := ident(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ident(%q) 应当拒绝，实得 %q", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ident(%q) 不该报错: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ident(%q) = %q，期望 %q", c.in, got, c.want)
		}
	}
}

// TestIdentRoundTrip 从结果反推：剥掉外层引号、把双写反引号还原之后，
// 应当正好等于去空白后的原输入。这条性质就是"整串只被当成一个标识符、
// 没有任何一段提前闭合出去当语法用"的形式化说法。
func TestIdentRoundTrip(t *testing.T) {
	for _, in := range []string{"users", "a`b", "order", "select", "1;drop table x", "  spaced  ", "` , 1 FROM mysql.user -- "} {
		out, err := ident(in)
		if err != nil {
			t.Fatalf("ident(%q): %v", in, err)
		}
		if !strings.HasPrefix(out, "`") || !strings.HasSuffix(out, "`") || len(out) < 2 {
			t.Errorf("ident(%q) = %q 未加引号，关键字与空格会参与语法解析", in, out)
			continue
		}
		body := out[1 : len(out)-1]
		roundTrip := strings.ReplaceAll(body, "``", "`")
		if roundTrip != strings.TrimSpace(in) {
			t.Errorf("ident(%q) = %q 还原成 %q，与原输入不一致 —— 说明引号没能把整串包成一个标识符",
				in, out, roundTrip)
		}
	}
}
