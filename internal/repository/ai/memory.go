package ai

import (
	"errors"
	"time"

	aimodel "github.com/listen808/citebase/internal/model/ai"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

type AIMemoryRepository struct {
	db *gorm.DB
}

// DB 返回底层 *gorm.DB（供 Service 层事务使用）
func (r *AIMemoryRepository) DB() *gorm.DB {
	return r.db
}

func NewAIMemoryRepository() *AIMemoryRepository {
	return &AIMemoryRepository{db: repos.DB}
}

// Create 创建记忆
func (r *AIMemoryRepository) Create(mem *aimodel.AIMemory) error {
	return r.db.Create(mem).Error
}

// FindByUserID 按用户查找记忆
func (r *AIMemoryRepository) FindByUserID(userID uint, memoryType string, limit, offset int) ([]*aimodel.AIMemory, int64, error) {
	var mems []*aimodel.AIMemory
	var total int64
	query := r.db.Model(&aimodel.AIMemory{}).Where("user_id = ?", userID)
	if memoryType != "" {
		query = query.Where("memory_type = ?", memoryType)
	}
	// 排除已过期
	query = query.Where("expires_at IS NULL OR expires_at > ?", time.Now())
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("importance DESC, updated_at DESC").Limit(limit).Offset(offset).Find(&mems).Error
	return mems, total, err
}

// SearchByContent 基于内容关键词搜索。
// P2-8 全量待确认：召回只取 confirmed 的记忆——向量召回与关键词召回都经此函数，
// 在此单点过滤即挡住两条路径，pending 的记忆不进 prompt。
func (r *AIMemoryRepository) SearchByContent(userID uint, keyword string, memoryType string, limit int) ([]*aimodel.AIMemory, error) {
	var mems []*aimodel.AIMemory
	query := r.db.Where("user_id = ?", userID).
		Where("status = ?", aimodel.MemoryStatusConfirmed).
		Where("(content LIKE ? OR summary LIKE ?)", "%"+keyword+"%", "%"+keyword+"%")
	if memoryType != "" {
		query = query.Where("memory_type = ?", memoryType)
	}
	query = query.Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		Order("importance DESC, access_count DESC").
		Limit(limit)
	err := query.Find(&mems).Error
	return mems, err
}

// ConfirmMemory 把待确认记忆标记为已确认（P2-8）。只改 status，不碰其他字段。
func (r *AIMemoryRepository) ConfirmMemory(id uint, userID uint) error {
	return r.db.Model(&aimodel.AIMemory{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("status", aimodel.MemoryStatusConfirmed).Error
}

// FindPendingByUserID 列出某用户待确认的记忆（按重要性+时间排序）。
func (r *AIMemoryRepository) FindPendingByUserID(userID uint, limit int) ([]*aimodel.AIMemory, error) {
	var mems []*aimodel.AIMemory
	if limit <= 0 {
		limit = 50
	}
	err := r.db.Where("user_id = ? AND status = ?", userID, aimodel.MemoryStatusPending).
		Order("importance DESC, created_at DESC").
		Limit(limit).
		Find(&mems).Error
	return mems, err
}

// FindRecent 获取最近的记忆
func (r *AIMemoryRepository) FindRecent(userID uint, limit int) ([]*aimodel.AIMemory, error) {
	var mems []*aimodel.AIMemory
	err := r.db.Where("user_id = ?", userID).
		Where("expires_at IS NULL OR expires_at > ?", time.Now()).
		Order("updated_at DESC").
		Limit(limit).
		Find(&mems).Error
	return mems, err
}

// IncrementAccess 增加访问计数
func (r *AIMemoryRepository) IncrementAccess(id uint) error {
	return r.db.Model(&aimodel.AIMemory{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"access_count":     gorm.Expr("access_count + 1"),
			"updated_at":       time.Now(),
			"last_accessed_at": time.Now(),
		}).Error
}

// TouchOnRecall P1-7: 召回时更新访问计数与最后访问时间（不触碰 UpdatedAt）
func (r *AIMemoryRepository) TouchOnRecall(id uint) error {
	now := time.Now()
	return r.db.Model(&aimodel.AIMemory{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"access_count":     gorm.Expr("access_count + 1"),
			"last_accessed_at": now,
		}).Error
}

// PromoteToLongTerm P1-7 §3.2.2: 将 Importance >= threshold 的短期记忆升级为长期
// 返回受影响行数
func (r *AIMemoryRepository) PromoteToLongTerm(userID uint, threshold float64) (int64, error) {
	result := r.db.Model(&aimodel.AIMemory{}).
		Where("user_id = ? AND memory_tier = ? AND importance >= ?",
			userID, aimodel.MemoryTierShort, threshold).
		Update("memory_tier", aimodel.MemoryTierLong)
	return result.RowsAffected, result.Error
}

// ExpireStaleMemories P1-7 §3.2.2: 记忆分层 TTL 过期
//   - 长期记忆超过 staleThreshold（默认 90 天）未命中 → 降级为 stale
//   - stale 记忆超过 deleteThreshold（默认 180 天）未命中 → 物理删除
//
// 返回 (降级数, 删除数)
func (r *AIMemoryRepository) ExpireStaleMemories(staleThreshold, deleteThreshold time.Duration) (int64, int64, error) {
	staleCutoff := time.Now().Add(-staleThreshold)
	deleteCutoff := time.Now().Add(-deleteThreshold)

	// 1. long → stale（LastAccessedAt 为空时回退到 UpdatedAt）
	staleResult := r.db.Model(&aimodel.AIMemory{}).
		Where("memory_tier = ? AND (last_accessed_at IS NULL OR last_accessed_at < ?) AND updated_at < ?",
			aimodel.MemoryTierLong, staleCutoff, staleCutoff).
		Update("memory_tier", aimodel.MemoryTierStale)

	// 2. stale → 删除（超过 deleteThreshold）
	deleteResult := r.db.Where("memory_tier = ? AND (last_accessed_at IS NULL OR last_accessed_at < ?) AND updated_at < ?",
		aimodel.MemoryTierStale, deleteCutoff, deleteCutoff).
		Delete(&aimodel.AIMemory{})

	return staleResult.RowsAffected, deleteResult.RowsAffected, errors.Join(staleResult.Error, deleteResult.Error)
}

// Delete 删除记忆
func (r *AIMemoryRepository) Delete(id uint) error {
	return r.db.Delete(&aimodel.AIMemory{}, id).Error
}

// DeleteByUserID 删除用户所有记忆
func (r *AIMemoryRepository) DeleteByUserID(userID uint) error {
	return r.db.Where("user_id = ?", userID).Delete(&aimodel.AIMemory{}).Error
}

// DeleteExpired 清理过期记忆
func (r *AIMemoryRepository) DeleteExpired() (int64, error) {
	result := r.db.Where("expires_at IS NOT NULL AND expires_at <= ?", time.Now()).
		Delete(&aimodel.AIMemory{})
	return result.RowsAffected, result.Error
}

// ===== 会话摘要 =====

func (r *AIMemoryRepository) CreateSummary(sum *aimodel.AIMemorySummary) error {
	return r.db.Create(sum).Error
}

func (r *AIMemoryRepository) UpdateSummary(sum *aimodel.AIMemorySummary) error {
	return r.db.Save(sum).Error
}

// FindSummaryByConvID 按 (会话, 用户) 联合定位摘要。
// conv_id 由客户端提供，旧实现只按 conv_id 过滤：任何用户传他人的 conversation_id
// 即可命中对方记录并整行覆写（含 UserID 被改写）。必须同时约束 user_id。
func (r *AIMemoryRepository) FindSummaryByConvID(convID uint, userID uint) (*aimodel.AIMemorySummary, error) {
	var sum aimodel.AIMemorySummary
	err := r.db.Where("conv_id = ? AND user_id = ?", convID, userID).First(&sum).Error
	if err != nil {
		return nil, err
	}
	return &sum, nil
}

func (r *AIMemoryRepository) FindSummariesByUserID(userID uint, limit int) ([]*aimodel.AIMemorySummary, error) {
	var sums []*aimodel.AIMemorySummary
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&sums).Error
	return sums, err
}

func (r *AIMemoryRepository) DeleteSummaryByConvID(convID uint) error {
	return r.db.Where("conv_id = ?", convID).Delete(&aimodel.AIMemorySummary{}).Error
}
