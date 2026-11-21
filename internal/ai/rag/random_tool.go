package rag

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
)

// RandomNumberTool 随机数工具。
// P1 修复（清单 69）：
//  1. 旧实现持有 *rand.Rand 单例字段，而工具经 register.go 注册进全局注册表、
//     会在并行工具调用协程中并发执行（agent_chat.executeToolCallsParallel），
//     rand.Rand 非并发安全 -> 数据竞争。改用 math/rand 包级函数（并发安全、自动播种）。
//  2. min/max 由模型 JSON 提供，maxVal-minVal 超过 int 范围时 int(maxVal-minVal)+1
//     溢出为负 -> rand.Intn 负数 panic（在裸协程中会击穿进程）。现显式钳制范围跨度。
type RandomNumberTool struct{}

func NewRandomNumberTool() *RandomNumberTool {
	return &RandomNumberTool{}
}

func (t *RandomNumberTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "random_number",
		Description: "随机数生成工具：生成指定范围内的整数、浮点数、批量随机数、洗牌等",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"int", "float", "batch", "shuffle", "pick"},
					"description": "操作类型：int=整数，float=浮点数，batch=批量生成，shuffle=洗牌，pick=随机选择",
				},
				"min": map[string]interface{}{
					"type":        "number",
					"description": "最小值，默认0",
				},
				"max": map[string]interface{}{
					"type":        "number",
					"description": "最大值，默认100",
				},
				"count": map[string]interface{}{
					"type":        "integer",
					"description": "生成数量（batch操作），默认5",
				},
				"items": map[string]interface{}{
					"type":        "string",
					"description": "shuffle/pick操作的选项，用逗号分隔",
				},
			},
			"required": []string{"operation"},
		},
	}
}

// maxIntSpan 整数取值跨度上限：超过该值时 int 转换可能溢出/无意义，直接拒绝
const maxIntSpan = int64(1) << 53

func (t *RandomNumberTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	op, _ := params["operation"].(string)
	if op == "" {
		op = "int"
	}

	minVal := 0.0
	if v, ok := params["min"].(float64); ok {
		minVal = v
	}

	maxVal := 100.0
	if v, ok := params["max"].(float64); ok {
		maxVal = v
	}

	if maxVal <= minVal {
		return &ToolResult{
			ToolName: "random_number",
			Success:  false,
			Error:    "max must be greater than min",
		}, nil
	}
	// 防御 NaN/Inf 与 int 溢出（模型可给出 1e18 级别的 max）
	if math.IsNaN(minVal) || math.IsNaN(maxVal) || math.IsInf(minVal, 0) || math.IsInf(maxVal, 0) {
		return &ToolResult{ToolName: "random_number", Success: false, Error: "min/max 必须是有限数值"}, nil
	}
	span := int64(maxVal - minVal)
	if span < 0 || span > maxIntSpan {
		return &ToolResult{ToolName: "random_number", Success: false, Error: "max-min 超出可处理范围（<= 2^53）"}, nil
	}

	var result string
	switch strings.ToLower(op) {
	case "int":
		n := rand.Int63n(span+1) + int64(minVal)
		result = fmt.Sprintf("随机整数: %d (范围: %d~%d)", n, int64(minVal), int64(maxVal))
	case "float":
		n := minVal + rand.Float64()*(maxVal-minVal)
		result = fmt.Sprintf("随机浮点数: %.4f (范围: %.2f~%.2f)", n, minVal, maxVal)
	case "batch":
		count := 5
		if c, ok := params["count"].(float64); ok && c > 0 {
			count = int(c)
		}
		if count > 100 {
			count = 100
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("批量生成 %d 个随机整数 (范围: %d~%d):\n", count, int64(minVal), int64(maxVal)))
		for i := 0; i < count; i++ {
			n := rand.Int63n(span+1) + int64(minVal)
			sb.WriteString(fmt.Sprintf("  [%d] %d\n", i+1, n))
		}
		result = sb.String()
	case "shuffle":
		items, _ := params["items"].(string)
		if items == "" {
			return &ToolResult{
				ToolName: "random_number",
				Success:  false,
				Error:    "items is required for shuffle operation",
			}, nil
		}
		parts := strings.Split(items, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		rand.Shuffle(len(parts), func(i, j int) {
			parts[i], parts[j] = parts[j], parts[i]
		})
		result = fmt.Sprintf("洗牌结果: [%s]", strings.Join(parts, ", "))
	case "pick":
		items, _ := params["items"].(string)
		if items == "" {
			return &ToolResult{
				ToolName: "random_number",
				Success:  false,
				Error:    "items is required for pick operation",
			}, nil
		}
		parts := strings.Split(items, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		idx := rand.Intn(len(parts))
		result = fmt.Sprintf("随机选中: %s (从 %d 个选项中)", parts[idx], len(parts))
	default:
		return &ToolResult{
			ToolName: "random_number",
			Success:  false,
			Error:    "不支持的操作: " + op,
		}, nil
	}

	return &ToolResult{
		ToolName: "random_number",
		Success:  true,
		Result:   result,
	}, nil
}
