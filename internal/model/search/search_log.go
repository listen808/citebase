package search

import (
	"strings"
	"time"
)

// SearchLog 一次搜索请求的底账。
// 索引监控页的趋势、热搜词、无结果率、平均耗时都只能从逐条记录聚合出来：
// Bleve 的 DocCount 不含时间维度，也无法回答"昨天搜了什么没搜到"。
type SearchLog struct {
	ID          int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Keyword     string `gorm:"column:keyword;size:100;not null;default:''" json:"keyword"`
	Query       string `gorm:"column:query;size:500;not null;default:''" json:"query"`
	Source      string `gorm:"column:source;size:20;not null;default:'fulltext'" json:"source"`
	Type        string `gorm:"column:type;size:20;not null;default:''" json:"type"`
	SpaceID     int    `gorm:"column:space_id;not null;default:0" json:"space_id"`
	UserID      int    `gorm:"column:user_id;not null;default:0" json:"user_id"`
	ResultCount int    `gorm:"column:result_count;not null;default:0" json:"result_count"`
	LatencyMS   int64  `gorm:"column:latency_ms;not null;default:0" json:"latency_ms"`
	CreateTime  int64  `gorm:"column:create_time;not null;default:0" json:"create_time"`
}

// TableName 表名
func (SearchLog) TableName() string {
	return "aw_search_log"
}

// 搜索来源常量：写入与聚合共用这套值，避免拼写漂移导致统计漏计。
// suggest 不记账——它是随键入触发的补全，逐条落库只会用输入过程淹没真实检索量。
const (
	SearchSourceFulltext = "fulltext"
	SearchSourceHybrid   = "hybrid"
)

// SearchTrendPoint 单日搜索量
type SearchTrendPoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// HotSearchWord 热门搜索词及出现次数
type HotSearchWord struct {
	Term  string `json:"term"`
	Count int64  `json:"count"`
}

// SearchStatistics 搜索统计总览（索引监控页的数据源）
type SearchStatistics struct {
	RangeDays     int                `json:"range_days"`
	TotalSearches int64              `json:"total_searches"`
	TodaySearches int64              `json:"today_searches"`
	NoResultRate  float64            `json:"no_result_rate"`
	AvgLatencyMS  float64            `json:"avg_latency_ms"`
	UniqueUsers   int64              `json:"unique_users"`
	Trend         []SearchTrendPoint `json:"trend"`
	HotWords      []HotSearchWord    `json:"hot_words"`
}

const (
	// maxKeywordLen 归一化后保留的长度。超长查询计入热词时截断，否则一整段粘贴
	// 文本会被当成一个独立词条，热词榜会被它淹没。
	maxKeywordLen = 100
	// maxTrendDays 趋势窗口上限，防止前端传入巨大区间把日志表全表扫描。
	maxTrendDays = 365
	// maxHotWords 热词榜上限。
	maxHotWords = 50
	// MaxQueryLen 与 aw_search_log.query 的 varchar(500) 对齐，供写入方截断
	MaxQueryLen = 500
)

// TruncateRunes 按字符数截断。必须按 rune 而不是字节：utf8mb4 下 varchar(500)
// 计的是字符数，而按字节切会在多字节中文中间断开，产出非法 UTF-8，
// MySQL 严格模式会直接拒绝这条写入。
func TruncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// NormalizeSearchKeyword 把用户原始查询归一化成可聚合的热词键。
// 取原始 query 而非同义词扩展后的查询串：扩展结果是 `A|B|C` 这类内部语法，
// 记进热词榜既不可读，还会把同一个词的多个变体拆成好几条。
func NormalizeSearchKeyword(raw string) string {
	s := strings.Join(strings.Fields(strings.ToLower(raw)), " ")
	if s == "" {
		return ""
	}
	return TruncateRunes(s, maxKeywordLen)
}

// ClampTrendDays 把请求天数收敛到合法区间，非法或缺省按 7 天。
func ClampTrendDays(days int) int {
	switch {
	case days <= 0:
		return 7
	case days > maxTrendDays:
		return maxTrendDays
	default:
		return days
	}
}

// ClampHotWordLimit 把热词条数收敛到合法区间，非法或缺省按 10 条。
func ClampHotWordLimit(limit int) int {
	switch {
	case limit <= 0:
		return 10
	case limit > maxHotWords:
		return maxHotWords
	default:
		return limit
	}
}

// FillTrendBuckets 把"只含有搜索的那些天"补成连续日期序列。
// GROUP BY 的结果天生缺日，直接交给前端画图会跳过空白天：七天的窗口只剩三根柱子，
// 且横轴标签错位。dayCounts 以「相对窗口起点的第几天」（0 起）为键，与仓储层
// (create_time - startOfWindow) DIV 86400 的分桶口径一致——两处必须同一把尺，
// 所以键的语义在这里定义一次，不再由调用方先转成日期字符串。
func FillTrendBuckets(dayCounts map[int64]int64, days int, start time.Time) []SearchTrendPoint {
	if days <= 0 {
		return []SearchTrendPoint{}
	}
	out := make([]SearchTrendPoint, 0, days)
	for i := 0; i < days; i++ {
		day := start.AddDate(0, 0, i)
		out = append(out, SearchTrendPoint{
			Date:  day.Format("2006-01-02"),
			Count: dayCounts[int64(i)],
		})
	}
	return out
}

// NoResultRate 无结果率（百分比 0-100）。total 为 0 时返回 0 而不是 NaN：
// 前端把该值直接拼成 "x%" 展示，NaN 会渲染成 "NaN%"。
func NoResultRate(total, zeroResult int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(zeroResult) / float64(total) * 100
}

// AvgLatency 平均耗时（毫秒）。total 为 0 时返回 0。
func AvgLatency(totalLatencyMS, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(totalLatencyMS) / float64(total)
}
