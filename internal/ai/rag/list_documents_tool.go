package rag

import (
	"context"
	"fmt"
	"strings"

	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
)

type ListDocumentsTool struct {
	docRepo   *workspacerepo.DocumentRepository
	spaceRepo *workspacerepo.SpaceRepository
}

func NewListDocumentsTool() *ListDocumentsTool {
	return &ListDocumentsTool{
		docRepo:   workspacerepo.NewDocumentRepository(),
		spaceRepo: workspacerepo.NewSpaceRepository(),
	}
}

func (t *ListDocumentsTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_documents",
		Description: "列出指定空间下的文档，支持按层级树或平铺列表显示",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"space_id": map[string]interface{}{
					"type":        "integer",
					"description": "空间ID",
				},
				"parent_id": map[string]interface{}{
					"type":        "integer",
					"description": "父文档ID，0表示从根目录开始",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "返回数量限制，默认30",
				},
				"view": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"tree", "flat"},
					"description": "显示方式：tree=树形结构，flat=平铺列表，默认flat",
				},
			},
			"required": []string{"space_id"},
		},
	}
}

func (t *ListDocumentsTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	spaceIDF, _ := params["space_id"].(float64)
	spaceID := int(spaceIDF)
	if spaceID == 0 {
		return &ToolResult{
			ToolName: "list_documents",
			Success:  false,
			Error:    "space_id is required",
		}, nil
	}

	// space_id 来自模型输出，不可信：列出文档前必须确认该空间对当前调用者可见。
	if err := ensureSpaceVisible(ctx, spaceID); err != nil {
		return &ToolResult{
			ToolName: "list_documents",
			Success:  false,
			Error:    "无权访问该空间",
		}, nil
	}

	parentID := 0
	if pid, ok := params["parent_id"].(float64); ok {
		parentID = int(pid)
	}

	limit := 30
	if l, ok := params["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	view, _ := params["view"].(string)
	if view == "" {
		view = "flat"
	}

	var docs []*workspacemodel.Document
	var err error

	if view == "tree" {
		docs, err = t.docRepo.FindRootDocuments(spaceID)
	} else {
		if parentID > 0 {
			docs, err = t.docRepo.FindByParentID(parentID)
		} else {
			docs, err = t.docRepo.FindBySpaceID(spaceID)
		}
	}

	if err != nil {
		return &ToolResult{
			ToolName: "list_documents",
			Success:  false,
			Error:    "查询文档失败: " + err.Error(),
		}, nil
	}

	if len(docs) == 0 {
		return &ToolResult{
			ToolName: "list_documents",
			Success:  true,
			Result:   "该空间/目录下暂无文档",
		}, nil
	}

	totalCount := len(docs)
	if len(docs) > limit {
		docs = docs[:limit]
	}

	space, _ := t.spaceRepo.FindByID(spaceID)
	spaceName := "未知空间"
	if space != nil {
		spaceName = space.Name
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("空间 [%s] 文档列表 (共 %d 个，显示前 %d 个):\n\n", spaceName, totalCount, limit))

	if view == "tree" {
		for _, d := range docs {
			t.printDocTree(&result, d, 0, limit)
		}
	} else {
		for i, d := range docs {
			docType := "文档"
			if d.Type == 2 {
				docType = "目录"
			}
			result.WriteString(fmt.Sprintf("%d. [ID:%d] %s (%s)\n   路径: %s\n\n",
				i+1, d.ID, d.Name, docType, d.Path))
		}
	}

	return &ToolResult{
		ToolName: "list_documents",
		Success:  true,
		Result:   result.String(),
	}, nil
}

func (t *ListDocumentsTool) printDocTree(result *strings.Builder, doc *workspacemodel.Document, depth int, limit int) {
	prefix := strings.Repeat("  ", depth)
	docType := "📄"
	if doc.Type == 2 {
		docType = "📁"
	}
	result.WriteString(fmt.Sprintf("%s%s [ID:%d] %s\n", prefix, docType, doc.ID, doc.Name))

	if depth < 2 {
		children, err := t.docRepo.FindByParentID(doc.ID)
		if err == nil && len(children) > 0 {
			if len(children) > limit {
				children = children[:limit]
			}
			for _, child := range children {
				t.printDocTree(result, child, depth+1, limit)
			}
		}
	}
}
