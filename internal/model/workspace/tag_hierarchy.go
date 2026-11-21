package workspace

import (
	"time"

	"gorm.io/gorm"
)

// TagHierarchy represents parent-child relationships between tags.
type TagHierarchy struct {
	ID         int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ParentTag  string `gorm:"column:parent_tag;size:100;not null;index:idx_tag_parent" json:"parent_tag"`
	ChildTag   string `gorm:"column:child_tag;size:100;not null;uniqueIndex:uk_tag_child" json:"child_tag"`
	SpaceID    int64  `gorm:"column:space_id;not null;default:0;index:idx_tag_space" json:"space_id"`
	CreateTime int64  `gorm:"column:create_time;not null;default:0" json:"create_time"`
}

func (TagHierarchy) TableName() string {
	return "aw_tag_hierarchy"
}

func (h *TagHierarchy) BeforeCreate(tx *gorm.DB) error {
	if h.CreateTime == 0 {
		h.CreateTime = time.Now().Unix()
	}
	return nil
}

// TagHierarchyResponse is the API response for tag hierarchy.
type TagHierarchyResponse struct {
	ID        int64  `json:"id"`
	ParentTag string `json:"parent_tag"`
	ChildTag  string `json:"child_tag"`
	SpaceID   int64  `json:"space_id"`
}

// TagTreeNode represents a tag with its children for tree display.
type TagTreeNode struct {
	TagName  string         `json:"tag_name"`
	Count    int            `json:"count"`
	Children []*TagTreeNode `json:"children,omitempty"`
}

// CreateTagHierarchyRequest is the request body for creating tag hierarchy.
type CreateTagHierarchyRequest struct {
	ParentTag string `json:"parent_tag" binding:"required,max=100"`
	ChildTag  string `json:"child_tag" binding:"required,max=100"`
	SpaceID   int64  `json:"space_id"`
}
