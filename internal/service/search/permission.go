package search

import (
	logx "github.com/listen808/citebase/internal/log"
	searchrepo "github.com/listen808/citebase/internal/repository/search"
)

// GetUserAccessibleSpaceIDs 返回用户可访问的空间 ID 列表及是否超管。
// fail-closed 语义：出错或无空间时返回 (nil, false)。
func GetUserAccessibleSpaceIDs(uid int, roleID int) ([]int, bool, error) {
	isSuperAdmin := uid != 0 && roleID == 1
	if isSuperAdmin {
		return nil, true, nil // 超管：无需过滤
	}
	repo := searchrepo.NewSearchRepository()
	accessibleIDs, err := repo.FindAccessibleSpaceIDs(uid, isSuperAdmin)
	if err != nil {
		logx.Errorf("[SearchPermission] FindAccessibleSpaceIDs(uid=%d) error: %v -> fail-closed", uid, err)
		return nil, false, err
	}
	if len(accessibleIDs) == 0 {
		logx.Warnf("[SearchPermission] FindAccessibleSpaceIDs(uid=%d) returned empty -> fail-closed", uid)
		return nil, false, nil
	}
	return accessibleIDs, false, nil
}

// UintSpaces converts a []int to []uint.
func UintSpaces(ids []int) []uint {
	if ids == nil {
		return nil
	}
	result := make([]uint, len(ids))
	for i, id := range ids {
		result[i] = uint(id)
	}
	return result
}

// IntersectSpaceIDs 计算请求空间与用户可访问空间的交集。
// requestedIDs 第个元素是请求的空间 ID 列表（可为空，全空间检索）；
// accessibleIDs 是用户实际可访问的空间 ID。
// 返回值：用户可访问且被请求的空间 ID 列表（uint）；若请求为空则返回全部可访问空间。
func IntersectSpaceIDs(requestedIDs []uint, accessibleIDs []int) []uint {
	if len(accessibleIDs) == 0 {
		return nil
	}
	accessibleSet := make(map[int]bool, len(accessibleIDs))
	for _, sid := range accessibleIDs {
		accessibleSet[sid] = true
	}

	// 用户未指定空间 → 跨空间检索，限权为全部可访问空间
	if len(requestedIDs) == 0 {
		result := make([]uint, 0, len(accessibleIDs))
		for _, sid := range accessibleIDs {
			result = append(result, uint(sid))
		}
		return result
	}

	// 用户指定了空间列表 → 取交集
	var result []uint
	for _, sid := range requestedIDs {
		if accessibleSet[int(sid)] {
			result = append(result, sid)
		}
	}
	return result
}
