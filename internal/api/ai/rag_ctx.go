package ai

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/middleware"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/response"
)

// userCtx 为 RAG / Agent 链路补齐调用者身份。
//
// 检索器的权限过滤（FilterSourcesByPermission）与工具层的 space/document 归属校验
// 都以 ctx 中的 user_id 为前提；缺省时它们按「匿名 = 不过滤」处理，等于把私有空间内容
// 直接拼进 prompt 并随 sources 回传。因此所有会触达知识库内容的入口都必须带上下文，
// 而不是各自忘记注入。
func userCtx(c *gin.Context) context.Context {
	ctx := c.Request.Context()
	uid, err := middleware.GetUserID(c)
	if err != nil || uid <= 0 {
		// 保留 0：下游据此判定为「无身份」，由 fail-closed 逻辑拒绝跨空间读取
		return ctx
	}
	return provider.ContextWithUserID(ctx, uid)
}

// requireUserID 返回当前登录用户 ID；未认证时直接写出 401 并返回 false。
//
// 不要用 c.GetUint("user_id")：认证中间件存入的是 Go int，而 gin 的 GetUint 只对 uint
// 生效、其余情况静默返回 0 —— 于是「每个用户自己的数据」会全部落进 user_id=0 这一个
// 共享桶，形成横向越权（可读取、也可删除他人数据）。
func requireUserID(c *gin.Context) (uint, bool) {
	uid, err := middleware.GetUserID(c)
	if err != nil || uid <= 0 {
		response.Unauthorized(c, "unauthorized")
		return 0, false
	}
	return uint(uid), true
}

// accessibleSpaceIDs 返回当前调用者可见的空间 ID 集合；同时给出是否超管（不裁剪）。
// 知识库列表、分块统计这类「按空间聚合」的接口必须用它裁剪，否则会向任意登录用户
// 暴露全部私有空间的名称、文档数与分块内容。
func accessibleSpaceIDs(c *gin.Context) ([]int, bool, error) {
	uid, err := middleware.GetUserID(c)
	if err != nil {
		return nil, false, err
	}
	isSuper := middleware.IsSuperAdmin(c)
	ids, err := workspacerepo.NewSpaceRepository().FindAccessibleSpaceIDs(uid, isSuper)
	if err != nil {
		return nil, isSuper, err
	}
	return ids, isSuper, nil
}

// canAccessSpace 判断单个空间是否在调用者可见范围内。
func canAccessSpace(spaceID int, visibleIDs []int, unrestricted bool) bool {
	if unrestricted {
		return true
	}
	for _, id := range visibleIDs {
		if id == spaceID {
			return true
		}
	}
	return false
}
