package ai

import (
	aimodel "github.com/listen808/citebase/internal/model/ai"
	"gorm.io/gorm"
)

// ColumnStatsRepo 列统计仓储
type ColumnStatsRepo struct {
	db *gorm.DB
}

func NewColumnStatsRepo(db *gorm.DB) *ColumnStatsRepo {
	return &ColumnStatsRepo{db: db}
}

func (r *ColumnStatsRepo) Upsert(s *aimodel.ColumnStats) error {
	return r.db.Where("space_id = ? AND table_name = ? AND column_name = ?",
		s.SpaceID, s.TableNam, s.ColumnName).
		Assign(s).FirstOrCreate(s).Error
}

func (r *ColumnStatsRepo) GetByTable(spaceID uint64, tableName string) ([]aimodel.ColumnStats, error) {
	var list []aimodel.ColumnStats
	err := r.db.Where("space_id = ? AND table_name = ?", spaceID, tableName).Find(&list).Error
	return list, err
}

func (r *ColumnStatsRepo) DeleteBySpace(spaceID uint64) error {
	return r.db.Where("space_id = ?", spaceID).Delete(&aimodel.ColumnStats{}).Error
}
