package api_leak_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestAPIPaginationGoesThroughParsePagination 拦住"手工解析 page_size"这一族。
//
// utils.ParsePagination 是唯一的钳制点（MaxPageSize=200），而 internal/api 下实测
// 有 17 处绕开它自己 strconv.Atoi / fmt.Sscanf —— 后果不是返回错页，而是
// ?page_size=1000000 一路透传到 repository 变成一次全表扫描（api→service→repo 三层都不钳制）。
// 通知列表那条链路上早就修过这个问题（注释写在 pagination.go 里），
// 但只要解析点还散在各处，修过的只是被报过的那几页。
//
// 判据只看"这一行自己读 page_size 查询参数、且没走 ParsePagination"。
// 需要例外的必须写理由，比如 memory.go 自带 100 的上限 —— 那不是漏洞，
// 但也不该因为"反正有上限"就把它升格成第二种写法，所以留在例外里而不是搬进共用函数。
func TestAPIPaginationGoesThroughParsePagination(t *testing.T) {
	allow := map[string]string{
		// 自带 100 的上限，比 MaxPageSize 更严；不是绕过钳制，是另一档既有约定。
		"../api/ai/memory.go": "getPageSize() 自带 100 上限，比 MaxPageSize 更严",
	}

	var files []string
	root := ".."
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case "node_modules", ".git", "web", "docs", "data":
				return filepath.SkipDir
			}
			return nil
		}
		rel := filepath.ToSlash(p)
		if strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go") && strings.Contains(rel, "/api/") {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}
	if len(files) < 40 {
		t.Fatalf("只扫到 %d 个 API 源文件，明显低于本仓规模 —— 扫描退化会让这条断言安静地全绿", len(files))
	}

	var hits []string
	for _, f := range files {
		if _, ok := allow[f]; ok {
			continue
		}
		// 扫描用的是相对路径（../api/...），读文件时相对当前目录同样成立。
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("读 %s 失败: %v", f, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if !strings.Contains(line, `"page_size"`) {
				continue
			}
			// 只是把 page_size 当响应字段名往外写的，不算解析。
			if !strings.Contains(line, "strconv.Atoi") && !strings.Contains(line, "Sscanf") && !strings.Contains(line, "Query(") {
				continue
			}
			if strings.Contains(line, "ParsePagination") {
				continue
			}
			hits = append(hits, f+":"+itoa(i+1)+"  "+strings.TrimSpace(line))
		}
	}

	if len(hits) > 0 {
		sort.Strings(hits)
		t.Errorf("有 %d 处自己解析 page_size，绕过了 utils.ParsePagination 的 MaxPageSize 钳制：\n%s\n改成 utils.ParsePagination(c, 默认值)，或（确需另一档上限时）进 allow 并写理由",
			len(hits), strings.Join(hits, "\n"))
	}
}
