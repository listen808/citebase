package analyzer

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	knowledgerepo "github.com/listen808/citebase/internal/repository/knowledge"
	"github.com/listen808/citebase/internal/utils"
)

type Analyzer struct {
	processor   *BatchProcessor
	docReader   *DocReader
	mappingRepo *knowledgerepo.AIDocMappingRepository
	taskRepo    *knowledgerepo.AITaskRepository
	tasks       map[string]*AnalysisTask
	mu          sync.RWMutex
	onProgress  func(task *AnalysisTask)
	// progressMu 保护 onProgress 的读写：SetProgressCallback 由调度器调用，
	// 而分析协程会在进度更新时读取它（P0：无保护即构成 data race）。
	progressMu            sync.Mutex
	entityExtractor       *EntityExtractor
	qualityAssessor       *QualityAssessor
	entityResolver        *EntityResolver
	contradictionDetector *ContradictionDetector
	// 新增分析器
	relationExtractor     *RelationExtractor
	eventExtractor        *EventExtractor
	sentimentAnalyzer     *SentimentAnalyzer
	keywordExtractor      *KeywordExtractor
	knowledgeGraphBuilder *KnowledgeGraphBuilder
}

func NewAnalyzer(primary, fast, reasoning provider.Provider, aiSpaceID int, cfg *AnalyzerConfig) *Analyzer {
	processor := NewBatchProcessor(
		primary, fast, reasoning,
		cfg.BatchSize, cfg.MaxConcurrent, cfg.RateLimitRPM,
		cfg.DocMaxChars, cfg.ContextMaxChars,
	)
	docReader := NewDocReader(aiSpaceID)

	return &Analyzer{
		processor:             processor,
		docReader:             docReader,
		mappingRepo:           knowledgerepo.NewAIDocMappingRepository(),
		taskRepo:              knowledgerepo.NewAITaskRepository(),
		tasks:                 make(map[string]*AnalysisTask),
		onProgress:            nil,
		entityExtractor:       NewEntityExtractor(fast),
		qualityAssessor:       NewQualityAssessor(fast),
		entityResolver:        NewEntityResolver(fast),
		contradictionDetector: NewContradictionDetector(fast),
		// 初始化新增分析器
		relationExtractor:     NewRelationExtractor(fast),
		eventExtractor:        NewEventExtractor(fast),
		sentimentAnalyzer:     NewSentimentAnalyzer(fast),
		keywordExtractor:      NewKeywordExtractor(fast),
		knowledgeGraphBuilder: NewKnowledgeGraphBuilder(primary),
	}
}

func (a *Analyzer) SetProgressCallback(cb func(task *AnalysisTask)) {
	a.progressMu.Lock()
	a.onProgress = cb
	a.progressMu.Unlock()
}

// progressCallback 返回当前的进度回调（可能为 nil）。
func (a *Analyzer) progressCallback() func(task *AnalysisTask) {
	a.progressMu.Lock()
	defer a.progressMu.Unlock()
	return a.onProgress
}

func (a *Analyzer) Start(ctx context.Context, task *AnalysisTask) error {
	a.mu.Lock()
	task.Status = TaskStatusRunning
	task.StartedAt = time.Now().Unix()
	a.tasks[task.ID] = task
	// P0：清理已结束的历史任务，避免 a.tasks 只增不删造成内存泄漏
	a.pruneLocked()
	a.mu.Unlock()

	a.notifyProgress(task)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				a.mu.Lock()
				task.Status = TaskStatusFailed
				task.Message = fmt.Sprintf("panic: %v", r)
				a.mu.Unlock()
				a.notifyProgress(task)
			}
		}()

		taskCtx, cancel := context.WithCancel(context.Background())
		a.mu.Lock()
		task.cancel = cancel
		a.mu.Unlock()

		a.run(taskCtx, task)
	}()

	return nil
}

