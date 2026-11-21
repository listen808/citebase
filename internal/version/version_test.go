package version

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// readAt 从仓库根按相对路径读文件。测试跑在包目录里，所以要往上跳两级。
func readAt(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatalf("读 %s: %v（这个门禁的前提是仓库结构没挪动，挪了就要同步改路径）", rel, err)
	}
	return string(data)
}

// TestVersionMatchesPackageJSON 钉住"前端与后端同一个版本号"。
// 迁移之前这两处各写一份 2.0.0，改一处不会有任何东西提醒另一处。
func TestVersionMatchesPackageJSON(t *testing.T) {
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(readAt(t, "web/package.json")), &pkg); err != nil {
		t.Fatalf("解析 web/package.json: %v", err)
	}
	if pkg.Version != Version {
		t.Errorf("web/package.json 是 %q 而 internal/version 是 %q —— 只准改一处", pkg.Version, Version)
	}
}

// TestVersionMatchesDataSQL 钉住 databases/data.sql 的 system_version 种子行。
// 那一行是给"手工建库"的人用的，与 db.go 的 seedDefaults 是同一件事的两个副本。
func TestVersionMatchesDataSQL(t *testing.T) {
	re := regexp.MustCompile(`\('system_version',\s*'([^']+)'`)
	m := re.FindStringSubmatch(readAt(t, "databases/data.sql"))
	if m == nil {
		t.Fatal("databases/data.sql 里找不到 system_version 种子行")
	}
	if m[1] != Version {
		t.Errorf("databases/data.sql 的 system_version 是 %q 而 internal/version 是 %q", m[1], Version)
	}
}

// TestSwaggerInfoVersion 钉住生成文档里的版本号。
// docs/swagger 由 swag 从 main.go 的注解生成，注解里那份曾经写的是 "2.0"（少一段），
// 而 swagger.json 是生成物 —— 所以要改的是注解，测试只负责发现分叉。
func TestSwaggerInfoVersion(t *testing.T) {
	var doc struct {
		Info struct {
			Version string `json:"version"`
		} `json:"info"`
	}
	if err := json.Unmarshal([]byte(readAt(t, "docs/swagger/swagger.json")), &doc); err != nil {
		t.Fatalf("解析 docs/swagger/swagger.json: %v", err)
	}
	if doc.Info.Version != Version {
		t.Errorf("docs/swagger 的 info.version 是 %q 而 internal/version 是 %q —— 去改 app/server/main.go 的 @version 注解再 make swagger",
			doc.Info.Version, Version)
	}
}

// TestNoStrayVersionLiteral 是本节真正的防漂移装置：
// TestNoStrayVersionLiteral 是本节真正的防漂移装置：
// 除事实源本文件之外，Go 源码里不许再出现 "2.0.0" 这个字面量。
// 唯一豁免是 internal/version/version.go 自己；出现第二条豁免就说明有人在抄第四份。
func TestNoStrayVersionLiteral(t *testing.T) {
	offenders := scanLiteral(t, `"2.0.0"`, `'2.0.0'`)
	if len(offenders) > 0 {
		t.Errorf("又有人抄了一份版本号字面量（请用 internal/version.Version）:\n  %s",
			strings.Join(offenders, "\n  "))
	}
}

// scanLiteral 遍历非测试的 .go 文件，按行找命中任一 needle 的位置。
// 跳过 _test.go：测试里出现 "2.0.0" 多数是构造用例数据，不是又抄了一份事实源。
func scanLiteral(t *testing.T, needles ...string) []string {
	t.Helper()
	// 豁免只有两类，每条都要写理由；出现第三条说明有人在抄第四份。
	exempt := map[string]string{
		// 事实源本身。
		"internal/version/version.go": "版本号在这里定义",
		// swag 的生成物：源头是 app/server/main.go 的 @version 注解，
		// 而那条注解由 TestSwaggerInfoVersion 逐字比对，所以这里是派生副本而不是第二处事实。
		"docs/swagger/docs.go": "swag init 生成，由 TestSwaggerInfoVersion 兜住源头",
	}

	var out []string
	root := filepath.Join("..", "..") // 测试的工作目录是包目录，仓库根在两级之上
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
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
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		rel := strings.TrimPrefix(filepath.ToSlash(p), filepath.ToSlash(root)+"/")
		if _, ok := exempt[rel]; ok {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		for i, line := range strings.Split(string(data), "\n") {
			for _, needle := range needles {
				if strings.Contains(line, needle) {
					out = append(out, rel+":"+strconv.Itoa(i+1)+" "+strings.TrimSpace(line))
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("遍历源码失败: %v", err)
	}
	return out
}

// TestFrontendDoesNotHardcodeVersion 钉住链路的最后一环：
// web/src/utils/core/app-info.ts 必须由构建注入取版本，不许再出现 vX.Y.Z 字面量。
// 前端那侧另有 app-info.test.ts 断言"注入值真等于 package.json"——
// 两条合起来才是"版本号只有一个来源"，任何一条单独看都可能是假绿。
func TestFrontendDoesNotHardcodeVersion(t *testing.T) {
	src := readAt(t, "web/src/utils/core/app-info.ts")
	// 反引号不能出现在 Go 的原始字符串里，所以用 \x60。
	re := regexp.MustCompile(`["'\x60]v?\d+\.\d+\.\d+["'\x60]`)
	if m := re.FindString(src); m != "" {
		t.Errorf("app-info.ts 里又出现了版本字面量 %q —— 应写成 v${__APP_VERSION__}（由 vite define 注入）", m)
	}
}
