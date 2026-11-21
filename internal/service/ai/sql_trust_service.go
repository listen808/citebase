package ai

import (
	"encoding/json"
	"errors"
	"fmt"

	aimodel "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
)

// 这些哨兵带的是**日志用的英文**：中文句子现在住在 internal/response/messages.go，
// 由处理器按键回给前端。原先服务层直接 fmt.Errorf("中文: %w", err)，处理器再 err.Error()
// 原样回出 —— 于是英文界面上那几句永远是中文（2026-10-12 实测此类共 7 句真会走到用户屏幕）。
// 原因（%w 带出来的 GORM/驱动原文）留在日志里，不进响应：那是 error_leak_test 拦的泄漏面。
var (
	// "这条 SQL 历史记录不存在"用 sql_history_service.go 里的 ErrSQLHistoryNotFound，
	// 这里不再声明第二个同义哨兵。
	ErrSQLHistoryQueryFailed = errors.New("failed to query sql history")
	ErrSQLExplanationInvalid = errors.New("stored sql explanation is unreadable")
	ErrSQLAuditTraceNotFound = errors.New("sql audit trace not found")
)

// SQLTrustService SQL 可信闭环业务逻辑层
// 从 internal/api/ai/sql_trust.go 中提取的业务逻辑
type SQLTrustService struct {
	policyRepo  *airepo.SQLSecurityPolicyRepo
	historyRepo *airepo.SQLHistoryRepo
	auditRepo   *airepo.AIAuditLogRepository
}

// NewSQLTrustService 创建 SQLTrustService 实例
func NewSQLTrustService(
	policyRepo *airepo.SQLSecurityPolicyRepo,
	historyRepo *airepo.SQLHistoryRepo,
	auditRepo *airepo.AIAuditLogRepository,
) *SQLTrustService {
	return &SQLTrustService{
		policyRepo:  policyRepo,
		historyRepo: historyRepo,
		auditRepo:   auditRepo,
	}
}

// GetSQLSecurityPolicy 获取空间生效安全策略
func (s *SQLTrustService) GetSQLSecurityPolicy(spaceID uint64) (*aimodel.SQLSecurityPolicy, error) {
	policy, err := s.policyRepo.GetEffectivePolicy(spaceID)
	if err != nil {
		return nil, err
	}
	if policy == nil {
		policy = &aimodel.SQLSecurityPolicy{SpaceID: spaceID, MaxRows: 200, TimeoutSec: 15}
	}
	return policy, nil
}

// UpdateSQLSecurityPolicy 更新空间安全策略
func (s *SQLTrustService) UpdateSQLSecurityPolicy(req *aimodel.SQLSecurityPolicy) error {
	return s.policyRepo.CreateOrUpdateBySpace(req)
}

// SubmitSQLFeedbackRequest 提交 SQL 反馈请求参数
type SubmitSQLFeedbackRequest struct {
	HistoryID uint64 `json:"history_id" binding:"required"`
	Rating    int8   `json:"rating"`
	Comment   string `json:"comment"`
}

// SubmitSQLFeedback 提交 SQL 反馈
func (s *SQLTrustService) SubmitSQLFeedback(req *SubmitSQLFeedbackRequest) error {
	if err := s.historyRepo.UpdateRating(req.HistoryID, req.Rating); err != nil {
		return fmt.Errorf("保存评分失败: %w", err)
	}
	if err := s.historyRepo.UpdateFeedback(req.HistoryID, req.Comment); err != nil {
		return fmt.Errorf("保存反馈失败: %w", err)
	}
	return nil
}

// GetSQLExplanation 获取 SQL 解释
func (s *SQLTrustService) GetSQLExplanation(historyID uint64) (*aimodel.SQLExplanation, error) {
	history, err := s.historyRepo.FindByID(historyID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSQLHistoryQueryFailed, err)
	}
	if history == nil {
		return nil, ErrSQLHistoryNotFound
	}
	if history.ExplanationJSON == "" {
		return nil, nil
	}
	var explanation aimodel.SQLExplanation
	if err := json.Unmarshal([]byte(history.ExplanationJSON), &explanation); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSQLExplanationInvalid, err)
	}
	return &explanation, nil
}

// ListSQLAuditTraces 列出 SQL 审计追踪
func (s *SQLTrustService) ListSQLAuditTraces(limit, offset int) ([]*aimodel.AIAuditLog, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.auditRepo.FindByAction("text2sql", limit, offset)
}

// GetSQLAuditTrace 查询单条审计详情。
// 旧实现把记录主键当作分页 offset 传给 FindByAction("text2sql", 1, id)：
// 永远不会返回 id 对应的记录，而是返回「按时间排序的第 id+1 条」，管理员据此排障
// 会得到完全无关的内容。改为按主键直查。
func (s *SQLTrustService) GetSQLAuditTrace(id int) (*aimodel.AIAuditLog, error) {
	log, err := s.auditRepo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSQLAuditTraceNotFound, err)
	}
	return log, nil
}
