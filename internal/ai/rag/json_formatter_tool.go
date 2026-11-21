package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type JSONFormatterTool struct{}

func NewJSONFormatterTool() *JSONFormatterTool {
	return &JSONFormatterTool{}
}

func (t *JSONFormatterTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "json_formatter",
		Description: "JSON格式化工具：美化、压缩、验证",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"format", "minify", "validate"},
					"description": "操作类型：format=美化缩进，minify=压缩，validate=验证语法",
				},
				"data": map[string]interface{}{
					"type":        "string",
					"description": "JSON字符串",
				},
				"indent": map[string]interface{}{
					"type":        "integer",
					"description": "缩进空格数，默认2",
				},
			},
			"required": []string{"operation", "data"},
		},
	}
}

func (t *JSONFormatterTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	op, _ := params["operation"].(string)
	data, _ := params["data"].(string)

	if op == "" || data == "" {
		return &ToolResult{
			ToolName: "json_formatter",
			Success:  false,
			Error:    "operation and data are required",
		}, nil
	}

	// 首先验证JSON是否有效
	var parsed interface{}
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		return &ToolResult{
			ToolName: "json_formatter",
			Success:  false,
			Error:    fmt.Sprintf("JSON验证失败: %v", err),
		}, nil
	}

	var result string
	switch strings.ToLower(op) {
	case "format":
		indent := 2
		if ind, ok := params["indent"].(float64); ok && ind > 0 {
			indent = int(ind)
		}
		// P3：限制 indent 上限，避免恶意大值导致内存耗尽
		if indent > 32 {
			indent = 32
		}
		indentStr := strings.Repeat(" ", indent)
		formatted, err := json.MarshalIndent(parsed, "", indentStr)
		if err != nil {
			return &ToolResult{
				ToolName: "json_formatter",
				Success:  false,
				Error:    "格式化失败: " + err.Error(),
			}, nil
		}
		result = string(formatted)
	case "minify":
		minified, err := json.Marshal(parsed)
		if err != nil {
			return &ToolResult{
				ToolName: "json_formatter",
				Success:  false,
				Error:    "压缩失败: " + err.Error(),
			}, nil
		}
		result = string(minified)
	case "validate":
		result = "JSON格式有效 ✓"
	default:
		return &ToolResult{
			ToolName: "json_formatter",
			Success:  false,
			Error:    "不支持的操作: " + op,
		}, nil
	}

	return &ToolResult{
		ToolName: "json_formatter",
		Success:  true,
		Result:   result,
	}, nil
}
