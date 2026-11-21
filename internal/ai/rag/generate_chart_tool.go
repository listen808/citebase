package rag

import (
	"context"
	"encoding/json"
	"fmt"
)

type GenerateChartTool struct{}

func NewGenerateChartTool() *GenerateChartTool {
	return &GenerateChartTool{}
}

func (t *GenerateChartTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "generate_chart",
		Description: "生成图表配置数据（ECharts格式）。支持柱状图、折线图、饼图",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"chart_type": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"bar", "line", "pie"},
					"description": "图表类型",
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "图表标题",
				},
				"categories": map[string]interface{}{
					"type":        "array",
					"description": "X轴分类标签",
					"items":       map[string]interface{}{"type": "string"},
				},
				"values": map[string]interface{}{
					"type":        "array",
					"description": "数据值",
					"items":       map[string]interface{}{"type": "number"},
				},
				"series_name": map[string]interface{}{
					"type":        "string",
					"description": "数据系列名称",
				},
			},
			"required": []string{"chart_type", "title", "categories", "values"},
		},
	}
}

func (t *GenerateChartTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	chartType, _ := params["chart_type"].(string)
	title, _ := params["title"].(string)
	categoriesRaw, _ := params["categories"].([]interface{})
	valuesRaw, _ := params["values"].([]interface{})
	seriesName, _ := params["series_name"].(string)
	if seriesName == "" {
		seriesName = "数据"
	}

	if chartType == "" || title == "" || len(categoriesRaw) == 0 || len(valuesRaw) == 0 {
		return &ToolResult{ToolName: "generate_chart", Success: false, Error: "chart_type, title, categories, values are required"}, nil
	}

	categories := make([]string, len(categoriesRaw))
	for i, c := range categoriesRaw {
		categories[i] = fmt.Sprintf("%v", c)
	}
	values := make([]float64, len(valuesRaw))
	for i, v := range valuesRaw {
		if f, ok := v.(float64); ok {
			values[i] = f
		}
	}

	var option map[string]interface{}
	switch chartType {
	case "bar":
		option = map[string]interface{}{
			"title":  map[string]interface{}{"text": title},
			"xAxis":  map[string]interface{}{"type": "category", "data": categories},
			"yAxis":  map[string]interface{}{"type": "value"},
			"series": []map[string]interface{}{{"name": seriesName, "type": "bar", "data": values}},
		}
	case "line":
		option = map[string]interface{}{
			"title":  map[string]interface{}{"text": title},
			"xAxis":  map[string]interface{}{"type": "category", "data": categories},
			"yAxis":  map[string]interface{}{"type": "value"},
			"series": []map[string]interface{}{{"name": seriesName, "type": "line", "data": values}},
		}
	case "pie":
		// P1 修复（清单 13）：categories/values 长度均由模型给出，categories 比 values 长
		// 时 values[i] 直接下标越界 panic；工具在无 recover 的协程里执行会击穿整个进程。
		// 显式校验等长，缺额补 0（多出的 values 忽略）。
		if len(values) < len(categories) {
			padded := make([]float64, len(categories))
			copy(padded, values)
			values = padded
		}
		pieData := make([]map[string]interface{}, len(categories))
		for i, c := range categories {
			pieData[i] = map[string]interface{}{"name": c, "value": values[i]}
		}
		option = map[string]interface{}{
			"title":  map[string]interface{}{"text": title},
			"series": []map[string]interface{}{{"name": seriesName, "type": "pie", "data": pieData}},
		}
	default:
		return &ToolResult{ToolName: "generate_chart", Success: false, Error: "不支持的图表类型: " + chartType}, nil
	}

	jsonData, _ := json.MarshalIndent(option, "", "  ")
	return &ToolResult{
		ToolName: "generate_chart",
		Success:  true,
		Result:   fmt.Sprintf("图表配置已生成（ECharts格式）:\n%s", string(jsonData)),
	}, nil
}
