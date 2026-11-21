package repocheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// 这台的射程：**旧品牌名在代码里只能作为"双读的兜底参数"存在**。
//
// 为什么需要它：2026-10-07 改名 AIWiki → Citebase 时留了两处刻意的旧名 —— 表前缀 aw_ 与
// AIWIKI_* 环境变量（后者走 internal/utils/envname 的双读，因为 viper 与 os.Getenv 对认不出的键
// 一律静默丢弃，硬改名会砸掉所有现存 .env 与部署脚本）。这两处是**决定**，不是漏网。
// 但"决定"会随时间变成漏网：日志行、报错文案、贴到第三方页面上的 widget 脚本、打包脚本写进
// 产物 README 的那几行 —— 每一处都是一次"陌生人看到的项目名和 README 上的不一样"。
// 2026-10-10 的发布体检就是从 embed_widget.go 的 JS 全局名开始的：那段脚本原样进别人的页面，
// 等于让旧品牌在外面继续替本项目打广告。
//
// 判据用 AST 而不是 grep：grep 会把注释、双读的兜底实参和真正的用户可见串混成一桶，
// 而这里要的正是"只拦字符串字面量，且只放过 envname.Getenv 兜底那一个用法"。
//
// 反向验证（2026-10-10 两轮、2026-10-16 一轮都跑红过）：
//   ① 往非豁免 .go 里塞 logx.Info("请设置 AIWIKI_PROBE") → 第一条点名；
//   ② 把某处双读的实参顺序倒过来（旧名在前）→ 第二条点名。
//   ③ 2026-10-16 加前端兜底位之后：往 web/src/utils/core/storage.ts 里塞
//      `const BRAND_LABEL = 'AIWiki 知识库'`（普通常量行，不是注释、不含 LEGACY）→ 点名，
//      而同一个文件里 `const LEGACY_PREFIX = 'aiwiki:'` 与它下面那张表的各行都不点名。

var (
	brandOldName    = "aiwiki" // 一律转小写比较
	brandNewPrefix  = "CITEBASE_"
	brandFrontend   = "web/src"
	brandMinGoFiles = 600
)

// brandExemptDirs：旧名出现在这里是"说明兼容性的本体"，不是违例。
// tools/repocheck 是门禁自己的判据与夹具；internal/utils/envname 是双读机制的定义处。
// internal/brand 是第三种：它持有"改名前那几个写法"的清单，用来在启动时把存量库里
// 仍写着旧产品名的站点名归一掉（2026-10-12 实跑 /api/v1/site 回 "AIWiki" 引出的）。
// 这条豁免不是放宽判据 —— 清单内容本身被 internal/brand/brand_test.go 的
// TestLegacySiteNamesAreNothingButOldBrand 钉住：混进 "Wiki" 这类泛词，那条立刻红。
var brandExemptDirs = []string{"tools/repocheck/", "internal/utils/envname/", "internal/brand/"}

// brandFrontendLegacyHits 按行扫一个前端文件，返回"旧名出现在哪几行、其中哪些是兜底位"。
//
// 兜底位只认三种形状，每一种都必须是"兼容性的本体"而不是用户可见文案：
//
//	① 注释行 —— 说明"改名前叫什么、为什么要回落"，进不了任何产物；
//	② 名字里带 LEGACY 的声明**及其表体** —— 旧前缀与旧键名只准住在这类常量里。
//	   表体要一起认（`['aiwiki_doc_draft_', ...]` 那几行自己不含 LEGACY），
//	   否则就是把可读的字面量逼成拼接表达式，为了让门禁闭眼而把代码写弯，那是本末倒置；
//	③ *.test.ts —— 回落逻辑的夹具必须写出旧键名，不然那条测试测的是一个不存在的输入。
//
// 按行 + 按表体范围，而不是按文件放行：按文件豁免会让同一个文件里的用户可见串一起混过去，
// 而 storage.ts 恰恰是"既有回落常量、也可能被塞进文案"的那种文件。
func brandFrontendLegacyHits(text string, isTest bool) (legacy, total int) {
	inLegacyBlock := false
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.Contains(strings.ToLower(line), brandOldName) {
			// 表体结束的标志是顶格的 `];`（与 `const LEGACY...= [` 成对）
			if strings.HasPrefix(raw, "];") || strings.HasPrefix(raw, "] as") {
				inLegacyBlock = false
			}
			continue
		}
		total++
		trimmed := strings.TrimSpace(line)
		comment := strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "/*")
		if comment || strings.Contains(line, "LEGACY") || inLegacyBlock || isTest {
			legacy++
		}
		if strings.HasPrefix(trimmed, "const LEGACY") || strings.HasPrefix(trimmed, "let LEGACY") {
			inLegacyBlock = true
		}
	}
	return legacy, total
}

