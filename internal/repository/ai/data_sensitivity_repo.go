package ai

import (
	aimodel "github.com/listen808/citebase/internal/model/ai"
	"gorm.io/gorm"
)

type DataSensitivityRepo struct {
	db *gorm.DB
}

func NewDataSensitivityRepo(db *gorm.DB) *DataSensitivityRepo {
	return &DataSensitivityRepo{db: db}
}

func (r *DataSensitivityRepo) Create(rule *aimodel.DataSensitivityRule) error {
	return r.db.Create(rule).Error
}

func (r *DataSensitivityRepo) Update(rule *aimodel.DataSensitivityRule) error {
	return r.db.Save(rule).Error
}

func (r *DataSensitivityRepo) Delete(id uint64) error {
	return r.db.Delete(&aimodel.DataSensitivityRule{}, id).Error
}

func (r *DataSensitivityRepo) FindByID(id uint64) (*aimodel.DataSensitivityRule, error) {
	var rule aimodel.DataSensitivityRule
	if err := r.db.First(&rule, id).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *DataSensitivityRepo) ListBySpace(spaceID uint64) ([]*aimodel.DataSensitivityRule, error) {
	var rules []*aimodel.DataSensitivityRule
	if err := r.db.Where("space_id = ?", spaceID).Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *DataSensitivityRepo) ListBySpaceAndTable(spaceID uint64, tableName string) ([]*aimodel.DataSensitivityRule, error) {
	var rules []*aimodel.DataSensitivityRule
	if err := r.db.Where("space_id = ? AND table_name = ?", spaceID, tableName).Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *DataSensitivityRepo) GetBySpaceTableField(spaceID uint64, tableName, fieldName string) (*aimodel.DataSensitivityRule, error) {
	var rule aimodel.DataSensitivityRule
	if err := r.db.Where("space_id = ? AND table_name = ? AND field_name = ?", spaceID, tableName, fieldName).First(&rule).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}
