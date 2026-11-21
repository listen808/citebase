package knowledge

import "time"

// VectorSyncState 文档级向量索引指纹（P0 增量缓存）。
//
// 记录每个文档"上次成功索引时的内容 SHA256 + embedding 身份"。
// Enqueue 时指纹未变 → 直接跳过，不再重复消耗 embedding 调用；
// embedding 模型/供应商一换，指纹必然失配 → 全量重建，
// 不再出现"换模型后新旧维度向量混存"的隐患。
type VectorSyncState struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	DocID         int       `gorm:"column:doc_id;not null;uniqueIndex:uk_doc_id" json:"doc_id"`
	ContentSHA256 string    `gorm:"column:content_sha256;type:char(64);not null" json:"content_sha256"`
	EmbIdentity   string    `gorm:"column:emb_identity;type:varchar(255);not null;default:''" json:"emb_identity"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (VectorSyncState) TableName() string {
	return "aw_vector_sync_state"
}
