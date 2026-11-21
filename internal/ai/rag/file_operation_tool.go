package rag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileOperationTool 文件操作工具
type FileOperationTool struct {
	allowedDirs []string // 允许操作的目录列表
}

func NewFileOperationTool(allowedDirs ...string) *FileOperationTool {
	return &FileOperationTool{
		allowedDirs: allowedDirs,
	}
}

func (t *FileOperationTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "file_operation",
		Description: "执行文件读写操作，支持读取、写入、追加、检查存在、获取信息。操作必须在允许的目录范围内",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"read", "write", "append", "exists", "info"},
					"description": "操作类型：read(读取)、write(写入)、append(追加)、exists(检查存在)、info(获取信息)",
				},
				"path": map[string]interface{}{
					"type":        "string",
					"description": "文件路径（绝对路径或相对路径）",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "文件内容（write和append操作时需要）",
				},
			},
			"required": []string{"action", "path"},
		},
	}
}

func (t *FileOperationTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	action, _ := params["action"].(string)
	path, _ := params["path"].(string)
	content, _ := params["content"].(string)

	if action == "" || path == "" {
		return &ToolResult{
			ToolName: "file_operation",
			Success:  false,
			Error:    "缺少action或path参数",
		}, nil
	}

	// 转换为绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return &ToolResult{
			ToolName: "file_operation",
			Success:  false,
			Error:    fmt.Sprintf("路径解析失败: %v", err),
		}, nil
	}

	// 安全检查：确保在允许的目录范围内
	if !t.isPathAllowed(absPath) {
		return &ToolResult{
			ToolName: "file_operation",
			Success:  false,
			Error:    fmt.Sprintf("安全限制：文件路径 %s 不在允许的目录范围内", absPath),
		}, nil
	}

	switch strings.ToLower(action) {
	case "read":
		return t.readFile(absPath)
	case "write":
		return t.writeFile(absPath, content)
	case "append":
		return t.appendFile(absPath, content)
	case "exists":
		return t.checkExists(absPath)
	case "info":
		return t.getFileInfo(absPath)
	default:
		return &ToolResult{
			ToolName: "file_operation",
			Success:  false,
			Error:    fmt.Sprintf("不支持的操作: %s", action),
		}, nil
	}
}

// isPathAllowed 判断路径是否允许访问。
//
// 旧实现在 allowedDirs 为空时直接 return true，而注册处用的正是无参构造
// —— 等于把服务器任意文件的读写权限开放给模型输出。现在一律 fail-closed：
// 未配置白名单即拒绝；并用相对路径判定，避免字符串前缀把 /docs 放大到 /docs-evil。
func (t *FileOperationTool) isPathAllowed(path string) bool {
	if len(t.allowedDirs) == 0 {
		return false
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absPath = filepath.Clean(absPath)

	for _, allowed := range t.allowedDirs {
		absAllowed, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}
		absAllowed = filepath.Clean(absAllowed)
		rel, err := filepath.Rel(absAllowed, absPath)
		if err != nil {
			continue
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			continue
		}
		return true
	}
	return false
}

func (t *FileOperationTool) readFile(path string) (*ToolResult, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ToolResult{
				ToolName: "file_operation",
				Success:  false,
				Error:    fmt.Sprintf("文件不存在: %s", path),
			}, nil
		}
		return &ToolResult{
			ToolName: "file_operation",
			Success:  false,
			Error:    fmt.Sprintf("读取文件失败: %v", err),
		}, nil
	}

	// 限制文件大小（1MB）
	if len(content) > 1024*1024 {
		return &ToolResult{
			ToolName: "file_operation",
			Success:  false,
			Error:    "文件过大（超过1MB限制）",
		}, nil
	}

	return &ToolResult{
		ToolName: "file_operation",
		Success:  true,
		Result:   string(content),
	}, nil
}

func (t *FileOperationTool) writeFile(path, content string) (*ToolResult, error) {
	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &ToolResult{
			ToolName: "file_operation",
			Success:  false,
			Error:    fmt.Sprintf("创建目录失败: %v", err),
		}, nil
	}

	err := os.WriteFile(path, []byte(content), 0600)
	if err != nil {
		return &ToolResult{
			ToolName: "file_operation",
			Success:  false,
			Error:    fmt.Sprintf("写入文件失败: %v", err),
		}, nil
	}

	return &ToolResult{
		ToolName: "file_operation",
		Success:  true,
		Result:   fmt.Sprintf("成功写入文件: %s (%d 字节)", path, len(content)),
	}, nil
}

func (t *FileOperationTool) appendFile(path, content string) (*ToolResult, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return &ToolResult{
			ToolName: "file_operation",
			Success:  false,
			Error:    fmt.Sprintf("打开文件失败: %v", err),
		}, nil
	}
	defer f.Close()

	n, err := f.WriteString(content)
	if err != nil {
		return &ToolResult{
			ToolName: "file_operation",
			Success:  false,
			Error:    fmt.Sprintf("追加内容失败: %v", err),
		}, nil
	}

	return &ToolResult{
		ToolName: "file_operation",
		Success:  true,
		Result:   fmt.Sprintf("成功追加 %d 字节到文件: %s", n, path),
	}, nil
}

func (t *FileOperationTool) checkExists(path string) (*ToolResult, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ToolResult{
				ToolName: "file_operation",
				Success:  true,
				Result:   "文件不存在",
			}, nil
		}
		return &ToolResult{
			ToolName: "file_operation",
			Success:  false,
			Error:    fmt.Sprintf("检查文件失败: %v", err),
		}, nil
	}

	return &ToolResult{
		ToolName: "file_operation",
		Success:  true,
		Result:   "文件存在",
	}, nil
}

func (t *FileOperationTool) getFileInfo(path string) (*ToolResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ToolResult{
				ToolName: "file_operation",
				Success:  false,
				Error:    fmt.Sprintf("文件不存在: %s", path),
			}, nil
		}
		return &ToolResult{
			ToolName: "file_operation",
			Success:  false,
			Error:    fmt.Sprintf("获取文件信息失败: %v", err),
		}, nil
	}

	fileType := "文件"
	if info.IsDir() {
		fileType = "目录"
	}

	result := fmt.Sprintf("%s: %s\n", fileType, path)
	result += fmt.Sprintf("大小: %d 字节\n", info.Size())
	result += fmt.Sprintf("权限: %s\n", info.Mode().String())
	result += fmt.Sprintf("修改时间: %s\n", info.ModTime().Format("2006-01-02 15:04:05"))

	return &ToolResult{
		ToolName: "file_operation",
		Success:  true,
		Result:   result,
	}, nil
}