func (a *Analyzer) run(ctx context.Context, task *AnalysisTask) {
	modifiedSince := 0
	prevResult := &AnalysisResult{}
	if task.Type == TaskTypeIncremental {
		a.updateTask(task, TaskStatusRunning, 0.0, "Incremental analysis: finding last completed task...")
		lastTask, err := a.taskRepo.FindLastCompleted()
		if err == nil && lastTask != nil && lastTask.CompletedAt != nil {
			modifiedSince = int(lastTask.CompletedAt.Unix())
			a.updateTask(task, TaskStatusRunning, 0.0, fmt.Sprintf("Incremental: checking documents modified since %s", lastTask.CompletedAt.Format("2006-01-02 15:04:05")))

			// M-5: 增量差异分析 - 加载上次分析结果的哈希映射，用于内容比对
			if lastTask.Result != "" {
				if err := json.Unmarshal([]byte(lastTask.Result), prevResult); err == nil {
					hashes := make(map[int]string)
					for i := range prevResult.Summaries {
						if prevResult.Summaries[i].ContentHash != "" {
							hashes[prevResult.Summaries[i].SourceDocID] = prevResult.Summaries[i].ContentHash
						}
					}
					a.mu.Lock()
					task.CheckpointPrevHashes = hashes
					a.mu.Unlock()
					if len(hashes) > 0 {
						logx.Infof("[Analyzer] M-5: Loaded %d content hashes from previous analysis", len(hashes))
					}
				}
			}
		} else {
			// 没有历史任务时，改为分析最近30天的文档
			modifiedSince = int(time.Now().Add(-30 * 24 * time.Hour).Unix())
			a.updateTask(task, TaskStatusRunning, 0.0, "No previous analysis found, analyzing documents from last 30 days instead...")
		}
	} else if task.Type == TaskTypeQuick {
		// 修改：从 24 小时改为 7 天，避免漏掉文档
		modifiedSince = int(time.Now().Add(-7 * 24 * time.Hour).Unix())
		a.updateTask(task, TaskStatusRunning, 0.0, "Quick scan: reading documents updated in last 7 days...")
	} else {
		a.updateTask(task, TaskStatusRunning, 0.0, "Reading all documents...")
	}

	var docs []*DocumentContent
	var readDiag *ReadDiagnostics
	var err error

	if task.Type == TaskTypeSingleDoc && task.TargetDocID > 0 {
		// P0-1: 直接按 docID 单条查询，避免全量扫描
		a.updateTask(task, TaskStatusRunning, 0.0, fmt.Sprintf("Single doc analysis: loading document %d...", task.TargetDocID))
		doc, docErr := a.docReader.ReadSingleDocument(task.TargetDocID)
		if docErr != nil {
			a.updateTask(task, TaskStatusFailed, 0, fmt.Sprintf("Load document %d failed: %v", task.TargetDocID, docErr))
			return
		}
		docs = []*DocumentContent{doc}
		readDiag = &ReadDiagnostics{TotalDocs: 1, TotalSpaces: 1}
	} else if task.Type == TaskTypeMultiDoc && len(task.TargetDocIDs) > 0 {
		// P0-1: 多文档分析，批量查询
		a.updateTask(task, TaskStatusRunning, 0.0, fmt.Sprintf("Multi-doc analysis: loading %d documents...", len(task.TargetDocIDs)))
		multiDocs, docErr := a.docReader.ReadDocumentsByIDs(task.TargetDocIDs)
		if docErr != nil {
			a.updateTask(task, TaskStatusFailed, 0, fmt.Sprintf("Load documents failed: %v", docErr))
			return
		}
		docs = multiDocs
		readDiag = &ReadDiagnostics{TotalDocs: len(docs), TotalSpaces: 0}
	} else {
		// P0-1: 优先使用 task.ModifiedSince（用户日期范围），其次用任务类型推导的 modifiedSince
		effectiveModifiedSince := modifiedSince
		if task.ModifiedSince > 0 {
			effectiveModifiedSince = int(task.ModifiedSince)
		}
		docs, readDiag, err = a.docReader.ReadAllDocuments(task.TargetSpaceIDs, effectiveModifiedSince)
		if err != nil {
			a.updateTask(task, TaskStatusFailed, 0, fmt.Sprintf("Read documents error: %v", err))
			return
		}
	}

	skipCount := 0
	unchangedCount := 0
	if task.Type == TaskTypeIncremental {
		manualEditMappings, err := a.mappingRepo.FindByReviewStatus(knowledgemodel.ReviewStatusManualEdit)
		if err == nil && len(manualEditMappings) > 0 {
			skipDocIDs := make(map[int]bool)
			for _, m := range manualEditMappings {
				skipDocIDs[m.SourceDocID] = true
			}
			var filteredDocs []*DocumentContent
			for _, doc := range docs {
				if !skipDocIDs[doc.DocID] {
					filteredDocs = append(filteredDocs, doc)
				} else {
					skipCount++
				}
			}
			docs = filteredDocs
			logx.Infof("[Analyzer] Skipped %d manual_edit documents", skipCount)
		}

		// M-5: 增量差异分析 - 基于内容哈希过滤未变更文档
		a.mu.RLock()
		prevHashes := task.CheckpointPrevHashes
		a.mu.RUnlock()
		if len(prevHashes) > 0 {
			var diffFilteredDocs []*DocumentContent
			for _, doc := range docs {
				h := sha256.Sum256([]byte(doc.Content))
				currHash := fmt.Sprintf("%x", h)
				if prevHash, ok := prevHashes[doc.DocID]; ok && prevHash == currHash {
					unchangedCount++
					continue
				}
				diffFilteredDocs = append(diffFilteredDocs, doc)
			}
			docs = diffFilteredDocs
			if unchangedCount > 0 {
				logx.Infof("[Analyzer] M-5: Skipped %d unchanged documents (content hash match)", unchangedCount)
			}
		}
	}

	a.setTotalDocs(task, len(docs))

	// 构建诊断消息
	diagMsg := fmt.Sprintf("Diagnostics: spaces=%d, aiSpaceID=%d, targetSpaceIDs=%v, modifiedSince=%d (%s), docs=%d (skipped: type=%d, delete=%d, time=%d)",
		readDiag.TotalSpaces, readDiag.AISpaceID, readDiag.TargetSpaceIDs, readDiag.ModifiedSince, readDiag.ModifiedSinceTime,
		readDiag.TotalDocs, readDiag.SkippedByType, readDiag.SkippedByDelete, readDiag.SkippedByTime)

	msg := fmt.Sprintf("Found %d documents, batching...", len(docs))
	if skipCount > 0 {
		msg = fmt.Sprintf("Found %d documents (skipped %d manual_edit)", len(docs), skipCount)
	}
	if unchangedCount > 0 {
		msg = fmt.Sprintf("Found %d documents (diff-skipped %d unchanged)", len(docs), unchangedCount)
	}
	msg = msg + "\n" + diagMsg
	a.updateTask(task, TaskStatusRunning, 0.0, msg)

	if len(docs) == 0 {
		hint := ""
		switch task.Type {
		case TaskTypeQuick:
			hint = "（最近7天没有文档被修改，请尝试全量分析）"
		case TaskTypeIncremental:
			hint = "（上次分析后没有文档被修改，请尝试全量分析）"
		case TaskTypeSpaceOnly:
			hint = "（所选空间没有可分析的文档，请检查空间选择）"
		default:
			hint = "（请检查知识库中是否有文档）"
		}
		a.updateTask(task, TaskStatusCompleted, 1.0, "No documents to analyze"+hint)
		a.publishResult(task, &AnalysisResult{GeneratedAt: time.Now().Unix()})
		return
	}

	batches := a.processor.BatchDocuments(docs)
	logx.Infof("[Analyzer] Task %s: %d docs -> %d batches", task.ID, len(docs), len(batches))

	if len(batches) == 0 {
		a.updateTask(task, TaskStatusCompleted, 1.0, fmt.Sprintf("No batches created (check if AI provider configured)"))
		a.publishResult(task, &AnalysisResult{TotalDocs: len(docs), GeneratedAt: time.Now().Unix()})
		return
	}

	a.updateTask(task, TaskStatusRunning, 0.02, "Generating global category framework...")
	globalCategories := a.generateGlobalCategories(ctx, docs)
	logx.Infof("[Analyzer] Task %s: global categories length=%d", task.ID, len(globalCategories))

	allResults := &AnalysisResult{
		TotalDocs:   len(docs),
		GeneratedAt: time.Now().Unix(),
	}

	// S8: 恢复时加载已有结果（P0：读取 task 字段必须持锁；Result 是已发布的不可变快照）
	if cp := a.checkpointView(task); cp != nil && len(cp.ProcessedDocIDs) > 0 {
		a.mu.RLock()
		prevPublished := task.Result
		a.mu.RUnlock()
		if prevPublished != nil {
			allResults.Categories = prevPublished.Categories
			allResults.Summaries = prevPublished.Summaries
			allResults.Relations = prevPublished.Relations
			allResults.Themes = prevPublished.Themes
			logx.Infof("[Analyzer] Loaded previous results: %d categories, %d summaries, %d relations, %d themes",
				len(allResults.Categories), len(allResults.Summaries), len(allResults.Relations), len(allResults.Themes))
		}
	}

	cumulativeSummary := ""
	errorCount := 0
	startBatchIndex := 0

	// S8: 如果有checkpoint，从断点恢复
	if cp := a.checkpointView(task); cp != nil && cp.LastBatchIndex >= 0 {
		startBatchIndex = cp.LastBatchIndex + 1
		a.setProcessedDocs(task, len(cp.ProcessedDocIDs))
		logx.Infof("[Analyzer] Resuming from batch %d, already processed %d docs", startBatchIndex, len(cp.ProcessedDocIDs))
		a.updateTask(task, TaskStatusRunning, float64(startBatchIndex)/float64(len(batches)),
			fmt.Sprintf("Resuming from batch %d/%d", startBatchIndex+1, len(batches)))
	}

	for i, batch := range batches {
		// S8: 跳过已处理的批次
		if i < startBatchIndex {
			continue
		}

		select {
		case <-ctx.Done():
			a.updateTask(task, TaskStatusPaused, float64(i)/float64(len(batches)), "Paused by user")
			// S8: 暂停时保存checkpoint
			a.saveCheckpoint(task)
			return
		default:
		}

		a.updateTask(task, TaskStatusRunning, float64(i)/float64(len(batches)),
			fmt.Sprintf("Processing batch %d/%d (space: %s)", i+1, len(batches), batch[0].SpaceName))

		result, tokens, err := a.processor.ProcessBatch(ctx, batch, globalCategories, cumulativeSummary)
		if err != nil {
			errorCount++
			logx.Infof("[Analyzer] Batch %d/%d error: %v", i+1, len(batches), err)
			if errorCount == len(batches) {
				a.updateTask(task, TaskStatusFailed, float64(i)/float64(len(batches)),
					fmt.Sprintf("All %d batches failed, last error: %v", len(batches), err))
				// S8: 失败时保存checkpoint以支持断点续分析
				a.saveCheckpoint(task)
				return
			}
			continue
		}

		a.addTokenUsage(task, tokens)

		allResults.Categories = mergeCategories(allResults.Categories, result.Categories)
		allResults.Summaries = append(allResults.Summaries, result.Summaries...)
		allResults.Relations = append(allResults.Relations, result.Relations...)
		allResults.Themes = append(allResults.Themes, result.Themes...)

		batchSummary := a.generateBatchSummary(result)
		if batchSummary != "" {
			if cumulativeSummary != "" {
				cumulativeSummary += "\n\n"
			}
			runes := []rune(batchSummary)
			if len(runes) > 800 {
				cumulativeSummary += string(runes[:800])
			} else {
				cumulativeSummary += batchSummary
			}
			allRunes := []rune(cumulativeSummary)
			if len(allRunes) > 3000 {
				cumulativeSummary = string(allRunes[len(allRunes)-3000:])
			}
		}

		a.addProcessedDocs(task, len(batch))
		a.setTaskProgress(task, float64(i+1)/float64(len(batches)))
		// P0：发布的是不可变深拷贝，run() 之后继续原地改 allResults 也不会与读取方竞争
		a.publishResult(task, allResults)
		a.notifyProgress(task)

		// S8: 每批完成后保存checkpoint（P0：写入 Checkpoint 必须持锁）
		a.mu.Lock()
		chk := a.ensureCheckpointLocked(task)
		chk.LastBatchIndex = i
		for _, doc := range batch {
			chk.ProcessedDocIDs = append(chk.ProcessedDocIDs, doc.DocID)
		}
		a.mu.Unlock()
		if (i+1)%3 == 0 || i == len(batches)-1 {
			a.saveCheckpoint(task)
		}
	}

	// L-8: 增量合并策略 - 如果是增量分析，将新结果与上次分析结果合并
	if task.Type == TaskTypeIncremental && prevResult != nil && len(prevResult.Summaries) > 0 {
		allResults = mergeWithExisting(allResults, prevResult)
		logx.Infof("[Analyzer] L-8: Merged with existing results, total summaries: %d", len(allResults.Summaries))
	}

	a.mu.RLock()
	allResults.ProcessedDocs = task.ProcessedDocs
	a.mu.RUnlock()

	a.updateTask(task, TaskStatusRunning, 0.92, "Post-processing: entity extraction and quality assessment...")

	// P0: 实体提取集成 - 对每个文档提取URL、配置项、别名、待办事项
	if a.entityExtractor != nil {
		lowConfidenceCount := 0
		for i := range allResults.Summaries {
			s := &allResults.Summaries[i]
			if s.FullContent == "" {
				continue
			}
			entities, err := a.entityExtractor.ExtractFromContent(ctx, s.FullContent)
			if err != nil {
				logx.Infof("[Analyzer] Entity extraction error for doc %d: %v", s.SourceDocID, err)
				continue
			}
			if entities != nil {
				s.URLs = entities.URLs
				s.ConfigItems = entities.ConfigItems
				s.Aliases = entities.Aliases
				s.ActionItems = entities.ActionItems
				if s.Confidence == "low" {
					lowConfidenceCount++
				}
			}
		}
		logx.Infof("[Analyzer] Entity extraction completed, low-confidence docs: %d", lowConfidenceCount)
	}

	// P0: 质量评估集成 - 对每个文档进行5维度质量评分
	if a.qualityAssessor != nil {
		for i := range allResults.Summaries {
			s := &allResults.Summaries[i]
			if s.FullContent == "" {
				continue
			}
			quality, err := a.qualityAssessor.AssessQuality(ctx, s.SourceDocID, s.FullContent)
			if err != nil {
				logx.Infof("[Analyzer] Quality assessment error for doc %d: %v", s.SourceDocID, err)
				continue
			}
			if quality != nil {
				s.Quality = quality
				if quality.OverallScore < 60 {
					logx.Infof("[Analyzer] Low quality doc %d: score=%.0f, issues=%d",
						s.SourceDocID, quality.OverallScore, len(quality.Issues))
				}
			}
		}
	}

	// M-18: 文档过期预警检测
	{
		now := time.Now().Unix()
		expiredCount := 0
		for i := range allResults.Summaries {
			s := &allResults.Summaries[i]
			// 方式1: 质量评估中的时效性得分 < 30 视为过期
			if s.Quality != nil && s.Quality.Freshness < 30 {
				s.ExpiredWarning = fmt.Sprintf("时效性评分偏低(%.0f/100)，建议审查更新", s.Quality.Freshness)
				expiredCount++
				continue
			}
			// 方式2: 超过180天未更新视为过期（规则退避）
			if s.UpdateTime > 0 {
				daysSinceUpdate := (now - int64(s.UpdateTime)) / 86400
				if daysSinceUpdate > 180 {
					s.ExpiredWarning = fmt.Sprintf("超过%d天未更新（上次更新：%s），建议审查",
						daysSinceUpdate, time.Unix(int64(s.UpdateTime), 0).Format("2006-01-02"))
					expiredCount++
				}
			}
		}
		if expiredCount > 0 {
			logx.Infof("[Analyzer] M-18: Flagged %d docs as expired", expiredCount)
		}
	}

	// L-7: 分析置信度自校验 - 对低置信度文档翻转质量为低质量，空摘要标记为低质量
	{
		lowConfCheckCount := 0
		emptySummaryCount := 0
		for i := range allResults.Summaries {
			s := &allResults.Summaries[i]
			if s.Confidence == "low" {
				lowConfCheckCount++
				if s.Quality == nil {
					s.Quality = &QualityScore{
						DocID:        s.SourceDocID,
						OverallScore: 40,
						Completeness: 40,
						Freshness:    50,
						Clarity:      50,
						Consistency:  50,
						Structure:    50,
						Issues:       []QualityIssue{{Type: "low_confidence", Severity: "high", Description: "分析置信度低，建议人工审查", Location: "全文"}},
						Suggestions:  []string{"建议人工审核文档内容"},
					}
				} else if s.Quality.OverallScore > 40 {
					s.Quality.OverallScore = 40
				}
			}
			if s.Summary == "" || len(s.KeyPoints) == 0 {
				emptySummaryCount++
				if s.Quality == nil {
					s.Quality = &QualityScore{
						DocID:        s.SourceDocID,
						OverallScore: 30,
						Completeness: 30,
						Freshness:    50,
						Clarity:      50,
						Consistency:  50,
						Structure:    50,
						Issues:       []QualityIssue{{Type: "missing_info", Severity: "high", Description: "分析结果为空摘要，质量不可靠", Location: "全文"}},
						Suggestions:  []string{"建议重新分析该文档"},
					}
				} else if s.Quality.OverallScore > 30 {
					s.Quality.OverallScore = 30
				}
			}
		}
		if lowConfCheckCount > 0 || emptySummaryCount > 0 {
			logx.Infof("[Analyzer] L-7: Confidence check: %d low-confidence, %d empty summaries", lowConfCheckCount, emptySummaryCount)
		}
	}

	// M-15: 跨文档实体归并 - 三级策略：精确匹配 → 模糊匹配 → LLM语义确认
	if a.entityResolver != nil {
		canonicalEntities := a.entityResolver.ResolveEntities(ctx, allResults.Summaries)
		if len(canonicalEntities) > 0 {
			logx.Infof("[Analyzer] M-15: Resolved %d canonical entities from %d docs",
				len(canonicalEntities), len(allResults.Summaries))
			// 将归并结果写入分析结果（暂存在 first summary 的 ActionItems 列表）
			// 后续可扩展为 AnalysisResult 的独立字段
		}
	}

	a.updateTask(task, TaskStatusRunning, 0.95, "Post-processing: cross-batch relations and category normalization...")

	crossBatchRelations := a.discoverCrossBatchRelations(ctx, allResults)
	allResults.Relations = append(allResults.Relations, crossBatchRelations...)

	a.normalizeCategories(ctx, allResults)

	a.mergeDuplicateThemes(allResults)

	// S9: 知识缺口发现
	allResults.Gaps = a.discoverKnowledgeGaps(ctx, allResults)

	// 矛盾检测
	if a.contradictionDetector != nil && len(allResults.Summaries) >= 2 {
		contradictions, err := a.contradictionDetector.DetectContradictions(ctx, allResults.Summaries)
		if err == nil && len(contradictions) > 0 {
			logx.Infof("[Analyzer] Detected %d contradictions", len(contradictions))
			for _, c := range contradictions {
				allResults.Relations = append(allResults.Relations, KnowledgeRelation{
					SourceDocID:  c.DocAID,
					TargetDocID:  c.DocBID,
					RelationType: "contradicts",
					Description:  c.Description,
					Confidence:   c.Severity,
				})
			}
		}
	}

	a.updateTask(task, TaskStatusRunning, 0.96, "Post-processing: enhanced analysis (relation/event/keyword extraction)...")

	// Enhanced: Relation Extraction - extract entity relations from documents
	if a.relationExtractor != nil {
		relationCount := 0
		for i := range allResults.Summaries {
			s := &allResults.Summaries[i]
			if s.FullContent == "" {
				continue
			}
			result, err := a.relationExtractor.ExtractRelations(ctx, s.FullContent)
			if err != nil {
				logx.Infof("[Analyzer] Relation extraction error for doc %d: %v", s.SourceDocID, err)
				continue
			}
			if result != nil && len(result.Relations) > 0 {
				allResults.ExtractedRelations = append(allResults.ExtractedRelations, result.Relations...)
				relationCount += len(result.Relations)
			}
		}
		logx.Infof("[Analyzer] Extracted %d semantic relations from documents", relationCount)
	}

	// Enhanced: Event Extraction - identify events from document summaries
	if a.eventExtractor != nil && len(allResults.Summaries) > 0 {
		events, err := a.eventExtractor.ExtractEventsFromSummaries(ctx, allResults.Summaries)
		if err == nil && len(events) > 0 {
			allResults.ExtractedEvents = events
			logx.Infof("[Analyzer] Extracted %d events from documents", len(events))
		}
	}

	// Enhanced: Keyword Extraction - extract keywords from each document
	if a.keywordExtractor != nil {
		for i := range allResults.Summaries {
			s := &allResults.Summaries[i]
			if s.FullContent == "" {
				continue
			}
			kwResult, err := a.keywordExtractor.ExtractKeywords(ctx, s.FullContent, 10)
			if err == nil && kwResult != nil && len(kwResult.Keywords) > 0 {
				allResults.KeywordResults = append(allResults.KeywordResults, DocKeywords{
					DocID:    s.SourceDocID,
					Keywords: kwResult,
				})
			}
		}
		logx.Infof("[Analyzer] Extracted keywords from %d documents", len(allResults.KeywordResults))
	}

	// Enhanced: Knowledge Graph Building - build KG from combined content
	if a.knowledgeGraphBuilder != nil && len(allResults.Summaries) >= 3 {
		var combinedContent strings.Builder
		docCount := 0
		for _, s := range allResults.Summaries {
			if docCount >= 5 { // Limit to first 5 docs for context window
				break
			}
			if s.FullContent != "" {
				content := s.FullContent
				if len(content) > 2000 {
					content = content[:2000]
				}
				combinedContent.WriteString(fmt.Sprintf("【文档：%s】\n%s\n\n", s.SourceName, content))
				docCount++
			}
		}
		if combinedContent.Len() > 0 {
			graph, err := a.knowledgeGraphBuilder.BuildKnowledgeGraph(ctx, combinedContent.String())
			if err == nil && graph != nil && len(graph.Entities) > 0 {
				allResults.KnowledgeGraph = graph
				logx.Infof("[Analyzer] Built knowledge graph: %d entities, %d relations",
					len(graph.Entities), len(graph.Relations))
			}
		}
	}

	a.mu.Lock()
	a.publishResultLocked(task, allResults)
	task.Progress = 1.0
	processedDocs := task.ProcessedDocs
	a.mu.Unlock()

	var errMsg string
	if errorCount > 0 {
		errMsg = fmt.Sprintf("Completed: %d docs analyzed, %d categories, %d relations (%d batches failed)",
			processedDocs, len(allResults.Categories), len(allResults.Relations), errorCount)
	} else {
		entityCount := 0
		lowQualityCount := 0
		expiredCount := 0
		for _, s := range allResults.Summaries {
			if len(s.URLs)+len(s.ConfigItems)+len(s.Aliases)+len(s.ActionItems) > 0 {
				entityCount++
			}
			if s.Quality != nil {
				lowQualityCount++
			}
			if s.ExpiredWarning != "" {
				expiredCount++
			}
		}
		errMsg = fmt.Sprintf("Completed: %d docs analyzed, %d categories, %d relations, %d with entities, %d low-quality, %d expired, %d extracted-relations, %d events, %d kg-entities",
			processedDocs, len(allResults.Categories), len(allResults.Relations), entityCount, lowQualityCount, expiredCount,
			len(allResults.ExtractedRelations), len(allResults.ExtractedEvents),
			func() int {
				if allResults.KnowledgeGraph != nil {
					return len(allResults.KnowledgeGraph.Entities)
				}
				return 0
			}())
	}
	a.updateTask(task, TaskStatusCompleted, 1.0, errMsg)
}

