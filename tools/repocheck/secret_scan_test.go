package repocheck

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// 这台扫的是**入库内容**里像真实凭据的东西。
//
// 为什么需要它：仓库要公开，而"提交前记得把手工填的 DSN 注释掉"这件事靠人记是记不住的 ——
// 本仓库的旧血统里就真有两条：一条注释里指向内网 MySQL 的 root 口令 DSN，
// 以及 internal/ai/schema/data/.jwt_secret 那个 base64 密钥文件本身。
// 两者的端口与提交号已按 docs/release/launch-checklist.md 第 8 条脱敏（由 doc_redaction_test.go 常驻拦着）。
// 它们不在 master 上（见 docs/release/launch-checklist.md 第 8 条），
// 所以这台现在应当是绿的 —— 它的职责是保证以后别再进来。
//
// 判据是"形状 + 白名单"两条一起：形状负责抓，白名单负责不抓占位符。
// 只按形状抓会把 root:YOUR_PASSWORD@tcp(...) 这类模板样例报成违例，
// 于是所有人第一次跑就学会忽略它 —— 那是永远红，和永远绿一样糟。

var secretPatterns = []struct {
	name  string
	re    *regexp.Regexp
	allow []*regexp.Regexp
}{
	{
		// 形状：dsn 之类的键 = "user:口令@"。
		// 不能要求以 @" 收尾 —— 真实 DSN 是 root:pass@tcp(host)/db，口令后面跟的是 @t 而不是引号，
		// 上一版正是这个错，让它对历史里那条真凭据完全视而不见（反向验证才抓出来）。
		name: "DSN 里带疑似真实口令",
		re:   regexp.MustCompile(`(?i)[a-z0-9_]*dsn[a-z0-9_]*\s*[:=]\s*"[^\s"]{2,}:[^\s@"]{5,}@`),
		allow: []*regexp.Regexp{
			regexp.MustCompile(`(?i)YOUR_PASSWORD|你的密码|CHANGE_ME|xxxx|placeholder|s3cr3t|secret123`),
			// 口令位是环境变量插值时那是模板不是凭据（docker-compose 就这么写）。
			// 模板里最常见的假账号对（package-windows.bat 生成的那份配置用它教人怎么填）。
			regexp.MustCompile(`"user:password@`),
			regexp.MustCompile(`[=:]\s*"[^"\s]*:\$\{`),
		},
	},
	{
		// RE2 不支持 (?!) 前瞻，所以"清一色 x 的占位 key"交给 allow 那条去放过。
		name:  "像真的 API key（sk- 后跟 20+ 字符）",
		re:    regexp.MustCompile(`sk-[A-Za-z0-9+/_-]{20,}`),
		allow: []*regexp.Regexp{regexp.MustCompile(`(?i)YOUR|CHANGE|EXAMPLE`), regexp.MustCompile(`sk-x{6,}`)},
	},
	{
		name: "私钥块",
		re:   regexp.MustCompile(`BEGIN [A-Z ]*PRIVATE KEY`),
	},
	{
		name: "整行就是一条 44 字符 base64（密钥文件的典型形状）",
		re:   regexp.MustCompile(`^[A-Za-z0-9+/]{43}=$`),
	},
	{
		// 上一条只管"整个文件就是一行密钥"（data/.jwt_secret 的真实形状）。
		// 这一条管写在配置里的形状：键名点明是 jwt secret / encrypt key，值是 32+ 字符。
		// 不敢放宽成"任何 43 字符 base64"—— package-lock 里的 sha512 integrity 会撞进来，
		// 门禁第一天就常红，然后所有人都学会忽略它。
		name: "配置里写死的 JWT secret / 加密密钥",
		re:   regexp.MustCompile(`(?i)(jwt[_-]?secret|encrypt[_-]?key)[\s]*[:=][\s]*["']?[A-Za-z0-9+/]{32,}`),
		allow: []*regexp.Regexp{
			regexp.MustCompile(`(?i)YOUR|CHANGE|xxxx|placeholder|<[A-Z_]+>`),
			regexp.MustCompile(`[=:]\s*["']?\$\{`), // 环境变量插值
			regexp.MustCompile(`[=:]\s*["']?\s*$`), // 空值（模板里就是留空让人填）
		},
	},
}

