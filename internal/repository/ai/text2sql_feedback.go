package ai

import (
	"time"

	aimodel "github.com/listen808/citebase/internal/model/ai"
	"gorm.io/gorm"
)

// Text2SQLFeedbackRepo Text2SQL 反馈记录仓储
type Text2SQLFeedbackRepo struct {
	db *gorm.DB
}

func NewText2SQLFeedbackRepo(db *gorm.DB) *Text2SQLFeedbackRepo {
	return &Text2SQLFeedbackRepo{db: db}
}

// Create 创建反馈记录
func (r *Text2SQLFeedbackRepo) Create(record *aimodel.Text2SQLFeedback) error {
	return r.db.Create(record).Error
}

// ListBySpace 按空间查询反馈记录
func (r *Text2SQLFeedbackRepo) ListBySpace(spaceID uint64, limit, offset int) ([]*aimodel.Text2SQLFeedback, error) {
	var records []*aimodel.Text2SQLFeedback
	err := r.db.Where("space_id = ?", spaceID).Order("created_at DESC").Limit(limit).Offset(offset).Find(&records).Error
	return records, err
}

// GetErrorPatternFrequency 统计指定空间内的错误类型频率
func (r *Text2SQLFeedbackRepo) GetErrorPatternFrequency(spaceID uint64, since time.Time) (map[string]int, error) {
	type result struct {
		ErrorType string
		Count     int
	}
	var rows []result
	query := r.db.Model(&aimodel.Text2SQLFeedback{}).
		Select("error_type, COUNT(*) as count").
		Where("execution_ok = ? AND error_type != ?", false, "")
	if spaceID > 0 {
		query = query.Where("space_id = ?", spaceID)
	}
	if !since.IsZero() {
		query = query.Where("created_at >= ?", since)
	}
	err := query.Group("error_type").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	freq := make(map[string]int)
	for _, row := range rows {
		freq[row.ErrorType] = row.Count
	}
	return freq, nil
}

// GetSuccessExamples 获取成功的查询-SQL 对，用于 Few-shot
func (r *Text2SQLFeedbackRepo) GetSuccessExamples(spaceID uint64, limit int) ([]*aimodel.Text2SQLFeedback, error) {
	var records []*aimodel.Text2SQLFeedback
	query := r.db.Where("execution_ok = ?", true)
	if spaceID > 0 {
		query = query.Where("space_id = ?", spaceID)
	}
	err := query.Order("created_at DESC").Limit(limit).Find(&records).Error
	return records, err
}

// ErrorSummary 返回错误类型分布
func (r *Text2SQLFeedbackRepo) ErrorSummary(spaceID uint64) (map[string]int64, error) {
	var rows []struct {
		ErrorType string `gorm:"column:error_type"`
		Count     int64  `gorm:"column:count"`
	}
	err := r.db.Model(&aimodel.Text2SQLFeedback{}).
		Select("error_type, COUNT(*) as count").
		Where("space_id = ? AND execution_ok = ?", spaceID, false).
		Group("error_type").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	summary := make(map[string]int64)
	for _, r := range rows {
		if r.ErrorType == "" {
			r.ErrorType = "unknown"
		}
		summary[r.ErrorType] = r.Count
	}
	return summary, nil
}

// RecentFailures 返回最近的失败记录
func (r *Text2SQLFeedbackRepo) RecentFailures(spaceID uint64, limit int) ([]aimodel.Text2SQLFeedback, error) {
	var list []aimodel.Text2SQLFeedback
	err := r.db.Where("space_id = ? AND execution_ok = ?", spaceID, false).
		Order("created_at DESC").
		Limit(limit).
		Find(&list).Error
	return list, err
}
