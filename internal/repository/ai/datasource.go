package ai

import (
	"time"

	aimodel "github.com/listen808/citebase/internal/model/ai"
	"gorm.io/gorm"
)

type DataSourceRepo struct {
	db *gorm.DB
}

func NewDataSourceRepo(db *gorm.DB) *DataSourceRepo {
	return &DataSourceRepo{db: db}
}

func (r *DataSourceRepo) Create(ds *aimodel.DataSource) error {
	return r.db.Create(ds).Error
}

func (r *DataSourceRepo) Update(ds *aimodel.DataSource) error {
	return r.db.Save(ds).Error
}

func (r *DataSourceRepo) Delete(id uint64) error {
	return r.db.Unscoped().Delete(&aimodel.DataSource{}, id).Error
}

func (r *DataSourceRepo) FindByID(id uint64) (*aimodel.DataSource, error) {
	var ds aimodel.DataSource
	err := r.db.Where("id = ?", id).First(&ds).Error
	return &ds, err
}

func (r *DataSourceRepo) FindBySpace(spaceID uint64) ([]aimodel.DataSource, error) {
	var list []aimodel.DataSource
	err := r.db.Where("space_id = ?", spaceID).Order("id DESC").Find(&list).Error
	return list, err
}

func (r *DataSourceRepo) FindActiveBySpace(spaceID uint64) ([]aimodel.DataSource, error) {
	var list []aimodel.DataSource
	err := r.db.Where("space_id = ? AND is_active = ?", spaceID, true).Order("id DESC").Find(&list).Error
	return list, err
}

func (r *DataSourceRepo) UpdateLastSync(id uint64) error {
	now := time.Now()
	return r.db.Model(&aimodel.DataSource{}).Where("id = ?", id).Updates(map[string]interface{}{
		"last_sync_at": now,
		"updated_at":   now,
	}).Error
}

// SetPassword 已删除：口令加密归 DataSourceService（写入前加密、读取时解密），
// 留一个"把外部给的字符串直接写进 password 列"的方法，等于留一条绕过加密的旁路。
// 它当初也是 0 调用方 —— 加密能力写好了但没人用它，这正是明文口令一直留在库里的原因。

type SQLHistoryRepo struct {
	db *gorm.DB
}

func NewSQLHistoryRepo(db *gorm.DB) *SQLHistoryRepo {
	return &SQLHistoryRepo{db: db}
}

func (r *SQLHistoryRepo) Create(h *aimodel.SQLHistory) error {
	return r.db.Create(h).Error
}

func (r *SQLHistoryRepo) Update(h *aimodel.SQLHistory) error {
	return r.db.Save(h).Error
}

func (r *SQLHistoryRepo) Delete(id uint64) error {
	return r.db.Unscoped().Delete(&aimodel.SQLHistory{}, id).Error
}

func (r *SQLHistoryRepo) FindByID(id uint64) (*aimodel.SQLHistory, error) {
	var h aimodel.SQLHistory
	err := r.db.Where("id = ?", id).First(&h).Error
	return &h, err
}

func (r *SQLHistoryRepo) FindByTraceID(traceID string) (*aimodel.SQLHistory, error) {
	var h aimodel.SQLHistory
	err := r.db.Where("trace_id = ?", traceID).First(&h).Error
	return &h, err
}

func (r *SQLHistoryRepo) List(spaceID, userID uint64, page, pageSize int) ([]aimodel.SQLHistory, int64, error) {
	var list []aimodel.SQLHistory
	var total int64

	query := r.db.Model(&aimodel.SQLHistory{})
	if spaceID > 0 {
		query = query.Where("space_id = ?", spaceID)
	}
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}

	query.Count(&total)
	err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (r *SQLHistoryRepo) ListFavorites(spaceID, userID uint64, page, pageSize int) ([]aimodel.SQLHistory, int64, error) {
	var list []aimodel.SQLHistory
	var total int64

	query := r.db.Model(&aimodel.SQLHistory{}).Where("is_favorite = ?", true)
	if spaceID > 0 {
		query = query.Where("space_id = ?", spaceID)
	}
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}

	query.Count(&total)
	err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

// ToggleFavorite 切换收藏。
// 旧实现写成 `Where("id = ?")` 却漏传参数：占位符没有对应值，任何调用都直接报错，
// 收藏功能从未真正生效。
func (r *SQLHistoryRepo) ToggleFavorite(id uint64) error {
	return r.db.Model(&aimodel.SQLHistory{}).Where("id = ?", id).
		Update("is_favorite", gorm.Expr("NOT is_favorite")).Error
}

