package ai

import (
	aimodel "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
)

// AuditService AI 审计日志业务逻辑层
type AuditService struct {
	repo *airepo.AIAuditLogRepository
}

// NewAuditService 创建 AuditService 实例
func NewAuditService(repo *airepo.AIAuditLogRepository) *AuditService {
	return &AuditService{repo: repo}
}

// ListAuditLogs 列出审计日志
func (s *AuditService) ListAuditLogs(action string, limit, offset int, userID int, isAdmin bool) ([]*aimodel.AIAuditLog, int64, error) {
	// fail-closed：非管理员且无有效身份时不得返回数据。
	// 旧兜底分支会 FindRecent(limit) 把全站审计日志（含他人提问/回答）发给无身份请求。
	if !isAdmin && userID <= 0 {
		return []*aimodel.AIAuditLog{}, 0, nil
	}

	if isAdmin {
		if action != "" {
			return s.repo.FindByAction(action, limit, offset)
		}
		return s.repo.FindAll(limit, offset)
	}

	// 非管理员 + 指定 action：action 必须下推为 SQL 条件。
	// 旧实现先按用户取一页再内存里逐条比对，total 却是该用户全部动作的计数，
	// 前端按 total 算页数时每页条数远小于 limit、翻页出现空洞与重复。
	if action != "" {
		return s.repo.FindByUserIDAndAction(userID, action, limit, offset)
	}
	return s.repo.FindByUserID(userID, limit, offset)
}
