package shared

import "sort"

// RRFItem 表示待融合的搜索结果项
type RRFItem struct {
	Key   string  // 去重键（表名、文档ID等）
	Score float64 // 原始分数
	Data  any     // 附加数据
}

// RRFMerge 执行 Reciprocal Rank Fusion 融合
// lists: 待融合的多个结果列表
// k: RRF 常数（默认 60.0）
// limit: 返回的最大结果数
func RRFMerge(lists [][]RRFItem, k float64, limit int) []RRFItem {
	scores := make(map[string]float64)
	items := make(map[string]RRFItem)

	for _, list := range lists {
		for rank, item := range list {
			scores[item.Key] += 1.0 / (k + float64(rank+1))
			if _, exists := items[item.Key]; !exists {
				items[item.Key] = item
			}
		}
	}

	type scored struct {
		key   string
		score float64
	}
	var ranked []scored
	for key, score := range scores {
		ranked = append(ranked, scored{key, score})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	var result []RRFItem
	for i := 0; i < len(ranked) && i < limit; i++ {
		result = append(result, items[ranked[i].key])
	}
	return result
}
