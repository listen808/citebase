package text2sql

import (
	"sync"
	"time"

	logx "github.com/listen808/citebase/internal/log"

	aimodel "github.com/listen808/citebase/internal/model/ai"
)

// FeedbackLoop 反馈闭环系统
type FeedbackLoop struct {
	mu sync.RWMutex

	// 成功记录
	successRecords []FeedbackRecord

	// 失败记录
	failureRecords []FeedbackRecord

	// 修正记录（用户或AST修正）
	correctionRecords []FeedbackRecord

	// 错误模式统计
	errorPatterns map[string]int

	// 持久化仓库（可选）
	repo FeedbackRepo

	// 异步写入通道
	writeChan chan FeedbackRecord
}

// feedbackMaxRecords 进程级反馈缓冲上限，避免长期运行内存无界增长。
const feedbackMaxRecords = 1000

// FeedbackRepo 反馈记录持久化接口
type FeedbackRepo interface {
	Create(record *aimodel.Text2SQLFeedback) error
}

// FeedbackRecord 反馈记录
type FeedbackRecord struct {
	ID            int64
	SpaceID       int
	UserQuery     string
	GeneratedSQL  string
	ExecutionOK   bool
	ErrorType     string
	ErrorMessage  string
	CorrectedSQL  string
	CorrectionSrc string // "ast", "llm", "user"
	Duration      time.Duration
	CreatedAt     time.Time
}

// NewFeedbackLoop 创建反馈闭环（内存模式）
func NewFeedbackLoop() *FeedbackLoop {
	return &FeedbackLoop{
		successRecords:    make([]FeedbackRecord, 0),
		failureRecords:    make([]FeedbackRecord, 0),
		correctionRecords: make([]FeedbackRecord, 0),
		errorPatterns:     make(map[string]int),
		writeChan:         make(chan FeedbackRecord, 100),
	}
}

// NewFeedbackLoopWithRepo 创建支持持久化的反馈闭环
func NewFeedbackLoopWithRepo(repo FeedbackRepo) *FeedbackLoop {
	f := NewFeedbackLoop()
	if repo != nil {
		f.repo = repo
		go f.persistWorker()
	}
	return f
}

// SetRepo 运行时注入持久化仓库
func (f *FeedbackLoop) SetRepo(repo FeedbackRepo) {
	if repo == nil {
		return
	}
	f.mu.Lock()
	needStart := f.repo == nil
	f.repo = repo
	f.mu.Unlock()
	if needStart {
		go f.persistWorker()
	}
}

// persistWorker 后台持久化工作线程
func (f *FeedbackLoop) persistWorker() {
	for record := range f.writeChan {
		f.mu.RLock()
		repo := f.repo
		f.mu.RUnlock()
		if repo == nil {
			continue
		}
		dbRecord := &aimodel.Text2SQLFeedback{
			SpaceID:       uint64(record.SpaceID),
			UserQuery:     record.UserQuery,
			GeneratedSQL:  record.GeneratedSQL,
			ExecutionOK:   record.ExecutionOK,
			ErrorType:     record.ErrorType,
			ErrorMessage:  record.ErrorMessage,
			CorrectedSQL:  record.CorrectedSQL,
			CorrectionSrc: record.CorrectionSrc,
			DurationMs:    int(record.Duration.Milliseconds()),
		}
		if err := repo.Create(dbRecord); err != nil {
			logx.Infof("[FeedbackLoop] persist failed: %v", err)
		}
	}
}

// asyncPersist 异步持久化，永不阻塞主流程
func (f *FeedbackLoop) asyncPersist(record FeedbackRecord) {
	if f.repo == nil {
		return
	}
	select {
	case f.writeChan <- record:
	default:
		logx.Infof("[FeedbackLoop] write channel full, drop one record")
	}
}

