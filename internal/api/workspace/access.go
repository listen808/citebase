package workspace

import (
	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	"github.com/listen808/citebase/internal/repository"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/response"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
)

// 本文件收敛「按 document_id / space_id 直接操作」的接口归属校验。
//
// 标签、版本标签、视图、翻译关联这类接口的 service 层签名里没有用户维度
// （只收 documentID / spaceID），所以越权只能在 handler 层收口：
// 只校验 RequireUser 等于任何登录用户都能读改他人私有空间的数据。

// spaceSvc 复用 SpaceService（其构造无副作用，权限判定统一走 GetUserSpacePrivilege）
func spaceSvc() *workspacesvc.SpaceService {
	return workspacesvc.NewSpaceService()
}

// requireSpaceMember 校验当前登录用户是目标空间成员（privilege>=1）。
// 超级管理员在 GetUserSpacePrivilege 中直接返回 2，因此天然放行。
// 校验失败时已写出响应，调用方只需 return。
func requireSpaceMember(c *gin.Context, spaceID int) bool {
	uid, ok := middleware.RequireUser(c)
	if !ok {
		return false
	}
	if spaceID <= 0 {
		response.NotFound(c, "space not found")
		return false
	}
	privilege, err := spaceSvc().GetUserSpacePrivilege(spaceID, uid)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return false
	}
	if privilege < 1 {
		response.ForbiddenKey(c, response.MsgNoPermissionSpaceContent)
		return false
	}
	return true
}

// requireDocMember 先按 document_id 定位所属空间，再做与 requireSpaceMember 相同的成员校验。
// 用于路径里只有 :id（文档 ID）的写接口。
func requireDocMember(c *gin.Context, docID int) bool {
	if _, ok := middleware.RequireUser(c); !ok {
		return false
	}
	doc, err := workspacerepo.NewDocumentRepository().FindByID(docID)
	if err != nil || doc == nil {
		response.NotFound(c, "document not found")
		return false
	}
	return requireSpaceMember(c, doc.SpaceID)
}

// requireSpaceReadable 校验当前用户对空间的读取权限（公开空间对已登录用户放行）。
// 用于按 space_id 返回整棵文档树这类读接口。
func requireSpaceReadable(c *gin.Context, spaceID int) bool {
	uid, ok := middleware.RequireUser(c)
	if !ok {
		return false
	}
	granted, err := spaceSvc().CheckSpaceAccess(spaceID, uid)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return false
	}
	if !granted {
		response.ForbiddenKey(c, response.MsgNoPermissionSpace)
		return false
	}
	return true
}

// requireDocReadable 按 document_id 定位所属空间后做读取校验（与文档详情接口同一口径）。
func requireDocReadable(c *gin.Context, docID int) bool {
	uid, ok := middleware.RequireUser(c)
	if !ok {
		return false
	}
	doc, err := workspacerepo.NewDocumentRepository().FindByID(docID)
	if err != nil || doc == nil {
		response.NotFound(c, "document not found")
		return false
	}
	granted, err := spaceSvc().CheckSpaceAccess(doc.SpaceID, uid)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return false
	}
	if !granted {
		response.ForbiddenKey(c, response.MsgNoPermissionDocSpace)
		return false
	}
	return true
}

// requireVersionTagWritable 按版本标签 ID 反查所属文档后做成员校验。
// 删除接口的路径里只有 tagId，service 层 DeleteTag 也只按 ID 删除，
// 不在此收口时任意登录用户都能删掉他人空间的里程碑标签。
func requireVersionTagWritable(c *gin.Context, tagID int64) bool {
	var tag workspacemodel.DocumentVersionTag
	if err := repository.DB.Where("id = ?", tagID).First(&tag).Error; err != nil {
		response.NotFound(c, "version tag not found")
		return false
	}
	return requireDocMember(c, int(tag.DocumentID))
}

// requireTranslationWritable 按翻译关联 ID 反查原文档后做成员校验。
// service 层 DeleteTranslation(id) 只按主键删除，不校验时任何人都能删断他人的翻译关联。
func requireTranslationWritable(c *gin.Context, translationID int) bool {
	var tr workspacemodel.DocumentTranslation
	if err := repository.DB.Where("id = ?", translationID).First(&tr).Error; err != nil {
		response.NotFound(c, "translation not found")
		return false
	}
	return requireDocMember(c, tr.DocumentID)
}
