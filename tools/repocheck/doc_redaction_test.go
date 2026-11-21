package repocheck

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// 这台的射程：**发布物料里的"泄漏坐标"**。
//
// 为什么需要它：SECURITY.md 与 launch-checklist 第 8 条要如实写出"曾经泄漏过什么"，
// 但如实和可用是两件事 —— 一句同时写着"真实口令 + 占位内网主机 + 实名端口 + 三个提交号 +
// 密钥文件路径"的话，在内部是取证线索，公开之后是一张"曾经有什么、在哪、长什么样"的地图。
// 而 `docs/` 在凭据扫描里是免检的（那台门禁的排除表里有 docs/，因为文档示例必须给占位 DSN），
// 于是这类句子恰好从所有门禁中间走过去。2026-10-10 做发布体检时就是它漏出去的。
//
// 判据只抓"形状像坐标"的两类，不抓"提到了曾经过泄漏"这件事本身：
//   R1 十六进制反引号 token（提交号的典型长相）**与泄漏叙述同框** —— 单独一个 commit 号是
//      正常的历史引用（CHANGELOG、"这次删除发生在 commit xxx"），只有和"凭据/口令/血统"写在
//      一起时它才是"去哪取那把密钥"的坐标；
//   R2 尖括号占位主机后面跟真实端口 —— 占位 IP 配实名端口就是这条。
// 端口/IP 一律不许和占位尖括号同框：写成"一台内网 MySQL 的端口"就合规了。
//
// 反向验证：往 SECURITY.md 里塞一句"占位主机 + 真实端口"的 DSN，再塞一个和泄漏叙述同框的
// 十六进制反引号 token → 两条各点名一次。注意本文件自己的注释也不能留这些形状，
// 否则这台会把自己判红（第一版就把自己判红了，可见判据是真的在跑）。

var (
	redactHexToken   = regexp.MustCompile("`[0-9a-f]{8,40}`")
	redactNarrative  = regexp.MustCompile(`(?i)泄漏|凭据|口令|密钥|jwt|secret|血统`)
	redactCoordinate = regexp.MustCompile(`@tcp\(<[^>]*>:[0-9]{2,5}\)`)
	redactHostPort   = regexp.MustCompile(`(?i)(内网|intranet)[^。\n]{0,20}\b(?:10|172|192)\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}`)
)

// redactScope 是"会被陌生人读到"的那批文件。CHANGELOG 与 v2.0.0 发布说明不在里面：
// 前者是生成物（改写它等于伪造历史），后者记录的是发布当时的名字与状态。
func redactScope(t *testing.T) []string {
	t.Helper()
	root := filepath.Join("..", "..")
	files := []string{"SECURITY.md", "README.md", "README.zh-CN.md", "CONTRIBUTING.md", "AGENTS.md",
		"docs/design/brand.md", "docs/README.md"}
	globs := []string{"docs/release", "docs/engineering", "docs/gates", "tools/repocheck"}
	for _, g := range globs {
		entries, err := os.ReadDir(filepath.Join(root, g))
		if err != nil {
			t.Fatalf("读 %s: %v", g, err)
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := filepath.ToSlash(e.Name())
			if strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".go") {
				files = append(files, filepath.ToSlash(filepath.Join(g, name)))
			}
		}
	}
	return files
}

func TestReleaseDocsCarryNoLeakCoordinates(t *testing.T) {
	root := filepath.Join("..", "..")
	var hits []string
	scanned := 0
	for _, rel := range redactScope(t) {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		scanned++
		for i, line := range strings.Split(string(data), "\n") {
			line = strings.TrimRight(line, "\r")
			where := rel + ":" + strconv.Itoa(i+1)
			if redactHexToken.MatchString(line) && redactNarrative.MatchString(line) {
				hits = append(hits, "提交号与泄漏叙述同框 → "+where+" → "+clip(line))
			}
			if redactCoordinate.MatchString(line) {
				hits = append(hits, "泄漏坐标（占位主机配真实端口） → "+where+" → "+clip(line))
			}
			if redactHostPort.MatchString(line) {
				hits = append(hits, "内网地址字面量 → "+where+" → "+clip(line))
			}
		}
	}
	// 范围塌了的时候"0 命中"最像"一切干净"，所以下界必须写明。
	if scanned < 20 {
		t.Fatalf("只扫了 %d 个发布物料文件（应 20+）—— 射程判据坏了", scanned)
	}
	if len(hits) > 0 {
		t.Errorf("发布物料里有 %d 处泄漏坐标。公开仓库里这些不该是文档内容而是取证线索：\n  %s",
			len(hits), strings.Join(hits, "\n  "))
	}
	t.Logf("发布物料脱敏扫描：%d 个文件、%d 处坐标", scanned, len(hits))
}

func clip(s string) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) > 90 {
		return string([]rune(s)[:90]) + "…"
	}
	return s
}
