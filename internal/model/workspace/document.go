package workspace

import (
	"time"

	"gorm.io/gorm"
)

const (
	DocumentTypePage = 1 // 文档页
	DocumentTypeDir  = 2 // 目录

	// 文档状态：published=已发布（默认，所有人可见），draft=草稿（仅作者和管理员可见）
	DocumentStatusPublished = "published"
	DocumentStatusDraft     = "draft"
)

// Document 文档模型 (对应 aw_document 表)
// 注意: 文档内容存储在文件系统中，不在数据库中
type Document struct {
	ID           int    `gorm:"column:document_id;primaryKey;autoIncrement" json:"id"`
	ParentID     int    `gorm:"column:parent_id" json:"parent_id"`
	SpaceID      int    `gorm:"column:space_id" json:"space_id"`
	Name         string `gorm:"column:name;size:255;not null" json:"name"`
	Content      string `gorm:"-" json:"content"`                 // 内容存储在文件系统，不参与数据库操作
	HtmlContent  string `gorm:"-" json:"html_content"`            // HTML 内容由 Content 转换，不参与数据库操作
	Type         int    `gorm:"column:type" json:"type"`          // 1 page 2 dir
	Path         string `gorm:"column:path;size:500" json:"path"` // 路径格式: 0,1,2,...
	Sequence     int    `gorm:"column:sequence" json:"sequence"`
	Icon         string `gorm:"column:icon;size:50;default:''" json:"icon"`                // 页面图标 (emoji)
	CoverImage   string `gorm:"column:cover_image;size:500;default:''" json:"cover_image"` // 封面图 URL
	CreatedBy    int    `gorm:"-" json:"created_by"`                                       // 仅用于响应，实际使用 create_user_id
	CreateUserID int    `gorm:"column:create_user_id;index:idx_create_user_id" json:"create_user_id"`
	EditUserID   int    `gorm:"column:edit_user_id;index:idx_edit_user_id" json:"edit_user_id"`
	// 🔧 P1-1: 移除 IsShare，文档分享权限由空间级别控制
	IsDelete   int `gorm:"column:is_delete" json:"is_delete"`
	CreateTime int `gorm:"column:create_time" json:"create_time"`
	UpdateTime int `gorm:"column:update_time" json:"update_time"`
	// #7 草稿/发布分离：published=已发布（默认），draft=草稿
	// 旧数据迁移：ensureMissingColumns 会自动补充此字段，DEFAULT 'published'
	Status      string `gorm:"column:status;size:20;default:'published';index:idx_status" json:"status"`
	PublishedAt int    `gorm:"column:published_at" json:"published_at"`           // 发布时间（Unix 时间戳），草稿态为 0
	IsImmutable int    `gorm:"column:is_immutable;default:0" json:"is_immutable"` // raw 层不可变标记：1=内容变更触发 wiki 重编译
}

// BeforeCreate 插入前钩子
func (d *Document) BeforeCreate(tx *gorm.DB) error {
	now := int(time.Now().Unix())
	d.CreateTime = now
	d.UpdateTime = now
	// #7 默认状态为已发布
	if d.Status == "" {
		d.Status = DocumentStatusPublished
	}
	if d.Status == DocumentStatusPublished && d.PublishedAt == 0 {
		d.PublishedAt = now
	}
	return nil
}

// BeforeUpdate 更新前钩子
func (d *Document) BeforeUpdate(tx *gorm.DB) error {
	d.UpdateTime = int(time.Now().Unix())
	return nil
}

// TableName 指定表名
func (Document) TableName() string {
	return "aw_document"
}

// DocumentVersion 文档版本模型 (新增表，支持版本追溯)
type DocumentVersion struct {
	ID         int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	DocumentID int    `gorm:"column:document_id;index:idx_document_id" json:"document_id"`
	Version    int    `gorm:"column:version" json:"version"`
	Content    string `gorm:"column:content;type:mediumtext" json:"content"`
	ChangeLog  string `gorm:"column:change_log;size:500" json:"change_log"`
	CreatedBy  int    `gorm:"column:created_by" json:"created_by"`
	CreatedAt  int    `gorm:"column:created_at" json:"created_at"` // Unix 时间戳
}

// BeforeCreate 文档版本插入前钩子
func (v *DocumentVersion) BeforeCreate(tx *gorm.DB) error {
	v.CreatedAt = int(time.Now().Unix())
	return nil
}

// TableName 指定表名
func (DocumentVersion) TableName() string {
	return "aw_document_version"
}

// CreateDocumentRequest 创建文档请求
type CreateDocumentRequest struct {
	ParentID int    `json:"parent_id" binding:"gte=0"`
	SpaceID  int    `json:"space_id" binding:"required,gt=0"`
	Name     string `json:"name" binding:"required,min=1,max=150"`
	Type     int    `json:"type" binding:"required,oneof=1 2"` // 1 page 2 dir
	Sequence int    `json:"sequence"`
	Content  string `json:"content"` // 仅当 type=1 时需要
}

// UpdateDocumentRequest 更新文档请求
type UpdateDocumentRequest struct {
	Name           string `json:"name" binding:"omitempty,min=1,max=150"`
	Content        string `json:"content"`
	Sequence       int    `json:"sequence"`
	Icon           string `json:"icon"`             // 页面图标 (emoji)
	CoverImage     string `json:"cover_image"`      // 封面图 URL
	ChangeLog      string `json:"change_log"`       // 版本变更说明
	LastUpdateTime int    `json:"last_update_time"` // 冲突检测：客户端上次读取时的 update_time
	// IsFollowDoc 已于 P2-5 从本请求中移除：
	// UpdateDocument 不再有关注副作用，如需关注文档，前端请单独调用 POST /api/v1/follow/document
}

