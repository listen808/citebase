package rag

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type CalculateTool struct{}

func NewCalculateTool() *CalculateTool {
	return &CalculateTool{}
}

func (t *CalculateTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "calculate",
		Description: "执行数学计算。支持基本运算（加减乘除）、百分比、平均值、求和、最大最小值",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"expression": map[string]interface{}{
					"type":        "string",
					"description": "数学表达式或函数，如 '2+3*4'、'avg'、'sum'、'percent(25,100)'",
				},
				"numbers": map[string]interface{}{
					"type":        "array",
					"description": "数字列表，用于 avg/sum/min/max 等聚合函数",
					"items":       map[string]interface{}{"type": "number"},
				},
			},
			"required": []string{"expression"},
		},
	}
}

func (t *CalculateTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	expression, _ := params["expression"].(string)
	if expression == "" {
		return &ToolResult{ToolName: "calculate", Success: false, Error: "expression is required"}, nil
	}

	var numbers []float64
	if nums, ok := params["numbers"].([]interface{}); ok {
		for _, n := range nums {
			if f, ok := n.(float64); ok {
				numbers = append(numbers, f)
			}
		}
	}

	expression = strings.ToLower(strings.TrimSpace(expression))

	var result float64
	var err error

	switch {
	case expression == "avg" || (strings.HasPrefix(expression, "avg(") && strings.HasSuffix(expression, ")")):
		if len(numbers) == 0 {
			return &ToolResult{ToolName: "calculate", Success: false, Error: "avg 需要 numbers 参数"}, nil
		}
		result = calcAvg(numbers)
	case expression == "sum" || (strings.HasPrefix(expression, "sum(") && strings.HasSuffix(expression, ")")):
		result = calcSum(numbers)
	case expression == "min" || (strings.HasPrefix(expression, "min(") && strings.HasSuffix(expression, ")")):
		if len(numbers) == 0 {
			return &ToolResult{ToolName: "calculate", Success: false, Error: "min 需要 numbers 参数"}, nil
		}
		result = calcMin(numbers)
	case expression == "max" || (strings.HasPrefix(expression, "max(") && strings.HasSuffix(expression, ")")):
		if len(numbers) == 0 {
			return &ToolResult{ToolName: "calculate", Success: false, Error: "max 需要 numbers 参数"}, nil
		}
		result = calcMax(numbers)
	case strings.HasPrefix(expression, "percent("):
		result, err = calcPercent(expression)
	default:
		result, err = simpleEval(expression)
	}

	if err != nil {
		return &ToolResult{ToolName: "calculate", Success: false, Error: err.Error()}, nil
	}

	return &ToolResult{
		ToolName: "calculate",
		Success:  true,
		Result:   fmt.Sprintf("计算结果: %s = %g", expression, result),
	}, nil
}

func calcAvg(nums []float64) float64 {
	if len(nums) == 0 {
		return 0
	}
	return calcSum(nums) / float64(len(nums))
}

func calcSum(nums []float64) float64 {
	var s float64
	for _, n := range nums {
		s += n
	}
	return s
}

func calcMin(nums []float64) float64 {
	m := nums[0]
	for _, n := range nums[1:] {
		if n < m {
			m = n
		}
	}
	return m
}

func calcMax(nums []float64) float64 {
	m := nums[0]
	for _, n := range nums[1:] {
		if n > m {
			m = n
		}
	}
	return m
}

func calcPercent(expr string) (float64, error) {
	inner := strings.TrimPrefix(expr, "percent(")
	inner = strings.TrimSuffix(inner, ")")
	parts := strings.Split(inner, ",")
	if len(parts) != 2 {
		return 0, fmt.Errorf("percent 需要两个参数: percent(部分, 总数)")
	}
	part, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, fmt.Errorf("第一个参数无效: %v", err)
	}
	total, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, fmt.Errorf("第二个参数无效: %v", err)
	}
	if total == 0 {
		return 0, fmt.Errorf("总数不能为0")
	}
	return (part / total) * 100, nil
}

func simpleEval(expr string) (float64, error) {
	expr = strings.ReplaceAll(expr, " ", "")

	for _, op := range []string{"+", "-", "*", "/"} {
		idx := strings.LastIndex(expr, op)
		if idx > 0 {
			left, err1 := strconv.ParseFloat(expr[:idx], 64)
			right, err2 := strconv.ParseFloat(expr[idx+len(op):], 64)
			if err1 == nil && err2 == nil {
				switch op {
				case "+":
					return left + right, nil
				case "-":
					return left - right, nil
				case "*":
					return left * right, nil
				case "/":
					if right == 0 {
						return 0, fmt.Errorf("除数不能为0")
					}
					return left / right, nil
				}
			}
		}
	}

	val, err := strconv.ParseFloat(expr, 64)
	if err != nil {
		return math.NaN(), fmt.Errorf("无法解析表达式: %s", expr)
	}
	return val, nil
}
