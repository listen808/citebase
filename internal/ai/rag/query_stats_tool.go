package rag

import (
	"context"
	"fmt"
	"strings"

	userrepo "github.com/listen808/citebase/internal/repository/user"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
)

type QueryStatsTool struct {
	docRepo   *workspacerepo.DocumentRepository
	spaceRepo *workspacerepo.SpaceRepository
	userRepo  *userrepo.UserRepository
}

func NewQueryStatsTool() *QueryStatsTool {
	return &QueryStatsTool{
		docRepo:   workspacerepo.NewDocumentRepository(),
		spaceRepo: workspacerepo.NewSpaceRepository(),
		userRepo:  userrepo.NewUserRepository(),
	}
}

func (t *QueryStatsTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "query_stats",
		Description: "查询知识库统计数据：文档数量、空间数量、用户数量、指定空间文档数",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"stat_type": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"doc_count", "space_count", "user_count", "doc_by_space"},
					"description": "统计类型",
				},
				"space_id": map[string]interface{}{
					"type":        "integer",
					"description": "空间ID（doc_by_space 时需要）",
				},
			},
			"required": []string{"stat_type"},
		},
	}
}

func (t *QueryStatsTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	statType, _ := params["stat_type"].(string)
	if statType == "" {
		return &ToolResult{ToolName: "query_stats", Success: false, Error: "stat_type is required"}, nil
	}

	var result strings.Builder
	switch statType {
	case "doc_count":
		count, err := t.docRepo.CountAll()
		if err != nil {
			return &ToolResult{ToolName: "query_stats", Success: false, Error: err.Error()}, nil
		}
		// 明确标注为全库口径，避免被误读成「当前用户可见范围」的统计。
		result.WriteString(fmt.Sprintf("全库文档总数: %d", count))
	case "space_count":
		// 不能用 FindAll：那会把私有空间也计入，等于向任意会话泄露空间规模。
		visibleIDs, err := t.spaceRepo.FindAccessibleSpaceIDs(ctxUserID(ctx), false)
		if err != nil {
			return &ToolResult{ToolName: "query_stats", Success: false, Error: "权限校验失败"}, nil
		}
		result.WriteString(fmt.Sprintf("可见空间数: %d", len(visibleIDs)))
	case "user_count":
		count, err := t.userRepo.CountNormalUsers()
		if err != nil {
			return &ToolResult{ToolName: "query_stats", Success: false, Error: err.Error()}, nil
		}
		result.WriteString(fmt.Sprintf("活跃用户数: %d", count))
	case "doc_by_space":
		spaceIDF, _ := params["space_id"].(float64)
		spaceID := int(spaceIDF)
		if spaceID == 0 {
			return &ToolResult{ToolName: "query_stats", Success: false, Error: "doc_by_space 需要 space_id 参数"}, nil
		}
		// space_id 来自模型输出，不可信：先确认该空间对当前调用者可见。
		if err := ensureSpaceVisible(ctx, spaceID); err != nil {
			return &ToolResult{ToolName: "query_stats", Success: false, Error: "无权访问该空间"}, nil
		}
		count, err := t.docRepo.CountBySpaceID(spaceID)
		if err != nil {
			return &ToolResult{ToolName: "query_stats", Success: false, Error: err.Error()}, nil
		}
		result.WriteString(fmt.Sprintf("空间 %d 文档数: %d", spaceID, count))
	default:
		return &ToolResult{ToolName: "query_stats", Success: false, Error: "不支持的统计类型: " + statType}, nil
	}

	return &ToolResult{ToolName: "query_stats", Success: true, Result: result.String()}, nil
}
