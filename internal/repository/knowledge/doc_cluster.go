package knowledge

import (
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

// AIDocClusterRepository AI 文档聚类仓储
type AIDocClusterRepository struct {
	db *gorm.DB
}

// NewAIDocClusterRepository 创建仓储
func NewAIDocClusterRepository(db *gorm.DB) *AIDocClusterRepository {
	return &AIDocClusterRepository{
		db: repos.DB,
	}
}

// DefaultAIDocClusterRepository 使用全局 repos.DB 创建仓储
func DefaultAIDocClusterRepository() *AIDocClusterRepository {
	return &AIDocClusterRepository{db: repos.DB}
}

// Create 创建聚类记录
func (r *AIDocClusterRepository) Create(cluster *knowledgemodel.AIDocCluster) error {
	return r.db.Create(cluster).Error
}

// CreateBatch 批量创建
func (r *AIDocClusterRepository) CreateBatch(clusters []knowledgemodel.AIDocCluster) error {
	return r.db.CreateInBatches(clusters, 100).Error
}

// FindByClusterID 根据聚类 ID 查找
func (r *AIDocClusterRepository) FindByClusterID(clusterID string) ([]knowledgemodel.AIDocCluster, error) {
	var clusters []knowledgemodel.AIDocCluster
	err := r.db.Where("cluster_id = ?", clusterID).Order("similarity DESC").Find(&clusters).Error
	return clusters, err
}

// FindByDocID 根据文档 ID 查找
func (r *AIDocClusterRepository) FindByDocID(docID int) ([]knowledgemodel.AIDocCluster, error) {
	var clusters []knowledgemodel.AIDocCluster
	err := r.db.Where("doc_id = ?", docID).Find(&clusters).Error
	return clusters, err
}

// FindAll 查找所有聚类（分页）
func (r *AIDocClusterRepository) FindAll(page, pageSize int) ([]knowledgemodel.AIDocCluster, int64, error) {
	var clusters []knowledgemodel.AIDocCluster
	var total int64

	offset := (page - 1) * pageSize
	err := r.db.Model(&knowledgemodel.AIDocCluster{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = r.db.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&clusters).Error
	return clusters, total, err
}

// UpdateStatus 更新状态
func (r *AIDocClusterRepository) UpdateStatus(id int, status string) error {
	return r.db.Model(&knowledgemodel.AIDocCluster{}).Where("id = ?", id).Update("status", status).Error
}

// UpdateMergePlan 更新合并计划
func (r *AIDocClusterRepository) UpdateMergePlan(id int, mergePlan knowledgemodel.JSONArray) error {
	return r.db.Model(&knowledgemodel.AIDocCluster{}).Where("id = ?", id).Update("merge_plan", mergePlan).Error
}

// S13: Update 通用更新
func (r *AIDocClusterRepository) Update(cluster *knowledgemodel.AIDocCluster) error {
	return r.db.Save(cluster).Error
}

// DeleteByClusterID 删除聚类
func (r *AIDocClusterRepository) DeleteByClusterID(clusterID string) error {
	return r.db.Where("cluster_id = ?", clusterID).Delete(&knowledgemodel.AIDocCluster{}).Error
}

// FindPendingClusters 查找待处理的聚类
func (r *AIDocClusterRepository) FindPendingClusters() ([]knowledgemodel.AIDocCluster, error) {
	var clusters []knowledgemodel.AIDocCluster
	err := r.db.Where("status = ?", "pending").Order("similarity DESC").Find(&clusters).Error
	return clusters, err
}
