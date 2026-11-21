package rag

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/listen808/citebase/internal/brand"
)

// TestSafeBaseURLRejectsHostHeaderEscapes 钉住那条 XSS：
// 三处插值点（var origin=…、base=…、<iframe src="…/ai/widget">）填的都是
// 由 c.Request.Host 拼出来的串，而 Host 是客户端说了算的头。
// 修复前打一个带引号的 Host 就能把 JS 塞进页面。
func TestSafeBaseURLRejectsHostHeaderEscapes(t *testing.T) {
	bad := []string{
		`https://x';alert(document.cookie);//`,
		`https://x" onload="alert(1)`,
		`https://x\';alert(1);//`,
		"https://x\nY-Header: 1",
		"https://x\r\n",
		`https://<script>alert(1)</script>`,
		`javascript:alert(1)`,
		`https://evil.com:80@good.example`, // userinfo 伪装
		`https://good.example/redirect/x`,  // 带路径就不是"基址"
		`http://`,                          // 空主机
		``,                                 // 空串
		`ftp://host.example`,               // scheme 不在白名单
		`https://ex ample.com`,             // 空格
	}
	for _, raw := range bad {
		if got, ok := SafeBaseURL(raw); ok {
			t.Errorf("SafeBaseURL(%q) 竟然通过并返回 %q", raw, got)
		}
	}

	good := []string{
		"http://localhost:3000",
		"https://citebase.example.com",
		"https://citebase.example.com:8443",
		"http://192.168.1.20:3001",
		"https://citebase.example.com/",
	}
	for _, raw := range good {
		got, ok := SafeBaseURL(raw)
		if !ok {
			t.Errorf("SafeBaseURL(%q) 应当放行", raw)
			continue
		}
		if strings.HasSuffix(got, "/") {
			t.Errorf("SafeBaseURL(%q) 返回了带尾斜杠的 %q —— 调用方会拼 /ai/widget，双斜杠会变出一个空路径段", raw, got)
		}
	}
}

// TestNewEmbedWidgetRejectsUnsafeBase 钉住构造函数会把不合格基址挡在对象外面
// （返回 nil，由调用方拒绝出页），而不是让它带着可执行内容往下走。
func TestNewEmbedWidgetRejectsUnsafeBase(t *testing.T) {
	if w := NewEmbedWidget(`https://x';alert(1);//`); w != nil {
		t.Error("NewEmbedWidget 接受了带引号的 Host")
	}
	if w := NewEmbedWidget("https://citebase.example.com"); w == nil {
		t.Error("NewEmbedWidget 应当接受合法基址")
	}
}

// TestJSStringNeutralizesJSLiterals 钉住编码器本身：
// 单引号、双引号、反斜杠、换行，以及手写转义表漏掉的 U+2028/U+2029。
func TestJSStringNeutralizesJSLiterals(t *testing.T) {
	payloads := []string{
		`x';alert(1);//`,
		`x";alert(1);//`,
		`x\';alert(1);//`,
		"x\nalert(1)",
		"x\r",
		"a\u2028b",
		"a\u2029b",
		`</script><script>alert(1)</script>`,
	}
	for _, p := range payloads {
		enc := jsString(p)
		if !strings.HasPrefix(enc, `"`) || !strings.HasSuffix(enc, `"`) {
			t.Errorf("jsString(%q) 没带引号：%q", p, enc)
			continue
		}
		// 编码结果里不能出现裸换行或那两个行分隔符（它们必须还是转义态）。
		inner := enc[1 : len(enc)-1]
		if strings.ContainsAny(inner, "\n\r") {
			t.Errorf("jsString(%q) 的输出里还有裸换行: %q", p, enc)
		}
		if strings.ContainsRune(inner, '\u2028') || strings.ContainsRune(inner, '\u2029') {
			t.Errorf("jsString(%q) 没有转义行分隔符 —— 手写 ReplaceAll 表漏的就是这两个: %q", p, enc)
		}
	}
}

// 这段 HTML 由 fmt.Sprintf 产出，模板里同时有 CSS 的 `%%`、站点名的 `%[1]s` 与 base URL 的 `%[2]s`。
// 这条测的是**实参顺序与 CSS 百分号**两件事，各对应一种真实翻车：
// 顺序调换 → 字标位置印出 base URL（实测红）；`%%` 少写一层 → `max-width:85%%` 原样进页面把样式写坏。
//
// 顺带纠正一个我原本写进注释的误解：`%[1]s` 之后再出现裸 `%s` **不会**产出 `%!s(MISSING)`，
// 探针实测 Go 是让裸 `%s` 顺序取下一个实参 —— 所以"`%!` 残迹"这条断言留着当兜底，
// 但它不是这台门禁会红的理由，别照着它推断"混用一定会被发现"。
// 反向验证过的那条是：把两个实参调换顺序，本条立刻红。
func TestServeWidgetPageRendersSiteNameWithoutFormatLeftovers(t *testing.T) {
	defer brand.InstallBrandReader(nil)
	brand.InstallBrandReader(func(key string) (string, error) {
		if key == brand.ConfigKeySiteName {
			return "团队百科", nil
		}
		return "", nil
	})

	widget := NewEmbedWidget("https://kb.example.com")
	if widget == nil {
		t.Fatal("NewEmbedWidget 拒绝了合法基址")
	}

	rec := httptest.NewRecorder()
	widget.ServeWidgetPage(rec, httptest.NewRequest(http.MethodGet, "/ai/widget", nil))
	body := rec.Body.String()

	if !strings.Contains(body, `class="header">团队百科 AI 知识助手`) {
		t.Errorf("字标没跟着站点名：%s", clipLine(body))
	}
	if strings.Contains(body, "%!") {
		t.Errorf("Sprintf 留下了没对上参数的残迹（混用裸 %%s 与索引式引用的表现）：%s", clipLine(body))
	}
	// CSS 里的百分比必须还原成单个 %，不然 `max-width:85%%` 会原样进页面把样式写坏。
	if !strings.Contains(body, "max-width:85%;") {
		t.Errorf("CSS 的双写百分号没还原成单个百分号：%s", clipLine(body))
	}
}

func clipLine(s string) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", " ")
	if len(s) > 160 {
		return s[:160] + "…"
	}
	return s
}