func (a *Analyzer) generateGlobalCategories(ctx context.Context, docs []*DocumentContent) string {
	llm := a.processor.primaryProvider
	if llm == nil {
		llm = a.processor.fastProvider
	}
	if llm == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("以下是知识库中所有文档的空间分布、路径及内容摘要，请生成一个全局分类体系：\n\n")

	spaceMap := make(map[int]string)
	for _, doc := range docs {
		if _, ok := spaceMap[doc.SpaceID]; !ok {
			spaceMap[doc.SpaceID] = doc.SpaceName
			sb.WriteString(fmt.Sprintf("空间: %s (%s)\n", doc.SpaceName, doc.SpaceDesc))
		}
	}
	sb.WriteString("\n文档信息（路径 + 内容摘要，最多40个）：\n")
	limit := min(40, len(docs))
	for i := 0; i < limit; i++ {
		doc := docs[i]
		// P1: 除了面包屑路径，还包含文档内容摘要供分类参考
		contentPreview := doc.Content
		runes := []rune(contentPreview)
		if len(runes) > 200 {
			contentPreview = string(runes[:200]) + "..."
		}
		contentPreview = strings.ReplaceAll(contentPreview, "\n", " ")
		sb.WriteString(fmt.Sprintf("- %s\n  内容: %s\n", doc.BreadCrumb, contentPreview))
	}

	resp, err := llm.Chat(ctx, "你是一个知识分类专家。请根据文档层级、空间分布和实际内容，生成一个全局分类体系。输出格式：每行一个分类，用 / 表示层级，如：\n技术架构/API设计\n技术架构/数据库\n产品/需求文档", []provider.Message{
		provider.NewTextMessage("user", sb.String()),
	}, provider.WithMaxTokens(1024), provider.WithTemperature(0.2))
	if err != nil {
		logx.Infof("[Analyzer] Global category generation error: %v", err)
		return ""
	}

	return resp.Content
}

