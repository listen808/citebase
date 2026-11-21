package repocheck

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/listen808/citebase/internal/brand"
	"github.com/listen808/citebase/internal/repository"
)

// 这台门禁管的是"产品名到底有几份真相"。
//
// 2026-10-07 改名 AIWiki → Citebase 之后，新名字本身又漂回去了：
//   - 安装向导的种子写 "Citebase 知识库"，db.go 与 data.sql 写 "Citebase"（同一个键两个值）；
//   - 分享页的 og:site_name、测试邮件、巡检邮件、/health 的 message、嵌入式小组件的 <title>
//     各写一份字面量；
//   - 前端 app-info.ts 有 APP_NAME，router/guards.ts 又抄一份 APP_TITLE，
//     ApBrand/LoginView/SpaceMobileHeader/ShareView/邮件预览各自再抄一份；
//   - 而 aw_config 里 main_title / system_name / main_description 三行只有写入、没有读取
//     —— 管理员在「系统设置」里改站点名改的是空气。
// 三条判据各自挡掉其中一类复发：
//   ① 名字只准 internal/brand 写一次（AST 扫字符串字面量，注释不管）；
//   ② 三份种子（DefaultConfigSeeds / data.sql / 安装向导）逐字段一致；
//   ③ 配置键与前端默认名必须与后端同源（读哪个键、默认叫什么，两侧不许各说各话）。

// 只拦**显示拼写** Citebase。CITEBASE_*（环境变量前缀）与 citebase（仓库 slug / 包路径）
// 都是协议标识符而不是给用户看的名字：前者由 deploy_env_test.go 与 internal/utils/envname
// 的双读契约管，后者改了反而对不上 GitHub。旧名 AIWiki 的射程是 brand_hygiene_test.go 的活。
var brandLiteralRe = regexp.MustCompile(`Citebase`)

// 排除：品牌包自己（那就是唯一定义处）、生成物 docs/swagger。
var brandLiteralExempt = []string{"internal/brand/", "docs/swagger/", "tools/repocheck/"}

func TestBrandNameWrittenOnce(t *testing.T) {
	hits, scanned := 0, 0

	for _, rel := range trackedFiles(t) {
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || brandRelExempt(rel) {
			continue
		}
		scanned++
		for _, lit := range goStringLiterals(t, rel) {
			if brandLiteralRe.MatchString(lit) {
				hits++
				t.Errorf("%s: 字符串字面量里写死了产品名 → %s（唯一落点是 internal/brand）", rel, clipLit(lit))
			}
		}
	}

	// 下界：扫描射程空了（判据坏了）时 0 命中不可信 —— 与 brand_hygiene 同一手法。
	if scanned < 600 {
		t.Fatalf("只扫了 %d 个非测试 .go（现行 600+）—— 遍历判据坏了", scanned)
	}
	if hits == 0 {
		t.Logf("品牌名收口：%d 个 .go 里除 internal/brand 外零处写死产品名", scanned)
	}
}

func brandRelExempt(rel string) bool {
	for _, e := range brandLiteralExempt {
		if strings.HasPrefix(rel, e) {
			return true
		}
	}
	return false
}

// ---------- ② 三份种子必须逐字段一致 ----------

type seedRow struct {
	Key    string
	Value  string
	Label  string
	Group  string
	Type   string
	Sort   int
	HasRow bool
}

func TestConfigSeedSourcesAgree(t *testing.T) {
	codeSeeds := map[string]seedRow{}
	for _, cfg := range repository.DefaultConfigSeeds() {
		codeSeeds[cfg.Key] = seedRow{Key: cfg.Key, Value: cfg.Value, Label: cfg.Label, Group: cfg.GroupKey, Type: cfg.Type, Sort: cfg.Sort, HasRow: true}
	}
	if len(codeSeeds) < 8 {
		t.Fatalf("DefaultConfigSeeds 只有 %d 行（现行 10）—— 清单被截断，比对没有意义", len(codeSeeds))
	}

	sqlSeeds := parseDataSQLConfigs(t, readRepoFile(t, "databases/data.sql"))
	if len(sqlSeeds) < 8 {
		t.Fatalf("data.sql 的 aw_config 只解析出 %d 行（现行 10）—— 解析判据坏了", len(sqlSeeds))
	}

	// data.sql 是 db.go 的镜像：两边键集合必须相同，共有键逐字段相同。
	for key, want := range codeSeeds {
		got, ok := sqlSeeds[key]
		if !ok {
			t.Errorf("data.sql 缺 aw_config 行 %q（db.go 的 DefaultConfigSeeds 有这一行）", key)
			continue
		}
		if *got != want {
			t.Errorf("aw_config[%s] 两份种子不一致：db.go=%+v data.sql=%+v", key, want, *got)
		}
	}
	for key := range sqlSeeds {
		if _, ok := codeSeeds[key]; !ok {
			t.Errorf("data.sql 多出 aw_config 行 %q —— 种子只准 DefaultConfigSeeds 一处写", key)
		}
	}
}

