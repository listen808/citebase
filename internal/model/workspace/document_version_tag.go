package workspace

import (
	"time"

	"gorm.io/gorm"
)

// DocumentVersionTag represents a tag/milestone applied to a specific document version.
type DocumentVersionTag struct {
	ID         int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	DocumentID int64  `gorm:"column:document_id;not null;index:idx_vtag_doc" json:"document_id"`
	Version    int    `gorm:"column:version;not null" json:"version"`
	TagName    string `gorm:"column:tag_name;size:100;not null;index:idx_vtag_name" json:"tag_name"`
	Color      string `gorm:"column:color;size:20;default:''" json:"color"`
	Note       string `gorm:"column:note;size:500;default:''" json:"note"`
	CreatedBy  int64  `gorm:"column:created_by;not null" json:"created_by"`
	CreateTime int64  `gorm:"column:create_time;not null" json:"create_time"`
}

func (DocumentVersionTag) TableName() string {
	return "aw_document_version_tag"
}

func (t *DocumentVersionTag) BeforeCreate(tx *gorm.DB) error {
	if t.CreateTime == 0 {
		t.CreateTime = time.Now().Unix()
	}
	return nil
}

// DocumentVersionTagResponse is the API response for version tags.
type DocumentVersionTagResponse struct {
	ID          int64  `json:"id"`
	DocumentID  int64  `json:"document_id"`
	Version     int    `json:"version"`
	TagName     string `json:"tag_name"`
	Color       string `json:"color"`
	Note        string `json:"note"`
	CreatedBy   int64  `json:"created_by"`
	CreateTime  int64  `json:"create_time"`
	CreatorName string `json:"creator_name,omitempty"`
}

func (t *DocumentVersionTag) ToResponse() *DocumentVersionTagResponse {
	return &DocumentVersionTagResponse{
		ID:         t.ID,
		DocumentID: t.DocumentID,
		Version:    t.Version,
		TagName:    t.TagName,
		Color:      t.Color,
		Note:       t.Note,
		CreatedBy:  t.CreatedBy,
		CreateTime: t.CreateTime,
	}
}

// CreateVersionTagRequest is the request body for creating a version tag.
type CreateVersionTagRequest struct {
	Version int    `json:"version" binding:"required"`
	TagName string `json:"tag_name" binding:"required,max=100"`
	Color   string `json:"color" binding:"max=20"`
	Note    string `json:"note" binding:"max=500"`
}
