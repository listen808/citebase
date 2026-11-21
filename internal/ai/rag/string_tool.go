package rag

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type StringManipulateTool struct{}

func NewStringManipulateTool() *StringManipulateTool {
	return &StringManipulateTool{}
}

func (t *StringManipulateTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "string_manipulate",
		Description: "字符串处理工具：长度计算、截取、大小写转换、查找、替换、去空格等",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"operation": map[string]interface{}{
					"type": "string",
					"enum": []string{
						"length", "upper", "lower", "title",
						"substring", "trim", "trim_space",
						"replace", "contains", "count",
						"reverse", "split",
					},
					"description": "操作类型",
				},
				"text": map[string]interface{}{
					"type":        "string",
					"description": "输入文本",
				},
				"start": map[string]interface{}{
					"type":        "integer",
					"description": "substring操作的起始位置",
				},
				"end": map[string]interface{}{
					"type":        "integer",
					"description": "substring操作的结束位置",
				},
				"old": map[string]interface{}{
					"type":        "string",
					"description": "replace操作的旧字符串",
				},
				"new": map[string]interface{}{
					"type":        "string",
					"description": "replace操作的新字符串",
				},
				"search": map[string]interface{}{
					"type":        "string",
					"description": "contains/count操作中要搜索的字符串",
				},
				"separator": map[string]interface{}{
					"type":        "string",
					"description": "split操作的分隔符",
				},
			},
			"required": []string{"operation", "text"},
		},
	}
}

func (t *StringManipulateTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	op, _ := params["operation"].(string)
	text, _ := params["text"].(string)

	if op == "" {
		return &ToolResult{
			ToolName: "string_manipulate",
			Success:  false,
			Error:    "operation is required",
		}, nil
	}

	var result string
	switch strings.ToLower(op) {
	case "length":
		result = fmt.Sprintf("长度: %d 字符 (含空格)", len(text))
		runeCount := len([]rune(text))
		if runeCount != len(text) {
			result += fmt.Sprintf(" (%d 个 Unicode 字符)", runeCount)
		}
	case "upper":
		result = strings.ToUpper(text)
	case "lower":
		result = strings.ToLower(text)
	case "title":
		result = toTitleCase(text)
	case "substring":
		start := 0
		if s, ok := params["start"].(float64); ok {
			start = int(s)
		}
		end := len(text)
		if e, ok := params["end"].(float64); ok {
			end = int(e)
		}
		runes := []rune(text)
		// P1 修复（清单 20）：start/end 均来自模型输出，旧实现只钳了 start 下界与
		// end 上界，start>end 交换后即可产生 start>len(runes) 或负下标，
		// runes[start:end] 直接 slice 越界 panic（工具跑在无 recover 的协程里会击穿进程）。
		// 交换前、交换后各做一次 [0, len(runes)] 双向钳制。
		if start < 0 {
			start = 0
		}
		if start > len(runes) {
			start = len(runes)
		}
		if end < 0 {
			end = 0
		}
		if end > len(runes) {
			end = len(runes)
		}
		if start > end {
			start, end = end, start
		}
		if end > len(runes) {
			end = len(runes)
		}
		result = string(runes[start:end])
	case "trim":
		result = strings.TrimSpace(text)
	case "trim_space":
		// 移除所有空格
		result = strings.ReplaceAll(text, " ", "")
	case "replace":
		oldStr, _ := params["old"].(string)
		newStr, _ := params["new"].(string)
		result = strings.ReplaceAll(text, oldStr, newStr)
	case "contains":
		search, _ := params["search"].(string)
		found := strings.Contains(text, search)
		result = fmt.Sprintf("%v (包含 '%s')", found, search)
	case "count":
		search, _ := params["search"].(string)
		count := strings.Count(text, search)
		result = fmt.Sprintf("'%s' 出现次数: %d 次", search, count)
	case "reverse":
		runes := []rune(text)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		result = string(runes)
	case "split":
		sep, _ := params["separator"].(string)
		if sep == "" {
			sep = ","
		}
		parts := strings.Split(text, sep)
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("分割为 %d 部分:\n", len(parts)))
		for i, p := range parts {
			sb.WriteString(fmt.Sprintf("  [%d] %s\n", i+1, p))
		}
		result = sb.String()
	default:
		return &ToolResult{
			ToolName: "string_manipulate",
			Success:  false,
			Error:    "不支持的操作: " + op,
		}, nil
	}

	return &ToolResult{
		ToolName: "string_manipulate",
		Success:  true,
		Result:   result,
	}, nil
}

func parseIntParam(params map[string]interface{}, key string, defaultValue int) int {
	if v, ok := params[key].(float64); ok {
		return int(v)
	}
	if v, ok := params[key].(string); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultValue
}

// toTitleCase 将每个单词的首字母大写（替代已弃用的 strings.Title）。
func toTitleCase(s string) string {
	if s == "" {
		return s
	}
	words := strings.Fields(s)
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