func (a *Analyzer) generateBatchSummary(result *AnalysisResult) string {
	if len(result.Summaries) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("本批次分析了 %d 个文档，发现 %d 个分类：\n", result.TotalDocs, len(result.Categories)))
	for _, cat := range result.Categories {
		sb.WriteString(fmt.Sprintf("- %s (%d个文档): %s\n", cat.Name, len(cat.DocIDs), cat.Description))
	}
	if len(result.Relations) > 0 {
		sb.WriteString(fmt.Sprintf("发现 %d 个文档间关系\n", len(result.Relations)))
		for _, r := range result.Relations {
			sb.WriteString(fmt.Sprintf("- 文档%d → 文档%d: %s (%s)\n", r.SourceDocID, r.TargetDocID, r.Description, r.RelationType))
		}
	}
	// 增加被分析文档的概览
	sb.WriteString("\n文档概览：\n")
	for _, s := range result.Summaries {
		sb.WriteString(fmt.Sprintf("- [%d] %s → %s (要点: %d, 标签: %d)\n",
			s.SourceDocID, s.SourceName, s.Category, len(s.KeyPoints), len(s.Tags)))
	}

	runes := []rune(sb.String())
	if len(runes) > 1000 {
		return string(runes[:1000]) + "..."
	}
	return sb.String()
}

