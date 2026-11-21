package rag

import (
	"context"
	"fmt"
	"strings"

	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/service/doccontent"
	"github.com/listen808/citebase/internal/utils"
)

type EditDocumentTool struct {
	docRepo    *workspacerepo.DocumentRepository
	spaceRepo  *workspacerepo.SpaceRepository
	docStorage *utils.DocumentStorage
}

func NewEditDocumentTool() *EditDocumentTool {
	return &EditDocumentTool{
		docRepo:    workspacerepo.NewDocumentRepository(),
		spaceRepo:  workspacerepo.NewSpaceRepository(),
		docStorage: utils.NewDocumentStorageWithDir("./data/markdowns"),
	}
}

func (t *EditDocumentTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "edit_document",
		Description: "编辑知识库中的文档内容。支持三种操作：replace（替换指定章节）、append（追加内容到文档末尾）、prepend（在文档开头插入内容）",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"document_id": map[string]interface{}{
					"type":        "integer",
					"description": "文档ID",
				},
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"replace", "append", "prepend"},
					"description": "操作类型：replace=替换章节, append=追加到末尾, prepend=插入开头",
				},
				"section_heading": map[string]interface{}{
					"type":        "string",
					"description": "目标章节标题（仅 replace 操作需要，匹配 Markdown 标题）",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "新内容（Markdown 格式）",
				},
			},
			"required": []string{"document_id", "action", "content"},
		},
	}
}

func (t *EditDocumentTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	docIDF, _ := params["document_id"].(float64)
	docID := int(docIDF)
	action, _ := params["action"].(string)
	sectionHeading, _ := params["section_heading"].(string)
	content, _ := params["content"].(string)

	if docID == 0 || action == "" || content == "" {
		return &ToolResult{ToolName: "edit_document", Success: false, Error: "document_id, action, content are required"}, nil
	}

	doc, err := t.docRepo.FindByID(docID)
	if err != nil || doc == nil {
		return &ToolResult{ToolName: "edit_document", Success: false, Error: "文档不存在"}, nil
	}

	// document_id 来自模型输出，不可信：写入前必须确认该文档所属空间对当前调用者可见，
	// 否则任意用户只要猜到一个文档 ID 就能改写他人（甚至私有空间）的正文。
	if err := ensureSpaceVisible(ctx, doc.SpaceID); err != nil {
		return &ToolResult{ToolName: "edit_document", Success: false, Error: "无权编辑该文档"}, nil
	}

	pageFile := getPageFilePath(doc, t.docRepo, t.spaceRepo, t.docStorage)
	oldContent, err := doccontent.Get(doc.ID, t.docStorage, pageFile)
	if err != nil {
		return &ToolResult{ToolName: "edit_document", Success: false, Error: "读取文档内容失败: " + err.Error()}, nil
	}

	var newContent string
	switch action {
	case "append":
		newContent = oldContent + "\n\n" + content
	case "prepend":
		newContent = content + "\n\n" + oldContent
	case "replace":
		if sectionHeading == "" {
			return &ToolResult{ToolName: "edit_document", Success: false, Error: "replace 操作需要 section_heading 参数"}, nil
		}
		newContent = replaceSection(oldContent, sectionHeading, content)
		if newContent == oldContent {
			return &ToolResult{ToolName: "edit_document", Success: false, Error: fmt.Sprintf("未找到标题 '%s' 对应的章节", sectionHeading)}, nil
		}
	default:
		return &ToolResult{ToolName: "edit_document", Success: false, Error: "不支持的操作: " + action}, nil
	}

	if err := doccontent.Set(doc.ID, t.docStorage, pageFile, newContent); err != nil {
		return &ToolResult{ToolName: "edit_document", Success: false, Error: "保存文档失败: " + err.Error()}, nil
	}

	return &ToolResult{
		ToolName: "edit_document",
		Success:  true,
		Result:   fmt.Sprintf("文档编辑成功: ID=%d, 操作=%s", docID, action),
	}, nil
}

func replaceSection(content, heading, newSectionContent string) string {
	lines := strings.Split(content, "\n")
	headingLevel := 0
	startIdx := -1
	endIdx := len(lines)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			level := 0
			for _, ch := range trimmed {
				if ch == '#' {
					level++
				} else {
					break
				}
			}
			title := strings.TrimSpace(trimmed[level:])
			if startIdx == -1 && title == heading {
				startIdx = i
				headingLevel = level
			} else if startIdx != -1 && level <= headingLevel {
				endIdx = i
				break
			}
		}
	}

	if startIdx == -1 {
		return content
	}

	var result strings.Builder
	result.WriteString(strings.Join(lines[:startIdx], "\n"))
	if startIdx > 0 {
		result.WriteString("\n")
	}
	result.WriteString(newSectionContent)
	if endIdx < len(lines) {
		result.WriteString("\n")
		result.WriteString(strings.Join(lines[endIdx:], "\n"))
	}
	return result.String()
}
