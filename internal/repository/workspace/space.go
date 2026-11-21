package workspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	adminmodel "github.com/listen808/citebase/internal/model/admin"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrSpaceNotFound = errors.New("space not found")
)

// SpaceRepository 空间数据访问层
type SpaceRepository struct {
	db *gorm.DB
}

// NewSpaceRepository 创建空间仓储实例
func NewSpaceRepository() *SpaceRepository {
	return &SpaceRepository{db: repos.DB}
}

// NewSpaceRepositoryWithDB 使用指定 repos.DB 创建空间仓储实例（用于事务）
func NewSpaceRepositoryWithDB(db *gorm.DB) *SpaceRepository {
	return &SpaceRepository{db: db}
}

// Transaction 在事务中执行回调函数
func (r *SpaceRepository) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

// Create 创建空间
func (r *SpaceRepository) Create(space *workspacemodel.Space) error {
	return r.db.Create(space).Error
}

// FindByID 根据 ID 查找空间
func (r *SpaceRepository) FindByID(id int) (*workspacemodel.Space, error) {
	var space workspacemodel.Space
	err := r.db.Scopes(repos.NotDeleted).First(&space, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSpaceNotFound
		}
		return nil, err
	}
	return &space, nil
}

// spaceCacheTTL 空间缓存有效期：空间元信息变更极少，5 分钟足够
const spaceCacheTTL = 5 * time.Minute

// FindByIDCached 查询空间并使用 AppCache 缓存
// 空间元信息（name、is_share 等）变更极少，缓存 5 分钟可消除大量重复 DB 查询
func (r *SpaceRepository) FindByIDCached(id int) (*workspacemodel.Space, error) {
	if id <= 0 {
		return nil, ErrSpaceNotFound
	}
	cacheKey := fmt.Sprintf("space:%d", id)
	ctx := context.Background()

	// 尝试从缓存读取
	if repos.AppCache != nil {
		var cached workspacemodel.Space
		if err := repos.AppCache.GetJSON(ctx, cacheKey, &cached); err == nil && cached.ID > 0 {
			return &cached, nil
		}
	}

	// 缓存未命中，查 DB
	space, err := r.FindByID(id)
	if err != nil {
		return nil, err
	}

	// 回填缓存（失败不阻断主流程）
	if repos.AppCache != nil && space != nil {
		_ = repos.AppCache.SetJSON(ctx, cacheKey, space, spaceCacheTTL)
	}
	return space, nil
}

// InvalidateSpaceCache 清除指定空间的缓存（空间更新/删除时调用）
func (r *SpaceRepository) InvalidateSpaceCache(id int) {
	if repos.AppCache == nil || id <= 0 {
		return
	}
	ctx := context.Background()
	_ = repos.AppCache.Del(ctx, fmt.Sprintf("space:%d", id))
}

// Update 更新空间信息
func (r *SpaceRepository) Update(space *workspacemodel.Space) error {
	err := r.db.Save(space).Error
	if err == nil && space != nil {
		r.InvalidateSpaceCache(space.ID)
	}
	return err
}

// Delete 删除空间 (软删除)
func (r *SpaceRepository) Delete(id int) error {
	err := r.db.Model(&workspacemodel.Space{}).Where("space_id = ?", id).Update("is_delete", 1).Error
	if err == nil {
		r.InvalidateSpaceCache(id)
	}
	return err
}

