package knowledge

import "time"

type VectorSyncTask struct {
	ID         int       `gorm:"primaryKey;autoIncrement" json:"id"`
	DocID      int       `gorm:"column:doc_id;not null;index:idx_doc_id" json:"doc_id"`
	SpaceID    int       `gorm:"column:space_id;not null" json:"space_id"`
	DocName    string    `gorm:"type:varchar(500)" json:"doc_name"`
	Content    string    `gorm:"type:longtext" json:"content"`
	OpType     string    `gorm:"type:varchar(20);not null" json:"op_type"`
	Status     string    `gorm:"type:varchar(20);not null;default:pending;index:idx_status" json:"status"`
	RetryCount int       `gorm:"column:retry_count;type:tinyint;default:0" json:"retry_count"`
	LastError  string    `gorm:"type:varchar(500)" json:"last_error"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (VectorSyncTask) TableName() string {
	return "aw_vector_sync_task"
}
