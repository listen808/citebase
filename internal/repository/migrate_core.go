package repository

import (
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	socialmodel "github.com/listen808/citebase/internal/model/social"
	usermodel "github.com/listen808/citebase/internal/model/user"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
)

// 核心业务表：用户 / 角色 / 权限 / 空间 / 文档 / 社交 / 管理配置。
// 顺序保证被依赖的基础表（用户、空间）先建。
func init() {
	RegisterModels(
		&usermodel.User{},
		&usermodel.Role{},
		&usermodel.Privilege{},
		&usermodel.RolePrivilege{},
		&workspacemodel.Space{},
		&workspacemodel.SpaceUser{},
		&workspacemodel.Document{},
		&workspacemodel.DocumentContent{},
		&workspacemodel.DocumentVersion{},
		&workspacemodel.DocumentViewLog{},
		&workspacemodel.DocumentTranslation{},
		&workspacemodel.DocumentLink{},
		&workspacemodel.DocumentEditLock{},
		&workspacemodel.DocumentTrash{},
		&workspacemodel.DocumentTag{},
		&adminmodel.DocumentLog{},
		&workspacemodel.Attachment{},
		// P2: 附件分块入库
		&workspacemodel.AttachmentChunk{},
		&socialmodel.Collection{},
		&socialmodel.Follow{},
		&socialmodel.Notification{},
		&adminmodel.Config{},
		&adminmodel.Email{},
		&adminmodel.AuthSSO{},
		&usermodel.Contact{},
		&adminmodel.Link{},
		&adminmodel.Plugin{},
		&adminmodel.SystemLog{},
		&socialmodel.Comment{},
		// P1: @提及 / 审批流 / 文档模板
		&workspacemodel.DocumentMention{},
		&workspacemodel.DocumentApproval{},
		&workspacemodel.DocumentApprovalStep{},
		&workspacemodel.DocumentTemplate{},
		// P1: 版本标签/里程碑
		&workspacemodel.DocumentVersionTag{},
		// P1: 标签层级
		&workspacemodel.TagHierarchy{},
		// P1: 保存的搜索
		&workspacemodel.SavedSearch{},
		// P1: Webhook 通知
		&workspacemodel.WebhookConfig{},
		&workspacemodel.WebhookDelivery{},
		// P1: G029 API 开放平台
		&adminmodel.ApiKey{},
		// P1-3: 文档级 ACL（此前只有空间成员级权限）
		&workspacemodel.DocumentPermission{},
	)
}
