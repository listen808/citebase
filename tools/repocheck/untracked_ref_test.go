package repocheck

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 这台的射程：文档里用反引号写出来的**仓库相对路径**，是否落在 `.gitignore` 覆盖的目录里。
//
// 为什么需要它：`tools/validate-doc-links.mjs` 只解析 `[标题](路径)` 这一种形状 ——
// 反引号里的路径、代码块里的路径、行号引用它一概看不见（这条盲区已经咬过一次：
// README 首页曾经展示一份早就删掉的文档）。而"引用了一个不入库的本地产物"这件事，
// 在本地永远是真的（那个目录就在磁盘上），推上去之后才变成死链 ——
// 于是它恰好是所有门禁里最抓不到的那一类：只在公开之后暴露。
//
// 判据故意收窄到"缺失 **且** 被 .gitignore 命中"两条同时成立才红：
// 只按"从本文档目录解析不到"判红会误伤两类合法写法 —— 运行时按 CWD 解析的路径
// （`./web/dist/index.html`）与站在别的目录说话的路径（`../../SECURITY.md`，那是 Go 测试读它的方式）。
// 这两类实测都存在，第一版把它们都报了，于是按上面的判据收回。
//
// 反向验证：在 docs/design/ 下临时写一份引用 `../spark-output/bench/x.md` 的文档 → 一条红，
// 同时确认 `./web/dist/index.html` 那种运行时路径不跟着红。

var (
	// 两种写法都要抽：反引号里的路径 validate-doc-links 根本不解析；`[标题](路径)` 它解析，
	// 但只验"磁盘上在不在"—— 而未入库的本地产物恰好"在"，于是从它中间走过去。
	ignoreRefTok   = regexp.MustCompile("`\\.\\./[^`]+`|\\]\\(\\.\\./[^)]+\\)")
	ignoreRefValid = regexp.MustCompile(`\.(md|html|png|svg|go|json|yml|yaml|mjs|ts|vue)$`)
)

// trimRef 把两种形状的包装都剥掉。
func trimRef(tok string) string {
	tok = strings.TrimPrefix(tok, "`")
	tok = strings.TrimSuffix(tok, "`")
	tok = strings.TrimPrefix(tok, "](")
	tok = strings.TrimSuffix(tok, ")")
	return tok
}

func TestDocsDoNotReferenceUntrackedLocalPaths(t *testing.T) {
	root := filepath.Join("..", "..")
	var hits, deadInformative []string
	var candidates int

	// 遍历面 = git 跟踪的 .md（不是磁盘上的全部 .md）：磁盘上有 .git/、node_modules/、
	// web/dist/、data/ 这些加起来几百 MB 的东西，走一遍既慢又会把不属于发布面的文件算进判据。
	var mds []string
	for _, f := range trackedFiles(t) {
		if strings.HasSuffix(f, ".md") && !mdRefSkipped(f) {
			mds = append(mds, f)
		}
	}
	if len(mds) < 15 {
		t.Fatalf("只找到 %d 个参与扫描的 md（仓库里应有 20+）—— 范围判据坏了", len(mds))
	}

	for _, rel := range mds {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("读 %s: %v", rel, err)
		}
		dir := filepath.Dir(filepath.FromSlash(rel))
		for _, tok := range ignoreRefTok.FindAllString(string(data), -1) {
			cand := trimRef(tok)
			if !strings.HasPrefix(cand, "../") || !ignoreRefValid.MatchString(cand) ||
				strings.ContainsAny(cand, "{}*<>| ") {
				continue // 只认" ../ 起头 + 已知扩展名 + 无通配/花括号"的仓库相对写法
			}
			candidates++
			// 存在性检查必须拼到仓库根：测试的工作目录是 tools/repocheck，
			// 拿 "docs/../AGENTS.md" 直接 Stat 会把每个正常链接都数成"解析不到"（第一版就这样）。
			target := filepath.Join(root, dir, filepath.FromSlash(cand))
			if _, err := os.Stat(target); err == nil {
				continue
			}
			if gitIgnored(root, filepath.ToSlash(filepath.Join(dir, filepath.FromSlash(cand)))) {
				hits = append(hits, rel+" 引用了未入库的本地产物 → "+cand)
			} else {
				deadInformative = append(deadInformative, rel+" → "+cand)
			}
		}
	}

	// 判据坏掉的两种样子都要能看见：一个候选都没数到 = 抽取正则哑了。
	if candidates < 3 {
		t.Fatalf("只数到 %d 个仓库相对路径候选 —— 抽取判据坏了，此时的 0 命中不可信", candidates)
	}
	if len(hits) > 0 {
		t.Errorf("文档把不入库的本地产物当成了仓库路径（公开后就是死链，而且没有任何别的门禁会发现）:\n  %s",
			strings.Join(hits, "\n  "))
	}
	if len(deadInformative) > 0 {
		t.Logf("另有 %d 处 ../ 形状的路径从本文档目录解析不到但不属于未入库产物（运行时路径或跨目录说话，故意不判红）: %s",
			len(deadInformative), strings.Join(deadInformative, " | "))
	}
	t.Logf("未入库引用扫描：%d 个候选路径、%d 处判红", candidates, len(hits))
}

// mdRefSkipped：生成物与历史快照不参与（它们的内容由生成脚本或当时的事实决定，改它等于伪造）。
func mdRefSkipped(rel string) bool {
	for _, s := range []string{"CHANGELOG.md", "docs/swagger/", "docs/audits/", "node_modules/", ".git/"} {
		if strings.Contains(rel, s) {
			return true
		}
	}
	return false
}

// gitIgnored 用 git 自己的判断，不重解析 .gitignore —— 后者会把"目录规则/取反规则/锚定斜杠"
// 重新实现一遍，而那正是最容易和真实 git 行为分叉的地方。
func gitIgnored(root, relPath string) bool {
	cmd := exec.Command("git", "check-ignore", "-q", "--", relPath)
	cmd.Dir = root
	err := cmd.Run()
	// check-ignore：exit 0 = 命中 ignore 规则；exit 1 = 没命中；>=128 = git 自己出错。
	if err == nil {
		return true
	}
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
		return false
	}
	return false
}
