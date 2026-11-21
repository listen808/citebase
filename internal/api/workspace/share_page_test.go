package workspace

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/brand"
	"unicode/utf8"
)

// 分享页的服务端 OG：判据是"什么情况下不许说出标题"，这条只能在纯函数上钉死，
// resolveShareMeta 那半截要连库，交给这里的 Public 开关覆盖其渲染结果。

const indexStub = `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">` +
	`<title>Citebase</title></head><body><div id="app"></div></body></html>`

func TestBuildShareHeadKeepsPrivateTitlesOut(t *testing.T) {
	// 故意把 Title/Description 填上：Public=false 时它们必须一个字都不出现。
	head := BuildShareHead(SharePageMeta{
		Title:       "机密并购方案",
		Description: "仅限高管",
		URL:         "http://x/share/9",
		Public:      false,
	})
	for _, leak := range []string{"机密并购方案", "仅限高管", "http://x/share/9"} {
		if strings.Contains(head, leak) {
			t.Fatalf("受保护分享把 %q 写进了 head：%s", leak, head)
		}
	}
	if !strings.Contains(head, `content="noindex, nofollow"`) {
		t.Fatalf("受保护分享必须带 noindex，否则流出去的链接会被爬虫收进索引：%s", head)
	}
	if !strings.Contains(head, `og:title`) {
		t.Fatal("不公开也要有通用 og:title，否则粘出去是一张空白卡")
	}
	// 受保护那一支已经是 noindex；再给 canonical 等于告诉抓取器"这页值得规范化"，
	// 而且 href 里会带上分享地址本身。
	if strings.Contains(head, "rel=\"canonical\"") {
		t.Fatalf("受保护分享不该有 canonical：%s", head)
	}
}

func TestBuildShareHeadPublicCarriesTitle(t *testing.T) {
	head := BuildShareHead(SharePageMeta{
		Title:       "全景流程图",
		Description: "工作文档 · 由 周游 创建",
		URL:         "https://kb.example.com/share/356",
		Image:       "https://kb.example.com/pwa-icon.svg",
		Public:      true,
	})
	for _, want := range []string{"全景流程图", "工作文档 · 由 周游 创建", `content="https://kb.example.com/share/356"`, `content="article"`} {
		if !strings.Contains(head, want) {
			t.Fatalf("公开分享缺了 %q：%s", want, head)
		}
	}
	// canonical 与 og:url 同源：一条分享链接能被复制成无数带查询串的变体。
	if !strings.Contains(head, `<link rel="canonical" href="https://kb.example.com/share/356">`) {
		t.Fatalf("公开分享缺 canonical：%s", head)
	}
	if strings.Contains(head, "noindex") {
		t.Fatal("公开分享不该被 noindex，那正好废掉这条链接的可分享性")
	}
}

// 文档标题是用户起的：不过一遍转义，`"><script>` 就是直接进属性位的 XSS。
func TestBuildShareHeadEscapesUserControlledTitle(t *testing.T) {
	evil := `"><script>alert(1)</script>`
	head := BuildShareHead(SharePageMeta{Title: evil, Description: evil, Public: true})
	if strings.Contains(head, `"><script>`) {
		t.Fatalf("标题未被转义，属性位被冲出：%s", head)
	}
	if !strings.Contains(head, "&lt;script&gt;") {
		t.Fatalf("期望转义成实体，实际：%s", head)
	}
	if strings.Contains(head, `<script`) {
		t.Fatal("head 里出现了可执行标签")
	}
}

func TestClipRuneIsCharacterSafe(t *testing.T) {
	long := strings.Repeat("文", shareMetaTitleMax+40)
	got := clipRune(long, shareMetaTitleMax)
	if !utf8.ValidString(got) {
		t.Fatal("按字符裁完出现非法 UTF-8 —— 按字节裁就会这样")
	}
	if n := utf8.RuneCountInString(got); n != shareMetaTitleMax+1 { // +1 是省略号
		t.Fatalf("裁完 %d 个字符，期望 %d", n, shareMetaTitleMax+1)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatal("超长内容必须带省略号，免得看着像标题本来就这么短")
	}
	if short := clipRune("短标题", shareMetaTitleMax); short != "短标题" {
		t.Fatalf("没超长不该动它：%s", short)
	}
}

