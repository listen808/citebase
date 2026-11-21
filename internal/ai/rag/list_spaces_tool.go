package rag

import (
	"context"
	"fmt"
	"strings"

	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
)

type ListSpacesTool struct {
	spaceRepo *workspacerepo.SpaceRepository
}

func NewListSpacesTool() *ListSpacesTool {
	return &ListSpacesTool{
		spaceRepo: workspacerepo.NewSpaceRepository(),
	}
}

func (t *ListSpacesTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_spaces",
		Description: "列出知识库中的所有空间，支持关键词搜索",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"keyword": map[string]interface{}{
					"type":        "string",
					"description": "搜索关键词，留空列出全部",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "返回数量限制，默认50",
				},
			},
			"required": []string{},
		},
	}
}

func (t *ListSpacesTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	keyword, _ := params["keyword"].(string)

	limit := 50
	if l, ok := params["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	spaces, _, err := t.spaceRepo.FindAll(0, limit, keyword, "", "")
	if err != nil {
		return &ToolResult{
			ToolName: "list_spaces",
			Success:  false,
			Error:    "查询空间失败: " + err.Error(),
		}, nil
	}

	// FindAll 不带权限语义，会把私有空间名称与描述泄露给任意会话。
	// 这里改用调用者的可见集合过滤，权限查询失败时直接拒绝而不是放行。
	visibleIDs, verr := t.spaceRepo.FindAccessibleSpaceIDs(ctxUserID(ctx), false)
	if verr != nil {
		return &ToolResult{ToolName: "list_spaces", Success: false, Error: "权限校验失败"}, nil
	}
	visible := make(map[int]bool, len(visibleIDs))
	for _, id := range visibleIDs {
		visible[id] = true
	}

	var accessible []*workspacemodel.Space
	for _, s := range spaces {
		if !visible[s.ID] {
			continue
		}
		accessible = append(accessible, s)
	}

	if len(accessible) == 0 {
		return &ToolResult{
			ToolName: "list_spaces",
			Success:  true,
			Result:   "未找到匹配的空间",
		}, nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("找到 %d 个空间（当前用户可见数量）:\n\n", len(accessible)))
	for i, s := range accessible {
		desc := s.Description
		if desc == "" {
			desc = "无描述"
		}
		if len(desc) > 60 {
			desc = desc[:60] + "..."
		}
		result.WriteString(fmt.Sprintf("%d. [ID:%d] %s\n   描述: %s\n   标签: %s\n\n",
			i+1, s.ID, s.Name, desc, s.Tags))
	}

	return &ToolResult{
		ToolName: "list_spaces",
		Success:  true,
		Result:   result.String(),
	}, nil
}
