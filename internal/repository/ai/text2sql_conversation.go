package ai

import (
	"fmt"
	"time"

	aimodel "github.com/listen808/citebase/internal/model/ai"
	"gorm.io/gorm"
)

// Text2SQLConversationRepository Text2SQL 对话轮次仓储
type Text2SQLConversationRepository struct {
	db *gorm.DB
}

func NewText2SQLConversationRepository(db *gorm.DB) *Text2SQLConversationRepository {
	return &Text2SQLConversationRepository{db: db}
}

// CreateTurn 创建对话轮次
func (r *Text2SQLConversationRepository) CreateTurn(turn *aimodel.SQLConversationTurn) error {
	return r.db.Create(turn).Error
}

// GetTurnByID 根据ID获取轮次
func (r *Text2SQLConversationRepository) FindByID(id uint64) (*aimodel.SQLConversationTurn, error) {
	var turn aimodel.SQLConversationTurn
	err := r.db.Where("id = ?", id).First(&turn).Error
	if err != nil {
		return nil, err
	}
	return &turn, nil
}

// ListTurnsByConversationID 列出对话的所有轮次
func (r *Text2SQLConversationRepository) ListTurnsByConversationID(conversationID uint64) ([]*aimodel.SQLConversationTurn, error) {
	var turns []*aimodel.SQLConversationTurn
	err := r.db.Where("conversation_id = ?", conversationID).Order("turn_number ASC").Find(&turns).Error
	return turns, err
}

// UpdateTurn 更新轮次
func (r *Text2SQLConversationRepository) UpdateTurn(turn *aimodel.SQLConversationTurn) error {
	return r.db.Save(turn).Error
}

// GetOrCreateSummary 获取或创建摘要
func (r *Text2SQLConversationRepository) GetOrCreateSummary(conversationID uint64) (*aimodel.SQLConversationSummary, error) {
	var summary aimodel.SQLConversationSummary
	err := r.db.Where("conversation_id = ?", conversationID).First(&summary).Error
	if err == nil {
		return &summary, nil
	}
	if err == gorm.ErrRecordNotFound {
		summary = aimodel.SQLConversationSummary{
			ConversationID: conversationID,
		}
		if err := r.db.Create(&summary).Error; err != nil {
			return nil, err
		}
		return &summary, nil
	}
	return nil, err
}

// UpdateSummary 更新摘要
func (r *Text2SQLConversationRepository) UpdateSummary(summary *aimodel.SQLConversationSummary) error {
	return r.db.Save(summary).Error
}

// SaveEmbedding 保存轮次向量
func (r *Text2SQLConversationRepository) SaveEmbedding(emb *aimodel.SQLTurnEmbedding) error {
	return r.db.Create(emb).Error
}

// FindSimilarTurns 查找相似轮次
func (r *Text2SQLConversationRepository) FindSimilarTurns(userID uint64, query string, limit int) ([]*aimodel.SQLConversationTurn, error) {
	var turns []*aimodel.SQLConversationTurn
	err := r.db.Where("user_id = ? AND match(combined_text) against(? in boolean mode) > 0",
		userID, query).Limit(limit).Find(&turns).Error
	if err != nil {
		return nil, err
	}
	return turns, nil
}

// GetOrCreateSession 获取或创建会话
func (r *Text2SQLConversationRepository) GetOrCreateSession(userID, conversationID uint64) (*aimodel.SQLConversationSession, error) {
	var session aimodel.SQLConversationSession
	sessionKey := fmt.Sprintf("%d_%d", userID, conversationID)
	err := r.db.Where("session_key = ?", sessionKey).First(&session).Error
	if err == nil {
		return &session, nil
	}
	if err == gorm.ErrRecordNotFound {
		session = aimodel.SQLConversationSession{
			SessionKey:     sessionKey,
			UserID:         userID,
			ConversationID: conversationID,
			IsActive:       true,
			LastAccessAt:   time.Now(),
		}
		if err := r.db.Create(&session).Error; err != nil {
			return nil, err
		}
		return &session, nil
	}
	return nil, err
}

// UpdateSession 更新会话
func (r *Text2SQLConversationRepository) UpdateSession(session *aimodel.SQLConversationSession) error {
	session.LastAccessAt = time.Now()
	return r.db.Save(session).Error
}

// UpdateTurnEmbeddingID 更新轮次的 embeddingID
func (r *Text2SQLConversationRepository) UpdateTurnEmbeddingID(turnID uint64, embeddingID string) error {
	return r.db.Model(&aimodel.SQLConversationTurn{}).Where("id = ?", turnID).Update("embedding_id", embeddingID).Error
}

// GetTurnsByConversationLimit 按限制获取轮次
func (r *Text2SQLConversationRepository) GetTurnsByConversationLimit(conversationID uint64, limit int) ([]*aimodel.SQLConversationTurn, error) {
	var turns []*aimodel.SQLConversationTurn
	err := r.db.Where("conversation_id = ?", conversationID).Order("turn_number DESC").Limit(limit).Find(&turns).Error
	return turns, err
}