func TestRenderShareHTML(t *testing.T) {
	out := RenderShareHTML(indexStub, SharePageMeta{Title: "演练", Public: true})
	if n := strings.Count(out, "<title>"); n != 1 {
		t.Fatalf("<title> 出现 %d 次；注入第二个是浏览器取第一个，白注入", n)
	}
	if !strings.Contains(out, "<title>演练</title>") {
		t.Fatalf("页题没换成文档标题：%s", out)
	}
	if strings.Index(out, "og:title") > strings.Index(out, "</head>") {
		t.Fatal("meta 必须落在 </head> 之前，落到 body 里抓取器读不到")
	}
	if !strings.Contains(out, `<div id="app">`) {
		t.Fatal("注入把应用挂载点弄坏了")
	}
}

func TestRenderShareHTMLPrivateKeepsGenericTitle(t *testing.T) {
	out := RenderShareHTML(indexStub, SharePageMeta{})
	if !strings.Contains(out, "<title>"+shareSiteName()+"</title>") {
		t.Fatalf("不公开时页题该退回站点名：%s", out)
	}
	if strings.Contains(out, "og:type\" content=\"article\"") {
		t.Fatal("不公开不该声明这是一篇文章")
	}
}

// 分享页是"链接被发到别处"时别人唯一能看到的一张脸，所以站点名必须跟着后台的值走。
// 这条钉的是接线：把 shareSiteName 改回常量、或让它无视 brand，本条立刻红。
func TestShareSiteNameFollowsAdminOverride(t *testing.T) {
	defer brand.InstallBrandReader(nil)

	brand.InstallBrandReader(func(key string) (string, error) {
		if key == brand.ConfigKeySiteName {
			return "团队百科", nil
		}
		return "", nil
	})

	out := BuildShareHead(SharePageMeta{Public: false})
	if !strings.Contains(out, `og:site_name" content="团队百科"`) {
		t.Fatalf("og:site_name 没采用管理员的站点名：%s", out)
	}
	if strings.Contains(out, brand.DefaultName) {
		t.Fatalf("覆盖了还带着代码默认名：%s", out)
	}
}

// 站点描述同样要真的生效：管理员在「系统设置 → 基础」填的那一句，
// 是不公开分享与"文档没写描述"两种情况下唯一被说出来的一句话。
func TestShareDescriptionFollowsSiteConfig(t *testing.T) {
	defer brand.InstallBrandReader(nil)

	brand.InstallBrandReader(func(key string) (string, error) {
		if key == brand.ConfigKeySiteDescription {
			return "团队知识资产的入口", nil
		}
		return "", nil
	})

	if got := shareGenericDescription(); got != "团队知识资产的入口" {
		t.Fatalf("没采用管理员填的站点描述：%q", got)
	}

	// 没填则退回通用句 —— 这一支不能因为"读到了空串"就输出一个空的 description。
	brand.InstallBrandReader(func(string) (string, error) { return "", nil })
	if got := shareGenericDescription(); got != shareGenericDesc {
		t.Fatalf("站点描述为空时应退回通用句：%q", got)
	}
}

// 走一遍真实 handler：路由 → 读盘 → 注入 → 响应。用一个取不到文档的 id，
// 于是这条覆盖的正是"任何异常都必须落到不公开那一支"——它同时也是不连库就能跑的原因。
func TestSharePageServesHTMLWithoutLeaking(t *testing.T) {
	dir := t.TempDir()
	index := dir + "/index.html"
	if err := writeFileForShareTest(index, indexStub); err != nil {
		t.Fatal(err)
	}
	old := shareIndexHTML
	shareIndexHTML = index
	defer func() { shareIndexHTML = old }()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/share/:id", (&DocumentAPI{}).SharePage)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/share/not-a-number", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("状态码 %d，期望 200（分享页取不到文档也要出壳子，让 SPA 自己显示错误态）", w.Code)
	}
	body := w.Body.String()
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("Content-Type %q，必须是 HTML 否则抓取器当文件下载", ct)
	}
	if !strings.Contains(body, "noindex") {
		t.Fatal("取不到文档时必须 noindex")
	}
	if strings.Contains(body, "og:type\" content=\"article\"") {
		t.Fatal("没确认公开之前不许声明这是文章")
	}
	if !strings.Contains(body, `<div id="app">`) {
		t.Fatal("注入把应用挂载点弄坏了")
	}
}

func writeFileForShareTest(p, s string) error { return os.WriteFile(p, []byte(s), 0o600) }
