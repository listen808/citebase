package rag

import (
	"context"
	"fmt"
	"strings"

	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/service/doccontent"
	"github.com/listen808/citebase/internal/utils"
)

type GetDocumentTool struct {
	docRepo    *workspacerepo.DocumentRepository
	docStorage *utils.DocumentStorage
	perm       *PermissionChecker
}

func NewGetDocumentTool() *GetDocumentTool {
	return &GetDocumentTool{
		docRepo:    workspacerepo.NewDocumentRepository(),
		docStorage: utils.NewDocumentStorageWithDir("./data/markdowns"),
		perm:       NewPermissionChecker(),
	}
}

func (t *GetDocumentTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "get_document",
		Description: "获取指定文档的详细信息和内容",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"document_id": map[string]interface{}{
					"type":        "integer",
					"description": "文档ID",
				},
				"include_content": map[string]interface{}{
					"type":        "boolean",
					"description": "是否包含文档内容，默认true",
				},
				"content_limit": map[string]interface{}{
					"type":        "integer",
					"description": "内容最大字符数，默认2000，0表示不限制",
				},
			},
			"required": []string{"document_id"},
		},
	}
}

func (t *GetDocumentTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	docIDF, _ := params["document_id"].(float64)
	docID := int(docIDF)
	if docID == 0 {
		return &ToolResult{
			ToolName: "get_document",
			Success:  false,
			Error:    "document_id is required",
		}, nil
	}

	includeContent := true
	if ic, ok := params["include_content"].(bool); ok {
		includeContent = ic
	}

	contentLimit := 2000
	if cl, ok := params["content_limit"].(float64); ok {
		contentLimit = int(cl)
	}
	// P1（清单 14）：content_limit 由模型控制，旧实现里 0/负数 = "不限制"，
	// 配合多轮调用可把任意可见正文整篇灌进上下文。设服务端硬上限与缺省兜底。
	if contentLimit <= 0 {
		contentLimit = 2000
	}
	if contentLimit > 8000 {
		contentLimit = 8000
	}

	doc, err := t.docRepo.FindByID(docID)
	if err != nil || doc == nil {
		return &ToolResult{
			ToolName: "get_document",
			Success:  false,
			Error:    "文档不存在",
		}, nil
	}

	// document_id 来自模型输出：必须先确认该文档所在空间对调用者可见，
	// 否则任何会话都能按 ID 读出任意私有文档正文。
	if err := t.perm.EnsureSpaceVisible(ctx, doc.SpaceID); err != nil {
		return &ToolResult{
			ToolName: "get_document",
			Success:  false,
			Error:    "无权访问该文档",
		}, nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("文档信息:\n"))
	result.WriteString(fmt.Sprintf("  ID: %d\n", doc.ID))
	result.WriteString(fmt.Sprintf("  标题: %s\n", doc.Name))
	result.WriteString(fmt.Sprintf("  空间ID: %d\n", doc.SpaceID))
	result.WriteString(fmt.Sprintf("  父文档: %d\n", doc.ParentID))
	result.WriteString(fmt.Sprintf("  类型: %d\n", doc.Type))
	result.WriteString(fmt.Sprintf("  创建人: %d\n", doc.CreateUserID))
	result.WriteString(fmt.Sprintf("  路径: %s\n", doc.Path))

	if includeContent {
		pageFile := getPageFilePath(doc, t.docRepo, workspacerepo.NewSpaceRepository(), t.docStorage)
		content, err := doccontent.Get(doc.ID, t.docStorage, pageFile)
		if err != nil {
			result.WriteString(fmt.Sprintf("\n  内容: 读取失败 (%s)\n", err.Error()))
		} else if content == "" {
			result.WriteString("\n  内容: (空文档)\n")
		} else {
			if contentLimit > 0 && len(content) > contentLimit {
				content = content[:contentLimit] + "\n\n...(已截断)"
			}
			result.WriteString(fmt.Sprintf("\n  内容:\n---\n%s\n---\n", content))
		}
	}

	return &ToolResult{
		ToolName: "get_document",
		Success:  true,
		Result:   result.String(),
	}, nil
}
