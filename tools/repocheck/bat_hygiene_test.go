package repocheck

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// 这台管 Windows 打包脚本自身的一条硬约束：**scripts\*.bat 必须纯 ASCII，
// 而且 cmd 括号块里的 echo 文本必须把 `( ) & < > |` 逐个转义。**
//
// 起因（2026-10-14 实测）：给 package-windows.bat 加中文注释之后，脚本在 [3/7] 报
// `'——' is not recognized`、在 [5/7] 报 `'y' is not recognized` 后整条链中断 ——
// zip 与安装包都不产出。不是显示乱码那么简单：cmd 是**按字节偏移逐行续读**脚本的，
// chcp 65001 下一条多字节行的字节数与字符数不等，读到后面偏移就错位，从某行中间继续解析。
// 错位点取决于字节数，所以症状会漂 —— 第一次崩在注释行，第二次崩在 `if not defined GSE_DIR` 上。
//
// 同一批里还有第二处独立故障：README 生成块的 echo 文本里出现未转义的
// `(legacy AIWIKI_* works too)`（旧名双读那批改动加进去的），cmd 提前闭合括号块，
// 剩下一个孤零零的 `.` 报 `. was unexpected at this time.`。
//
// 两类都只有真的用 cmd 跑一遍才暴露，而完整打包要 4 分钟还会清 dist，
// 所以在这里用静态检查把判据钉住，改 bat 的人当场就能拿到。

var (
	batEscapedMeta = regexp.MustCompile(`\^[()&<>|]`)
	batEchoLine    = regexp.MustCompile(`(?i)^\s*echo[ .]`)
)

// 剥掉转义后的元字符，剩下才是 cmd 括号配对真正看到的形状。
func batShape(line string) string {
	return batEscapedMeta.ReplaceAllString(strings.TrimSpace(line), "")
}

func TestWindowsBatchFilesArePureAscii(t *testing.T) {
	dir := filepath.Join("..", "..", "scripts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读不到 scripts 目录: %v", err)
	}
	var checked int
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".bat") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("读 scripts\\%s: %v", e.Name(), err)
		}
		checked++
		for i, line := range strings.Split(string(data), "\n") {
			for j, r := range line {
				if r > 0x7f {
					t.Errorf("scripts\\%s 第 %d 行第 %d 列有非 ASCII 字符 %q —— cmd 在 chcp 65001 下按字节续读脚本，"+
						"多字节内容让偏移错位，把行中间的内容当命令执行（实测 '——'/'y' is not recognized，打包链中断）。"+
						"注释改成英文；要中文文案就放随包文件让 bat copy。",
						filepath.Base(e.Name()), i+1, j+1, string(r))
					break
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("scripts 下一个 .bat 都没扫到 —— 判据在空转")
	}
}

func TestBatchEchoBlocksHaveNoUnescapedMetacharacters(t *testing.T) {
	p := filepath.Join("..", "..", "scripts", "package-windows.bat")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读 %s: %v", p, err)
	}
	lines := strings.Split(string(data), "\n")
	var hits []string
	blocks := 0
	inBlock := false
	for i, line := range lines {
		shape := batShape(line)
		if !inBlock {
			if strings.HasSuffix(shape, "(") {
				inBlock = true
				blocks++
			}
			continue
		}
		// 块内的 echo 才可能被 cmd 当成块文本重新解析，其余行（copy/set/xcopy）不在这里管
		if m := batEchoLine.FindString(line); m != "" {
			payload := batEscapedMeta.ReplaceAllString(line[len(m):], "")
			for _, meta := range []string{"(", ")", "&", "<", ">", "|"} {
				if strings.Contains(payload, meta) {
					hits = append(hits, "第 "+strconv.Itoa(i+1)+" 行：括号块内的 echo 有未转义的 '"+meta+
						"'（该写成 ^"+meta+"）—— cmd 会提前闭合括号块，剩下的半行被当命令执行")
					break
				}
			}
		}
		if strings.HasPrefix(shape, ")") {
			// `) else (` 是关了又开，只有不再以 ( 收尾才算真出块
			if !strings.HasSuffix(shape, "(") {
				inBlock = false
			}
		}
	}
	for _, h := range hits {
		t.Error(h)
	}
	if blocks == 0 {
		t.Fatal("一个括号块都没认出来 —— 判据在空转")
	}
	if inBlock {
		t.Fatal("扫到文件结尾仍在括号块里：块结构没解析完，上面那条检查不能算通过")
	}
}