// RecordSuccess 记录成功执行
func (f *FeedbackLoop) RecordSuccess(record FeedbackRecord) {
	f.mu.Lock()
	defer f.mu.Unlock()

	record.ExecutionOK = true
	record.CreatedAt = time.Now()
	f.successRecords = append(f.successRecords, record)
	// 环形上限：超过 feedbackMaxRecords 时丢弃最旧记录
	if len(f.successRecords) > feedbackMaxRecords {
		f.successRecords = f.successRecords[len(f.successRecords)-feedbackMaxRecords:]
	}
	f.asyncPersist(record)
}

// RecordFailure 记录失败执行
func (f *FeedbackLoop) RecordFailure(record FeedbackRecord) {
	f.mu.Lock()
	defer f.mu.Unlock()

	record.ExecutionOK = false
	record.CreatedAt = time.Now()
	f.failureRecords = append(f.failureRecords, record)
	if len(f.failureRecords) > feedbackMaxRecords {
		f.failureRecords = f.failureRecords[len(f.failureRecords)-feedbackMaxRecords:]
	}

	// 统计错误模式
	if record.ErrorType != "" {
		f.errorPatterns[record.ErrorType]++
	}
	f.asyncPersist(record)
}

// RecordCorrection 记录修正（用户或AST修正）
func (f *FeedbackLoop) RecordCorrection(record FeedbackRecord) {
	f.mu.Lock()
	defer f.mu.Unlock()

	record.CreatedAt = time.Now()
	f.correctionRecords = append(f.correctionRecords, record)
	if len(f.correctionRecords) > feedbackMaxRecords {
		f.correctionRecords = f.correctionRecords[len(f.correctionRecords)-feedbackMaxRecords:]
	}
	f.asyncPersist(record)
}

// GetErrorPatternFrequency 获取错误模式频率
func (f *FeedbackLoop) GetErrorPatternFrequency() map[string]int {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make(map[string]int)
	for k, v := range f.errorPatterns {
		result[k] = v
	}
	return result
}

// GetSuccessExamples 获取成功的 SQL 示例（用于 Few-shot）
func (f *FeedbackLoop) GetSuccessExamples(limit int) []FeedbackRecord {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if limit <= 0 || limit > len(f.successRecords) {
		limit = len(f.successRecords)
	}

	// 返回最近的 limit 条的拷贝，避免共享底层数组被后续 append 覆写
	start := len(f.successRecords) - limit
	if start < 0 {
		start = 0
	}
	result := make([]FeedbackRecord, limit)
	copy(result, f.successRecords[start:])
	return result
}

// GetCorrectionExamples 获取修正示例
func (f *FeedbackLoop) GetCorrectionExamples(limit int) []FeedbackRecord {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if limit <= 0 || limit > len(f.correctionRecords) {
		limit = len(f.correctionRecords)
	}

	start := len(f.correctionRecords) - limit
	if start < 0 {
		start = 0
	}
	result := make([]FeedbackRecord, limit)
	copy(result, f.correctionRecords[start:])
	return result
}

// GetMostFrequentError 获取最频繁的错误类型
func (f *FeedbackLoop) GetMostFrequentError() (string, int) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	maxType := ""
	maxCount := 0
	for errType, count := range f.errorPatterns {
		if count > maxCount {
			maxType = errType
			maxCount = count
		}
	}
	return maxType, maxCount
}

// GetStats 获取反馈统计
func (f *FeedbackLoop) GetStats() FeedbackStats {
	f.mu.RLock()
	defer f.mu.RUnlock()

	total := len(f.successRecords) + len(f.failureRecords)
	successRate := 0.0
	if total > 0 {
		successRate = float64(len(f.successRecords)) / float64(total)
	}

	return FeedbackStats{
		TotalQueries: total,
		SuccessCount: len(f.successRecords),
		FailureCount: len(f.failureRecords),
		SuccessRate:  successRate,
		Corrections:  len(f.correctionRecords),
	}
}

// FeedbackStats 反馈统计
type FeedbackStats struct {
	TotalQueries int     `json:"total_queries"`
	SuccessCount int     `json:"success_count"`
	FailureCount int     `json:"failure_count"`
	SuccessRate  float64 `json:"success_rate"`
	Corrections  int     `json:"corrections"`
}
