package rag

import "strings"

// EstimateQueryComplexity 估算查询复杂度（0-100）
// P1-5 (§3.2.1): 用于决定是否走 ReAct 循环驱动检索路径
// 复杂度 > 50 的查询跳过预检索，让 Agent 自主多轮检索
func EstimateQueryComplexity(question string) int {
	q := strings.ToLower(question)
	score := 0

	// 多实体/多主题
	if strings.Count(q, "和") >= 2 || strings.Count(q, "与") >= 2 || strings.Count(q, ",") >= 2 {
		score += 20
	}
	// 对比类查询
	if strings.Contains(q, "对比") || strings.Contains(q, "比较") || strings.Contains(q, "区别") || strings.Contains(q, "差异") {
		score += 20
	}
	// 多步骤/流程类
	if strings.Contains(q, "流程") || strings.Contains(q, "步骤") || strings.Contains(q, "如何") || strings.Contains(q, "怎么") {
		score += 15
	}
	// 分析/统计类
	if strings.Contains(q, "分析") || strings.Contains(q, "统计") || strings.Contains(q, "汇总") || strings.Contains(q, "趋势") {
		score += 15
	}
	// 时间范围
	if strings.Contains(q, "时间") || strings.Contains(q, "期间") || strings.Contains(q, "区间") {
		score += 10
	}
	// 长问题
	if len(q) > 60 {
		score += 10
	}
	// 条件分支
	if strings.Contains(q, "如果") || strings.Contains(q, "或者") || strings.Contains(q, "要么") {
		score += 10
	}

	if score > 100 {
		return 100
	}
	return score
}
