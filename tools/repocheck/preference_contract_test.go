package repocheck

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// 跨语言的取值清单必须一致 —— 这条测试是"偏好跟账号走"这整个功能唯一的安全带。
//
// 形状是这样的：外观与材质的**真相源在前端**（`web/src/styles/theme/themes.ts` 的
// THEME_REGISTRY / MATERIALS），服务端为了校验必须持有一份同样的清单，而 Go 不能 import TS。
// 于是加一档外观时有两处可改，漏掉任何一处的后果都不是"新档位不能用"这种看得见的错，
// 而是**用户在另一台设备上的选择被当成非法值丢掉**（PUT 被拒 → 前端按既有值继续 → 没人知道账号上没存进去）。
//
// 判据按集合比，不按顺序、不按写法：两边各按自己的语言习惯排列是正常的，
// 只有"少一个/多一个"才是会丢数据的那种不一致。

var (
	// THEME_REGISTRY 里的 { id: 'light', ... }
	feThemeRe  = regexp.MustCompile(`(?s)THEME_REGISTRY[^=]*=\s*\[[\s\S]*?\n\];`)
	idInBlock  = regexp.MustCompile(`\{\s*id:\s*'([a-z-]+)'`)
	feMatRe    = regexp.MustCompile(`MATERIALS\s*=\s*\[([^\]]*)\]`)
	goListRe   = regexp.MustCompile(`(?s)(ThemeIDs|MaterialIDs)\s*=\s*\[\]string\{([^}]*)\}`)
	quotedRe   = regexp.MustCompile(`"([^"]+)"`)
	feAutoName = "auto" // 前端把"跟随系统"作为伪外观写进偏好；它不在 THEME_REGISTRY 里（那是调色板清单）
	goAutoName = "auto"
)