// MoveDocumentRequest 移动文档请求
type MoveDocumentRequest struct {
	ParentID int `json:"parent_id"`
	SpaceID  int `json:"space_id" binding:"required"`
}

// CopyDocumentRequest 复制文档请求
type CopyDocumentRequest struct {
	ParentID int `json:"parent_id"`
	SpaceID  int `json:"space_id" binding:"required"`
}

// SortDocumentsRequest 批量排序文档请求
type SortDocumentsRequest struct {
	Orders []DocumentOrder `json:"orders" binding:"required"`
}

// DocumentOrder 单个文档排序项
type DocumentOrder struct {
	ID       int `json:"id" binding:"required"`
	Sequence int `json:"sequence"`
}

// DocumentResponse 文档响应
type DocumentResponse struct {
	ID           int                 `json:"id"`
	ParentID     int                 `json:"parent_id"`
	SpaceID      int                 `json:"space_id"`
	SpaceName    string              `json:"space_name,omitempty"` // 空间名（用于面包屑，直接访问文档时也有效）
	Name         string              `json:"name"`
	Type         int                 `json:"type"`
	Path         string              `json:"path"`
	Sequence     int                 `json:"sequence"`
	Icon         string              `json:"icon,omitempty"`        // P0 修复：对齐前端 Document.icon
	CoverImage   string              `json:"cover_image,omitempty"` // P0 修复：对齐前端 Document.cover_image
	CreateUserID int                 `json:"create_user_id"`
	EditUserID   int                 `json:"edit_user_id"`
	CreatorID    int                 `json:"creator_id,omitempty"` // 前端兼容别名
	EditorID     int                 `json:"editor_id,omitempty"`  // 前端兼容别名
	IsDelete     int                 `json:"is_delete"`
	CreateTime   int                 `json:"create_time"`
	UpdateTime   int                 `json:"update_time"`
	CreatorName  string              `json:"creator_name,omitempty"`
	EditorName   string              `json:"editor_name,omitempty"`
	HasChildren  bool                `json:"has_children"`       // 标记是否有子节点（用于懒加载）
	Children     []*DocumentResponse `json:"children,omitempty"` // 子文档（树形结构）
	Version      int                 `json:"version,omitempty"`  // 最新版本号
	Status       string              `json:"status"`             // #7: published / draft
	PublishedAt  int                 `json:"published_at"`       // #7: 发布时间
}

// DocumentFullResponse 合并响应：文档元数据 + 内容 + 路径（减少 HTTP 往返）
type DocumentFullResponse struct {
	Document DocumentResponse         `json:"document"`
	Content  string                   `json:"content"`
	Path     []map[string]interface{} `json:"path"`
}

// ToResponse 转换为响应对象（不含内容，内容需单独从文件系统获取）
func (d *Document) ToResponse(spaceName ...string) DocumentResponse {
	resp := DocumentResponse{
		ID:           d.ID,
		ParentID:     d.ParentID,
		SpaceID:      d.SpaceID,
		Name:         d.Name,
		Type:         d.Type,
		Path:         d.Path,
		Sequence:     d.Sequence,
		Icon:         d.Icon,
		CoverImage:   d.CoverImage,
		CreateUserID: d.CreateUserID,
		EditUserID:   d.EditUserID,
		CreatorID:    d.CreateUserID,
		EditorID:     d.EditUserID,
		IsDelete:     d.IsDelete,
		CreateTime:   d.CreateTime,
		UpdateTime:   d.UpdateTime,
		Status:       d.Status,
		PublishedAt:  d.PublishedAt,
	}
	if len(spaceName) > 0 {
		resp.SpaceName = spaceName[0]
	}
	return resp
}

// DiffBlockType diff 块类型
const (
	DiffBlockContext = "context" // 未变更行
	DiffBlockAdded   = "added"   // 新增行
	DiffBlockRemoved = "removed" // 删除行
)

// DiffBlock diff 块
type DiffBlock struct {
	Type  string   `json:"type"`
	Lines []string `json:"lines"`
}

// DiffResponse 版本对比响应
type DiffResponse struct {
	DocumentID   int         `json:"document_id"`
	FromVersion  int         `json:"from_version"`
	ToVersion    int         `json:"to_version"`
	HasChanges   bool        `json:"has_changes"`
	DiffBlocks   []DiffBlock `json:"diff_blocks"`
	AddedCount   int         `json:"added_count"`
	RemovedCount int         `json:"removed_count"`
}

// DocumentVersionResponse 文档版本响应
type DocumentVersionResponse struct {
	ID          int    `json:"id"`
	DocumentID  int    `json:"document_id"`
	Version     int    `json:"version"`
	Content     string `json:"content"`
	ChangeLog   string `json:"change_log"`
	CreatedBy   int    `json:"created_by"`
	UserID      int    `json:"user_id,omitempty"`      // 前端兼容别名
	CreatorName string `json:"creator_name,omitempty"` // 创建者姓名
	CreatedAt   int    `json:"created_at"`
	Author      string `json:"author,omitempty"`
}

// ToVersionResponse 转换为版本响应
func (v *DocumentVersion) ToVersionResponse() DocumentVersionResponse {
	return DocumentVersionResponse{
		ID:         v.ID,
		DocumentID: v.DocumentID,
		Version:    v.Version,
		Content:    v.Content,
		ChangeLog:  v.ChangeLog,
		CreatedBy:  v.CreatedBy,
		UserID:     v.CreatedBy,
		CreatedAt:  v.CreatedAt,
	}
}
