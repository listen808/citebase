package rag

import (
	"context"
	"fmt"
	"strings"

	"github.com/listen808/citebase/internal/ctxkeys"
)

// UserContext 用户上下文 - 携带用户身份和权限信息
type UserContext struct {
	UserID   int                    `json:"user_id"`
	Username string                 `json:"username"`
	RoleID   int                    `json:"role_id"` // 1=超级管理员, 2=管理员, 3=普通用户
	RoleName string                 `json:"role_name"`
	SpaceIDs []int                  `json:"space_ids"` // 用户可访问的空间ID列表
	Groups   []string               `json:"groups"`    // 用户所属的组
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// IsAdmin 是否管理员
func (uc *UserContext) IsAdmin() bool {
	return uc.RoleID <= 2
}

// HasSpaceAccess 是否有指定空间的访问权限
func (uc *UserContext) HasSpaceAccess(spaceID int) bool {
	if spaceID <= 0 {
		return true
	}
	if uc.IsAdmin() {
		return true
	}
	for _, id := range uc.SpaceIDs {
		if id == spaceID {
			return true
		}
	}
	return false
}

// HasGroup 是否属于指定组
func (uc *UserContext) HasGroup(group string) bool {
	for _, g := range uc.Groups {
		if g == group {
			return true
		}
	}
	return false
}

// ToolPermission 工具权限定义
type ToolPermission struct {
	ToolName       string `json:"tool_name"`
	RequiredRole   int    `json:"required_role"`   // 最低角色要求 (0=任何, 1=超管, 2=管理员, 3=普通用户)
	RequiredGroup  string `json:"required_group"`  // 需要的组 (空=不需要)
	RequiredSpaces []int  `json:"required_spaces"` // 需要访问这些空间 (空=不需要)
	DeniedRoles    []int  `json:"denied_roles"`    // 禁止的角色
}

// UserAwareToolRegistry 用户感知的工具注册表
type UserAwareToolRegistry struct {
	tools    map[string]*ToolPermission
	fallback *ToolRegistry
}

func NewUserAwareToolRegistry(fallback *ToolRegistry) *UserAwareToolRegistry {
	return &UserAwareToolRegistry{
		tools:    make(map[string]*ToolPermission),
		fallback: fallback,
	}
}

// RegisterPermission 注册工具权限
func (r *UserAwareToolRegistry) RegisterPermission(perm ToolPermission) {
	r.tools[perm.ToolName] = &perm
}

// CheckPermission 检查用户是否有权使用指定工具
func (r *UserAwareToolRegistry) CheckPermission(ctx context.Context, toolName string, user *UserContext) error {
	if user == nil {
		return fmt.Errorf("user context is required")
	}

	perm, ok := r.tools[toolName]
	if !ok {
		// 未注册权限的工具，默认允许
		return nil
	}

	// 检查角色要求
	if perm.RequiredRole > 0 && user.RoleID > perm.RequiredRole {
		return fmt.Errorf("tool '%s' requires role %d or higher, your role is %d", toolName, perm.RequiredRole, user.RoleID)
	}

	// 检查禁止的角色
	for _, denied := range perm.DeniedRoles {
		if user.RoleID == denied {
			return fmt.Errorf("tool '%s' is denied for your role", toolName)
		}
	}

	// 检查组要求
	if perm.RequiredGroup != "" && !user.HasGroup(perm.RequiredGroup) {
		return fmt.Errorf("tool '%s' requires group '%s'", toolName, perm.RequiredGroup)
	}

	// 检查空间要求
	if len(perm.RequiredSpaces) > 0 {
		hasAccess := false
		for _, requiredSpace := range perm.RequiredSpaces {
			if user.HasSpaceAccess(requiredSpace) {
				hasAccess = true
				break
			}
		}
		if !hasAccess {
			return fmt.Errorf("tool '%s' requires access to one of the specified spaces", toolName)
		}
	}

	return nil
}

// WrapTool 为工具添加权限检查包装
func (r *UserAwareToolRegistry) WrapTool(tool ToolExecutor) ToolExecutor {
	return &userAwareToolWrapper{
		registry: r,
		tool:     tool,
	}
}

// userAwareToolWrapper 带权限检查的工具包装器
type userAwareToolWrapper struct {
	registry *UserAwareToolRegistry
	tool     ToolExecutor
}

func (w *userAwareToolWrapper) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// 从context中提取用户信息
	user := extractUserFromContext(ctx)
	if user != nil {
		def := w.tool.Definition()
		if err := w.registry.CheckPermission(ctx, def.Name, user); err != nil {
			return &ToolResult{
				ToolName: def.Name,
				Success:  false,
				Error:    err.Error(),
			}, nil
		}
	}
	return w.tool.Execute(ctx, params)
}

func (w *userAwareToolWrapper) Definition() ToolDefinition {
	return w.tool.Definition()
}

// extractUserFromContext 从上下文中提取用户信息
func extractUserFromContext(ctx context.Context) *UserContext {
	userID := ctxkeys.UserID(ctx)
	if userID <= 0 {
		return nil
	}
	roleID, _ := ctx.Value("role_id").(int)
	username, _ := ctx.Value("username").(string)
	spaceIDs, _ := ctx.Value("space_ids").([]int)

	return &UserContext{
		UserID:   userID,
		Username: username,
		RoleID:   roleID,
		SpaceIDs: spaceIDs,
	}
}

// BuildPermissionPrompt 构建权限提示信息（注入到LLM system prompt）
func BuildPermissionPrompt(user *UserContext) string {
	if user == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("当前用户: %s (ID: %d)\n", user.Username, user.UserID))
	sb.WriteString(fmt.Sprintf("角色: %s (Level: %d)\n", user.RoleName, user.RoleID))

	if len(user.SpaceIDs) > 0 {
		sb.WriteString(fmt.Sprintf("可访问空间: %v\n", user.SpaceIDs))
	}

	if len(user.Groups) > 0 {
		sb.WriteString(fmt.Sprintf("所属组: %s\n", strings.Join(user.Groups, ", ")))
	}

	sb.WriteString("\n请根据用户权限提供回答。如果用户无权访问某些数据，请明确告知。")

	return sb.String()
}
