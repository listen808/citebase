package rag

import (
	"context"
	"fmt"
	"strings"

	userrepo "github.com/listen808/citebase/internal/repository/user"
)

type SearchUserTool struct {
	userRepo *userrepo.UserRepository
}

func NewSearchUserTool() *SearchUserTool {
	return &SearchUserTool{
		userRepo: userrepo.NewUserRepository(),
	}
}

func (t *SearchUserTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "search_user",
		Description: "搜索系统用户，返回匹配的用户列表（按用户名搜索）",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"keyword": map[string]interface{}{
					"type":        "string",
					"description": "搜索关键词（匹配用户名）",
				},
			},
			"required": []string{"keyword"},
		},
	}
}

func (t *SearchUserTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	keyword, _ := params["keyword"].(string)
	if keyword == "" {
		return &ToolResult{ToolName: "search_user", Success: false, Error: "keyword is required"}, nil
	}

	users, _, err := t.userRepo.FindAll(0, 10, 0, keyword)
	if err != nil {
		return &ToolResult{ToolName: "search_user", Success: false, Error: err.Error()}, nil
	}

	if len(users) == 0 {
		return &ToolResult{ToolName: "search_user", Success: true, Result: "未找到匹配的用户"}, nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("找到 %d 个用户：\n", len(users)))
	for i, u := range users {
		// P1 修复（清单 75）：该工具对任何登录会话可达且无角色判定，直接输出
		// 邮箱+部门等于给普通用户一条批量采集全目录 PII 的通道。这里脱敏邮箱、
		// 不再输出部门；若后续接入角色守卫，再按管理员放宽。
		result.WriteString(fmt.Sprintf("%d. %s (邮箱: %s)\n", i+1, u.Username, maskEmailForTool(u.Email)))
	}

	return &ToolResult{ToolName: "search_user", Success: true, Result: result.String()}, nil
}

// maskEmailForTool 保留本地部分前 2 字符用于人工比对，其余打码，域名保留首字符。
func maskEmailForTool(email string) string {
	if email == "" {
		return "-"
	}
	at := strings.Index(email, "@")
	if at <= 0 {
		return "***"
	}
	local := email[:at]
	domain := email[at+1:]
	if len(local) > 2 {
		local = local[:2] + "***"
	} else {
		local = local[:1] + "***"
	}
	if len(domain) > 1 {
		domain = domain[:1] + "***"
	}
	return local + "@" + domain
}
