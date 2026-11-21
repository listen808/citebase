package text2sql

import "testing"

func TestEstimateComplexity(t *testing.T) {
	simple := EstimateComplexity("查询所有用户")
	if simple != 0 {
		t.Errorf("simple query complexity = %d, want 0", simple)
	}

	withJoin := EstimateComplexity("关联订单表和用户表")
	if withJoin < 20 {
		t.Errorf("join query complexity = %d, want >= 20", withJoin)
	}

	// 窗口函数得分最高（25）
	window := EstimateComplexity("使用 row_number 窗口函数排名")
	if window < 25 {
		t.Errorf("window query complexity = %d, want >= 25", window)
	}
}

func TestEstimateComplexityCap(t *testing.T) {
	// 组合多个高分特征应被限制在 100
	q := "join 关联 子查询 with cte group by 分组聚合统计 窗口 over row_number rank 同比 环比 趋势"
	got := EstimateComplexity(q)
	if got > 100 {
		t.Errorf("complexity should be capped at 100, got %d", got)
	}
	if got != 100 {
		t.Errorf("high-complexity query should reach cap 100, got %d", got)
	}
}

func TestEstimateComplexityMonotonic(t *testing.T) {
	base := EstimateComplexity("统计销量")
	more := EstimateComplexity("统计销量并按月分组关联产品表")
	if more <= base {
		t.Errorf("more complex query (%d) should score higher than base (%d)", more, base)
	}
}
