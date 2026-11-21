package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"sync"
	"time"

	"github.com/listen808/citebase/internal/ai/analyzer"
	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/ai/wiki"
	"github.com/listen808/citebase/internal/ai/writer"
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	knowledgerepo "github.com/listen808/citebase/internal/repository/knowledge"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
)

type Scheduler struct {
	analyzer     *analyzer.Analyzer
	writer       *writer.SpaceWriter
	taskRepo     *knowledgerepo.AITaskRepository
	mappingRepo  *knowledgerepo.AIDocMappingRepository
	resultRepo   *knowledgerepo.AIAnalysisResultRepository
	wikiCompiler *wiki.Compiler // P1：分析完成后自动触发 LLM Wiki 编译
	tasks        map[string]*analyzer.AnalysisTask
	mu           sync.RWMutex
	aiSpaceName  string
	systemUserID int
	// P0：analysisMu/analysisRunning 保证同一时刻只有一个分析任务在跑。
	// analyzer/writer 是共享可变状态，并发跑两个任务会产生 concurrent map writes /
	// 结果互相覆盖，属于 runtime fatal 级别问题。
	analysisMu      sync.Mutex
	analysisRunning bool
	runningTaskID   string
	analysisStarted time.Time
}

// maxAnalysisSlot 单个分析任务最长占用槽位时间；超过则允许新任务抢占，
// 避免异常卡死的任务把调度器永久锁死。
const maxAnalysisSlot = 6 * time.Hour

// ErrAnalysisRunning 已有分析任务在运行时重复触发的错误
var ErrAnalysisRunning = fmt.Errorf("analysis already in progress, please wait for the current task to finish")

// tryBeginAnalysis 抢占唯一分析槽位；已被占用时返回 false。
func (s *Scheduler) tryBeginAnalysis(taskID string) bool {
	s.analysisMu.Lock()
	defer s.analysisMu.Unlock()
	if s.analysisRunning {
		if time.Since(s.analysisStarted) < maxAnalysisSlot {
			return false
		}
		logx.Warnf("[Scheduler] Analysis slot held by %s for over %v, new task %s takes over",
			s.runningTaskID, maxAnalysisSlot, taskID)
	}
	s.analysisRunning = true
	s.runningTaskID = taskID
	s.analysisStarted = time.Now()
	return true
}

// releaseAnalysis 释放唯一分析槽位（仅当确实由 taskID 持有时）。
func (s *Scheduler) releaseAnalysis(taskID string) {
	s.analysisMu.Lock()
	defer s.analysisMu.Unlock()
	if !s.analysisRunning || s.runningTaskID != taskID {
		return
	}
	s.analysisRunning = false
	s.runningTaskID = ""
}

// isTerminalStatus 任务是否已到达不再占用分析槽位的状态。
// Paused 也算：分析协程此时已经 return，槽位必须释放，否则 Resume/新任务会被永久卡住。
func isTerminalStatus(status string) bool {
	return status == analyzer.TaskStatusCompleted ||
		status == analyzer.TaskStatusFailed ||
		status == analyzer.TaskStatusPaused
}

func (s *Scheduler) TaskCount() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tasks)
}

func NewScheduler(primary, fast, reasoning provider.Provider, aiSpaceName string, systemUserID int, analyzerCfg *analyzer.AnalyzerConfig) *Scheduler {
	aiSpaceID := 0
	w := writer.NewSpaceWriter(aiSpaceName)
	if spaceID, err := w.EnsureAISpace(systemUserID); err == nil {
		aiSpaceID = spaceID
	} else {
		logx.Infof("[Scheduler] Failed to ensure AI space: %v", err)
	}

	s := &Scheduler{
		analyzer:     analyzer.NewAnalyzer(primary, fast, reasoning, aiSpaceID, analyzerCfg),
		writer:       w,
		taskRepo:     knowledgerepo.NewAITaskRepository(),
		mappingRepo:  knowledgerepo.NewAIDocMappingRepository(),
		resultRepo:   knowledgerepo.NewAIAnalysisResultRepository(),
		tasks:        make(map[string]*analyzer.AnalysisTask),
		aiSpaceName:  aiSpaceName,
		systemUserID: systemUserID,
	}
	if fast != nil {
		s.wikiCompiler = wiki.NewCompiler(fast)
	}

	s.loadRecentTasks()

	return s
}

