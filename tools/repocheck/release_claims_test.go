package repocheck

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// 这台的射程：README 首页「Known gaps (measured)」那张表里的**每一个数字**。
//
// 为什么需要它：那张表是这份发布面最值得信任的一段 —— 它把欠账写成数字而不是形容词。
// 也正因为它写着"measured"，一个过期的数字比没有数字更坏：读的人会拿它当今天的实况。
// 2026-10-10 的发布体检里，同一类句子已经错过三次（"GitHub 仓库还不存在"其实已建、
// "本地分支已改名 main"其实仍叫 master、"本机连不上 github.com"其实通了）。
// 人写的事实句没人复核，所以把复核做成一条命令：数字由这台现场重算，README 必须逐字说同一个数。
//
// 判据形状：不是"README 里的数 == 测试文件里抄的另一个数"（那是两处手抄，一处改了另一处不会响），
// 而是"README 必须包含由现场重算出来的那个字符串"。代码动了路由数 → README 立刻不匹配 → 红，
// 修法只有两种：真的去补注解/翻译，或者重跑并把 README 与核对日期一起改。
//
// 反向验证：把 README 里任意一个数字改一位 → 那一条点名；把日期改成 2020-01-01 → 过期那条红。

var (
	claimCJK      = regexp.MustCompile(`[\x{4e00}-\x{9fff}]`)
	claimUseI18n  = regexp.MustCompile(`\$t\(|useI18n`)
	claimRouter   = routerAnno  // 与 swagger_coverage_test.go 同一口径，不许各数各的
	claimRouteReg = routeDeclRe // 同上：注册数的口径必须和棘轮一致
	claimDate     = regexp.MustCompile(`Last re-run against HEAD: \*\*(\d{4}-\d{2}-\d{2})\*\*`)
)

// claimStaleAfter 是"measured"这个词的承诺期。给一个季度：够长，不至于让一个正常迭代的
// 贡献者因为无关改动被红；够短，不至于让发布出去的 README 带着半年前的数字生活。
const claimStaleAfter = 120 * 24 * time.Hour

func TestReadmeKnownGapsNumbers(t *testing.T) {
	root := filepath.Join("..", "..")
	rawReadme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatalf("读 README.md: %v", err)
	}
	readme := string(rawReadme)

	m := claimMatches(t, root)
	if len(m) != 6 {
		t.Fatalf("重算出的判据不到 6 条（%d）—— 有一台测量本身坏了", len(m))
	}
	for _, c := range m {
		if !strings.Contains(readme, c.want) {
			t.Errorf("README 的 Known gaps 表与现场不符：%s 现在实测是 %d，README 必须逐字出现 %q",
				c.name, c.value, c.want)
		}
	}

	subs := claimDate.FindStringSubmatch(readme)
	if subs == nil {
		t.Fatalf("README 的 Known gaps 一节没有「Last re-run against HEAD: **YYYY-MM-DD**」这行 —— 没有核对日期的数字不算 measured")
	}
	when, err := time.Parse("2006-01-02", subs[1])
	if err != nil {
		t.Fatalf("核对日期解析不了：%q", subs[1])
	}
	if age := time.Since(when); age > claimStaleAfter {
		t.Errorf("Known gaps 的数字已经过期（核对于 %s，超过 %d 天）。重跑那几条命令并把日期改到今天，别删这行。",
			subs[1], int(claimStaleAfter.Hours()/24))
	}
	t.Logf("README 数字与现场一致（核对日期 %s），6 条判据：%s", subs[1], strings.Join(dumpClaims(m), " / "))
}

type measuredClaim struct {
	name  string
	value int
	want  string
}

// visibleCJK 只数会被渲染或被当文案用的中文：模板文本节点、模板属性值、脚本里的字符串字面量。
// 注释先整体剥掉 —— 本仓的注释规矩是中文说理，把它们算进"用户看得见的欠账"是虚报。
// 与 web/scripts/i18n-inventory.mjs 的分类保持同一套形状，两边数出来的东西才可比。
func visibleCJK(text string) int {
	trimmed := stripGoComments(text)
	tplEnd := strings.LastIndex(trimmed, "</template>")
	tpl, script := trimmed, ""
	if tplEnd > 0 {
		tpl, script = trimmed[:tplEnd], trimmed[tplEnd:]
	}
	n := 0
	for _, m := range visTextRe.FindAllStringSubmatch(tpl, -1) {
		n += len(claimCJK.FindAllString(m[1], -1))
	}
	for _, m := range visAttrRe.FindAllStringSubmatch(tpl, -1) {
		n += len(claimCJK.FindAllString(m[1], -1))
	}
	for _, m := range visLitRe.FindAllStringSubmatch(script, -1) {
		n += len(claimCJK.FindAllString(m[0], -1))
	}
	return n
}

