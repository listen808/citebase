package workspace

import (
	"time"

	"gorm.io/gorm"
)

// SavedSearch represents a user's saved search query.
type SavedSearch struct {
	ID         int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID     int64  `gorm:"column:user_id;not null;index:idx_saved_user" json:"user_id"`
	Name       string `gorm:"column:name;size:100;not null" json:"name"`
	Query      string `gorm:"column:query;size:500;not null" json:"query"`
	Type       string `gorm:"column:type;size:20;default:'all'" json:"type"` // all/title/content
	SpaceID    int64  `gorm:"column:space_id;default:0" json:"space_id"`
	TagFilter  string `gorm:"column:tag_filter;size:100;default:''" json:"tag_filter"`
	SortBy     string `gorm:"column:sort_by;size:20;default:'relevance'" json:"sort_by"`
	IsPublic   int    `gorm:"column:is_public;default:0" json:"is_public"`
	UseCount   int    `gorm:"column:use_count;default:0" json:"use_count"`
	CreateTime int64  `gorm:"column:create_time;not null;default:0" json:"create_time"`
	UpdateTime int64  `gorm:"column:update_time;not null;default:0" json:"update_time"`
}

func (SavedSearch) TableName() string {
	return "aw_saved_search"
}

func (s *SavedSearch) BeforeCreate(tx *gorm.DB) error {
	if s.CreateTime == 0 {
		s.CreateTime = time.Now().Unix()
	}
	if s.UpdateTime == 0 {
		s.UpdateTime = time.Now().Unix()
	}
	return nil
}

func (s *SavedSearch) BeforeUpdate(tx *gorm.DB) error {
	s.UpdateTime = time.Now().Unix()
	return nil
}

// SavedSearchResponse is the API response for saved searches.
type SavedSearchResponse struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"user_id"`
	Name       string `json:"name"`
	Query      string `json:"query"`
	Type       string `json:"type"`
	SpaceID    int64  `json:"space_id"`
	TagFilter  string `json:"tag_filter"`
	SortBy     string `json:"sort_by"`
	IsPublic   int    `json:"is_public"`
	UseCount   int    `json:"use_count"`
	CreateTime int64  `json:"create_time"`
	UpdateTime int64  `json:"update_time"`
}

// CreateSavedSearchRequest is the request body for creating a saved search.
type CreateSavedSearchRequest struct {
	Name      string `json:"name" binding:"required,max=100"`
	Query     string `json:"query" binding:"required,max=500"`
	Type      string `json:"type" binding:"omitempty,oneof=all title content"`
	SpaceID   int64  `json:"space_id"`
	TagFilter string `json:"tag_filter" binding:"max=100"`
	SortBy    string `json:"sort_by" binding:"omitempty,oneof=relevance time_desc time_asc"`
	IsPublic  int    `json:"is_public" binding:"omitempty,oneof=0 1"`
}

// UpdateSavedSearchRequest is the request body for updating a saved search.
type UpdateSavedSearchRequest struct {
	Name      string `json:"name" binding:"omitempty,max=100"`
	Query     string `json:"query" binding:"omitempty,max=500"`
	Type      string `json:"type" binding:"omitempty,oneof=all title content"`
	SpaceID   int64  `json:"space_id"`
	TagFilter string `json:"tag_filter" binding:"max=100"`
	SortBy    string `json:"sort_by" binding:"omitempty,oneof=relevance time_desc time_asc"`
	IsPublic  int    `json:"is_public" binding:"omitempty,oneof=0 1"`
}
