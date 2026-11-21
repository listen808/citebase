package search

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	modelsearch "github.com/listen808/citebase/internal/model/search"
)

// fakeStore 记录调用参数的假仓储。本机没有 MySQL，聚合的正确性只能靠
// "传给 SQL 的窗口起点是不是应用时钟的自然日零点"这一层来钉住——
// 窗口算错的失败方式是趋势图整体平移一天，肉眼几乎发现不了。
type fakeStore struct {
	totalsStart     int64
	dailyStart      int64
	dailyDays       int
	hotStart        int64
	hotLimit        int
	betweenFrom     int64
	betweenTo       int64
	uniqueStart     int64
	totals          func() (int64, int64, int64, error)
	daily           func() (map[int64]int64, error)
	hot             func() ([]modelsearch.HotSearchWord, error)
	countBetween    func() (int64, error)
	uniqueUsers     func() (int64, error)
	totalsCalled    bool
	dailyCalled     bool
	hotCalled       bool
	betweenCalled   bool
	uniqueUsersCall bool
}

func (f *fakeStore) Totals(start int64) (int64, int64, int64, error) {
	f.totalsCalled, f.totalsStart = true, start
	if f.totals != nil {
		return f.totals()
	}
	return 0, 0, 0, nil
}

func (f *fakeStore) DailyCounts(start int64, days int) (map[int64]int64, error) {
	f.dailyCalled, f.dailyStart, f.dailyDays = true, start, days
	if f.daily != nil {
		return f.daily()
	}
	return map[int64]int64{}, nil
}

func (f *fakeStore) HotWords(start int64, limit int) ([]modelsearch.HotSearchWord, error) {
	f.hotCalled, f.hotStart, f.hotLimit = true, start, limit
	if f.hot != nil {
		return f.hot()
	}
	return nil, nil
}

func (f *fakeStore) CountBetween(from, to int64) (int64, error) {
	f.betweenCalled, f.betweenFrom, f.betweenTo = true, from, to
	if f.countBetween != nil {
		return f.countBetween()
	}
	return 0, nil
}

func (f *fakeStore) UniqueUsers(start int64) (int64, error) {
	f.uniqueUsersCall, f.uniqueStart = true, start
	if f.uniqueUsers != nil {
		return f.uniqueUsers()
	}
	return 0, nil
}

// fixedClock 固定"现在"，避免断言依赖真实时钟
func fixedClock(y int, m time.Month, d, hh int) time.Time {
	return time.Date(y, m, d, hh, 0, 0, 0, time.Local)
}

func TestStatisticsWindowStartsAtMidnightNMinusDaysAgo(t *testing.T) {
	store := &fakeStore{}
	a := &SearchAnalytics{store: store, now: func() time.Time { return fixedClock(2026, time.September, 30, 13) }}

	_, err := a.Statistics(7, 10)
	if err != nil {
		t.Fatalf("Statistics error: %v", err)
	}

	wantStart := fixedClock(2026, time.September, 24, 0).Unix() // 含今天往前 7 天
	if store.totalsStart != wantStart {
		t.Errorf("Totals 窗口起点 = %d, want %d (2026-09-24 本地零点)", store.totalsStart, wantStart)
	}
	if store.dailyStart != wantStart || store.dailyDays != 7 {
		t.Errorf("DailyCounts(%d, %d), want (%d, 7)", store.dailyStart, store.dailyDays, wantStart)
	}
	if store.hotStart != wantStart {
		t.Errorf("HotWords 窗口起点 = %d, want %d", store.hotStart, wantStart)
	}
	if store.uniqueStart != wantStart {
		t.Errorf("UniqueUsers 窗口起点 = %d, want %d", store.uniqueStart, wantStart)
	}

	// "今日搜索"必须按自然日零点起算，而不是"过去 24 小时"
	wantTodayFrom := fixedClock(2026, time.September, 30, 0).Unix()
	wantTodayTo := fixedClock(2026, time.October, 1, 0).Unix()
	if !store.betweenCalled || store.betweenFrom != wantTodayFrom || store.betweenTo != wantTodayTo {
		t.Errorf("CountBetween = (%d, %d), want (%d, %d)", store.betweenFrom, store.betweenTo, wantTodayFrom, wantTodayTo)
	}
}

func TestStatisticsRateMath(t *testing.T) {
	store := &fakeStore{
		totals:       func() (int64, int64, int64, error) { return 200, 30, 4000, nil },
		countBetween: func() (int64, error) { return 12, nil },
		uniqueUsers:  func() (int64, error) { return 5, nil },
	}
	a := &SearchAnalytics{store: store, now: func() time.Time { return fixedClock(2026, time.September, 30, 9) }}

	stats, err := a.Statistics(7, 10)
	if err != nil {
		t.Fatalf("Statistics error: %v", err)
	}
	if stats.TotalSearches != 200 {
		t.Errorf("TotalSearches = %d, want 200", stats.TotalSearches)
	}
	if stats.NoResultRate != 15 {
		t.Errorf("NoResultRate = %v, want 15 (30/200)", stats.NoResultRate)
	}
	if stats.AvgLatencyMS != 20 {
		t.Errorf("AvgLatencyMS = %v, want 20 (4000/200)", stats.AvgLatencyMS)
	}
	if stats.TodaySearches != 12 || stats.UniqueUsers != 5 {
		t.Errorf("TodaySearches=%d UniqueUsers=%d, want 12/5", stats.TodaySearches, stats.UniqueUsers)
	}
	if stats.RangeDays != 7 {
		t.Errorf("RangeDays = %d, want 7（应回显收敛后的窗口，前端据此画横轴）", stats.RangeDays)
	}
}

