package workspace

// DocumentViewLog 文档访问日志（阅读量统计）
type DocumentViewLog struct {
	ID         int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	DocumentID int    `gorm:"column:document_id;index:idx_document_id" json:"document_id"`
	SpaceID    int    `gorm:"column:space_id;index:idx_space_id" json:"space_id"`
	UserID     int    `gorm:"column:user_id;index:idx_user_id" json:"user_id"`
	IP         string `gorm:"column:ip;size:45" json:"ip"`
	UserAgent  string `gorm:"column:user_agent;size:500" json:"user_agent"`
	ViewedAt   int    `gorm:"column:viewed_at;index:idx_viewed_at" json:"viewed_at"`
}

func (DocumentViewLog) TableName() string {
	return "aw_document_view_log"
}

// DocumentViewStats 文档访问统计响应
type DocumentViewStats struct {
	DocumentID     int   `json:"document_id"`
	TotalViews     int64 `json:"total_views"`
	UniqueVisitors int64 `json:"unique_visitors"`
	TodayViews     int64 `json:"today_views"`
	WeekViews      int64 `json:"week_views"`
	// 最近7天趋势
	DailyViews []DailyViewCount `json:"daily_views"`
}

// DailyViewCount 每日访问量
type DailyViewCount struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}
