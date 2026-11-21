package search

import (
	modelsearch "github.com/listen808/citebase/internal/model/search"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

// SearchLogRepository 搜索日志仓储（aw_search_log）
type SearchLogRepository struct {
	db *gorm.DB
}

func NewSearchLogRepository() *SearchLogRepository {
	return &SearchLogRepository{db: repos.DB}
}

// Create 记录一次搜索
func (r *SearchLogRepository) Create(log *modelsearch.SearchLog) error {
	return r.db.Create(log).Error
}

const secondsPerDay = 86400

// DailyCounts 按"应用时钟的自然日"统计窗口内每天的搜索量。
// 这里不用 FROM_UNIXTIME(create_time, '%Y-%m-%d')：那个函数按 MySQL 会话时区
// 切自然日，而 DSN 默认不带 loc，服务端时区与应用时钟不一致时，分桶结果会和
// Go 侧生成的日期标签错位（趋势图表现为整体平移或空白天）。
// 改为由调用方传入 startOfWindow（应用本地零点的 Unix 秒），桶号 =
// (create_time - startOfWindow) DIV 86400，"一天"的定义只有一处来源。
func (r *SearchLogRepository) DailyCounts(startOfWindow int64, days int) (map[int64]int64, error) {
	type row struct {
		DayIndex int64
		Count    int64
	}
	var rows []row
	err := r.db.Model(&modelsearch.SearchLog{}).
		Select("(create_time - ?) DIV ? AS day_index, COUNT(*) AS count", startOfWindow, secondsPerDay).
		Where("create_time >= ?", startOfWindow).
		Group("day_index").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[int64]int64, len(rows))
	for _, x := range rows {
		if x.DayIndex < 0 || x.DayIndex >= int64(days) {
			continue
		}
		out[x.DayIndex] = x.Count
	}
	return out, nil
}

// HotWords 窗口内出现次数最多的搜索词。空 keyword 被排除：那是没有任何输入的
// 探测请求，计入榜单只会产出一个空词条。
func (r *SearchLogRepository) HotWords(startOfWindow int64, limit int) ([]modelsearch.HotSearchWord, error) {
	var rows []modelsearch.HotSearchWord
	err := r.db.Model(&modelsearch.SearchLog{}).
		Select("keyword AS term, COUNT(*) AS count").
		Where("create_time >= ? AND keyword <> ''", startOfWindow).
		Group("keyword").
		Order("count DESC, keyword ASC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// totalsAggregate 窗口内的总量、零结果数、总耗时
type totalsAggregate struct {
	Total        int64
	ZeroResults  int64
	SumLatencyMS int64
}

// Totals 一次查询拿到总数/无结果数/累计耗时，避免三趟全表扫描。
// ZeroResults 用 SUM(result_count = 0)：MySQL 下布尔表达式返回 1/0，可直接求和。
func (r *SearchLogRepository) Totals(startOfWindow int64) (total, zeroResults, sumLatencyMS int64, err error) {
	var row totalsAggregate
	err = r.db.Model(&modelsearch.SearchLog{}).
		Select("COUNT(*) AS total, SUM(result_count = 0) AS zero_results, SUM(latency_ms) AS sum_latency_ms").
		Where("create_time >= ?", startOfWindow).
		Scan(&row).Error
	if err != nil {
		return 0, 0, 0, err
	}
	return row.Total, row.ZeroResults, row.SumLatencyMS, nil
}

// CountBetween 统计 [from, to) 区间内的搜索次数
func (r *SearchLogRepository) CountBetween(from, to int64) (int64, error) {
	var count int64
	err := r.db.Model(&modelsearch.SearchLog{}).
		Where("create_time >= ? AND create_time < ?", from, to).
		Count(&count).Error
	return count, err
}

// UniqueUsers 窗口内发起过搜索的登录用户数（匿名 user_id=0 不计入）
func (r *SearchLogRepository) UniqueUsers(startOfWindow int64) (int64, error) {
	var count int64
	err := r.db.Model(&modelsearch.SearchLog{}).
		Where("create_time >= ? AND user_id > 0", startOfWindow).
		Distinct("user_id").
		Count(&count).Error
	return count, err
}
