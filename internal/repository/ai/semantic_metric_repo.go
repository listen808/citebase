package ai

import (
	aimodel "github.com/listen808/citebase/internal/model/ai"
	"gorm.io/gorm"
)

// SemanticMetricRepo 语义指标仓储
type SemanticMetricRepo struct {
	db *gorm.DB
}

func NewSemanticMetricRepo(db *gorm.DB) *SemanticMetricRepo {
	return &SemanticMetricRepo{db: db}
}

func (r *SemanticMetricRepo) Create(m *aimodel.SemanticMetric) error {
	return r.db.Create(m).Error
}

func (r *SemanticMetricRepo) FindByID(id uint64) (*aimodel.SemanticMetric, error) {
	var m aimodel.SemanticMetric
	err := r.db.First(&m, id).Error
	return &m, err
}

func (r *SemanticMetricRepo) Update(m *aimodel.SemanticMetric) error {
	return r.db.Save(m).Error
}

func (r *SemanticMetricRepo) Delete(id uint64) error {
	return r.db.Delete(&aimodel.SemanticMetric{}, id).Error
}

func (r *SemanticMetricRepo) ListBySpace(spaceID uint64, page, pageSize int) ([]aimodel.SemanticMetric, int64, error) {
	var list []aimodel.SemanticMetric
	var total int64
	query := r.db.Model(&aimodel.SemanticMetric{}).Where("space_id = ?", spaceID)
	query.Count(&total)
	err := query.Offset((page - 1) * pageSize).Limit(pageSize).Order("id DESC").Find(&list).Error
	return list, total, err
}

func (r *SemanticMetricRepo) GetByCode(spaceID uint64, code string) (*aimodel.SemanticMetric, error) {
	var m aimodel.SemanticMetric
	err := r.db.Where("space_id = ? AND code = ?", spaceID, code).First(&m).Error
	return &m, err
}

// SemanticDimensionRepo 语义维度仓储
type SemanticDimensionRepo struct {
	db *gorm.DB
}

func NewSemanticDimensionRepo(db *gorm.DB) *SemanticDimensionRepo {
	return &SemanticDimensionRepo{db: db}
}

func (r *SemanticDimensionRepo) Create(d *aimodel.SemanticDimension) error {
	return r.db.Create(d).Error
}

func (r *SemanticDimensionRepo) Update(d *aimodel.SemanticDimension) error {
	return r.db.Save(d).Error
}

func (r *SemanticDimensionRepo) Delete(id uint64) error {
	return r.db.Delete(&aimodel.SemanticDimension{}, id).Error
}

func (r *SemanticDimensionRepo) ListBySpace(spaceID uint64, page, pageSize int) ([]aimodel.SemanticDimension, int64, error) {
	var list []aimodel.SemanticDimension
	var total int64
	query := r.db.Model(&aimodel.SemanticDimension{}).Where("space_id = ?", spaceID)
	query.Count(&total)
	err := query.Offset((page - 1) * pageSize).Limit(pageSize).Order("id DESC").Find(&list).Error
	return list, total, err
}