func (s *Scheduler) loadRecentTasks() {
	tasks, _, err := s.taskRepo.FindAll(20, 0)
	if err != nil {
		logx.Infof("[Scheduler] Load recent tasks error: %v", err)
		return
	}
	for _, t := range tasks {
		task := &analyzer.AnalysisTask{
			ID:            t.TaskID,
			Type:          t.Type,
			Status:        t.Status,
			Progress:      t.Progress,
			TotalDocs:     t.TotalDocs,
			ProcessedDocs: t.ProcessedDocs,
			TokenUsage:    t.TokenUsage,
			CreatedAt:     t.CreatedAt.Unix(),
		}
		if t.StartedAt != nil {
			task.StartedAt = t.StartedAt.Unix()
		}
		if t.CompletedAt != nil {
			task.CompletedAt = t.CompletedAt.Unix()
		}
		if t.TargetSpaceIDs != nil {
			task.TargetSpaceIDs = []int(t.TargetSpaceIDs)
		}
		if t.Result != "" {
			var result analyzer.AnalysisResult
			if err := json.Unmarshal([]byte(t.Result), &result); err == nil {
				task.Result = &result
			}
		}
		if t.Checkpoint != "" {
			var checkpoint analyzer.AnalysisCheckpoint
			if err := json.Unmarshal([]byte(t.Checkpoint), &checkpoint); err == nil {
				task.Checkpoint = &checkpoint
			}
		}
		s.tasks[t.TaskID] = task
	}
	logx.Infof("[Scheduler] Loaded %d recent tasks from DB", len(tasks))
}

func (s *Scheduler) RunFullAnalysis(ctx context.Context) (*analyzer.AnalysisTask, error) {
	return s.runAnalysis(ctx, &analyzer.AnalysisTask{Type: analyzer.TaskTypeFull})
}

func (s *Scheduler) RunIncrementalAnalysis(ctx context.Context) (*analyzer.AnalysisTask, error) {
	return s.runAnalysis(ctx, &analyzer.AnalysisTask{Type: analyzer.TaskTypeIncremental})
}

func (s *Scheduler) RunSpaceAnalysis(ctx context.Context, spaceIDs []int) (*analyzer.AnalysisTask, error) {
	return s.runAnalysis(ctx, &analyzer.AnalysisTask{
		Type:           analyzer.TaskTypeSpaceOnly,
		TargetSpaceIDs: spaceIDs,
	})
}

func (s *Scheduler) RunQuickAnalysis(ctx context.Context) (*analyzer.AnalysisTask, error) {
	return s.runAnalysis(ctx, &analyzer.AnalysisTask{Type: analyzer.TaskTypeQuick})
}

func (s *Scheduler) RunSingleDocAnalysis(ctx context.Context, docID int) (*analyzer.AnalysisTask, error) {
	return s.runAnalysis(ctx, &analyzer.AnalysisTask{
		Type:        analyzer.TaskTypeSingleDoc,
		TargetDocID: docID,
	})
}

func (s *Scheduler) RunMultiDocAnalysis(ctx context.Context, docIDs []int) (*analyzer.AnalysisTask, error) {
	return s.runAnalysis(ctx, &analyzer.AnalysisTask{
		Type:         analyzer.TaskTypeMultiDoc,
		TargetDocIDs: docIDs,
	})
}

func (s *Scheduler) RunAnalysis(ctx context.Context, task *analyzer.AnalysisTask) (*analyzer.AnalysisTask, error) {
	return s.runAnalysis(ctx, task)
}

