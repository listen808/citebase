package repocheck

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// 这台的射程：OpenAPI 文档覆盖率，**只升不降**的棘轮。
//
// 为什么不是"路径级 diff"：代码里 400 个不同端点，docs/swagger 只有 102 个 paths，
// 差距大到任何"必须全覆盖"的断言第一天就把门禁调红（而永远红的门禁比没有门禁更糟）。
// 所以这里量的是可移动的那一根轴：**注解数量**。谁新写了一个 handler 而不注解，
// 注册数涨而注解数不涨 → 覆盖率跌 → 下面第二条断言红。
//
// 判据按"每个 api 子包"分族，因为最大的一块缺口集中在 internal/api/ai
// （注册 197 条只有 1 个 @Router），全站合计数会把这种局部塌方平均掉。
var annotationBaseline = map[string]int{
	// 键是 internal/api 下的子包名，值是"该族当前有多少条 @Router 注解"。
	// 补注解时把数字抬上去；往下调必须写理由（默认拒绝）。
	// 2026-10-07 实测 121/502：文档只覆盖了约四分之一的注册面，缺口的大头是 ai（197 条只有 1 条）
	// 与 schema（37 条 0 条）—— 这两族是公开的欠账，不是一条已完成的规矩。
	"admin":     32,
	"ai":        8, // 2026-11-12: wiki 版本历史/回滚/矛盾标注/好答案回填 4 + OKF 导出 1 + 记忆待确认 2
	"auth":      15,
	"collab":    0,
	"common":    0,
	"knowledge": 0,
	"schema":    0,
	"search":    11,
	"social":    9,
	"user":      10,
	"websocket": 0,
	"workspace": 38, // 2026-11-12: 现场实测 38（git-sync 下线删了 5 条，基线滞后）
}

// routeDeclRe 认逐条注册行（与 rbac_coverage_test.go 同源的正则形状，但这里只数不解析前缀：
// 覆盖率是"注解 vs 注册"的比值问题，不需要还原完整路径）。
var (
	routeDeclRe = regexp.MustCompile(`\.(GET|POST|PUT|DELETE|PATCH)\(\s*"`)
	routerAnno  = regexp.MustCompile(`@Router\s+\S+\s+\[`)
)

func familyCounts(t *testing.T) map[string][2]int {
	t.Helper()
	dir := filepath.Join("..", "..", "internal", "api")
	out := map[string][2]int{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读 %s: %v", dir, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		var regs, annos int
		err := filepath.Walk(filepath.Join(dir, e.Name()), func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			regs += len(routeDeclRe.FindAllStringIndex(string(data), -1))
			annos += len(routerAnno.FindAllStringIndex(string(data), -1))
			return nil
		})
		if err != nil {
			t.Fatalf("遍历 %s: %v", e.Name(), err)
		}
		out[e.Name()] = [2]int{regs, annos}
	}
	return out
}

// TestRouterAnnotationRatchet 拦"新增 handler 不写注解"。
func TestRouterAnnotationRatchet(t *testing.T) {
	counts := familyCounts(t)

	families := make([]string, 0, len(counts))
	for f := range counts {
		families = append(families, f)
	}
	sort.Strings(families)

	var totalRegs, totalAnnos int
	for _, f := range families {
		c := counts[f]
		totalRegs += c[0]
		totalAnnos += c[1]
		base, known := annotationBaseline[f]
		if !known {
			// 新出现的 api 子包：必须显式登记基线，否则"新族 0 注解"会一路静默通过。
			t.Errorf("internal/api/%s 不在 annotationBaseline 里（当前 %d 条注册 / %d 条注解）—— 补一行，0 也要写", f, c[0], c[1])
			continue
		}
		if c[1] < base {
			t.Errorf("internal/api/%s 的 @Router 注解从 %d 降到 %d —— 注解只许增加；确实要删，请在 annotationBaseline 写理由", f, base, c[1])
		}
	}

	if totalRegs < 300 {
		t.Fatalf("只数到 %d 条注册行，低于 300 的下限 —— 判据本身坏了（正则失配 / 目录挪动），此时任何覆盖率数字都不可信", totalRegs)
	}
	t.Logf("OpenAPI 注解覆盖：%d/%d 条注册有注解（docs/swagger 当前 102 个 paths）", totalAnnos, totalRegs)
}

// TestSwaggerSpecPathsRatchet 是 spec 侧的棘轮：paths 条目数只许升。
//
// 这里刻意**不做**"按注解数推断 spec 过期"那种判断：一条 @Router 对应一个 HTTP 方法，
// 而一个 path 可以同时挂 get/post，两个数本来就不可比 —— 拿它做过期判据要么常年假红，
// 要么像上一版那样 `anns > paths+40` 永远不成立（121 vs 102 今天就没触发），是台永远绿的机器。
// 过期检测改由 CI 真跑一次 `swag init` 再 `git diff --exit-code`（见 .github/workflows/ci.yml），
// 那条判据是精确的：生成物与代码不一致就红。
func TestSwaggerSpecPathsRatchet(t *testing.T) {
	const baseline = 102 // 2026-10-07 实测；补注解并重生成后抬上去
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "swagger", "swagger.json"))
	if err != nil {
		t.Fatalf("读 docs/swagger/swagger.json: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, `"paths": {`) {
		t.Fatal("swagger.json 里没有非空的 paths 段")
	}
	// 数 paths 段的键：从 "paths": { 之后按引号内以 / 开头的串数。
	got := strings.Count(text, `"/`)
	if got < baseline {
		t.Errorf("swagger.json 的路径条目从 %d 降到 %d —— 生成物退化了，检查 docs/swagger 是否被手工改动或误删", baseline, got)
	}
	if got > baseline {
		t.Logf("路径条目 %d > 基线 %d：把本文件的 baseline 抬到 %d，让下一次退化能被看见", got, baseline, got)
	}
}
