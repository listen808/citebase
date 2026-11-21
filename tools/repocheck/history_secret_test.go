package repocheck

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// 这台的射程：**master 的整条提交历史**，而不是 HEAD 那一个快照。
//
// 为什么需要它：secret_scan_test.go 的判据是 `git ls-files`，也就是"现在还挂在树上的文件"。
// 一个在第 40 个提交里加进去、第 90 个提交里删掉的口令，对它完全隐形 —— 而仓库一公开，
// 别人 `git log -p` 就能把它翻出来。"当前树干净"和"公开出去的东西干净"是两件事，
// 后者才是发布要的判据。（本仓库真正泄漏过的凭据恰好只在另一条血统上，见 launch-checklist 第 8 条：
// 那类东西正是"从当前树里看不见"的形状。）
//
// 范围只用 `git rev-list master` 那一条血统，不是 --all：脏血统的 ref 早就不在远端也不在本地，
// 把它请回判据里只会让门禁永远红，而永远红的门禁没人看。
//
// 反向验证（2026-10-10）：往工作区造一条 `AIWIKI_JWT_SECRET="32+字符"` 提交一次，本机跑必然红；
// 把那次提交删掉（reset + 清 reflog）后复跑归零 —— 归零时这里会打印扫了多少行，
// 行数掉到几千说明 diff 范围被路径排除规则吃掉了，那种"0 命中"不可信。

// RE2 不支持 (?!\+) 这种前瞻（本仓另一台门禁也记过同一个坑），所以"排除 +++ 文件头"
// 用两次前缀判断手工做。
func isAddedLine(line string) bool {
	return strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "++")
}

// historySkipPathspecs 与 HEAD 那台保持同一套排除：测试夹具、文档示例、模板 DSN。
var historySkipPathspecs = []string{
	":(exclude,glob)**/*_test.go",
	":(exclude)docs",
	":(exclude)configs/config_sample.yaml",
	":(exclude)web/package-lock.json",
	":(exclude,glob)**/*.snap",
}

func TestNoCredentialsInMasterHistory(t *testing.T) {
	root := filepath.Join("..", "..")
	args := append([]string{"log", "-p", "-U0", "--no-color", "master", "--", "."}, historySkipPathspecs...)
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git log -p: %v（stderr 里通常写着原因）", err)
	}

	count := 0
	for _, c := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(c, "commit ") && len(c) > 46 {
			count++
		}
	}
	if count < 150 {
		t.Fatalf("只数到 %d 个提交（master 上应有 175 个）—— 范围判据坏了，此时的 0 命中不可信", count)
	}

	hits := []string{}
	scanned := 0
	for _, line := range strings.Split(string(out), "\n") {
		if !isAddedLine(line) {
			continue // 只看新增行：删除行是把东西清出去的动作，不是把它公开出去的动作
		}
		body := line[1:]
		if strings.Contains(body, "\x00") {
			continue
		}
		scanned++
		hits = append(hits, historyHits(body)...)
	}

	// 同样给"判据自己坏了"留一道下界：路径排除写错时命中数是 0，扫到的行数也会塌下去。
	if scanned < 20000 {
		t.Fatalf("只扫了 %d 行新增内容 —— git log -p 的输出范围不对，此时的 0 命中不可信", scanned)
	}
	if len(hits) > 0 {
		t.Errorf("master 历史里出现过疑似真实凭据 %d 处（仓库一旦公开，git log -p 就能翻到）:\n  %s",
			len(hits), strings.Join(hits, "\n  "))
	}
	t.Logf("历史凭据扫描：%d 个提交、%s 行新增内容、%d 处疑似凭据", count, strconv.Itoa(scanned), len(hits))
}

// historyHits 是这台真正的判据：一行新增内容里有哪些凭据形状。
// 单独抽出来是为了能被**hermetic 地反向验证** —— 历史扫描要证明"它会红"就得往 master 上造一次
// 真提交再 reset，那是在改分支指针；把匹配逻辑拿出来直接喂形状，同样能在有人把正则调哑的那天响。
func historyHits(body string) []string {
	var hits []string
	for _, sp := range secretPatterns {
		if !sp.re.MatchString(body) || secretAllowed(sp.allow, body) {
			continue
		}
		hits = append(hits, sp.name+" → "+clipSecret(body))
	}
	return hits
}

func TestHistoryMatcherStillCatchesRealShapes(t *testing.T) {
	mustCatch := []struct{ name, line string }{
		// 夹具一律用"明显是假的"值：文档 RFC 5737 的 TEST-NET 地址、看得出的假口令、清一色 A 的 base64。
		// 判据要的形状照样满足，但陌生人不会把一个真的内网地址或真密钥读进公开仓库。
		{"注释里的 DSN 形状", `dsn: "root:examplepw@tcp(192.0.2.1:3306)/citebase?charset=utf8mb4"`},
		{"写死的 JWT secret", `AIWIKI_JWT_SECRET="notarealsecret0123456789abcdefghijklmnop"`},
		{"整行一把 base64 密钥", `AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=`},
		{"长 API key", `openai_key: sk-abcdEFGH1234ijklMNOP5678qrstUVWX`},
		{"私钥块", `-----BEGIN RSA PRIVATE KEY-----`},
	}
	for _, c := range mustCatch {
		if len(historyHits(c.line)) == 0 {
			t.Errorf("判据哑了：这条真凭据形状没被抓住 —— %s", c.name)
		}
	}

	mustPass := []struct{ name, line string }{
		{"环境变量插值模板", `AIWIKI_DB_DSN: "citebase:${MYSQL_PASSWORD}@tcp(mysql:3306)/citebase"`},
		{"明显的占位口令", `dsn: "root:YOUR_PASSWORD@tcp(127.0.0.1:3306)/citebase"`},
		{"假账号对", `dsn: "user:password@tcp(127.0.0.1:3306)/citebase"`},
		{"空值待填", `AIWIKI_ENCRYPT_KEY=`},
	}
	for _, c := range mustPass {
		if got := historyHits(c.line); len(got) > 0 {
			t.Errorf("白名单漏了，模板样例被报成违例（这样门禁会常红，然后没人看它）: %s → %v", c.name, got)
		}
	}
}
