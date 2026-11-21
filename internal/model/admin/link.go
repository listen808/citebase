package admin

import (
	"time"

	"gorm.io/gorm"
)

// Link 快捷链接
type Link struct {
	LinkID   uint   `gorm:"primaryKey;column:link_id;autoIncrement" json:"id"` // P0 修复：前端期望 id，与 Document 等模型保持一致
	Name     string `gorm:"column:name;size:50;not null" json:"name"`
	URL      string `gorm:"column:url;size:500;not null" json:"url"`
	Sort     int    `gorm:"column:sort;type:int;default:0" json:"sort"`
	Sequence int    `gorm:"-" json:"sequence"` // 兼容性字段
	// ClickCount 打开次数。由 POST /api/v1/links/:id/click 原子自增，
	// 只有 AutoMigrate + ensureMissingColumns 会补这一列，databases/table.sql 是生成物、不手改。
	ClickCount int   `gorm:"column:click_count;type:int;default:0" json:"click_count"`
	IsDelete   int   `gorm:"column:is_delete;default:0" json:"-"`
	CreateTime int64 `gorm:"column:create_time" json:"create_time"`
	UpdateTime int64 `gorm:"column:update_time" json:"update_time"`
}

func (l *Link) BeforeCreate(tx *gorm.DB) error {
	now := tx.Statement.Context.Value("now")
	if now == nil {
		now = time.Now().Unix()
	}
	l.CreateTime = now.(int64)
	l.UpdateTime = now.(int64)
	return nil
}

func (l *Link) BeforeUpdate(tx *gorm.DB) error {
	l.UpdateTime = time.Now().Unix()
	return nil
}

func (l *Link) AfterFind(tx *gorm.DB) error {
	l.Sequence = l.Sort
	return nil
}

func (Link) TableName() string {
	return "aw_link"
}