// FindAllVisible 面向「列表/浏览」场景：只在调用者可见的空间集合内分页。
// visibleIDs 为空即返回空集（绝不退化成全表可见）；unrestricted=true（超级管理员）不裁剪。
func (r *SpaceRepository) FindAllVisible(offset, limit int, keyword string, tags string, visitLevel string, visibleIDs []int, unrestricted bool) ([]*workspacemodel.Space, int64, error) {
	if !unrestricted && len(visibleIDs) == 0 {
		return []*workspacemodel.Space{}, 0, nil
	}

	if unrestricted {
		return r.FindAll(offset, limit, keyword, tags, visitLevel)
	}

	var spaces []*workspacemodel.Space
	var total int64
	query := r.db.Model(&workspacemodel.Space{}).Scopes(repos.NotDeleted).Where("space_id IN ?", visibleIDs)
	if keyword != "" {
		query = query.Where("name LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if tags != "" {
		query = query.Where("tags LIKE ?", "%"+tags+"%")
	}
	if visitLevel != "" {
		query = query.Where("visit_level IN ?", workspacemodel.VisitLevelAliases(visitLevel))
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("sequence ASC, space_id DESC").Offset(offset).Limit(limit).Find(&spaces).Error
	return spaces, total, err
}

// FindAll 获取所有空间 (分页，支持关键词/标签筛选)
//
// 注意：这是「无可见性裁剪」的全量查询，只能用于索引重建、后台统计等内部链路；
// 任何面向用户的列表都必须走 FindAllVisible，否则私有空间的名称与描述会被任意人枚举。
func (r *SpaceRepository) FindAll(offset, limit int, keyword string, tags string, visitLevel string) ([]*workspacemodel.Space, int64, error) {
	var spaces []*workspacemodel.Space
	var total int64

	query := r.db.Model(&workspacemodel.Space{}).Scopes(repos.NotDeleted)

	if keyword != "" {
		query = query.Where("name LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if tags != "" {
		// 标签模糊匹配，逗号分隔的标签列表
		query = query.Where("tags LIKE ?", "%"+tags+"%")
	}
	if visitLevel != "" {
		query = query.Where("visit_level IN ?", workspacemodel.VisitLevelAliases(visitLevel))
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("sequence ASC, space_id DESC").Offset(offset).Limit(limit).Find(&spaces).Error
	return spaces, total, err
}

// FindByUserID 获取用户参与的所有空间 (分页，支持关键词/标签筛选)
func (r *SpaceRepository) FindByUserID(userID int, offset, limit int, keyword string, tags string) ([]*workspacemodel.Space, int64, error) {
	var spaces []*workspacemodel.Space
	var total int64

	// 使用 GORM 查询避免字段遗漏，使用 NotDeleted scope 替代硬编码 is_delete = 0
	query := r.db.Model(&workspacemodel.Space{}).
		Joins("INNER JOIN aw_space_user su ON aw_space.space_id = su.space_id").
		Where("su.user_id = ?", userID).
		Scopes(repos.NotDeleted)

	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("aw_space.name LIKE ? OR aw_space.description LIKE ?", like, like)
	}
	if tags != "" {
		query = query.Where("aw_space.tags LIKE ?", "%"+tags+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("aw_space.sequence ASC, aw_space.space_id DESC").
		Offset(offset).Limit(limit).
		Find(&spaces).Error; err != nil {
		return nil, 0, err
	}

	return spaces, total, nil
}

// BatchUpdateSequence 批量更新空间排序序号
func (r *SpaceRepository) BatchUpdateSequence(orders []workspacemodel.SpaceOrder) error {
	if len(orders) == 0 {
		return nil
	}
	for _, order := range orders {
		if err := r.db.Model(&workspacemodel.Space{}).Where("space_id = ?", order.ID).Update("sequence", order.Sequence).Error; err != nil {
			return err
		}
	}
	return nil
}

// GetMaxSequence 获取当前最大排序序号（用于新建空间默认值）
func (r *SpaceRepository) GetMaxSequence() (int, error) {
	var maxSeq int
	err := r.db.Model(&workspacemodel.Space{}).Scopes(repos.NotDeleted).Select("COALESCE(MAX(sequence), 0)").Scan(&maxSeq).Error
	return maxSeq, err
}

// FindSpaceUser 获取空间成员关系
func (r *SpaceRepository) FindSpaceUser(spaceID, userID int) (*workspacemodel.SpaceUser, error) {
	var spaceUser workspacemodel.SpaceUser
	err := r.db.Where("space_id = ? AND user_id = ?", spaceID, userID).First(&spaceUser).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("user is not a member of this space")
		}
		return nil, err
	}
	return &spaceUser, nil
}

// spaceUserCacheTTL 空间成员关系缓存：权限变更极少，2 分钟足够
// 用于图片/附件权限校验高频场景，避免每张图片都查 DB
const spaceUserCacheTTL = 2 * time.Minute

// FindSpaceUserCached 获取空间成员关系（带缓存）
// 用于图片/附件权限校验：一篇文档 N 张图片，避免 N 次 DB 查询
func (r *SpaceRepository) FindSpaceUserCached(spaceID, userID int) (*workspacemodel.SpaceUser, error) {
	if spaceID <= 0 || userID <= 0 {
		return nil, errors.New("user is not a member of this space")
	}
	cacheKey := fmt.Sprintf("space_user:%d:%d", spaceID, userID)
	ctx := context.Background()

	// 尝试从缓存读取
	if repos.AppCache != nil {
		var cached workspacemodel.SpaceUser
		if err := repos.AppCache.GetJSON(ctx, cacheKey, &cached); err == nil && cached.SpaceUserID > 0 {
			return &cached, nil
		}
	}

	// 缓存未命中，查 DB
	spaceUser, err := r.FindSpaceUser(spaceID, userID)
	if err != nil {
		return nil, err
	}

	// 回填缓存（失败不阻断主流程）
	if repos.AppCache != nil && spaceUser != nil {
		_ = repos.AppCache.SetJSON(ctx, cacheKey, spaceUser, spaceUserCacheTTL)
	}
	return spaceUser, nil
}

// InvalidateSpaceUserCache 清除指定用户在指定空间的成员关系缓存
func (r *SpaceRepository) InvalidateSpaceUserCache(spaceID, userID int) {
	if repos.AppCache == nil || spaceID <= 0 || userID <= 0 {
		return
	}
	ctx := context.Background()
	_ = repos.AppCache.Del(ctx, fmt.Sprintf("space_user:%d:%d", spaceID, userID))
}

// AddSpaceUser 添加空间成员
func (r *SpaceRepository) AddSpaceUser(spaceUser *workspacemodel.SpaceUser) error {
	err := r.db.Create(spaceUser).Error
	if err == nil && spaceUser != nil {
		r.InvalidateSpaceUserCache(spaceUser.SpaceID, spaceUser.UserID)
	}
	return err
}

// UpdateSpaceUserPrivilege 更新成员权限
func (r *SpaceRepository) UpdateSpaceUserPrivilege(spaceID, userID, privilege int) error {
	err := r.db.Model(&workspacemodel.SpaceUser{}).
		Where("space_id = ? AND user_id = ?", spaceID, userID).
		Update("privilege", privilege).Error
	if err == nil {
		r.InvalidateSpaceUserCache(spaceID, userID)
	}
	return err
}

// RemoveSpaceUser 移除空间成员
func (r *SpaceRepository) RemoveSpaceUser(spaceID, userID int) error {
	err := r.db.Where("space_id = ? AND user_id = ?", spaceID, userID).Delete(&workspacemodel.SpaceUser{}).Error
	if err == nil {
		r.InvalidateSpaceUserCache(spaceID, userID)
	}
	return err
}

// GetSpaceUsers 获取空间所有成员
func (r *SpaceRepository) GetSpaceUsers(spaceID int) ([]*workspacemodel.SpaceUser, error) {
	var spaceUsers []*workspacemodel.SpaceUser
	err := r.db.Where("space_id = ?", spaceID).Find(&spaceUsers).Error
	return spaceUsers, err
}

// GetSpaceMemberCount 获取空间成员数量
func (r *SpaceRepository) GetSpaceMemberCount(spaceID int) (int64, error) {
	var count int64
	err := r.db.Model(&workspacemodel.SpaceUser{}).Where("space_id = ?", spaceID).Count(&count).Error
	return count, err
}

// CheckSpacePrivilege 检查用户权限 (返回：是否存在，权限等级)
func (r *SpaceRepository) CheckSpacePrivilege(spaceID, userID int) (bool, int, error) {
	var spaceUser workspacemodel.SpaceUser
	err := r.db.Where("space_id = ? AND user_id = ?", spaceID, userID).Limit(1).Find(&spaceUser).Error
	if err != nil {
		return false, 0, err
	}
	if spaceUser.SpaceUserID == 0 {
		return false, 0, nil
	}
	return true, spaceUser.Privilege, nil
}

// GetSpaceCreator 获取空间创建者 ID
func (r *SpaceRepository) GetSpaceCreator(spaceID int) (int, error) {
	var spaceUser workspacemodel.SpaceUser
	// 创建者是第一个加入空间的成员（privilege=2 的管理员中最小的 space_user_id）
	err := r.db.Where("space_id = ? AND privilege = 2", spaceID).
		Order("space_user_id ASC").First(&spaceUser).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return spaceUser.UserID, nil
}

// HasSpaceName 检查空间名是否已存在
func (r *SpaceRepository) HasSpaceName(name string) (bool, error) {
	var count int64
	err := r.db.Model(&workspacemodel.Space{}).Where("name = ?", name).Scopes(repos.NotDeleted).Count(&count).Error
	return count > 0, err
}

// HasSameName 检查空间名是否已存在（排除自身）
func (r *SpaceRepository) HasSameName(spaceID int, name string) (bool, error) {
	var count int64
	err := r.db.Model(&workspacemodel.Space{}).Where("name = ? AND space_id != ?", name, spaceID).Scopes(repos.NotDeleted).Count(&count).Error
	return count > 0, err
}

// DeleteSpaceUsersBySpaceID 删除空间所有成员
func (r *SpaceRepository) DeleteSpaceUsersBySpaceID(spaceID int) error {
	return r.db.Where("space_id = ?", spaceID).Delete(&workspacemodel.SpaceUser{}).Error
}

// DeleteSpaceDocuments 软删除空间所有文档
func (r *SpaceRepository) DeleteSpaceDocuments(spaceID int) error {
	return r.db.Model(&workspacemodel.Document{}).Where("space_id = ?", spaceID).Scopes(repos.NotDeleted).Update("is_delete", 1).Error
}

// CountDocumentsBySpaceID 统计空间下未删除的文档数量
func (r *SpaceRepository) CountDocumentsBySpaceID(spaceID int) (int64, error) {
	var count int64
	err := r.db.Model(&workspacemodel.Document{}).Where("space_id = ?", spaceID).Scopes(repos.NotDeleted).Count(&count).Error
	return count, err
}

// CountMembersBySpaceID 统计空间成员数量
func (r *SpaceRepository) CountMembersBySpaceID(spaceID int) (int64, error) {
	var count int64
	err := r.db.Model(&workspacemodel.SpaceUser{}).Where("space_id = ?", spaceID).Count(&count).Error
	return count, err
}

// DeleteSpaceAttachments 删除空间所有附件记录
func (r *SpaceRepository) DeleteSpaceAttachments(spaceID int) error {
	var docIDs []int
	err := r.db.Model(&workspacemodel.Document{}).Where("space_id = ?", spaceID).Scopes(repos.NotDeleted).Pluck("document_id", &docIDs).Error
	if err != nil {
		return err
	}
	if len(docIDs) == 0 {
		return nil
	}
	return r.db.Where("document_id IN ?", docIDs).Delete(&workspacemodel.Attachment{}).Error
}

// SearchByTag 按标签搜索空间（返回所有匹配的空间名称，用于自动补全）
func (r *SpaceRepository) SearchByTag(tag string, limit int) ([]string, error) {
	var tags []string
	err := r.db.Model(&workspacemodel.Space{}).Scopes(repos.NotDeleted).
		Where("tags != ''").
		Pluck("tags", &tags).Error
	if err != nil {
		return nil, err
	}

	// 解析所有标签并匹配
	tagSet := make(map[string]bool)
	for _, t := range tags {
		for _, part := range strings.Split(t, ",") {
			part = strings.TrimSpace(part)
			if strings.Contains(part, tag) {
				tagSet[part] = true
			}
		}
	}

	result := make([]string, 0, len(tagSet))
	for t := range tagSet {
		result = append(result, t)
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

// CountAll 统计所有空间数
func (r *SpaceRepository) CountAll() (int64, error) {
	var count int64
	err := r.db.Model(&workspacemodel.Space{}).Scopes(repos.NotDeleted).Count(&count).Error
	return count, err
}

// FindAccessibleSpaceIDs 获取用户可访问的空间 ID 列表
// visit_level: 0=所有人可见, 1=登录用户可见, 2=仅空间成员可见
func (r *SpaceRepository) FindAccessibleSpaceIDs(userID int, isSuperAdmin bool) ([]int, error) {
	if isSuperAdmin {
		// 超级管理员可访问所有空间
		var ids []int
		err := r.db.Model(&workspacemodel.Space{}).Scopes(repos.NotDeleted).Pluck("space_id", &ids).Error
		return ids, err
	}

	if userID == 0 {
		// 未登录用户：仅公开空间
		var ids []int
		err := r.db.Model(&workspacemodel.Space{}).Scopes(repos.NotDeleted).Where("visit_level IN ?", []string{"0", "public"}).Pluck("space_id", &ids).Error
		return ids, err
	}

	// 登录用户：公开空间 + 登录可见空间 + 用户所属的成员空间
	var ids []int
	err := r.db.Model(&workspacemodel.Space{}).Scopes(repos.NotDeleted).Where("visit_level IN ?", []string{"0", "public", "1", "login"}).Pluck("space_id", &ids).Error
	if err != nil {
		return nil, err
	}

	// 加上用户所属的空间（visit_level='2'/'member' 的空间）
	// 注意：aw_space_user 没有 is_delete 列，软删过滤必须作用于 join 的 aw_space 表，
	// 否则 SQL 报错 1054（Unknown column 'aw_space_user.is_delete'）导致该函数整体失败，
	// 上层 fail-closed 后会把搜索结果全部清空。
	var memberIDs []int
	err = r.db.Model(&workspacemodel.SpaceUser{}).
		Joins("INNER JOIN aw_space ON aw_space.space_id = aw_space_user.space_id").
		Where("aw_space_user.user_id = ?", userID).
		Where("aw_space.is_delete = ?", 0).
		Pluck("aw_space_user.space_id", &memberIDs).Error
	if err != nil {
		return nil, err
	}

	// 合并去重
	idSet := make(map[int]bool)
	for _, id := range ids {
		idSet[id] = true
	}
	for _, id := range memberIDs {
		idSet[id] = true
	}
	result := make([]int, 0, len(idSet))
	for id := range idSet {
		result = append(result, id)
	}
	return result, nil
}

// GetTopCollector 获取收藏最多的用户
func (r *SpaceRepository) GetTopCollector() *adminmodel.UserStatItem {
	var result struct {
		UserID    int
		Username  string
		GivenName string
		Count     int
	}
	err := r.db.Table("aw_collection").
		Select("aw_collection.user_id, aw_user.username, aw_user.given_name, COUNT(*) as count").
		Joins("LEFT JOIN aw_user ON aw_collection.user_id = aw_user.user_id").
		Where("aw_collection.type = 1"). // 文档收藏
		Group("aw_collection.user_id").
		Order("count DESC").
		Limit(1).
		Scan(&result).Error
	if err != nil || result.UserID == 0 {
		return nil
	}
	return &adminmodel.UserStatItem{UserID: result.UserID, Username: result.Username, GivenName: result.GivenName, Count: result.Count}
}

// GetSpaceRank 获取空间文档排行
func (r *SpaceRepository) GetSpaceRank(limit int) []adminmodel.SpaceDocCount {
	var results []struct {
		SpaceID int
		Total   int
	}
	err := r.db.Table("aw_document").
		Select("space_id, COUNT(*) as total").Scopes(repos.NotDeleted).
		Group("space_id").
		Order("total DESC").
		Limit(limit).
		Scan(&results).Error
	if err != nil {
		return nil
	}

	// 填充空间名称
	spaceIDs := make([]int, len(results))
	for i, r := range results {
		spaceIDs[i] = r.SpaceID
	}

	spaceMap := make(map[int]string)
	if len(spaceIDs) > 0 {
		var spaces []workspacemodel.Space
		if err := r.db.Where("space_id IN ?", spaceIDs).Find(&spaces).Error; err != nil {
			return nil
		}
		for _, s := range spaces {
			spaceMap[s.ID] = s.Name
		}
	}

	rank := make([]adminmodel.SpaceDocCount, len(results))
	for i, r := range results {
		rank[i] = adminmodel.SpaceDocCount{
			SpaceID:   r.SpaceID,
			SpaceName: spaceMap[r.SpaceID],
			Total:     r.Total,
		}
	}
	return rank
}

// FindByIDs 根据 ID 列表批量查询空间（排除已删除的）
func (r *SpaceRepository) FindByIDs(ids []int) ([]*workspacemodel.Space, error) {
	if len(ids) == 0 {
		return []*workspacemodel.Space{}, nil
	}
	var spaces []*workspacemodel.Space
	err := r.db.Where("space_id IN ?", ids).Scopes(repos.NotDeleted).Find(&spaces).Error
	return spaces, err
}

// BatchSpaceMemberCount 批量查询多个空间的成员数（消除 N+1）
// 返回 map[spaceID]int64，未出现在 map 中的 spaceID 视为 0
func (r *SpaceRepository) BatchSpaceMemberCount(spaceIDs []int) (map[int]int64, error) {
	result := make(map[int]int64, len(spaceIDs))
	if len(spaceIDs) == 0 {
		return result, nil
	}
	type row struct {
		SpaceID int   `gorm:"column:space_id"`
		Count   int64 `gorm:"column:count"`
	}
	var rows []row
	err := r.db.Model(&workspacemodel.SpaceUser{}).
		Select("space_id, COUNT(*) as count").
		Where("space_id IN ?", spaceIDs).
		Group("space_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.SpaceID] = r.Count
	}
	return result, nil
}

// BatchSpaceDocCount 批量查询多个空间的未删除文档数（消除 N+1）
func (r *SpaceRepository) BatchSpaceDocCount(spaceIDs []int) (map[int]int64, error) {
	result := make(map[int]int64, len(spaceIDs))
	if len(spaceIDs) == 0 {
		return result, nil
	}
	type row struct {
		SpaceID int   `gorm:"column:space_id"`
		Count   int64 `gorm:"column:count"`
	}
	var rows []row
	err := r.db.Model(&workspacemodel.Document{}).
		Select("space_id, COUNT(*) as count").
		Where("space_id IN ?", spaceIDs).
		Scopes(repos.NotDeleted).
		Group("space_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.SpaceID] = r.Count
	}
	return result, nil
}

// BatchUserSpacePrivileges 批量查询用户在多个空间的权限（消除 N+1）
// 返回 map[spaceID]int，未出现在 map 中的 spaceID 视为非成员
func (r *SpaceRepository) BatchUserSpacePrivileges(userID int, spaceIDs []int) (map[int]int, error) {
	result := make(map[int]int, len(spaceIDs))
	if len(spaceIDs) == 0 || userID <= 0 {
		return result, nil
	}
	type row struct {
		SpaceID   int `gorm:"column:space_id"`
		Privilege int `gorm:"column:privilege"`
	}
	var rows []row
	err := r.db.Model(&workspacemodel.SpaceUser{}).
		Select("space_id, privilege").
		Where("user_id = ? AND space_id IN ?", userID, spaceIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.SpaceID] = r.Privilege
	}
	return result, nil
}
