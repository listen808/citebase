package ai

import (
	aimodel "github.com/listen808/citebase/internal/model/ai"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

// BusinessGlossaryRepo 业务术语映射仓库
type BusinessGlossaryRepo struct {
	db *gorm.DB
}

func NewBusinessGlossaryRepo() *BusinessGlossaryRepo {
	return &BusinessGlossaryRepo{db: repos.DB}
}

// ListBySpace 获取指定空间的全部术语映射
func (r *BusinessGlossaryRepo) ListBySpace(spaceID int) ([]*aimodel.BusinessGlossary, error) {
	var items []*aimodel.BusinessGlossary
	err := r.db.Where("space_id = ?", spaceID).Order("term ASC").Find(&items).Error
	return items, err
}

// Create 创建术语映射
func (r *BusinessGlossaryRepo) Create(item *aimodel.BusinessGlossary) error {
	return r.db.Create(item).Error
}

// Update 更新术语映射
func (r *BusinessGlossaryRepo) Update(item *aimodel.BusinessGlossary) error {
	return r.db.Save(item).Error
}

// Delete 删除术语映射
func (r *BusinessGlossaryRepo) Delete(id int) error {
	return r.db.Delete(&aimodel.BusinessGlossary{}, id).Error
}

// GetByID 按 ID 查询
func (r *BusinessGlossaryRepo) FindByID(id int) (*aimodel.BusinessGlossary, error) {
	var item aimodel.BusinessGlossary
	err := r.db.First(&item, id).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// FindByTerm 按术语查找映射（精确匹配，不区分大小写）
func (r *BusinessGlossaryRepo) FindByTerm(spaceID int, term string) ([]*aimodel.BusinessGlossary, error) {
	var items []*aimodel.BusinessGlossary
	q := r.db.Where("LOWER(term) = LOWER(?)", term)
	if spaceID > 0 {
		q = q.Where("space_id = ?", spaceID)
	}
	err := q.Find(&items).Error
	return items, err
}

// FindByTable 获取指定表的所有术语映射
func (r *BusinessGlossaryRepo) FindByTable(spaceID int, tableName string) ([]*aimodel.BusinessGlossary, error) {
	var items []*aimodel.BusinessGlossary
	q := r.db.Where("table_name = ?", tableName)
	if spaceID > 0 {
		q = q.Where("space_id = ?", spaceID)
	}
	err := q.Find(&items).Error
	return items, err
}
