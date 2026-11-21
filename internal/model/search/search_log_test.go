package search

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestNormalizeSearchKeyword(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"trim与折叠空白", "  数据库   备份  ", "数据库 备份"},
		{"大小写归一", "MySQL Backup", "mysql backup"},
		{"制表符换行都算空白", "a\tb\nc", "a b c"},
		{"空输入", "", ""},
		{"纯空白", "   \t ", ""},
		{"中文按rune截断不产生非法UTF-8", strings.Repeat("知", 150), strings.Repeat("知", 100)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeSearchKeyword(tc.in)
			if got != tc.want {
				t.Errorf("NormalizeSearchKeyword(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if !isUTF8Valid(got) {
				t.Errorf("结果不是合法 UTF-8，MySQL 严格模式会拒绝写入: %q", got)
			}
		})
	}
}

func TestTruncateRunesKeepsMultibyteIntact(t *testing.T) {
	in := "知识库检索趋势"
	got := TruncateRunes(in, 4)
	if got != "知识库检" {
		t.Fatalf("TruncateRunes 应按字符截断, got %q", got)
	}
	if !isUTF8Valid(got) {
		t.Errorf("截断结果非法 UTF-8: %q", got)
	}
	// 短于上限时原样返回（同一字符串，不做多余拷贝改写）
	if TruncateRunes(in, 100) != in {
		t.Error("未超长不应改变内容")
	}
}

func TestClampTrendDays(t *testing.T) {
	for _, tc := range []struct{ in, want int }{
		{0, 7}, {-3, 7}, {1, 1}, {7, 7}, {365, 365}, {366, 365}, {99999, 365},
	} {
		if got := ClampTrendDays(tc.in); got != tc.want {
			t.Errorf("ClampTrendDays(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestClampHotWordLimit(t *testing.T) {
	for _, tc := range []struct{ in, want int }{
		{0, 10}, {-1, 10}, {3, 3}, {50, 50}, {51, 50}, {1000, 50},
	} {
		if got := ClampHotWordLimit(tc.in); got != tc.want {
			t.Errorf("ClampHotWordLimit(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// 仓储层的分桶口径是 (create_time - startOfWindow) DIV 86400，即"相对窗口起点的第几天"。
// FillTrendBuckets 必须用同一把尺，否则缺白天补 0 会整体错位。
func TestFillTrendBucketsFillsMissingDays(t *testing.T) {
	start := time.Date(2026, 9, 24, 0, 0, 0, 0, time.Local)
	counts := map[int64]int64{0: 5, 2: 9} // 第0天和第2天有搜索，第1天没有

	got := FillTrendBuckets(counts, 7, start)

	if len(got) != 7 {
		t.Fatalf("7 天窗口应产出 7 个点, got %d", len(got))
	}
	wantDates := []string{"2026-09-24", "2026-09-25", "2026-09-26", "2026-09-27", "2026-09-28", "2026-09-29", "2026-09-30"}
	wantCounts := []int64{5, 0, 9, 0, 0, 0, 0}
	for i, p := range got {
		if p.Date != wantDates[i] {
			t.Errorf("第%d个点日期 = %q, want %q", i, p.Date, wantDates[i])
		}
		if p.Count != wantCounts[i] {
			t.Errorf("%s 数量 = %d, want %d（缺日必须补 0，不能让前端跳过）", p.Date, p.Count, wantCounts[i])
		}
	}
}

func TestFillTrendBucketsIgnoresOutOfRange(t *testing.T) {
	start := time.Date(2026, 9, 24, 0, 0, 0, 0, time.Local)
	got := FillTrendBuckets(map[int64]int64{7: 100, -1: 50, 3: 1}, 7, start)
	if len(got) != 7 {
		t.Fatalf("want 7 points, got %d", len(got))
	}
	for _, p := range got {
		if p.Date == "2026-10-01" || p.Date == "2026-09-23" {
			t.Errorf("窗口外的桶被写进了结果: %s", p.Date)
		}
	}
	if got[3].Count != 1 {
		t.Errorf("窗口内的桶丢失: %+v", got)
	}
}

func TestFillTrendBucketsNonPositiveDays(t *testing.T) {
	got := FillTrendBuckets(map[int64]int64{0: 1}, 0, time.Now())
	if len(got) != 0 {
		t.Errorf("days<=0 应返回空序列, got %d", len(got))
	}
	if got == nil {
		t.Error("应返回空切片而非 nil，前端直接 .length 不能靠可选链兜着")
	}
}

func TestNoResultRateNeverReturnsNaN(t *testing.T) {
	if got := NoResultRate(0, 0); got != 0 {
		t.Errorf("无数据时应为 0, got %v", got)
	}
	if got := NoResultRate(10, 3); got != 30 {
		t.Errorf("10 次里 3 次无结果应为 30%%, got %v", got)
	}
	if got := NoResultRate(3, 0); got != 0 {
		t.Errorf("全有结果应为 0, got %v", got)
	}
}

func TestAvgLatencyNeverReturnsNaN(t *testing.T) {
	if got := AvgLatency(0, 0); got != 0 {
		t.Errorf("无数据时应为 0, got %v", got)
	}
	if got := AvgLatency(1500, 10); got != 150 {
		t.Errorf("1500ms/10 次应为 150, got %v", got)
	}
}

func TestSearchLogTableNameHasPrefix(t *testing.T) {
	// 全库表名统一 aw_ 前缀，改名要同时动 model、手写 SQL 和 databases/*.sql
	if got := (SearchLog{}).TableName(); got != "aw_search_log" {
		t.Errorf("TableName() = %q, want aw_search_log", got)
	}
}

func isUTF8Valid(s string) bool {
	// 必须用标准库：`for range` 会把非法字节静默替换成 U+FFFD，自己遍历反而查不出来
	return utf8.ValidString(s)
}