func (s *Scheduler) runAnalysis(ctx context.Context, task *analyzer.AnalysisTask) (*analyzer.AnalysisTask, error) {
	taskID := fmt.Sprintf("%s-%d", task.Type, time.Now().UnixMilli())
	task.ID = taskID

	// P0：串行化分析任务 —— analyzer 的 processor/writer 是共享可变状态，
	// 并发运行两个任务会造成 concurrent map writes（runtime fatal）与结果互相覆盖。
	if !s.tryBeginAnalysis(taskID) {
		return nil, ErrAnalysisRunning
	}

	task.Status = analyzer.TaskStatusPending
	task.CreatedAt = time.Now().Unix()

	dbTask := &knowledgemodel.AITask{
		TaskID:        taskID,
		Type:          task.Type,
		Status:        analyzer.TaskStatusPending,
		Progress:      0,
		TotalDocs:     0,
		ProcessedDocs: 0,
		TokenUsage:    0,
		TargetDocID:   task.TargetDocID,
	}
	if task.TargetSpaceIDs != nil {
		dbTask.TargetSpaceIDs = knowledgemodel.IntArray(task.TargetSpaceIDs)
	}
	if task.ModifiedSince > 0 {
		dbTask.CreatedAt = time.Unix(task.ModifiedSince, 0)
	}
	if err := s.taskRepo.Create(dbTask); err != nil {
		logx.Infof("[Scheduler] Create task record error: %v", err)
	}

	s.analyzer.SetProgressCallback(func(t *analyzer.AnalysisTask) {
		s.mu.Lock()
		s.tasks[t.ID] = t
		s.mu.Unlock()

		status := t.Status
		progress := t.Progress
		tokenUsage := t.TokenUsage
		errMsg := ""
		if t.Status == analyzer.TaskStatusFailed {
			errMsg = t.Message
		}

		if err := s.taskRepo.UpdateStatus(t.ID, status, progress, tokenUsage, errMsg); err != nil {
			logx.Infof("[Scheduler] Update task status error: %v", err)
		}

		if (t.Status == analyzer.TaskStatusPaused || t.Status == analyzer.TaskStatusFailed) && t.Result != nil {
			resultJSON, _ := json.Marshal(t.Result)
			if err := s.taskRepo.UpdateResult(t.ID, string(resultJSON), t.TotalDocs, t.ProcessedDocs); err != nil {
				logx.Infof("[Scheduler] Update task result error: %v", err)
			}
			s.saveAnalysisResults(t.Result)
		}

		if t.Status == analyzer.TaskStatusCompleted && t.Result != nil {
			resultJSON, _ := json.Marshal(t.Result)
			if err := s.taskRepo.UpdateResult(t.ID, string(resultJSON), t.TotalDocs, t.ProcessedDocs); err != nil {
				logx.Infof("[Scheduler] Update task result error: %v", err)
			}

			s.saveAnalysisResults(t.Result)

			if err := s.writer.WriteResult(t.Result, s.systemUserID); err != nil {
				logx.Infof("[Scheduler] Write result error: %v", err)
			} else {
				logx.Infof("[Scheduler] Result written to AI space successfully")
			}

			// P1 两步摄入：分析完成后把本次覆盖的文档编译进 LLM Wiki。
			// 异步执行：编译是每文档 1+N 次 LLM 调用，不能拖住状态回调。
			s.compileWikiAsync(t.Result)
		}

		// P0：任务结束/暂停后释放唯一分析槽位（放在回调最后，保证写库/写空间期间槽位仍被占用）
		if isTerminalStatus(t.Status) {
			s.releaseAnalysis(t.ID)
		}
	})

	s.mu.Lock()
	s.tasks[task.ID] = task
	s.mu.Unlock()

	if err := s.analyzer.Start(ctx, task); err != nil {
		s.releaseAnalysis(task.ID)
		return nil, err
	}

	return task, nil
}

func (s *Scheduler) saveAnalysisResults(result *analyzer.AnalysisResult) {
	if result == nil || len(result.Summaries) == 0 || s.resultRepo == nil {
		return
	}

	var dbResults []*knowledgemodel.AIAnalysisResult
	for _, summary := range result.Summaries {
		if summary.SourceSpaceID == 0 || summary.SourceDocID == 0 {
			continue
		}

		keyPointsJSON, _ := json.Marshal(summary.KeyPoints)
		tagsJSON, _ := json.Marshal(summary.Tags)
		qualityIssuesJSON := []byte("[]")
		entitiesJSON := []byte("{}")
		qualityScore := 0.0
		if summary.Quality != nil {
			qualityScore = summary.Quality.OverallScore
			qualityIssuesJSON, _ = json.Marshal(summary.Quality.Issues)
		}

		dbResults = append(dbResults, &knowledgemodel.AIAnalysisResult{
			SpaceID:       summary.SourceSpaceID,
			DocID:         summary.SourceDocID,
			Summary:       summary.Summary,
			KeyPoints:     string(keyPointsJSON),
			Category:      summary.Category,
			Tags:          string(tagsJSON),
			QualityScore:  qualityScore,
			QualityIssues: string(qualityIssuesJSON),
			Entities:      string(entitiesJSON),
			ContentHash:   summary.ContentHash,
		})
	}

	if len(dbResults) > 0 {
		if err := s.resultRepo.BatchSave(dbResults); err != nil {
			logx.Infof("[Scheduler] Batch save analysis results error: %v", err)
		} else {
			logx.Infof("[Scheduler] Saved %d analysis results to DB", len(dbResults))
		}
	}
}

func (s *Scheduler) GetTask(taskID string) (*analyzer.AnalysisTask, error) {
	s.mu.RLock()
	task, ok := s.tasks[taskID]
	s.mu.RUnlock()
	if ok {
		return task, nil
	}
	return s.analyzer.GetProgress(taskID)
}

// compileWikiAsync P1 两步摄入的调度侧入口：把分析结果里的文档编译进 LLM Wiki。
// 独立 goroutine + panic recover：编译失败只记日志，绝不影响分析任务本身。
func (s *Scheduler) compileWikiAsync(result *analyzer.AnalysisResult) {
	if s == nil || s.wikiCompiler == nil || result == nil || len(result.Summaries) == 0 {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logx.Infof("[Scheduler] Wiki compile panic recovered: %v", r)
			}
		}()
		s.compileWiki(result)
	}()
}

