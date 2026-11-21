package ai

import (
	"time"

	aimodel "github.com/listen808/citebase/internal/model/ai"
	"gorm.io/gorm"
)

// SQLTraceRepo SQL Trace 仓储
type SQLTraceRepo struct {
	db *gorm.DB
}

func NewSQLTraceRepo(db *gorm.DB) *SQLTraceRepo {
	return &SQLTraceRepo{db: db}
}

func (r *SQLTraceRepo) Create(t *aimodel.SQLTrace) error {
	return r.db.Create(t).Error
}

func (r *SQLTraceRepo) FindByTraceID(traceID string) (*aimodel.SQLTrace, error) {
	var t aimodel.SQLTrace
	err := r.db.Where("trace_id = ?", traceID).First(&t).Error
	return &t, err
}

func (r *SQLTraceRepo) List(spaceID uint64, page, pageSize int, since time.Time) ([]aimodel.SQLTrace, int64, error) {
	var list []aimodel.SQLTrace
	var total int64
	query := r.db.Model(&aimodel.SQLTrace{}).Where("space_id = ?", spaceID)
	if !since.IsZero() {
		query = query.Where("created_at >= ?", since)
	}
	query.Count(&total)
	err := query.Offset((page - 1) * pageSize).Limit(pageSize).Order("created_at DESC").Find(&list).Error
	return list, total, err
}