func (a *Analyzer) discoverCrossBatchRelations(ctx context.Context, result *AnalysisResult) []KnowledgeRelation {
	llm := a.processor.reasoningProvider
	if llm == nil {
		llm = a.processor.fastProvider
	}
	if llm == nil {
		llm = a.processor.primaryProvider
	}
	if llm == nil || len(result.Summaries) < 2 {
		return nil
	}

	docMap := make(map[int]*DocSummary)
	for i := range result.Summaries {
		docMap[result.Summaries[i].SourceDocID] = &result.Summaries[i]
	}

	// Phase2: 跨批次关联分析使用更丰富的上下文（摘要+关键要点+标签+章节标题）
	var sb strings.Builder
	sb.WriteString("以下是所有文档的详细分析结果，请发现跨文档的知识关系：\n\n")
	limit := min(40, len(result.Summaries)) // 减少数量，增加每个文档的信息量
	for i := 0; i < limit; i++ {
		s := result.Summaries[i]
		sb.WriteString(fmt.Sprintf("### 文档%d: %s\n", s.SourceDocID, s.SourceName))
		sb.WriteString(fmt.Sprintf("- 分类: %s\n", s.Category))
		sb.WriteString(fmt.Sprintf("- 摘要: %s\n", s.Summary))
		sb.WriteString(fmt.Sprintf("- 空间: %s\n", s.SpaceName))
		if len(s.KeyPoints) > 0 {
			sb.WriteString(fmt.Sprintf("- 关键要点: %s\n", strings.Join(s.KeyPoints, ", ")))
		}
		if len(s.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("- 标签: %s\n", strings.Join(s.Tags, ", ")))
		}
		// 包含章节标题，帮助LLM理解文档结构
		if len(s.Sections) > 0 {
			var headings []string
			for _, sec := range s.Sections {
				if len(sec.Heading) > 0 {
					headings = append(headings, sec.Heading)
				}
			}
			if len(headings) > 0 {
				sb.WriteString(fmt.Sprintf("- 章节: %s\n", strings.Join(headings, " > ")))
			}
		}
		// 包含文档全文预览，帮助发现深层关联
		if s.FullContent != "" {
			contentRunes := []rune(s.FullContent)
			if len(contentRunes) > 2000 {
				sb.WriteString(fmt.Sprintf("- 内容预览: %s...\n", string(contentRunes[:2000])))
			} else {
				sb.WriteString(fmt.Sprintf("- 内容预览: %s\n", s.FullContent))
			}
		}
		sb.WriteString("\n")
	}

	resp, err := llm.Chat(ctx, `请基于以上文档的详细信息（含摘要、要点、标签、章节结构），发现跨文档的知识关系。
输出JSON数组：[{"source_doc_id":0,"target_doc_id":0,"type":"references","description":"..."}]
关系类型：references/depends_on/related_to/contradicts/parent_of/child_of/prerequisite_of/alternative_of
仅输出确信的关系，不确定的不输出。`, []provider.Message{
		provider.NewTextMessage("user", sb.String()),
	}, provider.WithMaxTokens(4096), provider.WithTemperature(0.2))
	if err != nil {
		logx.Infof("[Analyzer] Cross-batch relation discovery error: %v", err)
		return nil
	}

	var relations []KnowledgeRelation
	jsonStr := utils.ExtractJSON(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), &relations); err != nil {
		arrStr := resp.Content
		if idx := strings.Index(arrStr, "["); idx >= 0 {
			arrStr = arrStr[idx:]
			if end := strings.LastIndex(arrStr, "]"); end >= 0 {
				arrStr = arrStr[:end+1]
			}
		}
		_ = json.Unmarshal([]byte(arrStr), &relations)
	}

	return relations
}

