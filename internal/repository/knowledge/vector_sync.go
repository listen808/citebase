package knowledge

import (
	"database/sql"
	"time"

	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

type VectorSyncTaskRepository struct {
	db *gorm.DB
}

func VectorSyncTaskRepo() *VectorSyncTaskRepository {
	return &VectorSyncTaskRepository{db: repos.DB}
}

func NewVectorSyncTaskRepository() *VectorSyncTaskRepository {
	return &VectorSyncTaskRepository{db: repos.DB}
}

func NewVectorSyncTaskRepositoryWithDB(db *gorm.DB) *VectorSyncTaskRepository {
	return &VectorSyncTaskRepository{db: db}
}

func (r *VectorSyncTaskRepository) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

// DocSyncStatus 一条「某文档在本空间的最新同步状态」。
type DocSyncStatus struct {
	DocID  int
	Status string
}

// ListDocStatusBySpace 按创建时间倒序取本空间所有文档的同步状态行。
// 调用方要的是"每个 doc 最新那一条"，所以排序是契约的一部分，不是可选优化。
func (r *VectorSyncTaskRepository) ListDocStatusBySpace(spaceID int) ([]DocSyncStatus, error) {
	var rows []DocSyncStatus
	err := r.db.Table("aw_vector_sync_task").
		Select("doc_id, status").
		Where("space_id = ?", spaceID).
		Order("created_at DESC").
		Scan(&rows).Error
	return rows, err
}

// CountDistinctDocs 本空间中处于指定状态的去重文档数。
func (r *VectorSyncTaskRepository) CountDistinctDocs(spaceID int, status string) (int64, error) {
	var count int64
	err := r.db.Table("aw_vector_sync_task").
		Where("space_id = ? AND status = ?", spaceID, status).
		Distinct("doc_id").Count(&count).Error
	return count, err
}

// LatestCompletedAt 本空间最近一次完成同步的时间。
// 返回 sql.NullTime 而不是 time.Time：没有完成记录时"零值时间"会被渲染成
// 0001-01-01 那样的一个日期，而"从未同步"和"1 年 1 月 1 日同步过"是两件事。
func (r *VectorSyncTaskRepository) LatestCompletedAt(spaceID int) (sql.NullTime, error) {
	var row struct {
		LastAt *time.Time
	}
	err := r.db.Table("aw_vector_sync_task").
		Select("MAX(updated_at) AS last_at").
		Where("space_id = ? AND status = ?", spaceID, "completed").
		Scan(&row).Error
	if err != nil {
		return sql.NullTime{}, err
	}
	if row.LastAt == nil {
		return sql.NullTime{}, nil
	}
	return sql.NullTime{Time: *row.LastAt, Valid: true}, nil
}

func (r *VectorSyncTaskRepository) Create(task *knowledgemodel.VectorSyncTask) error {
	return r.db.Create(task).Error
}

func (r *VectorSyncTaskRepository) FetchPending(limit int) ([]knowledgemodel.VectorSyncTask, error) {
	var tasks []knowledgemodel.VectorSyncTask
	err := r.db.Where("status = ? AND retry_count < ?", "pending", 3).
		Order("id ASC").
		Limit(limit).
		Find(&tasks).Error
	return tasks, err
}

func (r *VectorSyncTaskRepository) MarkProcessing(id int) error {
	return r.db.Model(&knowledgemodel.VectorSyncTask{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": "processing", "updated_at": time.Now()}).Error
}

func (r *VectorSyncTaskRepository) MarkDone(id int) error {
	return r.db.Model(&knowledgemodel.VectorSyncTask{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": "done", "updated_at": time.Now()}).Error
}

func (r *VectorSyncTaskRepository) MarkFailed(id int, errMsg string) error {
	return r.db.Model(&knowledgemodel.VectorSyncTask{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      "pending",
			"retry_count": gorm.Expr("retry_count + 1"),
			"last_error":  errMsg,
			"updated_at":  time.Now(),
		}).Error
}

func (r *VectorSyncTaskRepository) MarkDead(id int, errMsg string) error {
	return r.db.Model(&knowledgemodel.VectorSyncTask{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     "dead",
			"last_error": errMsg,
			"updated_at": time.Now(),
		}).Error
}

func (r *VectorSyncTaskRepository) PurgeDone(before time.Time) error {
	return r.db.Where("status = ? AND updated_at < ?", "done", before).
		Delete(&knowledgemodel.VectorSyncTask{}).Error
}

func (r *VectorSyncTaskRepository) CountByStatus(status string) (int64, error) {
	var count int64
	err := r.db.Model(&knowledgemodel.VectorSyncTask{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

func (r *VectorSyncTaskRepository) FetchByStatus(status string, limit int) ([]knowledgemodel.VectorSyncTask, error) {
	var tasks []knowledgemodel.VectorSyncTask
	err := r.db.Where("status = ?", status).
		Order("updated_at DESC").
		Limit(limit).
		Find(&tasks).Error
	return tasks, err
}
