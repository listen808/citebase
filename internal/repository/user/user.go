package user

import (
	"database/sql"
	"errors"
	"sort"
	"time"

	adminmodel "github.com/listen808/citebase/internal/model/admin"
	usermodel "github.com/listen808/citebase/internal/model/user"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

// UserRepository 用户数据访问层
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建用户仓储实例
func NewUserRepository() *UserRepository {
	return &UserRepository{db: repos.DB}
}

// NewUserRepositoryWithDB 使用指定 DB 创建用户仓储实例（用于事务）
func NewUserRepositoryWithDB(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Transaction 在事务中执行回调函数
func (r *UserRepository) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

// Create 创建用户
func (r *UserRepository) Create(user *usermodel.User) error {
	return r.db.Create(user).Error
}

// FindByID 根据 ID 查找用户
func (r *UserRepository) FindByID(id int) (*usermodel.User, error) {
	var user usermodel.User
	err := r.db.First(&user, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// FindByUsername 根据用户名查找用户
func (r *UserRepository) FindByUsername(username string) (*usermodel.User, error) {
	var user usermodel.User
	err := r.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// FindByEmail 根据邮箱查找用户（用于 OIDC 身份映射）
func (r *UserRepository) FindByEmail(email string) (*usermodel.User, error) {
	if email == "" {
		return nil, ErrUserNotFound
	}
	var user usermodel.User
	err := r.db.Where("email = ? AND is_delete = 0", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// Update 更新用户信息
func (r *UserRepository) Update(user *usermodel.User) error {
	return r.db.Save(user).Error
}

// Delete 删除用户 (软删除)
func (r *UserRepository) Delete(id int) error {
	return r.db.Model(&usermodel.User{}).Where("user_id = ?", id).Update("is_delete", 1).Error
}

// FindAll 获取所有用户 (分页, 可选角色过滤)
func (r *UserRepository) FindAll(offset, limit int, roleID int, keyword string) ([]*usermodel.User, int64, error) {
	var users []*usermodel.User
	var total int64

	query := r.db.Model(&usermodel.User{}).Scopes(repos.NotDeleted)

	if roleID > 0 {
		query = query.Where("role_id = ?", roleID)
	}
	if keyword != "" {
		query = query.Where("username LIKE ? OR given_name LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Offset(offset).Limit(limit).Order("user_id ASC").Find(&users).Error
	return users, total, err
}

// FindByRoleID 根据角色 ID 查找用户
func (r *UserRepository) FindByRoleID(roleID int) ([]*usermodel.User, error) {
	var users []*usermodel.User
	err := r.db.Where("role_id = ?", roleID).Scopes(repos.NotDeleted).Find(&users).Error
	return users, err
}

// UpdateRole 变更用户所属角色
func (r *UserRepository) UpdateRole(id int, roleID int) error {
	return r.db.Model(&usermodel.User{}).Where("user_id = ?", id).Update("role_id", roleID).Error
}

// ResetUsersByRoleID 将指定角色的所有非管理员用户重置为 role_id=3（普通用户）
func (r *UserRepository) ResetUsersByRoleID(roleID int, newRoleID int) error {
	return r.db.Model(&usermodel.User{}).Where("role_id = ?", roleID).Scopes(repos.NotDeleted).Update("role_id", newRoleID).Error
}

// UpdateLastLogin 更新最后登录信息
func (r *UserRepository) UpdateLastLogin(id int, ip string) error {
	return r.db.Model(&usermodel.User{}).Where("user_id = ?", id).Updates(map[string]interface{}{
		"last_ip":   ip,
		"last_time": gorm.Expr("UNIX_TIMESTAMP(NOW())"),
	}).Error
}

// FindAllForExport 获取所有用户（用于导出，不分页）
func (r *UserRepository) FindAllForExport() ([]*usermodel.User, error) {
	var users []*usermodel.User
	err := r.db.Scopes(repos.NotDeleted).Order("user_id ASC").Find(&users).Error
	return users, err
}

// UpdatePassword 更新密码
func (r *UserRepository) UpdatePassword(id int, password string) error {
	return r.db.Model(&usermodel.User{}).Where("user_id = ?", id).Update("password", password).Error
}

// UpdatePreferences 只写 preferences 这一列。
// 单独一条而不是复用 Update(user)（那是整行 Save）：偏好是每次切换主题都会发的小写，
// 走整行等于"改个主题顺带把我刚读到的昵称/邮箱再写一遍" ——
// 与同一时刻进行的资料编辑互相覆盖，而这种丢数据只在开两个标签页时才看得见。
func (r *UserRepository) UpdatePreferences(id int, raw string, updateTime int) error {
	return r.db.Model(&usermodel.User{}).Where("user_id = ?", id).
		Updates(map[string]any{"preferences": raw, "update_time": updateTime}).Error
}

// ToggleForbidden 切换用户禁用状态
func (r *UserRepository) ToggleForbidden(id int, isForbidden int) error {
	return r.db.Model(&usermodel.User{}).Where("user_id = ?", id).Update("is_forbidden", isForbidden).Error
}

// CountNormalUsers 统计正常用户数
func (r *UserRepository) CountNormalUsers() (int64, error) {
	var count int64
	err := r.db.Model(&usermodel.User{}).Scopes(repos.NotDeleted).Where("is_forbidden = 0").Count(&count).Error
	return count, err
}

// CountForbiddenUsers 统计禁用用户数
func (r *UserRepository) CountForbiddenUsers() (int64, error) {
	var count int64
	err := r.db.Model(&usermodel.User{}).Scopes(repos.NotDeleted).Where("is_forbidden = 1").Count(&count).Error
	return count, err
}

// UpdateLastActivity 更新用户最后活跃时间（每次请求时调用）
func (r *UserRepository) UpdateLastActivity(id int) error {
	return r.db.Model(&usermodel.User{}).Where("user_id = ?", id).Update("last_time", gorm.Expr("UNIX_TIMESTAMP(NOW())")).Error
}

// CountOnlineUsers 统计最近30分钟内有活动的在线用户数
func (r *UserRepository) CountOnlineUsers() (int64, error) {
	threshold := time.Now().Add(-30 * time.Minute).Unix()
	var count int64
	err := r.db.Model(&usermodel.User{}).Scopes(repos.NotDeleted).Where("last_time >= ?", threshold).Count(&count).Error
	return count, err
}

// GetTopCreator 获取创建文档最多的用户
func (r *UserRepository) GetTopCreator() *adminmodel.UserStatItem {
	var result struct {
		UserID    int
		Username  string
		GivenName string
		Count     int
	}
	err := r.db.Table("aw_document").
		Select("aw_document.create_user_id as user_id, aw_user.username, aw_user.given_name, COUNT(*) as count").
		Joins("LEFT JOIN aw_user ON aw_document.create_user_id = aw_user.user_id").Scopes(repos.NotDeleted).
		Group("aw_document.create_user_id").
		Order("count DESC").
		Limit(1).
		Scan(&result).Error
	if err != nil || result.UserID == 0 {
		return nil
	}
	return &adminmodel.UserStatItem{UserID: result.UserID, Username: result.Username, GivenName: result.GivenName, Count: result.Count}
}

// GetTopEditor 获取编辑文档最多的用户
func (r *UserRepository) GetTopEditor() *adminmodel.UserStatItem {
	var result struct {
		UserID    int
		Username  string
		GivenName string
		Count     int
	}
	err := r.db.Table("aw_document").
		Select("aw_document.edit_user_id as user_id, aw_user.username, aw_user.given_name, COUNT(*) as count").
		Joins("LEFT JOIN aw_user ON aw_document.edit_user_id = aw_user.user_id").Scopes(repos.NotDeleted).
		Group("aw_document.edit_user_id").
		Order("count DESC").
		Limit(1).
		Scan(&result).Error
	if err != nil || result.UserID == 0 {
		return nil
	}
	return &adminmodel.UserStatItem{UserID: result.UserID, Username: result.Username, GivenName: result.GivenName, Count: result.Count}
}

// FindByIDs 根据 ID 列表批量查询用户
func (r *UserRepository) FindByIDs(ids []int) ([]*usermodel.User, error) {
	if len(ids) == 0 {
		return []*usermodel.User{}, nil
	}
	var users []*usermodel.User
	err := r.db.Where("user_id IN ?", ids).Find(&users).Error
	return users, err
}

// GetActiveContributors 获取活跃贡献者（综合创建、编辑、收藏、关注）
func (r *UserRepository) GetActiveContributors(limit int) ([]adminmodel.UserStatItem, error) {
	var creators []adminmodel.UserStatItem
	if err := r.db.Table("aw_document").
		Select("aw_document.create_user_id as user_id, aw_user.username, aw_user.given_name, aw_user.last_time, aw_user.is_forbidden, aw_user.is_delete, COUNT(*) as count").
		Joins("LEFT JOIN aw_user ON aw_document.create_user_id = aw_user.user_id").Scopes(repos.NotDeleted).
		Group("aw_document.create_user_id").
		Order("count DESC").
		Limit(limit).
		Scan(&creators).Error; err != nil {
		return nil, err
	}

	var editors []adminmodel.UserStatItem
	if err := r.db.Table("aw_document").
		Select("aw_document.edit_user_id as user_id, aw_user.username, aw_user.given_name, aw_user.last_time, aw_user.is_forbidden, aw_user.is_delete, COUNT(*) as count").
		Joins("LEFT JOIN aw_user ON aw_document.edit_user_id = aw_user.user_id").Scopes(repos.NotDeleted).
		Where("aw_document.edit_user_id > 0").
		Group("aw_document.edit_user_id").
		Order("count DESC").
		Limit(limit).
		Scan(&editors).Error; err != nil {
		return nil, err
	}

	userScores := make(map[int]*adminmodel.UserStatItem)

	for _, creator := range creators {
		if _, ok := userScores[creator.UserID]; !ok {
			userScores[creator.UserID] = &adminmodel.UserStatItem{
				UserID:      creator.UserID,
				Username:    creator.Username,
				GivenName:   creator.GivenName,
				LastTime:    creator.LastTime,
				IsForbidden: creator.IsForbidden,
				IsDelete:    creator.IsDelete,
				Count:       0,
			}
		}
		userScores[creator.UserID].Count += creator.Count * 3
	}

	for _, editor := range editors {
		if _, ok := userScores[editor.UserID]; !ok {
			userScores[editor.UserID] = &adminmodel.UserStatItem{
				UserID:      editor.UserID,
				Username:    editor.Username,
				GivenName:   editor.GivenName,
				LastTime:    editor.LastTime,
				IsForbidden: editor.IsForbidden,
				IsDelete:    editor.IsDelete,
				Count:       0,
			}
		}
		userScores[editor.UserID].Count += editor.Count * 2
	}

	var items []adminmodel.UserStatItem
	for _, item := range userScores {
		items = append(items, *item)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Count > items[j].Count
	})

	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}