func (a *Analyzer) normalizeCategories(ctx context.Context, result *AnalysisResult) {
	if len(result.Categories) < 2 {
		return
	}

	llm := a.processor.fastProvider
	if llm == nil {
		llm = a.processor.primaryProvider
	}
	if llm == nil {
		return
	}

	var names []string
	for _, cat := range result.Categories {
		names = append(names, cat.Name)
	}

	resp, err := llm.Chat(ctx, `以下是一些知识分类名称，请将语义相同或相近的分类合并。
输出JSON对象，key为原分类名，value为合并后的分类名：
{"前端开发": "前端开发", "Web前端": "前端开发", "数据库设计": "数据库"}
仅合并语义相同的分类，不同的保持不变。`, []provider.Message{
		provider.NewTextMessage("user", strings.Join(names, "\n")),
	}, provider.WithMaxTokens(1024), provider.WithTemperature(0.1))
	if err != nil {
		logx.Infof("[Analyzer] Category normalization error: %v", err)
		return
	}

	var mapping map[string]string
	jsonStr := utils.ExtractJSON(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), &mapping); err != nil {
		return
	}

	changed := false
	catMap := make(map[string]*Category)
	for i := range result.Categories {
		if newName, ok := mapping[result.Categories[i].Name]; ok && newName != result.Categories[i].Name {
			result.Categories[i].Name = newName
			changed = true
		}
		if existing, ok := catMap[result.Categories[i].Name]; ok {
			existing.DocIDs = append(existing.DocIDs, result.Categories[i].DocIDs...)
			existing.SpaceIDs = append(existing.SpaceIDs, result.Categories[i].SpaceIDs...)
			result.Categories[i].Name = ""
		} else {
			catMap[result.Categories[i].Name] = &result.Categories[i]
		}
	}

	if changed {
		var filtered []Category
		for _, cat := range result.Categories {
			if cat.Name != "" {
				filtered = append(filtered, cat)
			}
		}
		result.Categories = filtered

		for i := range result.Summaries {
			if newName, ok := mapping[result.Summaries[i].Category]; ok {
				result.Summaries[i].Category = newName
			}
		}
	}
}

func (a *Analyzer) mergeDuplicateThemes(result *AnalysisResult) {
	if len(result.Themes) < 2 {
		return
	}

	merged := make([]bool, len(result.Themes))
	var newThemes []ThemeSummary

	for i := 0; i < len(result.Themes); i++ {
		if merged[i] {
			continue
		}
		t := result.Themes[i]
		for j := i + 1; j < len(result.Themes); j++ {
			if merged[j] {
				continue
			}
			overlap := docIDOverlap(t.RelatedDocIDs, result.Themes[j].RelatedDocIDs)
			if overlap >= 0.5 {
				t.Summary += "；" + result.Themes[j].Summary
				t.RelatedDocIDs = mergeDocIDs(t.RelatedDocIDs, result.Themes[j].RelatedDocIDs)
				merged[j] = true
			}
		}
		newThemes = append(newThemes, t)
		merged[i] = true
	}

	result.Themes = newThemes
}

func docIDOverlap(a, b []int) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	setA := make(map[int]bool)
	for _, id := range a {
		setA[id] = true
	}
	common := 0
	for _, id := range b {
		if setA[id] {
			common++
		}
	}
	return float64(common) / float64(min(len(a), len(b)))
}

func mergeDocIDs(a, b []int) []int {
	set := make(map[int]bool)
	var result []int
	for _, id := range a {
		if !set[id] {
			set[id] = true
			result = append(result, id)
		}
	}
	for _, id := range b {
		if !set[id] {
			set[id] = true
			result = append(result, id)
		}
	}
	return result
}

func (a *Analyzer) Pause(taskID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	task, ok := a.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	if task.Status != TaskStatusRunning {
		return fmt.Errorf("task is not running: %s", task.Status)
	}
	if task.cancel != nil {
		task.cancel()
	}
	task.Status = TaskStatusPaused
	return nil
}

func (a *Analyzer) Resume(taskID string) error {
	// S8: 断点续分析实现
	a.mu.Lock()
	defer a.mu.Unlock()

	task, ok := a.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	if task.Status != TaskStatusPaused && task.Status != TaskStatusFailed {
		return fmt.Errorf("task cannot be resumed: status=%s", task.Status)
	}

	// S8: 即使没有checkpoint也允许恢复（从头开始）
	if task.Checkpoint == nil {
		logx.Infof("[Analyzer] No checkpoint for task %s, will restart from beginning", taskID)
		task.Checkpoint = &AnalysisCheckpoint{
			LastBatchIndex:  -1,
			ProcessedDocIDs: []int{},
			Phase:           "batch",
		}
	}

	task.Status = TaskStatusRunning
	task.Message = "Resuming..."
	// 注意：此处仍持有 a.mu，只能直接用快照回调（notifyProgress 会再取 RLock → 自死锁）
	if cb := a.progressCallback(); cb != nil {
		cb(a.snapshotLocked(task))
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				a.mu.Lock()
				task.Status = TaskStatusFailed
				task.Message = fmt.Sprintf("panic: %v", r)
				a.mu.Unlock()
				a.notifyProgress(task)
			}
		}()

		taskCtx, cancel := context.WithCancel(context.Background())
		a.mu.Lock()
		task.cancel = cancel
		a.mu.Unlock()

		a.run(taskCtx, task)
	}()

	return nil
}

