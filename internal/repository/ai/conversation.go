package ai

import (
	aimodel "github.com/listen808/citebase/internal/model/ai"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

type AIConversationRepository struct {
	db *gorm.DB
}

func NewAIConversationRepository() *AIConversationRepository {
	return &AIConversationRepository{db: repos.DB}
}

func (r *AIConversationRepository) Create(conv *aimodel.AIConversation) error {
	return r.db.Create(conv).Error
}

func (r *AIConversationRepository) Update(conv *aimodel.AIConversation) error {
	// 关键：使用 Updates + map 避免 GORM Save 的全字段覆盖问题
	// Save 在主键存在时会执行 UPDATE all fields，但若 conv 是从 DB 读出的对象再传回，
	// 会覆盖其他字段。对 messages 这种 json 列尤其危险（双重序列化风险）
	updates := map[string]interface{}{
		"title":       conv.Title,
		"messages":    conv.Messages,
		"token_usage": conv.TokenUsage,
		"space_id":    conv.SpaceID,
	}
	return r.db.Model(&aimodel.AIConversation{}).Where("id = ?", conv.ID).Updates(updates).Error
}

func (r *AIConversationRepository) FindByID(id int) (*aimodel.AIConversation, error) {
	var conv aimodel.AIConversation
	err := r.db.Where("id = ?", id).First(&conv).Error
	if err != nil {
		return nil, err
	}
	return &conv, nil
}

func (r *AIConversationRepository) FindByUserID(userID int, limit, offset int) ([]*aimodel.AIConversation, int64, error) {
	var convs []*aimodel.AIConversation
	var total int64
	if err := r.db.Model(&aimodel.AIConversation{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := r.db.Where("user_id = ?", userID).Order("updated_at DESC").Limit(limit).Offset(offset).Find(&convs).Error
	return convs, total, err
}

func (r *AIConversationRepository) FindByUserIDAndType(userID int, convType string, limit, offset int) ([]*aimodel.AIConversation, int64, error) {
	var convs []*aimodel.AIConversation
	var total int64

	var query *gorm.DB
	if convType == "sql" {
		query = r.db.Model(&aimodel.AIConversation{}).
			Where("user_id = ? AND (type = ? OR (type = 'ai' AND title LIKE '[SQL] %'))", userID, convType)
	} else if convType == "ai" {
		query = r.db.Model(&aimodel.AIConversation{}).
			Where("user_id = ? AND type = ? AND (title NOT LIKE '[SQL] %' OR title IS NULL)", userID, convType)
	} else {
		query = r.db.Model(&aimodel.AIConversation{}).Where("user_id = ? AND type = ?", userID, convType)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("updated_at DESC").Limit(limit).Offset(offset).Find(&convs).Error
	return convs, total, err
}

func (r *AIConversationRepository) Delete(id int) error {
	return r.db.Delete(&aimodel.AIConversation{}, id).Error
}

func (r *AIConversationRepository) AddMessage(id int, msg aimodel.ConvMsgs, tokenUsage int) error {
	return r.db.Model(&aimodel.AIConversation{}).Where("id = ?", id).Updates(map[string]interface{}{
		"messages":    msg,
		"token_usage": gorm.Expr("token_usage + ?", tokenUsage),
	}).Error
}
