package ai

import (
	"github.com/gin-gonic/gin"

	"github.com/listen808/citebase/internal/middleware"
	"github.com/listen808/citebase/internal/response"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
)

// 本文件收敛 Wiki 接口的「按 space_id 直接操作」归属校验。
//
// wiki 的 service 层与 api/workspace 下的标签、版本标签等接口一样，签名里没有
// 用户维度（只收 spaceID），所以越权只能在 handler 层收口：
// 只校验 RequireUser 等于任何登录用户都能读写他人私有空间的知识页。
//
// 读口径：CheckSpaceAccess（与文档树一致，公开空间对已登录用户放行）；
// 写口径：GetUserSpacePrivilege >= 1（成员）；超级管理员在该方法里直接返回 2。

// wikiSpaceSvc 复用 SpaceService（构造无副作用，权限判定统一走同一套实现）。
func wikiSpaceSvc() *workspacesvc.SpaceService {
	return workspacesvc.NewSpaceService()
}

// requireWikiReadable 校验当前用户可读目标空间。失败时已写出响应，调用方直接 return。
func requireWikiReadable(c *gin.Context, spaceID int) bool {
	uid, ok := middleware.RequireUser(c)
	if !ok {
		return false
	}
	if spaceID <= 0 {
		response.NotFound(c, "space not found")
		return false
	}
	granted, err := wikiSpaceSvc().CheckSpaceAccess(spaceID, uid)
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

// requireWikiMember 校验当前用户是目标空间成员（privilege>=1），用于写接口。
func requireWikiMember(c *gin.Context, spaceID int) bool {
	uid, ok := middleware.RequireUser(c)
	if !ok {
		return false
	}
	if spaceID <= 0 {
		response.NotFound(c, "space not found")
		return false
	}
	privilege, err := wikiSpaceSvc().GetUserSpacePrivilege(spaceID, uid)
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