func (a *Analyzer) GetProgress(taskID string) (*AnalysisTask, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	task, ok := a.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	// P0：返回快照而不是共享指针 —— 调用方（HTTP 层）会序列化该对象，
	// 与分析协程并发读写同一 task 会产生 data race。
	return a.snapshotLocked(task), nil
}

func (a *Analyzer) ListTasks() []*AnalysisTask {
	a.mu.RLock()
	defer a.mu.RUnlock()
	tasks := make([]*AnalysisTask, 0, len(a.tasks))
	for _, t := range a.tasks {
		tasks = append(tasks, a.snapshotLocked(t))
	}
	return tasks
}

func (a *Analyzer) updateTask(task *AnalysisTask, status string, progress float64, message string) {
	a.mu.Lock()
	task.Status = status
	task.Progress = progress
	task.Message = message
	if status == TaskStatusCompleted || status == TaskStatusFailed {
		task.CompletedAt = time.Now().Unix()
	}
	// P0：任务进入终态/暂停后释放 context.CancelFunc，否则每次 WithCancel 生成的
	// context 会一直挂在父 context 上造成泄漏。
	if status == TaskStatusCompleted || status == TaskStatusFailed || status == TaskStatusPaused {
		if task.cancel != nil {
			task.cancel()
			task.cancel = nil
		}
	}
	snap := a.snapshotLocked(task)
	a.pruneLocked()
	a.mu.Unlock()
	if cb := a.progressCallback(); cb != nil {
		cb(snap)
	}
}

// snapshotLocked 生成任务快照。调用方必须持有 a.mu。
// P0：Result 字段在发布前已由 publishResultLocked 替换为不可变深拷贝，
// 因此这里浅拷贝标量 + 复制切片/地图即可安全交给外部（HTTP 序列化）读取。
func (a *Analyzer) snapshotLocked(task *AnalysisTask) *AnalysisTask {
	cp := *task
	cp.cancel = nil // 快照不参与取消，取消仍作用于真实任务
	if task.Checkpoint != nil {
		chk := *task.Checkpoint
		if task.Checkpoint.ProcessedDocIDs != nil {
			chk.ProcessedDocIDs = append([]int(nil), task.Checkpoint.ProcessedDocIDs...)
		}
		if task.Checkpoint.FailedDocIDs != nil {
			chk.FailedDocIDs = append([]int(nil), task.Checkpoint.FailedDocIDs...)
		}
		cp.Checkpoint = &chk
	}
	if task.CheckpointPrevHashes != nil {
		m := make(map[int]string, len(task.CheckpointPrevHashes))
		for k, v := range task.CheckpointPrevHashes {
			m[k] = v
		}
		cp.CheckpointPrevHashes = m
	}
	if task.TargetSpaceIDs != nil {
		cp.TargetSpaceIDs = append([]int(nil), task.TargetSpaceIDs...)
	}
	if task.TargetDocIDs != nil {
		cp.TargetDocIDs = append([]int(nil), task.TargetDocIDs...)
	}
	return &cp
}

// publishResultLocked 把当前分析结果以“不可变深拷贝”的形式发布到 task.Result。
// 调用方必须持有 a.mu。run() 会继续原地修改 allResults，若直接发布同一指针，
// 读取方（GetProgress/序列化/落库）会与分析协程产生 data race。
func (a *Analyzer) publishResultLocked(task *AnalysisTask, result *AnalysisResult) {
	if result == nil {
		task.Result = nil
		return
	}
	if data, err := json.Marshal(result); err == nil {
		var clone AnalysisResult
		if uerr := json.Unmarshal(data, &clone); uerr == nil {
			task.Result = &clone
			return
		}
	}
	// 兜底：序列化失败时仍发布原指针（仅记录，不中断分析）
	logx.Warnf("[Analyzer] Task %s: result snapshot marshal failed, publishing live result", task.ID)
	task.Result = result
}

// setTotalDocs 持锁写入 TotalDocs（P0：run() 曾无锁写 task 字段）。
func (a *Analyzer) setTotalDocs(task *AnalysisTask, n int) {
	a.mu.Lock()
	task.TotalDocs = n
	a.mu.Unlock()
}

// addProcessedDocs 持锁累加已处理文档数。
func (a *Analyzer) addProcessedDocs(task *AnalysisTask, n int) {
	a.mu.Lock()
	task.ProcessedDocs += n
	a.mu.Unlock()
}

// setProcessedDocs 持锁设置已处理文档数。
func (a *Analyzer) setProcessedDocs(task *AnalysisTask, n int) {
	a.mu.Lock()
	task.ProcessedDocs = n
	a.mu.Unlock()
}

// setTaskProgress 持锁设置进度。
func (a *Analyzer) setTaskProgress(task *AnalysisTask, progress float64) {
	a.mu.Lock()
	task.Progress = progress
	a.mu.Unlock()
}

// addTokenUsage 持锁累加 token 用量。
func (a *Analyzer) addTokenUsage(task *AnalysisTask, n int) {
	a.mu.Lock()
	task.TokenUsage += n
	a.mu.Unlock()
}

// publishResult 发布结果快照（内部持锁）。
func (a *Analyzer) publishResult(task *AnalysisTask, result *AnalysisResult) {
	a.mu.Lock()
	a.publishResultLocked(task, result)
	a.mu.Unlock()
}

// checkpointView 返回 checkpoint 的只读快照（内部持锁）。
func (a *Analyzer) checkpointView(task *AnalysisTask) *AnalysisCheckpoint {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if task.Checkpoint == nil {
		return nil
	}
	chk := *task.Checkpoint
	chk.ProcessedDocIDs = append([]int(nil), task.Checkpoint.ProcessedDocIDs...)
	chk.FailedDocIDs = append([]int(nil), task.Checkpoint.FailedDocIDs...)
	return &chk
}

// ensureCheckpointLocked 确保 checkpoint 已初始化并可安全写入（调用方持锁）
func (a *Analyzer) ensureCheckpointLocked(task *AnalysisTask) *AnalysisCheckpoint {
	if task.Checkpoint == nil {
		task.Checkpoint = &AnalysisCheckpoint{Phase: "batch"}
	}
	return task.Checkpoint
}

// pruneLocked 清理已结束且超过保留期的任务；数量超限时优先淘汰最早结束的任务。
// 调用方必须持有 a.mu。
func (a *Analyzer) pruneLocked() {
	const (
		taskRetention = 30 * time.Minute
		maxRetained   = 200
	)
	now := time.Now().Unix()
	type entry struct {
		id string
		at int64
	}
	var finished []entry
	for id, t := range a.tasks {
		isFinished := t.Status == TaskStatusCompleted || t.Status == TaskStatusFailed
		if !isFinished {
			continue
		}
		at := t.CompletedAt
		if at == 0 {
			at = t.CreatedAt
		}
		finished = append(finished, entry{id: id, at: at})
		if now-at > int64(taskRetention.Seconds()) {
			delete(a.tasks, id)
		}
	}
	if len(a.tasks) <= maxRetained {
		return
	}
	sort.Slice(finished, func(i, j int) bool { return finished[i].at < finished[j].at })
	for _, e := range finished {
		if len(a.tasks) <= maxRetained {
			return
		}
		delete(a.tasks, e.id)
	}
}

