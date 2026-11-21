package knowledge

import "time"

// WikiPageHistory 页面版本历史。
//
// 每次已发布版本被替换时落一行（旧版本的内容），回滚 = 以历史内容为基线再发布一次。
// 只在 version >= 2 时记录：v1 由 Create 产生，无"被替换的前一版"可言。
type WikiPageHistory struct {
	ID          int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SpaceID     int       `gorm:"column:space_id;not null;index:idx_space_id" json:"space_id"`
	Slug        string    `gorm:"column:slug;size:190;not null" json:"slug"`
	Version     int       `gorm:"column:version;not null" json:"version"`
	Title       string    `gorm:"column:title;size:255;not null;default:''" json:"title"`
	Summary     string    `gorm:"column:summary;type:text" json:"summary"`
	Content     string    `gorm:"column:content;type:longtext" json:"content"`
	Sources     IntArray  `gorm:"column:sources;type:text" json:"sources"`
	Provenance  string    `gorm:"column:provenance;size:16;not null;default:'llm'" json:"provenance"`
	ContentHash string    `gorm:"column:content_hash;size:64;not null;default:''" json:"content_hash"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (WikiPageHistory) TableName() string { return "aw_wiki_page_history" }
