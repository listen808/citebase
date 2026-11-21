package repocheck

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// 这台管部署侧的三张脸是否对得上：
//   docker-compose.yml 里的 ${VAR} ↔ .env.example 提供的键 ↔ 代码真的 os.Getenv 的名字。
//
// 起因是本轮实测出的一条断链：README 与 SECURITY.md 都写着"生产环境务必设置
// AIWIKI_ENCRYPT_KEY"，而 compose 的 environment 段里**根本没有这一项**，
// .env 模板里也没有 —— 于是照着文档装 Docker 的人，那条要求从来没有生效过，
// 并且不报错：应用会安静地在容器里自己生成一把。单容器看不出来，
// 多副本时每个副本一把，A 加密的口令 B 解不开。
// 这类"文档要求 ↔ 部署通路"的断点，静态门禁与运行时巡检都看不见，只有对着读三份文件才有结果。

var (
	composeVarRe = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(:?[-?])?`)
	envKeyRe     = regexp.MustCompile(`(?m)^#?\s*([A-Z][A-Z0-9_]*)=`)
	// 改名之后环境变量走 envname.Getenv("CITEBASE_X", "AIWIKI_X") 双读，
	// 所以判据要认两种调用形状：只匹配 os.Getenv 的话，所有 AIWIKI_* 都会被误报成"没人读"。
	getenvCallRe = regexp.MustCompile(`(?:os|envname)\.Getenv\([^)]*\)`)
	quotedNameRe = regexp.MustCompile(`"([A-Za-z0-9_]+)"`)
)

// envNamesRead 从一行源码里取出所有被 Getenv 调用引用的变量名。
func envNamesRead(line string) []string {
	loc := getenvCallRe.FindStringIndex(line)
	if loc == nil {
		return nil
	}
	var names []string
	for _, m := range quotedNameRe.FindAllStringSubmatch(line[loc[0]:loc[1]], -1) {
		names = append(names, m[1])
	}
	return names
}

// requiredPassthrough：代码读它、文档要求部署者设置，所以 compose 必须把它传进容器。
var requiredPassthrough = []string{
	"AIWIKI_DB_DSN",
	"AIWIKI_JWT_SECRET",
	"AIWIKI_ENCRYPT_KEY",
	"AIWIKI_ENV",
}

// isBrandEnv：两种前缀都算部署侧的"品牌环境变量"，出现在部署文件里就必须有代码读它。
// 改名走的是双读（CITEBASE_* 优先、AIWIKI_* 兜底），所以判据只认旧名会漏掉新名那一半。
func isBrandEnv(name string) bool {
	return strings.HasPrefix(name, "AIWIKI_") || strings.HasPrefix(name, "CITEBASE_")
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatalf("读 %s: %v", rel, err)
	}
	return string(data)
}

func TestComposeEnvContract(t *testing.T) {
	compose := readRepoFile(t, "docker-compose.yml")
	example := readRepoFile(t, ".env.example")

	used := map[string]bool{}
	hasDefault := map[string]bool{}
	for _, m := range composeVarRe.FindAllStringSubmatch(compose, -1) {
		used[m[1]] = true
		if m[2] != "" { // ${VAR:-默认} / ${VAR:?报错} 都算"不写也能起"
			hasDefault[m[1]] = true
		}
	}

	offered := map[string]bool{}
	for _, m := range envKeyRe.FindAllStringSubmatch(example, -1) {
		offered[m[1]] = true
	}

	// 1) compose 用了、模板没提供、也没有默认值 → 部署者从 .env.example 复制出来的 .env 起不来
	for v := range used {
		if !offered[v] && !hasDefault[v] {
			t.Errorf("docker-compose 需要 %s，但 .env.example 既没有这一项也没有默认值", v)
		}
	}

	// 2) 模板里有、compose 不消费 → 死文档（比缺项更容易误导，因为它读起来像"这个开关有用"）
	for v := range offered {
		if !used[v] {
			t.Errorf(".env.example 提供 %s，但 docker-compose.yml 从不引用它", v)
		}
	}

	// 3) 文档要求的生产项必须真的能传进容器
	appBlock := compose[strings.Index(compose, "services:"):]
	for _, name := range requiredPassthrough {
		if !strings.Contains(appBlock, name+":") {
			t.Errorf("compose 的 environment 里没有把 %s 传进应用 —— 文档要求它而部署通路没有，等于这项从来没生效", name)
		}
	}
}

func TestEnvNamesMatchCode(t *testing.T) {
	// .env.example / compose 里出现的 AIWIKI_* 名字，代码必须真的读它。
	// 读集来自**全仓 .go**，不是手挑的几个文件 —— 手挑那份一开始就漏了 internal/log 与
	// internal/repository/dbcache，于是 AIWIKI_LOG_LEVEL 被报成"没人读"。
	// 拼错的键不会报错，只会被 viper 与 Getenv 一起忽略：又是一条"以为设了"的配置。
	read := map[string]bool{}
	err := filepath.Walk(filepath.Join("..", ".."), func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			switch filepath.Base(p) {
			case ".git", "node_modules", "dist", "data":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		for _, line := range strings.Split(string(data), "\n") {
			for _, name := range envNamesRead(line) {
				read[name] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("遍历源码失败: %v", err)
	}
	if len(read) < 10 {
		t.Fatalf("只从源码里认出 %d 个 os.Getenv 名字，判据本身不可信", len(read))
	}

	var missing []string
	for _, src := range []string{".env.example", "docker-compose.yml"} {
		for _, m := range envKeyRe.FindAllStringSubmatch(readRepoFile(t, src), -1) {
			name := m[1]
			if isBrandEnv(name) && !read[name] {
				missing = append(missing, src+": "+name)
			}
		}
		for _, m := range composeVarRe.FindAllStringSubmatch(readRepoFile(t, src), -1) {
			name := m[1]
			if isBrandEnv(name) && !read[name] {
				missing = append(missing, src+": "+name)
			}
		}
	}
	sort.Strings(missing)
	seen := map[string]bool{}
	var out []string
	for _, m := range missing {
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	if len(out) > 0 {
		t.Errorf("这些 AIWIKI_* 变量在部署侧文件里出现，但全仓 .go 里没有任何 os.Getenv 读它（要么补读、要么删掉这一项）:\n  %s",
			strings.Join(out, "\n  "))
	}
}
