package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"time"

	aimodel "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	"github.com/listen808/citebase/internal/text2sql"
)

// SQLHistoryService SQL 历史记录业务逻辑层
type SQLHistoryService struct {
	repo *airepo.SQLHistoryRepo
}

// NewSQLHistoryService 创建 SQLHistoryService 实例
func NewSQLHistoryService(repo *airepo.SQLHistoryRepo) *SQLHistoryService {
	return &SQLHistoryService{repo: repo}
}

// ErrSQLHistoryForbidden 表示调用者不是该条 SQL 历史的所有者。
var ErrSQLHistoryForbidden = errors.New("无权操作他人的 SQL 历史")

// ErrSQLHistoryNotFound 同 sql_trust_service.go 顶部那条理由：句子交给表，这里只管日志。
var ErrSQLHistoryNotFound = errors.New("sql history record not found")

// requireOwner 校验记录归属。SQL 历史是「谁问的、问了什么、跑了哪条 SQL」的私密轨迹，
// 此前 service 层只按 id 取记录、完全不校验调用者，任何登录用户都能读取与删除他人记录。
func (s *SQLHistoryService) requireOwner(id, actorUserID uint64) error {
	if actorUserID == 0 {
		return ErrSQLHistoryForbidden
	}
	h, err := s.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSQLHistoryNotFound, err)
	}
	if h.UserID != actorUserID {
		return ErrSQLHistoryForbidden
	}
	return nil
}

// List 列出当前用户自己的 SQL 历史；actorUserID 为 0 时不返回任何数据（fail-closed）。
func (s *SQLHistoryService) List(spaceID, actorUserID uint64, page, pageSize int) ([]aimodel.SQLHistory, int64, error) {
	if actorUserID == 0 {
		return []aimodel.SQLHistory{}, 0, nil
	}
	return s.repo.List(spaceID, actorUserID, page, pageSize)
}

// ListFavorites 列出当前用户自己收藏的 SQL。
func (s *SQLHistoryService) ListFavorites(spaceID, actorUserID uint64, page, pageSize int) ([]aimodel.SQLHistory, int64, error) {
	if actorUserID == 0 {
		return []aimodel.SQLHistory{}, 0, nil
	}
	return s.repo.ListFavorites(spaceID, actorUserID, page, pageSize)
}

// ToggleFavorite 切换收藏状态（仅限本人记录）
func (s *SQLHistoryService) ToggleFavorite(id, actorUserID uint64) error {
	if err := s.requireOwner(id, actorUserID); err != nil {
		return err
	}
	return s.repo.ToggleFavorite(id)
}

// Rate 评分（仅限本人记录）
func (s *SQLHistoryService) Rate(id, actorUserID uint64, rating int8) error {
	if err := s.requireOwner(id, actorUserID); err != nil {
		return err
	}
	return s.repo.UpdateRating(id, rating)
}

// SaveRequest 保存请求
type SaveSQLHistoryRequest struct {
	SpaceID      uint64 `json:"space_id" binding:"required"`
	QueryText    string `json:"query_text" binding:"required"`
	GeneratedSQL string `json:"generated_sql"`
	ExecutedSQL  string `json:"executed_sql"`
	TablesUsed   string `json:"tables_used"`
	DurationMs   int    `json:"duration_ms"`
	Status       string `json:"status"`
	Feedback     string `json:"feedback"`
}

// Save 保存 SQL 历史
func (s *SQLHistoryService) Save(userID uint64, req *SaveSQLHistoryRequest) (*aimodel.SQLHistory, error) {
	h := &aimodel.SQLHistory{
		SpaceID:      req.SpaceID,
		UserID:       userID,
		QueryText:    req.QueryText,
		GeneratedSQL: req.GeneratedSQL,
		ExecutedSQL:  req.ExecutedSQL,
		TablesUsed:   req.TablesUsed,
		DurationMs:   req.DurationMs,
		Status:       req.Status,
		Feedback:     req.Feedback,
	}
	if h.Status == "" {
		h.Status = "success"
	}
	if err := s.repo.Create(h); err != nil {
		return nil, err
	}
	return h, nil
}

// Delete 删除（仅限本人记录）
func (s *SQLHistoryService) Delete(id, actorUserID uint64) error {
	if err := s.requireOwner(id, actorUserID); err != nil {
		return err
	}
	return s.repo.Delete(id)
}

// Get 获取详情（仅限本人记录）
func (s *SQLHistoryService) Get(id, actorUserID uint64) (*aimodel.SQLHistory, error) {
	if err := s.requireOwner(id, actorUserID); err != nil {
		return nil, err
	}
	h, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSQLHistoryNotFound, err)
	}
	return h, nil
}

// ClearResult 清除结果
type ClearResult struct {
	Message      string `json:"message"`
	DeletedCount int64  `json:"deleted_count"`
}

// Clear 清除 SQL 历史
func (s *SQLHistoryService) Clear(userID uint64, olderThanDays int, clearAll bool, isAdmin bool) (*ClearResult, error) {
	var deleted int64
	var err error
	if clearAll && isAdmin {
		deleted, err = s.repo.ClearAll(olderThanDays)
	} else {
		deleted, err = s.repo.ClearByUser(userID, olderThanDays)
	}
	if err != nil {
		return nil, err
	}
	return &ClearResult{Message: "清除成功", DeletedCount: deleted}, nil
}

// RecordSQLHistoryFromResult 从 SQL 生成结果记录历史
func (s *SQLHistoryService) RecordSQLHistoryFromResult(userID int, spaceID uint64, question string, result *aimodel.SQLResult) uint64 {
	if result == nil {
		return 0
	}

	tablesUsed := ""
	if len(result.Tables) > 0 {
		b, _ := json.Marshal(result.Tables)
		tablesUsed = string(b)
	}

	explanationJSON := ""
	if result.Explanation != nil {
		b, _ := json.Marshal(result.Explanation)
		explanationJSON = string(b)
	}

	history := &aimodel.SQLHistory{
		SpaceID:         spaceID,
		UserID:          uint64(userID),
		QueryText:       question,
		GeneratedSQL:    result.SQL,
		TablesUsed:      tablesUsed,
		ExplanationJSON: explanationJSON,
		TraceID:         result.TraceID,
		GeneratorTier:   result.GeneratorTier,
		Status:          "success",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := s.repo.Create(history); err != nil {
		logx.Infof("[SQLHistoryService] RecordSQLHistoryFromResult failed: %v", err)
		return 0
	}
	return history.ID
}

// UpdateSQLHistoryExecution 更新 SQL 执行结果
func (s *SQLHistoryService) UpdateSQLHistoryExecution(traceID, sqlStr string, result *text2sql.ExecResult) {
	if result == nil || traceID == "" {
		return
	}

	history, err := s.repo.FindByTraceID(traceID)
	if err != nil || history == nil {
		return
	}

	executionResult := ""
	if result.Success {
		b, _ := json.Marshal(map[string]interface{}{
			"columns":   result.Columns,
			"rows":      result.Rows,
			"row_count": result.RowCount,
		})
		executionResult = string(b)
	}

	executionError := ""
	if !result.Success {
		executionError = result.Error
		history.Status = "failed"
	}

	history.ExecutedSQL = sqlStr
	history.ExecutionResult = executionResult
	history.ExecutionError = executionError
	history.ExecutionTimeMs = result.Duration
	history.UpdatedAt = time.Now()

	if err := s.repo.Update(history); err != nil {
		logx.Infof("[SQLHistoryService] UpdateSQLHistoryExecution failed: %v", err)
	}
}
