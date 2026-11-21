package workspace

import (
	"fmt"
	"html"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/brand"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
)

// 分享页是全站唯一"链接会被发到别处"的入口：粘进聊天工具/邮件时，对方那边跑的是一次
// 不执行 JS 的抓取，而 index.html 由 c.File 原样送出，客户端 usePageMeta 那套标签根本到不了。
// 所以这一层必须由服务端补。
//
// 隐私边界（这条是本文件存在的另一半理由）：**只有真正公开分享才带文档标题**。
// "公开"在这里的判据与 ShareDocument 一致 —— 空间开了分享、没设分享密码、文档没进回收站。
// 带密码的链接在输对密码之前连标题都不该给，否则"密码保护正文"就退化成"只保护正文"，
// 而链接本身成了泄露渠道（爬虫会把标题连 URL 一起记进索引）。
// 不满足时只出站点级通用标签，并补一条 robots: noindex 让已经流出去的链接不再被收。

const (
	shareGenericDesc  = "团队知识库中的文档分享链接"
	shareMetaTitleMax = 100
	shareMetaDescMax  = 200
)

// shareSiteName 是站点展示名，取自 internal/brand（管理员在「系统设置」里改过就用改后的）。
// 写成函数而不是常量：常量会在进程启动那一刻把名字钉死，而这一层是"链接被发到别处"时
// 别人看到的唯一一张脸，必须跟着后台的值走。
func shareSiteName() string { return brand.SiteName() }

// shareGenericDescription 是"不许说出文档内容"时顶上的那一句。
// 管理员在「系统设置 → 基础」填过站点描述就用它，没填沿用通用句 —— 两个分支里它都是
// 站点级文案，不涉及任何文档字段，所以不违反上面那条隐私边界。
func shareGenericDescription() string {
	if desc := brand.SiteDescription(); desc != "" {
		return desc
	}
	return shareGenericDesc
}

// shareIndexHTML 是构建产物路径；写成变量是给 HTTP 层测试一个指向临时文件的缝。
var shareIndexHTML = "./web/dist/index.html"

// SharePageMeta 是要注入 <head> 的内容。Public=false 时 Title/Description 必须留空，
// 由 BuildShareHead 负责只渲染通用标签——判据与渲染分开，测试才能各自钉住。
type SharePageMeta struct {
	Title       string
	Description string
	URL         string
	Image       string
	Public      bool
}

