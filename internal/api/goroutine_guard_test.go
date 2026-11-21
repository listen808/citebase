package api_leak_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestBackgroundGoroutinesAreGuarded 是给"请求点起来的后台协程没有 recover"这一类立的只降不升门禁。
//
// gin 的 Recovery 只护得住请求协程自己。`go s.triggerIndex(doc)` 换了一个协程跑，
// 那里 panic 没有任何东西接得住 —— 进程直接退出，用户看到的现象是"服务毫无征兆重启"，
// 而那条真正说明原因的 panic 日志经常和进程一起没了。
// internal/service + internal/api 实测 49 处这种 fire-and-forget（写审计日志、刷索引、
// 发通知、解析 @ 提及、投递 webhook），其中 35 处已换成 safego.Go；
// 剩下的记录在下面的基线里，**只能减少不能增加**。
//
// 为什么做棘轮而不是一次修完：剩下的这些形态不一（有闭包、有循环、有自己的退出语义），
// 逐个包裹需要在每处判断"崩了之后该继续还是该停"，那是逐处的设计判断，
// 不该由一个正则批量决定。写进基线是为了让这笔债看得见、且不会再长。
const unguardedGoroutineBaseline = 14

func TestBackgroundGoroutinesAreGuarded(t *testing.T) {
	var found []string
	for _, root := range []string{"../service", "../api"} {
		err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			rel := filepath.ToSlash(p)
			if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			lines := strings.Split(string(data), "\n")
			for i, line := range lines {
				trimmed := strings.TrimSpace(line)
				isGo := strings.HasPrefix(trimmed, "go func") ||
					(strings.HasPrefix(trimmed, "go ") && !strings.HasPrefix(trimmed, "go func()") &&
						strings.Contains(trimmed, "("))
				if !isGo {
					continue
				}
				// recover 在窗口内出现即算已兜住（要么本来写了，要么是注释里说明了去处）。
				hi := i + 14
				if hi > len(lines) {
					hi = len(lines)
				}
				window := strings.Join(lines[i:hi], "\n")
				if strings.Contains(window, "recover()") {
					continue
				}
				// safego.Go 本身不算违例（它内部就有 recover），但这一行以 go 开头就不是它。
				found = append(found, rel+":"+itoa(i+1)+"  "+trimmed)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("扫描 %s 失败: %v", root, err)
		}
	}

	sort.Strings(found)
	t.Logf("未兜住的后台协程：当前 %d 处 / 基线 %d 处", len(found), unguardedGoroutineBaseline)
	if len(found) > unguardedGoroutineBaseline {
		t.Errorf("后台协程无 recover 的数量从基线 %d 涨到了 %d：\n  %s\n新起的 fire-and-forget 协程请用 safego.Go(name, func(){…})，别依赖 gin 的 Recovery（它只管请求协程）",
			unguardedGoroutineBaseline, len(found), strings.Join(found, "\n  "))
	}
	if len(found) < unguardedGoroutineBaseline {
		t.Logf("已降到 %d 处（低于基线 %d），可以把 unguardedGoroutineBaseline 一起降下来",
			len(found), unguardedGoroutineBaseline)
	}
}
