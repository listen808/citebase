package user

import (
	"database/sql"
	"errors"

	usermodel "github.com/listen808/citebase/internal/model/user"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrRoleNotFound = errors.New("role not found")
)

type RoleRepository struct {
	db *gorm.DB
}

func NewRoleRepository() *RoleRepository {
	return &RoleRepository{}
}

func NewRoleRepositoryWithDB(db *gorm.DB) *RoleRepository {
	return &RoleRepository{db: db}
}

// getDB 懒加载获取DB实例，解决包级变量初始化时DB为nil的问题
func (r *RoleRepository) getDB() *gorm.DB {
	if r.db != nil {
		return r.db
	}
	return repos.DB
}

func (r *RoleRepository) Transaction(fn func(tx *gorm.DB) error) error {
	return r.getDB().Transaction(fn)
}

func (r *RoleRepository) FindRoleByID(roleID int) (*usermodel.Role, error) {
	var role usermodel.Role
	err := r.getDB().First(&role, roleID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, err
	}
	return &role, nil
}

func (r *RoleRepository) FindAllRoles() ([]*usermodel.Role, error) {
	var roles []*usermodel.Role
	err := r.getDB().Scopes(repos.NotDeleted).Order("role_id ASC").Find(&roles).Error
	return roles, err
}

func (r *RoleRepository) HasPrivilege(userID int, controller, action string) (bool, error) {
	var user usermodel.User
	if err := r.getDB().First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	if user.RoleID == 1 {
		return true, nil
	}

	var count int64
	err := r.getDB().Table("aw_role_privilege rp").
		Joins("JOIN aw_privilege p ON rp.privilege_id = p.privilege_id").
		Where("rp.role_id = ? AND p.controller = ? AND p.action = ?", user.RoleID, controller, action).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *RoleRepository) GetUserPrivilegeIDs(userID int) ([]int, error) {
	var user usermodel.User
	if err := r.getDB().First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return []int{}, nil
		}
		return nil, err
	}

	if user.RoleID == 1 {
		var allPrivilegeIDs []int
		if err := r.getDB().Model(&usermodel.Privilege{}).Pluck("privilege_id", &allPrivilegeIDs).Error; err != nil {
			return nil, err
		}
		return allPrivilegeIDs, nil
	}

	var ids []int
	err := r.getDB().Table("aw_role_privilege").
		Where("role_id = ?", user.RoleID).
		Pluck("privilege_id", &ids).Error
	return ids, err
}

func (r *RoleRepository) GetPrivilegeTree() ([]*usermodel.PrivilegeNode, error) {
	var privileges []*usermodel.Privilege
	err := r.getDB().Where("is_display = 1").Order("sequence ASC").Find(&privileges).Error
	if err != nil {
		return nil, err
	}

	nodeMap := make(map[int]*usermodel.PrivilegeNode)
	var roots []*usermodel.PrivilegeNode

	for _, p := range privileges {
		node := &usermodel.PrivilegeNode{
			ID:         p.ID,
			Name:       p.Name,
			ParentID:   p.ParentID,
			Type:       p.Type,
			Controller: p.Controller,
			Action:     p.Action,
			Icon:       p.Icon,
			IsDisplay:  p.IsDisplay,
		}
		nodeMap[p.ID] = node
		if p.ParentID == 0 {
			roots = append(roots, node)
		} else {
			if parent, ok := nodeMap[p.ParentID]; ok {
				if parent.Children == nil {
					parent.Children = []*usermodel.PrivilegeNode{}
				}
				parent.Children = append(parent.Children, node)
			}
		}
	}

	return roots, nil
}

func (r *RoleRepository) GetRolePrivileges(roleID int) ([]*usermodel.Privilege, error) {
	var privileges []*usermodel.Privilege
	err := r.getDB().Table("aw_privilege p").
		Joins("JOIN aw_role_privilege rp ON p.privilege_id = rp.privilege_id").
		Where("rp.role_id = ?", roleID).
		Find(&privileges).Error
	return privileges, err
}

func (r *RoleRepository) IsSuperAdmin(userID int) (bool, error) {
	var user usermodel.User
	if err := r.getDB().First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return user.RoleID == 1, nil
}

func (r *RoleRepository) IsAdmin(userID int) (bool, error) {
	var user usermodel.User
	if err := r.getDB().First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return user.RoleID == 1 || user.RoleID == 2, nil
}

func (r *RoleRepository) CreateRole(name string, roleType int) (*usermodel.Role, error) {
	role := &usermodel.Role{Name: name, Type: roleType}
	if err := r.getDB().Create(role).Error; err != nil {
		return nil, err
	}
	return role, nil
}

func (r *RoleRepository) UpdateRole(id int, name string) error {
	return r.getDB().Model(&usermodel.Role{}).Where("role_id = ?", id).Update("name", name).Error
}

func (r *RoleRepository) DeleteRole(id int) error {
	return r.getDB().Model(&usermodel.Role{}).Where("role_id = ?", id).Update("is_delete", 1).Error
}

func (r *RoleRepository) SetRolePrivileges(roleID int, privilegeIDs []int) error {
	return r.getDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", roleID).Delete(&usermodel.RolePrivilege{}).Error; err != nil {
			return err
		}
		for _, pid := range privilegeIDs {
			if err := tx.Create(&usermodel.RolePrivilege{RoleID: roleID, PrivilegeID: pid}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *RoleRepository) FindByID(id int) (*usermodel.Role, error) {
	var role usermodel.Role
	if err := r.getDB().Where("role_id = ?", id).Scopes(repos.NotDeleted).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRoleNotFound
		}
		return nil, err
	}
	return &role, nil
}

func (r *RoleRepository) GetRolePrivilegeIDs(roleID int) ([]int, error) {
	var ids []int
	err := r.getDB().Model(&usermodel.RolePrivilege{}).Where("role_id = ?", roleID).Pluck("privilege_id", &ids).Error
	return ids, err
}

// IsSuperAdminByID 判定角色是否为超级管理员。
// 旧实现返回 role.Type == 1，但 Type=1 的含义是「系统内置角色」——超管/管理员/普通用户
// 三个内置角色都带 Type=1，导致管理员与普通用户角色也被上层当作超管保护、完全无法维护。
// 真正的超管语义与 IsSuperAdmin（user.RoleID==1）一致：只有 role_id==1。
func (r *RoleRepository) IsSuperAdminByID(roleID int) (bool, error) {
	return roleID == 1, nil
}

func (r *RoleRepository) CreatePrivilege(p *usermodel.Privilege) error {
	return r.getDB().Create(p).Error
}

func (r *RoleRepository) GetPrivilegeByID(id int) (*usermodel.Privilege, error) {
	var p usermodel.Privilege
	if err := r.getDB().Where("privilege_id = ?", id).Scopes(repos.NotDeleted).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *RoleRepository) UpdatePrivilege(id int, updates map[string]interface{}) error {
	return r.getDB().Model(&usermodel.Privilege{}).Where("privilege_id = ?", id).Updates(updates).Error
}

func (r *RoleRepository) DeletePrivilege(id int) error {
	return r.getDB().Model(&usermodel.Privilege{}).Where("privilege_id = ?", id).Update("is_delete", 1).Error
}
