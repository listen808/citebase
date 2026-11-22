package repocheck

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// 这台管三件事，都是 NOTICE.md 会不会变成"看着合规的空壳"的关键：
//
//  1. **覆盖率**：NOTICE 里那份 machine-readable 清单必须和现场算出来的构建清单**逐条相等**。
//     多了 = 清单在声明我们分发了一个其实没链接进来的东西（法律上比漏写更糟）；
//     少了 = 加了依赖没人重算 —— 后者正是这种文件最常见的死法。
//  2. **copyleft 报警**：Go 侧出现 GPL / AGPL / LGPL 档就红。本产品是 MIT，
//     链接进二进制的 copyleft 库不是"注明一下"能解决的，属于发布决策。
//  3. **不许有未判名**：出现 UNKNOWN / NO_SOURCE 说明"我们没真读过那份许可证"，
//     而清单存在的意义恰恰是"读过了"。宁可红，不许含糊。
//
// 判据形状与 release_claims_test.go 一致：不拿测试里抄的数和文件比，而是现场重算再比字符串。
// 反向验证：把 NOTICE 清单里任意一行删掉 → 点名缺失；改成构建清单里没有的模块 → 点名多余；
// 往 web/package.json 加一个依赖而不重生成 → 红。
//
// 生成器是 NOTICE.md 的唯一作者：`node tools/gen-notice.mjs`。手改会在下一次重算时被冲掉。

// noticeRow 是清单里的一行，kind 取 "go" 或 "npm"。
type noticeRow struct {
	kind string
	name string // go 是 module@version，npm 是包名
}

// NOTICE 的声明对象是 Windows 安装包（发布物），清单口径与生成器 tools/gen-notice.mjs 保持一致。
const (
	noticeGOOS   = "windows"
	noticeGOARCH = "amd64"
)

func TestNoticeCoversBuildList(t *testing.T) {
	root := filepath.Join("..", "..")
	raw, err := os.ReadFile(filepath.Join(root, "NOTICE.md"))
	if err != nil {
		t.Fatalf("读 NOTICE.md: %v —— 生成：node tools/gen-notice.mjs", err)
	}
	listed := parseNoticeRows(t, string(raw))

	gotGo, gotNpm := buildList(t, root)
	declaredGo, declaredNpm := splitRows(listed)

	// Go 侧比 module@version 全串：只比模块名会让"版本升了但清单没重算"静默通过，
	// 而许可正文是按版本读的（同一库换过许可的真实案例：apexcharts 从 MIT 换成双许可）。
	assertSetEqual(t, "Go 构建清单", normalize(gotGo), normalize(declaredGo))
	assertSetEqual(t, "web/package.json 直接依赖", normalize(gotNpm), normalize(declaredNpm))

	for _, line := range strings.Split(string(raw), "\n") {
		head := strings.TrimSpace(line)
		if !strings.HasPrefix(head, "### ") {
			continue
		}
		kind := strings.TrimSpace(strings.SplitN(head[4:], "—", 2)[0])
		switch {
		case strings.Contains(kind, "GPL"):
			t.Errorf("NOTICE 里有 copyleft 档 %q —— MIT 产品链接进产物的库不能靠注明解决，见文件头的说明", kind)
		case kind == "UNKNOWN" || kind == "NO_SOURCE":
			t.Errorf("NOTICE 里有未判名的许可档 %q —— 说明那份 LICENSE 没被真读过，别把它省略成合规的样子", kind)
		}
	}
}

// parseNoticeRows 读文件末尾那段围栏代码块（注释把它钉在最后）。
func parseNoticeRows(t *testing.T, notice string) []noticeRow {
	t.Helper()
	i := strings.LastIndex(notice, "machine-readable coverage list")
	if i < 0 {
		t.Fatalf("NOTICE.md 没有 machine-readable 清单段 —— 它是这台唯一的比对面，别删")
	}
	rest := notice[i:]
	open := strings.Index(rest, "```")
	if open < 0 {
		t.Fatalf("清单注释后面找不到开栏")
	}
	body := rest[open+3:]
	end := strings.Index(body, "```")
	if end < 0 {
		t.Fatalf("清单段没有闭栏")
	}
	var rows []noticeRow
	for _, line := range strings.Split(strings.TrimSpace(body[:end]), "\n") {
		kind, name, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok || name == "" {
			t.Errorf("清单里有一行不成形：%q（应为 go<TAB>module@version 或 npm<TAB>包名）", line)
			continue
		}
		if kind != "go" && kind != "npm" {
			t.Errorf("清单里有一种不认识的类别 %q", kind)
			continue
		}
		rows = append(rows, noticeRow{kind, name})
	}
	if len(rows) < 50 {
		t.Fatalf("清单只有 %d 行，构建清单应有 100 上下 —— 解析坏了", len(rows))
	}
	return rows
}