func (s *Scheduler) compileWiki(result *analyzer.AnalysisResult) {
	// 去重收集本次分析覆盖的 (space, doc)
	seen := make(map[int]bool)
	type docRef struct {
		spaceID int
		docID   int
	}
	var refs []docRef
	for _, sum := range result.Summaries {
		if sum.SourceSpaceID == 0 || sum.SourceDocID == 0 || seen[sum.SourceDocID] {
			continue
		}
		seen[sum.SourceDocID] = true
		refs = append(refs, docRef{spaceID: sum.SourceSpaceID, docID: sum.SourceDocID})
	}
	if len(refs) == 0 {
		return
	}

	docIDs := make([]int, 0, len(refs))
	for _, r := range refs {
		docIDs = append(docIDs, r.docID)
	}
	docs, err := analyzer.NewDocReader(0).ReadDocumentsByIDs(docIDs)
	if err != nil {
		logx.Infof("[Scheduler] Wiki compile: read documents failed: %v", err)
		return
	}
	contentByID := make(map[int]*analyzer.DocumentContent, len(docs))
	for _, d := range docs {
		if d != nil {
			contentByID[d.DocID] = d
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// 按空间分组编译：Plan/Generate 都是空间级操作
	candsBySpace := make(map[int][]wiki.Candidate)
	for _, r := range refs {
		d := contentByID[r.docID]
		if d == nil || d.Content == "" {
			continue
		}
		candsBySpace[r.spaceID] = append(candsBySpace[r.spaceID],
			wiki.AnalyzeDocumentToCandidates(ctx, s.wikiCompiler.Provider(), d.DocID, d.DocName, d.Content)...)
	}

	for spaceID, cands := range candsBySpace {
		plan, err := s.wikiCompiler.Plan(ctx, spaceID, cands)
		if err != nil {
			logx.Infof("[Scheduler] Wiki compile plan failed space=%d: %v", spaceID, err)
			continue
		}
		cost, err := s.wikiCompiler.Generate(ctx, spaceID, cands, 2)
		if err != nil {
			logx.Infof("[Scheduler] Wiki compile generate failed space=%d: %v", spaceID, err)
			continue
		}
		// P1 #3: 标记源文档为 raw 层不可变
		docRepo := workspacerepo.NewDocumentRepository()
		for _, r := range refs {
			if r.spaceID == spaceID {
				_ = docRepo.MarkImmutable(r.docID, true)
			}
		}
		var logLines []string
		for _, slug := range plan.Created {
			logLines = append(logLines, "新建页面：`"+slug+"`")
		}
		for _, slug := range plan.Updated {
			logLines = append(logLines, "更新页面：`"+slug+"`（并入溯源）")
		}
		if cost != nil && cost.TotalTokens > 0 {
			logLines = append(logLines, fmt.Sprintf("Token 消耗：prompt %d / completion %d / 共 %d",
				cost.PromptTokens, cost.CompletionTokens, cost.TotalTokens))
		}
		s.wikiCompiler.AppendCompileLog(spaceID, logLines)
		if err := s.wikiCompiler.RefreshOverview(ctx, spaceID); err != nil {
			logx.Infof("[Scheduler] Wiki overview refresh failed space=%d: %v", spaceID, err)
		}
		// 置信度治理
		for _, slug := range append(append([]string{}, plan.Created...), plan.Updated...) {
			s.wikiCompiler.CheckConfidence(ctx, spaceID, slug)
		}
		logx.Infof("[Scheduler] Wiki compile done space=%d created=%d updated=%d skipped=%d",
			spaceID, len(plan.Created), len(plan.Updated), plan.Skipped)
	}
}

func (s *Scheduler) ListTasks() []*analyzer.AnalysisTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tasks := make([]*analyzer.AnalysisTask, 0, len(s.tasks))
	for _, t := range s.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

func (s *Scheduler) PauseTask(taskID string) error {
	return s.analyzer.Pause(taskID)
}

func (s *Scheduler) ResumeTask(taskID string) error {
	// P0：恢复同样会重新起分析协程并使用共享的 analyzer/writer，
	// 因此必须占用同一个唯一槽位。
	if !s.tryBeginAnalysis(taskID) {
		return ErrAnalysisRunning
	}
	if err := s.analyzer.Resume(taskID); err != nil {
		s.releaseAnalysis(taskID)
		return err
	}
	return nil
}

func (s *Scheduler) UpdateReviewStatus(aiDocID int, status string) error {
	return s.mappingRepo.UpdateReviewStatus(aiDocID, status)
}

func (s *Scheduler) GetDocMapping(aiDocID int) (*knowledgemodel.AIDocMapping, error) {
	return s.mappingRepo.FindByAIDocID(aiDocID)
}

// GetAnalyzer 返回内部的分析器实例
func (s *Scheduler) GetAnalyzer() *analyzer.Analyzer {
	return s.analyzer
}