// 不扫这些：测试里造的多是假凭据；文档示例一律用占位符且由链接门禁管着；
// configs/config_sample.yaml 必须给出 DSN 形状，用的是占位口令。
var secretSkipSubstrings = []string{"_test.go", "testdata", "docs/", "configs/config_sample.yaml"}

var secretSkipExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true, ".zip": true,
	".gz": true, ".exe": true, ".dll": true, ".woff": true, ".woff2": true, ".ttf": true,
	".eot": true, ".pdf": true, ".bolt": true, ".db": true, ".mp4": true, ".webm": true,
}

func TestNoCommittedCredentials(t *testing.T) {
	// 扫描面 = 会公开出去的东西 = git 跟踪的文件清单。
	// 本地被 ignore 掉的 configs/config.yaml 里当然该是真口令，把它报成违例只会让门禁常红，
	// 而常红的门禁和没有门禁一样没人看。
	tracked := trackedFiles(t)
	root := filepath.Join("..", "..")
	var hits []string
	scanned := 0

	for _, rel := range tracked {
		if secretSkipped(rel) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		scanned++
		for i, line := range strings.Split(string(data), "\n") {
			line = strings.TrimRight(line, "\r")
			for _, sp := range secretPatterns {
				if !sp.re.MatchString(line) {
					continue
				}
				if secretAllowed(sp.allow, line) {
					continue
				}
				hits = append(hits, sp.name+" → "+rel+":"+strconv.Itoa(i+1)+" → "+clipSecret(line))
			}
		}
	}

	// 判据自己坏了也要看得见：范围算错时"0 命中"最像"一切干净"。
	if scanned < 800 {
		t.Fatalf("只扫了 %d 个入库文件（仓库里应有 1100+）—— 遍历判据坏了，此时的 0 命中不可信", scanned)
	}
	if len(hits) > 0 {
		t.Errorf("入库内容里出现疑似真实凭据 %d 处（公开之前必须清掉，或改成明显的占位符）:\n  %s",
			len(hits), strings.Join(hits, "\n  "))
	}
	t.Logf("凭据扫描：%d 个入库文件，%d 处疑似真实凭据", scanned, len(hits))
}

// TestNoSecretFilenamesTracked 拦"文件名本来在 .gitignore 名单里，却被 git add -f 加进来"。
// 判据读的是 git 的跟踪清单而不是磁盘 —— 本地存在 ./data/.jwt_secret 是设计如此（首启生成），
// 被跟踪才是问题。
func TestNoSecretFilenamesTracked(t *testing.T) {
	banned := map[string]bool{
		"config.yaml":  true,
		".jwt_secret":  true,
		".encrypt_key": true,
		".env":         true,
	}
	for _, f := range trackedFiles(t) {
		if banned[filepath.Base(f)] {
			t.Errorf("%s 被提交进了仓库 —— 它在 .gitignore 名单里，只该存在于本地", f)
		}
	}
}

// trackedFiles 取 git 的跟踪清单。必须显式指到仓库根：测试的工作目录是包目录，
// 不设 cmd.Dir 的话 git 只列 tools/repocheck 下面那几个文件 —— 上面那个数量下限就是为这种情况准备的
// （它确实响了：第一次跑只数到 2 个文件，而判据当时会打印"0 处凭据，一切干净"）。
func trackedFiles(t *testing.T) []string {
	t.Helper()
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = filepath.Join("..", "..")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	// 分隔符用 []byte{0} 而不是字面转义：写这台的上一版里那个转义在落盘时变成了真 NUL 字节，
	// Go 编译器直接报 illegal character NUL —— 换成字节构造就没有这个坑。
	var files []string
	for _, f := range strings.Split(string(out), string([]byte{0})) {
		if f != "" {
			files = append(files, filepath.ToSlash(f))
		}
	}
	return files
}

func secretSkipped(rel string) bool {
	for _, s := range secretSkipSubstrings {
		if strings.Contains(rel, s) {
			return true
		}
	}
	return secretSkipExts[filepath.Ext(rel)]
}

func secretAllowed(allow []*regexp.Regexp, line string) bool {
	for _, a := range allow {
		if a.MatchString(line) {
			return true
		}
	}
	return false
}

func clipSecret(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}
