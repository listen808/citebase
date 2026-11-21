package text2sql

import "testing"

// TestEstimateComplexityEnglish 钉住 EstimateComplexity 对英文问题的计分。
// 关键词表是中英混排（join / cte / group by / over / row_number ...），
// 此前测试只覆盖中文用例，英文侧的关键词命中与分值漂移没有判据；
// 这里按当前实现把每条英文关键词的分值钉死，改实现必须先改这张表
// （或反过来：调分值时测试会指明是哪一条变了）。
func TestEstimateComplexityEnglish(t *testing.T) {
	cases := []struct {
		name string
		q    string
		want int
	}{
		// 基线：无特征英文问题
		{"no features", "list all users", 0},
		// 单关键词各自的分值
		{"join", "join the orders and customers tables", 20},
		{"cte", "use a cte to precompute totals", 20},
		{"with", "with a recursive query walk the tree", 20},
		{"group by", "group by department for the report", 15},
		{"over", "over the last quarter sales", 25},
		{"row_number", "row_number partitioned by customer", 25},
		{"rank", "rank employees by revenue", 25},
		// 组合叠加
		{"join plus group by", "join orders to customers then group by region", 35},
		// 长度加分（>80 字符 +10）
		{"long question", "join orders to customers and then group by region and month for the last quarter to show the trend", 45},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EstimateComplexity(tc.q); got != tc.want {
				t.Errorf("EstimateComplexity(%q) = %d, want %d", tc.q, got, tc.want)
			}
		})
	}
}