func TestOldBrandConfinedToDualReadFallback(t *testing.T) {
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()

	var filesScanned, dualReads, violations int
	var hits []string

	for _, rel := range trackedFiles(t) {
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || brandExempt(rel) {
			continue
		}
		filesScanned++
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		f, err := parser.ParseFile(fset, rel, src, parser.SkipObjectResolution)
		if err != nil {
			violations++
			hits = append(hits, rel+": 解析失败 → "+err.Error())
			continue
		}

		// 第一趟：登记 envname.Getenv 的实参位置，并核对双读本身的形状。
		fallbackPos := map[token.Pos]bool{}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isDualReadCall(call) {
				return true
			}
			dualReads++
			newName, legacyName := litValue(fset, call.Args[0]), litValue(fset, call.Args[1])
			if !strings.HasPrefix(newName, brandNewPrefix) || !strings.Contains(strings.ToLower(legacyName), brandOldName) {
				violations++
				hits = append(hits, rel+": 双读写反了 → 应为 Getenv(\""+brandNewPrefix+"…\", \"AIWIKI…\")，实为 ("+newName+", "+legacyName+")")
			}
			for _, arg := range call.Args {
				if lit, ok := arg.(*ast.BasicLit); ok {
					fallbackPos[lit.Pos()] = true
				}
			}
			return true
		})

		// 第二趟：带旧名的字符串字面量，只要不在兜底位上，就是违例。
		// 按 token.Pos 而不是文本判 —— 按文本判会把合法的双读全部误判成违例，
		// 于是所有人第一次跑就看到满屏红，然后学会关掉它（那和没有门禁一样糟）。
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			v := litValue(fset, lit)
			if !strings.Contains(strings.ToLower(v), brandOldName) || fallbackPos[lit.Pos()] {
				return true
			}
			violations++
			hits = append(hits, rel+": 用户可见串里出现旧品牌 → "+clip(v))
			return true
		})
	}

	// 前端：web/src 里旧名应当一处不剩（界面文案还没接 i18n，所以连"翻译键里的旧名"都不该有）。
	//
	// 例外只有三种，每一种都是"兼容性的本体"而不是用户可见文案 —— 见 brandFrontendAllowedLine：
	// 改名那次把 localStorage 前缀也换掉了却没留回落，2026-10-16 补这条链时，
	// 旧名必须重新出现在代码里才能读到老用户浏览器上那份值。禁止它等于禁止这次修复。
	feScanned := 0
	feFallbackLines := 0
	for _, rel := range trackedFiles(t) {
		if !strings.HasPrefix(rel, brandFrontend+"/") {
			continue
		}
		feScanned++
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		text := string(data)
		if !strings.Contains(strings.ToLower(text), brandOldName) {
			continue
		}
		allowed, total := brandFrontendLegacyHits(text, strings.HasSuffix(rel, ".test.ts"))
		feFallbackLines += allowed
		if total > allowed {
			violations += total - allowed
			hits = append(hits, rel+": 前端源码里有 "+strconv.Itoa(total-allowed)+" 行旧名不在兜底位（注释 / LEGACY 声明及其表体 / *.test.ts 夹具）")
		}
	}

	// 三道下界，各挡一种"判据自己坏了"：射程空了 / 双读没被 AST 认出来 / 前端目录名写错。
	if filesScanned < brandMinGoFiles {
		t.Fatalf("只解析了 %d 个 .go（非测试的应有 %d+）—— 遍历判据坏了，此时的 0 命中不可信",
			filesScanned, brandMinGoFiles)
	}
	if dualReads < 15 {
		t.Fatalf("只数到 %d 次 envname.Getenv 双读（现行代码有 18 次）—— AST 没认出现行的写法，兜底位白名单等于没生效", dualReads)
	}
	if feScanned < 100 {
		t.Fatalf("只扫了 %d 个 web/src 文件（应有 100+）—— 目录名写错时这一条会静默通过", feScanned)
	}
	// 第四道下界：豁免不许空转。前端旧名回落（storage.ts 的 LEGACY_PREFIX）要是被删掉了，
	// 上面那三档豁免就一条都不命中 —— 那时候该把豁免一起删掉，而不是让它等着被滥用。
	if feFallbackLines == 0 {
		t.Errorf("前端旧名豁免一次都没命中（扫了 %d 个文件）—— 要么回落逻辑已经不在了，要么判据改了形状；把 brandFrontendAllowedLine 一起清掉", feScanned)
	}

	if len(hits) > 0 {
		t.Errorf("旧品牌走出了「兜底参数」这个射程（违例 %d 处）。规矩：新名当正脸，旧名只许出现在两处兜底位 —— 后端的 envname.Getenv 第二实参、前端的注释 / LEGACY 声明 / *.test.ts 夹具：\n  %s",
			violations, strings.Join(hits, "\n  "))
	}
	t.Logf("品牌收口扫描：%d 个 .go、%d 次双读、%d 个前端文件（旧名兜底行 %d）、%d 处违例", filesScanned, dualReads, feScanned, feFallbackLines, violations)
}

func brandExempt(rel string) bool {
	for _, d := range brandExemptDirs {
		if strings.HasPrefix(rel, d) {
			return true
		}
	}
	return false
}

// isDualReadCall 只认 envname.Getenv("新名", "旧名") 这一种形状。
// 刻意不认裸的 Getenv：os.Getenv 也叫这个名字，把它当双读会既漏判（它的第二参不是兜底名）
// 又误判（它的实参本来就该是现行名）。
func isDualReadCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Getenv" || len(call.Args) != 2 {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "envname"
}

func litValue(fset *token.FileSet, e ast.Expr) string {
	lit, ok := e.(*ast.BasicLit)
	if !ok {
		return ""
	}
	if s, err := strconv.Unquote(lit.Value); err == nil {
		return s
	}
	return strings.Trim(lit.Value, "`\"")
}
