package rag

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type GetCurrentTimeTool struct{}

func NewGetCurrentTimeTool() *GetCurrentTimeTool {
	return &GetCurrentTimeTool{}
}

func (t *GetCurrentTimeTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_current_time",
		Description: "获取当前时间和日期，支持多种格式输出",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"format": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"standard", "iso", "date", "time", "timestamp", "chinese", "rfc1123"},
					"description": "输出格式，默认standard",
				},
				"timezone": map[string]interface{}{
					"type":        "string",
					"description": "时区偏移，如+08:00，默认使用本地时区",
				},
			},
			"required": []string{},
		},
	}
}

func (t *GetCurrentTimeTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	format, _ := params["format"].(string)
	if format == "" {
		format = "standard"
	}

	var now time.Time
	if tz, ok := params["timezone"].(string); ok && tz != "" {
		loc, err := time.LoadLocation(tz)
		if err == nil {
			now = time.Now().In(loc)
		} else {
			now = time.Now()
		}
	} else {
		now = time.Now()
	}

	var result string
	switch strings.ToLower(format) {
	case "iso":
		result = now.Format(time.RFC3339)
	case "date":
		result = now.Format("2006-01-02")
	case "time":
		result = now.Format("15:04:05")
	case "timestamp":
		result = fmt.Sprintf("%d", now.Unix())
	case "chinese":
		weekday := []string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"}[now.Weekday()]
		result = fmt.Sprintf("%d年%d月%d日 %s %02d:%02d:%02d",
			now.Year(), now.Month(), now.Day(), weekday, now.Hour(), now.Minute(), now.Second())
	case "rfc1123":
		result = now.Format(time.RFC1123)
	default:
		result = now.Format("2006-01-02 15:04:05")
	}

	return &ToolResult{
		ToolName: "get_current_time",
		Success:  true,
		Result:   fmt.Sprintf("当前时间: %s", result),
	}, nil
}