func stripGoComments(text string) string {
	text = visBlockCommentRe.ReplaceAllString(text, " ")
	return visLineCommentRe.ReplaceAllString(text, "")
}

var (
	visBlockCommentRe = regexp.MustCompile(`(?s)<!--.*?-->|/\*.*?\*/`)
	visLineCommentRe  = regexp.MustCompile(`(?m)^[ \t]*//[^\n]*`)
	visTextRe         = regexp.MustCompile(`>([^<>]+)<`)
	visAttrRe         = regexp.MustCompile(`[\w-]+\s*=\s*"([^"]*)"`)
	visLitRe          = regexp.MustCompile("(?:'[^'\\n]*'|`[^`]*`)")
)

// claimMatches 把那张表背后的六条测量各自跑一遍。全部只用仓库里的文件，不连数据库、不起服务。
func claimMatches(t *testing.T, root string) []measuredClaim {
	t.Helper()

	// 口径覆盖 src/views 与 src/components 两处：只数 views 会低估 ——
	// 共享外壳（侧边栏、页脚、文档工具条）与聊天组件都在 components 里，它们出现在每一个页面上。
	var vueFiles, i18nFiles, cjk int
	for _, sub := range []string{"views", "components"} {
		dir := filepath.Join(root, "web", "src", sub)
		err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			text := string(data)
			if filepath.Ext(p) == ".vue" {
				if sub == "views" {
					vueFiles++
				}
				if claimUseI18n.MatchString(text) {
					i18nFiles++
				}
			}
			// 只数会被渲染出来的中文：模板文本节点、属性值、字符串字面量，注释一律不算。
			cjk += visibleCJK(text)
			return nil
		})
		if err != nil {
			t.Fatalf("遍历 %s: %v", dir, err)
		}
	}
	if vueFiles < 50 {
		t.Fatalf("只数到 %d 个 .vue（web/src/views 应有 100 上下）—— 遍历判据坏了", vueFiles)
	}

	// OpenAPI 两条口径复用 swagger_coverage_test.go 的正则与分族遍历。
	// 注意 fam 的下标顺序是 familyCounts 定的：[0]=注册数、[1]=注解数（写反过一次，
	// 写反时这台会报"注解 502 条 / 注册 121 条"，一个看起来比实况好四倍的假数字）。
	var annos, regs int
	for _, fam := range familyCounts(t) {
		regs += fam[0]
		annos += fam[1]
	}

	swaggerFile := filepath.Join(root, "docs", "swagger", "swagger.json")
	raw, err := os.ReadFile(swaggerFile)
	if err != nil {
		t.Fatalf("读 %s: %v（`make swagger` 生成；它不在的时候这条判据没有意义）", swaggerFile, err)
	}
	var doc struct {
		Paths map[string]map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("swagger.json 解析失败: %v", err)
	}

	return []measuredClaim{
		{"web/src/views 的 .vue 文件数", vueFiles, fmt.Sprintf("%d view files", vueFiles)},
		{"用了 i18n 的文件数（views + components）", i18nFiles, fmt.Sprintf("`%d` use", i18nFiles)},
		{"界面里硬编码的中文字符数", cjk, withComma(cjk) + " hard-coded"},
		{"@Router 注解数", annos, fmt.Sprintf("%d `@Router`", annos)},
		{"路由注册数", regs, fmt.Sprintf("%d route registrations", regs)},
		{"swagger 里的 paths 数", len(doc.Paths), fmt.Sprintf("has %d paths", len(doc.Paths))},
	}
}

func dumpClaims(m []measuredClaim) []string {
	out := make([]string, 0, len(m))
	for _, c := range m {
		out = append(out, c.name+"="+fmt.Sprint(c.value))
	}
	return out
}

// withComma 只为对上 README 里的千分位写法（README 给人读，判据必须按人读的那个样子比）。
func withComma(n int) string {
	s := fmt.Sprint(n)
	if len(s) <= 3 {
		return s
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return strings.Join(parts, ",")
}