func TestStatisticsTrendLabelsAlignWithWindow(t *testing.T) {
	store := &fakeStore{
		daily: func() (map[int64]int64, error) { return map[int64]int64{0: 1, 6: 7}, nil },
	}
	a := &SearchAnalytics{store: store, now: func() time.Time { return fixedClock(2026, time.September, 30, 23) }}

	stats, err := a.Statistics(7, 10)
	if err != nil {
		t.Fatalf("Statistics error: %v", err)
	}
	if len(stats.Trend) != 7 {
		t.Fatalf("want 7 trend points, got %d", len(stats.Trend))
	}
	if stats.Trend[0].Date != "2026-09-24" || stats.Trend[0].Count != 1 {
		t.Errorf("首点 = %+v, want 2026-09-24/1", stats.Trend[0])
	}
	if stats.Trend[6].Date != "2026-09-30" || stats.Trend[6].Count != 7 {
		t.Errorf("末点（今天）= %+v, want 2026-09-30/7", stats.Trend[6])
	}
	if stats.Trend[1].Count != 0 {
		t.Errorf("无搜索的日子应补 0, got %d", stats.Trend[1].Count)
	}
}

// 空表时把残缺数据当完整数据展示，比报错更难发现——四路查询任一失败都必须抛出
func TestStatisticsPropagatesEveryStoreError(t *testing.T) {
	boom := errors.New("db down")
	cases := map[string]*fakeStore{
		"totals":       {totals: func() (int64, int64, int64, error) { return 0, 0, 0, boom }},
		"daily":        {daily: func() (map[int64]int64, error) { return nil, boom }},
		"hot":          {hot: func() ([]modelsearch.HotSearchWord, error) { return nil, boom }},
		"countBetween": {countBetween: func() (int64, error) { return 0, boom }},
		"uniqueUsers":  {uniqueUsers: func() (int64, error) { return 0, boom }},
	}
	for name, store := range cases {
		t.Run(name, func(t *testing.T) {
			a := &SearchAnalytics{store: store, now: func() time.Time { return fixedClock(2026, time.September, 30, 9) }}
			if _, err := a.Statistics(7, 10); !errors.Is(err, boom) {
				t.Errorf("期望错误向上抛, got %v", err)
			}
		})
	}
}

func TestStatisticsClampsOutOfRangeRequests(t *testing.T) {
	store := &fakeStore{}
	a := &SearchAnalytics{store: store, now: func() time.Time { return fixedClock(2026, time.September, 30, 9) }}

	stats, err := a.Statistics(0, 0)
	if err != nil {
		t.Fatalf("Statistics error: %v", err)
	}
	if stats.RangeDays != 7 || store.hotLimit != 10 {
		t.Errorf("缺省应收敛为 7 天/10 词, got days=%d limit=%d", stats.RangeDays, store.hotLimit)
	}

	if _, err := a.Statistics(100000, 100000); err != nil {
		t.Fatalf("Statistics error: %v", err)
	}
	if store.dailyDays != 365 {
		t.Errorf("超大窗口应被夹到 365, got %d（否则监控页一次点击就全表扫）", store.dailyDays)
	}
	if store.hotLimit != 50 {
		t.Errorf("热词上限应被夹到 50, got %d", store.hotLimit)
	}
}

func TestStatisticsReturnsEmptySlicesNotNil(t *testing.T) {
	a := &SearchAnalytics{store: &fakeStore{}, now: func() time.Time { return fixedClock(2026, time.September, 30, 9) }}
	stats, err := a.Statistics(7, 10)
	if err != nil {
		t.Fatalf("Statistics error: %v", err)
	}
	if stats.Trend == nil || stats.HotWords == nil {
		t.Errorf("JSON 序列化成 null 会让前端必须写可选链, trend=%v hot=%v", stats.Trend, stats.HotWords)
	}
}

func TestIndexDiskFactsMeasuresRealFiles(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "store")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dir, "a")
	newer := filepath.Join(sub, "b")
	oldTime := fixedClock(2026, time.September, 28, 10)
	newerTime := fixedClock(2026, time.September, 30, 8)
	for _, tc := range []struct {
		path string
		mt   time.Time
	}{
		{old, oldTime},
		{newer, newerTime},
	} {
		if err := os.WriteFile(tc.path, []byte("12345"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(tc.path, tc.mt, tc.mt); err != nil {
			t.Fatal(err)
		}
	}

	savedIndexPath = dir
	defer func() { savedIndexPath = "" }()

	size, lastUpdated := IndexDiskFacts()
	if size != 10 {
		t.Errorf("索引大小 = %d, want 10（两个文件各 5 字节，子目录也要递归统计到）", size)
	}
	if lastUpdated != newerTime.Unix() {
		t.Errorf("最后更新 = %d, want %d（取最新段文件，不是最早的那个）", lastUpdated, newerTime.Unix())
	}
}

func TestIndexDiskFactsWithoutInit(t *testing.T) {
	savedIndexPath = ""
	size, lastUpdated := IndexDiskFacts()
	if size != 0 || lastUpdated != 0 {
		t.Errorf("未初始化索引时应返回 0/0, got %d/%d", size, lastUpdated)
	}
}

func TestIndexPathAccessor(t *testing.T) {
	savedIndexPath = "/tmp/whatever"
	defer func() { savedIndexPath = "" }()
	if IndexPath() != "/tmp/whatever" {
		t.Errorf("IndexPath() 应回显已保存的索引路径")
	}
}
