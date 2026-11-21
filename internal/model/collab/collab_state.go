package collab

import "time"

// DocumentCollabState stores the Yjs CRDT state for a document.
// This enables real-time collaborative editing with conflict-free merging.
type DocumentCollabState struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	DocumentID uint      `gorm:"uniqueIndex:uk_document_id;not null;column:document_id" json:"document_id"`
	State      []byte    `gorm:"type:mediumblob" json:"-"`
	Version    int       `gorm:"not null;default:0" json:"version"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (DocumentCollabState) TableName() string {
	return "aw_document_collab_state"
}

// DocumentCollabSession tracks active collaborative editing sessions.
type DocumentCollabSession struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	DocumentID   uint      `gorm:"index:idx_document_id;not null;column:document_id" json:"document_id"`
	UserID       uint      `gorm:"not null;column:user_id;index:idx_user_id" json:"user_id"`
	UserName     string    `gorm:"size:100;column:user_name" json:"user_name"`
	Color        string    `gorm:"size:7;column:color" json:"color"`
	ConnectedAt  time.Time `gorm:"column:connected_at" json:"connected_at"`
	LastActiveAt time.Time `gorm:"autoUpdateTime;column:last_active_at" json:"last_active_at"`
}

func (DocumentCollabSession) TableName() string {
	return "aw_document_collab_session"
}