// S8: saveCheckpoint 保存断点信息到数据库
func (a *Analyzer) saveCheckpoint(task *AnalysisTask) {
	if a.taskRepo == nil {
		return
	}
	// P0：Checkpoint 由分析协程持续追加，序列化必须在持锁状态下取快照，
	// 否则与 run() 的写入并发（json 遍历 map/slice 时会 fatal）。
	a.mu.RLock()
	checkpoint := task.Checkpoint
	if checkpoint != nil {
		chk := *checkpoint
		chk.ProcessedDocIDs = append([]int(nil), checkpoint.ProcessedDocIDs...)
		chk.FailedDocIDs = append([]int(nil), checkpoint.FailedDocIDs...)
		checkpoint = &chk
	}
	taskID := task.ID
	a.mu.RUnlock()

	if checkpoint == nil {
		return
	}
	checkpointJSON, err := json.Marshal(checkpoint)
	if err != nil {
		logx.Infof("[Analyzer] Checkpoint marshal error for task %s: %v", taskID, err)
		return
	}
	if err := a.taskRepo.UpdateCheckpoint(taskID, string(checkpointJSON)); err != nil {
		logx.Infof("[Analyzer] Checkpoint save error for task %s: %v", taskID, err)
	}
}

func (a *Analyzer) notifyProgress(task *AnalysisTask) {
	cb := a.progressCallback()
	if cb == nil {
		return
	}
	// P0：回调收到的是快照，避免回调方（落库/序列化）与分析协程共享同一对象
	a.mu.RLock()
	snap := a.snapshotLocked(task)
	a.mu.RUnlock()
	cb(snap)
}

func mergeCategories(existing, new []Category) []Category {
	catMap := make(map[string]*Category)
	for i := range existing {
		catMap[existing[i].Name] = &existing[i]
	}
	for _, cat := range new {
		if existingCat, ok := catMap[cat.Name]; ok {
			existingCat.DocIDs = append(existingCat.DocIDs, cat.DocIDs...)
			existingCat.SpaceIDs = append(existingCat.SpaceIDs, cat.SpaceIDs...)
		} else {
			existing = append(existing, cat)
			catMap[cat.Name] = &existing[len(existing)-1]
		}
	}
	return existing
}

type AnalyzerConfig struct {
	BatchSize       int `mapstructure:"batch_size"`
	MaxConcurrent   int `mapstructure:"max_concurrent"`
	RateLimitRPM    int `mapstructure:"rate_limit_rpm"`
	DocMaxChars     int `mapstructure:"doc_max_chars"`
	ContextMaxChars int `mapstructure:"context_max_chars"`
}

func DefaultAnalyzerConfig() *AnalyzerConfig {
	return &AnalyzerConfig{
		BatchSize:       5,
		MaxConcurrent:   1,
		RateLimitRPM:    60,
		DocMaxChars:     12000, // Phase1: 从 4000 提升到 12000，支持 3 万字文档
		ContextMaxChars: 32000, // Phase1: 从 16000 提升到 32000
	}
}

// S9: discoverKnowledgeGaps 知识缺口发现
// 识别有分类但缺文档的空白区域，以及分类体系中的薄弱环节
func (a *Analyzer) discoverKnowledgeGaps(ctx context.Context, result *AnalysisResult) []KnowledgeGap {
	llm := a.processor.primaryProvider
	if llm == nil {
		llm = a.processor.fastProvider
	}
	if llm == nil || len(result.Categories) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("以下是知识库的分类体系和每个分类下的文档数量。请识别知识缺口：\n\n")
	for _, cat := range result.Categories {
		sb.WriteString(fmt.Sprintf("分类：%s（描述：%s，文档数：%d）\n", cat.Name, cat.Description, len(cat.DocIDs)))
	}
	sb.WriteString("\n请找出以下类型的知识缺口：\n")
	sb.WriteString("1. 有分类但文档数为0或极少（<2）的空白分类\n")
	sb.WriteString("2. 分类体系中明显缺失的领域（基于已有分类推断）\n")
	sb.WriteString("3. 某分类下的文档内容可能不充分\n\n")
	sb.WriteString(`输出JSON数组：[{"category":"分类名","description":"缺口描述","suggested_title":"建议补充的文档标题","priority":"high/medium/low"}]`)

	prompt := sb.String()
	resp, err := llm.Chat(ctx, "你是一个知识管理专家，擅长识别知识体系中的缺口。只输出JSON数组。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(1024), provider.WithTemperature(0.3))
	if err != nil {
		logx.Infof("[Analyzer] Knowledge gap discovery error: %v", err)
		return nil
	}

	var gaps []KnowledgeGap
	trimmed := strings.TrimSpace(resp.Content)
	if idx := strings.Index(trimmed, "["); idx >= 0 {
		trimmed = trimmed[idx:]
		if last := strings.LastIndex(trimmed, "]"); last >= 0 {
			trimmed = trimmed[:last+1]
		}
	}
	if err := json.Unmarshal([]byte(trimmed), &gaps); err != nil {
		logx.Infof("[Analyzer] Knowledge gap parse error: %v", err)
		return nil
	}
	logx.Infof("[Analyzer] Discovered %d knowledge gaps", len(gaps))
	return gaps
}

// L-8: mergeWithExisting 增量合并策略 - 将新分析结果与已有结果合并
// 对于新结果中的每个文档，如果已有结果中存在相同文档的 summary，追加其 key_points 和 tags
func mergeWithExisting(result *AnalysisResult, existing *AnalysisResult) *AnalysisResult {
	existingDocMap := make(map[int]*DocSummary)
	for i := range existing.Summaries {
		existingDocMap[existing.Summaries[i].SourceDocID] = &existing.Summaries[i]
	}
	for i := range result.Summaries {
		s := &result.Summaries[i]
		if es, ok := existingDocMap[s.SourceDocID]; ok {
			existingKeySet := make(map[string]bool)
			for _, kp := range s.KeyPoints {
				existingKeySet[kp] = true
			}
			for _, kp := range es.KeyPoints {
				if !existingKeySet[kp] {
					s.KeyPoints = append(s.KeyPoints, kp)
				}
			}
			existingTagSet := make(map[string]bool)
			for _, tag := range s.Tags {
				existingTagSet[tag] = true
			}
			for _, tag := range es.Tags {
				if !existingTagSet[tag] {
					s.Tags = append(s.Tags, tag)
				}
			}
		}
	}
	return result
}