// aw_config 的列序是 (key, value, label, group_key, type, options, is_private, sort, ...)。
// options 是 'openai,anthropic,...' 这种**带逗号的字符串**：上一版按"一个字段一个逗号"切分，
// 凡是 select 行都解析失败，于是门禁把 data.sql 报成"缺行" —— 缺的是解析器，不是种子。
// 现在用命名分组，并且只在 aw_config 那条 INSERT 内部取行（别表的行也可能长成五个引号串的样）。
var dataSQLRowRe = regexp.MustCompile(
	`^\('(?P<key>[^']*)',\s*'(?P<value>[^']*)',\s*'(?P<label>[^']*)',\s*'(?P<group>[^']*)',\s*'(?P<type>[^']*)',\s*(?:NULL|'[^']*'),\s*\d+,\s*(?P<sort>\d+),`)

func parseDataSQLConfigs(t *testing.T, dump string) map[string]*seedRow {
	t.Helper()

	start := strings.Index(dump, "INSERT INTO `aw_config`")
	if start < 0 {
		t.Fatalf("data.sql 里找不到 INSERT INTO `aw_config` —— 种子表改名了，这台门禁该跟着改")
	}
	block := dump[start:]
	if end := strings.Index(block, ";"); end >= 0 {
		block = block[:end]
	}

	out := map[string]*seedRow{}
	for _, line := range strings.Split(block, "\n") {
		m := dataSQLRowRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		g := dataSQLRowRe.SubexpIndex
		sortValue, err := strconv.Atoi(m[g("sort")])
		if err != nil {
			t.Fatalf("sort 解析失败 %q: %v", m[g("sort")], err)
		}
		out[m[g("key")]] = &seedRow{
			Key: m[g("key")], Value: m[g("value")], Label: m[g("label")],
			Group: m[g("group")], Type: m[g("type")], Sort: sortValue, HasRow: true,
		}
	}
	return out
}

// ---------- ②b 安装向导不许再抄第二份清单 ----------

func TestInstallWizardUsesSharedSeeds(t *testing.T) {
	rel := "internal/api/auth/install.go"
	src := readRepoFile(t, rel)

	if strings.Contains(src, "{Key: \"") {
		t.Errorf("%s 里又出现了一份配置种子清单 —— 只准引用 repository.DefaultConfigSeeds()", rel)
	}
	if !strings.Contains(src, "repos.DefaultConfigSeeds()") {
		t.Errorf("%s 没有引用 repository.DefaultConfigSeeds()：安装向导与 AutoMigrate 会再次各自漂移", rel)
	}
}

// ---------- ③ 接上的配置键必须真的被种下来，且默认名与前后端同源 ----------

func TestWiredConfigKeysAreSeeded(t *testing.T) {
	seeds := map[string]bool{}
	for _, cfg := range repository.DefaultConfigSeeds() {
		seeds[cfg.Key] = true
	}

	// brand 读这三个键；没种下就等于"读一个不存在的键"，管理员在后台根本没有这一栏。
	for _, key := range []string{brand.ConfigKeySiteName, brand.ConfigKeySiteNameLegacy, brand.ConfigKeySiteDescription} {
		if !seeds[key] {
			t.Errorf("internal/brand 读配置键 %q，但种子清单里没有这一行 —— 界面可编辑、实际不生效", key)
		}
	}

	// 种子写进站名两行的值必须就是代码默认名，否则"装了就用另一个名字"。
	for _, key := range []string{brand.ConfigKeySiteName, brand.ConfigKeySiteNameLegacy} {
		for _, cfg := range repository.DefaultConfigSeeds() {
			if cfg.Key == key && cfg.Value != brand.DefaultName {
				t.Errorf("aw_config[%s] 的种子值是 %q，与代码默认名 %q 不一致", key, cfg.Value, brand.DefaultName)
			}
		}
	}
}