// buildList 现场重算两份清单：链接进产物的 Go 模块、前端的直接依赖。
// 用 `go list -deps` 而不是 `go list -m all`：后者量的是模块图（219 条），
// 前者才是"这个二进制真的带了它"（102 条）。把没链接进来的也声明一遍是虚报分发范围。
func buildList(t *testing.T, root string) (goMods, npmDeps []string) {
	t.Helper()

	cmd := exec.Command("go", "list", "-deps",
		"-f", "{{if .Module}}{{.Module.Path}} {{.Module.Version}}{{end}}",
		"./app/server", "./tools/...")
	// go list 的 ./... 按进程工作目录解析，而测试的工作目录是 tools/repocheck —— 不显式给
	// cmd.Dir 会去 stat tools/repocheck/tools/... 然后报"目录不存在"（实测踩过）。
	cmd.Dir = root
	// NOTICE.md 是按 Windows 安装包（唯一对外分发的二进制）的构建清单生成的；
	// 两个平台真正链接进来的原生依赖并不相同（Windows 走 go-sysconf/numcpus，Linux 走 go-ole/wmi），
	// 不钉 GOOS 就会在 Linux CI 上拿 Linux 清单去比 Windows 清单，永远对不上。
	// 生成器 tools/gen-notice.mjs 用同一对常量，两边必须一起改。
	cmd.Env = append(os.Environ(), "GOOS="+noticeGOOS, "GOARCH="+noticeGOARCH)
	// 只解析 stdout：模块缓存是冷的时候，go list 会往 stderr 打 "@downloading …"，
	// CombinedOutput 把两股流混在一起，这些行会被当成模块行，两条平台专属依赖就会"消失"。
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -deps 失败（contributor 机器上先跑得通 go build）：%v\n%s", err, stderr.String())
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		mod, ver, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok || mod == "" || mod == "github.com/listen808/citebase" || ver == "(devel)" {
			continue
		}
		key := mod + "@" + ver
		if !seen[key] {
			seen[key] = true
			goMods = append(goMods, key)
		}
	}

	data, err := os.ReadFile(filepath.Join(root, "web", "package.json"))
	if err != nil {
		t.Fatalf("读 web/package.json: %v", err)
	}
	var pkg struct {
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		t.Fatalf("解析 web/package.json: %v", err)
	}
	for name := range pkg.Dependencies {
		npmDeps = append(npmDeps, name)
	}
	if len(goMods) < 50 || len(npmDeps) < 5 {
		t.Fatalf("现场重算的清单太小（go=%d npm=%d）—— 判据坏了", len(goMods), len(npmDeps))
	}
	return goMods, npmDeps
}

func splitRows(rows []noticeRow) (goMods, npmDeps []string) {
	for _, r := range rows {
		if r.kind == "go" {
			goMods = append(goMods, r.name)
		} else {
			npmDeps = append(npmDeps, r.name)
		}
	}
	return goMods, npmDeps
}

func normalize(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func assertSetEqual(t *testing.T, label string, want, got []string) {
	t.Helper()
	w := map[string]bool{}
	g := map[string]bool{}
	for _, x := range want {
		w[x] = true
	}
	for _, x := range got {
		g[x] = true
	}
	var missing, extra []string
	for _, x := range want {
		if !g[x] {
			missing = append(missing, x)
		}
	}
	for _, x := range got {
		if !w[x] {
			extra = append(extra, x)
		}
	}
	if len(missing) > 0 {
		t.Errorf("%s 有 %d 条没写进 NOTICE：%s（重跑 node tools/gen-notice.mjs）",
			label, len(missing), joinSome(missing))
	}
	if len(extra) > 0 {
		t.Errorf("%s 里多写了 %d 条其实不在现场清单上的东西：%s —— 声明分发范围宁窄勿宽",
			label, len(extra), joinSome(extra))
	}
	t.Logf("%s：%d 条与现场一致", label, len(want))
}

func joinSome(s []string) string {
	if len(s) > 8 {
		return strings.Join(s[:8], ", ") + " …"
	}
	return strings.Join(s, ", ")
}
