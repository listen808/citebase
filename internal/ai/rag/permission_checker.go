package rag

import (
	"context"
	"errors"
	"fmt"

	"github.com/listen808/citebase/internal/ctxkeys"
	logx "github.com/listen808/citebase/internal/log"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
)

type PermissionChecker struct {
	spaceRepo *workspacerepo.SpaceRepository
}

func NewPermissionChecker() *PermissionChecker {
	return &PermissionChecker{
		spaceRepo: workspacerepo.NewSpaceRepository(),
	}
}

// FilterSourcesByPermission 过滤掉调用者无权访问的空间来源。
//
// 两处修正：
//  1. 可见集合改走 FindAccessibleSpaceIDs，与检索/搜索使用同一套语义
//     （visit_level 历史上存在 "0/1/2" 与 "public/login/member" 两种编码，
//     只比较 "public" 会漏判）；
//  2. 查询失败时返回空集而不是原样放行 —— 权限判定出错绝不能变成 fail-open。
func (pc *PermissionChecker) FilterSourcesByPermission(ctx context.Context, userID int, sources []Source) []Source {
	if len(sources) == 0 {
		return sources
	}

	visibleIDs, err := pc.spaceRepo.FindAccessibleSpaceIDs(userID, false)
	if err != nil {
		logx.Warnf("[RAG] 可见空间查询失败，按最小权限丢弃全部检索来源: %v", err)
		return nil
	}
	visible := make(map[int]bool, len(visibleIDs))
	for _, id := range visibleIDs {
		visible[id] = true
	}

	filtered := make([]Source, 0, len(sources))
	for _, src := range sources {
		if visible[src.SpaceID] {
			filtered = append(filtered, src)
		}
	}
	return filtered
}

// CheckSpaceAccess 判断用户能否访问指定空间。
// spaceID <= 0 表示「不限定空间」，在权限判断里不是合法输入，一律拒绝。
func (pc *PermissionChecker) CheckSpaceAccess(userID, spaceID int) bool {
	if spaceID <= 0 {
		return false
	}

	visibleIDs, err := pc.spaceRepo.FindAccessibleSpaceIDs(userID, false)
	if err != nil {
		return false
	}
	for _, id := range visibleIDs {
		if id == spaceID {
			return true
		}
	}
	return false
}

// ErrPermissionDenied 表示当前调用者无权访问目标对象。
var ErrPermissionDenied = errors.New("无权访问该资源")

// ctxUserID 取出上下文中的调用者 ID。
func ctxUserID(ctx context.Context) int {
	return ctxkeys.UserID(ctx)
}

// EnsureSpaceVisible 供工具层复用：目标空间不在调用者可见范围内时返回错误。
// 工具参数里的 space_id / document_id 都来自模型输出，绝不能当作可信输入直接使用。
func (pc *PermissionChecker) EnsureSpaceVisible(ctx context.Context, spaceID int) error {
	if spaceID <= 0 {
		return ErrPermissionDenied
	}
	if !pc.CheckSpaceAccess(ctxUserID(ctx), spaceID) {
		return ErrPermissionDenied
	}
	return nil
}

// ensureSpaceVisible 包级便捷函数：目标空间对当前调用者不可见时返回 ErrPermissionDenied。
// 每次新建 PermissionChecker（而非包级 var）：NewSpaceRepository 捕获 repos.DB 指针，
// 包加载时 DB 还是 nil，用包级 var 会永远拿到空指针。
func ensureSpaceVisible(ctx context.Context, spaceID int) error {
	pc := NewPermissionChecker()
	return pc.EnsureSpaceVisible(ctx, spaceID)
}

func (pc *PermissionChecker) BuildPermissionContext(userID int, roleID int) string {
	roleName := "普通用户"
	if roleID == 1 {
		roleName = "超级管理员"
	} else if roleID == 2 {
		roleName = "管理员"
	}

	spaces, _, err := pc.spaceRepo.FindAll(0, 10000, "", "", "")
	spaceCount := 0
	if err == nil {
		for _, s := range spaces {
			if s.IsPublic() || (userID > 0 && s.IsLoggedInVisible()) {
				spaceCount++
			} else {
				member, err := pc.spaceRepo.FindSpaceUser(s.ID, userID)
				if err == nil && member != nil {
					spaceCount++
				}
			}
		}
	}

	return fmt.Sprintf("当前用户角色: %s, 可访问空间数: %d", roleName, spaceCount)
}