func TestFrontendBrandDefaultMatchesBackend(t *testing.T) {
	rel := "web/src/utils/core/app-info.ts"
	src := readRepoFile(t, rel)

	m := regexp.MustCompile(`APP_NAME = '([^']*)'`).FindStringSubmatch(src)
	if m == nil {
		t.Fatalf("%s 里找不到 APP_NAME 常量 —— 前端默认名的落点变了，这台门禁该跟着改", rel)
	}
	// 两侧默认名不一致的表现是：管理员没改过配置时，标签页标题与后台/邮件里的名字不同。
	if fmt.Sprintf("%q", m[1]) != fmt.Sprintf("%q", brand.DefaultName) {
		t.Errorf("前端默认名 %q ≠ 后端默认名 %q —— 改名要同时改这两处", m[1], brand.DefaultName)
	}
}

// 前端还持有一份"哪几个配置键改的是站点品牌"的清单（site-brand.ts 的 BRAND_CONFIG_KEYS），
// 管理端保存完这些键要不要 refreshSiteBrand() 由它决定 —— 少列一个键，
// 症状是"管理员改了，全站不动，必须手动刷新"（后台那一栏从写死不读的时代开始就一直是这个毛病），
// 多列一个键则每次保存都白发一次 /site 请求。后端那份常量才是真相源，两侧对不上要红。
func TestFrontendBrandConfigKeysMatchBackend(t *testing.T) {
	rel := "web/src/utils/core/site-brand.ts"
	src := readRepoFile(t, rel)

	m := regexp.MustCompile(`BRAND_CONFIG_KEYS = \[([^\]]*)\]`).FindStringSubmatch(src)
	if m == nil {
		t.Fatalf("%s 里找不到 BRAND_CONFIG_KEYS —— 前端那份品牌键清单换形状了，这条判据该跟着改", rel)
	}
	listed := map[string]bool{}
	for _, item := range regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(m[1], -1) {
		listed[item[1]] = true
	}

	want := []string{brand.ConfigKeySiteName, brand.ConfigKeySiteNameLegacy, brand.ConfigKeySiteDescription}
	for _, key := range want {
		if !listed[key] {
			t.Errorf("前端 BRAND_CONFIG_KEYS 缺 %q（后端 internal/brand 在读它）—— 改完那一栏不会刷新展示名/描述", key)
		}
	}
	for key := range listed {
		if !slices.Contains(want, key) {
			t.Errorf("前端 BRAND_CONFIG_KEYS 多出 %q —— 后端 internal/brand 没有这个键；加键要两侧一起改", key)
		}
	}
}

// ---------- ④ swagger 注解里的 @title（注释形态，字面量判据看不见） ----------

// @title 落在注释里，TestBrandNameWrittenOnce 只数 BasicLit，一条也不会命中 ——
// 于是它成了"改名时唯一没人提醒的那份拷贝"：改完名 swagger 页顶还写着旧名字，
// 而 `make swagger` 只会把旧注解原样再生一遍，没有任何一步会红。
// 判据用前缀而不是整串相等：@title 的现行值是 "Citebase API"，
// 后半截（API/Server 之类的品类词）不是产品名。
var swaggerTitleRe = regexp.MustCompile(`@title[ \t]+(\S+)`)

func TestSwaggerTitleUsesDefaultName(t *testing.T) {
	found := 0
	for _, rel := range trackedFiles(t) {
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			continue
		}
		for _, line := range strings.Split(readRepoFile(t, rel), "\n") {
			m := swaggerTitleRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			found++
			title := m[1]
			if !strings.HasPrefix(title, brand.DefaultName) {
				t.Errorf("%s: @title 是 %q，产品名那一份拷贝没跟着改（唯一落点 internal/brand）", rel, title)
			}
			if strings.Contains(strings.ToLower(title), "aiwiki") {
				t.Errorf("%s: @title 里出现旧品牌 → %q", rel, title)
			}
		}
	}
	// 下界同其它判据：一条都没数到说明注解挪了形状，此时的"通过"是空转。
	if found == 0 {
		t.Fatalf("全仓一个 @title 都没匹配到 —— swagger 注解换写法了，这条判据在空转")
	}
	t.Logf("swagger 注解：%d 处 @title，首词都是 %q", found, brand.DefaultName)
}

// goStringLiterals 取一个 .go 里所有字符串字面量的值（注释不算、变量名不算）。
// 用 AST 而不是 grep：grep 会把 "给运维看的注释里提到的旧写法" 和 "真显示给用户的那一句" 混成一桶。
func goStringLiterals(t *testing.T, rel string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, readRepoFile(t, rel), parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("解析 %s: %v", rel, err)
	}
	var lits []string
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		if v, err := strconv.Unquote(lit.Value); err == nil {
			lits = append(lits, v)
		}
		return true
	})
	return lits
}

func clipLit(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 80 {
		return s[:80] + "…"
	}
	return s
}
