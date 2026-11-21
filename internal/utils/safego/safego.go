// Package safego 给"HTTP 请求点起来的后台协程"补上唯一的兜底。
package safego

import (
	"runtime/debug"

	logx "github.com/listen808/citebase/internal/log"
)

// Go 启动一个带 recover 的协程。
//
// 为什么需要它：gin 的 Recovery 中间件只护得住**请求协程自己**。
// handler 里一句 `go s.triggerIndex(doc)` 就换一个协程跑，那个协程 panic 时
// 没有任何东西接得住 —— 进程直接没了，表现是"服务毫无征兆地重启"，
// 而日志里那条 panic 常常被一起带走。
// internal/service 与 internal/api 下实测有 49 处这种"发完就不管"的协程
// （写审计日志、刷索引、发通知、解析 @ 提及、投递 webhook），
// 它们的共同点是：**失败只该让那件后台小事没做成，不该让整站下线**。
//
// name 是必填的：panic 日志要能指回是哪一处，否则"某个协程崩了"这种
// 无主语错误在十几个 fire-and-forget 点里根本查不动。
func Go(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logx.Errorf("[safego:%s] 后台协程 panic，已接住（其余服务不受影响）: %v\n%s",
					name, r, debug.Stack())
			}
		}()
		fn()
	}()
}