func (r *SQLHistoryRepo) UpdateRating(id uint64, rating int8) error {
	return r.db.Model(&aimodel.SQLHistory{}).Where("id = ?", id).Update("rating", rating).Error
}

func (r *SQLHistoryRepo) UpdateFeedback(id uint64, feedback string) error {
	return r.db.Model(&aimodel.SQLHistory{}).Where("id = ?", id).Update("feedback", feedback).Error
}

func (r *SQLHistoryRepo) ClearByUser(userID uint64, olderThanDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -olderThanDays)
	result := r.db.Where("user_id = ? AND created_at < ?", userID, cutoff).Unscoped().Delete(&aimodel.SQLHistory{})
	return result.RowsAffected, result.Error
}

func (r *SQLHistoryRepo) ClearAll(olderThanDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -olderThanDays)
	result := r.db.Where("created_at < ?", cutoff).Unscoped().Delete(&aimodel.SQLHistory{})
	return result.RowsAffected, result.Error
}

type PromptTemplateRepo struct {
	db *gorm.DB
}

func NewPromptTemplateRepo(db *gorm.DB) *PromptTemplateRepo {
	return &PromptTemplateRepo{db: db}
}

func (r *PromptTemplateRepo) Create(tpl *aimodel.PromptTemplate) error {
	return r.db.Create(tpl).Error
}

func (r *PromptTemplateRepo) Update(tpl *aimodel.PromptTemplate) error {
	return r.db.Save(tpl).Error
}

func (r *PromptTemplateRepo) Delete(id uint64) error {
	return r.db.Unscoped().Delete(&aimodel.PromptTemplate{}, id).Error
}

func (r *PromptTemplateRepo) FindByID(id uint64) (*aimodel.PromptTemplate, error) {
	var tpl aimodel.PromptTemplate
	err := r.db.Where("id = ?", id).First(&tpl).Error
	return &tpl, err
}

func (r *PromptTemplateRepo) List(spaceID uint64, includeGlobal bool) ([]aimodel.PromptTemplate, error) {
	var list []aimodel.PromptTemplate
	query := r.db.Model(&aimodel.PromptTemplate{})

	if spaceID > 0 {
		if includeGlobal {
			query = query.Where("space_id = ? OR space_id IS NULL", spaceID)
		} else {
			query = query.Where("space_id = ?", spaceID)
		}
	} else {
		query = query.Where("space_id IS NULL")
	}

	err := query.Where("is_active = ?", true).Order("sort ASC, id ASC").Find(&list).Error
	return list, err
}

func (r *PromptTemplateRepo) FindByQueryType(spaceID uint64, queryType string) (*aimodel.PromptTemplate, error) {
	var tpl aimodel.PromptTemplate
	query := r.db.Model(&aimodel.PromptTemplate{})

	if spaceID > 0 {
		query = query.Where("(space_id = ? OR space_id IS NULL)", spaceID)
	} else {
		query = query.Where("space_id IS NULL")
	}

	err := query.Where("query_type = ? AND is_active = ?", queryType, true).Order("is_default DESC, sort ASC, id ASC").First(&tpl).Error
	return &tpl, err
}

func (r *PromptTemplateRepo) SetDefault(id uint64, spaceID uint64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if spaceID > 0 {
			if err := tx.Model(&aimodel.PromptTemplate{}).Where("space_id = ?", spaceID).Update("is_default", false).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Model(&aimodel.PromptTemplate{}).Where("space_id IS NULL").Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Model(&aimodel.PromptTemplate{}).Where("id = ?", id).Update("is_default", true).Error
	})
}

func (r *PromptTemplateRepo) ToggleActive(id uint64, active bool) error {
	return r.db.Model(&aimodel.PromptTemplate{}).Where("id = ?", id).Update("is_active", active).Error
}

func (r *PromptTemplateRepo) ListByTypeAndQueryType(templateType, queryType string) ([]aimodel.PromptTemplate, error) {
	var templates []aimodel.PromptTemplate
	query := r.db.Where("1 = 1")
	if templateType != "" {
		query = query.Where("type = ?", templateType)
	}
	if queryType != "" {
		query = query.Where("query_type = ?", queryType)
	}
	err := query.Order("is_active DESC, is_default DESC, id ASC").Find(&templates).Error
	return templates, err
}

func (r *PromptTemplateRepo) ListByQueryType(queryType string) ([]aimodel.PromptTemplate, error) {
	return r.ListByTypeAndQueryType("", queryType)
}