func quoteSet(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func TestPreferencesListsMatchFrontendRegistry(t *testing.T) {
	root := filepath.Join("..", "..")
	feSrc := mustRead(t, filepath.Join(root, "web", "src", "styles", "theme", "themes.ts"))
	goSrc := mustRead(t, filepath.Join(root, "internal", "model", "user", "preferences.go"))

	// 前端外观清单：只认 THEME_REGISTRY 那一段里的 id，别按全文扫（注释里也会出现 'light'）
	registry := feThemeRe.FindString(feSrc)
	if registry == "" {
		t.Fatal("没在前端解析出 THEME_REGISTRY 段 —— 判据坏了，不要信下面的结论")
	}
	var feThemes []string
	for _, m := range idInBlock.FindAllStringSubmatch(registry, -1) {
		feThemes = append(feThemes, m[1])
	}
	if len(feThemes) < 2 {
		t.Fatalf("前端外观只解析出 %d 个（应有 4 套）", len(feThemes))
	}
	mats := feMatRe.FindStringSubmatch(feSrc)
	if len(mats) == 0 {
		t.Fatal("没解析出 MATERIALS")
	}
	var feMaterials []string
	for _, s := range strings.Split(mats[1], ",") {
		s = strings.TrimSpace(strings.Trim(strings.TrimSpace(s), "'"))
		if s != "" {
			feMaterials = append(feMaterials, s)
		}
	}

	goLists := map[string][]string{}
	for _, m := range goListRe.FindAllStringSubmatch(goSrc, -1) {
		for _, q := range quotedRe.FindAllStringSubmatch(m[2], -1) {
			goLists[m[1]] = append(goLists[m[1]], q[1])
		}
	}
	if len(goLists["ThemeIDs"]) == 0 || len(goLists["MaterialIDs"]) == 0 {
		t.Fatalf("没在服务端解析出两份清单：%v", goLists)
	}

	// 服务端那份清单是"调色板清单"，`auto` 单独住在一个常量里（它不是外观，是"由 matchMedia 决定"）。
	// 比对要用**服务端实际接受的集合**，所以把它拼回去 —— 少拼这一步，这条门禁会天天红。
	goAuto := regexp.MustCompile(`PrefThemeAuto\s*=\s*"([^"]+)"`).FindStringSubmatch(goSrc)
	if len(goAuto) == 0 {
		t.Fatal("没在服务端解析出 PrefThemeAuto 常量")
	}
	serverThemes := append(append([]string{}, goLists["ThemeIDs"]...), goAuto[1])

	// 服务端外观清单必须**恰好**是前端那批 + 'auto'
	want := quoteSet(append(append([]string{}, feThemes...), feAutoName))
	got := quoteSet(serverThemes)
	if strings.Join(want, ",") != strings.Join(got, ",") {
		t.Errorf("服务端认得的外观与前端不是同一批：前端(+auto)=%s 服务端实际接受=%s —— 漏掉的那一档，用户在另一台设备上的选择会被当非法值丢掉",
			strings.Join(want, " "), strings.Join(got, " "))
	}
	if strings.Join(quoteSet(feMaterials), ",") != strings.Join(quoteSet(goLists["MaterialIDs"]), ",") {
		t.Errorf("服务端认得的材质与前端不是同一批：前端=%s 服务端=%s",
			strings.Join(quoteSet(feMaterials), " "), strings.Join(quoteSet(goLists["MaterialIDs"]), " "))
	}
	if goAuto[1] != feAutoName {
		t.Errorf("服务端的 auto 常量是 %q，前端用的是 %q —— 两边名字不一致时，「跟随系统」这一档永远同步不上", goAuto[1], feAutoName)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读 %s: %v", path, err)
	}
	return string(data)
}

// TestBehaviorAxisListsMatchFrontend 管三条**行为档**（动效强度 / 对比度 / 透明度，2026-10-16 批2）。
//
// 判据与上面那条同构，但形状更好：前端把 `auto` 直接写进数组里（不像 theme 那样另立一个常量），
// 所以这里只需集合相等，不需要"前端 + auto"那种拼接 —— 那次拼接正是上一批第一次跑红的原因
// （判据只比 ThemeIDs，漏了住在 PrefThemeAuto 里的 auto）。
// 少登记一档的症状仍然是静默的：PUT 被拒 → 前端按既有值继续 → 没人知道账号上没存进去。
func TestBehaviorAxisListsMatchFrontend(t *testing.T) {
	root := filepath.Join("..", "..")
	pers := mustRead(t, filepath.Join(root, "web", "src", "styles", "theme", "personalization.ts"))
	goSrc := mustRead(t, filepath.Join(root, "internal", "model", "user", "preferences.go"))
	feItemRe := regexp.MustCompile(`'([^']+)'`)

	// 每一条都同时核"默认档在不在"：前三条的默认档是 auto（跟随系统信号），
	// persona 的默认档是 plain —— 它**没有 auto**，因为系统里根本没有"要不要入场编排"这个开关。
	// 所以这里不能一刀切地要求含 auto（第一版就是这么写的，然后被自己的判据红了）。
	for _, ax := range []struct{ goName, feConst, wantDefault string }{
		{"MotionIDs", "MOTION_MODES", "auto"},
		{"ContrastIDs", "CONTRAST_MODES", "auto"},
		{"TransparencyIDs", "TRANSPARENCY_MODES", "auto"},
		{"PersonaIDs", "PERSONA_MODES", "plain"},
	} {
		feBlock := regexp.MustCompile(ax.feConst + `\s*=\s*\[([^\]]*)\]`).FindStringSubmatch(pers)
		if len(feBlock) == 0 {
			t.Fatalf("前端解析不出 %s —— 判据引用的结构已变了，先修判据再说结论", ax.feConst)
		}
		var fe []string
		for _, m := range feItemRe.FindAllStringSubmatch(feBlock[1], -1) {
			fe = append(fe, m[1])
		}
		goBlock := regexp.MustCompile(`(?s)` + ax.goName + `\s*=\s*\[\]string\{([^}]*)\}`).FindStringSubmatch(goSrc)
		if len(goBlock) == 0 {
			t.Fatalf("服务端解析不出 %s", ax.goName)
		}
		var server []string
		for _, m := range quotedRe.FindAllStringSubmatch(goBlock[1], -1) {
			server = append(server, m[1])
		}
		fe, server = quoteSet(fe), quoteSet(server)
		if strings.Join(fe, ",") != strings.Join(server, ",") {
			t.Errorf("%s：前端=%s 服务端=%s —— 漏掉的那一档，用户在另一台设备上的选择会被当非法值丢掉",
				ax.goName, strings.Join(fe, " "), strings.Join(server, " "))
		}
		// 两边的清单都必须含**那一档的默认值**。
		// 前端少一个：界面上没有默认档可选；服务端少一个：用户没表过态的请求（空串归一后的值）会被判非法，
		// 症状是每次登录都 400，而屏幕上看不出来。
		for side, list := range map[string][]string{"前端 " + ax.feConst: fe, "服务端 " + ax.goName: server} {
			if !listContains(list, ax.wantDefault) {
				t.Errorf("%s 的清单里没有默认档 %q：%s", side, ax.wantDefault,
					"这一档的默认语义是“没表过态”，缺它等于把默认路径判成非法值")
			}
		}
	}
}

func listContains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// TestAccentPaletteMatchesFrontend 管批3 的强调色。它比前三条多做一件事：
// **顺手校验色板本身可用** —— 每档必须有浅/深两枚 6 位 hex，缺一枚的后果不是"少一档"，
// 而是深色外观上 `accentColorFor` 返回 undefined，运行时把 `--apple-blue` 覆写成字符串 "undefined"，
// 于是整条染色族（选中态/焦点环/浅染）一起失效，而界面上看起来"主题没切过去"。
// 那种失效 `check:contrast` 看不见（它按注册表里的 hex 算），所以只能在清单这一侧拦。
func TestAccentPaletteMatchesFrontend(t *testing.T) {
	root := filepath.Join("..", "..")
	acc := mustRead(t, filepath.Join(root, "web", "src", "styles", "theme", "accents.ts"))
	goSrc := mustRead(t, filepath.Join(root, "internal", "model", "user", "preferences.go"))

	entryRe := regexp.MustCompile(`\{\s*id: '([a-z]+)',[^
]*?light: '(#[0-9a-fA-F]{6})',\s*dark: '(#[0-9a-fA-F]{6})'\s*\}`)
	var fe []string
	seen := map[string]bool{}
	for _, m := range entryRe.FindAllStringSubmatch(acc, -1) {
		fe = append(fe, m[1])
		if seen[m[1]] {
			t.Errorf("强调色 %s 在 ACCENT_REGISTRY 里登记了两次 —— 服务端清单只能有一条，用户在另一台设备上的选择会命中到哪一条说不清", m[1])
		}
		seen[m[1]] = true
	}
	if len(fe) < 4 {
		t.Fatalf("前端只解析出 %d 档强调色 —— 判据引用的结构已变了，先修判据再说结论", len(fe))
	}
	fe = quoteSet(append(append([]string{}, fe...), "auto"))

	block := regexp.MustCompile(`(?s)AccentIDs\s*=\s*\[\]string\{([^}]*)\}`).FindStringSubmatch(goSrc)
	if len(block) == 0 {
		t.Fatal("服务端解析不出 AccentIDs")
	}
	var server []string
	for _, m := range quotedRe.FindAllStringSubmatch(block[1], -1) {
		server = append(server, m[1])
	}
	if strings.Join(fe, ",") != strings.Join(quoteSet(server), ",") {
		t.Errorf("强调色清单：前端(含auto)=%s 服务端=%s —— 漏掉的那一档，用户在另一台设备上的选择会被当非法值丢掉",
			strings.Join(fe, " "), strings.Join(quoteSet(server), " "))
	}
}
