package ai

import (
	"context"
	logx "github.com/listen808/citebase/internal/log"
	"strings"
	"sync"

	"github.com/listen808/citebase/internal/ai/analyzer"
	"github.com/listen808/citebase/internal/ai/scheduler"
	"github.com/robfig/cron/v3"
)

type CronScheduler struct {
	cron     *cron.Cron
	sched    *scheduler.Scheduler
	mu       sync.Mutex
	started  bool
	schedule string
	taskType string // P2-2: full/incremental/space_only/quick
	spaceIDs []int  // P2-2: space_only 时的目标空间

	// P0-3: 自动化评测任务。此前它只调度知识分析，评测一次都没自动跑过。
	evalSchedule string
	evalFn       func(context.Context)
}

func NewCronScheduler(sched *scheduler.Scheduler, schedule string) *CronScheduler {
	if schedule == "" {
		schedule = defaultAnalysisSchedule
	}
	return &CronScheduler{
		sched:    sched,
		schedule: schedule,
		taskType: "incremental",
	}
}

// defaultAnalysisSchedule 每天 02:00 执行（5 字段表达式，与 configs 中的样例保持一致）
const defaultAnalysisSchedule = "0 2 * * *"

// newCronFor P0 修复：cron.New(cron.WithSeconds()) 会把表达式按 6 字段（含秒）解析，
// 而默认/配置里的 "0 2 * * *" 是 5 字段 → AddFunc 直接报错，且错误被忽略导致定时分析静默不跑。
// 这里按字段数量选择解析器：5 字段用标准解析器，6 字段（带秒）用 WithSeconds。
func newCronFor(schedule string) *cron.Cron {
	if len(strings.Fields(strings.TrimSpace(schedule))) >= 6 {
		return cron.New(cron.WithSeconds())
	}
	return cron.New()
}

// SetScheduleScope P2-2: 设置定时分析的范围
func (cs *CronScheduler) SetScheduleScope(taskType string, spaceIDs []int) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if taskType != "" {
		cs.taskType = taskType
	}
	cs.spaceIDs = spaceIDs
}

func (cs *CronScheduler) Start() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.started {
		return nil
	}

	cs.cron = newCronFor(cs.schedule)

	_, err := cs.cron.AddFunc(cs.schedule, func() {
		cs.triggerAnalysis()
	})
	if err != nil {
		logx.Errorf("[CronScheduler] Invalid schedule %q, scheduled analysis NOT started: %v", cs.schedule, err)
		cs.cron = nil
		return err
	}

	cs.addEvalJob()

	cs.cron.Start()
	cs.started = true
	logx.Infof("[CronScheduler] Started with schedule: %s, task_type: %s, spaceIDs: %v", cs.schedule, cs.taskType, cs.spaceIDs)
	return nil
}

// SetEvalJob P0-3: 注册自动化评测任务（需在 Start 之前调用）
func (cs *CronScheduler) SetEvalJob(schedule string, fn func(context.Context)) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.evalSchedule = schedule
	cs.evalFn = fn
}

// addEvalJob 把评测任务挂到当前 cron 实例上，调用方需持有锁
func (cs *CronScheduler) addEvalJob() {
	if cs.evalFn == nil || cs.cron == nil {
		return
	}
	if cs.evalSchedule == "" {
		cs.evalSchedule = defaultEvalSchedule
	}
	if _, err := cs.cron.AddFunc(cs.evalSchedule, func() {
		cs.evalFn(context.Background())
	}); err != nil {
		logx.Errorf("[CronScheduler] Invalid eval schedule %q, auto evaluation NOT scheduled: %v", cs.evalSchedule, err)
		return
	}
	logx.Infof("[CronScheduler] Auto evaluation scheduled: %s", cs.evalSchedule)
}

// triggerAnalysis P2-2: 根据配置的 taskType 触发对应类型的分析
func (cs *CronScheduler) triggerAnalysis() {
	ctx := context.Background()
	var (
		task *analyzer.AnalysisTask
		err  error
	)
	cs.mu.Lock()
	taskType := cs.taskType
	spaceIDs := cs.spaceIDs
	cs.mu.Unlock()

	logx.Infof("[CronScheduler] Triggering scheduled analysis: type=%s, spaces=%v", taskType, spaceIDs)

	switch taskType {
	case "full":
		task, err = cs.sched.RunFullAnalysis(ctx)
	case "quick":
		task, err = cs.sched.RunQuickAnalysis(ctx)
	case "space_only":
		if len(spaceIDs) > 0 {
			task, err = cs.sched.RunSpaceAnalysis(ctx, spaceIDs)
		} else {
			logx.Infof("[CronScheduler] space_only type but no spaceIDs configured, falling back to incremental")
			task, err = cs.sched.RunIncrementalAnalysis(ctx)
		}
	default:
		task, err = cs.sched.RunIncrementalAnalysis(ctx)
	}
	if err != nil {
		logx.Infof("[CronScheduler] Scheduled analysis error: %v", err)
		return
	}
	logx.Infof("[CronScheduler] Scheduled analysis started: task %s", task.ID)
}

func (cs *CronScheduler) Stop() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if !cs.started {
		return
	}
	ctx := cs.cron.Stop()
	<-ctx.Done()
	cs.started = false
	logx.Infof("[CronScheduler] Stopped")
}

func (cs *CronScheduler) UpdateSchedule(schedule string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.started {
		ctx := cs.cron.Stop()
		<-ctx.Done()
	}

	cs.schedule = schedule
	cs.cron = newCronFor(cs.schedule)

	_, err := cs.cron.AddFunc(cs.schedule, func() {
		cs.triggerAnalysis()
	})
	if err != nil {
		// P0：错误必须上报，且不能留下“已启动但无任务”的状态
		logx.Errorf("[CronScheduler] Invalid schedule %q, analysis schedule NOT updated: %v", schedule, err)
		cs.cron = nil
		cs.started = false
		return err
	}

	if cs.started {
		cs.addEvalJob()
		cs.cron.Start()
	}
	logx.Infof("[CronScheduler] Schedule updated to: %s", schedule)
	return nil
}

func (cs *CronScheduler) GetSchedule() string {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.schedule
}

func (cs *CronScheduler) IsRunning() bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.started
}
