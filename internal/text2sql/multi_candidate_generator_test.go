package text2sql

import (
	"strings"
	"testing"
)

// TestStrategyPlanIsCompleteAndClean 把 strategyPlanFor 五个分支的策略清单
// 与 strategyHints 映射逐条对回。此前 " CTE" 带前导空格的策略名在
// buildStrategyPrompt 的 switch 里匹配不上（case 是 "CTE"），提示词为空、
// LLM 在不知道策略约束的情况下自由发挥；该串还会原样进 CandidateSQL.Strategy
// 与"使用%s策略生成"的理由文案。这条测试钉住：
//  1. 每个分支的清单逐字等于登记值（顺序即优先级，不许悄悄换序）；
//  2. 每个策略名非空、无首尾空白；
//  3. 每个策略名在 strategyHints 里都有非空提示词。
func TestStrategyPlanIsCompleteAndClean(t *testing.T) {
	plans := map[string][]string{
		"aggregation": {"basic", "window", "CTE"},
		"join":        {"inner_join", "left_join", "CTE"},
		"subquery":    {"nested", "CTE", "temp_table"},
		"complex":     {"stepwise", "CTE", "union"},
		"":            {"basic", "alternative", "CTE"}, // default 分支
	}

	for queryType, want := range plans {
		t.Run(queryType, func(t *testing.T) {
			got := strategyPlanFor(queryType)
			if len(got) != len(want) {
				t.Fatalf("strategyPlanFor(%q) = %v, want %v", queryType, got, want)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("strategyPlanFor(%q)[%d] = %q, want %q", queryType, i, got[i], want[i])
					break
				}
			}
			for _, s := range got {
				if s == "" || s != strings.TrimSpace(s) {
					t.Errorf("strategy %q must be non-empty and without surrounding whitespace", s)
				}
				if hint := strategyHints[s]; hint == "" {
					t.Errorf("strategy %q has no entry in strategyHints (prompt would be empty)", s)
				}
			}
		})
	}
}

// TestDetectQueryStrategyRouting 钉住 detectQueryStrategy 的关键词→类型映射，
// 保证 strategyPlanFor 拿到的 queryType 是五个登记分支之一。
// 用例只取每类的第一命中关键词（映射本身是"首个命中即返回"，
// 全量关键词清单不在此钉 —— 那是调参面，不是契约面）。
func TestDetectQueryStrategyRouting(t *testing.T) {
	g := &MultiCandidateGenerator{}

	cases := []struct {
		question string
		want     string
	}{
		{"统计每组的累计总和", "aggregation"},
		{"各部门的排名", "aggregation"},
		{"订单和用户表关联", "join"},
		{"所有满足条件的记录", "subquery"},
		{"每月销售额趋势", "complex"},
		{"查一下用户列表", "basic"},
	}
	for _, tc := range cases {
		if got := g.detectQueryStrategy(tc.question); got != tc.want {
			t.Errorf("detectQueryStrategy(%q) = %q, want %q", tc.question, got, tc.want)
		}
	}
}
