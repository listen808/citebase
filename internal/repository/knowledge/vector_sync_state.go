package knowledge

import (
	"time"

	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type VectorSyncStateRepository struct {
	db *gorm.DB
}

func VectorSyncStateRepo() *VectorSyncStateRepository {
	return &VectorSyncStateRepository{db: repos.DB}
}

func NewVectorSyncStateRepositoryWithDB(db *gorm.DB) *VectorSyncStateRepository {
	return &VectorSyncStateRepository{db: db}
}

// GetByDocID 取文档当前指纹；无记录返回 gorm.ErrRecordNotFound
func (r *VectorSyncStateRepository) GetByDocID(docID int) (*knowledgemodel.VectorSyncState, error) {
	var s knowledgemodel.VectorSyncState
	if err := r.db.Where("doc_id = ?", docID).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// Upsert 以 doc_id 为唯一键覆盖指纹
func (r *VectorSyncStateRepository) Upsert(state *knowledgemodel.VectorSyncState) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "doc_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"content_sha256", "emb_identity", "updated_at"}),
	}).Create(state).Error
}

func (r *VectorSyncStateRepository) DeleteByDocID(docID int) error {
	return r.db.Where("doc_id = ?", docID).Delete(&knowledgemodel.VectorSyncState{}).Error
}

// DeleteOlderThan 兜底清理长期未更新的状态行（正常情况下文档删除走 DeleteByDocID）
func (r *VectorSyncStateRepository) DeleteOlderThan(before time.Time) error {
	return r.db.Where("updated_at < ?", before).Delete(&knowledgemodel.VectorSyncState{}).Error
}
