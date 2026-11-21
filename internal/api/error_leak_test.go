package api_leak_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestNoInternalErrorInErrorResponse 是给"把内部错误原文返给客户端"这一类缺陷立的门禁。
//
// 起因：internal/api 下实测有 53 处把 err.Error() 拼进响应体。500 那一档泄的是
// GORM/驱动原文 —— 表名、列名、SQL 片段、约束名，偶尔还有连接信息。
// 这些不是"调试方便"，是攻击者的指纹材料：拿到一条 1054 Unknown column 就知道列名，
// 拿到一条 duplicate entry 就知道索引名，拿到 dial tcp 就知道内网拓扑。
// 内部细节该去日志（response.ServerErrorSafe / response.FailSafe 都这么干）。
//
// 判据只管 500 与 4xx 里的"服务侧失败"，不管参数校验：
// BadRequest(c, err.Error()) 这一族（实测 77 处）返回的是 gin binding 的错误，
// 那是给用户看的"哪个字段没填对"，属于产品反馈不是内部细节 ——
// 把它一起拦掉会让表单校验全部变成一句"参数错误"，是倒退。
// 所以这条测试拦的是"响应状态是服务端错误、且消息里拼了 err.Error()"。
func TestNoInternalErrorInErrorResponse(t *testing.T) {
	var files []string
	root := ".."
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// api_leak_test 自己所在的目录，以及非 API 层
			name := info.Name()
			if name == "node_modules" || name == ".git" || name == "web" || name == "docs" || name == "data" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") && strings.Contains(filepath.ToSlash(p), "/api/") {
			files = append(files, p)
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
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("读 %s 失败: %v", f, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if !strings.Contains(line, "err.Error()") {
				continue
			}
			if !isResponseBuilder(line) {
				continue
			}
			if !isServerSideStatus(line) {
				continue
			}
			hits = append(hits, filepath.ToSlash(f)+":"+itoa(i+1)+"  "+strings.TrimSpace(line))
		}
	}

	if len(hits) > 0 {
		sort.Strings(hits)
		t.Errorf("有 %d 处把内部错误原文拼进了服务端错误响应（泄漏表名/列名/SQL/连接信息）：\n%s\n改成 response.ServerErrorSafe(c, err) 或 response.FailSafe(c, status, \"给用户的消息\", err)",
			len(hits), strings.Join(hits, "\n"))
	}
}

func isResponseBuilder(line string) bool {
	for _, p := range []string{
		"response.Fail(", "response.Error(", "response.ServerError(",
		"JSON(", "c.JSON(",
	} {
		if strings.Contains(line, p) {
			return true
		}
	}
	return false
}

func isServerSideStatus(line string) bool {
	if strings.Contains(line, "http.StatusInternalServerError") || strings.Contains(line, "500,") || strings.Contains(line, "StatusServiceUnavailable") {
		return true
	}
	// response.ServerError 这一条本身就固定是 500。
	return strings.Contains(line, "response.ServerError(")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