// UpdateTurnExecutionResult 更新执行结果
func (r *Text2SQLConversationRepository) UpdateTurnExecutionResult(turnID uint64, result string, isSuccess bool, errMsg string) error {
	updates := map[string]interface{}{
		"execution_result": result,
		"is_success":       isSuccess,
		"execution_error":  errMsg,
		"updated_at":       time.Now(),
	}
	return r.db.Model(&aimodel.SQLConversationTurn{}).Where("id = ?", turnID).Updates(updates).Error
}

// UpdateTurnFeedback 更新反馈评分
func (r *Text2SQLConversationRepository) UpdateTurnFeedback(turnID uint64, rating int8) error {
	return r.db.Model(&aimodel.SQLConversationTurn{}).Where("id = ?", turnID).Update("rating", rating).Error
}

// CreateOrUpdateSummary 创建或更新摘要
func (r *Text2SQLConversationRepository) CreateOrUpdateSummary(summary *aimodel.SQLConversationSummary) error {
	var existing aimodel.SQLConversationSummary
	err := r.db.Where("conversation_id = ?", summary.ConversationID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.Create(summary).Error
	}
	if err != nil {
		return err
	}
	summary.ID = existing.ID
	summary.CreatedAt = existing.CreatedAt
	return r.db.Save(summary).Error
}

// GetSummaryByConversation 获取摘要
func (r *Text2SQLConversationRepository) GetSummaryByConversation(conversationID uint64) (*aimodel.SQLConversationSummary, error) {
	var summary aimodel.SQLConversationSummary
	err := r.db.Where("conversation_id = ?", conversationID).First(&summary).Error
	if err != nil {
		return nil, err
	}
	return &summary, nil
}

// GetSessionByKey 按 key 获取会话
func (r *Text2SQLConversationRepository) GetSessionByKey(sessionKey string) (*aimodel.SQLConversationSession, error) {
	var session aimodel.SQLConversationSession
	err := r.db.Where("session_key = ?", sessionKey).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// CreateSession 创建会话
func (r *Text2SQLConversationRepository) CreateSession(session *aimodel.SQLConversationSession) error {
	return r.db.Create(session).Error
}

// UpdateSessionState 更新会话状态
func (r *Text2SQLConversationRepository) UpdateSessionState(sessionKey string, stateJSON string) error {
	return r.db.Model(&aimodel.SQLConversationSession{}).Where("session_key = ?", sessionKey).Updates(map[string]interface{}{
		"state_json":     stateJSON,
		"last_access_at": time.Now(),
	}).Error
}

// DeactivateSession 停用会话
func (r *Text2SQLConversationRepository) DeactivateSession(sessionKey string) error {
	return r.db.Model(&aimodel.SQLConversationSession{}).Where("session_key = ?", sessionKey).Update("is_active", false).Error
}

// DeleteExpiredSessions 删除过期会话
func (r *Text2SQLConversationRepository) DeleteExpiredSessions(ttlHours int) error {
	cutoffTime := time.Now().Add(-time.Duration(ttlHours) * time.Hour)
	return r.db.Where("last_access_at < ? AND is_active = ?", cutoffTime, false).Delete(&aimodel.SQLConversationSession{}).Error
}

// GetUserConversationStats 获取用户对话统计
func (r *Text2SQLConversationRepository) GetUserConversationStats(userID uint64) (totalTurns, successTurns, totalConvs int64, err error) {
	var total int64
	if err := r.db.Model(&aimodel.SQLConversationTurn{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return 0, 0, 0, err
	}

	var success int64
	if err := r.db.Model(&aimodel.SQLConversationTurn{}).Where("user_id = ? AND is_success = ?", userID, true).Count(&success).Error; err != nil {
		return total, 0, 0, err
	}

	var convs int64
	if err := r.db.Model(&aimodel.SQLConversationSession{}).Where("user_id = ?", userID).Count(&convs).Error; err != nil {
		return total, success, 0, err
	}

	return total, success, convs, nil
}

// GetRecentTurns 获取最近轮次
func (r *Text2SQLConversationRepository) GetRecentTurns(userID uint64, limit int) ([]*aimodel.SQLConversationTurn, error) {
	var turns []*aimodel.SQLConversationTurn
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(limit).Find(&turns).Error
	return turns, err
}

// GetMaxTurnNumber 获取对话的最大轮次号
func (r *Text2SQLConversationRepository) GetMaxTurnNumber(conversationID uint64) (int, error) {
	var turn aimodel.SQLConversationTurn
	err := r.db.Where("conversation_id = ?", conversationID).Order("turn_number DESC").First(&turn).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return turn.TurnNumber, nil
}
