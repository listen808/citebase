package workspace

import (
	"time"

	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

// DocumentViewLogRepository 文档访问日志仓储
type DocumentViewLogRepository struct {
	db *gorm.DB
}

func NewDocumentViewLogRepository() *DocumentViewLogRepository {
	return &DocumentViewLogRepository{db: repos.DB}
}

// Create 记录一次文档访问
func (r *DocumentViewLogRepository) Create(log *workspacemodel.DocumentViewLog) error {
	return r.db.Create(log).Error
}

// GetViewStats 获取文档访问统计
func (r *DocumentViewLogRepository) GetViewStats(documentID int) (*workspacemodel.DocumentViewStats, error) {
	stats := &workspacemodel.DocumentViewStats{DocumentID: documentID}

	// 总访问量
	if err := r.db.Model(&workspacemodel.DocumentViewLog{}).
		Where("document_id = ?", documentID).
		Count(&stats.TotalViews).Error; err != nil {
		return nil, err
	}

	// 独立访客数（按 user_id 去重，未登录用户 user_id=0 也算一个）
	if err := r.db.Model(&workspacemodel.DocumentViewLog{}).
		Where("document_id = ? AND user_id > 0", documentID).
		Distinct("user_id").
		Count(&stats.UniqueVisitors).Error; err != nil {
		return nil, err
	}
	// 加上匿名访客（如果有的话）
	var anonCount int64
	r.db.Model(&workspacemodel.DocumentViewLog{}).
		Where("document_id = ? AND user_id = 0", documentID).
		Distinct("ip").
		Count(&anonCount)
	stats.UniqueVisitors += anonCount

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekStart := todayStart.AddDate(0, 0, -int(now.Weekday()))

	// 今日访问量
	if err := r.db.Model(&workspacemodel.DocumentViewLog{}).
		Where("document_id = ? AND viewed_at >= ?", documentID, todayStart.Unix()).
		Count(&stats.TodayViews).Error; err != nil {
		return nil, err
	}

	// 本周访问量
	if err := r.db.Model(&workspacemodel.DocumentViewLog{}).
		Where("document_id = ? AND viewed_at >= ?", documentID, weekStart.Unix()).
		Count(&stats.WeekViews).Error; err != nil {
		return nil, err
	}

	// 最近7天趋势
	type dailyResult struct {
		Day   string
		Count int64
	}
	var results []dailyResult
	sevenDaysAgo := now.AddDate(0, 0, -6)
	sevenDaysStart := time.Date(sevenDaysAgo.Year(), sevenDaysAgo.Month(), sevenDaysAgo.Day(), 0, 0, 0, 0, sevenDaysAgo.Location())

	err := r.db.Model(&workspacemodel.DocumentViewLog{}).
		Select("FROM_UNIXTIME(viewed_at, '%Y-%m-%d') as day, COUNT(*) as count").
		Where("document_id = ? AND viewed_at >= ?", documentID, sevenDaysStart.Unix()).
		Group("day").
		Order("day ASC").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	// 填充7天数据（含无访问的天）
	dayMap := make(map[string]int64)
	for _, r := range results {
		dayMap[r.Day] = r.Count
	}
	stats.DailyViews = make([]workspacemodel.DailyViewCount, 7)
	for i := 0; i < 7; i++ {
		day := sevenDaysStart.AddDate(0, 0, i)
		dayStr := day.Format("2006-01-02")
		stats.DailyViews[i] = workspacemodel.DailyViewCount{
			Date:  dayStr,
			Count: dayMap[dayStr],
		}
	}

	return stats, nil
}
