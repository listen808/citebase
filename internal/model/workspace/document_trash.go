package workspace

import (
	"time"

	"gorm.io/gorm"
)

// DocumentTrash 回收站
type DocumentTrash struct {
	ID          int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	DocumentID  int    `gorm:"column:document_id;not null;index:idx_document_id" json:"document_id"`
	ParentID    int    `gorm:"column:parent_id" json:"parent_id"`
	SpaceID     int    `gorm:"column:space_id;not null;index:idx_space_id" json:"space_id"`
	Name        string `gorm:"column:name;size:255;not null" json:"name"`
	Type        int    `gorm:"column:type" json:"type"`
	Path        string `gorm:"column:path;size:500" json:"path"`
	Sequence    int    `gorm:"column:sequence" json:"sequence"`
	ContentPath string `gorm:"column:content_path;size:500" json:"content_path"`
	DeletedBy   int    `gorm:"column:deleted_by" json:"deleted_by"`
	DeletedAt   int    `gorm:"column:deleted_at;index:idx_deleted_at" json:"deleted_at"`
	DeleteLog   string `gorm:"column:delete_log;size:500" json:"delete_log"`
}

func (DocumentTrash) TableName() string {
	return "aw_document_trash"
}

func (t *DocumentTrash) BeforeCreate(tx *gorm.DB) error {
	t.DeletedAt = int(time.Now().Unix())
	return nil
}
