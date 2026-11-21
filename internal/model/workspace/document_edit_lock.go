package workspace

import "time"

// DocumentEditLock 文档编辑锁（分布式）
type DocumentEditLock struct {
	DocumentID int       `gorm:"column:document_id;primaryKey" json:"document_id"`
	UserID     int       `gorm:"column:user_id;not null;index:idx_user_id" json:"user_id"`
	Username   string    `gorm:"column:username;size:100" json:"username"`
	LockedAt   time.Time `gorm:"column:locked_at;not null" json:"locked_at"`
	Permanent  int       `gorm:"column:permanent;default:0" json:"permanent"` // 0=timed (30min TTL), 1=permanent (manual unlock only)
}

func (DocumentEditLock) TableName() string {
	return "aw_document_edit_lock"
}

// LockRequest is the request body for acquiring an edit lock.
type LockRequest struct {
	Permanent bool `json:"permanent"` // true=permanent lock, false=timed lock (30min)
}
