package ai

import (
	"fmt"
	"time"

	aimodel "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
)

// Text2SQLService Text2SQL 辅助业务逻辑层
// 从 internal/api/ai/ab_experiment.go、prompt_version.go、sql_trace.go、error_dashboard.go 中提取
type Text2SQLService struct {
	abExperimentRepo  *airepo.ABExperimentRepo
	promptVersionRepo *airepo.PromptVersionRepo
	traceRepo         *airepo.SQLTraceRepo
	feedbackRepo      *airepo.Text2SQLFeedbackRepo
}

// NewText2SQLService 创建 Text2SQLService 实例
func NewText2SQLService(
	abExperimentRepo *airepo.ABExperimentRepo,
	promptVersionRepo *airepo.PromptVersionRepo,
	traceRepo *airepo.SQLTraceRepo,
	feedbackRepo *airepo.Text2SQLFeedbackRepo,
) *Text2SQLService {
	return &Text2SQLService{
		abExperimentRepo:  abExperimentRepo,
		promptVersionRepo: promptVersionRepo,
		traceRepo:         traceRepo,
		feedbackRepo:      feedbackRepo,
	}
}

// ===== A/B 实验 =====

// ListABExperiments 列出 A/B 实验
func (s *Text2SQLService) ListABExperiments(spaceID uint64) ([]aimodel.ABExperiment, error) {
	return s.abExperimentRepo.ListBySpace(spaceID)
}

// CreateABExperiment 创建 A/B 实验
func (s *Text2SQLService) CreateABExperiment(e *aimodel.ABExperiment) error {
	if err := s.abExperimentRepo.Create(e); err != nil {
		return fmt.Errorf("create failed: %w", err)
	}
	return nil
}

// UpdateABExperiment 更新 A/B 实验
func (s *Text2SQLService) UpdateABExperiment(id uint64, e *aimodel.ABExperiment) error {
	e.ID = id
	if err := s.abExperimentRepo.Update(e); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	return nil
}

// DeleteABExperiment 删除 A/B 实验
func (s *Text2SQLService) DeleteABExperiment(id uint64) error {
	return s.abExperimentRepo.Delete(id)
}

// GetABExperiment 获取 A/B 实验
func (s *Text2SQLService) GetABExperiment(id uint64) (*aimodel.ABExperiment, error) {
	return s.abExperimentRepo.FindByID(id)
}

// ===== Prompt 版本 =====

// ListPromptVersions 列出 Prompt 版本
func (s *Text2SQLService) ListPromptVersions(spaceID uint64) ([]aimodel.PromptVersion, error) {
	return s.promptVersionRepo.ListBySpace(spaceID)
}

// CreatePromptVersion 创建 Prompt 版本
func (s *Text2SQLService) CreatePromptVersion(p *aimodel.PromptVersion) error {
	return s.promptVersionRepo.Create(p)
}

// UpdatePromptVersion 更新 Prompt 版本
func (s *Text2SQLService) UpdatePromptVersion(id uint64, p *aimodel.PromptVersion) error {
	p.ID = id
	return s.promptVersionRepo.Update(p)
}

// DeletePromptVersion 删除 Prompt 版本
func (s *Text2SQLService) DeletePromptVersion(id uint64) error {
	return s.promptVersionRepo.Delete(id)
}

// ActivatePromptVersion 启用指定版本并取消同空间同类型下其他版本的启用状态
func (s *Text2SQLService) ActivatePromptVersion(id uint64) error {
	p, err := s.promptVersionRepo.FindByID(id)
	if err != nil {
		return err
	}
	// 取消同空间同查询类型下所有已启用的版本
	if err := s.promptVersionRepo.DeactivateBySpaceAndType(p.SpaceID, p.QueryType); err != nil {
		return err
	}
	// 启用目标版本
	p.Active = true
	return s.promptVersionRepo.Update(p)
}

// ===== SQL Trace =====

// GetSQLTrace 按 trace_id 获取 SQL 追踪
func (s *Text2SQLService) GetSQLTrace(traceID string) (*aimodel.SQLTrace, error) {
	return s.traceRepo.FindByTraceID(traceID)
}

// ListSQLTraces 列出 SQL 追踪
func (s *Text2SQLService) ListSQLTraces(spaceID uint64, page, pageSize int, since time.Time) ([]aimodel.SQLTrace, int64, error) {
	return s.traceRepo.List(spaceID, page, pageSize, since)
}

// ===== 错误仪表盘 =====

// GetErrorSummary 获取错误汇总
func (s *Text2SQLService) GetErrorSummary(spaceID uint64) (interface{}, error) {
	return s.feedbackRepo.ErrorSummary(spaceID)
}

// GetRecentFailures 获取最近失败记录
func (s *Text2SQLService) GetRecentFailures(spaceID uint64, limit int) (interface{}, error) {
	return s.feedbackRepo.RecentFailures(spaceID, limit)
}
