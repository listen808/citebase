package rag

import (
	"math"
)

// BudgetAllocation Token预算分配配置
type BudgetAllocation struct {
	SystemPromptTokens     int // 系统提示预留 token
	QuestionTokens         int // 问题本身 token
	HistoryTokens          int // 对话历史 token
	AvailableContextTokens int // 可用于其他内容的 token
}

// ContextBudgetManager Token预算管理器
// 精确控制各部分Token使用，避免超出模型上下文限制
type ContextBudgetManager struct {
	maxContextTokens  int
	systemPromptRatio float64 // 系统提示占比（默认 5%）
	historyRatio      float64 // 历史记录占比（默认 20%）
	reservedRatio     float64 // 预留空间占比（默认 5%）
}

// NewContextBudgetManager 创建Token预算管理器
func NewContextBudgetManager(maxTokens int) *ContextBudgetManager {
	return &ContextBudgetManager{
		maxContextTokens:  maxTokens,
		systemPromptRatio: 0.05,
		historyRatio:      0.20,
		reservedRatio:     0.05,
	}
}

// NewContextBudgetManagerWithConfig 创建带自定义配置的预算管理器
func NewContextBudgetManagerWithConfig(maxTokens int, systemRatio, historyRatio, reservedRatio float64) *ContextBudgetManager {
	return &ContextBudgetManager{
		maxContextTokens:  maxTokens,
		systemPromptRatio: systemRatio,
		historyRatio:      historyRatio,
		reservedRatio:     reservedRatio,
	}
}

// Allocate 计算Token预算分配
// 返回: BudgetAllocation 包含各部分预算
func (c *ContextBudgetManager) Allocate(questionTokens, historyTokens, systemPromptTokens int) *BudgetAllocation {
	// 计算各部分预算
	historyBudget := int(math.Ceil(float64(c.maxContextTokens) * c.historyRatio))
	reservedBudget := int(math.Ceil(float64(c.maxContextTokens) * c.reservedRatio))

	// 可用于上下文的总 token 数
	totalBudget := c.maxContextTokens - reservedBudget

	// 计算已使用的 token
	usedTokens := systemPromptTokens + questionTokens + historyTokens

	// 如果已使用超过预算，按比例缩减
	if usedTokens > totalBudget {
		// 计算需要缩减的比例
		excess := usedTokens - totalBudget
		available := totalBudget - systemPromptTokens - questionTokens

		if available > 0 && excess > 0 {
			// 按比例缩减历史
			historyBudget = int(math.Max(0, float64(historyBudget-excess)))
		}
	}

	availableContext := totalBudget - systemPromptTokens - questionTokens
	if availableContext < 0 {
		availableContext = 0
	}

	return &BudgetAllocation{
		SystemPromptTokens:     systemPromptTokens,
		QuestionTokens:         questionTokens,
		HistoryTokens:          min(historyBudget, historyTokens),
		AvailableContextTokens: min(availableContext, c.maxContextTokens-usedTokens),
	}
}

// CalculateReduction 计算需要缩减的 token 数
func (c *ContextBudgetManager) CalculateReduction(currentTokens, targetTokens int) int {
	if currentTokens <= targetTokens {
		return 0
	}
	return currentTokens - targetTokens
}

// TruncateByBudget 根据预算截断内容
func (c *ContextBudgetManager) TruncateByBudget(text string, maxTokens int, t TokenCounter) string {
	currentTokens := t.CountTokens(text)
	if currentTokens <= maxTokens {
		return text
	}

	// 需要截断
	targetChars := int(float64(len(text)) * float64(maxTokens) / float64(currentTokens))
	if targetChars <= 0 {
		return "..."
	}

	return text[:targetChars] + "..."
}

// OptimizeDistribution 优化Token分配
// 优先保证问题、摘要的质量，适当压缩历史
func (c *ContextBudgetManager) OptimizeDistribution(questionTokens, historyTokens, contextTokens int) *BudgetAllocation {
	totalBudget := c.maxContextTokens

	// 固定分配给问题的 token
	questionBudget := min(questionTokens, totalBudget/4) // 问题最多占 25%
	if questionTokens > questionBudget {
		questionBudget = questionTokens
	}

	// 预留空间
	reservedBudget := int(math.Ceil(float64(c.maxContextTokens) * c.reservedRatio))

	// 剩余空间分配给历史和上下文
	remaining := totalBudget - questionBudget - reservedBudget

	// 历史和上下文按比例分配
	historyRatio := float64(historyTokens) / float64(historyTokens+contextTokens)
	if math.IsNaN(historyRatio) {
		historyRatio = 0.5
	}

	historyBudget := int(float64(remaining) * historyRatio)
	contextBudget := remaining - historyBudget

	return &BudgetAllocation{
		SystemPromptTokens:     0, // 由调用方设置
		QuestionTokens:         questionBudget,
		HistoryTokens:          historyBudget,
		AvailableContextTokens: contextBudget,
	}
}

// IsWithinBudget 检查是否在预算范围内
func (c *ContextBudgetManager) IsWithinBudget(totalTokens int) bool {
	return totalTokens <= c.maxContextTokens
}

// GetBudgetStatus 获取预算状态
type BudgetStatus struct {
	TotalBudget  int
	UsedTokens   int
	Remaining    int
	UsagePercent float64
	IsOverBudget bool
}

// GetBudgetStatus 获取当前预算使用状态
func (c *ContextBudgetManager) GetBudgetStatus(usedTokens int) *BudgetStatus {
	remaining := c.maxContextTokens - usedTokens
	if remaining < 0 {
		remaining = 0
	}

	return &BudgetStatus{
		TotalBudget:  c.maxContextTokens,
		UsedTokens:   usedTokens,
		Remaining:    remaining,
		UsagePercent: float64(usedTokens) / float64(c.maxContextTokens) * 100,
		IsOverBudget: usedTokens > c.maxContextTokens,
	}
}

// AdjustRatios 动态调整分配比例
func (c *ContextBudgetManager) AdjustRatios(questionLength, historyLength, contextLength int) {
	total := questionLength + historyLength + contextLength
	if total == 0 {
		return
	}

	// 根据实际长度动态调整
	// 如果问题很长，减少历史占比
	if questionLength > total/2 {
		c.historyRatio = 0.10
	} else if historyLength > contextLength*2 {
		// 如果历史很长，进一步压缩
		c.historyRatio = 0.15
	} else {
		c.historyRatio = 0.20
	}
}

// TokenCounter Token计数接口
type TokenCounter interface {
	CountTokens(text string) int
}

// SimpleTokenCounter 简单Token计数器
type SimpleTokenCounter struct{}

func (s *SimpleTokenCounter) CountTokens(text string) int {
	tokens := 0
	for _, r := range text {
		if r > 0x7F {
			tokens += 2
		} else {
			tokens++
		}
	}
	return tokens / 4
}