// clipRune 按字符裁（CJK 一个字符占三个字节，按字节裁会切出非法 UTF-8），并补省略号。
func clipRune(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

var titleTagRe = regexp.MustCompile(`(?s)<title>.*?</title>`)

// BuildShareHead 产出注入用的标签块。所有取值都过 html.EscapeString：
// 文档标题是用户起的，不过一遍就是 `"><script>` 直接进属性位的 XSS。
func BuildShareHead(m SharePageMeta) string {
	esc := html.EscapeString
	siteName := shareSiteName()
	if !m.Public {
		// 不拼任何来自文档的字段，连"这是一篇什么文档"都不说。
		return strings.Join([]string{
			`<meta name="description" content="` + esc(shareGenericDescription()) + `">`,
			`<meta property="og:site_name" content="` + esc(siteName) + `">`,
			`<meta property="og:type" content="website">`,
			`<meta property="og:title" content="` + esc(siteName) + `">`,
			`<meta property="og:description" content="` + esc("这是一条受保护的分享链接，需要密码或授权才能查看内容") + `">`,
			`<meta name="twitter:card" content="summary">`,
			`<meta name="twitter:title" content="` + esc(siteName) + `">`,
			`<meta name="robots" content="noindex, nofollow">`,
		}, "\n")
	}
	title := clipRune(m.Title, shareMetaTitleMax)
	desc := m.Description
	if desc == "" {
		desc = shareGenericDescription()
	}
	desc = clipRune(desc, shareMetaDescMax)
	tags := []string{
		`<meta name="description" content="` + esc(desc) + `">`,
		`<meta property="og:site_name" content="` + esc(siteName) + `">`,
		`<meta property="og:type" content="article">`,
		`<meta property="og:title" content="` + esc(title) + `">`,
		`<meta property="og:description" content="` + esc(desc) + `">`,
		`<meta name="twitter:card" content="summary">`,
		`<meta name="twitter:title" content="` + esc(title) + `">`,
		`<meta name="twitter:description" content="` + esc(desc) + `">`,
	}
	if m.URL != "" {
		tags = append(tags, `<meta property="og:url" content="`+esc(m.URL)+`">`)
		// canonical 与 og:url 同源：一条分享链接能被复制成无数带查询串的变体，
		// 指向同一个规范地址比让每个变体各自成页干净。只给公开那一支 ——
		// 受保护那一支已经是 noindex，再给 canonical 等于告诉抓取器"这页值得规范化"。
		tags = append(tags, `<link rel="canonical" href="`+esc(m.URL)+`">`)
	}
	if m.Image != "" {
		tags = append(tags, `<meta property="og:image" content="`+esc(m.Image)+`">`)
	}
	return strings.Join(tags, "\n")
}

// RenderShareHTML 把标签块插进 </head> 之前，并替换 <title> 的文本。
// 只替换不新增 <title>：文档里出现两个 title 时浏览器取第一个，注入的那个就白写了。
func RenderShareHTML(indexHTML string, m SharePageMeta) string {
	out := strings.Replace(indexHTML, "</head>", BuildShareHead(m)+"\n</head>", 1)
	title := shareSiteName()
	if m.Public && strings.TrimSpace(m.Title) != "" {
		title = clipRune(m.Title, shareMetaTitleMax)
	}
	return titleTagRe.ReplaceAllString(out, "<title>"+html.EscapeString(title)+"</title>")
}

// resolveShareMeta 查这篇文档能不能对外说出标题。任何一步取不到都退回"不公开"，
// 因为默认放开等于把 404/回收站/私有空间的标题一起放出去。
func (api *DocumentAPI) resolveShareMeta(docID int, base string) SharePageMeta {
	deny := SharePageMeta{}
	doc, err := api.docService.GetDocumentByID(docID)
	if err != nil || doc == nil || doc.IsDelete == 1 {
		return deny
	}
	space, err := api.spaceService.GetSpaceByID(doc.SpaceID)
	if err != nil || space == nil || space.IsShare != 1 {
		return deny
	}
	spaceModel, err := workspacerepo.NewSpaceRepository().FindByID(doc.SpaceID)
	if err != nil || spaceModel == nil || spaceModel.SharePassword != "" {
		return deny
	}
	desc := space.Name
	if doc.CreatorName != "" {
		desc += " · 由 " + doc.CreatorName + " 创建"
	}
	return SharePageMeta{
		Title:       doc.Name,
		Description: desc,
		URL:         fmt.Sprintf("%s/share/%d", base, docID),
		Image:       base + "/pwa-icon.svg",
		Public:      true,
	}
}

// requestBase 从请求推绝对前缀。og:url / og:image 要绝对地址，而部署域名不在配置里，
// 只能取 Host —— 它可被伪造，代价是"某次抓取看到的预览链接指向别处"，不是数据泄露。
// 要收死这条就在 config 里加一个对外基址，然后优先用它。
func requestBase(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := c.Request.Host
	if fwd := c.GetHeader("X-Forwarded-Host"); fwd != "" && !strings.ContainsAny(fwd, " \t\r\n\"'<>") {
		host = fwd
	}
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}

// SharePage 服务 GET /share/:id 的 HTML。注册在 NoRoute 之前，所以只有分享页会走
// 这次读盘 + 注入，其余路由仍由 setupStaticAndSPA 原样兜底。
func (api *DocumentAPI) SharePage(c *gin.Context) {
	page, err := os.ReadFile(shareIndexHTML)
	if err != nil {
		c.String(http.StatusNotFound, "frontend build not found")
		return
	}
	meta := SharePageMeta{}
	if id, e := strconv.Atoi(c.Param("id")); e == nil {
		meta = api.resolveShareMeta(id, strings.TrimSuffix(requestBase(c), "/"))
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(RenderShareHTML(string(page), meta)))
}

// RegisterSharePageRoutes 在根引擎上注册分享页。r 而不是 v1：它是页面路由，不是 API。
func RegisterSharePageRoutes(r *gin.Engine) {
	r.GET("/share/:id", NewDocumentAPI().SharePage)
}
