package knowledge

import (
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
	"time"
)

type AITaskRepository struct {
	db *gorm.DB
}

func NewAITaskRepository() *AITaskRepository {
	return &AITaskRepository{db: repos.DB}
}

func (r *AITaskRepository) Create(task *knowledgemodel.AITask) error {
	return r.db.Create(task).Error
}

func (r *AITaskRepository) Update(task *knowledgemodel.AITask) error {
	return r.db.Save(task).Error
}

func (r *AITaskRepository) FindByTaskID(taskID string) (*knowledgemodel.AITask, error) {
	var task knowledgemodel.AITask
	err := r.db.Where("task_id = ?", taskID).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *AITaskRepository) FindAll(limit, offset int) ([]*knowledgemodel.AITask, int64, error) {
	var tasks []*knowledgemodel.AITask
	var total int64

	if err := r.db.Model(&knowledgemodel.AITask{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := r.db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&tasks).Error
	return tasks, total, err
}

func (r *AITaskRepository) FindByStatus(status string) ([]*knowledgemodel.AITask, error) {
	var tasks []*knowledgemodel.AITask
	err := r.db.Where("status = ?", status).Order("created_at DESC").Find(&tasks).Error
	return tasks, err
}

func (r *AITaskRepository) UpdateStatus(taskID, status string, progress float64, tokenUsage int, errorMsg string) error {
	updates := map[string]interface{}{
		"status":   status,
		"progress": progress,
	}
	if tokenUsage > 0 {
		updates["token_usage"] = gorm.Expr("token_usage + ?", tokenUsage)
	}
	if errorMsg != "" {
		updates["error_message"] = errorMsg
	}
	if status == "running" {
		now := time.Now()
		updates["started_at"] = &now
	}
	if status == "completed" || status == "failed" {
		now := time.Now()
		updates["completed_at"] = &now
	}
	return r.db.Model(&knowledgemodel.AITask{}).Where("task_id = ?", taskID).Updates(updates).Error
}

func (r *AITaskRepository) UpdateResult(taskID string, result string, totalDocs, processedDocs int) error {
	return r.db.Model(&knowledgemodel.AITask{}).Where("task_id = ?", taskID).Updates(map[string]interface{}{
		"result":         result,
		"total_docs":     totalDocs,
		"processed_docs": processedDocs,
	}).Error
}

type AIDocMappingRepository struct {
	db *gorm.DB
}

func NewAIDocMappingRepository() *AIDocMappingRepository {
	return &AIDocMappingRepository{db: repos.DB}
}

func (r *AIDocMappingRepository) Create(mapping *knowledgemodel.AIDocMapping) error {
	return r.db.Create(mapping).Error
}

func (r *AIDocMappingRepository) FindByAIDocID(aiDocID int) (*knowledgemodel.AIDocMapping, error) {
	var m knowledgemodel.AIDocMapping
	err := r.db.Where("ai_doc_id = ?", aiDocID).First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *AIDocMappingRepository) FindBySourceDocID(sourceDocID int) ([]*knowledgemodel.AIDocMapping, error) {
	var mappings []*knowledgemodel.AIDocMapping
	err := r.db.Where("source_doc_id = ?", sourceDocID).Find(&mappings).Error
	return mappings, err
}

func (r *AIDocMappingRepository) UpdateReviewStatus(aiDocID int, status string) error {
	return r.db.Model(&knowledgemodel.AIDocMapping{}).Where("ai_doc_id = ?", aiDocID).Update("review_status", status).Error
}

func (r *AIDocMappingRepository) Upsert(mapping *knowledgemodel.AIDocMapping) error {
	var existing knowledgemodel.AIDocMapping
	err := r.db.Where("ai_doc_id = ?", mapping.AIDocID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return r.db.Create(mapping).Error
	}
	if err != nil {
		return err
	}
	existing.SourceDocID = mapping.SourceDocID
	existing.SourceSpaceID = mapping.SourceSpaceID
	existing.MappingType = mapping.MappingType
	now := time.Now()
	existing.LastSyncedAt = &now
	return r.db.Save(&existing).Error
}

func (r *AIDocMappingRepository) FindByReviewStatus(status string) ([]*knowledgemodel.AIDocMapping, error) {
	var mappings []*knowledgemodel.AIDocMapping
	err := r.db.Where("review_status = ?", status).Find(&mappings).Error
	return mappings, err
}

func (r *AIDocMappingRepository) FindBySourceDocIDs(docIDs []int) ([]*knowledgemodel.AIDocMapping, error) {
	var mappings []*knowledgemodel.AIDocMapping
	err := r.db.Where("source_doc_id IN ?", docIDs).Find(&mappings).Error
	return mappings, err
}

func (r *AITaskRepository) FindLastCompleted() (*knowledgemodel.AITask, error) {
	var task knowledgemodel.AITask
	err := r.db.Where("status = ?", "completed").Order("completed_at DESC").First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// S8: UpdateCheckpoint 更新断点信息
func (r *AITaskRepository) UpdateCheckpoint(taskID string, checkpoint string) error {
	return r.db.Model(&knowledgemodel.AITask{}).Where("task_id = ?", taskID).Update("checkpoint", checkpoint).Error
}

// S8: FindResumableTask 查找可恢复的失败/暂停任务
func (r *AITaskRepository) FindResumableTask(taskType string) (*knowledgemodel.AITask, error) {
	var task knowledgemodel.AITask
	err := r.db.Where("type = ? AND status IN ? AND checkpoint != ''", taskType, []string{"failed", "paused"}).
		Order("updated_at DESC").First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}
