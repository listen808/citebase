package search

import (
	"time"

	modelsearch "github.com/listen808/citebase/internal/model/search"
	searchrepo "github.com/listen808/citebase/internal/repository/search"

	logx "github.com/listen808/citebase/internal/log"
)

// SearchLogEntry 一次搜索的待记录事实。由 API 层在拿到搜索结果后填，
// Query 必须是用户原始输入而非同义词扩展后的查询串——扩展结果形如 `A|B|C`，
// 是内部查询语法，落进热词榜既不可读又把同一个词拆成多条。
type SearchLogEntry struct {
	Query       string
	Source      string
	Type        string
	SpaceID     int
	UserID      int
	ResultCount int
	LatencyMS   int64
}

// RecordSearch 异步落一条搜索底账。
// 语义与 middleware.AccessLog 一致：日志写失败绝不影响搜索结果，也不阻塞响应，
// 因此整段放进 goroutine 并 recover。代价是极端情况下可能少记几条——
// 统计口径宁可少计，不可让检索路径承担写库延迟。
func RecordSearch(e SearchLogEntry) {
	now := time.Now().Unix()
	keyword := modelsearch.NormalizeSearchKeyword(e.Query)
	source := e.Source
	if source == "" {
		source = modelsearch.SearchSourceFulltext
	}
	// Type 原样记录，不做默认：两条检索路径的词表不同——全文检索是
	// all/title/content，混合检索是 hybrid/fulltext/semantic。把空值统一填成
	// "all" 会给混合检索的行写上不属于它词表的值。
	entry := &modelsearch.SearchLog{
		Keyword:     keyword,
		Query:       modelsearch.TruncateRunes(e.Query, modelsearch.MaxQueryLen),
		Source:      source,
		Type:        e.Type,
		SpaceID:     e.SpaceID,
		UserID:      e.UserID,
		ResultCount: e.ResultCount,
		LatencyMS:   e.LatencyMS,
		CreateTime:  now,
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logx.Infof("[Search] record search log panicked: %v", r)
			}
		}()
		if err := searchrepo.NewSearchLogRepository().Create(entry); err != nil {
			logx.Infof("[Search] record search log failed: %v", err)
		}
	}()
}

// searchLogStore 聚合所需的数据访问面。抽成接口是为了让窗口计算（起始时刻、
// 天数、日期标签）能在无 MySQL 的环境下用假实现验证——本机没有数据库，
// 而窗口算错的失败方式是"趋势图整体平移一天"，肉眼几乎发现不了。
type searchLogStore interface {
	Totals(startOfWindow int64) (total, zeroResults, sumLatencyMS int64, err error)
	DailyCounts(startOfWindow int64, days int) (map[int64]int64, error)
	HotWords(startOfWindow int64, limit int) ([]modelsearch.HotSearchWord, error)
	CountBetween(from, to int64) (int64, error)
	UniqueUsers(startOfWindow int64) (int64, error)
}

// SearchAnalytics 搜索统计装配
type SearchAnalytics struct {
	store searchLogStore
	now   func() time.Time
}

// NewSearchAnalytics 使用真实仓储
func NewSearchAnalytics() *SearchAnalytics {
	return &SearchAnalytics{store: searchrepo.NewSearchLogRepository(), now: time.Now}
}

// Statistics 汇总窗口内的搜索统计。任一路查询失败即返回错误：
// 监控页把残缺数据当成完整数据展示，比报错更难发现。
func (a *SearchAnalytics) Statistics(days, hotLimit int) (*modelsearch.SearchStatistics, error) {
	days = modelsearch.ClampTrendDays(days)
	hotLimit = modelsearch.ClampHotWordLimit(hotLimit)

	now := a.now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startOfWindow := todayStart.AddDate(0, 0, -(days - 1))

	stats := &modelsearch.SearchStatistics{
		RangeDays: days,
		Trend:     []modelsearch.SearchTrendPoint{},
		HotWords:  []modelsearch.HotSearchWord{},
	}

	total, zeroResults, sumLatency, err := a.store.Totals(startOfWindow.Unix())
	if err != nil {
		return nil, err
	}
	stats.TotalSearches = total
	stats.NoResultRate = modelsearch.NoResultRate(total, zeroResults)
	stats.AvgLatencyMS = modelsearch.AvgLatency(sumLatency, total)

	counts, err := a.store.DailyCounts(startOfWindow.Unix(), days)
	if err != nil {
		return nil, err
	}
	stats.Trend = modelsearch.FillTrendBuckets(counts, days, startOfWindow)

	hot, err := a.store.HotWords(startOfWindow.Unix(), hotLimit)
	if err != nil {
		return nil, err
	}
	if hot != nil {
		stats.HotWords = hot
	}

	today, err := a.store.CountBetween(todayStart.Unix(), todayStart.AddDate(0, 0, 1).Unix())
	if err != nil {
		return nil, err
	}
	stats.TodaySearches = today

	users, err := a.store.UniqueUsers(startOfWindow.Unix())
	if err != nil {
		return nil, err
	}
	stats.UniqueUsers = users

	return stats, nil
}
