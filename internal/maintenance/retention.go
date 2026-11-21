// Package maintenance 放"没人调用就等于没在做"的周期性数据清理。
package maintenance

import (
	"context"
	"sync"
	"time"

	logx "github.com/listen808/citebase/internal/log"
	repos "github.com/listen808/citebase/internal/repository"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
)

const (
	// sweepInterval 一天扫一次足够：这三样都不是会立刻撑爆表的东西，
	// 而每次扫描都是全表 DELETE 条件的匹配，扫得越勤对写越多的大库越不友好。
	sweepInterval = 24 * time.Hour
	// trashRetention 与 deliveryRetention 取 30 天，跟仓里既有两档对齐：
	// 日志文件保留 30 天（internal/log），向量同步任务完成后 7 天清（vector_sync.go）。
	// 回收站与投递记录属于"用户可能回来查"的那一类，所以取 30 天而不是 7 天。
	trashRetention    = 30 * 24 * time.Hour
	deliveryRetention = 30 * 24 * time.Hour
)

var (
	mu      sync.Mutex
	running bool
	stopFn  context.CancelFunc
)

// Start 启动周期清理（幂等）。
//
// 加这个之前的状态更准确的说法是"这三个功能不存在"：
//   - DocumentTrashRepository.DeleteExpired —— 0 调用方。而 MoveToTrash 每次删目录
//     都会给每个后代写一行回收站记录，永久删除时原先只清顶层那一行，
//     于是 aw_document_trash 只增不减，回收站列表页越来越慢。
//   - WebhookRepository.DeleteExpiredDeliveries —— 0 调用方。每次投递都写一行，
//     配了 webhook 的空间跑得越久这张表越大。
//   - AIMemoryRepository.DeleteExpired —— 0 调用方，而写入时是按 expires_at 打过期的，
//     于是"过期记忆"只是被查询条件过滤掉，行本身永远留在表里。
//
// 三条都写好了 SQL，只差没人跑 —— 静态看代码时它们是"存在的能力"，
// 只有按调用方去数才会发现是死码。这里就是把它们接上生命周期。
func Start() {
	mu.Lock()
	defer mu.Unlock()
	if running {
		return
	}
	if repos.DB == nil {
		logx.Warnf("[maintenance] 数据库尚未就绪，跳过周期清理（回收站/投递记录/过期记忆不会自动回收）")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	stopFn = cancel
	running = true

	go func() {
		// 启动后先等一轮再扫：进程刚起来时数据库连接池和别的服务都在预热，
		// 一上来就跑三个全表 DELETE 条件是最差时机。
		timer := time.NewTimer(sweepInterval)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				SweepOnce()
				timer.Reset(sweepInterval)
			}
		}
	}()
	logx.Infof("[maintenance] 周期清理已启动（每 %v 扫一次：回收站 %v 前、投递记录 %v 前、过期记忆）",
		sweepInterval, trashRetention, deliveryRetention)
}

// Stop 停掉周期清理，供优雅关停调用。
func Stop() {
	mu.Lock()
	defer mu.Unlock()
	if !running {
		return
	}
	stopFn()
	stopFn = nil
	running = false
	logx.Info("[maintenance] 周期清理已停止")
}

// SweepOnce 立刻跑一轮三项清理，导出给测试与"手动整理"这类入口用。
// 每一项独立 recover：一张表清失败不该让另外两张也停下来。
func SweepOnce() map[string]int64 {
	out := map[string]int64{}
	out["trash"] = sweepTrash()
	out["deliveries"] = sweepDeliveries()
	out["memories"] = sweepMemories()
	return out
}

func sweepTrash() (removed int64) {
	defer func() {
		if r := recover(); r != nil {
			logx.Errorf("[maintenance] 清理过期回收站记录时 panic: %v", r)
		}
	}()
	before := time.Now().Add(-trashRetention)
	if err := workspacerepo.NewDocumentTrashRepository().DeleteExpired(before); err != nil {
		logx.Warnf("[maintenance] 清理过期回收站记录失败: %v", err)
		return 0
	}
	return 1
}

func sweepDeliveries() (removed int64) {
	defer func() {
		if r := recover(); r != nil {
			logx.Errorf("[maintenance] 清理 webhook 投递记录时 panic: %v", r)
		}
	}()
	before := time.Now().Add(-deliveryRetention).Unix()
	if err := workspacerepo.NewWebhookRepository().DeleteExpiredDeliveries(before); err != nil {
		logx.Warnf("[maintenance] 清理 webhook 投递记录失败: %v", err)
		return 0
	}
	return 1
}

func sweepMemories() (removed int64) {
	defer func() {
		if r := recover(); r != nil {
			logx.Errorf("[maintenance] 清理过期 AI 记忆时 panic: %v", r)
		}
	}()
	// 这一条是三者里唯一返回行数的（其余两个仓储只回 error），
	// 所以日志里的数量只有这一项给得出来 —— 不是漏记，是仓储接口没回。
	n, err := airepo.NewAIMemoryRepository().DeleteExpired()
	if err != nil {
		logx.Warnf("[maintenance] 清理过期 AI 记忆失败: %v", err)
		return 0
	}
	if n > 0 {
		logx.Infof("[maintenance] 清理过期 AI 记忆 %d 条", n)
	}
	return n
}
