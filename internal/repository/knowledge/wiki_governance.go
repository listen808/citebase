package knowledge

import (
	"time"

	"gorm.io/gorm"

	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	repos "github.com/listen808/citebase/internal/repository"
)

type WikiPinRepository struct {
	db *gorm.DB
}

func NewWikiPinRepository() *WikiPinRepository {
	return &WikiPinRepository{db: repos.DB}
}

func (r *WikiPinRepository) Create(pin *knowledgemodel.WikiPin) error {
	return r.db.Create(pin).Error
}

// ListActiveByPage 列出页面生效中的 pin，重编译后逐条复核。
func (r *WikiPinRepository) ListActiveByPage(spaceID int, slug string) ([]*knowledgemodel.WikiPin, error) {
	var pins []*knowledgemodel.WikiPin
	err := r.db.Where("space_id = ? AND page_slug = ? AND status IN ?", spaceID, slug,
		[]string{string(knowledgemodel.WikiPinStatusActive), string(knowledgemodel.WikiPinStatusConflict)}).
		Order("id ASC").Find(&pins).Error
	return pins, err
}

func (r *WikiPinRepository) ListBySpace(spaceID int, status string) ([]*knowledgemodel.WikiPin, error) {
	var pins []*knowledgemodel.WikiPin
	q := r.db.Where("space_id = ?", spaceID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Order("id DESC").Find(&pins).Error
	return pins, err
}

func (r *WikiPinRepository) UpdateStatus(id int, status string) error {
	return r.db.Model(&knowledgemodel.WikiPin{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": status, "updated_at": time.Now()}).Error
}

func (r *WikiPinRepository) Delete(id int) error {
	return r.db.Where("id = ?", id).Delete(&knowledgemodel.WikiPin{}).Error
}

type WikiReviewRepository struct {
	db *gorm.DB
}

func NewWikiReviewRepository() *WikiReviewRepository {
	return &WikiReviewRepository{db: repos.DB}
}

func (r *WikiReviewRepository) Create(item *knowledgemodel.WikiReview) error {
	return r.db.Create(item).Error
}

func (r *WikiReviewRepository) ListBySpace(spaceID int, status string) ([]*knowledgemodel.WikiReview, error) {
	var items []*knowledgemodel.WikiReview
	q := r.db.Where("space_id = ?", spaceID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Order("id DESC").Find(&items).Error
	return items, err
}

func (r *WikiReviewRepository) Resolve(id int, status string, resolvedBy int) error {
	return r.db.Model(&knowledgemodel.WikiReview{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      status,
			"resolved_by": resolvedBy,
			"updated_at":  time.Now(),
		}).Error
}

// HasPending 判断页面是否已有待审核项，避免同一草稿重复入队。
func (r *WikiReviewRepository) HasPending(spaceID int, slug string) (bool, error) {
	var n int64
	err := r.db.Model(&knowledgemodel.WikiReview{}).
		Where("space_id = ? AND page_slug = ? AND status = ?", spaceID, slug, knowledgemodel.WikiReviewStatusPending).
		Count(&n).Error
	return n > 0, err
}

type WikiLintRepository struct {
	db *gorm.DB
}

func NewWikiLintRepository() *WikiLintRepository {
	return &WikiLintRepository{db: repos.DB}
}

func (r *WikiLintRepository) Create(issue *knowledgemodel.WikiLintIssue) error {
	return r.db.Create(issue).Error
}

// ReplaceOpen 用本轮体检结果替换该空间所有未处理问题（每轮全量重算，避免累积陈旧项）。
func (r *WikiLintRepository) ReplaceOpen(spaceID int, issues []*knowledgemodel.WikiLintIssue) error {
	if err := r.db.Where("space_id = ? AND status = 'open'", spaceID).
		Delete(&knowledgemodel.WikiLintIssue{}).Error; err != nil {
		return err
	}
	if len(issues) == 0 {
		return nil
	}
	return r.db.CreateInBatches(issues, 100).Error
}

func (r *WikiLintRepository) ListBySpace(spaceID int, status string) ([]*knowledgemodel.WikiLintIssue, error) {
	var items []*knowledgemodel.WikiLintIssue
	q := r.db.Where("space_id = ?", spaceID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Order("id DESC").Find(&items).Error
	return items, err
}

// OpenBySlug 取某页当前未处理的体检问题（写入时矛盾标注用）。
func (r *WikiLintRepository) OpenBySlug(spaceID int, slug string) ([]*knowledgemodel.WikiLintIssue, error) {
	var items []*knowledgemodel.WikiLintIssue
	err := r.db.Where("space_id = ? AND page_slug = ? AND status = 'open'", spaceID, slug).
		Order("id DESC").Find(&items).Error
	return items, err
}

func (r *WikiLintRepository) Resolve(id int) error {
	return r.db.Model(&knowledgemodel.WikiLintIssue{}).Where("id = ?", id).
		Update("status", "resolved").Error
}
