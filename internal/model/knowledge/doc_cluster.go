package knowledge

import (
	"time"
)

// AIDocCluster AI 检测的文档聚类（重复/相似文档组）
type AIDocCluster struct {
	ID         int       `gorm:"primaryKey" json:"id"`
	ClusterID  string    `gorm:"size:64;index:idx_cluster_id;not null" json:"cluster_id"` // 聚类唯一标识
	DocID      int       `gorm:"index:idx_doc_id;not null" json:"doc_id"`                 // 文档 ID
	SpaceID    int       `gorm:"index:idx_space_id;not null" json:"space_id"`             // 空间 ID
	DocName    string    `gorm:"size:255;not null" json:"doc_name"`                       // 文档名称
	Similarity float64   `gorm:"type:decimal(5,4)" json:"similarity"`                     // 与聚类中心的相似度 (0-1)
	Status     string    `gorm:"size:32;not null" json:"status"`                          // pending/merged/kept/ignored
	Reason     string    `gorm:"type:text" json:"reason"`                                 // 判定理由
	MergePlan  JSONArray `gorm:"type:json" json:"merge_plan"`                             // 合并计划
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TableName 指定表名
func (AIDocCluster) TableName() string {
	return "aw_ai_doc_cluster"
}
