package workspace

import (
	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/api/admin"
	"github.com/listen808/citebase/internal/middleware"
)

// RegisterRoutes 注册空间/文档/附件/标签相关路由
func RegisterRoutes(v1 *gin.RouterGroup) {
	spaceAPI := NewSpaceAPI()
	docAPI := NewDocumentAPI()
	imageAPI := NewImageAPI()
	attachmentAPI := NewAttachmentAPI()
	logAPI := admin.NewLogAPI()
	docTagAPI := NewDocumentTagAPI()
	mentionAPI := NewMentionAPI()
	approvalAPI := NewApprovalAPI()
	templateAPI := NewTemplateAPI()
	translationAPI := NewTranslationAPI()
	versionTagAPI := NewVersionTagAPI()
	docViewAPI := NewDocumentViewAPI()
	tagHierarchyAPI := NewTagHierarchyAPI()
	savedSearchAPI := NewSavedSearchAPI()
	webhookAPI := NewWebhookAPI()

	// 需要认证 + RBAC
	// P1 修复：AccessLog 已提升为全局中间件（server.go setupGin），此处移除避免重复记录
	authGroup := v1.Group("")
	authGroup.Use(middleware.AuthMiddleware(), middleware.AutoRBAC())
	{
		// 空间管理
		authGroup.POST("/spaces", spaceAPI.CreateSpace)
		authGroup.PUT("/spaces/:id", spaceAPI.UpdateSpace)
		authGroup.DELETE("/spaces/:id", spaceAPI.DeleteSpace)
		authGroup.GET("/spaces/:id/pre-delete-check", spaceAPI.PreCheckDeleteSpace)
		authGroup.GET("/spaces/my", spaceAPI.GetMySpaces)
		authGroup.PUT("/spaces/sort", spaceAPI.SortSpaces)
		authGroup.GET("/spaces/:id/members", spaceAPI.GetSpaceMembers)
		authGroup.POST("/spaces/:id/members", spaceAPI.AddSpaceMember)
		authGroup.DELETE("/spaces/:id/members", spaceAPI.RemoveSpaceMember)
		authGroup.PUT("/spaces/:id/members/privilege", spaceAPI.UpdateMemberPrivilege)
		authGroup.GET("/spaces/:id/download", spaceAPI.DownloadSpace)

		// 文档操作
		authGroup.POST("/documents", docAPI.CreateDocument)
		authGroup.PUT("/documents/:id", docAPI.UpdateDocument)
		authGroup.DELETE("/documents/:id", docAPI.DeleteDocument)
		authGroup.POST("/documents/:id/restore", docAPI.RestoreDocument)
		authGroup.DELETE("/documents/:id/permanent", docAPI.PermanentDeleteDocument)
		authGroup.PUT("/documents/:id/move", docAPI.MoveDocument)
		authGroup.POST("/documents/:id/copy", docAPI.CopyDocument)
		authGroup.PUT("/documents/sort", docAPI.SortDocuments)
		authGroup.GET("/documents/:id/versions", docAPI.GetDocumentVersions)
		authGroup.GET("/spaces/:id/trash", docAPI.GetSpaceTrash)
		authGroup.POST("/spaces/:id/import", docAPI.ImportDocuments)
		authGroup.GET("/documents/:id/versions/:version", docAPI.GetVersionContent)
		authGroup.GET("/documents/:id/diff", docAPI.DiffVersions)
		authGroup.POST("/documents/:id/rollback", docAPI.RollbackToVersion)
		authGroup.GET("/documents/:id/history", logAPI.GetDocumentHistory)
		authGroup.GET("/documents/:id/export", docAPI.ExportDocument)
		authGroup.GET("/documents/:id/export-html", docAPI.ExportDocumentHTML)
		authGroup.GET("/documents/:id/export-word", docAPI.ExportDocumentWord)
		authGroup.GET("/documents/:id/export-pdf", docAPI.ExportDocumentPDF)
		authGroup.GET("/documents/:id/export-md", docAPI.ExportDocumentMarkdown)
		authGroup.GET("/documents/:id/backlinks", docAPI.GetBacklinks)
		authGroup.GET("/documents/:id/forward-links", docAPI.GetForwardLinks)
		authGroup.GET("/documents/:id/related", docAPI.GetRecommendations)
		authGroup.POST("/documents/:id/lock", docAPI.LockEdit)
		authGroup.DELETE("/documents/:id/lock", docAPI.UnlockEdit)
		authGroup.PUT("/documents/:id/lock", docAPI.HeartbeatEdit)
		authGroup.GET("/documents/:id/conflict", docAPI.CheckEditConflict)

		// #7 草稿/发布分离
		authGroup.POST("/documents/:id/publish", docAPI.PublishDocument)
		authGroup.POST("/documents/:id/unpublish", docAPI.UnpublishDocument)

		// 文档标签管理
		authGroup.GET("/documents/:id/tags", docTagAPI.GetDocumentTags)
		authGroup.PUT("/documents/:id/tags", docTagAPI.SetDocumentTags)
		authGroup.POST("/documents/:id/tags", docTagAPI.AddDocumentTags)
		authGroup.DELETE("/documents/:id/tags", docTagAPI.RemoveDocumentTags)

		// @提及
		authGroup.GET("/documents/:id/mentions", mentionAPI.GetDocumentMentions)
		authGroup.POST("/mentions/preview", mentionAPI.PreviewMentions)

		// 文档审批流（G007）
		authGroup.POST("/documents/approvals", approvalAPI.SubmitApproval)
		authGroup.POST("/documents/approvals/:id/handle", approvalAPI.HandleApproval)
		authGroup.GET("/documents/:id/approval", approvalAPI.GetDocumentApproval)
		authGroup.GET("/documents/approvals/pending", approvalAPI.ListPendingApprovals)
		authGroup.GET("/documents/approvals/submitted", approvalAPI.ListSubmittedApprovals)

		// 文档模板（G015）
		authGroup.POST("/templates", templateAPI.CreateTemplate)
		authGroup.PUT("/templates/:id", templateAPI.UpdateTemplate)
		authGroup.DELETE("/templates/:id", templateAPI.DeleteTemplate)
		authGroup.GET("/templates/:id", templateAPI.GetTemplate)
		authGroup.GET("/templates", templateAPI.ListTemplates)

		// 文档翻译（P2-2 多语言）
		authGroup.POST("/documents/:id/translations", translationAPI.CreateTranslation)
		authGroup.DELETE("/translations/:id", translationAPI.DeleteTranslation)

		// 版本标签/里程碑
		authGroup.POST("/documents/:id/version-tags", versionTagAPI.CreateVersionTag)
		authGroup.GET("/documents/:id/version-tags", versionTagAPI.GetDocumentVersionTags)
		authGroup.GET("/documents/:id/versions/:version/tags", versionTagAPI.GetVersionTags)
		authGroup.DELETE("/version-tags/:tagId", versionTagAPI.DeleteVersionTag)
		authGroup.GET("/spaces/:id/version-tags", versionTagAPI.GetSpaceVersionTags)

		// 全局标签管理
		authGroup.GET("/tags", docTagAPI.GetAllTags)
		authGroup.GET("/tags/stats", docTagAPI.GetTagStats)
		authGroup.GET("/tags/:tag/documents", docTagAPI.GetDocumentsByTag)

		// 标签层级管理
		authGroup.POST("/tags/hierarchy", tagHierarchyAPI.CreateHierarchy)
		authGroup.GET("/tags/hierarchy/tree", tagHierarchyAPI.GetTagTree)
		authGroup.GET("/tags/:tag/children", tagHierarchyAPI.GetChildren)
		authGroup.GET("/tags/:tag/parent", tagHierarchyAPI.GetParent)
		authGroup.DELETE("/tags/hierarchy/:id", tagHierarchyAPI.DeleteHierarchy)

		// 保存的搜索
		authGroup.POST("/saved-searches", savedSearchAPI.CreateSavedSearch)
		authGroup.GET("/saved-searches", savedSearchAPI.GetUserSavedSearches)
		authGroup.GET("/saved-searches/public", savedSearchAPI.GetPublicSavedSearches)
		authGroup.GET("/saved-searches/:id", savedSearchAPI.GetSavedSearch)
		authGroup.PUT("/saved-searches/:id", savedSearchAPI.UpdateSavedSearch)
		authGroup.POST("/saved-searches/:id/use", savedSearchAPI.UseSavedSearch)
		authGroup.DELETE("/saved-searches/:id", savedSearchAPI.DeleteSavedSearch)

		// Webhook 通知
		authGroup.POST("/webhooks", webhookAPI.CreateWebhook)
		authGroup.GET("/webhooks", webhookAPI.GetUserWebhooks)
		authGroup.GET("/webhooks/events", webhookAPI.GetSupportedEvents)
		authGroup.GET("/webhooks/:id", webhookAPI.GetWebhook)
		authGroup.PUT("/webhooks/:id", webhookAPI.UpdateWebhook)
		authGroup.DELETE("/webhooks/:id", webhookAPI.DeleteWebhook)
		authGroup.GET("/webhooks/:id/deliveries", webhookAPI.GetWebhookDeliveries)

		// 图片上传与查询
		authGroup.GET("/images/:document_id", imageAPI.GetDocumentImages)

		// 附件上传与管理
		authGroup.POST("/attachments/upload/:document_id", attachmentAPI.UploadAttachment)
		authGroup.GET("/attachments/:document_id", attachmentAPI.GetDocumentAttachments)
		authGroup.DELETE("/attachments/:id", attachmentAPI.DeleteAttachment)
		authGroup.GET("/attachments/download/:id", attachmentAPI.DownloadAttachment)

		// Git 同步已下线（割接决策 #4）：PullFromGit 直接写 data/markdowns，与正文落库冲突。
		// 相关 API/服务/调度器/模型随 P3 移除；存量 aw_git_sync_* 表保留但不再读写。

		// 文档视图模式（列表/看板/画廊）
		authGroup.GET("/documents/view/list", docViewAPI.GetDocumentListView)
		authGroup.GET("/documents/view/kanban", docViewAPI.GetDocumentKanbanView)
		authGroup.GET("/documents/view/gallery", docViewAPI.GetDocumentGalleryView)
	}

	// 公开路由（空间/文档基础查询，可选认证：已登录用户可访问受限内容）
	publicGroup := v1.Group("")
	publicGroup.Use(middleware.OptionalAuthMiddleware())
	{
		publicGroup.GET("/spaces", spaceAPI.GetAllSpaces)
		publicGroup.GET("/spaces/tags", spaceAPI.SearchTags)
		publicGroup.GET("/spaces/:id", spaceAPI.GetSpaceByID)
		publicGroup.GET("/documents/tree", docAPI.GetDocumentTree)
		publicGroup.GET("/documents/roots", docAPI.GetRootDocuments)
		// /documents/search 已于 P2-1 移除（与 /search/fulltext 重复，前端统一走 fulltextSearch）
		publicGroup.GET("/documents/:id/share", docAPI.ShareDocument)
		publicGroup.GET("/documents/:id/children", docAPI.GetDocumentChildren)
		publicGroup.GET("/documents/:id", docAPI.GetDocumentByID)
		publicGroup.GET("/documents/:id/content", docAPI.GetDocumentContent)
		publicGroup.GET("/documents/:id/path", docAPI.GetDocumentPath)
		publicGroup.GET("/documents/:id/full", docAPI.GetDocumentFull)
		publicGroup.GET("/documents/:id/stats", docAPI.GetDocumentStats)
		publicGroup.GET("/documents/:id/translations", translationAPI.GetTranslations)
		publicGroup.GET("/translations/languages", translationAPI.GetSupportedLanguages)
	}
}
