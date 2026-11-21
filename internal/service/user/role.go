package user

import (
	"errors"

	usermodel "github.com/listen808/citebase/internal/model/user"
	userrepo "github.com/listen808/citebase/internal/repository/user"
)

var (
	ErrCannotModifySuperAdmin = errors.New("cannot modify super admin")
)

// RoleService 角色权限业务逻辑层
type RoleService struct {
	roleRepo *userrepo.RoleRepository
	userRepo *userrepo.UserRepository
}

func NewRoleService() *RoleService {
	return &RoleService{
		roleRepo: userrepo.NewRoleRepository(),
		userRepo: userrepo.NewUserRepository(),
	}
}

// GetRoles 获取所有角色列表
func (s *RoleService) GetRoles() ([]*usermodel.Role, error) {
	return s.roleRepo.FindAllRoles()
}

// CreateRole 创建角色
func (s *RoleService) CreateRole(name string) (*usermodel.Role, error) {
	return s.roleRepo.CreateRole(name, usermodel.RoleTypeCustom)
}

// UpdateRole 更新角色
func (s *RoleService) UpdateRole(id int, name string) (*usermodel.Role, error) {
	if isSuper, err := s.roleRepo.IsSuperAdminByID(id); err != nil {
		return nil, err
	} else if isSuper {
		return nil, ErrCannotModifySuperAdmin
	}
	if err := s.roleRepo.UpdateRole(id, name); err != nil {
		return nil, err
	}
	return s.roleRepo.FindByID(id)
}

// DeleteRole 删除角色
func (s *RoleService) DeleteRole(id int) error {
	if isSuper, err := s.roleRepo.IsSuperAdminByID(id); err != nil {
		return err
	} else if isSuper {
		return ErrCannotModifySuperAdmin
	}
	return s.roleRepo.DeleteRole(id)
}

// GetPrivileges 获取权限树
func (s *RoleService) GetPrivileges() ([]*usermodel.PrivilegeNode, error) {
	return s.roleRepo.GetPrivilegeTree()
}

// GetRolePrivileges 获取角色权限列表
func (s *RoleService) GetRolePrivileges(id int) ([]*usermodel.Privilege, error) {
	return s.roleRepo.GetRolePrivileges(id)
}

// AssignPrivileges 为角色分配权限
func (s *RoleService) AssignPrivileges(id int, privilegeIDs []int) error {
	if isSuper, err := s.roleRepo.IsSuperAdminByID(id); err != nil {
		return err
	} else if isSuper {
		return ErrCannotModifySuperAdmin
	}
	return s.roleRepo.SetRolePrivileges(id, privilegeIDs)
}

// GetRoleUsers 获取角色下用户列表
func (s *RoleService) GetRoleUsers(id, page, pageSize int) ([]*usermodel.User, int64, string, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	role, err := s.roleRepo.FindByID(id)
	if err != nil {
		return nil, 0, "", err
	}

	users, total, err := s.userRepo.FindAll(offset, pageSize, id, "")
	if err != nil {
		return nil, 0, "", err
	}
	return users, total, role.Name, nil
}

// ResetUsers 将角色下用户重置为普通用户
func (s *RoleService) ResetUsers(id int) error {
	if isSuper, err := s.roleRepo.IsSuperAdminByID(id); err != nil {
		return err
	} else if isSuper {
		return ErrCannotModifySuperAdmin
	}
	return s.userRepo.ResetUsersByRoleID(id, 3)
}

// AddUserToRole 按用户名把用户划入指定角色，返回变更后的用户
func (s *RoleService) AddUserToRole(roleID int, username string) (*usermodel.User, error) {
	if isSuper, err := s.roleRepo.IsSuperAdminByID(roleID); err != nil {
		return nil, err
	} else if isSuper {
		return nil, ErrCannotModifySuperAdmin
	}
	role, err := s.roleRepo.FindByID(roleID)
	if err != nil {
		return nil, err
	}
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return nil, err
	}
	if isSuperAdminUser, err := s.roleRepo.IsSuperAdmin(user.ID); err != nil {
		return nil, err
	} else if isSuperAdminUser {
		return nil, ErrCannotModifySuperAdmin
	}
	if err := s.userRepo.UpdateRole(user.ID, role.ID); err != nil {
		return nil, err
	}
	user.RoleID = role.ID
	return user, nil
}

// CreatePrivilege 创建权限
func (s *RoleService) CreatePrivilege(privilege *usermodel.Privilege) error {
	return s.roleRepo.CreatePrivilege(privilege)
}

// UpdatePrivilege 更新权限
func (s *RoleService) UpdatePrivilege(id int, updates map[string]interface{}) error {
	return s.roleRepo.UpdatePrivilege(id, updates)
}

// GetPrivilegeByID 根据 ID 获取权限
func (s *RoleService) GetPrivilegeByID(id int) (*usermodel.Privilege, error) {
	return s.roleRepo.GetPrivilegeByID(id)
}

// DeletePrivilege 删除权限
func (s *RoleService) DeletePrivilege(id int) error {
	if err := s.roleRepo.DeletePrivilege(id); err != nil {
		return err
	}
	return nil
}
